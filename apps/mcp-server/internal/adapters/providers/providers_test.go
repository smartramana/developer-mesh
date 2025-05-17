// Package providers manages the registration and initialization of all adapter
// providers for the MCP Server.
package providers

import (
	"context"
	"testing"

	"go.uber.org/goleak"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/devops-mcp/internal/adapters/core"
	"github.com/S-Corkum/devops-mcp/internal/adapters/events"
	"github.com/S-Corkum/devops-mcp/internal/adapters/providers/github"
	"github.com/S-Corkum/devops-mcp/internal/observability"
)

// TestRegisterAllProviders tests that all providers can be registered successfully
func TestRegisterAllProviders(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Create dependencies
	configs := make(map[string]interface{})
	metricsClient := observability.NewMetricsClient()
	logger := observability.NewLogger("providers_test")
	factory := core.NewAdapterFactory(configs, metricsClient, logger)
	eventBus := events.NewEventBus(logger)
defer eventBus.Close()

	// Register all providers
	err := RegisterAllProviders(factory, eventBus, metricsClient, logger)
	require.NoError(t, err, "Provider registration should succeed")

	// Test GitHub adapter registration
	t.Run("github adapter registration", func(t *testing.T) {
		// Create a valid GitHub config
		config := github.DefaultConfig()
		config.Token = "test-token"
		config.DefaultOwner = "test-owner"
		config.DefaultRepo = "test-repo"

		// Set the config in the factory
		factory.SetConfig("github", config)

		// Attempt to create adapter
		adapter, err := factory.CreateAdapter(context.Background(), "github")
		
		// Verify adapter creation
		require.NoError(t, err, "GitHub adapter creation should succeed")
		require.NotNil(t, adapter, "Adapter should not be nil")
		defer adapter.Close() // Ensure all background workers are shut down
		assert.Equal(t, "github", adapter.Type(), "Adapter type should be 'github'")
	})

	// Test unregistered adapter type
	t.Run("unregistered adapter type", func(t *testing.T) {
		// Attempt to create adapter with unregistered type
		adapter, err := factory.CreateAdapter(context.Background(), "nonexistent")
		
		// Verify adapter creation fails
		assert.Error(t, err, "Creating unregistered adapter should fail")
		assert.Nil(t, adapter, "Adapter should be nil for unregistered type")
		assert.Contains(t, err.Error(), "no creator registered", "Error should indicate unknown adapter type")
	})
}

// TestProviderCompleteness verifies that all expected providers are registered
func TestProviderCompleteness(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Create dependencies
	configs := make(map[string]interface{})
	metricsClient := observability.NewMetricsClient()
	logger := observability.NewLogger("providers_test")
	factory := core.NewAdapterFactory(configs, metricsClient, logger)
	eventBus := events.NewEventBus(logger)
defer eventBus.Close()

	// Register all providers
	err := RegisterAllProviders(factory, eventBus, metricsClient, logger)
	require.NoError(t, err, "Provider registration should succeed")

	// Get the list of supported providers
	supportedProviders := GetSupportedProviders()
	require.NotEmpty(t, supportedProviders, "Should have at least one supported provider")
	
	// Verify each supported provider is actually registered
	for _, providerType := range supportedProviders {
		t.Run("provider registration: "+providerType, func(t *testing.T) {
			// Create a minimal config for testing registration
			var config interface{}
			
			// Use appropriate config based on provider type
			switch providerType {
			case "github":
				config = github.DefaultConfig()
				githubConfig := config.(github.Config)
				githubConfig.Token = "test-token"
				githubConfig.DefaultOwner = "test-owner"
				githubConfig.DefaultRepo = "test-repo"
				config = githubConfig
			default:
				t.Fatalf("Test case not implemented for provider type: %s", providerType)
				return
			}
			
			// Set the config in the factory
			factory.SetConfig(providerType, config)
			
			// Verify provider is registered by attempting to create it
			adapter, err := factory.CreateAdapter(context.Background(), providerType)
			require.NoError(t, err, "Should be able to create %s adapter", providerType)
			require.NotNil(t, adapter, "Adapter should not be nil")
			defer adapter.Close() // Ensure all background workers are shut down
			assert.Equal(t, providerType, adapter.Type(), "Adapter type should match requested type")
		})
	}
}

// TestGetSupportedProviders verifies that the GetSupportedProviders function
// returns all expected provider types
func TestGetSupportedProviders(t *testing.T) {
	defer goleak.VerifyNone(t)
	providers := GetSupportedProviders()
	
	// Should at least contain GitHub
	assert.Contains(t, providers, "github", "Supported providers should include GitHub")
	
	// Should have no duplicates
	providerSet := make(map[string]struct{})
	for _, p := range providers {
		_, exists := providerSet[p]
		assert.False(t, exists, "Provider %s should not be duplicated", p)
		providerSet[p] = struct{}{}
	}
}
