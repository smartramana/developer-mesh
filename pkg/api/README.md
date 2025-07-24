# API Package

## Overview

The `api` package provides the HTTP and WebSocket API implementations for the Developer Mesh platform. It includes REST endpoints, WebSocket handlers for real-time communication, middleware components, and integration with various DevOps tools.

> **Note**: This package is being migrated to the new monorepo structure. New REST endpoints should be implemented in `apps/rest-api/` and WebSocket functionality in `apps/mcp-server/`.

> **Implementation Status**: Most functionality has been migrated to the app modules. This package now primarily contains legacy code, shared types, and base middleware implementations.

## Architecture

```
api/
├── middleware.go       # Base middleware implementations
├── types/             # Shared API types
└── errors.go          # Error definitions

Actual implementations now in:
apps/mcp-server/
├── internal/api/
│   ├── server.go      # MCP WebSocket server
│   ├── mcp_api.go     # MCP context endpoints
│   └── websocket/     # WebSocket handlers

apps/rest-api/
├── internal/api/
│   ├── server.go      # REST API server
│   ├── routes.go      # Route registration
│   └── handlers/      # REST endpoint handlers
```

## REST API Endpoints

### Context Management

**MCP Server** (`/api/v1/mcp/context/*`):
```go
// Create new context
POST   /api/v1/mcp/context
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
GET    /api/v1/mcp/context/:contextID
```

**REST API** (`/api/v1/contexts/*`):
```go
// Full CRUD operations
POST   /api/v1/contexts          // Create
GET    /api/v1/contexts          // List
GET    /api/v1/contexts/:id      // Get
PUT    /api/v1/contexts/:id      // Update
DELETE /api/v1/contexts/:id      // Delete

// Additional operations
GET    /api/v1/contexts/:id/summary     // Get summary
POST   /api/v1/contexts/:id/search      // Search in context
```

### Agent Management (`/api/v1/agents`)

**Currently Implemented**:
```go
// Create agent
POST   /api/v1/agents
{
    "name": "code-analyzer-1",
    "type": "analyzer",
    "capabilities": ["code_analysis", "security_scan"]
}

// List agents  
GET    /api/v1/agents?status=active&capability=code_analysis

// Update agent
PUT    /api/v1/agents/:id
```

**Not Yet Implemented**:
```go
// Get agent details
GET    /api/v1/agents/:id            // TODO

// Update agent status
PATCH  /api/v1/agents/:id/status     // TODO

// Get agent workload
GET    /api/v1/agents/:id/workload   // TODO
```

**Note**: Agent registration primarily happens via WebSocket connection, not REST API.

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
// List available tools
GET    /api/v1/tools

// Get tool details
GET    /api/v1/tools/:tool

// List tool actions
GET    /api/v1/tools/:tool/actions

// Get action details
GET    /api/v1/tools/:tool/actions/:action

// Execute tool action
POST   /api/v1/tools/:tool/actions/:action
{
    "params": {
        "title": "Fix memory leak",
        "body": "Description...",
        "labels": ["bug", "performance"]
    }
}

// Query tool data
POST   /api/v1/tools/:tool/queries
{
    "query_type": "search_issues",
    "params": {
        "state": "open",
        "labels": ["bug"]
    }
}

// Currently supported tools:
// - github: Issues, PRs, commits, files
// - slack: Notifications (planned)
// - custom: User-defined tools (planned)
```

### Vector Operations (`/api/v1/vectors`)

**Note**: Vector API is implemented but not yet registered in the REST API routes.

```go
// Store vector
POST   /api/v1/vectors/store    // Note: different from documented path
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

// Additional endpoints (implemented):
GET    /api/v1/vectors/context/:context_id              // Get context embeddings
DELETE /api/v1/vectors/context/:context_id              // Delete context embeddings
GET    /api/v1/vectors/models                           // Get supported models
GET    /api/v1/vectors/context/:context_id/model/:model_id    // Get model embeddings
DELETE /api/v1/vectors/context/:context_id/model/:model_id    // Delete model embeddings
```

## WebSocket API (MCP Server)

The WebSocket server implements the Model Context Protocol with optional binary optimization:

### Connection

```javascript
// Connect to WebSocket server with required subprotocol
const ws = new WebSocket('wss://dev-mesh.io/ws', ['mcp.v1']);

// Send initialization message
ws.send(JSON.stringify({
    jsonrpc: "2.0",
    method: "initialize",
    params: {
        protocolVersion: "2024-11-05",  // Required protocol version
        capabilities: {
            tools: {},
            prompts: {},
            resources: {}
        },
        clientInfo: {
            name: "my-agent",  // This becomes the agent ID if not authenticated
            version: "1.0.0"
        }
    },
    id: 1
}));
```

**Important Notes**:
- The `mcp.v1` subprotocol is **required** for connection
- Agent ID is auto-assigned from `clientInfo.name` or generated if not provided
- Use `github.com/coder/websocket` for Go clients (NOT gorilla/websocket)

### Binary Protocol

For performance, the server supports optional binary message encoding:

```go
// Actual binary header format (12 bytes)
type BinaryHeader struct {
    Version      uint8    // Protocol version (currently 1)
    Flags        uint8    // Bit 0: compression enabled
    MessageType  uint16   // Message type identifier
    PayloadSize  uint32   // Size of payload
    Reserved     [4]byte  // Reserved for future use
}

// Enable binary protocol
{
    "jsonrpc": "2.0",
    "method": "protocol.set_binary",
    "params": {
        "enabled": true
    },
    "id": 2
}

// Features:
// - Automatic gzip compression for messages > threshold (configurable)
// - Messages remain JSON-encoded in payload
// - Can switch between text/binary modes dynamically
// - No magic bytes ("DMCP") in current implementation
```

### Real-time Features

```javascript
// Subscribe to various events
ws.send(JSON.stringify({
    jsonrpc: "2.0",
    method: "event.subscribe",  // General event subscription
    params: {
        event_type: "agent.status_change",
        filters: {
            agent_ids: ["agent-1", "agent-2"]
        }
    },
    id: 3
}));

// Agent-specific subscription
ws.send(JSON.stringify({
    jsonrpc: "2.0",
    method: "agent.subscribe",
    params: {
        events: ["status_change", "task_assigned"],
        agent_ids: ["agent-1", "agent-2"]
    },
    id: 4
}));

// Receive notifications
ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.method === "notification") {
        // Handle notification based on msg.params.type
    }
};
```

## Middleware

### Authentication

```go
// Enhanced authentication middleware (production-ready)
import "github.com/developer-mesh/developer-mesh/pkg/auth"

// Setup enhanced auth with rate limiting, metrics, and audit
authMiddleware, err := auth.SetupAuthentication(db, cache, logger, metrics)
if err != nil {
    // Fall back to basic auth service
    authService := auth.NewService(config, db, cache, logger)
    v1.Use(authService.GinMiddleware(auth.TypeAPIKey, auth.TypeJWT))
} else {
    v1.Use(authMiddleware.GinMiddleware())
}

// The middleware supports:
// - JWT tokens (Bearer prefix)
// - API keys (with or without Bearer prefix)
// - Custom API key headers (X-API-Key, etc.)
// - Rate limiting per user/IP
// - Metrics collection
// - Audit logging
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
// Metrics middleware (basic implementation)
app.Use(MetricsMiddleware())

// Prometheus metrics endpoint
app.GET("/metrics", metricsHandler)

// Available metrics:
// - api_request_total (counter)
// - api_request_duration_seconds (histogram)
// - websocket_connections_total (counter)
// - websocket_connections_active (gauge)
// - cache_hits_total (counter)
// - embedding_operations_total (counter by operation)
// - embedding_provider_errors_total (counter by provider)

// Note: Actual metric recording requires integration with metrics backend
```

## Webhook Support

### Webhook Support

**Note**: Webhook functionality is designed but not yet fully implemented.

```go
// Planned webhook architecture:
// - GitHub webhook handler
// - Custom webhook providers
// - Event processing pipeline

// Currently, webhook endpoints exist in REST API but are using mock implementations
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

## Current Implementation Status

### ✅ Fully Implemented
- Embedding API (with extra features)
- Tool API (RESTful design)
- WebSocket connection and messaging
- Authentication middleware (JWT + API key)
- Rate limiting, CORS, metrics middleware
- Binary protocol (simplified version)

### ⚠️ Partially Implemented
- Agent API (3 of 6 endpoints)
- Context API (different routing)
- Vector API (implemented but not registered)

### ❌ Not Implemented
- Webhook processing (mock only)
- Some documented binary protocol features

## Migration Status

Most functionality has been migrated:

1. **REST Endpoints**: Now in `apps/rest-api/internal/api/`
2. **WebSocket Handlers**: Now in `apps/mcp-server/internal/api/websocket/`
3. **Shared Types**: Some remain in `pkg/api/types/`
4. **Middleware**: Split between `pkg/api/` (base) and apps (specific)

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
- [Coder WebSocket](https://github.com/coder/websocket) - Used for WebSocket connections
- [OpenAPI Specification](https://swagger.io/specification/)
- [Model Context Protocol](https://modelcontextprotocol.io/)