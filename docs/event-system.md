# MCP Server Event System

This document describes the event system in the MCP Server, which is the core mechanism for handling and processing events from various integrated systems.

## Event Model

The MCP Server uses a unified event model to represent events from different sources. The core event type is defined in `pkg/mcp/event.go`:

```go
// Event represents an MCP event
type Event struct {
    // Source is the source of the event (e.g., github, harness, sonarqube)
    Source string `json:"source"`

    // Type is the type of the event (e.g., pull_request, deployment)
    Type string `json:"type"`

    // Timestamp is when the event occurred
    Timestamp time.Time `json:"timestamp"`

    // Data contains the event data
    Data interface{} `json:"data"`
}
```

### Event Sources

Event sources represent the external system that generated the event:

- `github`: Events from GitHub repositories
- `harness`: Events from Harness CI/CD and feature flags
- `sonarqube`: Events from SonarQube code analysis
- `artifactory`: Events from JFrog Artifactory
- `xray`: Events from JFrog Xray

### Event Types

Event types are specific to each source and represent the kind of event that occurred:

#### GitHub Event Types
- `pull_request`: Pull request created, updated, closed, etc.
- `push`: Code pushed to a repository

#### Harness Event Types
- `ci.build`: CI build started, completed, failed, etc.
- `cd.deployment`: CD deployment started, completed, failed, etc.
- `sto.experiment`: STO experiment started, completed, failed, etc.
- `ff.change`: Feature flag created, updated, toggled, etc.

#### SonarQube Event Types
- `quality_gate`: Quality gate status changed
- `task_completed`: Analysis task completed

#### Artifactory Event Types
- `artifact_created`: Artifact created or deployed
- `artifact_deleted`: Artifact deleted
- `artifact_property_changed`: Artifact property modified

#### Xray Event Types
- `security_violation`: Security vulnerability detected
- `license_violation`: License violation detected
- `scan_completed`: Scan completed

### Event Data

The `Data` field contains the specific payload for the event, which varies depending on the source and type. The data is typically represented using Go structs that mirror the external system's data model.

## Event Flow

Events flow through the MCP Server in the following way:

1. **Event Creation**: Events are created from:
   - Webhook payloads received by the API Server
   - Polling adapters that detect changes
   - Internal operations that need to signal events

2. **Event Validation**: Events are validated to ensure they contain required fields and valid data.

3. **Event Queuing**: Validated events are placed in a buffered channel in the Core Engine.

4. **Event Processing**: Worker goroutines in the Core Engine pick up events for processing.

5. **Handler Dispatch**: Events are dispatched to handlers based on their source and type.

6. **Handler Execution**: Handlers process the event and may trigger further actions.

7. **Event Storage**: Processed events are optionally stored for auditing and debugging.

## Event Processing

### Concurrent Processing

The MCP Server processes events concurrently using a configurable number of worker goroutines:

```go
// startEventProcessors starts multiple goroutines for event processing
func (e *Engine) startEventProcessors(count int) {
    for i := 0; i < count; i++ {
        e.wg.Add(1)
        go func() {
            defer e.wg.Done()
            for {
                select {
                case event, ok := <-e.events:
                    if !ok {
                        return
                    }
                    e.processEvent(event)
                case <-e.ctx.Done():
                    return
                }
            }
        }()
    }
}
```

The number of worker goroutines is configurable via the `concurrency_limit` parameter in the engine configuration.

### Event Timeouts

Each event has a configurable timeout to prevent long-running event processing from blocking the system:

```go
// processEvent processes a single event with appropriate error handling
func (e *Engine) processEvent(event mcp.Event) {
    // Create a context with timeout for event processing
    ctx, cancel := context.WithTimeout(e.ctx, e.config.EventTimeout)
    defer cancel()

    // Log the event
    log.Printf("Processing %s event from %s", event.Type, event.Source)

    // Record metrics
    e.metricsClient.RecordEvent(event.Source, event.Type)

    // Process based on event source and type
    switch event.Source {
    case "github":
        e.processGithubEvent(ctx, event)
    case "harness":
        e.processHarnessEvent(ctx, event)
    case "sonarqube":
        e.processSonarQubeEvent(ctx, event)
    case "artifactory":
        e.processArtifactoryEvent(ctx, event)
    case "xray":
        e.processXrayEvent(ctx, event)
    default:
        log.Printf("Unknown event source: %s", event.Source)
    }
}
```

The timeout is configurable via the `event_timeout` parameter in the engine configuration.

### Error Handling

Events are processed with comprehensive error handling to ensure that failures in one event don't affect others:

- Each event is processed in its own goroutine
- Errors are logged and recorded in metrics
- Retryable errors may trigger a retry mechanism
- Non-retryable errors are logged and the event is considered processed

## Event Subscription

The MCP Server uses a subscription-based pattern to handle events. Components can subscribe to specific event types and will be notified when those events occur.

### Subscription Model

Each adapter implements a subscription mechanism:

```go
// Subscribe adds a callback for a specific event type
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
    a.subscriberMu.Lock()
    defer a.subscriberMu.Unlock()

    a.subscribers[eventType] = append(a.subscribers[eventType], callback)
    return nil
}

// notifySubscribers notifies subscribers of an event
func (a *Adapter) notifySubscribers(eventType string, event interface{}) {
    a.subscriberMu.RLock()
    defer a.subscriberMu.RUnlock()

    for _, callback := range a.subscribers[eventType] {
        go callback(event)
    }
}
```

### Subscribing to Events

The Core Engine subscribes to events during adapter initialization:

```go
// setupGithubEventHandlers sets up event handlers for GitHub events
func (e *Engine) setupGithubEventHandlers(adapter *github.Adapter) error {
    // Subscribe to pull request events
    if err := adapter.Subscribe("pull_request", func(event interface{}) {
        e.events <- mcp.Event{
            Source:    "github",
            Type:      "pull_request",
            Timestamp: time.Now(),
            Data:      event,
        }
    }); err != nil {
        return err
    }

    // Subscribe to push events
    if err := adapter.Subscribe("push", func(event interface{}) {
        e.events <- mcp.Event{
            Source:    "github",
            Type:      "push",
            Timestamp: time.Now(),
            Data:      event,
        }
    }); err != nil {
        return err
    }

    return nil
}
```

## Event Filtering

The MCP Server supports filtering events based on various criteria using the `EventFilter` struct:

```go
// EventFilter defines criteria for filtering events
type EventFilter struct {
    Sources []string  `json:"sources"`
    Types   []string  `json:"types"`
    After   time.Time `json:"after"`
    Before  time.Time `json:"before"`
}

// MatchEvent checks if an event matches the filter criteria
func (f *EventFilter) MatchEvent(event Event) bool {
    // Check sources
    if len(f.Sources) > 0 {
        matched := false
        for _, source := range f.Sources {
            if event.Source == source {
                matched = true
                break
            }
        }
        if !matched {
            return false
        }
    }

    // Check types
    if len(f.Types) > 0 {
        matched := false
        for _, eventType := range f.Types {
            if event.Type == eventType {
                matched = true
                break
            }
        }
        if !matched {
            return false
        }
    }

    // Check timestamp
    if !f.After.IsZero() && event.Timestamp.Before(f.After) {
        return false
    }
    if !f.Before.IsZero() && event.Timestamp.After(f.Before) {
        return false
    }

    return true
}
```

Filters can be used to:
- Query historical events from storage
- Subscribe to specific event types
- Process only events that match certain criteria

## Event-Driven Architecture

The MCP Server follows event-driven architecture principles:

1. **Loose Coupling**: Components communicate through events rather than direct calls
2. **Asynchronous Processing**: Events are processed asynchronously
3. **Event Sourcing**: System state can be reconstructed from event history
4. **Single Responsibility**: Each component handles specific event types
5. **Resilience**: Failures in one component don't affect others

## Webhook Events

External systems send events to the MCP Server via webhooks. The webhook handling process is:

1. **Webhook Reception**: The API Server receives a webhook HTTP request
2. **Signature Validation**: The webhook signature is validated using the configured secret
3. **Payload Parsing**: The webhook payload is parsed into the appropriate format
4. **Event Creation**: An MCP Event is created from the webhook payload
5. **Event Processing**: The event is sent to the Core Engine for processing

Example webhook handler from `internal/api/webhooks.go`:

```go
// githubWebhookHandler handles webhooks from GitHub
func (s *Server) githubWebhookHandler(c *gin.Context) {
    // Get event type from header
    eventType := c.GetHeader("X-GitHub-Event")
    if eventType == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-GitHub-Event header"})
        return
    }

    // Read and validate the payload
    payload, err := s.readAndValidateWebhookPayload(c, "X-Hub-Signature-256", s.config.Webhooks.GitHub.Secret)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Get GitHub adapter
    adapter, err := s.engine.GetAdapter("github")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "GitHub adapter not configured"})
        return
    }

    // Forward to adapter for processing
    githubAdapter, ok := adapter.(*github.Adapter)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid GitHub adapter type"})
        return
    }

    if err := githubAdapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

## Event Metrics

The MCP Server collects metrics about events to monitor system performance:

- **Event Counts**: Number of events by source and type
- **Processing Time**: Time taken to process events
- **Error Rates**: Number of errors by source and type
- **Queue Length**: Number of events waiting for processing

These metrics are exposed via Prometheus and can be visualized in Grafana dashboards.

## Event Storage and Auditing

Processed events can be stored for auditing and debugging purposes:

- Events are stored in the PostgreSQL database
- Events can be queried using various filters
- Event history can be used to reconstruct system state
- Event storage can be configured with retention policies

## Best Practices for Event Handling

When working with the MCP Server event system, follow these best practices:

1. **Keep Handlers Focused**: Each event handler should have a single responsibility
2. **Make Handlers Idempotent**: Handlers should produce the same result if executed multiple times
3. **Handle Errors Gracefully**: Event handlers should handle errors without crashing
4. **Use Timeouts**: All operations should have timeouts to prevent hanging
5. **Add Metrics**: Important operations should be measured with metrics
6. **Log Appropriately**: Include relevant information in logs, but avoid sensitive data
7. **Test Event Handlers**: Write unit and integration tests for event handlers