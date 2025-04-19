package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/internal/cache"
	contextManager "github.com/S-Corkum/mcp-server/internal/core/context"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/events/system"
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/metrics"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/internal/storage"
	"github.com/S-Corkum/mcp-server/internal/storage/providers"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// Engine is the core engine of the MCP server
type Engine struct {
	adapterManager *adapters.AdapterManager
	contextManager *contextManager.Manager
	config         interfaces.CoreConfig
	metricsClient  metrics.Client
	logger         *observability.Logger
	eventBus       *system.EventBus
	lock           sync.RWMutex
}

// NewEngine creates a new engine
func NewEngine(
	ctx context.Context,
	config interfaces.CoreConfig,
	db *database.Database,
	cacheClient cache.Cache,
	metricsClient metrics.Client,
) (*Engine, error) {
	// Create logger
	logger := observability.NewLogger("engine")

	// Create event bus
	eventBus := system.NewEventBus()

	// Set up context storage
	var contextStorage providers.ContextStorage

	// Check if AWS S3 is configured
	if config.AWS != nil && config.AWS.S3 != nil {
		// Create S3 client
		s3Client, err := storage.NewS3Client(ctx, *config.AWS.S3)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 client: %w", err)
		}

		// Create S3 context storage
		contextStorage = providers.NewS3ContextStorage(s3Client, "contexts")
	} else {
		// Create a simple in-memory context storage for development/testing
		logger.Warn("AWS S3 is not configured, using in-memory context storage", nil)
		contextStorage = &providers.InMemoryContextStorage{} // You may need to implement this
	}

	// Create context manager
	ctxManager := contextManager.NewManager(
		db,
		cacheClient,
		contextStorage,
		eventBus,
		logger,
		observability.NewMetricsClient(),
	)

	// Create adapter manager
	adapterManager := adapters.NewAdapterManager(
		config,
		ctxManager,
		eventBus,
		logger,
		observability.NewMetricsClient(),
	)

	// Create engine
	engine := &Engine{
		adapterManager: adapterManager,
		contextManager: ctxManager,
		config:         config,
		metricsClient:  metricsClient,
		logger:         logger,
		eventBus:       eventBus,
	}

	return engine, nil
}

// GetAdapter gets an adapter by type
func (e *Engine) GetAdapter(adapterType string) (interface{}, error) {
	return e.adapterManager.GetAdapter(adapterType)
}

// GetContextManager returns the context manager
func (e *Engine) GetContextManager() interfaces.ContextManager {
	return e.contextManager
}

// ExecuteAdapterAction executes an action using the appropriate adapter
func (e *Engine) ExecuteAdapterAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error) {
	return e.adapterManager.ExecuteAction(ctx, contextID, adapterType, action, params)
}

// HandleAdapterWebhook handles a webhook event using the appropriate adapter
func (e *Engine) HandleAdapterWebhook(ctx context.Context, adapterType string, eventType string, payload []byte) error {
	// Use adapter manager to handle the webhook
	return e.adapterManager.HandleWebhook(ctx, adapterType, eventType, payload)
}

// RecordWebhookInContext records a webhook event in a context
func (e *Engine) RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error) {
	// Get or create context for agent
	contexts, err := e.contextManager.ListContexts(ctx, agentID, "", map[string]interface{}{"limit": 1})
	if err != nil || len(contexts) == 0 {
		// Create new context if none exists
		contextData := &mcp.Context{
			AgentID:   agentID,
			ModelID:   "unknown", // Set appropriate default
			MaxTokens: 4000,      // Default value
		}
		
		newContext, err := e.contextManager.CreateContext(ctx, contextData)
		if err != nil {
			return "", err
		}
		
		contexts = []*mcp.Context{newContext}
	}
	
	contextID := contexts[0].ID
	
	// Format webhook event as context item
	webhookItem := mcp.ContextItem{
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
	updateData := &mcp.Context{
		Content: []mcp.ContextItem{webhookItem},
	}
	
	options := &mcp.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	}
	
	_, err = e.contextManager.UpdateContext(ctx, contextID, updateData, options)
	if err != nil {
		return "", err
	}
	
	return contextID, nil
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
