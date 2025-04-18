package adapters

import (
	"context"
	
	"github.com/S-Corkum/mcp-server/internal/adapters/bridge"
	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/adapters/providers"
	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
	"github.com/S-Corkum/mcp-server/internal/config"
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// AdapterManager manages the lifecycle of adapters
type AdapterManager struct {
	factory        *core.DefaultAdapterFactory
	registry       *core.AdapterRegistry
	eventBus       *events.EventBus
	contextBridge  *bridge.ContextBridge
	eventBridge    *bridge.EventBridge
	logger         *observability.Logger
	metricsClient  *observability.MetricsClient
}

// NewAdapterManager creates a new adapter manager
func NewAdapterManager(
	cfg *config.Config,
	contextManager interfaces.ContextManager,
	systemEventBus interfaces.EventBus,
	logger *observability.Logger,
	metricsClient *observability.MetricsClient,
) *AdapterManager {
	// Create resilience patterns
	circuitBreakers := resilience.NewCircuitBreakerManager(map[string]resilience.CircuitBreakerConfig{})
	rateLimiters := resilience.NewRateLimiterManager(map[string]resilience.RateLimiterConfig{})
	
	// Create events bus for adapters
	eventBus := events.NewEventBus(logger)
	
	// Create adapter factory
	factory := core.NewAdapterFactory(
		cfg.Adapters,
		metricsClient,
		circuitBreakers,
		rateLimiters,
	)
	
	// Create adapter registry
	registry := core.NewAdapterRegistry(factory, logger)
	
	// Create bridges
	contextBridge := bridge.NewContextBridge(contextManager, logger, eventBus)
	eventBridge := bridge.NewEventBridge(eventBus, systemEventBus, logger, registry)
	
	// Register adapter providers
	providers.RegisterAllProviders(factory, eventBus, metricsClient, logger)
	
	// Create manager
	manager := &AdapterManager{
		factory:       factory,
		registry:      registry,
		eventBus:      eventBus,
		contextBridge: contextBridge,
		eventBridge:   eventBridge,
		logger:        logger,
		metricsClient: metricsClient,
	}
	
	return manager
}

// Initialize initializes all required adapters
func (m *AdapterManager) Initialize(ctx context.Context) error {
	// List of required adapters (can be configured)
	requiredAdapters := []string{
		"github",
	}
	
	// Initialize required adapters
	for _, adapterType := range requiredAdapters {
		_, err := m.registry.GetAdapter(ctx, adapterType)
		if err != nil {
			m.logger.Error("Failed to initialize adapter", map[string]interface{}{
				"adapterType": adapterType,
				"error":       err.Error(),
			})
			return err
		}
	}
	
	return nil
}

// GetAdapter gets an adapter by type
func (m *AdapterManager) GetAdapter(ctx context.Context, adapterType string) (core.Adapter, error) {
	return m.registry.GetAdapter(ctx, adapterType)
}

// ExecuteAction executes an action with an adapter and records it in context
func (m *AdapterManager) ExecuteAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error) {
	// Get adapter
	adapter, err := m.registry.GetAdapter(ctx, adapterType)
	if err != nil {
		return nil, err
	}
	
	// Execute action
	result, err := adapter.ExecuteAction(ctx, contextID, action, params)
	
	// Record in context
	if contextID != "" {
		if recordErr := m.contextBridge.RecordOperationInContext(ctx, contextID, adapterType, action, params, result, err); recordErr != nil {
			m.logger.Warn("Failed to record operation in context", map[string]interface{}{
				"contextID":   contextID,
				"adapterType": adapterType,
				"action":      action,
				"error":       recordErr.Error(),
			})
		}
	}
	
	return result, err
}

// HandleWebhook handles a webhook event
func (m *AdapterManager) HandleWebhook(ctx context.Context, adapterType string, eventType string, payload []byte) error {
	// Get adapter
	adapter, err := m.registry.GetAdapter(ctx, adapterType)
	if err != nil {
		return err
	}
	
	// Handle webhook
	return adapter.HandleWebhook(ctx, eventType, payload)
}

// RecordWebhookInContext records a webhook event in a context
func (m *AdapterManager) RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error) {
	return m.contextBridge.RecordWebhookInContext(ctx, agentID, adapterType, eventType, payload)
}

// Shutdown gracefully shuts down all adapters
func (m *AdapterManager) Shutdown(ctx context.Context) error {
	// Get all adapters
	adapters := m.registry.ListAdapters()
	
	// Close all adapters
	for adapterType, adapter := range adapters {
		if err := adapter.Close(); err != nil {
			m.logger.Warn("Error closing adapter", map[string]interface{}{
				"adapterType": adapterType,
				"error":       err.Error(),
			})
		}
	}
	
	return nil
}
