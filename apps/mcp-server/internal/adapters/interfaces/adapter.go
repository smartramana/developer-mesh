// Package interfaces provides adapter interfaces for the MCP application
package interfaces

import (
	"context"
)

// AdapterManager defines the interface for managing adapters
type AdapterManager interface {
	// Initialize initializes all required adapters
	Initialize(ctx context.Context) error
	
	// GetAdapter gets an adapter by type
	GetAdapter(adapterType string) (interface{}, error)
	
	// ExecuteAction executes an action with an adapter
	ExecuteAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error)
	
	// Shutdown gracefully shuts down all adapters
	Shutdown(ctx context.Context) error
}

// WebhookHandler defines the interface for handling webhooks
type WebhookHandler interface {
	// HandleWebhook handles a webhook event
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error
}
