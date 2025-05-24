package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mcp-server/internal/adapters"
	contextManager "mcp-server/internal/core/context"
	coreModels "mcp-server/internal/core/models"
	"github.com/S-Corkum/devops-mcp/pkg/common/aws"
	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/common/events"
	"github.com/S-Corkum/devops-mcp/pkg/common/events/system"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/storage/providers"
)

// MockMetricsClient is a mock implementation of observability.MetricsClient
type MockMetricsClient struct{}

// RecordCounter is a no-op implementation
func (m *MockMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {}

// RecordGauge is a no-op implementation
func (m *MockMetricsClient) RecordGauge(name string, value float64, labels map[string]string) {}

// RecordHistogram is a no-op implementation
func (m *MockMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {}

// RecordTimer is a no-op implementation
func (m *MockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
}

// StartTimer is a no-op implementation
func (m *MockMetricsClient) StartTimer(name string, labels map[string]string) func() {
	return func() {}
}

// RecordCacheOperation is a no-op implementation
func (m *MockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
}

// RecordOperation is a no-op implementation
func (m *MockMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
}

// Close is a no-op implementation
func (m *MockMetricsClient) Close() error { return nil }

// MockSystemEventBus is a mock implementation of system.EventBus
type MockSystemEventBus struct{}

// Publish is a no-op implementation
func (b *MockSystemEventBus) Publish(ctx context.Context, event system.Event) error {
	return nil
}

// Subscribe is a no-op implementation
func (b *MockSystemEventBus) Subscribe(eventType system.EventType, handler func(ctx context.Context, event system.Event) error) {
	// No-op
}

// Unsubscribe is a no-op implementation
func (b *MockSystemEventBus) Unsubscribe(eventType system.EventType, handler func(ctx context.Context, event system.Event) error) {
	// No-op
}

// CoreConfig defines the configuration for the core engine
type CoreConfig interface {
	// GetString gets a string value from the configuration
	GetString(key string) string
	// AWS returns the AWS configuration
	AWS() *aws.AWSConfig
	// S3 returns the S3 configuration if available
	S3() *aws.S3Config
	// GetConcurrencyLimit returns the concurrency limit
	ConcurrencyLimit() int
}

// WebhookHandler defines the interface for handling webhooks
type WebhookHandler interface {
	// HandleWebhook handles a webhook event
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error
}

// ContextManagerInterface defines the interface for the context manager
type ContextManagerInterface interface {
	CreateContext(ctx context.Context, contextData *coreModels.Context) (*coreModels.Context, error)
	GetContext(ctx context.Context, contextID string) (*coreModels.Context, error)
	UpdateContext(ctx context.Context, contextID string, updatedContext *coreModels.Context, options *coreModels.ContextUpdateOptions) (*coreModels.Context, error)
	DeleteContext(ctx context.Context, contextID string) error
	ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*coreModels.Context, error)
}

// Engine is the core engine of the MCP server
type Engine struct {
	adapterManager      *adapters.AdapterManager
	contextManager      ContextManagerInterface
	githubContentManager *GitHubContentManager
	config              CoreConfig
	metricsClient       observability.MetricsClient
	logger              observability.Logger
	eventBus            *events.EventBus
	lock                sync.RWMutex
}

// NewEngine creates a new engine
func NewEngine(
	ctx context.Context,
	config CoreConfig,
	db *database.Database,
	cacheClient cache.Cache,
	metricsClient observability.MetricsClient,
) (*Engine, error) {
	// Create logger
	logger := observability.NewLogger("engine")

	// Create regular event bus for internal use
	eventBus := events.NewEventBus(config.ConcurrencyLimit())

	// Create a mock system event bus to fix compile issues
	systemEventBus := &MockSystemEventBus{}

	// Initialize context storage provider: prefer S3 if configured, otherwise use in-memory
	var storage providers.ContextStorage
	s3Config := config.S3()
	useS3 := s3Config != nil && s3Config.Bucket != ""
	if useS3 && s3Config != nil {
		s3Prefix := "contexts"
		if v, ok := any(config).(interface{ GetString(string) string }); ok {
			if p := v.GetString("storage.context_storage.s3_path_prefix"); p != "" {
				s3Prefix = p
			}
		}
		
		// Create a new S3 client with the provided config
		s3Client, err := aws.NewS3Client(ctx, *s3Config)
		if err != nil {
			logger.Warn("Failed to initialize S3 client, falling back to in-memory context storage", map[string]interface{}{"error": err.Error()})
			storage = providers.NewInMemoryContextStorage()
		} else {
			// Use the bucket from s3Config which was already obtained from config.S3()
			logger.Info("Using S3 context storage", map[string]interface{}{"bucket": s3Config.Bucket, "prefix": s3Prefix})
			storage = providers.NewS3ContextStorage(s3Client, s3Prefix)
		}
	} else {
		logger.Info("Using in-memory context storage", nil)
		storage = providers.NewInMemoryContextStorage()
	}

	// Use the correct event bus and metrics types
	// system.NewSimpleEventBus returns *system.SimpleEventBus, which implements the required interface
	// Use a new observability.MetricsClient for the context manager (not metrics.Client)
	// Pass observability.NewMetricsClient() as observability.MetricsClient interface
	ctxManager := contextManager.NewManager(
		db,
		cacheClient,
		storage,
		nil, // Event bus (set to nil for now, or use a real one if available)
		logger.WithPrefix("context_manager"),
		observability.NewMetricsClient(),
	)

	// For now, pass the config directly to the adapter manager
	// We'll refactor the adapter manager later to handle interfaces.CoreConfig

	// Use a simpler adapter manager initialization to avoid type compatibility issues
	logger.Info("Initializing adapter manager", nil)

	// Create a new adapter manager using the constructor function instead of direct struct initialization
	// This avoids accessing unexported fields and uses the correct event bus type
	adapterManager := adapters.NewAdapterManager(
		nil,            // Config - we'll use nil for now
		nil,            // Context manager - we'll use nil for now
		systemEventBus, // System event bus - using the system.SimpleEventBus
		logger,         // Logger
		nil,            // Metrics client - we'll use nil for simplicity while fixing issues
	)

	// Create GitHub content manager
	var githubContentManager *GitHubContentManager
	
	// If S3 storage is configured, use the same client for GitHub content
	if useS3 {
		// Use the existing s3Config that was already obtained
		s3ClientForGithub, err := aws.NewS3Client(ctx, *s3Config)
		
		if err == nil {
			// Create a dummy MetricsClient that matches the observability.MetricsClient interface
			obsMetricsClient := observability.NewMetricsClient()
			
			// Create the GitHub content manager
			githubContentManager, err = NewGitHubContentManager(db, s3ClientForGithub, obsMetricsClient, nil)
			if err != nil {
				logger.Warn("Failed to create GitHub content manager, continuing without it", map[string]interface{}{
					"error": err.Error(),
				})
			}
		} else {
			logger.Warn("Failed to create S3 client for GitHub content, continuing without it", map[string]interface{}{
				"error": err.Error(),
			})
		}
	} else {
		logger.Info("S3 storage not configured, GitHub content manager not created", nil)
	}
	
	// Create engine
	engine := &Engine{
		adapterManager:      adapterManager,
		contextManager:      ctxManager,
		githubContentManager: githubContentManager,
		config:              config,
		metricsClient:       metricsClient,
		logger:              logger,
		eventBus:            eventBus,
	}

	return engine, nil
}

// GetAdapter gets an adapter by type
func (e *Engine) GetAdapter(adapterType string) (interface{}, error) {
	return e.adapterManager.GetAdapter(adapterType)
}

// GetContextManager returns the context manager
func (e *Engine) GetContextManager() ContextManagerInterface {
	return e.contextManager
}

// GetGitHubContentManager returns the GitHub content manager
func (e *Engine) GetGitHubContentManager() *GitHubContentManager {
	return e.githubContentManager
}

// ExecuteAdapterAction executes an action using the appropriate adapter
func (e *Engine) ExecuteAdapterAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error) {
	return e.adapterManager.ExecuteAction(ctx, contextID, adapterType, action, params)
}

// HandleAdapterWebhook handles a webhook event using the appropriate adapter
func (e *Engine) HandleAdapterWebhook(ctx context.Context, adapterType string, eventType string, payload []byte) error {
	// Use adapter manager to handle the webhook
	adapter, err := e.adapterManager.GetAdapter(adapterType)
	if err != nil {
		return fmt.Errorf("adapter not found: %w", err)
	}

	// Check if the adapter implements webhook handling
	if webhookHandler, ok := adapter.(WebhookHandler); ok {
		return webhookHandler.HandleWebhook(ctx, eventType, payload)
	}

	return fmt.Errorf("adapter does not support webhooks")
}

// RecordWebhookInContext records a webhook event in a context
func (e *Engine) RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error) {
	// Get or create context for agent
	contexts, err := e.contextManager.ListContexts(ctx, agentID, "", map[string]interface{}{"limit": 1})
	if err != nil || len(contexts) == 0 {
		// Create new context if none exists
		contextData := &coreModels.Context{
			AgentID:   agentID,
			ModelID:   "unknown", // Set appropriate default
			MaxTokens: 4000,      // Default value
		}

		newContext, err := e.contextManager.CreateContext(ctx, contextData)
		if err != nil {
			return "", err
		}

		contexts = []*coreModels.Context{newContext}
	}

	contextID := contexts[0].ID

	// Format webhook event as context item
	webhookItem := coreModels.ContextItem{
		Role:    "webhook",
		Content: fmt.Sprintf("Webhook event: %s from %s", eventType, adapterType),
		Tokens:  1, // Set appropriate token count or calculate based on content
		Metadata: map[string]interface{}{
			"adapter_type": adapterType,
			"event_type":   eventType,
			"payload":      payload,
		},
		Timestamp: time.Now(),
	}

	// Update context with webhook event
	updateData := &coreModels.Context{
		Content: []coreModels.ContextItem{webhookItem},
	}

	// Create core models version of the update options
	options := &coreModels.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	}

	_, err = e.contextManager.UpdateContext(ctx, contextID, updateData, options)
	if err != nil {
		return "", err
	}

	return contextID, nil
}

// Close releases all engine resources
func (e *Engine) Close() {
	if e.eventBus != nil {
		e.eventBus.Close()
	}
	e.Shutdown(context.Background())
}

// Shutdown performs a graceful shutdown of the engine
func (e *Engine) Shutdown(ctx context.Context) error {
	// Shutdown adapter manager
	if e.adapterManager != nil {
		if err := e.adapterManager.Shutdown(ctx); err != nil {
			e.logger.Warn("Error shutting down adapter manager", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return nil
}

// We'll remove this function as we're no longer using it

// Health returns the health status of all components
func (e *Engine) Health() map[string]string {
	// Create a map to store component health statuses
	health := make(map[string]string)

	// Add adapter manager health status if available
	if e.adapterManager != nil {
		health["adapter_manager"] = "healthy"
	} else {
		health["adapter_manager"] = "not available"
	}

	// Add context manager health status
	if e.contextManager != nil {
		health["context_manager"] = "healthy"
	} else {
		health["context_manager"] = "not available"
	}

	// Add overall engine status
	health["engine"] = "healthy"

	return health
}
