// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// MockAdapterFactory is a mock implementation of the AdapterFactory interface
type MockAdapterFactory struct {
	mock.Mock
}

// RegisterAdapterCreator mocks the registration of an adapter creator
func (m *MockAdapterFactory) RegisterAdapterCreator(adapterType string, creator core.AdapterCreator) {
	m.Called(adapterType, creator)
}

// Create mocks the creation of an adapter
func (m *MockAdapterFactory) Create(ctx context.Context, adapterType string, config interface{}) (core.Adapter, error) {
	args := m.Called(ctx, adapterType, config)
	if adapter, ok := args.Get(0).(core.Adapter); ok {
		return adapter, args.Error(1)
	}
	return nil, args.Error(1)
}

// TestRegisterProvider tests the provider registration process
func TestRegisterProvider(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		// Create mock factory and dependencies
		mockFactory := new(MockAdapterFactory)
		eventBus := NewTestEventBus()
		eventBus.On("Subscribe", mock.Anything, mock.Anything).Return()
		
		metricsClient := observability.NewMetricsClient()
		logger := observability.NewLogger("test-factory")

		// Set up expectations for the mock
		mockFactory.On("RegisterAdapterCreator", "github", mock.AnythingOfType("func(context.Context, interface{}) (core.Adapter, error)")).Return()

		// Create provider
		provider := NewProvider(logger, metricsClient, eventBus)
		
		// Register the provider with the factory
		err := provider.Register(mockFactory)
		
		// Verify the result
		require.NoError(t, err, "Register should succeed with valid dependencies")
		
		// Verify the expectations were met
		mockFactory.AssertExpectations(t)
	})
	
	t.Run("nil factory", func(t *testing.T) {
		// Create dependencies except factory
		eventBus := NewTestEventBus()
		metricsClient := observability.NewMetricsClient()
		logger := observability.NewLogger("test-factory")
		
		// Create provider
		provider := NewProvider(logger, metricsClient, eventBus)
		
		// Try to register with nil factory
		err := provider.Register(nil)
		
		// Verify error
		assert.Error(t, err, "Register should fail with nil factory")
		assert.Contains(t, err.Error(), "factory cannot be nil", "Error should mention nil factory")
	})
}

// Default timeout for tests
var defaultTestTimeout = 5 * time.Second
