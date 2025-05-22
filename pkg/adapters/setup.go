package adapters

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/bridge"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/core"
	adapterEvents "github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/providers"
	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/events/system"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// AdapterManager manages the lifecycle of adapters
type AdapterManager struct {
	factory         *core.AdapterFactory
	registry        *core.AdapterRegistry
	adapterEventBus *adapterEvents.EventBusImpl
	systemEventBus  *events.EventBus
	eventBridge     *bridge.EventBridge
	logger          observability.Logger
	MetricsClient   observability.MetricsClient
}

// NewAdapterManager creates a new adapter manager
func NewAdapterManager(
	cfg *config.Config,
	systemEventBus system.EventBus,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
) *AdapterManager {
	// Create events bus for adapters
	adapterEventBus := adapterEvents.NewEventBus(logger)

	// Create system event bus adapter
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
	eventBridge := bridge.NewEventBridge(adapterEventBus, systemEventBus, logger, registry)

	// Register providers with the factory
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

// GetGitHubAdapter returns the GitHub adapter
func (m *AdapterManager) GetGitHubAdapter(ctx context.Context) (interface{}, error) {
	return m.registry.GetAdapter(ctx, "github")
}

// GetAdapter returns an adapter by type
func (m *AdapterManager) GetAdapter(ctx context.Context, adapterType string) (interface{}, error) {
	return m.registry.GetAdapter(ctx, adapterType)
}
