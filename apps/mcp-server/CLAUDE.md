# MCP Server - Model Context Protocol Server

## Service Overview
The MCP Server is the core WebSocket server for real-time AI agent communication and orchestration. It handles:
- WebSocket connections for agents
- Dynamic tool management and execution
- Context management for agent sessions
- Embedding operations via AWS Bedrock
- Real-time event streaming
- Multi-agent task coordination

## Architecture
- **Protocol**: WebSocket (binary and text)
- **Port**: 8080 (configurable)
- **Framework**: Gin for HTTP, gorilla/websocket
- **Dependencies**: PostgreSQL, Redis, AWS Bedrock

## Key Components

### Core Engine (`internal/core/`)
- `Engine`: Main orchestration engine
- `ContextManager`: Manages agent context with S3 storage
- `AdapterContextBridge`: Bridges adapters with context
- `System`: System-level operations

### API Layer (`internal/api/`)
- `Server`: Main server with routing
- `WebSocketServer`: Handles WS connections
- `DynamicToolsAPI`: Tool management endpoints
- `EmbeddingProxy`: AWS Bedrock proxy

### Services (`internal/services/`)
- `ToolService`: Dynamic tool orchestration
- `DiscoveryService`: API discovery
- `ExecutionService`: Tool execution
- `HealthChecker`: Tool health monitoring
- `CredentialManager`: Secure credential handling

### Adapters (`internal/adapters/`)
- `OpenAPIAdapter`: OpenAPI spec handling
- Tool-specific adapters for execution

## WebSocket Protocol
```json
// Agent Registration
{
  "type": "agent.register",
  "payload": {
    "id": "agent-id",
    "capabilities": ["code", "security"],
    "model_id": "claude-3"
  }
}

// Task Assignment
{
  "type": "task.assign",
  "task_id": "uuid",
  "payload": {...}
}
```

## API Endpoints
- `GET /health` - Health check
- `GET /ws` - WebSocket endpoint
- `POST /api/v1/tools` - Create tool
- `GET /api/v1/tools` - List tools
- `POST /api/v1/tools/:id/execute` - Execute tool
- `POST /api/v1/embeddings` - Generate embeddings

## Testing
```bash
# Run all tests
cd apps/mcp-server && go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/core/...
```

## Common Issues

### WebSocket Connection Drops
- Check heartbeat configuration
- Verify client implements ping/pong
- Check proxy timeout settings

### Context Manager S3 Errors
- Verify AWS credentials
- Check S3 bucket permissions
- Ensure bucket exists in region

### Tool Execution Failures
- Check tool health status
- Verify credentials are valid
- Check network connectivity

## Security Considerations
- All WebSocket connections require authentication
- Tool credentials are encrypted at rest
- API keys validated with regex pattern
- Circuit breakers for external calls

## Configuration
```yaml
# Key settings in config.yaml
api:
  host: "0.0.0.0"
  port: 8080
  cors:
    enabled: true
    
websocket:
  heartbeat_interval: 30s
  max_message_size: 1048576
  
tools:
  health_check_interval: 5m
  execution_timeout: 30s
```

## Debugging Tips
```bash
# Check WebSocket connections
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: test" \
  http://localhost:8080/ws

# Monitor logs
docker-compose logs -f mcp-server

# Check tool health
curl http://localhost:8080/api/v1/tools/:id/health
```

## Performance Tuning
- Connection pool size: 100 (database)
- WebSocket buffer size: 1MB
- Context cache TTL: 5 minutes
- Health check parallelism: 10

## Integration Points
- **REST API**: Via REST client factory
- **PostgreSQL**: Main data store
- **Redis**: Caching and pub/sub
- **AWS Bedrock**: Embeddings
- **S3**: Context storage

## Development Workflow
1. Make changes in `internal/` directories
2. Run `go test ./...` to verify
3. Update API tests if endpoints change
4. Test WebSocket with mock clients
5. Verify health endpoints work

## Important Files
- `cmd/server/main.go` - Entry point
- `internal/api/server.go` - Server setup
- `internal/api/websocket/server.go` - WS handling
- `internal/core/engine.go` - Core logic
- `internal/api/config.go` - Configuration

## Metrics & Monitoring
- WebSocket connections: `mcp.websocket.connections`
- Tool executions: `mcp.tools.executions`
- Context operations: `mcp.context.operations`
- Embedding requests: `mcp.embeddings.requests`

## Error Patterns
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to execute tool %s: %w", toolID, err)
}

// Log WebSocket errors
if err := ws.WriteMessage(messageType, data); err != nil {
    s.logger.Error("Failed to write WebSocket message", map[string]interface{}{
        "error": err.Error(),
        "agent_id": agentID,
    })
}
```

## Testing Patterns
- Mock WebSocket connections
- Use testify for assertions
- Test tool execution with mocks
- Verify context persistence
- Check error scenarios

## Never Do
- Don't store credentials in logs
- Don't skip WebSocket ping/pong
- Don't ignore close errors
- Don't hardcode AWS regions
- Don't bypass auth checks