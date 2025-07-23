package mocks

import (
	"context"
	"sync"

	"github.com/developer-mesh/developer-mesh/pkg/adapters/events"
	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// MockEventBus is a mock implementation of the events.EventBus interface for testing
type MockEventBus struct {
	handlers       map[string][]events.EventHandler
	globalHandlers []events.EventHandler
	mu             sync.RWMutex
	emittedEvents  []*events.AdapterEvent
}

// NewMockEventBus creates a new mock event bus
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		handlers:       make(map[string][]events.EventHandler),
		globalHandlers: []events.EventHandler{},
		emittedEvents:  []*events.AdapterEvent{},
	}
}

// Subscribe subscribes to events of a specific type
func (b *MockEventBus) Subscribe(eventType events.AdapterEventType, handler events.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlers, exists := b.handlers[string(eventType)]
	if !exists {
		handlers = []events.EventHandler{}
	}

	b.handlers[string(eventType)] = append(handlers, handler)
}

// SubscribeAll subscribes to all events
func (b *MockEventBus) SubscribeAll(handler events.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.globalHandlers = append(b.globalHandlers, handler)
}

// Unsubscribe unsubscribes from events of a specific type
func (b *MockEventBus) Unsubscribe(eventType events.AdapterEventType, handler events.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlers, exists := b.handlers[string(eventType)]
	if !exists {
		return
	}

	// Filter out the handler
	filteredHandlers := make([]events.EventHandler, 0, len(handlers))
	for _, h := range handlers {
		if &h != &handler {
			filteredHandlers = append(filteredHandlers, h)
		}
	}

	b.handlers[string(eventType)] = filteredHandlers
}

// UnsubscribeAll unsubscribes from all events
func (b *MockEventBus) UnsubscribeAll(handler events.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Filter out the handler from global handlers
	filteredGlobalHandlers := make([]events.EventHandler, 0, len(b.globalHandlers))
	for _, h := range b.globalHandlers {
		if &h != &handler {
			filteredGlobalHandlers = append(filteredGlobalHandlers, h)
		}
	}

	b.globalHandlers = filteredGlobalHandlers

	// Also remove from specific event types
	for eventType, handlers := range b.handlers {
		filteredHandlers := make([]events.EventHandler, 0, len(handlers))
		for _, h := range handlers {
			if &h != &handler {
				filteredHandlers = append(filteredHandlers, h)
			}
		}

		b.handlers[eventType] = filteredHandlers
	}
}

// Emit emits an event to all subscribers
func (b *MockEventBus) Emit(ctx context.Context, event *events.AdapterEvent) error {
	b.mu.Lock()
	b.emittedEvents = append(b.emittedEvents, event)
	b.mu.Unlock()

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Copy handlers to avoid holding lock during processing
	handlers, exists := b.handlers[string(event.EventType)]
	handlersCopy := make([]events.EventHandler, len(handlers))
	copy(handlersCopy, handlers)

	globalHandlersCopy := make([]events.EventHandler, len(b.globalHandlers))
	copy(globalHandlersCopy, b.globalHandlers)

	// Process event (simple pass through for test)
	// Notify type-specific handlers
	if exists {
		for _, handler := range handlersCopy {
			_ = handler(ctx, event) // Ignore errors in mock
		}
	}

	// Notify global handlers
	for _, handler := range globalHandlersCopy {
		_ = handler(ctx, event) // Ignore errors in mock
	}

	return nil
}

// Publish publishes a model event to the system bus (required for EventBus interface)
func (b *MockEventBus) Publish(ctx context.Context, event *models.Event) {
	// This is a stub for the interface; implement as needed for your tests.
}

// Close closes the mock event bus
func (b *MockEventBus) Close() {
	// No-op for mock
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
