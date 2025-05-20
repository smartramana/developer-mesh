package mocks

import (
	"context"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
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

// SubscribeListener subscribes to events of a specific type
func (b *MockEventBus) SubscribeListener(eventType events.EventType, listener events.EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()

	listeners, exists := b.listeners[eventType]
	if !exists {
		listeners = []events.EventListener{}
	}

	b.listeners[eventType] = append(listeners, listener)
}

// SubscribeAll subscribes to all events
func (b *MockEventBus) SubscribeAll(listener events.EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.globalListeners = append(b.globalListeners, listener)
}

// UnsubscribeListener unsubscribes from events of a specific type
func (b *MockEventBus) UnsubscribeListener(eventType events.EventType, listener events.EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()

	listeners, exists := b.listeners[eventType]
	if !exists {
		return
	}

	// Filter out the listener
	filteredListeners := make([]events.EventListener, 0, len(listeners))
	for _, l := range listeners {
		if l != listener {
			filteredListeners = append(filteredListeners, l)
		}
	}

	b.listeners[eventType] = filteredListeners
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
}

// Emit emits an event to all subscribers
func (b *MockEventBus) Emit(ctx context.Context, event *events.AdapterEvent) error {
	b.mu.Lock()
	b.emittedEvents = append(b.emittedEvents, event)
	b.mu.Unlock()
	
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Copy listeners to avoid holding lock during processing
	listeners, exists := b.listeners[event.EventType]
	listenersCopy := make([]events.EventListener, len(listeners))
	copy(listenersCopy, listeners)
	
	globalListenersCopy := make([]events.EventListener, len(b.globalListeners))
	copy(globalListenersCopy, b.globalListeners)
	
	// Process event (simple pass through for test)
	// Notify type-specific listeners
	if exists {
		for _, listener := range listenersCopy {
			listener.Handle(ctx, event)
		}
	}
	
	// Notify global listeners
	for _, listener := range globalListenersCopy {
		listener.Handle(ctx, event)
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

// GetEmittedEvents returns all events that have been emitted
func (b *MockEventBus) GetEmittedEvents() []*events.AdapterEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	eventsCopy := make([]*events.AdapterEvent, len(b.emittedEvents))
	copy(eventsCopy, b.emittedEvents)
	
	return eventsCopy
}

// ClearEmittedEvents clears all emitted events
func (b *MockEventBus) ClearEmittedEvents() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.emittedEvents = []*events.AdapterEvent{}
}
