package events

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// EventType represents the type of an adapter event
type EventType string

// Common event types
const (
	EventTypeOperationStarting EventType = "operation.starting"
	EventTypeOperationSuccess  EventType = "operation.success"
	EventTypeOperationFailure  EventType = "operation.failure"
	EventTypeAdapterInitialized EventType = "adapter.initialized"
	EventTypeAdapterClosed      EventType = "adapter.closed"
	EventTypeAdapterHealthChanged EventType = "adapter.health_changed"
	EventTypeWebhookReceived    EventType = "webhook.received"
)

// AdapterEvent represents an event from an adapter
type AdapterEvent struct {
	ID          string                 // Unique event ID
	AdapterType string                 // Type of adapter that emitted the event
	EventType   EventType              // Type of event
	Payload     interface{}            // Event payload
	Timestamp   time.Time              // Time when the event occurred
	Metadata    map[string]interface{} // Additional metadata
}

// NewAdapterEvent creates a new adapter event
func NewAdapterEvent(adapterType string, eventType EventType, payload interface{}) *AdapterEvent {
	return &AdapterEvent{
		ID:          uuid.New().String(),
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

// EventEmitter allows adapters to emit events
type EventEmitter interface {
	// Emit emits an event
	Emit(ctx context.Context, event *AdapterEvent) error
	
	// EmitWithCallback emits an event and calls a callback when the event is processed
	EmitWithCallback(ctx context.Context, event *AdapterEvent, callback func(error)) error
}

// EventListener listens for adapter events
type EventListener interface {
	// Handle handles an event
	Handle(ctx context.Context, event *AdapterEvent) error
}

// EventBus is a simple event bus for adapter events
type EventBus struct {
	listeners       map[EventType][]EventListener
	globalListeners []EventListener
	mu              sync.RWMutex
	logger          *observability.Logger
}

// NewEventBus creates a new event bus
func NewEventBus(logger *observability.Logger) *EventBus {
	return &EventBus{
		listeners:       make(map[EventType][]EventListener),
		globalListeners: []EventListener{},
		logger:          logger,
	}
}

// IsInitialized checks if the event bus is properly initialized
func (b *EventBus) IsInitialized() bool {
	return b != nil && b.listeners != nil
}

// SubscribeListener subscribes to events of a specific type
func (b *EventBus) SubscribeListener(eventType EventType, listener EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()

	listeners, exists := b.listeners[eventType]
	if !exists {
		listeners = []EventListener{}
	}

	b.listeners[eventType] = append(listeners, listener)
}

// SubscribeAll subscribes to all events
func (b *EventBus) SubscribeAll(listener EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.globalListeners = append(b.globalListeners, listener)
}

// UnsubscribeListener unsubscribes from events of a specific type
func (b *EventBus) UnsubscribeListener(eventType EventType, listener EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()

	listeners, exists := b.listeners[eventType]
	if !exists {
		return
	}

	// Filter out the listener
	filteredListeners := make([]EventListener, 0, len(listeners))
	for _, l := range listeners {
		if l != listener {
			filteredListeners = append(filteredListeners, l)
		}
	}

	b.listeners[eventType] = filteredListeners
}

// UnsubscribeAll unsubscribes from all events
func (b *EventBus) UnsubscribeAll(listener EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Filter out the listener from global listeners
	filteredGlobalListeners := make([]EventListener, 0, len(b.globalListeners))
	for _, l := range b.globalListeners {
		if l != listener {
			filteredGlobalListeners = append(filteredGlobalListeners, l)
		}
	}
	
	b.globalListeners = filteredGlobalListeners
	
	// Also remove from specific event types
	for eventType, listeners := range b.listeners {
		filteredListeners := make([]EventListener, 0, len(listeners))
		for _, l := range listeners {
			if l != listener {
				filteredListeners = append(filteredListeners, l)
			}
		}
		
		b.listeners[eventType] = filteredListeners
	}
}

// Emit emits an event to all subscribers
func (b *EventBus) Emit(ctx context.Context, event *AdapterEvent) error {
	b.mu.RLock()
	
	// Copy listeners to avoid holding lock during processing
	listeners, exists := b.listeners[event.EventType]
	listenersCopy := make([]EventListener, len(listeners))
	copy(listenersCopy, listeners)
	
	globalListenersCopy := make([]EventListener, len(b.globalListeners))
	copy(globalListenersCopy, b.globalListeners)
	
	b.mu.RUnlock()
	
	// Process event
	b.logger.Debug("Emitting event", map[string]interface{}{
		"eventId":     event.ID,
		"adapterType": event.AdapterType,
		"eventType":   string(event.EventType),
		"listenersCount": len(listenersCopy) + len(globalListenersCopy),
	})
	
	// Notify type-specific listeners
	if exists {
		for _, listener := range listenersCopy {
			if err := listener.Handle(ctx, event); err != nil {
				b.logger.Warn("Error handling event", map[string]interface{}{
					"eventId":     event.ID,
					"adapterType": event.AdapterType,
					"eventType":   string(event.EventType),
					"error":       err.Error(),
				})
			}
		}
	}
	
	// Notify global listeners
	for _, listener := range globalListenersCopy {
		if err := listener.Handle(ctx, event); err != nil {
			b.logger.Warn("Error handling event", map[string]interface{}{
				"eventId":     event.ID,
				"adapterType": event.AdapterType,
				"eventType":   string(event.EventType),
				"error":       err.Error(),
			})
		}
	}
	
	return nil
}

// EmitWithCallback emits an event and calls a callback when the event is processed
func (b *EventBus) EmitWithCallback(ctx context.Context, event *AdapterEvent, callback func(error)) error {
	err := b.Emit(ctx, event)
	if callback != nil {
		callback(err)
	}
	return err
}

// Close implements the events.EventBusIface interface. No-op for adapter EventBus.
func (b *EventBus) Close() {
	// No resources to clean up
}

// Publish implements the events.EventBusIface interface for test compatibility. Not intended for use.
func (b *EventBus) Publish(ctx context.Context, event *mcp.Event) {
	if b.logger != nil {
		b.logger.Warn("Adapter EventBus.Publish called with mcp.Event; this is a no-op.", map[string]interface{}{})
	}
	// No-op: Adapter EventBus does not handle mcp.Event
}

// EventListenerFunc adapts a Handler to an EventListener for interface compatibility (no-op).
type EventListenerFunc func(ctx context.Context, event *mcp.Event) error

func (f EventListenerFunc) Handle(ctx context.Context, event *AdapterEvent) error {
	// No-op: cannot convert AdapterEvent to *mcp.Event
	return nil
}

// Subscribe implements events.EventBusIface for test compatibility.
func (b *EventBus) Subscribe(eventType EventType, handler func(ctx context.Context, event *mcp.Event) error) {
	if b.logger != nil {
		b.logger.Warn("Adapter EventBus.Subscribe called with Handler; this is a no-op.", map[string]interface{}{})
	}
	// No-op: types are incompatible
}

// Unsubscribe implements events.EventBusIface for test compatibility.
func (b *EventBus) Unsubscribe(eventType EventType, handler func(ctx context.Context, event *mcp.Event) error) {
	if b.logger != nil {
		b.logger.Warn("Adapter EventBus.Unsubscribe called with Handler; this is a no-op.", map[string]interface{}{})
	}
	// No-op: types are incompatible
}

