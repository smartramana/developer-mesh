package mocks

import (
	"context"
	"sync"

	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// MockEventBus is a mock implementation of the EventBusIface for testing
type MockEventBus struct {
	listeners       map[events.EventType][]events.EventListener
	globalListeners []events.EventListener
	mu              sync.RWMutex
	emittedEvents   []*events.AdapterEvent
}

// NewMockEventBus creates a new mock event bus
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		listeners:       make(map[events.EventType][]events.EventListener),
		globalListeners: []events.EventListener{},
		emittedEvents:   []*events.AdapterEvent{},
	}
}

// Subscribe subscribes to events of a specific type
// Satisfies EventBusIface by adapting to Handler signature
func (b *MockEventBus) Subscribe(eventType events.EventType, handler func(ctx context.Context, event *mcp.Event) error) {
	// This is a stub for the interface; implement as needed for your tests.
}

// Unsubscribe unsubscribes from events of a specific type
func (b *MockEventBus) Unsubscribe(eventType events.EventType, handler func(ctx context.Context, event *mcp.Event) error) {
	// This is a stub for the interface; implement as needed for your tests.
}

// Publish publishes an event to all subscribers
func (b *MockEventBus) Publish(ctx context.Context, event *mcp.Event) {
	// This is a stub for the interface; implement as needed for your tests.
}

// Close closes the mock event bus
func (b *MockEventBus) Close() {
	// No-op for mock
}



// SubscribeAll subscribes to all events
func (b *MockEventBus) SubscribeAll(listener events.EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.globalListeners = append(b.globalListeners, listener)
}



// UnsubscribeAll unsubscribes from all events
func (b *MockEventBus) UnsubscribeAll(listener events.EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Filter out the listener from global listeners
	filteredGlobalListeners := make([]events.EventListener, 0, len(b.globalListeners))
	for _, l := range b.globalListeners {
		if l != listener {
			filteredGlobalListeners = append(filteredGlobalListeners, l)
		}
	}
	
	b.globalListeners = filteredGlobalListeners
	
	// Also remove from specific event types
	for eventType, listeners := range b.listeners {
		filteredListeners := make([]events.EventListener, 0, len(listeners))
		for _, l := range listeners {
			if l != listener {
				filteredListeners = append(filteredListeners, l)
			}
		}
		
		b.listeners[eventType] = filteredListeners
	}
}

// Emit emits an event to all subscribers
func (b *MockEventBus) Emit(ctx context.Context, event *events.AdapterEvent) error {
	b.mu.Lock()
	b.emittedEvents = append(b.emittedEvents, event)
	
	// Copy listeners to avoid holding lock during processing
	listeners, exists := b.listeners[event.EventType]
	listenersCopy := make([]events.EventListener, len(listeners))
	copy(listenersCopy, listeners)
	
	globalListenersCopy := make([]events.EventListener, len(b.globalListeners))
	copy(globalListenersCopy, b.globalListeners)
	
	b.mu.Unlock()
	
	// Notify type-specific listeners
	if exists {
		for _, listener := range listenersCopy {
			if err := listener.Handle(ctx, event); err != nil {
				return err
			}
		}
	}
	
	// Notify global listeners
	for _, listener := range globalListenersCopy {
		if err := listener.Handle(ctx, event); err != nil {
			return err
		}
	}
	
	return nil
}

// EmitWithCallback emits an event and calls a callback when the event is processed
func (b *MockEventBus) EmitWithCallback(ctx context.Context, event *events.AdapterEvent, callback func(error)) error {
	err := b.Emit(ctx, event)
	if callback != nil {
		callback(err)
	}
	return err
}

// GetEmittedEvents returns all emitted events
func (b *MockEventBus) GetEmittedEvents() []*events.AdapterEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	eventsCopy := make([]*events.AdapterEvent, len(b.emittedEvents))
	copy(eventsCopy, b.emittedEvents)
	
	return eventsCopy
}

// ClearEmittedEvents clears the list of emitted events
func (b *MockEventBus) ClearEmittedEvents() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.emittedEvents = []*events.AdapterEvent{}
}
