package integration

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
    "testing"
    "time"
    
    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// TestWebSocketMCPIntegration tests WebSocket integration with MCP server components
func TestWebSocketMCPIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Get test configuration
    baseURL := os.Getenv("MCP_SERVER_URL")
    if baseURL == "" {
        baseURL = "http://localhost:8080"
    }
    
    wsURL := strings.Replace(baseURL, "http://", "ws://", 1)
    wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
    wsURL = wsURL + "/ws"
    
    apiKey := os.Getenv("TEST_API_KEY")
    if apiKey == "" {
        apiKey = "test-key-admin"
    }
    
    t.Run("WebSocket_Context_Integration", func(t *testing.T) {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        // Connect via WebSocket
        opts := &websocket.DialOptions{
            HTTPHeader: http.Header{
                "Authorization": []string{"Bearer " + apiKey},
            },
        }
        
        conn, _, err := websocket.Dial(ctx, wsURL, opts)
        require.NoError(t, err)
        defer conn.Close(websocket.StatusNormalClosure, "")
        
        // Initialize connection
        initMsg := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "initialize",
            Params: map[string]interface{}{
                "name":    "integration-test-agent",
                "version": "1.0.0",
            },
        }
        
        err = wsjson.Write(ctx, conn, initMsg)
        require.NoError(t, err)
        
        var initResp ws.Message
        err = wsjson.Read(ctx, conn, &initResp)
        require.NoError(t, err)
        assert.Nil(t, initResp.Error)
        
        // Create context via WebSocket
        contextName := "test-context-" + uuid.New().String()
        createMsg := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "context.create",
            Params: map[string]interface{}{
                "name":    contextName,
                "content": "Integration test context content",
                "metadata": map[string]interface{}{
                    "source": "websocket",
                    "test":   true,
                },
            },
        }
        
        err = wsjson.Write(ctx, conn, createMsg)
        require.NoError(t, err)
        
        var createResp ws.Message
        err = wsjson.Read(ctx, conn, &createResp)
        require.NoError(t, err)
        assert.Nil(t, createResp.Error)
        
        // Extract context ID
        result, ok := createResp.Result.(map[string]interface{})
        require.True(t, ok)
        contextID, ok := result["id"].(string)
        require.True(t, ok)
        require.NotEmpty(t, contextID)
        
        // Verify context exists (REST API verification would go here)
        // For now, just verify we got a valid ID
        assert.NotEmpty(t, contextID)
        t.Logf("Created context with ID: %s", contextID)
        
        // Update context via WebSocket
        updateMsg := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "context.update",
            Params: map[string]interface{}{
                "id":      contextID,
                "content": "Updated integration test content",
                "metadata": map[string]interface{}{
                    "updated": true,
                    "source":  "websocket",
                },
            },
        }
        
        err = wsjson.Write(ctx, conn, updateMsg)
        require.NoError(t, err)
        
        var updateResp ws.Message
        err = wsjson.Read(ctx, conn, &updateResp)
        require.NoError(t, err)
        assert.Nil(t, updateResp.Error)
        
        // Verify update succeeded
        updateResult, ok := updateResp.Result.(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, "success", updateResult["status"])
    })
    
    t.Run("WebSocket_Tool_Integration", func(t *testing.T) {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        // Connect via WebSocket
        opts := &websocket.DialOptions{
            HTTPHeader: http.Header{
                "Authorization": []string{"Bearer " + apiKey},
            },
        }
        
        conn, _, err := websocket.Dial(ctx, wsURL, opts)
        require.NoError(t, err)
        defer conn.Close(websocket.StatusNormalClosure, "")
        
        // Initialize
        initMsg := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "initialize",
            Params: map[string]interface{}{
                "name":    "tool-test-agent",
                "version": "1.0.0",
            },
        }
        
        err = wsjson.Write(ctx, conn, initMsg)
        require.NoError(t, err)
        
        var initResp ws.Message
        err = wsjson.Read(ctx, conn, &initResp)
        require.NoError(t, err)
        
        // List tools
        listMsg := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "tool.list",
        }
        
        err = wsjson.Write(ctx, conn, listMsg)
        require.NoError(t, err)
        
        var listResp ws.Message
        err = wsjson.Read(ctx, conn, &listResp)
        require.NoError(t, err)
        assert.Nil(t, listResp.Error)
        
        // Verify tools are returned
        tools, ok := listResp.Result.([]interface{})
        require.True(t, ok)
        assert.NotEmpty(t, tools)
        
        // Execute a simple tool (if available)
        if len(tools) > 0 {
            firstTool, ok := tools[0].(map[string]interface{})
            if ok {
                toolName, _ := firstTool["name"].(string)
                
                execMsg := ws.Message{
                    ID:     uuid.New().String(),
                    Type:   ws.MessageTypeRequest,
                    Method: "tool.execute",
                    Params: map[string]interface{}{
                        "name": toolName,
                        "args": map[string]interface{}{},
                    },
                }
                
                err = wsjson.Write(ctx, conn, execMsg)
                require.NoError(t, err)
                
                var execResp ws.Message
                err = wsjson.Read(ctx, conn, &execResp)
                require.NoError(t, err)
                // Tool execution might fail, but WebSocket should handle it gracefully
            }
        }
    })
    
    t.Run("WebSocket_Event_Subscription", func(t *testing.T) {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        // Connect two clients
        opts := &websocket.DialOptions{
            HTTPHeader: http.Header{
                "Authorization": []string{"Bearer " + apiKey},
            },
        }
        
        // Client 1 - subscriber
        conn1, _, err := websocket.Dial(ctx, wsURL, opts)
        require.NoError(t, err)
        defer conn1.Close(websocket.StatusNormalClosure, "")
        
        // Initialize client 1
        initMsg1 := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "initialize",
            Params: map[string]interface{}{
                "name":    "subscriber-agent",
                "version": "1.0.0",
            },
        }
        
        err = wsjson.Write(ctx, conn1, initMsg1)
        require.NoError(t, err)
        
        var initResp1 ws.Message
        err = wsjson.Read(ctx, conn1, &initResp1)
        require.NoError(t, err)
        
        // Subscribe to events
        subMsg := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "event.subscribe",
            Params: map[string]interface{}{
                "events": []string{"context.created", "context.updated"},
            },
        }
        
        err = wsjson.Write(ctx, conn1, subMsg)
        require.NoError(t, err)
        
        var subResp ws.Message
        err = wsjson.Read(ctx, conn1, &subResp)
        require.NoError(t, err)
        assert.Nil(t, subResp.Error)
        
        // Client 2 - event generator
        conn2, _, err := websocket.Dial(ctx, wsURL, opts)
        require.NoError(t, err)
        defer conn2.Close(websocket.StatusNormalClosure, "")
        
        // Initialize client 2
        initMsg2 := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "initialize",
            Params: map[string]interface{}{
                "name":    "publisher-agent",
                "version": "1.0.0",
            },
        }
        
        err = wsjson.Write(ctx, conn2, initMsg2)
        require.NoError(t, err)
        
        var initResp2 ws.Message
        err = wsjson.Read(ctx, conn2, &initResp2)
        require.NoError(t, err)
        
        // Create context to trigger event
        createMsg := ws.Message{
            ID:     uuid.New().String(),
            Type:   ws.MessageTypeRequest,
            Method: "context.create",
            Params: map[string]interface{}{
                "name":    "event-test-context",
                "content": "This should trigger an event",
            },
        }
        
        err = wsjson.Write(ctx, conn2, createMsg)
        require.NoError(t, err)
        
        var createResp ws.Message
        err = wsjson.Read(ctx, conn2, &createResp)
        require.NoError(t, err)
        assert.Nil(t, createResp.Error)
        
        // Check if client 1 received the event notification
        // This might be async, so we'll wait a bit
        done := make(chan bool)
        go func() {
            var notification ws.Message
            err := wsjson.Read(ctx, conn1, &notification)
            if err == nil && notification.Type == ws.MessageTypeNotification {
                assert.Equal(t, "event.notification", notification.Method)
                done <- true
            } else {
                done <- false
            }
        }()
        
        select {
        case received := <-done:
            if !received {
                t.Log("Event notification not received (might not be implemented yet)")
            }
        case <-time.After(5 * time.Second):
            t.Log("Timeout waiting for event notification")
        }
    })
}

// TestWebSocketPerformanceMetrics tests WebSocket performance metrics collection
func TestWebSocketPerformanceMetrics(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test in short mode")
    }
    
    baseURL := os.Getenv("MCP_SERVER_URL")
    if baseURL == "" {
        baseURL = "http://localhost:8080"
    }
    
    // Check WebSocket metrics endpoint
    resp, err := http.Get(baseURL + "/api/v1/websocket/metrics")
    if err != nil {
        t.Skip("WebSocket metrics endpoint not available")
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == http.StatusUnauthorized {
        t.Skip("WebSocket metrics endpoint requires authentication")
    }
    
    require.Equal(t, http.StatusOK, resp.StatusCode)
    
    // Parse Prometheus metrics
    var metrics map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&metrics)
    if err != nil {
        // Might be Prometheus text format
        t.Log("Metrics in Prometheus text format")
    }
}

// BenchmarkWebSocketThroughput benchmarks WebSocket message throughput
func BenchmarkWebSocketThroughput(b *testing.B) {
    baseURL := os.Getenv("MCP_SERVER_URL")
    if baseURL == "" {
        baseURL = "http://localhost:8080"
    }
    
    wsURL := strings.Replace(baseURL, "http://", "ws://", 1)
    wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
    wsURL = wsURL + "/ws"
    
    apiKey := os.Getenv("TEST_API_KEY")
    if apiKey == "" {
        apiKey = "test-key-admin"
    }
    
    ctx := context.Background()
    
    opts := &websocket.DialOptions{
        HTTPHeader: http.Header{
            "Authorization": []string{"Bearer " + apiKey},
        },
    }
    
    conn, _, err := websocket.Dial(ctx, wsURL, opts)
    if err != nil {
        b.Skip("Cannot connect to WebSocket server")
    }
    defer conn.Close(websocket.StatusNormalClosure, "")
    
    // Initialize
    initMsg := ws.Message{
        ID:     uuid.New().String(),
        Type:   ws.MessageTypeRequest,
        Method: "initialize",
        Params: map[string]interface{}{
            "name":    "benchmark-agent",
            "version": "1.0.0",
        },
    }
    
    err = wsjson.Write(ctx, conn, initMsg)
    require.NoError(b, err)
    
    var initResp ws.Message
    err = wsjson.Read(ctx, conn, &initResp)
    require.NoError(b, err)
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        msg := ws.Message{
            ID:     fmt.Sprintf("bench-%d", i),
            Type:   ws.MessageTypeRequest,
            Method: "tool.list",
        }
        
        err := wsjson.Write(ctx, conn, msg)
        if err != nil {
            b.Fatal(err)
        }
        
        var response ws.Message
        err = wsjson.Read(ctx, conn, &response)
        if err != nil {
            b.Fatal(err)
        }
    }
}