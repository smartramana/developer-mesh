package core

import (
	"context"
)

// Adapter defines the core interface for all adapters
type Adapter interface {
	// Type returns the adapter type identifier
	Type() string
	
	// ExecuteAction executes an action with context awareness
	ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error)
	
	// HandleWebhook processes webhook events from the external service
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error

	// Health returns the health status of the adapter
	Health() string

	// Close gracefully shuts down the adapter
	Close() error
	
	// Version returns the adapter's version
	Version() string
}

// AdapterFactory creates adapters
type AdapterFactory interface {
	// CreateAdapter creates an adapter for the given type and configuration
	CreateAdapter(ctx context.Context, adapterType string) (Adapter, error)
}
