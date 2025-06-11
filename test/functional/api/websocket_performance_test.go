package api

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "strings"
    "sync"
    "sync/atomic"
    "time"
    
    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
    "github.com/google/uuid"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

var _ = Describe("WebSocket Performance Tests", func() {
    var (
        wsURL   string
        apiKey  string
        baseURL string
    )
    
    BeforeEach(func() {
        // Get test configuration
        baseURL = os.Getenv("MCP_SERVER_URL")
        if baseURL == "" {
            baseURL = "http://localhost:8080"
        }
        
        // Convert HTTP URL to WebSocket URL
        wsURL = strings.Replace(baseURL, "http://", "ws://", 1)
        wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
        wsURL = wsURL + "/ws"
        
        apiKey = os.Getenv("TEST_API_KEY")
        if apiKey == "" {
            apiKey = "test-key-admin"
        }
    })
    
    Describe("Throughput Testing", func() {
        It("should handle high message throughput", func() {
            ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
            defer cancel()
            
            opts := &websocket.DialOptions{
                HTTPHeader: http.Header{
                    "Authorization": []string{"Bearer " + apiKey},
                },
            }
            
            conn, _, err := websocket.Dial(ctx, wsURL, opts)
            Expect(err).NotTo(HaveOccurred())
            defer func() {
                if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
                    GinkgoWriter.Printf("Error closing connection: %v\n", err)
                }
            }()
            
            // Initialize connection
            initMsg := ws.Message{
                ID:     uuid.New().String(),
                Type:   ws.MessageTypeRequest,
                Method: "initialize",
                Params: map[string]interface{}{
                    "name":    "perf-test-agent",
                    "version": "1.0.0",
                },
            }
            
            err = wsjson.Write(ctx, conn, initMsg)
            Expect(err).NotTo(HaveOccurred())
            
            var initResp ws.Message
            err = wsjson.Read(ctx, conn, &initResp)
            Expect(err).NotTo(HaveOccurred())
            
            // Performance test parameters
            numMessages := 1000
            var sentCount int64
            var receivedCount int64
            var errors int64
            
            // Start receiver goroutine
            var wg sync.WaitGroup
            wg.Add(1)
            go func() {
                defer wg.Done()
                for {
                    var response ws.Message
                    err := wsjson.Read(ctx, conn, &response)
                    if err != nil {
                        if websocket.CloseStatus(err) != -1 {
                            return
                        }
                        atomic.AddInt64(&errors, 1)
                        continue
                    }
                    atomic.AddInt64(&receivedCount, 1)
                    if atomic.LoadInt64(&receivedCount) >= int64(numMessages) {
                        return
                    }
                }
            }()
            
            // Send messages
            start := time.Now()
            for i := 0; i < numMessages; i++ {
                msg := ws.Message{
                    ID:     fmt.Sprintf("perf-%d", i),
                    Type:   ws.MessageTypeRequest,
                    Method: "tool.list",
                }
                
                err := wsjson.Write(ctx, conn, msg)
                if err != nil {
                    atomic.AddInt64(&errors, 1)
                    continue
                }
                atomic.AddInt64(&sentCount, 1)
            }
            
            // Wait for all responses or timeout
            done := make(chan bool)
            go func() {
                wg.Wait()
                close(done)
            }()
            
            select {
            case <-done:
                // Success
            case <-time.After(30 * time.Second):
                // Timeout
            }
            
            duration := time.Since(start)
            messagesPerSecond := float64(atomic.LoadInt64(&sentCount)) / duration.Seconds()
            
            fmt.Printf("Performance Results:\n")
            fmt.Printf("  Messages sent: %d\n", atomic.LoadInt64(&sentCount))
            fmt.Printf("  Messages received: %d\n", atomic.LoadInt64(&receivedCount))
            fmt.Printf("  Errors: %d\n", atomic.LoadInt64(&errors))
            fmt.Printf("  Duration: %v\n", duration)
            fmt.Printf("  Throughput: %.2f msgs/sec\n", messagesPerSecond)
            
            // Assertions
            Expect(atomic.LoadInt64(&errors)).To(BeNumerically("<", numMessages/10)) // Less than 10% errors
            Expect(messagesPerSecond).To(BeNumerically(">", 100)) // At least 100 msgs/sec
        })
    })
    
    Describe("Concurrent Connections", func() {
        It("should handle multiple concurrent connections efficiently", func() {
            ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
            defer cancel()
            
            numConnections := 10
            messagesPerConnection := 100
            
            var totalSent int64
            var totalReceived int64
            var totalErrors int64
            var wg sync.WaitGroup
            
            start := time.Now()
            
            for i := 0; i < numConnections; i++ {
                wg.Add(1)
                go func(connID int) {
                    defer wg.Done()
                    
                    opts := &websocket.DialOptions{
                        HTTPHeader: http.Header{
                            "Authorization": []string{"Bearer " + apiKey},
                        },
                    }
                    
                    conn, _, err := websocket.Dial(ctx, wsURL, opts)
                    if err != nil {
                        atomic.AddInt64(&totalErrors, 1)
                        return
                    }
                    defer func() {
                if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
                    GinkgoWriter.Printf("Error closing connection: %v\n", err)
                }
            }()
                    
                    // Initialize
                    initMsg := ws.Message{
                        ID:     uuid.New().String(),
                        Type:   ws.MessageTypeRequest,
                        Method: "initialize",
                        Params: map[string]interface{}{
                            "name":    fmt.Sprintf("concurrent-agent-%d", connID),
                            "version": "1.0.0",
                        },
                    }
                    
                    err = wsjson.Write(ctx, conn, initMsg)
                    if err != nil {
                        atomic.AddInt64(&totalErrors, 1)
                        return
                    }
                    
                    var initResp ws.Message
                    err = wsjson.Read(ctx, conn, &initResp)
                    if err != nil {
                        atomic.AddInt64(&totalErrors, 1)
                        return
                    }
                    
                    // Send messages
                    for j := 0; j < messagesPerConnection; j++ {
                        msg := ws.Message{
                            ID:     fmt.Sprintf("conn-%d-msg-%d", connID, j),
                            Type:   ws.MessageTypeRequest,
                            Method: "tool.list",
                        }
                        
                        err := wsjson.Write(ctx, conn, msg)
                        if err != nil {
                            atomic.AddInt64(&totalErrors, 1)
                            continue
                        }
                        atomic.AddInt64(&totalSent, 1)
                        
                        var response ws.Message
                        err = wsjson.Read(ctx, conn, &response)
                        if err != nil {
                            atomic.AddInt64(&totalErrors, 1)
                            continue
                        }
                        atomic.AddInt64(&totalReceived, 1)
                    }
                }(i)
            }
            
            wg.Wait()
            duration := time.Since(start)
            
            totalMessages := int64(numConnections * messagesPerConnection)
            throughput := float64(atomic.LoadInt64(&totalSent)) / duration.Seconds()
            
            fmt.Printf("Concurrent Connection Results:\n")
            fmt.Printf("  Connections: %d\n", numConnections)
            fmt.Printf("  Messages per connection: %d\n", messagesPerConnection)
            fmt.Printf("  Total messages sent: %d\n", atomic.LoadInt64(&totalSent))
            fmt.Printf("  Total messages received: %d\n", atomic.LoadInt64(&totalReceived))
            fmt.Printf("  Total errors: %d\n", atomic.LoadInt64(&totalErrors))
            fmt.Printf("  Duration: %v\n", duration)
            fmt.Printf("  Throughput: %.2f msgs/sec\n", throughput)
            
            // Assertions
            Expect(atomic.LoadInt64(&totalErrors)).To(BeNumerically("<", totalMessages/10)) // Less than 10% errors
            Expect(atomic.LoadInt64(&totalReceived)).To(BeNumerically(">", totalMessages*8/10)) // At least 80% success
        })
    })
    
    Describe("Message Latency", func() {
        It("should maintain low latency under load", func() {
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()
            
            opts := &websocket.DialOptions{
                HTTPHeader: http.Header{
                    "Authorization": []string{"Bearer " + apiKey},
                },
            }
            
            conn, _, err := websocket.Dial(ctx, wsURL, opts)
            Expect(err).NotTo(HaveOccurred())
            defer func() {
                if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
                    GinkgoWriter.Printf("Error closing connection: %v\n", err)
                }
            }()
            
            // Initialize
            initMsg := ws.Message{
                ID:     uuid.New().String(),
                Type:   ws.MessageTypeRequest,
                Method: "initialize",
                Params: map[string]interface{}{
                    "name":    "latency-test-agent",
                    "version": "1.0.0",
                },
            }
            
            err = wsjson.Write(ctx, conn, initMsg)
            Expect(err).NotTo(HaveOccurred())
            
            var initResp ws.Message
            err = wsjson.Read(ctx, conn, &initResp)
            Expect(err).NotTo(HaveOccurred())
            
            // Measure latencies
            numSamples := 100
            latencies := make([]time.Duration, 0, numSamples)
            
            for i := 0; i < numSamples; i++ {
                msg := ws.Message{
                    ID:     fmt.Sprintf("latency-%d", i),
                    Type:   ws.MessageTypeRequest,
                    Method: "tool.list",
                }
                
                start := time.Now()
                err := wsjson.Write(ctx, conn, msg)
                if err != nil {
                    continue
                }
                
                var response ws.Message
                err = wsjson.Read(ctx, conn, &response)
                if err != nil {
                    continue
                }
                
                latency := time.Since(start)
                latencies = append(latencies, latency)
                
                // Small delay between messages
                time.Sleep(10 * time.Millisecond)
            }
            
            // Calculate statistics
            var totalLatency time.Duration
            var maxLatency time.Duration
            minLatency := time.Hour
            
            for _, lat := range latencies {
                totalLatency += lat
                if lat > maxLatency {
                    maxLatency = lat
                }
                if lat < minLatency {
                    minLatency = lat
                }
            }
            
            avgLatency := totalLatency / time.Duration(len(latencies))
            
            fmt.Printf("Latency Results:\n")
            fmt.Printf("  Samples: %d\n", len(latencies))
            fmt.Printf("  Min latency: %v\n", minLatency)
            fmt.Printf("  Max latency: %v\n", maxLatency)
            fmt.Printf("  Avg latency: %v\n", avgLatency)
            
            // Assertions
            Expect(avgLatency).To(BeNumerically("<", 100*time.Millisecond)) // Average under 100ms
            Expect(maxLatency).To(BeNumerically("<", 500*time.Millisecond)) // Max under 500ms
        })
    })
})