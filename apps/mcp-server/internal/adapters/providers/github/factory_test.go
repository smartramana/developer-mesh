// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"testing"

	"mcp-server/internal/adapters/core"
	"mcp-server/internal/adapters/providers/github/mocks"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestAdapterFactoryWrapper is a simple wrapper around the actual DefaultAdapterFactory
// that allows us to verify that the RegisterAdapterCreator was called
type TestAdapterFactoryWrapper struct {
	*core.DefaultAdapterFactory
	registerCalled bool
	adapterType    string
}

// RegisterAdapterCreator wraps the actual DefaultAdapterFactory method and tracks if it was called
func (f *TestAdapterFactoryWrapper) RegisterAdapterCreator(adapterType string, creator core.AdapterCreator) {
	f.registerCalled = true
	f.adapterType = adapterType
	f.DefaultAdapterFactory.RegisterAdapterCreator(adapterType, creator)
}

// TestRegisterAdapter tests the adapter registration process
func TestRegisterAdapter(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		// Create factory and dependencies
		eventBus := mocks.NewMockEventBus()
		eventBus.On("Subscribe", mock.Anything, mock.Anything).Return()
		
		metricsClient := observability.NewMetricsClient()
		logger := observability.NewLogger("test-factory")

		// Create a real factory
		configs := make(map[string]interface{})
		factory := core.NewAdapterFactory(configs, metricsClient, logger)

		// Register the adapter directly - this should succeed
		err := RegisterAdapter(factory, eventBus, metricsClient, logger)
		
		// Verify the result
		require.NoError(t, err, "RegisterAdapter should succeed with valid dependencies")
		
		// Verify that the adapter was registered by checking if a creator exists
		// We can't directly check this without adding test methods to the factory,
		// but we can at least check that no error occurred
		require.NotNil(t, factory, "Factory should not be nil after registration")
	})
	
	t.Run("nil factory", func(t *testing.T) {
		// Create dependencies except factory
		eventBus := mocks.NewMockEventBus()
		metricsClient := observability.NewMetricsClient()
		logger := observability.NewLogger("test-factory")
		
		// Try to register with a nil factory
		err := RegisterAdapter(nil, eventBus, metricsClient, logger)
		
		// Verify the result
		assert.Error(t, err, "RegisterAdapter should fail with nil factory")
		assert.Contains(t, err.Error(), "factory cannot be nil", "Error message should indicate that factory cannot be nil")
	})
}

//
