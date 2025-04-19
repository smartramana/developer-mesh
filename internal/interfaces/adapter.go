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
	
	// ExecuteAction executes an action with an adapter and records it in context
	ExecuteAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error)
	
	// HandleWebhook handles a webhook event
	HandleWebhook(ctx context.Context, adapterType string, eventType string, payload []byte) error
	
	// RecordWebhookInContext records a webhook event in a context
	RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error)
	
	// Shutdown gracefully shuts down all adapters
	Shutdown(ctx context.Context) error
}


