package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "sync"
    "sync/atomic"
    "time"
    
    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
    "github.com/google/uuid"
)

// Message types
type Message struct {
    ID     string      `json:"id"`
    Type   int         `json:"type"`
    Method string      `json:"method,omitempty"`
    Params interface{} `json:"params,omitempty"`
    Result interface{} `json:"result,omitempty"`
    Error  *Error      `json:"error,omitempty"`
}

type Error struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// Metrics
type Metrics struct {
    ConnectionsCreated   int64
    ConnectionsFailed    int64
    MessagesSent        int64
    MessagesReceived    int64
    Errors              int64
    TotalLatency        int64 // in microseconds
    LatencyCount        int64
    MinLatency          int64
    MaxLatency          int64
}

func (m *Metrics) RecordLatency(latency time.Duration) {
    us := latency.Microseconds()
    atomic.AddInt64(&m.TotalLatency, us)
    atomic.AddInt64(&m.LatencyCount, 1)
    
    // Update min/max
    for {
        min := atomic.LoadInt64(&m.MinLatency)
        if min == 0 || us < min {
            if atomic.CompareAndSwapInt64(&m.MinLatency, min, us) {
                break
            }
        } else {
            break
        }
    }
    
    for {
        max := atomic.LoadInt64(&m.MaxLatency)
        if us > max {
            if atomic.CompareAndSwapInt64(&m.MaxLatency, max, us) {
                break
            }
        } else {
            break
        }
    }
}

func runClient(ctx context.Context, id int, wsURL, apiKey string, messagesPerClient int, metrics *Metrics, wg *sync.WaitGroup) {
    defer wg.Done()
    
    opts := &websocket.DialOptions{
        HTTPHeader: http.Header{
            "Authorization": []string{"Bearer " + apiKey},
        },
    }
    
    conn, _, err := websocket.Dial(ctx, wsURL, opts)
    if err != nil {
        atomic.AddInt64(&metrics.ConnectionsFailed, 1)
        log.Printf("Client %d: Failed to connect: %v", id, err)
        return
    }
    defer func() {
        if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
            log.Printf("Client %d: Error closing connection: %v", id, err)
        }
    }()
    
    atomic.AddInt64(&metrics.ConnectionsCreated, 1)
    
    // Initialize connection
    initMsg := Message{
        ID:     uuid.New().String(),
        Type:   0, // Request
        Method: "initialize",
        Params: map[string]interface{}{
            "name":    fmt.Sprintf("load-test-agent-%d", id),
            "version": "1.0.0",
        },
    }
    
    err = wsjson.Write(ctx, conn, initMsg)
    if err != nil {
        atomic.AddInt64(&metrics.Errors, 1)
        log.Printf("Client %d: Failed to send init message: %v", id, err)
        return
    }
    atomic.AddInt64(&metrics.MessagesSent, 1)
    
    var initResp Message
    err = wsjson.Read(ctx, conn, &initResp)
    if err != nil {
        atomic.AddInt64(&metrics.Errors, 1)
        log.Printf("Client %d: Failed to read init response: %v", id, err)
        return
    }
    atomic.AddInt64(&metrics.MessagesReceived, 1)
    
    if initResp.Error != nil {
        atomic.AddInt64(&metrics.Errors, 1)
        log.Printf("Client %d: Init error: %v", id, initResp.Error.Message)
        return
    }
    
    // Send messages
    for i := 0; i < messagesPerClient; i++ {
        select {
        case <-ctx.Done():
            return
        default:
        }
        
        // Vary the message type
        var msg Message
        switch i % 3 {
        case 0:
            msg = Message{
                ID:     fmt.Sprintf("client-%d-msg-%d", id, i),
                Type:   0, // Request
                Method: "tool.list",
            }
        case 1:
            msg = Message{
                ID:     fmt.Sprintf("client-%d-msg-%d", id, i),
                Type:   0, // Request
                Method: "context.create",
                Params: map[string]interface{}{
                    "name":    fmt.Sprintf("test-context-%d-%d", id, i),
                    "content": "Load test context",
                },
            }
        case 2:
            msg = Message{
                ID:     fmt.Sprintf("client-%d-msg-%d", id, i),
                Type:   0, // Request
                Method: "context.list",
                Params: map[string]interface{}{
                    "limit": 10,
                },
            }
        }
        
        start := time.Now()
        err = wsjson.Write(ctx, conn, msg)
        if err != nil {
            atomic.AddInt64(&metrics.Errors, 1)
            continue
        }
        atomic.AddInt64(&metrics.MessagesSent, 1)
        
        var resp Message
        err = wsjson.Read(ctx, conn, &resp)
        if err != nil {
            atomic.AddInt64(&metrics.Errors, 1)
            continue
        }
        atomic.AddInt64(&metrics.MessagesReceived, 1)
        
        latency := time.Since(start)
        metrics.RecordLatency(latency)
        
        if resp.Error != nil {
            atomic.AddInt64(&metrics.Errors, 1)
        }
        
        // Small delay between messages
        time.Sleep(100 * time.Millisecond)
    }
}

func main() {
    var (
        wsURL        = flag.String("url", "ws://localhost:8080/ws", "WebSocket URL")
        apiKey       = flag.String("apikey", "test-key-admin", "API key")
        numClients   = flag.Int("connections", 10, "Number of concurrent connections")
        messagesPerClient = flag.Int("messages", 100, "Messages per connection")
        duration     = flag.Duration("duration", 60*time.Second, "Test duration")
    )
    flag.Parse()
    
    fmt.Println("WebSocket Load Test")
    fmt.Printf("URL: %s\n", *wsURL)
    fmt.Printf("Connections: %d\n", *numClients)
    fmt.Printf("Messages per connection: %d\n", *messagesPerClient)
    fmt.Printf("Duration: %v\n", *duration)
    fmt.Println()
    
    ctx, cancel := context.WithTimeout(context.Background(), *duration)
    defer cancel()
    
    metrics := &Metrics{}
    start := time.Now()
    
    var wg sync.WaitGroup
    
    // Start clients with some delay to avoid thundering herd
    for i := 0; i < *numClients; i++ {
        wg.Add(1)
        go runClient(ctx, i, *wsURL, *apiKey, *messagesPerClient, metrics, &wg)
        time.Sleep(50 * time.Millisecond) // Stagger connections
    }
    
    // Wait for all clients to finish
    wg.Wait()
    
    elapsed := time.Since(start)
    
    // Print results
    fmt.Println("\n=== Load Test Results ===")
    fmt.Printf("Duration: %v\n", elapsed)
    fmt.Printf("\nConnection Metrics:\n")
    fmt.Printf("  Created: %d\n", atomic.LoadInt64(&metrics.ConnectionsCreated))
    fmt.Printf("  Failed: %d\n", atomic.LoadInt64(&metrics.ConnectionsFailed))
    
    fmt.Printf("\nMessage Metrics:\n")
    fmt.Printf("  Sent: %d\n", atomic.LoadInt64(&metrics.MessagesSent))
    fmt.Printf("  Received: %d\n", atomic.LoadInt64(&metrics.MessagesReceived))
    fmt.Printf("  Errors: %d\n", atomic.LoadInt64(&metrics.Errors))
    
    sent := atomic.LoadInt64(&metrics.MessagesSent)
    if sent > 0 {
        fmt.Printf("  Success Rate: %.2f%%\n", float64(sent-atomic.LoadInt64(&metrics.Errors))/float64(sent)*100)
    }
    
    fmt.Printf("\nLatency Metrics:\n")
    count := atomic.LoadInt64(&metrics.LatencyCount)
    if count > 0 {
        avgLatency := atomic.LoadInt64(&metrics.TotalLatency) / count
        fmt.Printf("  Average: %.2fms\n", float64(avgLatency)/1000)
        fmt.Printf("  Min: %.2fms\n", float64(atomic.LoadInt64(&metrics.MinLatency))/1000)
        fmt.Printf("  Max: %.2fms\n", float64(atomic.LoadInt64(&metrics.MaxLatency))/1000)
    }
    
    fmt.Printf("\nThroughput:\n")
    fmt.Printf("  Messages/sec: %.2f\n", float64(sent)/elapsed.Seconds())
    
    // Exit with error if too many failures
    errorRate := float64(atomic.LoadInt64(&metrics.Errors)) / float64(sent)
    if errorRate > 0.1 { // More than 10% errors
        fmt.Printf("\nERROR: High error rate: %.2f%%\n", errorRate*100)
        log.Fatal("Load test failed due to high error rate")
    }
}