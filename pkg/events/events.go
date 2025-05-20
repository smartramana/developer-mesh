package events

import (
	"context"
)

// EventType defines the type of event
type EventType string

// Event represents a system event
type Event interface {
	// GetType returns the event type
	GetType() string
}

// Handler is a function that processes an event
type Handler func(ctx context.Context, event *Event) error

// EventBusIface is the interface for event bus implementations
type EventBus interface {
	// Publish publishes an event
	Publish(ctx context.Context, event *Event)

	// Subscribe subscribes to events of a specific type
	Subscribe(eventType EventType, handler Handler)

	// Unsubscribe unsubscribes from events of a specific type
	Unsubscribe(eventType EventType, handler Handler)

	// Close closes the event bus
	Close()
}
