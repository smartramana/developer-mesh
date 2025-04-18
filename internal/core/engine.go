package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/internal/adapters/artifactory"
	"github.com/S-Corkum/mcp-server/internal/adapters/github"
	"github.com/S-Corkum/mcp-server/internal/adapters/harness"
	"github.com/S-Corkum/mcp-server/internal/adapters/sonarqube"
	"github.com/S-Corkum/mcp-server/internal/adapters/xray"
	"github.com/S-Corkum/mcp-server/internal/cache"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/metrics"
	"github.com/S-Corkum/mcp-server/internal/safety"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// Config holds configuration for the MCP engine
type Config struct {
	// Event processing configuration
	EventBufferSize  int           `mapstructure:"event_buffer_size"`
	ConcurrencyLimit int           `mapstructure:"concurrency_limit"`
	EventTimeout     time.Duration `mapstructure:"event_timeout"`

	// Adapter configurations
	GithubConfig      github.Config      `mapstructure:"github"`
	HarnessConfig     harness.Config     `mapstructure:"harness"`
	SonarQubeConfig   sonarqube.Config   `mapstructure:"sonarqube"`
	ArtifactoryConfig artifactory.Config `mapstructure:"artifactory"`
	XrayConfig        xray.Config        `mapstructure:"xray"`
}

// Engine is the core MCP engine
type Engine struct {
	config         Config
	adapters       map[string]interfaces.Adapter
	events         chan mcp.Event
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	db             *database.Database
	cacheClient    cache.Cache
	metricsClient  metrics.Client
	ContextManager interfaces.ContextManager
	AdapterBridge  *AdapterContextBridge
}

// NewEngine creates a new MCP engine
func NewEngine(ctx context.Context, cfg Config, db *database.Database, cacheClient cache.Cache, metricsClient metrics.Client, contextManager interfaces.ContextManager) (*Engine, error) {
	engineCtx, cancel := context.WithCancel(ctx)

	engine := &Engine{
		config:        cfg,
		adapters:      make(map[string]interfaces.Adapter),
		events:        make(chan mcp.Event, cfg.EventBufferSize),
		ctx:           engineCtx,
		cancel:        cancel,
		db:            db,
		cacheClient:   cacheClient,
		metricsClient: metricsClient,
	}

	// Use provided context manager or create a new one
	if contextManager != nil {
		engine.ContextManager = contextManager
	} else {
		// Fall back to default context manager
		engine.ContextManager = NewContextManager(db, cacheClient)
	}

	// Initialize adapters
	if err := engine.initializeAdapters(); err != nil {
		cancel()
		return nil, err
	}

	// Initialize Adapter Bridge after both adapters and context manager are ready
	engine.AdapterBridge = NewAdapterContextBridge(engine.ContextManager, engine.adapters)

	// Start event processing
	engine.startEventProcessors(cfg.ConcurrencyLimit)

	return engine, nil
}

// initializeAdapters initializes all configured adapters
func (e *Engine) initializeAdapters() error {

	// Initialize GitHub adapter if configured
	if e.config.GithubConfig.APIToken != "" {
		githubAdapter, err := github.NewAdapter(e.config.GithubConfig)
		if err != nil {
			return err
		}
		
		// Initialize the adapter with the configuration
		if err := githubAdapter.Initialize(e.ctx, e.config.GithubConfig); err != nil {
			return fmt.Errorf("failed to initialize GitHub adapter: %w", err)
		}
		
		e.adapters["github"] = githubAdapter
		
		// Set up event handlers if the adapter implements the necessary methods
		if err = e.setupGithubEventHandlers(githubAdapter); err != nil {
			return err
		}
		
		log.Println("GitHub adapter initialized successfully")
	}

	// Initialize Harness adapter if configured
	if e.config.HarnessConfig.APIToken != "" {
		harnessAdapter, err := harness.NewAdapter(e.config.HarnessConfig)
		if err != nil {
			return err
		}
		
		// Initialize the adapter with the configuration
		if err := harnessAdapter.Initialize(e.ctx, e.config.HarnessConfig); err != nil {
			log.Printf("Warning: Harness adapter initialization warning: %v", err)
			// Don't fail completely if there's an issue
		}
		
		e.adapters["harness"] = harnessAdapter
		log.Println("Harness adapter initialized successfully")
	}

	// Initialize SonarQube adapter if configured
	if e.config.SonarQubeConfig.Token != "" || (e.config.SonarQubeConfig.Username != "" && e.config.SonarQubeConfig.Password != "") {
		sonarQubeAdapter, err := sonarqube.NewAdapter(e.config.SonarQubeConfig)
		if err != nil {
			return err
		}
		
		// Initialize the adapter with the configuration
		if err := sonarQubeAdapter.Initialize(e.ctx, e.config.SonarQubeConfig); err != nil {
			log.Printf("Warning: SonarQube adapter initialization warning: %v", err)
			// Don't fail completely if there's an issue
		}
		
		e.adapters["sonarqube"] = sonarQubeAdapter
		log.Println("SonarQube adapter initialized successfully")
	}

	// Initialize Artifactory adapter if configured
	if e.config.ArtifactoryConfig.Token != "" || (e.config.ArtifactoryConfig.Username != "" && e.config.ArtifactoryConfig.Password != "") {
		artifactoryAdapter, err := artifactory.NewAdapter(e.config.ArtifactoryConfig)
		if err != nil {
			return err
		}
		
		// Initialize the adapter with the configuration
		if err := artifactoryAdapter.Initialize(e.ctx, e.config.ArtifactoryConfig); err != nil {
			log.Printf("Warning: Artifactory adapter initialization warning: %v", err)
			// Don't fail completely if there's an issue
		}
		
		e.adapters["artifactory"] = artifactoryAdapter
		log.Println("Artifactory adapter initialized successfully")
	}

	// Initialize JFrog Xray adapter if configured
	if e.config.XrayConfig.Token != "" || (e.config.XrayConfig.Username != "" && e.config.XrayConfig.Password != "") {
		xrayAdapter, err := xray.NewAdapter(e.config.XrayConfig)
		if err != nil {
			return err
		}
		
		// Initialize the adapter with the configuration
		if err := xrayAdapter.Initialize(e.ctx, e.config.XrayConfig); err != nil {
			log.Printf("Warning: JFrog Xray adapter initialization warning: %v", err)
			// Don't fail completely if there's an issue
		}
		
		e.adapters["xray"] = xrayAdapter
		log.Println("JFrog Xray adapter initialized successfully")
	}

	return nil
}

// setupGithubEventHandlers sets up event handlers for GitHub events
func (e *Engine) setupGithubEventHandlers(adapter interfaces.Adapter) error {
	// Subscribe to pull request events
	if err := adapter.Subscribe("pull_request", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "github",
			Type:      "pull_request",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to push events
	if err := adapter.Subscribe("push", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "github",
			Type:      "push",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	return nil
}



// startEventProcessors starts multiple goroutines for event processing
func (e *Engine) startEventProcessors(count int) {
	for i := 0; i < count; i++ {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			for {
				select {
				case event, ok := <-e.events:
					if !ok {
						return
					}
					e.processEvent(event)
				case <-e.ctx.Done():
					return
				}
			}
		}()
	}
}

// processEvent processes a single event with appropriate error handling
func (e *Engine) processEvent(event mcp.Event) {
	// Create a context with timeout for event processing
	ctx, cancel := context.WithTimeout(e.ctx, e.config.EventTimeout)
	defer cancel()

	// Log the event
	log.Printf("Processing %s event from %s", event.Type, event.Source)

	// Record metrics
	e.metricsClient.RecordEvent(event.Source, event.Type)

	// Process based on event source and type
	switch event.Source {
	case "github":
		e.processGithubEvent(ctx, event)
	default:
		log.Printf("Unknown event source: %s", event.Source)
	}
}

// processGithubEvent processes GitHub events
func (e *Engine) processGithubEvent(ctx context.Context, event mcp.Event) {
	// Implementation specific to GitHub events
	switch event.Type {
	case "pull_request":
		// Process pull request event
	case "push":
		// Process push event
	default:
		log.Printf("Unknown GitHub event type: %s", event.Type)
	}
}

// No additional event processors needed

// Health checks the health of the engine and all adapters
func (e *Engine) Health() map[string]string {
	health := make(map[string]string)
	health["engine"] = "healthy"

	// Check each adapter's health independently
	for name, adapter := range e.adapters {
		status := adapter.Health()
		health[name] = status
	}

	return health
}

// Shutdown gracefully shuts down the engine
func (e *Engine) Shutdown(ctx context.Context) error {
	// Signal all goroutines to stop
	e.cancel()

	// Close the events channel
	close(e.events)

	// Wait for all event processors to finish with a timeout
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All processors completed
	case <-ctx.Done():
		log.Println("Shutdown timed out waiting for event processors")
	}

	// Close all adapters
	for name, adapter := range e.adapters {
		if err := adapter.Close(); err != nil {
			log.Printf("Error closing adapter %s: %v", name, err)
		}
	}

	return nil
}

// GetAdapter returns an adapter by name
func (e *Engine) GetAdapter(name string) (interfaces.Adapter, error) {
	adapter, ok := e.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter not found: %s", name)
	}
	return adapter, nil
}

// ProcessEvent adds an event to the engine's event queue
func (e *Engine) ProcessEvent(event mcp.Event) {
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	
	// Send to event queue
	select {
	case e.events <- event:
		// Event successfully sent
	default:
		// Queue is full, log a warning
		log.Printf("Warning: Event queue is full, dropping event: %s %s", event.Source, event.Type)
	}
}

// ExecuteAdapterAction executes an action on an adapter with context tracking and safety checks
func (e *Engine) ExecuteAdapterAction(ctx context.Context, adapterName string, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Get the adapter
	adapter, err := e.GetAdapter(adapterName)
	if err != nil {
		return nil, err
	}
	
	// Get the appropriate safety checker for this adapter
	safetyChecker := safety.GetCheckerForAdapter(adapterName)
	
	// Safety check - verify the operation is allowed
	isSafe, err := safetyChecker.IsSafeOperation(action, params)
	if err != nil {
		return nil, fmt.Errorf("safety check failed: %w", err)
	}
	
	if !isSafe {
		return nil, fmt.Errorf("operation '%s' on adapter '%s' is restricted for safety reasons", action, adapterName)
	}
	
	// Also run the adapter's internal safety check
	isSafe, err = adapter.IsSafeOperation(action, params)
	if err != nil {
		return nil, fmt.Errorf("adapter safety check failed: %w", err)
	}
	
	if !isSafe {
		return nil, fmt.Errorf("operation '%s' on adapter '%s' is restricted by adapter safety policy", action, adapterName)
	}
	
	// Execute the action
	result, err := adapter.ExecuteAction(ctx, contextID, action, params)
	if err != nil {
		return nil, err
	}
	
	// Record the operation in the context if a contextID is provided
	if contextID != "" {
		contextHelper := adapters.NewContextAwareAdapter(e.ContextManager, adapterName)
		if err := contextHelper.RecordOperationInContext(ctx, contextID, action, params, result); err != nil {
			// Log the error but don't fail the operation
			log.Printf("Warning: Failed to record operation in context: %v", err)
		}
	}
	
	// Log the successful execution of the action for audit purposes
	log.Printf("Executed action '%s' on adapter '%s'", action, adapterName)
	
	return result, nil
}

// ListAdapters returns a list of all available adapters
func (e *Engine) ListAdapters() []string {
	var adapterNames []string
	for name := range e.adapters {
		adapterNames = append(adapterNames, name)
	}
	return adapterNames
}
