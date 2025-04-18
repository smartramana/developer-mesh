// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"fmt"
	
	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

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
func RegisterAdapter(factory *core.DefaultAdapterFactory, eventBus *events.EventBus, 
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
		
		// Convert config to GitHub config
		githubConfig, ok := config.(Config)
		if !ok {
			return nil, fmt.Errorf("invalid configuration type for GitHub adapter: %T", config)
		}
		
		// Create adapter
		adapter, err := NewAdapter(githubConfig, eventBus, metricsClient, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub adapter: %w", err)
		}
		
		// Initialize adapter
		err = adapter.Initialize(ctx, githubConfig)
		if err != nil {
			// Clean up if initialization fails
			closeErr := adapter.Close()
			if closeErr != nil {
				logger.Warn("Failed to close adapter after initialization failure", 
					map[string]interface{}{
						"init_error": err.Error(),
						"close_error": closeErr.Error(),
					})
			}
			return nil, fmt.Errorf("failed to initialize GitHub adapter: %w", err)
		}
		
		logger.Info("GitHub adapter registered successfully", map[string]interface{}{
			"adapter_type": adapterType,
			"features": githubConfig.EnabledFeatures,
		})
		
		return adapter, nil
	})
	
	return nil
}
