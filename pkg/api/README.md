# API Package

## Overview

The `api` package provides the HTTP and WebSocket API implementations for the DevOps MCP platform. It includes REST endpoints, WebSocket handlers for real-time communication, middleware components, and integration with various DevOps tools.

> **Note**: This package is being migrated to the new monorepo structure. New REST endpoints should be implemented in `apps/rest-api/` and WebSocket functionality in `apps/mcp-server/`.

## Architecture

```
api/
├── server.go            # Main REST API server
├── mcp_api.go          # MCP context management endpoints
├── tool_api.go         # DevOps tool integrations
├── agent_api.go        # AI agent management
├── embedding_api_v2.go # Embedding operations
├── vector_api.go       # Vector search operations
├── middleware.go       # Authentication, rate limiting, etc.
├── webhook_server.go   # Webhook handling
├── context/           # Context-specific handlers
├── webhooks/          # Webhook providers
└── responses/         # Response utilities
```

## REST API Endpoints

### Context Management (`/api/v1/contexts`)

```go
// Create new context
POST   /api/v1/contexts
{
    "name": "authentication-refactor",
    "content": "Implement OAuth2 authentication...",
    "type": "task",
    "metadata": {
        "priority": "high",
        "tags": ["auth", "security"]
    }
}

// Get context
GET    /api/v1/contexts/:id

// Update context  
PUT    /api/v1/contexts/:id

// Delete context
DELETE /api/v1/contexts/:id

// Search contexts
GET    /api/v1/contexts/search?q=authentication&limit=10

// Summarize context
POST   /api/v1/contexts/:id/summarize
```

### Agent Management (`/api/v1/agents`)

```go
// Register agent
POST   /api/v1/agents
{
    "name": "code-analyzer-1",
    "type": "analyzer",
    "capabilities": ["code_analysis", "security_scan"],
    "endpoint": "ws://agent1.internal:8080"
}

// List agents
GET    /api/v1/agents?status=active&capability=code_analysis

// Get agent details
GET    /api/v1/agents/:id

// Update agent status
PATCH  /api/v1/agents/:id/status
{
    "status": "active"
}

// Get agent workload
GET    /api/v1/agents/:id/workload
```

### Embedding Operations (`/api/v1/embeddings`)

```go
// Generate embedding
POST   /api/v1/embeddings
{
    "text": "Fix memory leak in worker process",
    "model": "text-embedding-3-small",
    "metadata": {
        "type": "issue",
        "id": "ISSUE-123"
    }
}

// Batch generate
POST   /api/v1/embeddings/batch
{
    "texts": ["text1", "text2", "text3"],
    "model": "amazon.titan-embed-text-v1"
}

// Search similar
POST   /api/v1/embeddings/search
{
    "query": "authentication bug",
    "model": "text-embedding-3-small",
    "top_k": 10,
    "filters": {
        "type": "issue",
        "status": "open"
    }
}
```

### Tool Integration (`/api/v1/tools`)

```go
// Execute tool action
POST   /api/v1/tools/:tool/execute
{
    "action": "create_issue",
    "params": {
        "title": "Fix memory leak",
        "body": "Description...",
        "labels": ["bug", "performance"]
    }
}

// Supported tools:
// - github: Issues, PRs, commits, files
// - harness: Pipelines, deployments
// - sonarqube: Code quality metrics
// - slack: Notifications
// - custom: User-defined tools
```

### Vector Operations (`/api/v1/vectors`)

```go
// Store vector
POST   /api/v1/vectors
{
    "id": "doc-123",
    "vector": [0.1, 0.2, ...],
    "metadata": {
        "type": "document",
        "tags": ["api", "rest"]
    }
}

// Search vectors
POST   /api/v1/vectors/search
{
    "vector": [0.1, 0.2, ...],
    "top_k": 20,
    "min_similarity": 0.7
}
```

## WebSocket API (MCP Server)

The WebSocket server implements the Model Context Protocol with binary optimization:

### Connection

```javascript
// Connect to WebSocket server
const ws = new WebSocket('ws://localhost:8080/ws');

// Send initialization message
ws.send(JSON.stringify({
    jsonrpc: "2.0",
    method: "initialize",
    params: {
        protocolVersion: "0.1.0",
        capabilities: {
            tools: {},
            prompts: {},
            resources: {}
        },
        clientInfo: {
            name: "devops-agent",
            version: "1.0.0"
        }
    },
    id: 1
}));
```

### Binary Protocol

For performance, large messages use binary encoding:

```go
// Binary message format (24 bytes header)
type Header struct {
    Magic      [4]byte  // "DMCP"
    Version    uint8    // Protocol version
    Type       uint8    // Message type
    Method     uint16   // Method enum
    Flags      uint8    // Compression, encryption
    Reserved   [3]byte  // Future use
    PayloadLen uint32   // Payload size
    RequestID  uint64   // Request ID
}

// Automatic compression for messages > 1KB
// Supports gzip compression
// Max payload: ~4GB
```

### Real-time Features

```javascript
// Subscribe to agent events
ws.send(JSON.stringify({
    method: "agent.subscribe",
    params: {
        events: ["status_change", "task_assigned"],
        agent_ids: ["agent-1", "agent-2"]
    }
}));

// Receive notifications
ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.method === "agent.notification") {
        // Handle agent notification
    }
};
```

## Middleware

### Authentication

```go
// JWT authentication
authMiddleware := middleware.JWTAuth(jwtSecret)

// API key authentication
apiKeyAuth := middleware.APIKeyAuth(apiKeyValidator)

// Combined authentication
app.Use(middleware.Auth(
    middleware.WithJWT(jwtSecret),
    middleware.WithAPIKey(validator),
))
```

### Rate Limiting

```go
// Rate limit by IP
rateLimiter := middleware.RateLimit(
    100,              // requests
    time.Minute,      // per minute
    middleware.ByIP,  // key function
)

// Custom rate limits
customLimiter := middleware.RateLimit(
    10,
    time.Minute,
    func(c *gin.Context) string {
        return c.GetString("user_id")
    },
)
```

### CORS Configuration

```go
corsConfig := cors.Config{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    ExposeHeaders:    []string{"X-Total-Count"},
    AllowCredentials: true,
    MaxAge:          12 * time.Hour,
}
```

### Metrics Collection

```go
// Automatic metrics for all endpoints
metricsMiddleware := middleware.Metrics()

// Collected metrics:
// - http_requests_total
// - http_request_duration_seconds
// - http_request_size_bytes
// - http_response_size_bytes
// - http_requests_in_flight
```

## Webhook Support

### GitHub Webhooks

```go
// Configure webhook handler
webhookHandler := webhooks.NewGitHubHandler(
    githubSecret,
    func(event webhooks.Event) error {
        switch event.Type {
        case "push":
            // Handle push event
        case "pull_request":
            // Handle PR event
        case "issues":
            // Handle issue event
        }
        return nil
    },
)

// Register webhook endpoint
router.POST("/webhooks/github", webhookHandler.Handle)
```

### Custom Webhooks

```go
// Register custom webhook provider
webhookServer.RegisterProvider("custom", &CustomProvider{
    ValidateSignature: func(r *http.Request) error {
        // Validate webhook signature
    },
    ParseEvent: func(r *http.Request) (Event, error) {
        // Parse webhook payload
    },
})
```

## Error Handling

### Standard Error Response

```go
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}

// Example error response
{
    "error": {
        "code": "RESOURCE_NOT_FOUND",
        "message": "Context not found",
        "details": {
            "id": "ctx-123",
            "type": "context"
        }
    }
}
```

### Error Middleware

```go
// Global error handler
app.Use(middleware.ErrorHandler())

// Custom error handling
app.Use(func(c *gin.Context) {
    c.Next()
    
    if len(c.Errors) > 0 {
        err := c.Errors.Last()
        switch e := err.Err.(type) {
        case *ValidationError:
            c.JSON(400, ErrorResponse{
                Error: ErrorDetail{
                    Code: "VALIDATION_ERROR",
                    Message: e.Error(),
                    Details: e.Fields,
                },
            })
        default:
            c.JSON(500, ErrorResponse{
                Error: ErrorDetail{
                    Code: "INTERNAL_ERROR",
                    Message: "An error occurred",
                },
            })
        }
    }
})
```

## Response Utilities

### JSON Responses

```go
// Success response with data
responses.JSON(c, 200, map[string]interface{}{
    "data": result,
    "meta": map[string]interface{}{
        "count": len(result),
        "page": 1,
    },
})

// Error response
responses.Error(c, 404, "RESOURCE_NOT_FOUND", "Context not found")

// Created response with location header
responses.Created(c, "/api/v1/contexts/"+id, context)
```

### HATEOAS Support

```go
// Add hypermedia links
response := map[string]interface{}{
    "data": context,
    "_links": map[string]interface{}{
        "self": "/api/v1/contexts/" + context.ID,
        "update": "/api/v1/contexts/" + context.ID,
        "delete": "/api/v1/contexts/" + context.ID,
        "embeddings": "/api/v1/contexts/" + context.ID + "/embeddings",
    },
}
```

## Versioning

### API Version Management

```go
// Version in URL path
v1 := router.Group("/api/v1")
v2 := router.Group("/api/v2")

// Version in header
version := c.GetHeader("API-Version")
switch version {
case "1.0":
    handleV1(c)
case "2.0":
    handleV2(c)
default:
    c.JSON(400, gin.H{"error": "Invalid API version"})
}
```

## Performance Optimization

### Response Compression

```go
// Enable gzip compression
app.Use(gzip.Gzip(gzip.DefaultCompression))
```

### ETag Support

```go
// Generate ETag for responses
etag := generateETag(data)
c.Header("ETag", etag)

// Check If-None-Match
if c.GetHeader("If-None-Match") == etag {
    c.Status(304)
    return
}
```

### Connection Pooling

```go
// Configure HTTP client with pooling
client := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
    Timeout: 30 * time.Second,
}
```

## Testing

### Unit Tests

```go
// Test REST endpoints
func TestCreateContext(t *testing.T) {
    router := setupTestRouter()
    
    w := httptest.NewRecorder()
    body := bytes.NewBufferString(`{"name":"test"}`)
    req, _ := http.NewRequest("POST", "/api/v1/contexts", body)
    req.Header.Set("Content-Type", "application/json")
    
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 201, w.Code)
}
```

### WebSocket Tests

```go
// Test WebSocket connection
func TestWebSocketConnection(t *testing.T) {
    server := httptest.NewServer(wsHandler)
    defer server.Close()
    
    ws, _, err := websocket.DefaultDialer.Dial(
        "ws"+strings.TrimPrefix(server.URL, "http")+"/ws",
        nil,
    )
    require.NoError(t, err)
    defer ws.Close()
    
    // Test message exchange
    err = ws.WriteJSON(initMessage)
    require.NoError(t, err)
}
```

## Migration Guide

When migrating to the new structure:

1. **REST Endpoints**: Move to `apps/rest-api/internal/api/`
2. **WebSocket Handlers**: Move to `apps/mcp-server/internal/api/websocket/`
3. **Shared Types**: Keep in `pkg/api/types/`
4. **Middleware**: Move to respective app's `internal/middleware/`

## Best Practices

1. **Always validate input**: Use binding tags and custom validators
2. **Use middleware**: Leverage middleware for cross-cutting concerns
3. **Return consistent responses**: Use the response utilities
4. **Handle errors gracefully**: Never expose internal errors
5. **Add metrics**: Monitor all critical endpoints
6. **Document APIs**: Keep OpenAPI spec updated
7. **Version APIs**: Plan for backward compatibility

## References

- [Gin Framework Documentation](https://gin-gonic.com/docs/)
- [Gorilla WebSocket](https://github.com/gorilla/websocket)
- [OpenAPI Specification](https://swagger.io/specification/)
- [Model Context Protocol](https://modelcontextprotocol.io/)