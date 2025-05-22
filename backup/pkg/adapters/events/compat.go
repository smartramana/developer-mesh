// Package events provides adapter functionality for connecting to the system event bus.
// This file implements a compatibility layer between pkg/adapters/events and pkg/events
// following the pattern used successfully in the AWS package migration.
package events

import (
	"context"
	"time"

	corevents "github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// ----- Type Aliases for Core Types -----

// CoreEventType is an alias for events.EventType to prevent redeclarations
type CoreEventType = corevents.EventType

// CoreHandler is an alias for events.Handler to prevent redeclarations
type CoreHandler = corevents.Handler

// CoreEventBus is an alias for events.EventBusIface to prevent redeclarations
type CoreEventBus = corevents.EventBusIface

// ----- Conversion Functions -----

// LegacyToCore converts a legacy event type to a core event type
func LegacyToCore(eventType LegacyEventType) CoreEventType {
	return CoreEventType(string(eventType))
}

// CoreToLegacy converts a core event type to a legacy event type
func CoreToLegacy(eventType CoreEventType) LegacyEventType {
	return LegacyEventType(string(eventType))
}

// ----- Bridge Adapter -----

// EventBusBridge adapts a Core EventBus to the adapter EventBus interface
type EventBusBridge struct {
	core   CoreEventBus
	logger observability.Logger
}

// NewEventBusBridge creates a new EventBusBridge
func NewEventBusBridge(core CoreEventBus, logger observability.Logger) *EventBusBridge {
	return &EventBusBridge{
		core:   core,
		logger: logger,
	}
}

// Publish implements the EventBus.Publish method
func (b *EventBusBridge) Publish(ctx context.Context, event *mcp.Event) {
	if b.core != nil {
		b.core.Publish(ctx, event)
	}
}

// Subscribe implements bridge from adapter EventType to core EventType
func (b *EventBusBridge) Subscribe(eventType LegacyEventType, listener LegacyEventListener) {
	if b.core == nil {
		return
	}

	// Create a handler function that adapts between the core and adapter interfaces
	handler := CoreHandler(func(ctx context.Context, event *mcp.Event) error {
		legacyEvent := &LegacyAdapterEvent{
			AdapterType: "bridge",
			EventType:   eventType,
			Payload:     event.Data,
			Timestamp:   time.Now().Unix(),
			Metadata:    make(map[string]interface{}),
		}

		// Call the listener with the legacy adapter event
		err := listener.Handle(ctx, legacyEvent)
		return err
	})

	// Subscribe to the core event bus with the adapter handler
	b.core.Subscribe(LegacyToCore(eventType), handler)
}

// SubscribeAll subscribes a listener to all events 
func (b *EventBusBridge) SubscribeAll(listener LegacyEventListener) {
	if b.core == nil {
		return
	}

	// Create a handler function that adapts between the core and adapter interfaces
	handler := CoreHandler(func(ctx context.Context, event *mcp.Event) error {
		legacyEvent := &LegacyAdapterEvent{
			AdapterType: "bridge",
			EventType:   "unknown", // We don't know the specific type for wildcard handlers
			Payload:     event.Data,
			Timestamp:   time.Now().Unix(),
			Metadata:    make(map[string]interface{}),
		}

		// Call the listener with the legacy adapter event
		err := listener.Handle(ctx, legacyEvent)
		return err
	})

	// Subscribe to all events on the core event bus
	b.core.Subscribe("*", handler)
}

// Close implements the EventBus.Close method
func (b *EventBusBridge) Close() error {
	if b.core != nil {
		b.core.Close()
	}
	return nil
}

// ----- Utility Functions -----

// CreateBridgeEvent creates a new legacy adapter event with the given parameters
func CreateBridgeEvent(adapterType string, eventType LegacyEventType, payload interface{}) *LegacyAdapterEvent {
	return &LegacyAdapterEvent{
		AdapterType: adapterType,
		EventType:   eventType,
		Payload:     payload,
		Timestamp:   time.Now().Unix(),
		Metadata:    make(map[string]interface{}),
	}
}

// TimeToUnixTimestamp converts a time.Time to a Unix timestamp
func TimeToUnixTimestamp(t time.Time) int64 {
	return t.Unix()
}

// UnixTimestampToTime converts a Unix timestamp to a time.Time
func UnixTimestampToTime(ts int64) time.Time {
	return time.Unix(ts, 0)
}

// ----- Core Event Conversion Functions -----

// ConvertLegacyToCoreEvent converts a LegacyAdapterEvent to an mcp.Event
func ConvertLegacyToCoreEvent(event *LegacyAdapterEvent) *mcp.Event {
	return &mcp.Event{
		Source:    event.AdapterType,
		Type:      string(event.EventType),
		Timestamp: UnixTimestampToTime(event.Timestamp),
		Data:      event.Payload,
		AgentID:   "", // Empty default value
		SessionID: "", // Empty default value
	}
}

// ConvertCoreToLegacyEvent converts an mcp.Event to a LegacyAdapterEvent
func ConvertCoreToLegacyEvent(event *mcp.Event) *LegacyAdapterEvent {
	return &LegacyAdapterEvent{
		AdapterType: event.Source,
		EventType:   LegacyEventType(event.Type),
		Payload:     event.Data,
		Timestamp:   TimeToUnixTimestamp(event.Timestamp),
		Metadata: map[string]interface{}{
			"agentId":   event.AgentID,
			"sessionId": event.SessionID,
		},
	}
}
