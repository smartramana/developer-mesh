package adapters

import (
	"context"
	"fmt"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/common/events/system"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// AdapterConfig holds configuration needed for adapter initialization
type AdapterConfig struct {
	Adapters map[string]any
}

// AdapterManager manages the lifecycle of adapters
type AdapterManager struct {
	adapters      map[string]Adapter
	factory       *Factory
	logger        observability.Logger
	MetricsClient observability.MetricsClient
	mu            sync.RWMutex
}

// NewAdapterManager creates a new adapter manager
func NewAdapterManager(
	cfg *AdapterConfig,
	_ any, // Formerly contextManager, kept for backward compatibility
	systemEventBus system.EventBus,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
) *AdapterManager {
	if logger == nil {
		logger = observability.NewLogger("adapter_manager")
	}

	manager := &AdapterManager{
		adapters:      make(map[string]Adapter),
		factory:       NewFactory(logger),
		logger:        logger,
		MetricsClient: metricsClient,
	}

	return manager
}

// Initialize initializes all required adapters
func (m *AdapterManager) Initialize(ctx context.Context) error {
	m.logger.Info("Initializing adapter manager", nil)
	// Adapters are initialized on-demand
	return nil
}

// GetAdapter gets an adapter by type
func (m *AdapterManager) GetAdapter(adapterType string) (any, error) {
	ctx := context.Background()
	m.mu.RLock()
	adapter, exists := m.adapters[adapterType]
	m.mu.RUnlock()

	if exists {
		return adapter, nil
	}

	// Try to create the adapter
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if adapter, exists := m.adapters[adapterType]; exists {
		return adapter, nil
	}

	// Create new adapter using factory
	sourceAdapter, err := m.factory.CreateAdapter(ctx, adapterType)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter %s: %w", adapterType, err)
	}

	// Wrap in generic adapter
	genericAdapter := NewGenericAdapter(sourceAdapter, adapterType, "1.0.0")
	m.adapters[adapterType] = genericAdapter
	return genericAdapter, nil
}

// ExecuteAction executes an action with an adapter
func (m *AdapterManager) ExecuteAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]any) (any, error) {
	adapterInterface, err := m.GetAdapter(adapterType)
	if err != nil {
		return nil, err
	}

	adapter, ok := adapterInterface.(Adapter)
	if !ok {
		return nil, fmt.Errorf("invalid adapter type for %s", adapterType)
	}

	return adapter.ExecuteAction(ctx, action, params)
}

// Close releases all adapter resources
func (m *AdapterManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, adapter := range m.adapters {
		if err := adapter.Close(); err != nil {
			m.logger.Error("Failed to close adapter", map[string]any{
				"adapter": name,
				"error":   err.Error(),
			})
		}
	}

	m.adapters = make(map[string]Adapter)
	m.logger.Info("Closed all adapters", nil)
}

// Shutdown gracefully shuts down all adapters
func (m *AdapterManager) Shutdown(ctx context.Context) error {
	m.Close()
	return nil
}
