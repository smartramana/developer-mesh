# WebSocket Implementation Guide

This guide provides comprehensive documentation for the WebSocket implementation in the DevOps MCP platform.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Connection Management](#connection-management)
4. [Message Protocol](#message-protocol)
5. [Binary Protocol](#binary-protocol)
6. [Security](#security)
7. [Performance Optimizations](#performance-optimizations)
8. [API Reference](#api-reference)
9. [Configuration](#configuration)
10. [Testing](#testing)
11. [Monitoring](#monitoring)
12. [Troubleshooting](#troubleshooting)

## Overview

The WebSocket implementation provides real-time, bidirectional communication between AI agents, IDEs, and the MCP server. It offers:

- **High Performance**: Binary protocol with 10-15x performance improvement over JSON
- **Security**: JWT/API key authentication, HMAC signatures, and rate limiting
- **Scalability**: Connection pooling, message batching, and zero-allocation design
- **Reliability**: Anti-replay protection, automatic reconnection, and error handling

## Architecture

### Component Overview

```
┌─────────────┐     ┌──────────────┐     ┌───────────────┐
│  AI Agents  │────▶│  WebSocket   │────▶│  MCP Server   │
└─────────────┘     │   Server     │     │  Components   │
                    └──────────────┘     └───────────────┘
┌─────────────┐            │                     │
│    IDEs     │────────────┘                     │
└─────────────┘                                  │
                                                 ▼
                                        ┌─────────────────┐
                                        │ Context Manager │
                                        │ Tool Registry   │
                                        │ Event Bus       │
                                        └─────────────────┘
```

### Key Components

1. **WebSocket Server** (`server.go`)
   - Handles connection lifecycle
   - Routes messages to handlers
   - Manages authentication and authorization

2. **Connection Manager** (`connection.go`)
   - Read/write pumps for bidirectional communication
   - Per-connection rate limiting
   - State management

3. **Message Handlers** (`handlers.go`)
   - Process different message types
   - Interface with MCP components
   - Error handling and response formatting

4. **Binary Protocol** (`binary.go`)
   - High-performance binary encoding/decoding
   - 24-byte header format
   - Compression support

5. **Security Layer** (`auth.go`)
   - JWT and API key validation
   - HMAC message signatures
   - Session key management

## Connection Management

### Connection Lifecycle

1. **Handshake**
   ```http
   GET /ws HTTP/1.1
   Host: mcp-server.example.com
   Upgrade: websocket
   Connection: Upgrade
   Authorization: Bearer <api-key>
   ```

2. **Initialization**
   ```json
   {
     "id": "uuid",
     "type": 0,
     "method": "initialize",
     "params": {
       "name": "agent-name",
       "version": "1.0.0"
     }
   }
   ```

3. **Active Communication**
   - Request/Response pattern
   - Notifications for events
   - Ping/Pong for keep-alive

4. **Graceful Shutdown**
   - Close frame with status code
   - Cleanup of resources
   - Session termination

### Connection States

```go
const (
    ConnectionStateConnecting = iota
    ConnectionStateConnected
    ConnectionStateClosing
    ConnectionStateClosed
)
```

## Message Protocol

### Message Types

```go
const (
    MessageTypeRequest = iota      // Client request
    MessageTypeResponse            // Server response
    MessageTypeNotification        // Server notification
    MessageTypeError              // Error message
    MessageTypePing               // Keep-alive ping
    MessageTypePong               // Keep-alive pong
    MessageTypeBatch              // Batched messages
)
```

### Message Structure

```json
{
  "id": "unique-message-id",
  "type": 0,
  "method": "method.name",
  "params": {},
  "result": {},
  "error": {
    "code": 4000,
    "message": "Error description",
    "data": {}
  }
}
```

### Standard Methods

- `initialize` - Initialize connection
- `tool.list` - List available tools
- `tool.execute` - Execute a tool
- `context.create` - Create new context
- `context.get` - Retrieve context
- `context.update` - Update context
- `context.list` - List contexts
- `event.subscribe` - Subscribe to events
- `event.unsubscribe` - Unsubscribe from events

## Binary Protocol

### Header Format (24 bytes)

```
┌────────────┬─────────┬──────┬───────┬────────────┬────────┬──────────┬──────────┐
│   Magic    │ Version │ Type │ Flags │ SequenceID │ Method │ Reserved │ DataSize │
│  (4 bytes) │(1 byte) │(1 b) │ (2 b) │  (8 bytes) │ (2 b)  │  (2 b)   │ (4 bytes)│
└────────────┴─────────┴──────┴───────┴────────────┴────────┴──────────┴──────────┘
```

- **Magic**: 0x4D435057 ("MCPW")
- **Version**: Protocol version (currently 1)
- **Type**: Message type (0-6)
- **Flags**: Compression (bit 0), Encryption (bit 1)
- **SequenceID**: Unique message sequence number
- **Method**: Method enum value (not string)
- **Reserved**: Future use
- **DataSize**: Payload size in bytes

### Performance Benefits

- 10-15x faster than JSON encoding/decoding
- Reduced network overhead
- Zero-allocation design with object pooling
- CPU cache-friendly data layout

## Security

### Authentication

1. **API Key Authentication**
   ```http
   Authorization: Bearer <api-key>
   ```

2. **JWT Authentication**
   ```http
   Authorization: Bearer <jwt-token>
   ```

### Message Security

1. **HMAC Signatures** (optional)
   - Per-connection session keys
   - SHA-256 based HMAC
   - Prevents tampering

2. **Anti-Replay Protection**
   - Message ID tracking
   - Timestamp validation
   - Nonce verification

### Rate Limiting

- **Per-Connection**: Token bucket algorithm
- **Per-IP**: Separate rate limiting for IP addresses
- **Per-Tenant**: Tenant-specific limits

Default limits:
- 1000 messages per minute per connection
- Burst capacity of 100 messages
- Configurable per deployment

## Performance Optimizations

### Object Pooling

```go
// Message pooling
msg := GetMessage()
defer PutMessage(msg)

// Buffer pooling
buf := GetBuffer()
defer PutBuffer(buf)
```

### Message Batching

- Automatic batching of small messages
- Configurable batch size and flush interval
- Reduces system calls and network overhead

### Connection Pooling

- Pre-allocated connection structures
- Reuse of goroutines
- Reduced GC pressure

### Zero-Copy Operations

- Direct buffer manipulation
- Avoiding unnecessary allocations
- Efficient memory usage

## API Reference

### WebSocket Endpoints

- `GET /ws` - Main WebSocket endpoint
- `GET /api/v1/websocket/stats` - Connection statistics
- `GET /api/v1/websocket/health` - Health check
- `GET /api/v1/websocket/metrics` - Prometheus metrics

### Client Libraries

#### JavaScript/TypeScript
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onopen = () => {
  ws.send(JSON.stringify({
    id: crypto.randomUUID(),
    type: 0,
    method: 'initialize',
    params: { name: 'my-agent', version: '1.0.0' }
  }));
};
```

#### Go
```go
import "github.com/coder/websocket"

conn, _, err := websocket.Dial(ctx, "ws://localhost:8080/ws", &websocket.DialOptions{
    HTTPHeader: http.Header{
        "Authorization": []string{"Bearer " + apiKey},
    },
})
```

#### Python
```python
import websockets
import json

async with websockets.connect(
    'ws://localhost:8080/ws',
    extra_headers={"Authorization": f"Bearer {api_key}"}
) as websocket:
    await websocket.send(json.dumps({
        "id": str(uuid.uuid4()),
        "type": 0,
        "method": "initialize",
        "params": {"name": "my-agent", "version": "1.0.0"}
    }))
```

## Configuration

### WebSocket Configuration

```yaml
websocket:
  enabled: true
  max_connections: 1000
  read_buffer_size: 4096
  write_buffer_size: 4096
  ping_interval: 30s
  pong_timeout: 60s
  max_message_size: 1048576  # 1MB
  
  security:
    require_auth: true
    hmac_signatures: false
    allowed_origins:
      - "*"
    max_frame_size: 1048576
    
  rate_limit:
    enabled: true
    rate: 16.67  # 1000 per minute
    burst: 100
    per_ip: true
    per_user: true
```

### Environment Variables

- `WEBSOCKET_ENABLED` - Enable/disable WebSocket server
- `WEBSOCKET_MAX_CONNECTIONS` - Maximum concurrent connections
- `WEBSOCKET_AUTH_REQUIRED` - Require authentication
- `WEBSOCKET_RATE_LIMIT` - Messages per minute

## Testing

### Unit Tests
```bash
make test-websocket
```

### Functional Tests
```bash
make test-functional-websocket
```

### Performance Tests
```bash
make test-websocket-performance
```

### Load Tests
```bash
# Default load test (10 connections, 100 messages each)
make test-websocket-load

# Custom load test
make test-websocket-load-custom CONNECTIONS=50 MESSAGES=200 DURATION=120
```

## Monitoring

### Metrics

- `websocket_connections_total` - Total connections created
- `websocket_connections_active` - Currently active connections
- `websocket_messages_sent` - Total messages sent
- `websocket_messages_received` - Total messages received
- `websocket_message_latency` - Message processing latency
- `websocket_errors_total` - Total errors by type

### Health Checks

```bash
curl http://localhost:8080/api/v1/websocket/health
```

Response:
```json
{
  "status": "healthy",
  "checks": {
    "connections": {
      "status": "healthy",
      "active": 45,
      "max": 1000,
      "utilization": 0.045
    },
    "rate_limiter": {
      "status": "healthy",
      "error_rate": 0.001
    }
  }
}
```

## Troubleshooting

### Common Issues

1. **Connection Refused**
   - Check if WebSocket is enabled in configuration
   - Verify the server is running
   - Check firewall rules

2. **Authentication Failed**
   - Verify API key or JWT token
   - Check token expiration
   - Ensure proper Authorization header format

3. **Rate Limiting**
   - Check current rate limit configuration
   - Monitor message frequency
   - Consider batching messages

4. **High Latency**
   - Check network conditions
   - Monitor server load
   - Consider using binary protocol
   - Enable message batching

5. **Connection Drops**
   - Check ping/pong intervals
   - Monitor network stability
   - Review timeout settings

### Debug Mode

Enable debug logging:
```yaml
logging:
  level: debug
  websocket:
    verbose: true
```

### Performance Tuning

1. **For High Throughput**
   - Increase buffer sizes
   - Enable binary protocol
   - Use message batching
   - Increase connection pool size

2. **For Low Latency**
   - Disable message batching
   - Use smaller buffer sizes
   - Enable TCP_NODELAY
   - Optimize handler code

3. **For Many Connections**
   - Increase max connections limit
   - Enable connection pooling
   - Use efficient data structures
   - Monitor memory usage

## Best Practices

1. **Always Initialize**: Send initialize message after connection
2. **Handle Errors**: Implement proper error handling
3. **Use IDs**: Include unique IDs in all requests
4. **Batch When Possible**: Group related operations
5. **Monitor Metrics**: Track performance and errors
6. **Implement Reconnection**: Handle connection failures gracefully
7. **Validate Input**: Sanitize all user input
8. **Use Binary Protocol**: For performance-critical applications
9. **Rate Limit Clients**: Implement client-side rate limiting
10. **Clean Shutdown**: Close connections properly