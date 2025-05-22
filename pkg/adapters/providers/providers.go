// Package providers manages the registration and initialization of all adapter
// providers for the MCP Server. It centralizes provider registration to ensure
// a consistent approach across different provider implementations.
package providers

import (
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/providers/github"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// RegisterAllProviders registers all adapter providers with the factory.
// Each adapter is registered with the factory to support dynamic instantiation
// based on configuration. This approach follows the factory design pattern to
// decouple adapter creation from usage.
//
// Parameters:
//   - factory: The adapter factory (can be either *core.AdapterFactory or *core.DefaultAdapterFactory)
//   - eventBus: The event bus for adapter events
//   - metricsClient: The metrics client for telemetry
//   - logger: The logger for diagnostic information
//
// Returns:
//   - error: If any provider registration fails
func RegisterAllProviders(factory interface{}, eventBus interface{}, 
	metricsClient observability.MetricsClient, logger interface{}) error {
	
	// Handle logger interface conversion safely
	// We need to use a generic approach since *observability.Logger might not implement observability.Logger
	loggerInterface, ok := logger.(observability.Logger)
	if !ok {
		// If direct conversion fails, try to use default logger
		loggerInterface = observability.DefaultLogger
	}
	
	// Register GitHub adapter with appropriate factory type handling
	if err := github.RegisterAdapter(factory, eventBus, metricsClient, loggerInterface); err != nil {
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
