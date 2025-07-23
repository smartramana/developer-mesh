# Events Package

> **Purpose**: Event-driven architecture and pub/sub infrastructure for the DevOps MCP platform
> **Status**: Basic Implementation
> **Dependencies**: In-memory event bus, basic domain events

**Note**: This README describes an aspirational event-driven architecture. The actual implementation is much simpler with only basic in-memory event publishing.

## Overview

The events package provides a basic event system for the DevOps MCP platform. Currently, only an in-memory event bus is implemented (`EventBusImpl`), which provides simple publish/subscribe functionality.

**Implemented Features**:
- Basic in-memory event bus
- Simple publish/subscribe pattern
- Domain event structures
- Basic event interfaces

**Not Yet Implemented** (but documented below):
- Event sourcing and event store
- Redis event bus
- WebSocket integration  
- Sagas and process managers
- Event persistence
- Distributed event handling

## Current Architecture (Actual Implementation)

```
┌─────────────────────────────────────────────────────────────┐
│                  Simple Event System                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Publishers ──► EventBusImpl ──► Handlers (async)          │
│                  (in-memory)                                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Planned Architecture (Not Yet Implemented)

```
┌─────────────────────────────────────────────────────────────┐
│                     Event Architecture                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Publishers ──► Event Bus ──► Event Store ──► Subscribers  │
│                     │             │                         │
│                     ├─────────────┴─────────────┐           │
│                     │                           │           │
│                  Redis         WebSocket    In-Memory       │
│                 PubSub         Broadcast      Events        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Actual Implementation

### EventBusImpl (event_bus_impl.go)

```go
// Simple in-memory event bus
type EventBusImpl struct {
    handlers     map[EventType][]Handler
    mutex        sync.RWMutex
    maxQueueSize int
    queue        chan *models.Event
}

// Basic publish - calls handlers asynchronously
func (b *EventBusImpl) Publish(ctx context.Context, event *models.Event) {
    handlers := b.handlers[EventType(event.Type)]
    for _, handler := range handlers {
        go handler(ctx, event)
    }
}

// Subscribe to event type
func (b *EventBusImpl) Subscribe(eventType EventType, handler Handler)
```

### Domain Event Structure (interfaces.go)

```go
type DomainEvent struct {
    ID            uuid.UUID
    Type          string
    AggregateID   uuid.UUID
    AggregateType string
    Version       int
    Timestamp     time.Time
    Data          interface{}
    Metadata      Metadata
}
```

## Planned Components (Documentation for Future Implementation)

### 1. Event Interface

```go
// Event represents a domain event
type Event interface {
    // ID returns unique event identifier
    ID() string
    
    // Type returns event type for routing
    Type() string
    
    // Timestamp returns when event occurred
    Timestamp() time.Time
    
    // AggregateID returns the aggregate this event belongs to
    AggregateID() string
    
    // Version returns event schema version
    Version() int
    
    // Data returns event payload
    Data() interface{}
    
    // Metadata returns event metadata (user, correlation ID, etc)
    Metadata() map[string]string
}
```

### 2. Event Bus

```go
// EventBus handles event publishing and subscription
type EventBus interface {
    // Publish sends an event to all subscribers
    Publish(ctx context.Context, event Event) error
    
    // Subscribe registers a handler for event types
    Subscribe(eventType string, handler EventHandler) error
    
    // Unsubscribe removes a handler
    Unsubscribe(eventType string, handler EventHandler) error
    
    // Start begins processing events
    Start(ctx context.Context) error
    
    // Stop gracefully shuts down event processing
    Stop(ctx context.Context) error
}
```

### 3. Event Store

```go
// EventStore persists events for replay and audit
type EventStore interface {
    // Append adds events to the store
    Append(ctx context.Context, events ...Event) error
    
    // Load retrieves events for an aggregate
    Load(ctx context.Context, aggregateID string, fromVersion int) ([]Event, error)
    
    // LoadSnapshot retrieves latest snapshot
    LoadSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error)
    
    // SaveSnapshot stores aggregate snapshot
    SaveSnapshot(ctx context.Context, snapshot *Snapshot) error
    
    // Query retrieves events by criteria
    Query(ctx context.Context, query EventQuery) ([]Event, error)
}
```

## Domain Events

### Base Event Implementation

```go
// BaseEvent provides common event functionality
type BaseEvent struct {
    EventID      string            `json:"event_id"`
    EventType    string            `json:"event_type"`
    EventTime    time.Time         `json:"event_time"`
    AggregateID  string            `json:"aggregate_id"`
    EventVersion int               `json:"event_version"`
    EventData    interface{}       `json:"event_data"`
    EventMeta    map[string]string `json:"event_metadata"`
}

// NewBaseEvent creates a new domain event
func NewBaseEvent(eventType string, aggregateID string, data interface{}) *BaseEvent {
    return &BaseEvent{
        EventID:      uuid.New().String(),
        EventType:    eventType,
        EventTime:    time.Now().UTC(),
        AggregateID:  aggregateID,
        EventVersion: 1,
        EventData:    data,
        EventMeta:    make(map[string]string),
    }
}
```

### Common Domain Events

```go
// Context Events
type ContextCreatedEvent struct {
    *BaseEvent
    ContextID   string    `json:"context_id"`
    AgentID     string    `json:"agent_id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedBy   string    `json:"created_by"`
}

type ContextUpdatedEvent struct {
    *BaseEvent
    ContextID string                 `json:"context_id"`
    Changes   map[string]interface{} `json:"changes"`
    UpdatedBy string                 `json:"updated_by"`
}

// Agent Events
type AgentRegisteredEvent struct {
    *BaseEvent
    AgentID      string   `json:"agent_id"`
    Model        string   `json:"model"`
    Capabilities []string `json:"capabilities"`
    Version      string   `json:"version"`
}

type AgentDisconnectedEvent struct {
    *BaseEvent
    AgentID    string `json:"agent_id"`
    Reason     string `json:"reason"`
    LastSeen   time.Time `json:"last_seen"`
}

// Task Events
type TaskCreatedEvent struct {
    *BaseEvent
    TaskID      string `json:"task_id"`
    TaskType    string `json:"task_type"`
    Priority    int    `json:"priority"`
    AssignedTo  string `json:"assigned_to"`
}

type TaskCompletedEvent struct {
    *BaseEvent
    TaskID     string      `json:"task_id"`
    Result     interface{} `json:"result"`
    Duration   time.Duration `json:"duration"`
    CompletedBy string     `json:"completed_by"`
}

// Collaboration Events
type CollaborationStartedEvent struct {
    *BaseEvent
    SessionID    string   `json:"session_id"`
    Participants []string `json:"participants"`
    Strategy     string   `json:"strategy"`
}

type CollaborationMessageEvent struct {
    *BaseEvent
    SessionID   string `json:"session_id"`
    FromAgent   string `json:"from_agent"`
    ToAgent     string `json:"to_agent"`
    MessageType string `json:"message_type"`
    Content     interface{} `json:"content"`
}
```

## Event Handlers

### Handler Interface

```go
// EventHandler processes events
type EventHandler interface {
    // Handle processes an event
    Handle(ctx context.Context, event Event) error
    
    // HandlerID returns unique handler identifier
    HandlerID() string
}

// EventHandlerFunc is a function adapter for EventHandler
type EventHandlerFunc func(ctx context.Context, event Event) error

func (f EventHandlerFunc) Handle(ctx context.Context, event Event) error {
    return f(ctx, event)
}

func (f EventHandlerFunc) HandlerID() string {
    return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}
```

### Handler Examples

```go
// WebSocket broadcast handler
func NewWebSocketBroadcastHandler(hub *WebSocketHub) EventHandler {
    return EventHandlerFunc(func(ctx context.Context, event Event) error {
        // Convert to WebSocket message
        msg := &WebSocketMessage{
            Type:      "event",
            EventType: event.Type(),
            Payload:   event.Data(),
            Timestamp: event.Timestamp(),
        }
        
        // Broadcast to connected clients
        return hub.Broadcast(ctx, msg)
    })
}

// Task queue handler
func NewTaskQueueHandler(queue Queue) EventHandler {
    return EventHandlerFunc(func(ctx context.Context, event Event) error {
        switch e := event.(type) {
        case *TaskCreatedEvent:
            // Queue task for processing
            return queue.Enqueue(ctx, &Task{
                ID:       e.TaskID,
                Type:     e.TaskType,
                Priority: e.Priority,
            })
        }
        return nil
    })
}

// Notification handler
func NewNotificationHandler(notifier Notifier) EventHandler {
    return EventHandlerFunc(func(ctx context.Context, event Event) error {
        // Send notifications for important events
        switch event.Type() {
        case "agent.disconnected", "task.failed", "collaboration.error":
            return notifier.Notify(ctx, event)
        }
        return nil
    })
}
```

## Usage Examples

### Current Implementation Usage

```go
// Create event bus
eventBus := events.NewEventBus(1000)

// Subscribe to events
eventBus.Subscribe("task.created", func(ctx context.Context, event *models.Event) error {
    // Handle event
    fmt.Printf("Task created: %v\n", event.Data)
    return nil
})

// Publish event
event := &models.Event{
    Type: "task.created",
    Data: map[string]interface{}{
        "task_id": "123",
        "name":    "Process data",
    },
}
eventBus.Publish(ctx, event)
```

## Planned Event Bus Implementations (Not Yet Implemented)

### 2. Redis Event Bus

```go
// RedisEventBus for distributed deployments
type RedisEventBus struct {
    client      *redis.Client
    subscribers map[string][]EventHandler
    mu          sync.RWMutex
    serializer  EventSerializer
}

func NewRedisEventBus(client *redis.Client) *RedisEventBus {
    return &RedisEventBus{
        client:      client,
        subscribers: make(map[string][]EventHandler),
        serializer:  &JSONEventSerializer{},
    }
}

func (b *RedisEventBus) Publish(ctx context.Context, event Event) error {
    // Serialize event
    data, err := b.serializer.Serialize(event)
    if err != nil {
        return fmt.Errorf("serialize event: %w", err)
    }
    
    // Publish to Redis channel
    channel := fmt.Sprintf("events:%s", event.Type())
    return b.client.Publish(ctx, channel, data).Err()
}

func (b *RedisEventBus) Start(ctx context.Context) error {
    // Subscribe to Redis channels
    pubsub := b.client.PSubscribe(ctx, "events:*")
    ch := pubsub.Channel()
    
    go func() {
        for msg := range ch {
            // Deserialize event
            event, err := b.serializer.Deserialize([]byte(msg.Payload))
            if err != nil {
                logger.Error("deserialize event", "error", err)
                continue
            }
            
            // Dispatch to handlers
            b.dispatch(ctx, event)
        }
    }()
    
    return nil
}
```

### 3. Hybrid Event Bus

```go
// HybridEventBus combines multiple transports
type HybridEventBus struct {
    primary   EventBus  // For critical events
    secondary EventBus  // For best-effort delivery
    fallback  EventBus  // For resilience
}

func (h *HybridEventBus) Publish(ctx context.Context, event Event) error {
    // Try primary first
    if err := h.primary.Publish(ctx, event); err == nil {
        return nil
    }
    
    // Fallback to secondary
    if err := h.secondary.Publish(ctx, event); err == nil {
        return nil
    }
    
    // Last resort fallback
    return h.fallback.Publish(ctx, event)
}
```

## Event Sourcing

### Event-Sourced Aggregate

```go
// EventSourcedAggregate base for event-sourced entities
type EventSourcedAggregate struct {
    ID            string
    Version       int
    UncommittedEvents []Event
}

func (a *EventSourcedAggregate) ApplyEvent(event Event) {
    a.Version++
    a.UncommittedEvents = append(a.UncommittedEvents, event)
}

func (a *EventSourcedAggregate) MarkEventsAsCommitted() {
    a.UncommittedEvents = []Event{}
}

// Context aggregate example
type Context struct {
    EventSourcedAggregate
    Name        string
    Description string
    AgentID     string
    State       string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

func (c *Context) Create(name, description, agentID string) error {
    if c.Version != 0 {
        return ErrAggregateAlreadyExists
    }
    
    event := &ContextCreatedEvent{
        BaseEvent:   NewBaseEvent("context.created", c.ID, nil),
        ContextID:   c.ID,
        Name:        name,
        Description: description,
        AgentID:     agentID,
    }
    
    c.Apply(event)
    return nil
}

func (c *Context) Apply(event Event) {
    switch e := event.(type) {
    case *ContextCreatedEvent:
        c.Name = e.Name
        c.Description = e.Description
        c.AgentID = e.AgentID
        c.State = "active"
        c.CreatedAt = e.Timestamp()
        
    case *ContextUpdatedEvent:
        for field, value := range e.Changes {
            switch field {
            case "name":
                c.Name = value.(string)
            case "description":
                c.Description = value.(string)
            }
        }
        c.UpdatedAt = e.Timestamp()
    }
    
    c.ApplyEvent(event)
}
```

### Event Store Repository

```go
// EventStoreRepository loads and saves aggregates
type EventStoreRepository struct {
    store EventStore
    bus   EventBus
}

func (r *EventStoreRepository) Load(ctx context.Context, aggregateID string) (*Context, error) {
    // Load snapshot if available
    snapshot, err := r.store.LoadSnapshot(ctx, aggregateID)
    if err != nil && !errors.Is(err, ErrSnapshotNotFound) {
        return nil, err
    }
    
    // Create aggregate
    context := &Context{}
    if snapshot != nil {
        context = snapshot.Data.(*Context)
    }
    
    // Load and apply events
    events, err := r.store.Load(ctx, aggregateID, context.Version)
    if err != nil {
        return nil, err
    }
    
    for _, event := range events {
        context.Apply(event)
    }
    
    return context, nil
}

func (r *EventStoreRepository) Save(ctx context.Context, aggregate *Context) error {
    // Append uncommitted events
    if err := r.store.Append(ctx, aggregate.UncommittedEvents...); err != nil {
        return err
    }
    
    // Publish events
    for _, event := range aggregate.UncommittedEvents {
        if err := r.bus.Publish(ctx, event); err != nil {
            logger.Error("publish event failed", "error", err)
        }
    }
    
    // Create snapshot periodically
    if aggregate.Version%10 == 0 {
        snapshot := &Snapshot{
            AggregateID: aggregate.ID,
            Version:     aggregate.Version,
            Data:        aggregate,
            Timestamp:   time.Now(),
        }
        r.store.SaveSnapshot(ctx, snapshot)
    }
    
    aggregate.MarkEventsAsCommitted()
    return nil
}
```

## Event Patterns

### 1. Saga Pattern

```go
// Saga coordinates long-running transactions
type Saga struct {
    ID        string
    State     string
    Steps     []SagaStep
    CurrentStep int
}

type SagaStep interface {
    Execute(ctx context.Context) error
    Compensate(ctx context.Context) error
}

// Task processing saga example
type TaskProcessingSaga struct {
    Saga
    taskService   TaskService
    agentService  AgentService
    eventBus      EventBus
}

func (s *TaskProcessingSaga) Handle(ctx context.Context, event Event) error {
    switch e := event.(type) {
    case *TaskCreatedEvent:
        return s.start(ctx, e)
    case *AgentAssignedEvent:
        return s.processAssignment(ctx, e)
    case *TaskCompletedEvent:
        return s.complete(ctx, e)
    case *TaskFailedEvent:
        return s.compensate(ctx, e)
    }
    return nil
}
```

### 2. Process Manager Pattern

```go
// ProcessManager coordinates multiple aggregates
type CollaborationProcessManager struct {
    eventStore EventStore
    eventBus   EventBus
    sessions   map[string]*CollaborationSession
}

func (pm *CollaborationProcessManager) Handle(ctx context.Context, event Event) error {
    switch e := event.(type) {
    case *CollaborationRequestedEvent:
        // Start new collaboration
        session := NewCollaborationSession(e.Agents, e.Strategy)
        pm.sessions[session.ID] = session
        
        // Notify agents
        for _, agentID := range e.Agents {
            pm.eventBus.Publish(ctx, &AgentInvitedEvent{
                SessionID: session.ID,
                AgentID:   agentID,
            })
        }
        
    case *AgentJoinedEvent:
        session := pm.sessions[e.SessionID]
        session.AddAgent(e.AgentID)
        
        // Check if all agents joined
        if session.AllAgentsJoined() {
            pm.eventBus.Publish(ctx, &CollaborationStartedEvent{
                SessionID: session.ID,
            })
        }
    }
    
    return nil
}
```

### 3. Event Choreography

```go
// Choreography - services react to events independently
func SetupEventChoreography(bus EventBus) {
    // Agent service reacts to task events
    bus.Subscribe("task.created", EventHandlerFunc(func(ctx context.Context, event Event) error {
        e := event.(*TaskCreatedEvent)
        // Find suitable agent
        agent := findBestAgent(e.TaskType)
        
        // Publish assignment
        return bus.Publish(ctx, &AgentAssignedEvent{
            TaskID:  e.TaskID,
            AgentID: agent.ID,
        })
    }))
    
    // Notification service reacts to failures
    bus.Subscribe("task.failed", EventHandlerFunc(func(ctx context.Context, event Event) error {
        e := event.(*TaskFailedEvent)
        // Send notifications
        return notifyAdmins(e)
    }))
    
    // Analytics service reacts to all events
    bus.Subscribe("*", EventHandlerFunc(func(ctx context.Context, event Event) error {
        // Update metrics
        return updateMetrics(event)
    }))
}
```

## WebSocket Integration

```go
// WebSocketEventBridge connects events to WebSocket clients
type WebSocketEventBridge struct {
    eventBus EventBus
    wsHub    *WebSocketHub
}

func (b *WebSocketEventBridge) Setup() {
    // Forward specific events to WebSocket clients
    eventTypes := []string{
        "agent.registered",
        "agent.disconnected",
        "task.created",
        "task.completed",
        "collaboration.message",
    }
    
    for _, eventType := range eventTypes {
        b.eventBus.Subscribe(eventType, EventHandlerFunc(func(ctx context.Context, event Event) error {
            // Filter by client subscriptions
            clients := b.wsHub.GetSubscribedClients(event.Type())
            
            // Convert to WebSocket message
            msg := &WebSocketMessage{
                Type:      "event",
                EventType: event.Type(),
                EventID:   event.ID(),
                Payload:   event.Data(),
                Metadata:  event.Metadata(),
            }
            
            // Send to subscribed clients
            for _, client := range clients {
                client.Send(msg)
            }
            
            return nil
        }))
    }
}
```

## Monitoring & Observability

### Event Metrics

```go
var (
    eventsPublished = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_events_published_total",
            Help: "Total number of events published",
        },
        []string{"event_type"},
    )
    
    eventsProcessed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_events_processed_total",
            Help: "Total number of events processed",
        },
        []string{"event_type", "handler", "status"},
    )
    
    eventProcessingDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_event_processing_duration_seconds",
            Help:    "Event processing duration",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
        },
        []string{"event_type", "handler"},
    )
    
    eventQueueDepth = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_event_queue_depth",
            Help: "Current event queue depth",
        },
        []string{"queue_name"},
    )
)
```

### Event Tracing

```go
// TracedEventHandler adds OpenTelemetry tracing
type TracedEventHandler struct {
    handler EventHandler
    tracer  trace.Tracer
}

func (h *TracedEventHandler) Handle(ctx context.Context, event Event) error {
    ctx, span := h.tracer.Start(ctx, fmt.Sprintf("event.%s", event.Type()),
        trace.WithAttributes(
            attribute.String("event.id", event.ID()),
            attribute.String("event.type", event.Type()),
            attribute.String("aggregate.id", event.AggregateID()),
        ),
    )
    defer span.End()
    
    start := time.Now()
    err := h.handler.Handle(ctx, event)
    duration := time.Since(start)
    
    // Update metrics
    status := "success"
    if err != nil {
        status = "error"
        span.RecordError(err)
    }
    
    eventsProcessed.WithLabelValues(event.Type(), h.handler.HandlerID(), status).Inc()
    eventProcessingDuration.WithLabelValues(event.Type(), h.handler.HandlerID()).Observe(duration.Seconds())
    
    return err
}
```

## Configuration

### Environment Variables

```bash
# Event Bus
EVENT_BUS_TYPE=redis          # redis, memory, hybrid
EVENT_BUS_BUFFER_SIZE=10000
EVENT_BUS_WORKERS=10

# Redis Event Bus
REDIS_ADDR=127.0.0.1:6379
REDIS_EVENT_CHANNEL_PREFIX=mcp:events

# Event Store
EVENT_STORE_TYPE=postgres
EVENT_STORE_SNAPSHOT_FREQUENCY=10
EVENT_STORE_RETENTION_DAYS=90

# Event Processing
EVENT_PROCESSING_BATCH_SIZE=100
EVENT_PROCESSING_TIMEOUT=30s
```

### Configuration File

```yaml
events:
  bus:
    type: redis
    buffer_size: 10000
    workers: 10
    
  store:
    type: postgres
    snapshot_frequency: 10
    retention_days: 90
    
  processing:
    batch_size: 100
    timeout: 30s
    retry:
      max_attempts: 3
      backoff: exponential
      
  subscriptions:
    - event_type: "*.created"
      handlers:
        - websocket_broadcast
        - audit_logger
        
    - event_type: "task.*"
      handlers:
        - task_processor
        - metrics_collector
```

## Implementation Status Summary

### ✅ Implemented
- Basic EventBusImpl with in-memory publish/subscribe
- DomainEvent structure with metadata
- EventStore and EventPublisher interfaces
- Simple async handler execution
- Basic event types in models package

### ❌ Not Implemented (Documented Above)
- Event sourcing and aggregate patterns
- Redis event bus
- Event persistence/store
- WebSocket integration
- Sagas and process managers
- Event choreography patterns
- Monitoring and metrics
- Dead letter queues
- Event replay functionality

## Best Practices (For Current Implementation)

1. **Error Handling**: Handlers should not panic as they run in goroutines
2. **Context Usage**: Always pass context to handlers for cancellation
3. **Memory Usage**: Be aware that all events are in-memory
4. **Testing**: Use the simple EventBusImpl for unit tests
5. **Async Nature**: Remember handlers execute asynchronously

## Future Development

To implement the full event-driven architecture described in this README:

1. Add Redis event bus implementation for distributed systems
2. Implement event store with PostgreSQL
3. Add WebSocket event bridge
4. Implement saga orchestration
5. Add event sourcing aggregates
6. Integrate with monitoring/metrics

---

Package Version: 0.1.0 (Basic Implementation)
Last Updated: 2024-01-23