package events

import (
	"context"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// EventHandler is already defined in adapter.go

// EventBusImpl is the implementation of EventBus for adapters
type EventBusImpl struct {
	systemBus       events.EventBus
	logger          observability.Logger
	mu              sync.RWMutex
	handlers        map[string][]EventHandler  // eventType -> handlers
	globalHandlers  []EventHandler
}

// NewEventBusImpl creates a new event bus implementation
func NewEventBusImpl(systemBus events.EventBus, logger observability.Logger) *EventBusImpl {
	return &EventBusImpl{
		systemBus:      systemBus,
		logger:         logger,
		handlers:       make(map[string][]EventHandler),
		globalHandlers: make([]EventHandler, 0),
	}
}

// Subscribe subscribes to events of a specific type
func (b *EventBusImpl) Subscribe(eventType AdapterEventType, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlers, exists := b.handlers[string(eventType)]
	if !exists {
		handlers = []EventHandler{}
	}

	b.handlers[string(eventType)] = append(handlers, handler)
}

// SubscribeAll subscribes to all events
func (b *EventBusImpl) SubscribeAll(handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.globalHandlers = append(b.globalHandlers, handler)
}

// Unsubscribe unsubscribes from events of a specific type
func (b *EventBusImpl) Unsubscribe(eventType AdapterEventType, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlers, exists := b.handlers[string(eventType)]
	if !exists {
		return
	}

	// Filter out the handler
	filteredHandlers := make([]EventHandler, 0, len(handlers))
	for _, h := range handlers {
		if &h != &handler {
			filteredHandlers = append(filteredHandlers, h)
		}
	}

	b.handlers[string(eventType)] = filteredHandlers
}

// UnsubscribeAll unsubscribes from all events
func (b *EventBusImpl) UnsubscribeAll(handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Filter out the handler from global handlers
	filteredGlobalHandlers := make([]EventHandler, 0, len(b.globalHandlers))
	for _, h := range b.globalHandlers {
		if &h != &handler {
			filteredGlobalHandlers = append(filteredGlobalHandlers, h)
		}
	}
	
	b.globalHandlers = filteredGlobalHandlers
	
	// Also remove from specific event types
	for eventType, handlers := range b.handlers {
		filteredHandlers := make([]EventHandler, 0, len(handlers))
		for _, h := range handlers {
			if &h != &handler {
				filteredHandlers = append(filteredHandlers, h)
			}
		}
		
		b.handlers[eventType] = filteredHandlers
	}
}

// Emit emits an event to all subscribers
func (b *EventBusImpl) Emit(ctx context.Context, event *AdapterEvent) error {
	b.mu.RLock()
	
	// Copy handlers to avoid holding lock during processing
	handlers, exists := b.handlers[string(event.EventType)]
	handlersCopy := make([]EventHandler, len(handlers))
	copy(handlersCopy, handlers)
	
	globalHandlersCopy := make([]EventHandler, len(b.globalHandlers))
	copy(globalHandlersCopy, b.globalHandlers)
	
	b.mu.RUnlock()
	
	// Process event
	b.logger.Debug("Emitting event", map[string]interface{}{
		"adapterType": event.AdapterType,
		"eventType":   string(event.EventType),
		"handlersCount": len(handlersCopy) + len(globalHandlersCopy),
	})
	
	// Notify type-specific handlers
	if exists {
		for _, handler := range handlersCopy {
			if err := handler(ctx, event); err != nil {
				b.logger.Warn("Error handling event", map[string]interface{}{
					"adapterType": event.AdapterType,
					"eventType":   string(event.EventType),
					"error":       err.Error(),
				})
			}
		}
	}
	
	// Notify global handlers
	for _, handler := range globalHandlersCopy {
		if err := handler(ctx, event); err != nil {
			b.logger.Warn("Error handling event", map[string]interface{}{
				"adapterType": event.AdapterType,
				"eventType":   string(event.EventType),
				"error":       err.Error(),
			})
		}
	}

	// Forward to system event bus if available
	if b.systemBus != nil {
		modelEvent := event.ToModelEvent()
		b.systemBus.Publish(ctx, modelEvent)
	}
	
	return nil
}

// EmitWithCallback emits an event and calls a callback when the event is processed
func (b *EventBusImpl) EmitWithCallback(ctx context.Context, event *AdapterEvent, callback func(error)) error {
	err := b.Emit(ctx, event)
	if callback != nil {
		callback(err)
	}
	return err
}

// Close closes the event bus
func (b *EventBusImpl) Close() {
	// Clear handlers
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = make(map[string][]EventHandler)
	b.globalHandlers = make([]EventHandler, 0)
}

// ForwardToMainBus implements the events.EventBus interface for compatibility
func (b *EventBusImpl) Publish(ctx context.Context, event *models.Event) {
	if b.systemBus != nil {
		b.systemBus.Publish(ctx, event)
	}
}
