package events

import (
	"context"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// EventBusImpl is the implementation of the EventBus interface
type EventBusImpl struct {
	handlers     map[EventType][]Handler
	mutex        sync.RWMutex
	maxQueueSize int
	queue        chan *models.Event
}

// NewEventBus creates a new event bus
func NewEventBus(maxQueueSize int) *EventBusImpl {
	return &EventBusImpl{
		handlers:     make(map[EventType][]Handler),
		maxQueueSize: maxQueueSize,
		queue:        make(chan *models.Event, maxQueueSize),
	}
}

// Publish publishes an event
func (b *EventBusImpl) Publish(ctx context.Context, event *models.Event) {
	// Get handlers
	b.mutex.RLock()
	handlers := b.handlers[EventType(event.Type)]
	b.mutex.RUnlock()

	// Call handlers
	for _, handler := range handlers {
		go func(h Handler, e *models.Event) {
			err := h(ctx, e)
			if err != nil {
				// TODO: Add error logging when logger is available
				// Continue processing other handlers
				_ = err
			}
		}(handler, event)
	}
}

// Subscribe subscribes to events of a specific type
func (b *EventBusImpl) Subscribe(eventType EventType, handler Handler) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Unsubscribe unsubscribes from events of a specific type
func (b *EventBusImpl) Unsubscribe(eventType EventType, handler Handler) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var handlers []Handler
	for _, h := range b.handlers[eventType] {
		if &h != &handler {
			handlers = append(handlers, h)
		}
	}
	b.handlers[eventType] = handlers
}

// Close closes the event bus
func (b *EventBusImpl) Close() {
	// Stop processing events
	close(b.queue)

	// Clear handlers
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.handlers = make(map[EventType][]Handler)
}
