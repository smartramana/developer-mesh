package bridge

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/core"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	"github.com/S-Corkum/devops-mcp/pkg/events/system"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// isInterfaceNil checks if an interface value is nil or contains nil
func isInterfaceNil(i interface{}) bool {
	if i == nil {
		return true
	}
	
	v := reflect.ValueOf(i)
	
	// Only call IsNil on interfaces, pointers, maps, slices, channels, and funcs
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}

// EventBridge bridges adapter events to the system event bus.
// It handles the communication between adapter-specific events and the system-wide
// event bus, translating events between different formats and contexts.
type EventBridge struct {
	eventBus         interface{} // Could be events.EventBus
	systemEventBus   system.EventBus
	logger           observability.Logger
	adapterRegistry  interface{} // Using interface{} to support different adapter registry implementations
	adapterHandlers  map[string]map[string][]func(context.Context, *events.LegacyAdapterEvent) error
	mu               sync.RWMutex
}

// NewEventBridge creates a new event bridge.
func NewEventBridge(
	eventBus interface{},
	systemEventBus system.EventBus,
	logger observability.Logger,
	adapterRegistry interface{},
) *EventBridge {
	bridge := &EventBridge{
		eventBus:        eventBus,
		systemEventBus:  systemEventBus,
		logger:          logger,
		adapterRegistry: adapterRegistry,
		adapterHandlers: make(map[string]map[string][]func(context.Context, *events.LegacyAdapterEvent) error),
	}
	
	// Subscribe to events if event bus is available
	bridge.subscribeToEvents()
	
	return bridge
}

// subscribeToEvents subscribes to the event bus events
func (b *EventBridge) subscribeToEvents() {
	// Use a defer to catch any panic and log it
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("Failed to subscribe to event bus", map[string]interface{}{
				"error": r,
			})
		}
	}()
	
	// Check if the eventBus is nil before trying to use it
	if b.eventBus == nil {
		return
	}
	
	// Check if the eventBus implements the right interface
	if eventBus, ok := b.eventBus.(events.LegacyEventBus); ok {
		eventBus.SubscribeAll(b)
	} else {
		// Try to use a type assertion to call SubscribeAll directly
		if mockBus, ok := b.eventBus.(interface{ SubscribeAll(events.LegacyEventListener) }); ok {
			mockBus.SubscribeAll(b)
		}
	}
}

// Handle processes events from the adapter bus and forwards them to the system bus
func (b *EventBridge) Handle(ctx context.Context, event *events.LegacyAdapterEvent) error {
	return b.dispatchAdapterEvent(ctx, event)
}

// dispatchAdapterEvent dispatches an adapter event to the system event bus and calls registered handlers
func (b *EventBridge) dispatchAdapterEvent(ctx context.Context, event *events.LegacyAdapterEvent) error {
	// Forward event to system event bus
	systemEvent := b.mapToSystemEvent(event)
	if systemEvent != nil {
		if err := b.systemEventBus.Publish(ctx, systemEvent); err != nil {
			b.logger.Warn("Failed to publish system event", map[string]interface{}{
				"adapterType": event.AdapterType,
				"eventType":   string(event.EventType),
				"error":       err.Error(),
			})
		}
	}
	
	// Call registered handlers for this event
	return b.callEventHandlers(ctx, event)
}

// RegisterHandler registers a handler for a specific adapter and event type
func (b *EventBridge) RegisterHandler(adapterType string, eventType events.LegacyEventType, handler func(context.Context, *events.LegacyAdapterEvent) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Initialize maps if they don't exist
	handlers, ok := b.adapterHandlers[adapterType]
	if !ok {
		handlers = make(map[string][]func(context.Context, *events.LegacyAdapterEvent) error)
		b.adapterHandlers[adapterType] = handlers
	}
	
	// Append handler
	typeHandlers, ok := handlers[string(eventType)]
	if !ok {
		typeHandlers = []func(context.Context, *events.LegacyAdapterEvent) error{}
	}
	
	handlers[string(eventType)] = append(typeHandlers, handler)
}

// RegisterHandlerForAllAdapters registers a handler for all adapters with the specified event type
func (b *EventBridge) RegisterHandlerForAllAdapters(eventType events.LegacyEventType, handler func(context.Context, *events.LegacyAdapterEvent) error) {
	// Register a wildcard handler for this event type
	b.RegisterHandler("*", eventType, handler)
	
	// Also register for all existing adapters if registry is available
	if reg, ok := b.adapterRegistry.(interface{ ListAdapters() map[string]core.Adapter }); ok {
		adapters := reg.ListAdapters()
		for adapterType := range adapters {
			b.RegisterHandler(adapterType, eventType, handler)
		}
	}
}

// callEventHandlers calls registered handlers for an event
func (b *EventBridge) callEventHandlers(ctx context.Context, event *events.LegacyAdapterEvent) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var lastErr error

	// First try to find handlers for this specific adapter + event type
	if adapterHandlers, ok := b.adapterHandlers[event.AdapterType]; ok {
		// Try specific event type handlers
		if typeHandlers, ok := adapterHandlers[string(event.EventType)]; ok {
			for _, handler := range typeHandlers {
				if err := handler(ctx, event); err != nil {
					b.logger.Warn("Error in event handler", map[string]interface{}{
						"adapterType": event.AdapterType,
						"eventType":   string(event.EventType),
						"error":       err.Error(),
					})
					lastErr = err
				}
			}
			return lastErr
		}

		// Try wildcard handlers for this adapter
		if typeHandlers, ok := adapterHandlers["*"]; ok {
			for _, handler := range typeHandlers {
				if err := handler(ctx, event); err != nil {
					b.logger.Warn("Error in wildcard event handler", map[string]interface{}{
						"adapterType": event.AdapterType,
						"eventType":   string(event.EventType),
						"error":       err.Error(),
					})
					lastErr = err
				}
			}
		}
	}

	// Try wildcard handlers for any adapter + this event type
	if adapterHandlers, ok := b.adapterHandlers["*"]; ok {
		if typeHandlers, ok := adapterHandlers[string(event.EventType)]; ok {
			for _, handler := range typeHandlers {
				if err := handler(ctx, event); err != nil {
					b.logger.Warn("Error in global event handler", map[string]interface{}{
						"adapterType": event.AdapterType,
						"eventType":   string(event.EventType),
						"error":       err.Error(),
					})
					lastErr = err
				}
			}
		}
	}
	
	return lastErr
}

// mapToSystemEvent maps an adapter event to a system event
func (b *EventBridge) mapToSystemEvent(event *events.LegacyAdapterEvent) system.Event {
	// Create base event with current time since original timestamp might be incompatible
	baseEvent := system.BaseEvent{
		Type:      system.EventTypeAdapterGeneric, // Default type
		Timestamp: time.Now(),
	}

	// For simplicity during migration, convert all events to generic system events
	return &system.AdapterGenericEvent{
		BaseEvent:  baseEvent,
		AdapterType: event.AdapterType,
		EventType:  string(event.EventType),
		Payload:    event.Payload,
		Metadata:   event.Metadata,
	}
}

// getSystemEventType returns the system event type for a given adapter event type
func getSystemEventType(eventType events.LegacyEventType) system.EventType {
	switch eventType {
	case events.EventTypeOperationSuccess:
		return system.EventTypeAdapterOperationSuccess
	case events.EventTypeOperationFailure:
		return system.EventTypeAdapterOperationFailure
	case events.EventTypeWebhookReceived:
		return system.EventTypeWebhookReceived
	case events.EventTypeAdapterHealthChanged:
		return system.EventTypeAdapterHealthChanged
	default:
		return system.EventTypeAdapterGeneric
	}
}
