package websocket_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	
	"functional-tests/shared"
	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

var (
	config *shared.ServiceConfig
	wsURL  string
)

func TestWebSocket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WebSocket Suite")
}

var _ = BeforeSuite(func() {
	config = shared.GetTestConfig()
	wsURL = config.WebSocketURL
	
	// Verify WebSocket endpoint health (following CLAUDE.md patterns)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	healthURL := strings.Replace(wsURL, "ws://", "http://", 1)
	healthURL = strings.Replace(healthURL, "/ws", "/health", 1)
	
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	Expect(err).NotTo(HaveOccurred())
	
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(200))
})

var _ = Describe("WebSocket Connection", func() {
	var (
		conn *websocket.Conn
		ctx  context.Context
		cancel context.CancelFunc
	)
	
	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	})
	
	AfterEach(func() {
		if conn != nil {
			if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
				GinkgoWriter.Printf("Error closing connection in AfterEach: %v\n", err)
			}
		}
		cancel()
	})
	
	Context("Authentication", func() {
		It("should connect with valid API key", func() {
			apiKey := shared.GetTestAPIKey("test-tenant-1")
			headers := http.Header{
				"Authorization": []string{fmt.Sprintf("Bearer %s", apiKey)},
			}
			
			var err error
			conn, _, err = websocket.Dial(ctx, wsURL, &websocket.DialOptions{
				HTTPHeader: headers,
			})
			Expect(err).NotTo(HaveOccurred())
			
			// Send test message
			msg := ws.Message{
				ID:     "test-1",
				Method: "ping",
			}
			
			err = wsjson.Write(ctx, conn, msg)
			Expect(err).NotTo(HaveOccurred())
			
			// Read response
			var response ws.Message
			err = wsjson.Read(ctx, conn, &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.ID).To(Equal("test-1"))
		})
		
		It("should reject connection without authentication", func() {
			_, _, err := websocket.Dial(ctx, wsURL, nil)
			Expect(err).To(HaveOccurred())
		})
		
		It("should reject connection with invalid API key", func() {
			headers := http.Header{
				"Authorization": []string{"Bearer invalid-key"},
			}
			
			_, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
				HTTPHeader: headers,
			})
			Expect(err).To(HaveOccurred())
		})
	})
	
	Context("Tenant Isolation", func() {
		It("should isolate connections by tenant", func() {
			// Connect as tenant-1
			apiKey1 := shared.GetTestAPIKey("test-tenant-1")
			headers1 := http.Header{
				"Authorization": []string{fmt.Sprintf("Bearer %s", apiKey1)},
			}
			
			conn1, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
				HTTPHeader: headers1,
			})
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if err := conn1.Close(websocket.StatusNormalClosure, ""); err != nil {
					GinkgoWriter.Printf("Error closing connection 1: %v\n", err)
				}
			}()
			
			// Connect as tenant-2
			apiKey2 := shared.GetTestAPIKey("test-tenant-2")
			headers2 := http.Header{
				"Authorization": []string{fmt.Sprintf("Bearer %s", apiKey2)},
			}
			
			conn2, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
				HTTPHeader: headers2,
			})
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if err := conn2.Close(websocket.StatusNormalClosure, ""); err != nil {
					GinkgoWriter.Printf("Error closing connection 2: %v\n", err)
				}
			}()
			
			// Send message from tenant-1
			msg1 := ws.Message{
				ID:     "tenant-1-msg",
				Method: "broadcast",
				Params: map[string]interface{}{
					"message": "Hello from tenant 1",
				},
			}
			
			err = wsjson.Write(ctx, conn1, msg1)
			Expect(err).NotTo(HaveOccurred())
			
			// Tenant-2 should not receive tenant-1's broadcast
			// Use context with timeout instead of SetReadDeadline
			readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
			defer readCancel()
			
			var msg2 ws.Message
			err = wsjson.Read(readCtx, conn2, &msg2)
			Expect(err).To(HaveOccurred()) // Should timeout
		})
	})
})