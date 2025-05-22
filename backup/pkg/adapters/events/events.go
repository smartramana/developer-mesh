package events

import (
	"context"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	corevents "github.com/S-Corkum/devops-mcp/pkg/events"
)

// Using LegacyEventType from adapter.go instead of redefining event types here
// This avoids redeclaration conflicts
// Import the uuid package for event IDs

// AdapterEventV2 is a renamed version to avoid redeclarations with LegacyAdapterEvent
type AdapterEventV2 struct {
	ID          string                 // Unique event ID
	AdapterType string                 // Type of adapter that emitted the event
	EventType   LegacyEventType        // Type of event
	Payload     interface{}            // Event payload
	Timestamp   time.Time              // Time when the event occurred
	Metadata    map[string]interface{} // Additional metadata
}

// NewAdapterEventV2 creates a new adapter event
func NewAdapterEventV2(adapterType string, eventType LegacyEventType, payload interface{}) *AdapterEventV2 {
	return &AdapterEventV2{
		ID:          "event-" + time.Now().Format(time.RFC3339),
		AdapterType: adapterType,
		EventType:   eventType,
		Payload:     payload,
		Timestamp:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}
}

// WithMetadata adds metadata to an event
func (e *AdapterEventV2) WithMetadata(key string, value interface{}) *AdapterEventV2 {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// LegacyEventEmitter represents an object that can emit adapter events
type LegacyEventEmitter interface {
	// Emit emits an event to the event bus
	Emit(ctx context.Context, event *LegacyAdapterEvent) error
	
	// EmitWithCallback emits an event and calls a callback when the event is processed
	EmitWithCallback(ctx context.Context, event *LegacyAdapterEvent, callback func(error)) error
}

// Note: LegacyEventListener interface is defined in adapter.go

// EventBusImpl is a concrete implementation of the EventBus interface
type EventBusImpl struct {
	listeners       map[LegacyEventType][]LegacyEventListener
	globalListeners []LegacyEventListener
	mu              sync.RWMutex
	logger          observability.Logger
}

// NewEventBus creates a new event bus
func NewEventBus(logger observability.Logger) *EventBusImpl {
	return &EventBusImpl{
		listeners:       make(map[LegacyEventType][]LegacyEventListener),
		globalListeners: []LegacyEventListener{},
		logger:          logger,
	}
}

// IsInitialized checks if the event bus is properly initialized
func (b *EventBusImpl) IsInitialized() bool {
	return b != nil && b.listeners != nil
}

// SubscribeListener subscribes to events of a specific type
func (b *EventBusImpl) SubscribeListener(eventType LegacyEventType, listener LegacyEventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()

	listeners, exists := b.listeners[eventType]
	if !exists {
		listeners = []LegacyEventListener{}
	}

	b.listeners[eventType] = append(listeners, listener)
}

// SubscribeAll subscribes to all events
func (b *EventBusImpl) SubscribeAll(listener LegacyEventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.globalListeners = append(b.globalListeners, listener)
}

// UnsubscribeListener unsubscribes from events of a specific type
func (b *EventBusImpl) UnsubscribeListener(eventType LegacyEventType, listener LegacyEventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()

	listeners, exists := b.listeners[eventType]
	if !exists {
		return
	}

	// Filter out the listener
	filteredListeners := make([]LegacyEventListener, 0, len(listeners))
	for _, l := range listeners {
		if l != listener {
			filteredListeners = append(filteredListeners, l)
		}
	}

	b.listeners[eventType] = filteredListeners
}

// UnsubscribeAll unsubscribes from all events
func (b *EventBusImpl) UnsubscribeAll(listener LegacyEventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Filter out the listener from global listeners
	filteredGlobalListeners := make([]LegacyEventListener, 0, len(b.globalListeners))
	for _, l := range b.globalListeners {
		if l != listener {
			filteredGlobalListeners = append(filteredGlobalListeners, l)
		}
	}
	
	b.globalListeners = filteredGlobalListeners
	
	// Also remove from specific event types
	for eventType, listeners := range b.listeners {
		filteredListeners := make([]LegacyEventListener, 0, len(listeners))
		for _, l := range listeners {
			if l != listener {
				filteredListeners = append(filteredListeners, l)
			}
		}
		
		b.listeners[eventType] = filteredListeners
	}
}

// Emit emits an event to all subscribers
func (b *EventBusImpl) Emit(ctx context.Context, event *LegacyAdapterEvent) error {
	b.mu.RLock()
	
	// Copy listeners to avoid holding lock during processing
	listeners, exists := b.listeners[event.EventType]
	listenersCopy := make([]LegacyEventListener, len(listeners))
	copy(listenersCopy, listeners)
	
	globalListenersCopy := make([]LegacyEventListener, len(b.globalListeners))
	copy(globalListenersCopy, b.globalListeners)
	
	b.mu.RUnlock()
	
	// Process event
	b.logger.Debug("Emitting event", map[string]interface{}{
		"adapterType": event.AdapterType,
		"eventType":   string(event.EventType),
		"listenersCount": len(listenersCopy) + len(globalListenersCopy),
	})
	
	// Notify type-specific listeners
	if exists {
		for _, listener := range listenersCopy {
			if err := listener.Handle(ctx, event); err != nil {
				b.logger.Warn("Error handling event", map[string]interface{}{
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
				"adapterType": event.AdapterType,
				"eventType":   string(event.EventType),
				"error":       err.Error(),
			})
		}
	}
	
	return nil
}

// EmitWithCallback emits an event and calls a callback when the event is processed
func (b *EventBusImpl) EmitWithCallback(ctx context.Context, event *LegacyAdapterEvent, callback func(error)) error {
	err := b.Emit(ctx, event)
	if callback != nil {
		callback(err)
	}
	return err
}

// Close implements the events.EventBusIface interface
func (b *EventBusImpl) Close() {
	// No resources to clean up
}

// Publish implements the events.EventBusIface interface for compatibility
func (b *EventBusImpl) Publish(ctx context.Context, event *mcp.Event) {
	if b.logger != nil {
		b.logger.Warn("Adapter EventBus.Publish called with mcp.Event; this is a no-op.", map[string]interface{}{})
	}
	// No-op
}

// LegacyEventListenerFunc adapts a Handler to an LegacyEventListener for interface compatibility
type LegacyEventListenerFunc func(ctx context.Context, event *mcp.Event) error

func (f LegacyEventListenerFunc) Handle(ctx context.Context, event *LegacyAdapterEvent) error {
	// No-op: cannot convert AdapterEventV2 to *mcp.Event
	return nil
}

// Subscribe implements corevents.EventBusIface for compatibility
func (b *EventBusImpl) Subscribe(eventType corevents.EventType, handler corevents.Handler) {
	if b.logger != nil {
		b.logger.Warn("Adapter EventBus.Subscribe called with Handler; this is a no-op.", map[string]interface{}{})
	}
	// No-op
}

// Unsubscribe implements corevents.EventBusIface for compatibility
func (b *EventBusImpl) Unsubscribe(eventType corevents.EventType, handler corevents.Handler) {
	if b.logger != nil {
		b.logger.Warn("Adapter EventBus.Unsubscribe called with Handler; this is a no-op.", map[string]interface{}{})
	}
	// No-op
}
