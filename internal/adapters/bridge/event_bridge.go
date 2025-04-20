package bridge

import (
	"context"
	"fmt"
	"sync"

	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/events/system"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// EventBridge bridges adapter events to the system event bus.
// It handles the communication between adapter-specific events and the system-wide
// event bus, translating events between different formats and contexts.
type EventBridge struct {
	eventBus         *events.EventBus
	systemEventBus   system.EventBus
	logger           *observability.Logger
	adapterRegistry  interface{} // Using interface{} to support both real and mock types
	adapterHandlers  map[string]map[string][]func(context.Context, *events.AdapterEvent) error
	mu               sync.RWMutex
}

// NewEventBridge creates a new event bridge.
// This function accepts an adapter registry as interface{} to avoid dependency issues.
func NewEventBridge(
	eventBus *events.EventBus,
	systemEventBus system.EventBus,
	logger *observability.Logger,
	adapterRegistry interface{}, // Changed to interface{} to support mock types
) *EventBridge {
	bridge := &EventBridge{
		eventBus:        eventBus,
		systemEventBus:  systemEventBus,
		logger:          logger,
		adapterRegistry: adapterRegistry,
		adapterHandlers: make(map[string]map[string][]func(context.Context, *events.AdapterEvent) error),
	}
	
	// Subscribe to adapter events
	if eventBus != nil {
		eventBus.SubscribeAll(bridge)
	}
	
	return bridge
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
	// In real code, we would get the adapters from the registry
	// For now, just register the handler for the wildcard pattern
	// This avoids the nil pointer dereference while maintaining functionality
	b.RegisterHandler("*", eventType, handler)
}

// callEventHandlers calls registered handlers for an event
func (b *EventBridge) callEventHandlers(ctx context.Context, event *events.AdapterEvent) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Get handlers for this adapter
	adapterHandlers, ok := b.adapterHandlers[event.AdapterType]
	if !ok {
		// Try wildcard handlers
		adapterHandlers, ok = b.adapterHandlers["*"]
		if !ok {
			// No handlers registered
			return nil
		}
	}
	
	// Get handlers for this event type
	typeHandlers, ok := adapterHandlers[string(event.EventType)]
	if !ok {
		// Try wildcard handlers
		typeHandlers, ok = adapterHandlers["*"]
		if !ok {
			// No handlers registered
			return nil
		}
	}
	
	// Call all handlers
	var lastErr error
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

// mapToSystemEvent maps an adapter event to a system event
func (b *EventBridge) mapToSystemEvent(adapterEvent *events.AdapterEvent) system.Event {
	baseEvent := system.BaseEvent{
		Type:      getSystemEventType(adapterEvent.EventType),
		Timestamp: adapterEvent.Timestamp,
	}

	// Map adapter events to system events based on their type
	switch adapterEvent.EventType {
	case events.EventTypeOperationSuccess:
		return &system.AdapterOperationSuccessEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			Operation:   fmt.Sprintf("%v", adapterEvent.Metadata["operation"]),
			Result:      adapterEvent.Payload,
			ContextID:   fmt.Sprintf("%v", adapterEvent.Metadata["contextId"]),
		}
		
	case events.EventTypeOperationFailure:
		return &system.AdapterOperationFailureEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			Operation:   fmt.Sprintf("%v", adapterEvent.Metadata["operation"]),
			Error:       fmt.Sprintf("%v", adapterEvent.Metadata["error"]),
			ContextID:   fmt.Sprintf("%v", adapterEvent.Metadata["contextId"]),
		}
		
	case events.EventTypeWebhookReceived:
		return &system.WebhookReceivedEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			EventType:   fmt.Sprintf("%v", adapterEvent.Metadata["eventType"]),
			Payload:     adapterEvent.Payload,
			ContextID:   fmt.Sprintf("%v", adapterEvent.Metadata["contextId"]),
		}
		
	case events.EventTypeAdapterHealthChanged:
		return &system.AdapterHealthChangedEvent{
			BaseEvent:   baseEvent,
			AdapterType: adapterEvent.AdapterType,
			OldStatus:   fmt.Sprintf("%v", adapterEvent.Metadata["oldStatus"]),
			NewStatus:   fmt.Sprintf("%v", adapterEvent.Metadata["newStatus"]),
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
