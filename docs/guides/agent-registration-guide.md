# Agent Registration Guide

> **Purpose**: Step-by-step guide for registering AI agents with the DevOps MCP platform
> **Audience**: Developers integrating AI agents
> **Scope**: WebSocket connection and agent registration

## Overview

This guide explains how to register AI agents with the DevOps MCP platform. The registration process is straightforward:

1. **Connect via WebSocket** with authentication (API key or JWT token)
2. **Agent ID is assigned automatically** by the server based on your authentication
3. **Register your agent's capabilities** through WebSocket messages

## Important: WebSocket Client Requirements

⚠️ **Critical**: All WebSocket clients MUST request the `mcp.v1` subprotocol during connection. Without this, the server will reject your connection with HTTP 426 Upgrade Required.

### No Official SDK Yet

The DevOps MCP project doesn't provide an official client SDK yet. This guide shows how to connect using standard WebSocket libraries following the patterns used in the project's test suite.

## How Agent IDs Work

When you connect to the MCP server:
- **With JWT token**: Your agent ID will be your user ID from the JWT
- **With API key**: A new UUID will be generated as your agent ID
- **No manual ID required**: The server handles ID assignment automatically

## Quick Start (Go)

Using `github.com/coder/websocket` (the library used by DevOps MCP):

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    
    "github.com/coder/websocket"
)

func main() {
    ctx := context.Background()
    
    // CRITICAL: Include the mcp.v1 subprotocol
    dialOpts := &websocket.DialOptions{
        Subprotocols: []string{"mcp.v1"}, // REQUIRED!
        HTTPHeader: http.Header{
            "Authorization": []string{"Bearer " + apiKey},
        },
    }
    
    // For local development
    conn, _, err := websocket.Dial(ctx, "ws://localhost:8080/ws", dialOpts)
    // For production
    // conn, _, err := websocket.Dial(ctx, "wss://mcp.dev-mesh.io/ws", dialOpts)
    
    if err != nil {
        log.Fatal("dial failed:", err)
    }
    defer conn.Close(websocket.StatusNormalClosure, "")
    
    // Register agent capabilities
    registration := map[string]interface{}{
        "type":         "agent.register",
        "name":         "My Agent",
        "capabilities": []string{"code_analysis", "documentation"},
        "metadata": map[string]interface{}{
            "version": "1.0.0",
        },
    }
    
    // Send registration as JSON
    if err := wsjson.Write(ctx, conn, registration); err != nil {
        log.Fatal("send failed:", err)
    }
    
    // Read response
    var response map[string]interface{}
    if err := wsjson.Read(ctx, conn, &response); err != nil {
        log.Fatal("read failed:", err)
    }
    
    if response["type"] == "agent.registered" {
        fmt.Printf("Agent registered! ID: %s\n", response["agent_id"])
    }
}
```

## Quick Start (Other Languages)

### JavaScript/TypeScript
```javascript
// IMPORTANT: Include mcp.v1 in subprotocols array
const ws = new WebSocket('wss://mcp.dev-mesh.io/ws', ['mcp.v1']);

// Note: Authorization headers can't be set directly in browser WebSocket API
// You may need to use a query parameter or handle auth after connection
ws.onopen = () => {
    ws.send(JSON.stringify({
        type: 'agent.register',
        name: 'My Agent',
        capabilities: ['code_analysis', 'documentation'],
        metadata: { version: '1.0.0' }
    }));
};

ws.onmessage = (event) => {
    const response = JSON.parse(event.data);
    if (response.type === 'agent.registered') {
        console.log('Agent registered! ID:', response.agent_id);
    }
};
```

### Python
```python
import asyncio
import json
import websockets

async def register_agent():
    headers = {
        "Authorization": f"Bearer {api_key}"
    }
    
    # IMPORTANT: Include mcp.v1 subprotocol
    async with websockets.connect(
        'wss://mcp.dev-mesh.io/ws',
        subprotocols=['mcp.v1'],
        extra_headers=headers
    ) as websocket:
        # Send registration
        await websocket.send(json.dumps({
            'type': 'agent.register',
            'name': 'My Agent',
            'capabilities': ['code_analysis', 'documentation'],
            'metadata': {'version': '1.0.0'}
        }))
        
        # Receive response
        response = json.loads(await websocket.recv())
        if response['type'] == 'agent.registered':
            print(f"Agent registered! ID: {response['agent_id']}")

asyncio.run(register_agent())
```

## Complete Go Example (Based on Test Agent)

Here's a more complete example based on the project's test agent implementation:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"
    
    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
)

type Agent struct {
    conn         *websocket.Conn
    agentID      string
    name         string
    capabilities []string
    
    mu           sync.RWMutex
    connected    bool
}

func NewAgent(name string, capabilities []string) *Agent {
    return &Agent{
        name:         name,
        capabilities: capabilities,
    }
}

func (a *Agent) Connect(ctx context.Context, wsURL, apiKey string) error {
    dialOpts := &websocket.DialOptions{
        Subprotocols: []string{"mcp.v1"}, // REQUIRED!
        HTTPHeader: http.Header{
            "Authorization": []string{"Bearer " + apiKey},
        },
    }
    
    conn, _, err := websocket.Dial(ctx, wsURL, dialOpts)
    if err != nil {
        return fmt.Errorf("websocket dial failed: %w", err)
    }
    
    a.conn = conn
    a.connected = true
    
    // Start message handler
    go a.handleMessages(ctx)
    
    // Register agent
    return a.register(ctx)
}

func (a *Agent) register(ctx context.Context) error {
    msg := map[string]interface{}{
        "type":         "agent.register",
        "name":         a.name,
        "capabilities": a.capabilities,
        "metadata": map[string]interface{}{
            "version":   "1.0.0",
            "timestamp": time.Now().Unix(),
        },
    }
    
    if err := wsjson.Write(ctx, a.conn, msg); err != nil {
        return fmt.Errorf("failed to send registration: %w", err)
    }
    
    // Wait for registration response
    var response map[string]interface{}
    if err := wsjson.Read(ctx, a.conn, &response); err != nil {
        return fmt.Errorf("failed to read response: %w", err)
    }
    
    if response["type"] == "agent.registered" {
        a.agentID = response["agent_id"].(string)
        log.Printf("Agent registered successfully! ID: %s", a.agentID)
        return nil
    }
    
    return fmt.Errorf("registration failed: %v", response)
}

func (a *Agent) handleMessages(ctx context.Context) {
    for {
        var msg map[string]interface{}
        err := wsjson.Read(ctx, a.conn, &msg)
        if err != nil {
            log.Printf("Read error: %v", err)
            a.connected = false
            return
        }
        
        msgType, _ := msg["type"].(string)
        switch msgType {
        case "task.execute":
            a.handleTask(ctx, msg)
        case "ping":
            a.sendPong(ctx)
        default:
            log.Printf("Received message type: %s", msgType)
        }
    }
}

func (a *Agent) handleTask(ctx context.Context, msg map[string]interface{}) {
    taskID := msg["task_id"].(string)
    content := msg["content"].(string)
    
    log.Printf("Processing task %s: %s", taskID, content)
    
    // Send task result
    result := map[string]interface{}{
        "type":    "task.result",
        "task_id": taskID,
        "result": map[string]interface{}{
            "status": "completed",
            "output": "Task processed successfully",
        },
    }
    
    if err := wsjson.Write(ctx, a.conn, result); err != nil {
        log.Printf("Failed to send result: %v", err)
    }
}

func (a *Agent) sendPong(ctx context.Context) {
    pong := map[string]interface{}{"type": "pong"}
    if err := wsjson.Write(ctx, a.conn, pong); err != nil {
        log.Printf("Failed to send pong: %v", err)
    }
}

func (a *Agent) Close() error {
    if a.conn != nil {
        return a.conn.Close(websocket.StatusNormalClosure, "")
    }
    return nil
}

func main() {
    agent := NewAgent("Example Agent", []string{"code_analysis", "documentation"})
    
    ctx := context.Background()
    
    // Connect to local development server
    err := agent.Connect(ctx, "ws://localhost:8080/ws", "your-api-key")
    // For production: wss://mcp.dev-mesh.io/ws
    
    if err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer agent.Close()
    
    log.Printf("Agent %s connected and ready!", agent.agentID)
    
    // Keep the agent running
    select {}
}
```

## WebSocket Message Protocol

### Registration Message
```json
{
    "type": "agent.register",
    "name": "Agent Name",
    "capabilities": ["capability1", "capability2"],
    "metadata": {
        "version": "1.0.0",
        "custom_field": "value"
    }
}
```

### Registration Response
```json
{
    "type": "agent.registered",
    "agent_id": "generated-uuid",
    "name": "Agent Name",
    "capabilities": ["capability1", "capability2"],
    "registered_at": "2024-01-20T10:00:00Z"
}
```

### Error Response
```json
{
    "type": "error",
    "error": "Registration failed: reason",
    "code": 400
}
```

## Common Issues and Solutions

### HTTP 426 Upgrade Required
**Problem**: Connection rejected with 426 error  
**Solution**: Ensure you're including `Subprotocols: ["mcp.v1"]` in your dial options

### Authentication Failed
**Problem**: 401 Unauthorized error  
**Solution**: 
- Verify your API key is valid
- Ensure Authorization header format is `Bearer <token>`
- For JWT tokens, check expiration

### Connection Drops Immediately
**Problem**: Connection established but closes immediately  
**Solution**:
- Check server logs for detailed error messages
- Verify all required headers are being sent
- Ensure the WebSocket endpoint URL is correct (`/ws`)

### Using Wrong WebSocket Library
**Problem**: Code examples from other sources use different libraries  
**Solution**: While the server works with any compliant WebSocket client, this project's tests use `github.com/coder/websocket` for Go. You can use any library that supports:
- Custom subprotocols
- Custom headers for authentication
- JSON message encoding/decoding

## Binary Protocol Support

The MCP server supports a binary protocol for improved performance. This is automatically negotiated based on message size and type. The test agent implementation includes binary protocol support if you need this feature.

## Next Steps

- Review [WebSocket Client Requirements](../WEBSOCKET_CLIENT_REQUIREMENTS.md) for detailed protocol requirements
- See the [test agent implementation](../../test/e2e/agent/agent.go) for a complete example
- Check [MCP Server API Reference](../api-reference/mcp-server-reference.md) for all message types
- Learn about [Task Routing Algorithms](./task-routing-algorithms.md) for task distribution

## Note on SDK Development

The DevOps MCP project currently doesn't provide an official client SDK. The examples in this guide are based on the patterns used in the project's test suite. An official SDK may be developed in the future to simplify agent development.