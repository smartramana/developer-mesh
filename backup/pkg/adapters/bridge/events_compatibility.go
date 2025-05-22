package bridge

import (
	"context"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	"github.com/S-Corkum/devops-mcp/pkg/events/system"
)

// This file provides compatibility types and functions for the event bridge tests
// to ensure smooth migration from the old event types to the new ones.

// EventType is a compatibility alias for events.LegacyEventType
type EventType string

// Define event types for backward compatibility
const (
	EventTypeOperationSuccess  EventType = "operation.success"
	EventTypeOperationFailure  EventType = "operation.failure"
	EventTypeWebhookReceived   EventType = "webhook.received"
	EventTypeHealthChange      EventType = "health.change"
	EventTypeConfigurationChange EventType = "configuration.change"
)

// AdapterEvent represents a compatibility structure for the legacy adapter event
type AdapterEvent struct {
	ID          string                 // Unique event ID
	AdapterType string                 // Type of adapter that emitted the event
	EventType   EventType              // Type of event
	Payload     interface{}            // Event payload
	Timestamp   time.Time              // Time when the event occurred
	Metadata    map[string]interface{} // Additional metadata
}

// EventListener is a compatibility interface for event handlers
type EventListener interface {
	Handle(ctx context.Context, event *AdapterEvent) error
}

// EventBus represents the compatibility interface for the legacy event bus
type EventBus interface {
	Emit(ctx context.Context, event *AdapterEvent) error
	EmitWithCallback(ctx context.Context, event *AdapterEvent, callback func(error)) error
	SubscribeListener(eventType EventType, listener EventListener)
	SubscribeAll(listener EventListener)
	UnsubscribeListener(eventType EventType, listener EventListener)
	UnsubscribeAll(listener EventListener)
}

// NewAdapterEvent creates a new adapter event for backward compatibility
func NewAdapterEvent(adapterType string, eventType EventType, payload interface{}) *AdapterEvent {
	return &AdapterEvent{
		ID:          "event-" + time.Now().Format(time.RFC3339),
		AdapterType: adapterType,
		EventType:   eventType,
		Payload:     payload,
		Timestamp:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}
}

// WithMetadata adds metadata to an event
func (e *AdapterEvent) WithMetadata(key string, value interface{}) *AdapterEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// Convert adapts a legacy AdapterEvent to the new LegacyAdapterEvent format
func (e *AdapterEvent) Convert() *events.LegacyAdapterEvent {
	return &events.LegacyAdapterEvent{
		AdapterType: e.AdapterType,
		EventType:   events.LegacyEventType(e.EventType),
		Payload:     e.Payload,
		Timestamp:   e.Timestamp.Unix(),
		Metadata:    e.Metadata,
	}
}

// mapEventType maps between EventType and system.EventType
func mapEventType(eventType EventType) system.EventType {
	switch eventType {
	case EventTypeOperationSuccess:
		return system.EventTypeAdapterOperationSuccess
	case EventTypeOperationFailure:
		return system.EventTypeAdapterOperationFailure
	case EventTypeWebhookReceived:
		return system.EventTypeAdapterWebhookReceived
	case EventTypeHealthChange:
		return system.EventTypeAdapterHealthChange
	case EventTypeConfigurationChange:
		return system.EventTypeAdapterConfigurationChange
	default:
		return system.EventTypeUnknown
	}
}
