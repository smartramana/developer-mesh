package providers

import (
	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/adapters/providers/github"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// RegisterAllProviders registers all adapter providers with the factory
func RegisterAllProviders(factory *core.DefaultAdapterFactory, eventBus *events.EventBus, metricsClient *observability.MetricsClient, logger *observability.Logger) {
	// Register GitHub adapter
	github.RegisterAdapter(factory, eventBus, metricsClient, logger)
	
	// Register other adapters here
	// example: jfrog.RegisterAdapter(factory, eventBus, metricsClient, logger)
	// example: sonarqube.RegisterAdapter(factory, eventBus, metricsClient, logger)
}
