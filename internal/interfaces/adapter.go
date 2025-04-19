package interfaces

import (
	"context"
)

// AdapterManager defines the interface for managing adapters
type AdapterManager interface {
	// Initialize initializes all required adapters
	Initialize(ctx context.Context) error
	
	// GetAdapter gets an adapter by type
	GetAdapter(ctx context.Context, adapterType string) (interface{}, error)
	
	// ExecuteAction executes an action with an adapter
	ExecuteAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error)
	
	// Shutdown gracefully shuts down all adapters
	Shutdown(ctx context.Context) error
}


