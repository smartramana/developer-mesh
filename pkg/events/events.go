package events

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// EventType defines the type of event
type EventType string

// Event types
const (
	// Context events
	EventContextCreated  EventType = "context.created"
	EventContextUpdated  EventType = "context.updated"
	EventContextDeleted  EventType = "context.deleted"
	EventContextRetrieved EventType = "context.retrieved"
	EventContextSummarized EventType = "context.summarized"
	EventContextTruncated EventType = "context.truncated"
	
	// Vector events
	EventEmbeddingStored  EventType = "embedding.stored"
	EventEmbeddingDeleted EventType = "embedding.deleted"
	
	// Tool events
	EventToolActionExecuted EventType = "tool.action.executed"
	EventToolActionFailed   EventType = "tool.action.failed"
	
	// Message events
	EventMessageSent     EventType = "message.sent"
	EventMessageReceived EventType = "message.received"
	
	// Session events
	EventSessionStarted EventType = "session.started"
	EventSessionEnded   EventType = "session.ended"
	
	// Agent events
	EventAgentConnected    EventType = "agent.connected"
	EventAgentDisconnected EventType = "agent.disconnected"
	EventAgentError        EventType = "agent.error"
	
	// System events
	EventSystemStartup    EventType = "system.startup"
	EventSystemShutdown   EventType = "system.shutdown"
	EventSystemHealthCheck EventType = "system.health_check"
)

// Event represents a system event
type Event interface {
	// GetType returns the event type
	GetType() string
}

// Handler is a function that processes an event
type Handler func(ctx context.Context, event *models.Event) error

// EventBusIface is the interface for event bus implementations
type EventBus interface {
	// Publish publishes an event
	Publish(ctx context.Context, event *models.Event)

	// Subscribe subscribes to events of a specific type
	Subscribe(eventType EventType, handler Handler)

	// Unsubscribe unsubscribes from events of a specific type
	Unsubscribe(eventType EventType, handler Handler)

	// Close closes the event bus
	Close()
}
