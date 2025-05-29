// Package core provides core engine functionality for the MCP application
package core

import (
	"context"
)

// EngineInterface defines the interface for the core engine
type EngineInterface interface {
	// GetAdapter gets an adapter by type
	GetAdapter(adapterType string) (interface{}, error)
	
	// HandleAdapterWebhook handles a webhook event using the appropriate adapter
	HandleAdapterWebhook(ctx context.Context, adapterType string, eventType string, payload []byte) error
	
	// RecordWebhookInContext records a webhook event in a context
	RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error)
	
	// ExecuteAdapterAction executes an action using the appropriate adapter
	ExecuteAdapterAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error)
	
	// Shutdown performs a graceful shutdown of the engine
	Shutdown(ctx context.Context) error
}

// EventBusInterface defines the interface for an event bus
type EventBusInterface interface {
	// Publish publishes an event
	Publish(ctx context.Context, eventType string, data map[string]interface{}) error
	
	// Subscribe subscribes to events
	Subscribe(eventType string, handler func(ctx context.Context, eventType string, data map[string]interface{}) error)
	
	// Unsubscribe unsubscribes from events
	Unsubscribe(eventType string, handler func(ctx context.Context, eventType string, data map[string]interface{}) error)
}

// Ensure Engine implements EngineInterface
var _ EngineInterface = (*Engine)(nil)
