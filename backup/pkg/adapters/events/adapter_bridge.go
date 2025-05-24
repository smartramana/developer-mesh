package events

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// EventBusAdapter adapts the legacy event bus interface to the new event system
type EventBusAdapter struct {
	legacyBus *EventBusImpl
	logger    observability.Logger
}

// NewEventBusAdapter creates a new adapter between legacy event bus and new event system
func NewEventBusAdapter(logger observability.Logger) *EventBusAdapter {
	return &EventBusAdapter{
		legacyBus: NewEventBus(logger),
		logger:    logger,
	}
}

// Subscribe adds a new event handler for a specific event type
func (a *EventBusAdapter) Subscribe(eventType events.EventType, handler events.Handler) {
	// Convert to legacy event type and handler
	legacyHandler := func(ctx context.Context, event *LegacyAdapterEvent) error {
		// Map legacy event to new event format
		systemEvent := mapToSystemEvent(event)
		return handler.Handle(ctx, systemEvent)
	}
	
	// Create a LegacyEventListener from the handler function
	listener := LegacyEventListenerFunc(legacyHandler)
	
	// Subscribe to all events since we'll filter by type
	a.legacyBus.SubscribeAll(listener)
}

// Unsubscribe removes an event handler for a specific event type
func (a *EventBusAdapter) Unsubscribe(eventType events.EventType, handler events.Handler) {
	// Not fully implemented, but we acknowledge the call
	a.logger.Debug("Unsubscribe called on EventBusAdapter", map[string]interface{}{
		"eventType": string(eventType),
	})
}

// Publish publishes an event to the event bus
func (a *EventBusAdapter) Publish(ctx context.Context, event *models.Event) {
	// Convert system event to legacy event
	legacyEvent := &LegacyAdapterEvent{
		AdapterType: event.Source,
		EventType:   mapSystemEventTypeToLegacy(event.Type),
		Payload:     event.Payload,
		Timestamp:   event.Timestamp,
		Metadata:    event.Metadata,
	}
	
	// Emit through legacy event bus
	if err := a.legacyBus.Emit(ctx, legacyEvent); err != nil {
		a.logger.Warn("Error emitting legacy event", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   string(legacyEvent.EventType),
			"adapterType": legacyEvent.AdapterType,
		})
	}
}

// Close implements the EventBus interface
func (a *EventBusAdapter) Close() error {
	// No cleanup needed
	return nil
}

// mapSystemEventTypeToLegacy maps a system event type to a legacy event type
func mapSystemEventTypeToLegacy(eventType events.EventType) LegacyEventType {
	switch eventType {
	case events.EventTypeSuccess:
		return LegacyEventTypeSuccess
	case events.EventTypeFailure:
		return LegacyEventTypeFailure
	case events.EventTypeWebhookReceived:
		return LegacyEventTypeWebhookReceived
	case events.EventTypeHealthChange:
		return LegacyEventTypeHealthChange
	default:
		return LegacyEventTypeUnknown
	}
}

// mapToSystemEvent maps a legacy adapter event to a system event
func mapToSystemEvent(legacyEvent *LegacyAdapterEvent) *models.Event {
	eventType := mapLegacyEventTypeToSystem(legacyEvent.EventType)
	
	return &models.Event{
		Type:      eventType,
		Source:    legacyEvent.AdapterType,
		Payload:   legacyEvent.Payload,
		Timestamp: legacyEvent.Timestamp,
		Metadata:  legacyEvent.Metadata,
	}
}

// mapLegacyEventTypeToSystem maps a legacy event type to a system event type
func mapLegacyEventTypeToSystem(eventType LegacyEventType) events.EventType {
	switch eventType {
	case LegacyEventTypeSuccess:
		return events.EventTypeSuccess
	case LegacyEventTypeFailure:
		return events.EventTypeFailure
	case LegacyEventTypeWebhookReceived:
		return events.EventTypeWebhookReceived
	case LegacyEventTypeHealthChange:
		return events.EventTypeHealthChange
	default:
		return events.EventTypeUnknown
	}
}
