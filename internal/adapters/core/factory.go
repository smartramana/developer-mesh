package core

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// AdapterCreator is a function that creates an adapter
type AdapterCreator func(ctx context.Context, config interface{}) (Adapter, error)

// DefaultAdapterFactory is the default implementation of AdapterFactory
type DefaultAdapterFactory struct {
	creators      map[string]AdapterCreator
	configs       map[string]interface{}
	metricsClient observability.MetricsClient
	logger        *observability.Logger
	mu            sync.RWMutex
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory(
	configs map[string]interface{},
	metricsClient observability.MetricsClient,
	logger *observability.Logger,
) *DefaultAdapterFactory {
	if logger == nil {
		logger = observability.NewLogger("adapter_factory")
	}
	
	return &DefaultAdapterFactory{
		creators:      make(map[string]AdapterCreator),
		configs:       configs,
		metricsClient: metricsClient,
		logger:        logger,
	}
}

// RegisterAdapterCreator registers a creator function for an adapter type
func (f *DefaultAdapterFactory) RegisterAdapterCreator(adapterType string, creator AdapterCreator) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.creators[adapterType] = creator
	f.logger.Info("Registered adapter creator", map[string]interface{}{
		"adapterType": adapterType,
	})
}

// CreateAdapter creates an adapter for the given type and configuration
func (f *DefaultAdapterFactory) CreateAdapter(ctx context.Context, adapterType string) (Adapter, error) {
	f.mu.RLock()
	creator, exists := f.creators[adapterType]
	config := f.configs[adapterType]
	f.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no creator registered for adapter type: %s", adapterType)
	}
	
	return creator(ctx, config)
}

// ListRegisteredAdapterTypes returns a list of registered adapter types
func (f *DefaultAdapterFactory) ListRegisteredAdapterTypes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	types := make([]string, 0, len(f.creators))
	for adapterType := range f.creators {
		types = append(types, adapterType)
	}
	
	return types
}

// SetConfig sets the configuration for an adapter type
func (f *DefaultAdapterFactory) SetConfig(adapterType string, config interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.configs[adapterType] = config
}

// GetConfig gets the configuration for an adapter type
func (f *DefaultAdapterFactory) GetConfig(adapterType string) (interface{}, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	config, exists := f.configs[adapterType]
	return config, exists
}
