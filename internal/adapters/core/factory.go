package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// AdapterCreator is a function that creates an adapter
type AdapterCreator func(ctx context.Context, config interface{}) (Adapter, error)

// AdapterFactory defines the interface for creating adapters
type AdapterFactory interface {
	// CreateAdapter creates an adapter by type
	CreateAdapter(ctx context.Context, adapterType string) (Adapter, error)
	
	// RegisterAdapterCreator registers a function to create adapters of a specific type
	RegisterAdapterCreator(adapterType string, creator AdapterCreator)
}

// DefaultAdapterFactory is the default implementation of AdapterFactory
type DefaultAdapterFactory struct {
	configs          map[string]interface{}
	adapterCreators  map[string]AdapterCreator
	metricsClient    *observability.MetricsClient
	circuitBreakers  *resilience.CircuitBreakerManager
	rateLimiters     *resilience.RateLimiterManager
	mu               sync.RWMutex
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory(
	configs map[string]interface{},
	metricsClient *observability.MetricsClient,
	circuitBreakers *resilience.CircuitBreakerManager,
	rateLimiters *resilience.RateLimiterManager,
) *DefaultAdapterFactory {
	factory := &DefaultAdapterFactory{
		configs:         configs,
		adapterCreators: make(map[string]AdapterCreator),
		metricsClient:   metricsClient,
		circuitBreakers: circuitBreakers,
		rateLimiters:    rateLimiters,
	}
	
	// Default creators will be registered by specific provider packages
	
	return factory
}

// RegisterAdapterCreator registers an adapter creator
func (f *DefaultAdapterFactory) RegisterAdapterCreator(adapterType string, creator AdapterCreator) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.adapterCreators[adapterType] = creator
}

// CreateAdapter creates an adapter by type
func (f *DefaultAdapterFactory) CreateAdapter(ctx context.Context, adapterType string) (Adapter, error) {
	// Get configuration for the adapter type
	f.mu.RLock()
	config, ok := f.configs[adapterType]
	if !ok {
		f.mu.RUnlock()
		return nil, fmt.Errorf("configuration not found for adapter type: %s", adapterType)
	}
	
	// Get creator for the adapter type
	creator, ok := f.adapterCreators[adapterType]
	f.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("unsupported adapter type: %s", adapterType)
	}
	
	// Create the adapter
	adapter, err := creator(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter %s: %v", adapterType, err)
	}
	
	return adapter, nil
}
