package system

import (
	"context"
	"time"
)

// Event represents a system event
type Event interface {
	// GetType returns the event type
	GetType() EventType
	
	// GetTimestamp returns the event timestamp
	GetTimestamp() time.Time
}

// EventType defines the type of system event
type EventType string

// System event types
const (
	// Adapter events
	EventTypeAdapterOperationSuccess EventType = "adapter.operation.success"
	EventTypeAdapterOperationFailure EventType = "adapter.operation.failure"
	EventTypeAdapterHealthChanged    EventType = "adapter.health.changed"
	EventTypeWebhookReceived         EventType = "webhook.received"
	EventTypeAdapterGeneric          EventType = "adapter.generic"
	
	// System events
	EventTypeSystemStartup    EventType = "system.startup"
	EventTypeSystemShutdown   EventType = "system.shutdown"
	EventTypeSystemHealthCheck EventType = "system.health.check"
)

// EventBus defines the interface for the system event bus
type EventBus interface {
	// Publish publishes an event
	Publish(ctx context.Context, event Event) error
	
	// Subscribe subscribes to events of a specific type
	Subscribe(eventType EventType, handler func(ctx context.Context, event Event) error)
	
	// Unsubscribe unsubscribes from events of a specific type
	Unsubscribe(eventType EventType, handler func(ctx context.Context, event Event) error)
}

// BaseEvent contains common event fields
type BaseEvent struct {
	Type      EventType
	Timestamp time.Time
}

// GetType returns the event type
func (e *BaseEvent) GetType() EventType {
	return e.Type
}

// GetTimestamp returns the event timestamp
func (e *BaseEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// AdapterOperationSuccessEvent represents a successful adapter operation
type AdapterOperationSuccessEvent struct {
	BaseEvent
	AdapterType string
	Operation   string
	Result      interface{}
	ContextID   string
}

// AdapterOperationFailureEvent represents a failed adapter operation
type AdapterOperationFailureEvent struct {
	BaseEvent
	AdapterType string
	Operation   string
	Error       string
	ContextID   string
}

// WebhookReceivedEvent represents a webhook received event
type WebhookReceivedEvent struct {
	BaseEvent
	AdapterType string
	EventType   string
	Payload     interface{}
	ContextID   string
}

// AdapterHealthChangedEvent represents an adapter health status change
type AdapterHealthChangedEvent struct {
	BaseEvent
	AdapterType string
	OldStatus   string
	NewStatus   string
}

// AdapterGenericEvent represents a generic adapter event
type AdapterGenericEvent struct {
	BaseEvent
	AdapterType string
	EventType   string
	Payload     interface{}
	Metadata    map[string]interface{}
}
