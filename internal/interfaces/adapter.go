package interfaces

import (
	"context"
)

// Adapter defines the interface for all external service adapters
type Adapter interface {
	// Initialize sets up the adapter with configuration
	Initialize(ctx context.Context, config interface{}) error

	// GetData retrieves data from the external service
	GetData(ctx context.Context, query interface{}) (interface{}, error)
	
	// ExecuteAction executes an action with context awareness
	ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error)
	
	// HandleWebhook processes webhook events from the external service
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error
	
	// Subscribe registers a callback for a specific event type
	Subscribe(eventType string, callback func(interface{})) error
	
	// IsSafeOperation determines if an operation is safe to perform
	IsSafeOperation(operation string, params map[string]interface{}) (bool, error)

	// Health returns the health status of the adapter
	Health() string

	// Close gracefully shuts down the adapter
	Close() error
}

// WebhookHandler is an interface for adapters that can handle webhooks
type WebhookHandler interface {
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error
}
