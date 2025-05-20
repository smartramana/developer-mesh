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

// IAdapterFactory defines the interface for adapter factories
type IAdapterFactory interface {
	// CreateAdapter creates an adapter for the given type and configuration
	CreateAdapter(ctx context.Context, adapterType string) (Adapter, error)
	
	// ListRegisteredAdapterTypes returns a list of registered adapter types
	ListRegisteredAdapterTypes() []string
	
	// SetConfig sets the configuration for an adapter type
	SetConfig(adapterType string, config interface{})
	
	// GetConfig gets the configuration for an adapter type
	GetConfig(adapterType string) (interface{}, bool)
	
	// RegisterAdapterCreator registers a creator function for an adapter type
	RegisterAdapterCreator(adapterType string, creator AdapterCreator)
}
