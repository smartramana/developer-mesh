package core

import (
	"context"
	"sync"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/internal/cache"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/events/system"
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/metrics"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// Engine is the core engine of the MCP server
type Engine struct {
	adapterManager *adapters.AdapterManager
	config         interfaces.CoreConfig
	metricsClient  metrics.Client
	logger         *observability.Logger
	lock           sync.RWMutex
}

// NewEngine creates a new engine
func NewEngine(
	ctx context.Context,
	config interfaces.CoreConfig,
	db *database.Database,
	cacheClient cache.Cache,
	metricsClient metrics.Client,
	_ interfaces.ContextManager, // Keep parameter for backward compatibility
) (*Engine, error) {
	// Create logger
	logger := observability.NewLogger("engine")

	// Create a mock event bus for now to resolve the interface issue
	mockEventBus := &mockEventBus{}

	// Create adapter manager with nil config for now
	// This works because the adapter manager checks for nil config internally
	adapterManager := adapters.NewAdapterManager(nil, nil, mockEventBus, logger, observability.NewMetricsClient())

	// Create engine
	engine := &Engine{
		adapterManager: adapterManager,
		config:         config,
		metricsClient:  metricsClient,
		logger:         logger,
	}

	return engine, nil
}

// GetAdapter gets an adapter by type
func (e *Engine) GetAdapter(adapterType string) (interface{}, error) {
	return e.adapterManager.GetAdapter(adapterType)
}

// HandleAdapterWebhook handles a webhook event using the appropriate adapter
func (e *Engine) HandleAdapterWebhook(ctx context.Context, adapterType string, eventType string, payload []byte) error {
	return e.adapterManager.HandleAdapterWebhook(ctx, adapterType, eventType, payload)
}

// RecordWebhookInContext records a webhook event in a context
func (e *Engine) RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error) {
	return e.adapterManager.RecordWebhookInContext(ctx, agentID, adapterType, eventType, payload)
}

// ExecuteAdapterAction executes an action using the appropriate adapter
func (e *Engine) ExecuteAdapterAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error) {
	return e.adapterManager.ExecuteAction(ctx, contextID, adapterType, action, params)
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

// mockEventBus is a simple implementation of system.EventBus that does nothing
type mockEventBus struct{}

func (m *mockEventBus) Publish(ctx context.Context, event system.Event) error {
	return nil
}

func (m *mockEventBus) Subscribe(eventType system.EventType, handler func(ctx context.Context, event system.Event) error) {
	// Do nothing
}

func (m *mockEventBus) Unsubscribe(eventType system.EventType, handler func(ctx context.Context, event system.Event) error) {
	// Do nothing
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
	
	// Add overall engine status
	health["engine"] = "healthy"
	
	return health
}
