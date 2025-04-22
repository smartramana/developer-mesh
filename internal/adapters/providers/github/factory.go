// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"fmt"
	
	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	githubAdapter "github.com/S-Corkum/mcp-server/internal/adapters/github"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// Use a type alias for interface{} to make it clear this is a dummy bus
type dummyEventBus interface{}

// adapterType is the unique identifier for the GitHub adapter
const adapterType = "github"

// RegisterAdapter registers the GitHub adapter with the factory.
// This function registers a creator function that will be called
// when a GitHub adapter needs to be instantiated.
//
// Parameters:
//   - factory: The adapter factory to register with
//   - eventBus: The event bus for adapter events
//   - metricsClient: The metrics client for telemetry
//   - logger: The logger for diagnostic information
//
// Returns:
//   - error: If registration fails
func RegisterAdapter(factory *core.DefaultAdapterFactory, eventBus interface{}, 
	metricsClient *observability.MetricsClient, logger *observability.Logger) error {
	
	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}
	
	if logger == nil {
		return fmt.Errorf("logger cannot be nil")
	}
	
	factory.RegisterAdapterCreator(adapterType, func(ctx context.Context, config interface{}) (core.Adapter, error) {
		// Validate context
		if ctx == nil {
			ctx = context.Background()
		}
		
		// Get default config
		githubConfig := githubAdapter.DefaultConfig()
		
		// Convert config map to GitHub config
		if configMap, ok := config.(map[string]interface{}); ok {
			// Apply config values if available
			if token, ok := configMap["token"].(string); ok {
				githubConfig.Token = token
			}
			
			if baseURL, ok := configMap["base_url"].(string); ok {
				githubConfig.BaseURL = baseURL
			}
		}
		
		// Create adapter
		// Pass the eventBus directly as an interface{} and let the adapter handle it
		adapter, err := githubAdapter.New(githubConfig, logger, metricsClient, eventBus)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub adapter: %w", err)
		}
		
		logger.Info("GitHub adapter registered successfully", map[string]interface{}{
			"adapter_type": adapterType,
		})
		
		return adapter, nil
	})
	
	return nil
}
