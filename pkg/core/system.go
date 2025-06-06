package core

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/events/system"
)

// SystemEventBus implements the system.EventBus interface
type SystemEventBus struct {
	handlers map[system.EventType][]func(context.Context, system.Event) error
}

// NewSystemEventBus creates a new system event bus
func NewSystemEventBus() *SystemEventBus {
	return &SystemEventBus{
		handlers: make(map[system.EventType][]func(context.Context, system.Event) error),
	}
}

// Publish publishes an event to all subscribers
func (b *SystemEventBus) Publish(ctx context.Context, event system.Event) error {
	handlers, ok := b.handlers[event.GetType()]
	if !ok {
		return nil
	}

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			// Log error but continue
			_ = err
		}
	}

	return nil
}

// Subscribe subscribes to events of a specific type
func (b *SystemEventBus) Subscribe(eventType system.EventType, handler func(context.Context, system.Event) error) {
	if _, ok := b.handlers[eventType]; !ok {
		b.handlers[eventType] = []func(context.Context, system.Event) error{}
	}

	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Unsubscribe unsubscribes from events of a specific type
func (b *SystemEventBus) Unsubscribe(eventType system.EventType, handler func(context.Context, system.Event) error) {
	handlers, ok := b.handlers[eventType]
	if !ok {
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
