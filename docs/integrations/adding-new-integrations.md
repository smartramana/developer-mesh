# Adding New Integrations to MCP Server

This guide provides step-by-step instructions for adding new service integrations to the MCP Server.

## Overview

The MCP Server is designed with extensibility in mind, allowing developers to add new integrations with external DevOps tools and services. Each integration is implemented as an adapter that follows a common interface pattern.

## Prerequisites

Before adding a new integration, ensure you have:

1. Access to the external service's API documentation
2. Understanding of the events the service can generate
3. Knowledge of the authentication mechanisms used by the service
4. Familiarity with Go programming language
5. A development environment set up for MCP Server

## Step 1: Create the Adapter Package

Create a new package for your adapter in the `internal/adapters` directory:

```bash
mkdir -p internal/adapters/yourservice
touch internal/adapters/yourservice/yourservice.go
touch internal/adapters/yourservice/yourservice_test.go
```

## Step 2: Define the Adapter Configuration

In `internal/adapters/yourservice/yourservice.go`, define the configuration structure for your adapter:

```go
package yourservice

import (
    "context"
    "fmt"
    "net/http"
    "sync"
    "time"

    "github.com/S-Corkum/mcp-server/internal/adapters"
)

// Config holds configuration for the YourService adapter
type Config struct {
    APIToken         string        `mapstructure:"api_token"`
    BaseURL          string        `mapstructure:"base_url"`
    WebhookSecret    string        `mapstructure:"webhook_secret"`
    RequestTimeout   time.Duration `mapstructure:"request_timeout"`
    MaxRetries       int           `mapstructure:"max_retries"`
    RetryDelay       time.Duration `mapstructure:"retry_delay"`
    MockResponses    bool          `mapstructure:"mock_responses"`
    MockURL          string        `mapstructure:"mock_url"`
}
```

## Step 3: Implement the Adapter Interface

Create the adapter structure and implement the adapter interface:

```go
// Adapter implements the YourService integration
type Adapter struct {
    client       *http.Client
    config       Config
    baseURL      string
    subscribers  map[string][]func(interface{})
    subscriberMu sync.RWMutex
    baseAdapter  adapters.BaseAdapter
}

// NewAdapter creates a new YourService adapter
func NewAdapter(cfg Config) (*Adapter, error) {
    // Set default values if not provided
    if cfg.RequestTimeout == 0 {
        cfg.RequestTimeout = 10 * time.Second
    }
    if cfg.MaxRetries == 0 {
        cfg.MaxRetries = 3
    }
    if cfg.RetryDelay == 0 {
        cfg.RetryDelay = 1 * time.Second
    }

    // Create HTTP client
    httpClient := &http.Client{
        Timeout: cfg.RequestTimeout,
    }

    // Determine base URL
    baseURL := cfg.BaseURL
    if cfg.MockResponses {
        baseURL = cfg.MockURL
        if baseURL == "" {
            baseURL = "http://localhost:8081/mock-yourservice"
        }
    }

    adapter := &Adapter{
        client:      httpClient,
        config:      cfg,
        baseURL:     baseURL,
        subscribers: make(map[string][]func(interface{})),
        baseAdapter: adapters.BaseAdapter{
            RetryMax:   cfg.MaxRetries,
            RetryDelay: cfg.RetryDelay,
        },
    }

    return adapter, nil
}

// Initialize initializes the adapter
func (a *Adapter) Initialize(ctx context.Context, config interface{}) error {
    // Additional initialization if needed
    return nil
}

// GetData retrieves data from YourService
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
    // Implement your data retrieval logic here
    // This will depend on the specific API of your service
    
    // Example:
    // 1. Validate query parameters
    // 2. Build API request
    // 3. Execute request with retry logic
    // 4. Parse response
    // 5. Return formatted data
    
    return nil, fmt.Errorf("not implemented")
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
    // If we're in mock mode, return healthy
    if a.config.MockResponses {
        return "healthy (mock)"
    }
    
    // Try to make a simple API call to check health
    // For example, get API status or user info
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Make a simple API call to check health
    // Replace with appropriate API call for your service
    _, err := a.makeRequest(ctx, "GET", "/api/status", nil)
    if err != nil {
        return fmt.Sprintf("unhealthy: %v", err)
    }
    
    return "healthy"
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
    // Perform any necessary cleanup
    return nil
}

// HandleWebhook processes YourService webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
    // Parse the webhook payload based on the event type
    event, err := a.parseWebhookEvent(eventType, payload)
    if err != nil {
        return err
    }
    
    // Notify subscribers
    a.notifySubscribers(eventType, event)
    
    return nil
}

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

// parseWebhookEvent parses a webhook event from the payload
func (a *Adapter) parseWebhookEvent(eventType string, payload []byte) (interface{}, error) {
    // Implement parsing logic based on the event type and payload format
    // This will depend on the specific webhook format of your service
    
    return nil, fmt.Errorf("not implemented")
}

// makeRequest makes an HTTP request to the YourService API
func (a *Adapter) makeRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
    // Implement HTTP request logic with proper error handling
    // This will depend on the specific API of your service
    
    return nil, fmt.Errorf("not implemented")
}
```

## Step 4: Define Data Models

Create necessary data models in the `pkg/models` directory:

```go
// In pkg/models/models.go or a new file like pkg/models/yourservice.go

// YourServiceQuery defines parameters for querying YourService
type YourServiceQuery struct {
    Type   string `json:"type"`
    ID     string `json:"id"`
    Filter string `json:"filter"`
}

// YourService query types
const (
    YourServiceQueryTypeResource  = "resource"
    YourServiceQueryTypeProject   = "project"
    YourServiceQueryTypeOperation = "operation"
)

// YourServiceEvent represents an event from YourService
type YourServiceEvent struct {
    EventType string                 `json:"event_type"`
    Timestamp string                 `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
}
```

## Step 5: Add Webhook Handler

Create a webhook handler in `internal/api/webhooks.go`:

```go
// Add this to webhooks.go

// yourServiceWebhookHandler handles webhooks from YourService
func (s *Server) yourServiceWebhookHandler(c *gin.Context) {
    // Get event type from header or query parameter
    eventType := c.GetHeader("X-YourService-Event")
    if eventType == "" {
        eventType = c.Query("eventType")
        if eventType == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing event type"})
            return
        }
    }

    // Read and validate the payload
    payload, err := s.readAndValidateWebhookPayload(c, "X-YourService-Signature", s.config.Webhooks.YourService.Secret)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Get YourService adapter
    adapter, err := s.engine.GetAdapter("yourservice")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "YourService adapter not configured"})
        return
    }

    // Forward to adapter for processing
    yourServiceAdapter, ok := adapter.(*yourservice.Adapter)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid YourService adapter type"})
        return
    }

    if err := yourServiceAdapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

## Step 6: Update Configuration Structures

Update the configuration structures in `internal/config/config.go`:

```go
// Add to the engine config
type EngineConfig struct {
    // Existing fields...
    YourService yourservice.Config `mapstructure:"yourservice"`
}

// Add to the webhook config
type WebhookConfig struct {
    // Existing fields...
    YourService WebhookProviderConfig `mapstructure:"yourservice"`
}
```

## Step 7: Update Core Engine

Modify the Core Engine in `internal/core/engine.go` to initialize your adapter:

```go
// Add import
import (
    // Existing imports...
    "github.com/S-Corkum/mcp-server/internal/adapters/yourservice"
)

// Update initializeAdapters method
func (e *Engine) initializeAdapters() error {
    var err error

    // Existing adapter initialization...

    // Initialize YourService adapter if configured
    if e.config.YourService.APIToken != "" {
        if e.adapters["yourservice"], err = yourservice.NewAdapter(e.config.YourService); err != nil {
            return err
        }
        if err = e.setupYourServiceEventHandlers(e.adapters["yourservice"].(*yourservice.Adapter)); err != nil {
            return err
        }
    }

    return nil
}

// Add setupYourServiceEventHandlers method
func (e *Engine) setupYourServiceEventHandlers(adapter *yourservice.Adapter) error {
    // Subscribe to event types your service can generate
    // For example:
    
    // Subscribe to resource events
    if err := adapter.Subscribe("resource", func(event interface{}) {
        e.events <- mcp.Event{
            Source:    "yourservice",
            Type:      "resource",
            Timestamp: time.Now(),
            Data:      event,
        }
    }); err != nil {
        return err
    }

    // Subscribe to project events
    if err := adapter.Subscribe("project", func(event interface{}) {
        e.events <- mcp.Event{
            Source:    "yourservice",
            Type:      "project",
            Timestamp: time.Now(),
            Data:      event,
        }
    }); err != nil {
        return err
    }

    return nil
}

// Add processYourServiceEvent method
func (e *Engine) processYourServiceEvent(ctx context.Context, event mcp.Event) {
    // Implementation specific to YourService events
    switch event.Type {
    case "resource":
        // Process resource event
    case "project":
        // Process project event
    default:
        log.Printf("Unknown YourService event type: %s", event.Type)
    }
}

// Update processEvent method to include your service
func (e *Engine) processEvent(event mcp.Event) {
    // Existing code...
    
    // Process based on event source and type
    switch event.Source {
    // Existing cases...
    case "yourservice":
        e.processYourServiceEvent(ctx, event)
    default:
        log.Printf("Unknown event source: %s", event.Source)
    }
}
```

## Step 8: Update API Server Routes

Add the new webhook endpoint to the API Server in `internal/api/server.go`:

```go
// Update setupRoutes method
func (s *Server) setupRoutes() {
    // Existing code...

    // Setup YourService webhook endpoint if enabled
    if s.config.Webhooks.YourService.Enabled {
        path := "/yourservice"
        if s.config.Webhooks.YourService.Path != "" {
            path = s.config.Webhooks.YourService.Path
        }
        webhook.POST(path, s.yourServiceWebhookHandler)
    }
}
```

## Step 9: Update Mock Server

Add a mock endpoint to the mock server in `cmd/mockserver/main.go`:

```go
// Add mock handler for YourService
http.HandleFunc("/mock-yourservice/", func(w http.ResponseWriter, r *http.Request) {
    log.Printf("Mock YourService request: %s %s", r.Method, r.URL.Path)
    w.Header().Set("Content-Type", "application/json")
    response := map[string]interface{}{
        "success": true,
        "message": "Mock YourService response",
        "timestamp": time.Now().Format(time.RFC3339),
    }
    json.NewEncoder(w).Encode(response)
})

// Add mock webhook endpoint
http.HandleFunc("/api/v1/webhook/yourservice", func(w http.ResponseWriter, r *http.Request) {
    log.Printf("Mock YourService webhook received: %s", r.Method)
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
})
```

## Step 10: Update Configuration Template

Add your service configuration to `configs/config.yaml.template`:

```yaml
engine:
  # Existing configuration...
  
  # YourService Configuration
  yourservice:
    api_token: "${YOURSERVICE_API_TOKEN}"
    base_url: "${YOURSERVICE_URL}"
    webhook_secret: "${YOURSERVICE_WEBHOOK_SECRET}"
    request_timeout: 10s
    max_retries: 3
    retry_delay: 1s
    mock_responses: false
    mock_url: ""

# Webhook configuration
api:
  webhooks:
    # Existing webhooks...
    yourservice:
      enabled: true
      secret: "${YOURSERVICE_WEBHOOK_SECRET}"
      path: "/yourservice"
```

## Step 11: Write Tests

Create tests for your adapter in `internal/adapters/yourservice/yourservice_test.go`:

```go
package yourservice

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

func TestNewAdapter(t *testing.T) {
    cfg := Config{
        APIToken:       "test-token",
        BaseURL:        "https://api.example.com",
        WebhookSecret:  "test-secret",
        RequestTimeout: 10 * time.Second,
    }

    adapter, err := NewAdapter(cfg)
    if err != nil {
        t.Fatalf("Failed to create adapter: %v", err)
    }

    if adapter.baseURL != cfg.BaseURL {
        t.Errorf("Expected base URL %s, got %s", cfg.BaseURL, adapter.baseURL)
    }
}

func TestMockMode(t *testing.T) {
    cfg := Config{
        APIToken:      "test-token",
        MockResponses: true,
        MockURL:       "http://localhost:8081/mock-yourservice",
    }

    adapter, err := NewAdapter(cfg)
    if err != nil {
        t.Fatalf("Failed to create adapter: %v", err)
    }

    if adapter.baseURL != cfg.MockURL {
        t.Errorf("Expected mock URL %s, got %s", cfg.MockURL, adapter.baseURL)
    }

    health := adapter.Health()
    if health != "healthy (mock)" {
        t.Errorf("Expected health 'healthy (mock)', got '%s'", health)
    }
}

func TestSubscription(t *testing.T) {
    adapter, _ := NewAdapter(Config{})

    eventReceived := false
    done := make(chan bool)

    // Subscribe to test event
    err := adapter.Subscribe("test_event", func(event interface{}) {
        eventReceived = true
        done <- true
    })

    if err != nil {
        t.Fatalf("Failed to subscribe: %v", err)
    }

    // Trigger event
    go adapter.notifySubscribers("test_event", "test data")

    // Wait for event or timeout
    select {
    case <-done:
        if !eventReceived {
            t.Errorf("Event not received by subscriber")
        }
    case <-time.After(100 * time.Millisecond):
        t.Errorf("Timed out waiting for event")
    }
}

// Add more tests for HandleWebhook, GetData, etc.
```

## Step 12: Update Documentation

Add documentation for your new integration:

1. Create a new markdown file `docs/yourservice-integration.md`
2. Update `docs/README.md` to include the new integration
3. Add information to `docs/integration-points.md`

## Step 13: Test Your Integration

1. Implement the necessary functions in your adapter
2. Build and run the MCP Server
3. Test webhook handling with mock data
4. Test API interactions with mock server
5. Test with real service credentials if available

## Example Implementation Details

### Example Webhook Handling

For most services, webhook handling follows this pattern:

1. Receive HTTP POST with JSON payload
2. Validate webhook signature if supported
3. Extract event type and payload
4. Parse payload into appropriate data structure
5. Process event and notify subscribers

### Example API Interaction

For API interactions, follow this pattern:

1. Build API request with authentication
2. Execute request with retry logic
3. Handle rate limiting and other errors
4. Parse response into appropriate data structure
5. Return formatted data

## Best Practices

1. **Error Handling**: Use consistent error handling and propagation
2. **Retry Logic**: Implement retries for transient failures
3. **Rate Limiting**: Respect API rate limits
4. **Authentication**: Securely handle authentication credentials
5. **Logging**: Include useful logs for debugging
6. **Tests**: Write comprehensive tests for your adapter
7. **Documentation**: Document all events and API operations
8. **Configuration**: Allow flexible configuration via YAML or environment variables

## Debugging Tips

1. Enable mock mode for initial development
2. Use tools like Postman to test API endpoints
3. Set up webhook testing with ngrok for local development
4. Verify webhook signatures with manual calculations
5. Add detailed logging during development