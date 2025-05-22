// Package events provides adapter functionality for connecting to the system event bus.
package events

import (
	"context"
	"time"

	corevents "github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
)

// LegacyEventType represents the type of legacy adapter event
type LegacyEventType string

const (
	// Legacy Event types for adapters
	EventTypeOperationSuccess      LegacyEventType = "operation.success"
	EventTypeOperationFailure      LegacyEventType = "operation.failure"
	EventTypeWebhookReceived       LegacyEventType = "webhook.received"
	EventTypeAdapterHealthChanged  LegacyEventType = "adapter.health_changed"
	EventTypeRepositoryEvent       LegacyEventType = "repository.event"
)

// LegacyAdapterEvent represents an event from a legacy adapter
type LegacyAdapterEvent struct {
	AdapterType string
	EventType   LegacyEventType
	Payload     interface{}
	Timestamp   int64
	Metadata    map[string]interface{}
}

// LegacyEventListener is the interface for legacy event listeners
type LegacyEventListener interface {
	// Handle handles an adapter event
	Handle(ctx context.Context, event *LegacyAdapterEvent) error
}

// LegacyEventBus is the interface for legacy event bus implementations
type LegacyEventBus interface {
	// Publish publishes an event
	Publish(ctx context.Context, event *mcp.Event)
	// Subscribe subscribes to events of a specific type
	Subscribe(eventType LegacyEventType, listener LegacyEventListener)
	// SubscribeAll subscribes to all events
	SubscribeAll(listener LegacyEventListener)
	// Close closes the event bus (optional, not all implementations require this)
	Close() error
}

// LegacyEventBusAdapter adapts the pkg/events EventBusIface to work with the LegacyEventBus interface
type LegacyEventBusAdapter struct {
	underlying corevents.EventBusIface
}

// NewLegacyEventBusAdapter creates a new LegacyEventBusAdapter
func NewLegacyEventBusAdapter(bus corevents.EventBusIface) LegacyEventBus {
	return &LegacyEventBusAdapter{
		underlying: bus,
	}
}

// Publish implements the LegacyEventBus.Publish method
func (a *LegacyEventBusAdapter) Publish(ctx context.Context, event *mcp.Event) {
	if a.underlying != nil {
		a.underlying.Publish(ctx, event)
	}
}

// Subscribe implements the LegacyEventBus.Subscribe method
func (a *LegacyEventBusAdapter) Subscribe(eventType LegacyEventType, listener LegacyEventListener) {
	if a.underlying == nil {
		return
	}
	
	// Create a handler function that adapts the legacy listener to a core handler
	handler := corevents.Handler(func(ctx context.Context, event *mcp.Event) error {
		// Create a legacy adapter event with the event data
		legacyEvent := &LegacyAdapterEvent{
			AdapterType: "system",
			EventType:   eventType,
			Payload:     event.Data,
			Timestamp:   time.Now().Unix(),
			Metadata:    make(map[string]interface{}),
		}
		
		// Call the listener with the legacy adapter event
		err := listener.Handle(ctx, legacyEvent)
		return err
	})
	
	// Subscribe to the underlying event bus
	a.underlying.Subscribe(corevents.EventType(string(eventType)), handler)
}

// SubscribeAll implements the LegacyEventBus.SubscribeAll method
func (a *LegacyEventBusAdapter) SubscribeAll(listener LegacyEventListener) {
	if a.underlying == nil {
		return
	}
	
	// Create a handler for all events
	handler := corevents.Handler(func(ctx context.Context, event *mcp.Event) error {
		// Create a generic legacy adapter event
		legacyEvent := &LegacyAdapterEvent{
			AdapterType: "system",
			EventType:   "*", // Wildcard event type
			Payload:     event.Data,
			Timestamp:   time.Now().Unix(),
			Metadata:    make(map[string]interface{}),
		}
		
		// Call the listener with the legacy adapter event
		err := listener.Handle(ctx, legacyEvent)
		return err
	})
	
	// Use a wildcard pattern to subscribe to all events
	a.underlying.Subscribe("*", handler)
}

// Close implements the LegacyEventBus.Close method
func (a *LegacyEventBusAdapter) Close() error {
	// No-op since the underlying bus may not have a Close method
	if a.underlying != nil {
		a.underlying.Close()
	}
	return nil
}
