package core

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// AdapterCreator is a function that creates an adapter
type AdapterCreator func(ctx context.Context, config interface{}) (Adapter, error)

// AdapterFactory is the implementation of the AdapterFactory interface
// This is the core implementation that should be used going forward
type AdapterFactory struct {
	creators      map[string]AdapterCreator
	configs       map[string]interface{}
	metricsClient observability.MetricsClient
	logger        observability.Logger
	mu            sync.RWMutex
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory(
	configs map[string]interface{},
	metricsClient observability.MetricsClient,
	logger observability.Logger,
) *AdapterFactory {
	if logger == nil {
		logger = observability.DefaultLogger
	}
	
	return &AdapterFactory{
		creators:      make(map[string]AdapterCreator),
		configs:       configs,
		metricsClient: metricsClient,
		logger:        logger,
	}
}

// RegisterAdapterCreator registers a creator function for an adapter type
func (f *AdapterFactory) RegisterAdapterCreator(adapterType string, creator AdapterCreator) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.creators[adapterType] = creator
	f.logger.Info("Registered adapter creator", map[string]interface{}{
		"adapterType": adapterType,
	})
}

// CreateAdapter creates an adapter for the given type and configuration
func (f *AdapterFactory) CreateAdapter(ctx context.Context, adapterType string) (Adapter, error) {
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
func (f *AdapterFactory) ListRegisteredAdapterTypes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	types := make([]string, 0, len(f.creators))
	for adapterType := range f.creators {
		types = append(types, adapterType)
	}
	
	return types
}

// SetConfig sets the configuration for an adapter type
func (f *AdapterFactory) SetConfig(adapterType string, config interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.configs[adapterType] = config
}

// GetConfig gets the configuration for an adapter type
func (f *AdapterFactory) GetConfig(adapterType string) (interface{}, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	config, exists := f.configs[adapterType]
	return config, exists
}

// DefaultAdapterFactory is an alias for AdapterFactory to maintain backwards compatibility
// DEPRECATED: Use AdapterFactory directly in new code
type DefaultAdapterFactory struct {
	*AdapterFactory
}

// NewDefaultAdapterFactory creates a new adapter factory using the DefaultAdapterFactory type for backward compatibility
// DEPRECATED: Use NewAdapterFactory in new code
func NewDefaultAdapterFactory(
	configs map[string]interface{},
	metricsClient observability.MetricsClient,
	logger interface{}, // Accept both *observability.Logger and observability.Logger
) *DefaultAdapterFactory {
	// Handle different logger types
	var loggerInterface observability.Logger
	
	// Use type assertions instead of type switch to handle interface values
	if l, ok := logger.(observability.Logger); ok {
		loggerInterface = l
	} else {
		// If no valid Logger provided, use the default logger
		loggerInterface = observability.DefaultLogger
	}
	
	return &DefaultAdapterFactory{
		AdapterFactory: NewAdapterFactory(configs, metricsClient, loggerInterface),
	}
}
