package adapters

import (
	"context"
	"fmt"
	
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Manager manages the lifecycle of adapters
type Manager struct {
	factory *Factory
	logger  observability.Logger
}

// NewManager creates a new adapter manager
func NewManager(logger observability.Logger) *Manager {
	factory := NewFactory(logger)
	
	// Adapters should be registered externally to avoid import cycles
	// Example usage:
	// manager := adapters.NewManager(logger)
	// github.Register(manager.GetFactory())
	
	return &Manager{
		factory: factory,
		logger:  logger,
	}
}

// GetFactory returns the adapter factory for external registration
func (m *Manager) GetFactory() *Factory {
	return m.factory
}

// SetConfig sets configuration for a specific adapter
func (m *Manager) SetConfig(provider string, config Config) {
	m.factory.SetConfig(provider, config)
}

// GetAdapter returns an adapter for the specified provider
func (m *Manager) GetAdapter(ctx context.Context, provider string) (SourceControlAdapter, error) {
	adapter, err := m.factory.CreateAdapter(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get adapter: %w", err)
	}
	
	// Perform health check
	if err := adapter.Health(ctx); err != nil {
		return nil, fmt.Errorf("adapter health check failed: %w", err)
	}
	
	return adapter, nil
}

// ListProviders returns a list of available providers
func (m *Manager) ListProviders() []string {
	return m.factory.ListProviders()
}