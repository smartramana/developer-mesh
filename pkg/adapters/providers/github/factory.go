// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"fmt"

	adapterEvents "github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	githubAdapter "github.com/S-Corkum/devops-mcp/pkg/adapters/github"
	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

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
func RegisterAdapter(factory interface{}, eventBus interface{},
	metricsClient observability.MetricsClient, logger observability.Logger) error {

	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	if logger == nil {
		return fmt.Errorf("logger cannot be nil")
	}
	
	// Check if factory implements the required interface
	adapterFactory, ok := factory.(interface{
		RegisterAdapterCreator(string, func(context.Context, interface{}) (interface{}, error))
	})
	if !ok {
		return fmt.Errorf("factory does not implement required interface")
	}

	adapterFactory.RegisterAdapterCreator(adapterType, func(ctx context.Context, config interface{}) (interface{}, error) {


		// Get default config
		githubConfig := githubAdapter.DefaultConfig()

		// Convert config map to GitHub config
		if configMap, ok := config.(map[string]interface{}); ok {
			// Apply authentication settings
			if token, ok := configMap["token"].(string); ok {
				githubConfig.Auth.Token = token
				githubConfig.Auth.Type = "token"
			}

			// Handle GitHub App authentication
			if _, ok := configMap["app_id"].(string); ok {
				// For simplicity, we'll just set a placeholder value
				githubConfig.Auth.AppID = 1
				githubConfig.Auth.Type = "app"
			}

			if privateKey, ok := configMap["private_key"].(string); ok {
				githubConfig.Auth.PrivateKey = privateKey
			}

			if _, ok := configMap["installation_id"].(string); ok {
				// For simplicity, we'll just set a placeholder value
				githubConfig.Auth.InstallationID = 1
			}
		
			// Apply API settings
			if baseURL, ok := configMap["base_url"].(string); ok {
				githubConfig.BaseURL = baseURL
			}
		}

		// Assert eventBus to events.EventBusIface
		typedEventBus, ok := eventBus.(events.EventBusIface)
		if !ok {
			return nil, fmt.Errorf("eventBus does not implement events.EventBusIface")
		}
		
		// Create adapter for the event bus
		eventBusAdapter := adapterEvents.NewEventBusAdapter(typedEventBus)
		
		// Create GitHub adapter with the adapter
		adapter, err := githubAdapter.New(githubConfig, logger, metricsClient, eventBusAdapter)
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
