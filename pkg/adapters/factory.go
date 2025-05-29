package adapters

import (
	"context"
	"fmt"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Factory creates adapters based on provider type
type Factory struct {
	providers map[string]ProviderFunc
	configs   map[string]Config
	logger    observability.Logger
	mu        sync.RWMutex
}

// ProviderFunc is a function that creates an adapter
type ProviderFunc func(ctx context.Context, config Config, logger observability.Logger) (SourceControlAdapter, error)

// NewFactory creates a new adapter factory
func NewFactory(logger observability.Logger) *Factory {
	return &Factory{
		providers: make(map[string]ProviderFunc),
		configs:   make(map[string]Config),
		logger:    logger,
	}
}

// RegisterProvider registers a provider creation function
func (f *Factory) RegisterProvider(name string, provider ProviderFunc) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	f.providers[name] = provider
	f.logger.Info("registered adapter provider", map[string]any{
		"provider": name,
	})
	return nil
}

// SetConfig sets configuration for a provider
func (f *Factory) SetConfig(provider string, config Config) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.configs[provider] = config
}

// CreateAdapter creates an adapter for the specified provider
func (f *Factory) CreateAdapter(ctx context.Context, provider string) (SourceControlAdapter, error) {
	f.mu.RLock()
	providerFunc, exists := f.providers[provider]
	config := f.configs[provider]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	adapter, err := providerFunc(ctx, config, f.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s adapter: %w", provider, err)
	}

	f.logger.Info("created adapter", map[string]any{
		"provider": provider,
	})

	return adapter, nil
}

// ListProviders returns a list of registered providers
func (f *Factory) ListProviders() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	providers := make([]string, 0, len(f.providers))
	for name := range f.providers {
		providers = append(providers, name)
	}
	return providers
}
