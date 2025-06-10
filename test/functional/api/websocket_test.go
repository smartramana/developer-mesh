package api

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
    "sync"
    _ "testing" // Required for Ginkgo
    "time"
    
    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
    "github.com/google/uuid"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    
    ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

var _ = Describe("WebSocket API Functional Tests", func() {
    var (
        wsURL      string
        apiKey     string
        baseURL    string
        httpClient *http.Client
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
        
        httpClient = &http.Client{
            Timeout: 30 * time.Second,
        }
    })
    
    Describe("Connection Management", func() {
        It("should connect with valid API key", func() {
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()
            
            opts := &websocket.DialOptions{
                HTTPHeader: http.Header{
                    "Authorization": []string{"Bearer " + apiKey},
                },
            }
            
            conn, resp, err := websocket.Dial(ctx, wsURL, opts)
            Expect(err).NotTo(HaveOccurred())
            Expect(resp.StatusCode).To(Equal(http.StatusSwitchingProtocols))
            defer conn.Close(websocket.StatusNormalClosure, "")
            
            // Send initialize message
            initMsg := ws.Message{
                ID:     uuid.New().String(),
                Type:   ws.MessageTypeRequest,
                Method: "initialize",
                Params: map[string]interface{}{
                    "name":    "test-agent",
                    "version": "1.0.0",
                },
            }
            
            err = wsjson.Write(ctx, conn, initMsg)
            Expect(err).NotTo(HaveOccurred())
            
            // Read response
            var response ws.Message
            err = wsjson.Read(ctx, conn, &response)
            Expect(err).NotTo(HaveOccurred())
            Expect(response.Type).To(Equal(ws.MessageTypeResponse))
            Expect(response.ID).To(Equal(initMsg.ID))
            Expect(response.Error).To(BeNil())
        })
        
        It("should reject connection without authorization", func() {
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            
            _, resp, err := websocket.Dial(ctx, wsURL, nil)
            Expect(err).To(HaveOccurred())
            if resp != nil {
                Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
            }
        })
        
        It("should reject connection with invalid API key", func() {
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            
            opts := &websocket.DialOptions{
                HTTPHeader: http.Header{
                    "Authorization": []string{"Bearer invalid-key"},
                },
            }
            
            _, resp, err := websocket.Dial(ctx, wsURL, opts)
            Expect(err).To(HaveOccurred())
            if resp != nil {
                Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
            }
        })
        
        It("should handle multiple concurrent connections", func() {
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()
            
            numConnections := 5
            connections := make([]*websocket.Conn, numConnections)
            var wg sync.WaitGroup
            var mu sync.Mutex
            errors := make([]error, 0)
            
            for i := 0; i < numConnections; i++ {
                wg.Add(1)
                go func(idx int) {
                    defer wg.Done()
                    
                    opts := &websocket.DialOptions{
                        HTTPHeader: http.Header{
                            "Authorization": []string{"Bearer " + apiKey},
                        },
                    }
                    
                    conn, _, err := websocket.Dial(ctx, wsURL, opts)
                    if err != nil {
                        mu.Lock()
                        errors = append(errors, err)
                        mu.Unlock()
                        return
                    }
                    
                    mu.Lock()
                    connections[idx] = conn
                    mu.Unlock()
                }(i)
            }
            
            wg.Wait()
            Expect(errors).To(BeEmpty())
            
            // Clean up connections
            for _, conn := range connections {
                if conn != nil {
                    conn.Close(websocket.StatusNormalClosure, "")
                }
            }
        })
    })
    
    Describe("Message Exchange", func() {
        var conn *websocket.Conn
        var ctx context.Context
        var cancel context.CancelFunc
        
        BeforeEach(func() {
            ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
            
            opts := &websocket.DialOptions{
                HTTPHeader: http.Header{
                    "Authorization": []string{"Bearer " + apiKey},
                },
            }
            
            var err error
            conn, _, err = websocket.Dial(ctx, wsURL, opts)
            Expect(err).NotTo(HaveOccurred())
            
            // Initialize connection
            initMsg := ws.Message{
                ID:     uuid.New().String(),
                Type:   ws.MessageTypeRequest,
                Method: "initialize",
                Params: map[string]interface{}{
                    "name":    "test-agent",
                    "version": "1.0.0",
                },
            }
            
            err = wsjson.Write(ctx, conn, initMsg)
            Expect(err).NotTo(HaveOccurred())
            
            var response ws.Message
            err = wsjson.Read(ctx, conn, &response)
            Expect(err).NotTo(HaveOccurred())
        })
        
        AfterEach(func() {
            if conn != nil {
                conn.Close(websocket.StatusNormalClosure, "")
            }
            cancel()
        })
        
        It("should handle tool.list request", func() {
            msg := ws.Message{
                ID:     uuid.New().String(),
                Type:   ws.MessageTypeRequest,
                Method: "tool.list",
            }
            
            err := wsjson.Write(ctx, conn, msg)
            Expect(err).NotTo(HaveOccurred())
            
            var response ws.Message
            err = wsjson.Read(ctx, conn, &response)
            Expect(err).NotTo(HaveOccurred())
            Expect(response.Type).To(Equal(ws.MessageTypeResponse))
            Expect(response.ID).To(Equal(msg.ID))
            Expect(response.Error).To(BeNil())
            Expect(response.Result).NotTo(BeNil())
        })
        
        It("should handle context operations", func() {
            // Create context
            createMsg := ws.Message{
                ID:     uuid.New().String(),
                Type:   ws.MessageTypeRequest,
                Method: "context.create",
                Params: map[string]interface{}{
                    "name":    "test-context-" + uuid.New().String(),
                    "content": "This is a test context",
                },
            }
            
            err := wsjson.Write(ctx, conn, createMsg)
            Expect(err).NotTo(HaveOccurred())
            
            var createResp ws.Message
            err = wsjson.Read(ctx, conn, &createResp)
            Expect(err).NotTo(HaveOccurred())
            Expect(createResp.Error).To(BeNil())
            
            // Extract context ID
            result, ok := createResp.Result.(map[string]interface{})
            Expect(ok).To(BeTrue())
            contextID, ok := result["id"].(string)
            Expect(ok).To(BeTrue())
            Expect(contextID).NotTo(BeEmpty())
            
            // Get context
            getMsg := ws.Message{
                ID:     uuid.New().String(),
                Type:   ws.MessageTypeRequest,
                Method: "context.get",
                Params: map[string]interface{}{
                    "id": contextID,
                },
            }
            
            err = wsjson.Write(ctx, conn, getMsg)
            Expect(err).NotTo(HaveOccurred())
            
            var getResp ws.Message
            err = wsjson.Read(ctx, conn, &getResp)
            Expect(err).NotTo(HaveOccurred())
            Expect(getResp.Error).To(BeNil())
        })
        
        It("should handle invalid method gracefully", func() {
            msg := ws.Message{
                ID:     uuid.New().String(),
                Type:   ws.MessageTypeRequest,
                Method: "invalid.method",
            }
            
            err := wsjson.Write(ctx, conn, msg)
            Expect(err).NotTo(HaveOccurred())
            
            var response ws.Message
            err = wsjson.Read(ctx, conn, &response)
            Expect(err).NotTo(HaveOccurred())
            Expect(response.Type).To(Equal(ws.MessageTypeError))
            Expect(response.ID).To(Equal(msg.ID))
            Expect(response.Error).NotTo(BeNil())
            Expect(response.Error.Code).To(Equal(ws.ErrCodeMethodNotFound))
        })
        
        It("should handle malformed messages", func() {
            // Send raw malformed JSON
            malformed := []byte(`{"invalid json}`)
            err := conn.Write(ctx, websocket.MessageText, malformed)
            Expect(err).NotTo(HaveOccurred())
            
            // Should receive error response
            var response ws.Message
            err = wsjson.Read(ctx, conn, &response)
            // Connection might close or return error
            if err == nil {
                Expect(response.Type).To(Equal(ws.MessageTypeError))
                Expect(response.Error).NotTo(BeNil())
            }
        })
    })
    
    Describe("Rate Limiting", func() {
        It("should enforce rate limits", func() {
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()
            
            opts := &websocket.DialOptions{
                HTTPHeader: http.Header{
                    "Authorization": []string{"Bearer " + apiKey},
                },
            }
            
            conn, _, err := websocket.Dial(ctx, wsURL, opts)
            Expect(err).NotTo(HaveOccurred())
            defer conn.Close(websocket.StatusNormalClosure, "")
            
            // Send many messages rapidly
            var rateLimited bool
            for i := 0; i < 200; i++ {
                msg := ws.Message{
                    ID:     fmt.Sprintf("msg-%d", i),
                    Type:   ws.MessageTypeRequest,
                    Method: "tool.list",
                }
                
                err := wsjson.Write(ctx, conn, msg)
                if err != nil {
                    break
                }
                
                var response ws.Message
                err = wsjson.Read(ctx, conn, &response)
                if err != nil {
                    break
                }
                
                if response.Error != nil && response.Error.Code == ws.ErrCodeRateLimited {
                    rateLimited = true
                    break
                }
            }
            
            Expect(rateLimited).To(BeTrue(), "Expected rate limiting to kick in")
        })
    })
    
    Describe("Binary Protocol", func() {
        It("should support binary message format", func() {
            Skip("Binary protocol testing requires special client setup")
        })
    })
    
    Describe("Monitoring Endpoints", func() {
        It("should provide WebSocket statistics", func() {
            resp, err := httpClient.Get(baseURL + "/api/v1/websocket/stats")
            Expect(err).NotTo(HaveOccurred())
            defer resp.Body.Close()
            
            // Stats endpoint might require auth
            if resp.StatusCode == http.StatusUnauthorized {
                Skip("Stats endpoint requires authentication")
            }
            
            Expect(resp.StatusCode).To(Equal(http.StatusOK))
            
            var stats map[string]interface{}
            err = json.NewDecoder(resp.Body).Decode(&stats)
            Expect(err).NotTo(HaveOccurred())
            Expect(stats).To(HaveKey("server"))
            Expect(stats).To(HaveKey("connections"))
        })
        
        It("should provide health check", func() {
            resp, err := httpClient.Get(baseURL + "/api/v1/websocket/health")
            Expect(err).NotTo(HaveOccurred())
            defer resp.Body.Close()
            
            // Health endpoint might require auth
            if resp.StatusCode == http.StatusUnauthorized {
                Skip("Health endpoint requires authentication")
            }
            
            Expect(resp.StatusCode).To(Equal(http.StatusOK))
            
            var health map[string]interface{}
            err = json.NewDecoder(resp.Body).Decode(&health)
            Expect(err).NotTo(HaveOccurred())
            Expect(health).To(HaveKey("status"))
            Expect(health["status"]).To(Equal("healthy"))
        })
    })
})