package events

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"go.opentelemetry.io/otel/attribute"
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
	EventEmbeddingSearched EventType = "embedding.searched"
	
	// Tool events
	EventToolActionExecuted EventType = "tool.action.executed"
	EventToolDataQueried    EventType = "tool.data.queried"
	
	// Adapter events
	EventGitHubWebhookReceived EventType = "github.webhook.received"
	
	// Agent events
	EventAgentConnected    EventType = "agent.connected"
	EventAgentDisconnected EventType = "agent.disconnected"
	EventAgentError        EventType = "agent.error"
	
	// Session events
	EventSessionStarted EventType = "session.started"
	EventSessionEnded   EventType = "session.ended"
	
	// System events
	EventSystemStartup  EventType = "system.startup"
	EventSystemShutdown EventType = "system.shutdown"
	EventSystemHealthCheck EventType = "system.health_check"
)

// Handler is a function that handles an event
type Handler func(ctx context.Context, event *mcp.Event) error

// EventBusIface is an interface for the event bus, for testability.
type EventBusIface interface {
	Subscribe(eventType EventType, handler Handler)
	Unsubscribe(eventType EventType, handler Handler)
	Publish(ctx context.Context, event *mcp.Event)
	Close()
}

// EventBus is a central event hub that distributes events to registered handlers
type EventBus struct {
	handlers     map[EventType][]Handler
	handlerMutex sync.RWMutex
	
	// Asynchronous processing
	eventQueue    chan eventQueueItem
	metricsClient *observability.MetricsClient
	workers       int
}

// eventQueueItem represents an item in the event queue
type eventQueueItem struct {
	ctx   context.Context
	event *mcp.Event
}

// NewEventBus creates a new event bus
func NewEventBus(workers int) *EventBus {
	if workers <= 0 {
		workers = 4 // Default to 4 workers
	}
	
	bus := &EventBus{
		handlers:     make(map[EventType][]Handler),
		eventQueue:   make(chan eventQueueItem, 1000),
		metricsClient: observability.NewMetricsClient(),
		workers:      workers,
	}
	
	// Start worker goroutines
	for i := 0; i < workers; i++ {
		go bus.processEvents()
	}
	
	return bus
}

// Subscribe registers a handler for an event type
func (bus *EventBus) Subscribe(eventType EventType, handler Handler) {
	bus.handlerMutex.Lock()
	defer bus.handlerMutex.Unlock()
	
	bus.handlers[eventType] = append(bus.handlers[eventType], handler)
	log.Printf("Subscribed handler to event type: %s", eventType)
}

// SubscribeMultiple registers a handler for multiple event types
func (bus *EventBus) SubscribeMultiple(eventTypes []EventType, handler Handler) {
	for _, eventType := range eventTypes {
		bus.Subscribe(eventType, handler)
	}
}

// Unsubscribe removes a handler for an event type
func (bus *EventBus) Unsubscribe(eventType EventType, handler Handler) {
	bus.handlerMutex.Lock()
	defer bus.handlerMutex.Unlock()
	
	handlers := bus.handlers[eventType]
	for i, h := range handlers {
		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
			// Remove handler
			bus.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			log.Printf("Unsubscribed handler from event type: %s", eventType)
			return
		}
	}
}

// Publish publishes an event to all registered handlers
func (bus *EventBus) Publish(ctx context.Context, event *mcp.Event) {
	// Initialize timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	
	// Validate event
	if event.Type == "" {
		log.Printf("Warning: event published with empty type: %v", event)
		return
	}
	
	// Create trace span
	ctx, span := observability.StartSpan(ctx, "event.publish")
	span.SetAttributes(
		attribute.String("event.type", string(event.Type)),
		attribute.String("event.agent_id", event.AgentID),
		attribute.String("event.session_id", event.SessionID),
		attribute.String("event.source", event.Source),
	)
	defer span.End()
	
	// Queue event for async processing
	select {
	case bus.eventQueue <- eventQueueItem{ctx: ctx, event: event}:
		// Event queued successfully
	default:
		// Queue is full, log warning
		log.Printf("Warning: event queue is full, dropping event: %v", event)
		span.RecordError(fmt.Errorf("event queue full"))
	}
}

// processEvents processes events from the queue
func (bus *EventBus) processEvents() {
	for item := range bus.eventQueue {
		eventType := EventType(item.event.Type)
		
		// Get handlers
		bus.handlerMutex.RLock()
		handlers := bus.handlers[eventType]
		bus.handlerMutex.RUnlock()
		
		// Process event with each handler
		for _, handler := range handlers {
			startTime := time.Now()
			
			// Create trace span for handler
			ctx, span := observability.StartSpan(item.ctx, "event.handle")
			span.SetAttributes(
				attribute.String("event.type", string(eventType)),
				attribute.String("event.handler", fmt.Sprintf("%p", handler)),
			)
			
			// Call handler
			err := handler(ctx, item.event)
			
			// Record metrics
			_ = time.Since(startTime)
			// NOTE: Metrics recording is currently disabled
			
			// Handle error
			if err != nil {
				log.Printf("Error handling event %s: %v", eventType, err)
				span.RecordError(err)
			}
			
			span.End()
		}
	}
}

// Close shuts down the event bus
func (bus *EventBus) Close() {
	close(bus.eventQueue)
}

// PublishContextEvent publishes a context event
func PublishContextEvent(bus *EventBus, ctx context.Context, eventType EventType, contextID string, agentID string, modelID string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	
	// Add context ID to data
	data["context_id"] = contextID
	
	// Create event
	event := &mcp.Event{
		Type:      string(eventType),
		AgentID:   agentID,
		Timestamp: time.Now(),
		Data:      data,
		Source:    "mcp-server",
	}
	
	// Publish event
	bus.Publish(ctx, event)
}

// PublishToolEvent publishes a tool event
func PublishToolEvent(bus *EventBus, ctx context.Context, eventType EventType, tool string, action string, contextID string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	
	// Add tool and action to data
	data["tool"] = tool
	data["action"] = action
	data["context_id"] = contextID
	
	// Create event
	event := &mcp.Event{
		Type:      string(eventType),
		Timestamp: time.Now(),
		Data:      data,
		Source:    "mcp-server",
	}
	
	// Publish event
	bus.Publish(ctx, event)
}
