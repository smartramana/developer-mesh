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
	"github.com/S-Corkum/mcp-server/internal/metrics"
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
	adapters       map[string]adapters.Adapter
	events         chan mcp.Event
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	db             *database.Database
	cacheClient    cache.Cache
	metricsClient  metrics.Client
	ContextManager *ContextManager
	AdapterBridge  *AdapterContextBridge
}

// NewEngine creates a new MCP engine
func NewEngine(ctx context.Context, cfg Config, db *database.Database, cacheClient cache.Cache, metricsClient metrics.Client) (*Engine, error) {
	engineCtx, cancel := context.WithCancel(ctx)

	engine := &Engine{
		config:        cfg,
		adapters:      make(map[string]adapters.Adapter),
		events:        make(chan mcp.Event, cfg.EventBufferSize),
		ctx:           engineCtx,
		cancel:        cancel,
		db:            db,
		cacheClient:   cacheClient,
		metricsClient: metricsClient,
	}

	// Initialize Context Manager
	engine.ContextManager = NewContextManager(db, cacheClient)

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
	var err error

	// Initialize GitHub adapter if configured
	if e.config.GithubConfig.APIToken != "" {
		if e.adapters["github"], err = github.NewAdapter(e.config.GithubConfig); err != nil {
			return err
		}
		if err = e.setupGithubEventHandlers(e.adapters["github"].(*github.Adapter)); err != nil {
			return err
		}
	}

	// Initialize Harness adapter if configured
	if e.config.HarnessConfig.APIToken != "" {
		if e.adapters["harness"], err = harness.NewAdapter(e.config.HarnessConfig); err != nil {
			return err
		}
		if err = e.setupHarnessEventHandlers(e.adapters["harness"].(*harness.Adapter)); err != nil {
			return err
		}
	}

	// Initialize SonarQube adapter if configured
	if e.config.SonarQubeConfig.BaseURL != "" {
		if e.adapters["sonarqube"], err = sonarqube.NewAdapter(e.config.SonarQubeConfig); err != nil {
			return err
		}
		if err = e.setupSonarQubeEventHandlers(e.adapters["sonarqube"].(*sonarqube.Adapter)); err != nil {
			return err
		}
	}

	// Initialize Artifactory adapter if configured
	if e.config.ArtifactoryConfig.BaseURL != "" {
		if e.adapters["artifactory"], err = artifactory.NewAdapter(e.config.ArtifactoryConfig); err != nil {
			return err
		}
		if err = e.setupArtifactoryEventHandlers(e.adapters["artifactory"].(*artifactory.Adapter)); err != nil {
			return err
		}
	}

	// Initialize Xray adapter if configured
	if e.config.XrayConfig.BaseURL != "" {
		if e.adapters["xray"], err = xray.NewAdapter(e.config.XrayConfig); err != nil {
			return err
		}
		if err = e.setupXrayEventHandlers(e.adapters["xray"].(*xray.Adapter)); err != nil {
			return err
		}
	}

	return nil
}

// setupGithubEventHandlers sets up event handlers for GitHub events
func (e *Engine) setupGithubEventHandlers(adapter *github.Adapter) error {
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

// setupHarnessEventHandlers sets up event handlers for Harness events
func (e *Engine) setupHarnessEventHandlers(adapter *harness.Adapter) error {
	// Subscribe to CI build events
	if err := adapter.Subscribe("ci.build", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "harness",
			Type:      "ci.build",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to CD deployment events
	if err := adapter.Subscribe("cd.deployment", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "harness",
			Type:      "cd.deployment",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to STO experiment events
	if err := adapter.Subscribe("sto.experiment", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "harness",
			Type:      "sto.experiment",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to feature flag events
	if err := adapter.Subscribe("ff.change", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "harness",
			Type:      "ff.change",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	return nil
}

// setupSonarQubeEventHandlers sets up event handlers for SonarQube events
func (e *Engine) setupSonarQubeEventHandlers(adapter *sonarqube.Adapter) error {
	// Subscribe to quality gate events
	if err := adapter.Subscribe("quality_gate", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "sonarqube",
			Type:      "quality_gate",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to task completed events
	if err := adapter.Subscribe("task_completed", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "sonarqube",
			Type:      "task_completed",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	return nil
}

// setupArtifactoryEventHandlers sets up event handlers for Artifactory events
func (e *Engine) setupArtifactoryEventHandlers(adapter *artifactory.Adapter) error {
	// Subscribe to artifact created events
	if err := adapter.Subscribe("artifact_created", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "artifactory",
			Type:      "artifact_created",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to artifact deleted events
	if err := adapter.Subscribe("artifact_deleted", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "artifactory",
			Type:      "artifact_deleted",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to artifact property changed events
	if err := adapter.Subscribe("artifact_property_changed", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "artifactory",
			Type:      "artifact_property_changed",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	return nil
}

// setupXrayEventHandlers sets up event handlers for JFrog Xray events
func (e *Engine) setupXrayEventHandlers(adapter *xray.Adapter) error {
	// Subscribe to security violation events
	if err := adapter.Subscribe("security_violation", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "xray",
			Type:      "security_violation",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to license violation events
	if err := adapter.Subscribe("license_violation", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "xray",
			Type:      "license_violation",
			Timestamp: time.Now(),
			Data:      event,
		}
	}); err != nil {
		return err
	}

	// Subscribe to scan completed events
	if err := adapter.Subscribe("scan_completed", func(event interface{}) {
		e.events <- mcp.Event{
			Source:    "xray",
			Type:      "scan_completed",
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
	case "harness":
		e.processHarnessEvent(ctx, event)
	case "sonarqube":
		e.processSonarQubeEvent(ctx, event)
	case "artifactory":
		e.processArtifactoryEvent(ctx, event)
	case "xray":
		e.processXrayEvent(ctx, event)
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

// processHarnessEvent processes Harness events
func (e *Engine) processHarnessEvent(ctx context.Context, event mcp.Event) {
	// Implementation specific to Harness events
	switch event.Type {
	case "ci.build":
		// Process CI build event
	case "cd.deployment":
		// Process CD deployment event
	case "sto.experiment":
		// Process STO experiment event
	case "ff.change":
		// Process feature flag change event
	default:
		log.Printf("Unknown Harness event type: %s", event.Type)
	}
}

// processSonarQubeEvent processes SonarQube events
func (e *Engine) processSonarQubeEvent(ctx context.Context, event mcp.Event) {
	// Implementation specific to SonarQube events
	switch event.Type {
	case "quality_gate":
		// Process quality gate event
	case "task_completed":
		// Process task completed event
	default:
		log.Printf("Unknown SonarQube event type: %s", event.Type)
	}
}

// processArtifactoryEvent processes Artifactory events
func (e *Engine) processArtifactoryEvent(ctx context.Context, event mcp.Event) {
	// Implementation specific to Artifactory events
	switch event.Type {
	case "artifact_created":
		// Process artifact created event
	case "artifact_deleted":
		// Process artifact deleted event
	case "artifact_property_changed":
		// Process artifact property changed event
	default:
		log.Printf("Unknown Artifactory event type: %s", event.Type)
	}
}

// processXrayEvent processes JFrog Xray events
func (e *Engine) processXrayEvent(ctx context.Context, event mcp.Event) {
	// Implementation specific to JFrog Xray events
	switch event.Type {
	case "security_violation":
		// Process security violation event
	case "license_violation":
		// Process license violation event
	case "scan_completed":
		// Process scan completed event
	default:
		log.Printf("Unknown Xray event type: %s", event.Type)
	}
}

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
func (e *Engine) GetAdapter(name string) (adapters.Adapter, error) {
	adapter, ok := e.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter not found: %s", name)
	}
	return adapter, nil
}

// ExecuteAdapterAction executes an action on an adapter with context tracking
func (e *Engine) ExecuteAdapterAction(ctx context.Context, adapterName string, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Get the adapter
	adapter, err := e.GetAdapter(adapterName)
	if err != nil {
		return nil, err
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
