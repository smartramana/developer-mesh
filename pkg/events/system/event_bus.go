package system

import (
	"context"
	"sync"
)

// EventHandler defines a handler function for system events
type EventHandler func(ctx context.Context, event Event) error

// SimpleEventBus provides a basic implementation of the EventBus interface
type SimpleEventBus struct {
	handlers map[EventType][]EventHandler
	mutex    sync.RWMutex
}

// NewSimpleEventBus creates a new SimpleEventBus
func NewSimpleEventBus() *SimpleEventBus {
	return &SimpleEventBus{
		handlers: make(map[EventType][]EventHandler),
	}
}

// Publish publishes an event to all registered handlers
func (b *SimpleEventBus) Publish(ctx context.Context, event Event) error {
	if event == nil {
		return nil
	}

	b.mutex.RLock()
	handlers, exists := b.handlers[event.GetType()]
	// Make a copy to avoid holding the lock during handler execution
	handlersCopy := make([]EventHandler, len(handlers))
	copy(handlersCopy, handlers)
	b.mutex.RUnlock()

	if !exists || len(handlersCopy) == 0 {
		return nil
	}

	// Execute handlers
	for _, handler := range handlersCopy {
		if err := handler(ctx, event); err != nil {
			// In a real implementation, we might want to log errors here
			// but we continue processing other handlers
			_ = err
		}
	}

	return nil
}

// Subscribe registers a handler for events of a specific type
func (b *SimpleEventBus) Subscribe(eventType EventType, handler EventHandler) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if _, exists := b.handlers[eventType]; !exists {
		b.handlers[eventType] = []EventHandler{}
	}

	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Unsubscribe removes a handler for events of a specific type
func (b *SimpleEventBus) Unsubscribe(eventType EventType, handler EventHandler) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	handlers, exists := b.handlers[eventType]
	if !exists {
		return
	}

	// Find and remove the handler
	for i, h := range handlers {
		if &h == &handler {
			// Remove by replacing with last element and truncating
			handlers[i] = handlers[len(handlers)-1]
			b.handlers[eventType] = handlers[:len(handlers)-1]
			break
		}
	}
}
