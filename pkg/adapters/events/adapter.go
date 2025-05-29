// Package events provides adapter functionality for connecting to the system event bus.
package events

import (
	"context"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// AdapterEventType defines standard event types for adapters
type AdapterEventType string

// Standard adapter event types
const (
	EventTypeOperationSuccess     AdapterEventType = "operation.success"
	EventTypeOperationFailure     AdapterEventType = "operation.failure"
	EventTypeWebhookReceived      AdapterEventType = "webhook.received"
	EventTypeAdapterHealthChanged AdapterEventType = "adapter.health.changed"
	EventTypeRepositoryEvent      AdapterEventType = "repository.event"
)

// AdapterEvent represents an event from an adapter
type AdapterEvent struct {
	ID          string           // Unique event ID
	AdapterType string           // Type of adapter that emitted the event
	EventType   AdapterEventType // Type of event
	Payload     any              // Event payload
	Timestamp   time.Time        // Time when the event occurred
	Metadata    map[string]any   // Additional metadata
}

// NewAdapterEvent creates a new adapter event
func NewAdapterEvent(adapterType string, eventType AdapterEventType, payload any) *AdapterEvent {
	return &AdapterEvent{
		ID:          "event-" + time.Now().Format(time.RFC3339),
		AdapterType: adapterType,
		EventType:   eventType,
		Payload:     payload,
		Timestamp:   time.Now(),
		Metadata:    make(map[string]any),
	}
}

// WithMetadata adds metadata to an event
func (e *AdapterEvent) WithMetadata(key string, value any) *AdapterEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// ToModelEvent converts an adapter event to a models.Event for use with the standard event bus
func (e *AdapterEvent) ToModelEvent() *models.Event {
	return &models.Event{
		Type:      string(e.EventType),
		Data:      e.Payload,
		Timestamp: e.Timestamp,
		Source:    e.AdapterType,
	}
}

// EventListener is the interface for adapter event listeners
type EventListener interface {
	// Handle handles an adapter event
	Handle(ctx context.Context, event *AdapterEvent) error
}

// EventHandler is a function that processes an event
type EventHandler func(ctx context.Context, event *AdapterEvent) error

// EventBus is the interface for adapter event bus implementations
type EventBus interface {
	// Emit emits an event to the event bus
	Emit(ctx context.Context, event *AdapterEvent) error

	// Subscribe subscribes to events of a specific type
	Subscribe(eventType AdapterEventType, handler EventHandler)

	// SubscribeAll subscribes to all events
	SubscribeAll(handler EventHandler)

	// Unsubscribe unsubscribes from events of a specific type
	Unsubscribe(eventType AdapterEventType, handler EventHandler)

	// UnsubscribeAll unsubscribes from all events
	UnsubscribeAll(handler EventHandler)

	// Close closes the event bus
	Close()
}
