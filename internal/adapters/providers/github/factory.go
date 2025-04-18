package github

import (
	"context"
	"fmt"
	
	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// RegisterAdapter registers the GitHub adapter with the factory
func RegisterAdapter(factory *core.DefaultAdapterFactory, eventBus *events.EventBus, metricsClient *observability.MetricsClient, logger *observability.Logger) {
	factory.RegisterAdapterCreator("github", func(ctx context.Context, config interface{}) (core.Adapter, error) {
		// Convert config to GitHub config
		githubConfig, ok := config.(Config)
		if !ok {
			return nil, fmt.Errorf("invalid configuration type for GitHub adapter")
		}
		
		// Create adapter
		adapter, err := NewAdapter(githubConfig, eventBus, metricsClient, logger)
		if err != nil {
			return nil, err
		}
		
		// Initialize adapter
		err = adapter.Initialize(ctx, githubConfig)
		if err != nil {
			return nil, err
		}
		
		return adapter, nil
	})
}
