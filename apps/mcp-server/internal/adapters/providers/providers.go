// Package providers manages the registration and initialization of all adapter
// providers for the MCP Server. It centralizes provider registration to ensure
// a consistent approach across different provider implementations.
package providers

import (
	"fmt"

	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/adapters/providers/github"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// RegisterAllProviders registers all adapter providers with the factory.
// Each adapter is registered with the factory to support dynamic instantiation
// based on configuration. This approach follows the factory design pattern to
// decouple adapter creation from usage.
//
// Parameters:
//   - factory: The adapter factory to register providers with
//   - eventBus: The event bus for adapter events
//   - metricsClient: The metrics client for telemetry
//   - logger: The logger for diagnostic information
//
// Returns:
//   - error: If any provider registration fails
func RegisterAllProviders(factory *core.DefaultAdapterFactory, eventBus interface{}, 
	metricsClient observability.MetricsClient, logger observability.Logger) error {
	
	// Register GitHub adapter
	if err := github.RegisterAdapter(factory, eventBus, metricsClient, logger); err != nil {
		return fmt.Errorf("failed to register GitHub adapter: %w", err)
	}
	
	// Register other adapters here
	// example: if err := jfrog.RegisterAdapter(factory, eventBus, metricsClient, logger); err != nil {
	//     return fmt.Errorf("failed to register JFrog adapter: %w", err)
	// }
	
	return nil
}

// GetSupportedProviders returns a list of all supported provider types.
// This is useful for validation and documentation purposes.
func GetSupportedProviders() []string {
	return []string{
		"github",
		// Add other provider types as they are implemented
	}
}
