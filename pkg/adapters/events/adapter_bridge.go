package events

import (
	"context"
	"reflect"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/core"
	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// isInterfaceNil checks if an interface is nil
func isInterfaceNil(i interface{}) bool {
	if i == nil {
		return true
	}
	
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Interface, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// AdapterEventBridge bridges adapter events to the main event bus
type AdapterEventBridge struct {
	eventBus        events.EventBus
	adapterRegistry interface{}
	logger          observability.Logger
	mu              sync.RWMutex
	handlers        map[string]map[string][]func(context.Context, *AdapterEvent) error
}

// NewAdapterEventBridge creates a new adapter event bridge
func NewAdapterEventBridge(eventBus events.EventBus, adapterRegistry interface{}, logger observability.Logger) *AdapterEventBridge {
	return &AdapterEventBridge{
		eventBus:        eventBus,
		adapterRegistry: adapterRegistry,
		logger:          logger,
		handlers:        make(map[string]map[string][]func(context.Context, *AdapterEvent) error),
	}
}

// EmitEvent emits an event to the main event bus
func (b *AdapterEventBridge) EmitEvent(ctx context.Context, adapterType string, eventType AdapterEventType, payload interface{}) error {
	// Create an adapter event
	event := NewAdapterEvent(adapterType, eventType, payload)
	
	// Call handlers
	if err := b.callHandlers(ctx, event); err != nil {
		return err
	}
	
	// Create a MCP event for the system event bus
	mcpEvent := event.ToMCPEvent()
	
	// Publish to event bus
	b.eventBus.Publish(ctx, mcpEvent)
	
	return nil
}

// RegisterHandler registers a handler for events from a specific adapter type
func (b *AdapterEventBridge) RegisterHandler(adapterType string, eventType AdapterEventType, handler func(context.Context, *AdapterEvent) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Create a map for this adapter type if it doesn't exist
	handlers, ok := b.handlers[adapterType]
	if !ok {
		handlers = make(map[string][]func(context.Context, *AdapterEvent) error)
		b.handlers[adapterType] = handlers
	}
	
	// Create a list for this event type if it doesn't exist
	typeHandlers, ok := handlers[string(eventType)]
	if !ok {
		typeHandlers = make([]func(context.Context, *AdapterEvent) error, 0)
	}
	
	// Add the handler
	handlers[string(eventType)] = append(typeHandlers, handler)
}

// RegisterHandlerForAllAdapters registers a handler for events from all adapters
func (b *AdapterEventBridge) RegisterHandlerForAllAdapters(eventType AdapterEventType, handler func(context.Context, *AdapterEvent) error) {
	// Register for wildcard adapter type
	b.RegisterHandler("*", eventType, handler)
	
	// If adapter registry is available, register for all adapter types
	if reg, ok := b.adapterRegistry.(interface{ ListAdapters() map[string]core.Adapter }); ok {
		adapters := reg.ListAdapters()
		for adapterType := range adapters {
			b.RegisterHandler(adapterType, eventType, handler)
		}
	}
}

// callHandlers calls all registered handlers for the given event
func (b *AdapterEventBridge) callHandlers(ctx context.Context, event *AdapterEvent) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Try to find handlers for this adapter type
	if adapterHandlers, ok := b.handlers[event.AdapterType]; ok {
		// Try to find handlers for this event type
		if handlers, ok := adapterHandlers[string(event.EventType)]; ok {
			for _, handler := range handlers {
				if err := handler(ctx, event); err != nil {
					b.logger.Warn("Error handling event", map[string]interface{}{
						"adapterType": event.AdapterType,
						"eventType": string(event.EventType),
						"error": err.Error(),
					})
				}
			}
		}
	}
	
	// Try wildcard handlers
	if adapterHandlers, ok := b.handlers["*"]; ok {
		if handlers, ok := adapterHandlers[string(event.EventType)]; ok {
			for _, handler := range handlers {
				if err := handler(ctx, event); err != nil {
					b.logger.Warn("Error handling event (wildcard handler)", map[string]interface{}{
						"adapterType": event.AdapterType,
						"eventType": string(event.EventType),
						"error": err.Error(),
					})
				}
			}
		}
	}
	
	return nil
}
