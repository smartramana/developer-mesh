package bridge

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/core"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	eventsmocks "github.com/S-Corkum/devops-mcp/pkg/adapters/events/mocks"
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
	logger           *observability.Logger
	adapterRegistry  interface{} // Using interface{} to support different adapter registry implementations
	adapterHandlers  map[string]map[string][]func(context.Context, *events.AdapterEvent) error
	mu               sync.RWMutex
}

// NewEventBridge creates a new event bridge.
func NewEventBridge(
	eventBus interface{},
	systemEventBus system.EventBus,
	logger *observability.Logger,
	adapterRegistry interface{},
) *EventBridge {
	bridge := &EventBridge{
		eventBus:        eventBus,
		systemEventBus:  systemEventBus,
		logger:          logger,
		adapterRegistry: adapterRegistry,
		adapterHandlers: make(map[string]map[string][]func(context.Context, *events.AdapterEvent) error),
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
	if eventBus, ok := b.eventBus.(*events.EventBus); ok && eventBus != nil {
		eventBus.SubscribeAll(b)
	} else if eventBus, ok := b.eventBus.(events.EventBus); ok {
		eventBus.SubscribeAll(b)
	} else if eventBus, ok := b.eventBus.(*eventsmocks.MockEventBus); ok && eventBus != nil {
		eventBus.SubscribeAll(b)
	} else {
		// Try to use a type assertion to call SubscribeAll directly
		// This is needed for tests where we use mock.Mock based mocks
		if mockBus, ok := b.eventBus.(interface{ SubscribeAll(events.EventListener) }); ok {
			mockBus.SubscribeAll(b)
		}
	}
}

// Handle handles adapter events and maps them to system events
func (b *EventBridge) Handle(ctx context.Context, event *events.AdapterEvent) error {
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
func (b *EventBridge) RegisterHandler(adapterType string, eventType events.EventType, handler func(context.Context, *events.AdapterEvent) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Initialize maps if they don't exist
	handlers, ok := b.adapterHandlers[adapterType]
	if !ok {
		handlers = make(map[string][]func(context.Context, *events.AdapterEvent) error)
		b.adapterHandlers[adapterType] = handlers
	}
	
	// Append handler
	typeHandlers, ok := handlers[string(eventType)]
	if !ok {
		typeHandlers = []func(context.Context, *events.AdapterEvent) error{}
	}
	
	handlers[string(eventType)] = append(typeHandlers, handler)
}

// RegisterHandlerForAllAdapters registers a handler for all adapters with the specified event type
func (b *EventBridge) RegisterHandlerForAllAdapters(eventType events.EventType, handler func(context.Context, *events.AdapterEvent) error) {
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
func (b *EventBridge) callEventHandlers(ctx context.Context, event *events.AdapterEvent) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	var lastErr error
	
	// First try specific adapter + event type handlers
	if adapterHandlers, ok := b.adapterHandlers[event.AdapterType]; ok {
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
			// If we found handlers for this specific combination, return
			return lastErr
		}
	}
	
	// Try wildcard handlers for this adapter + any event type
	if adapterHandlers, ok := b.adapterHandlers[event.AdapterType]; ok {
		if typeHandlers, ok := adapterHandlers["*"]; ok {
			for _, handler := range typeHandlers {
				if err := handler(ctx, event); err != nil {
					lastErr = err
				}
			}
			return lastErr
		}
	}
	
	// Try wildcard handlers for any adapter + this event type
	if adapterHandlers, ok := b.adapterHandlers["*"]; ok {
		if typeHandlers, ok := adapterHandlers[string(event.EventType)]; ok {
			for _, handler := range typeHandlers {
				if err := handler(ctx, event); err != nil {
					lastErr = err
				}
			}
			return lastErr
		}
	}
	
	// Try wildcard handlers for any adapter + any event type
	if adapterHandlers, ok := b.adapterHandlers["*"]; ok {
		if typeHandlers, ok := adapterHandlers["*"]; ok {
			for _, handler := range typeHandlers {
				if err := handler(ctx, event); err != nil {
					lastErr = err
				}
			}
			return lastErr
		}
	}
	
	// No handlers found
	return nil
}

// mapToSystemEvent maps an adapter event to a system event
func (b *EventBridge) mapToSystemEvent(adapterEvent *events.AdapterEvent) system.Event {
	baseEvent := system.BaseEvent{
		Type:      getSystemEventType(adapterEvent.EventType),
		Timestamp: adapterEvent.Timestamp,
	}

	// Map adapter events to system events based on their type
	switch adapterEvent.EventType {
	case events.EventTypeOperationSuccess:
		// Extract contextId or default to empty string if not present
		contextID := ""
		if id, ok := adapterEvent.Metadata["contextId"]; ok {
			contextID = fmt.Sprintf("%v", id)
		}
		
		// Extract operation or default to empty string if not present
		operation := ""
		if op, ok := adapterEvent.Metadata["operation"]; ok {
			operation = fmt.Sprintf("%v", op)
		}
		
		return &system.AdapterOperationSuccessEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			Operation:   operation,
			Result:      adapterEvent.Payload,
			ContextID:   contextID,
		}
		
	case events.EventTypeOperationFailure:
		// Extract contextId or default to empty string if not present
		contextID := ""
		if id, ok := adapterEvent.Metadata["contextId"]; ok {
			contextID = fmt.Sprintf("%v", id)
		}
		
		// Extract operation or default to empty string if not present
		operation := ""
		if op, ok := adapterEvent.Metadata["operation"]; ok {
			operation = fmt.Sprintf("%v", op)
		}
		
		// Extract error message or default to empty string if not present
		errorMsg := ""
		if err, ok := adapterEvent.Metadata["error"]; ok {
			errorMsg = fmt.Sprintf("%v", err)
		}
		
		return &system.AdapterOperationFailureEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			Operation:   operation,
			Error:       errorMsg,
			ContextID:   contextID,
		}
		
	case events.EventTypeWebhookReceived:
		// Extract contextId or default to empty string if not present
		contextID := ""
		if id, ok := adapterEvent.Metadata["contextId"]; ok {
			contextID = fmt.Sprintf("%v", id)
		}
		
		// Extract eventType or default to empty string if not present
		eventType := ""
		if et, ok := adapterEvent.Metadata["eventType"]; ok {
			eventType = fmt.Sprintf("%v", et)
		}
		
		return &system.WebhookReceivedEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			EventType:   eventType,
			Payload:     adapterEvent.Payload,
			ContextID:   contextID,
		}
		
	case events.EventTypeAdapterHealthChanged:
		// Extract status information
		oldStatus := ""
		if os, ok := adapterEvent.Metadata["oldStatus"]; ok {
			oldStatus = fmt.Sprintf("%v", os)
		}
		
		newStatus := ""
		if ns, ok := adapterEvent.Metadata["newStatus"]; ok {
			newStatus = fmt.Sprintf("%v", ns)
		}
		
		return &system.AdapterHealthChangedEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			OldStatus:   oldStatus,
			NewStatus:   newStatus,
		}
		
	default:
		// Generic adapter event
		return &system.AdapterGenericEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			EventType:   string(adapterEvent.EventType),
			Payload:     adapterEvent.Payload,
			Metadata:    adapterEvent.Metadata,
		}
	}
}

// getSystemEventType maps adapter event types to system event types
func getSystemEventType(eventType events.EventType) system.EventType {
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

// End of EventBridge implementation
