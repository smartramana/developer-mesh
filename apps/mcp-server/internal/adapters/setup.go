package adapters

import (
	"context"

	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/adapters/bridge"
	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/adapters/core"
	adapterEvents "github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/adapters/providers"
	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/common/events"
	"github.com/S-Corkum/devops-mcp/pkg/common/events/system"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// AdapterManager manages the lifecycle of adapters
type AdapterManager struct {
	factory         *core.DefaultAdapterFactory
	registry        *core.AdapterRegistry
	adapterEventBus *adapterEvents.EventBus
	systemEventBus  *events.EventBus
	eventBridge     *bridge.EventBridge
	logger          observability.Logger
	MetricsClient   observability.MetricsClient
}

// NewAdapterManager creates a new adapter manager
func NewAdapterManager(
	cfg *config.Config,
	_ interface{}, // Formerly contextManager, kept for backward compatibility
	systemEventBus system.EventBus,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
) *AdapterManager {
	// Create events bus for adapters
	adapterEventBus := adapterEvents.NewEventBus(logger)

	// Create system event bus adapter that implements events.EventBus
	systemEventBusAdapter := events.NewEventBus(5) // Use 5 workers like default

	// Create adapter factory with empty map if config is nil
	var adapterConfigs map[string]interface{}
	if cfg != nil && cfg.Adapters != nil {
		adapterConfigs = cfg.Adapters
	} else {
		adapterConfigs = make(map[string]interface{})
	}

	factory := core.NewAdapterFactory(
		adapterConfigs,
		metricsClient,
		logger,
	)

	// Create adapter registry
	registry := core.NewAdapterRegistry(factory, logger)

	// Create event bridge using the adapter event bus directly
	// NOTE: We pass the event bus as an interface{} and handle the type conversion inside NewEventBridge
	eventBridge := bridge.NewEventBridge(adapterEventBus, systemEventBus, logger, registry)

	// Register providers with the factory
	// NOTE: We pass the systemEventBusAdapter directly as interface{}
	providers.RegisterAllProviders(factory, systemEventBusAdapter, metricsClient, logger)

	// Create manager
	manager := &AdapterManager{
		factory:         factory,
		registry:        registry,
		adapterEventBus: adapterEventBus,
		systemEventBus:  systemEventBusAdapter,
		eventBridge:     eventBridge,
		logger:          logger,
		MetricsClient:   metricsClient,
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
func (m *AdapterManager) GetAdapter(adapterType string) (interface{}, error) {
	adapter, err := m.registry.GetAdapter(context.Background(), adapterType)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

// ExecuteAction executes an action with an adapter
func (m *AdapterManager) ExecuteAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error) {
	// Get adapter
	adapter, err := m.registry.GetAdapter(ctx, adapterType)
	if err != nil {
		return nil, err
	}

	// Log the operation
	m.logger.Info("Executing adapter action", map[string]interface{}{
		"adapterType": adapterType,
		"action":      action,
		"contextID":   contextID,
	})

	// Execute action
	result, err := adapter.ExecuteAction(ctx, contextID, action, params)

	return result, err
}

// Close releases all event bus resources
func (m *AdapterManager) Close() {
	if m.adapterEventBus != nil {
		m.adapterEventBus.Close()
	}
	if m.systemEventBus != nil {
		m.systemEventBus.Close()
	}
}

// Shutdown gracefully shuts down all adapters
func (m *AdapterManager) Shutdown(ctx context.Context) error {
	m.Close()
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
