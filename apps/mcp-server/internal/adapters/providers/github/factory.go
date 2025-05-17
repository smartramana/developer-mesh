// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/internal/adapters/core"
	githubAdapter "github.com/S-Corkum/devops-mcp/internal/adapters/github"
	"github.com/S-Corkum/devops-mcp/internal/events"
	"github.com/S-Corkum/devops-mcp/internal/observability"
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
func RegisterAdapter(factory *core.DefaultAdapterFactory, eventBus interface{},
	metricsClient observability.MetricsClient, logger *observability.Logger) error {

	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	if logger == nil {
		return fmt.Errorf("logger cannot be nil")
	}

	factory.RegisterAdapterCreator(adapterType, func(ctx context.Context, config interface{}) (core.Adapter, error) {


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

		// Assert eventBus to events.EventBusIface
		typedEventBus, ok := eventBus.(events.EventBusIface)
		if !ok {
			return nil, fmt.Errorf("eventBus does not implement events.EventBusIface")
		}
		adapter, err := githubAdapter.New(githubConfig, logger, metricsClient, typedEventBus)
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
