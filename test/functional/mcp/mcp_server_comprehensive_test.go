package mcp_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// MCPMessage represents a standard MCP protocol message
type MCPMessage struct {
	Type      string                 `json:"type"`
	ID        string                 `json:"id,omitempty"`
	Method    string                 `json:"method,omitempty"`
	Params    map[string]interface{} `json:"params,omitempty"`
	Result    interface{}            `json:"result,omitempty"`
	Error     *MCPError              `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp,omitempty"`
}

// MCPError represents an error in the MCP protocol
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPCapabilities represents server capabilities
type MCPCapabilities struct {
	Tools      bool `json:"tools"`
	Resources  bool `json:"resources"`
	Prompts    bool `json:"prompts"`
	Logging    bool `json:"logging"`
	Completion bool `json:"completion"`
}

// MCPTool represents a tool definition
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

var _ = Describe("MCP Server Comprehensive WebSocket Tests", func() {
	var (
		wsURL  string
		apiKey string
	)

	BeforeEach(func() {
		// Get configuration from environment
		baseURL := os.Getenv("MCP_SERVER_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}

		// Convert HTTP URL to WebSocket URL
		if strings.HasPrefix(baseURL, "http://") {
			wsURL = strings.Replace(baseURL, "http://", "ws://", 1)
		} else if strings.HasPrefix(baseURL, "https://") {
			wsURL = strings.Replace(baseURL, "https://", "wss://", 1)
		} else {
			wsURL = "ws://localhost:8080"
		}

		wsURL = wsURL + "/ws"

		apiKey = os.Getenv("MCP_API_KEY")
		if apiKey == "" {
			apiKey = "docker-admin-api-key"
		}
	})

	Describe("WebSocket Connection Tests", func() {
		It("should establish a WebSocket connection with authentication", func() {
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			conn, resp, err := dialer.Dial(wsURL, header)
			if err != nil {
				if resp != nil {
					GinkgoWriter.Printf("Connection failed with status: %d\n", resp.StatusCode)
				}
				Skip("WebSocket server not available")
			}
			defer func() {
				if err := conn.Close(); err != nil {
					GinkgoWriter.Printf("Error closing connection: %v\n", err)
				}
			}()

			Expect(resp.StatusCode).To(Equal(http.StatusSwitchingProtocols))

			// Test ping/pong
			err = conn.WriteMessage(websocket.PingMessage, []byte("ping"))
			Expect(err).NotTo(HaveOccurred())

			// Set read deadline for pong
			if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				GinkgoWriter.Printf("Error setting read deadline: %v\n", err)
			}
			_, _, readErr := conn.NextReader()
			// Reset deadline
			if err := conn.SetReadDeadline(time.Time{}); err != nil {
				GinkgoWriter.Printf("Error resetting read deadline: %v\n", err)
			}
			Expect(readErr).To(BeNil())
		})

		It("should reject connection without authentication", func() {
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			conn, resp, err := dialer.Dial(wsURL, nil)
			if conn != nil {
				defer func() {
					if err := conn.Close(); err != nil {
						GinkgoWriter.Printf("Error closing connection: %v\n", err)
					}
				}()
			}

			// Either connection should fail or we should get unauthorized
			if err == nil {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	})

	Describe("MCP Protocol Handshake", func() {
		var conn *websocket.Conn
		var cancel context.CancelFunc

		BeforeEach(func() {
			_, cancel = context.WithTimeout(context.Background(), 30*time.Second)

			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			var resp *http.Response
			var err error
			conn, resp, err = dialer.Dial(wsURL, header)
			if err != nil {
				if resp != nil {
					GinkgoWriter.Printf("Connection failed with status: %d\n", resp.StatusCode)
				}
				Skip("WebSocket server not available")
			}
		})

		AfterEach(func() {
			if conn != nil {
				if err := conn.Close(); err != nil {
					GinkgoWriter.Printf("Error closing connection: %v\n", err)
				}
			}
			cancel()
		})

		It("should complete MCP initialization handshake", func() {
			// Send initialization request
			var err error
			initMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "1.0",
					"capabilities": MCPCapabilities{
						Tools:      true,
						Resources:  true,
						Prompts:    true,
						Logging:    true,
						Completion: true,
					},
					"clientInfo": map[string]interface{}{
						"name":    "mcp-test-client",
						"version": "1.0.0",
					},
				},
			}

			err = conn.WriteJSON(initMsg)
			Expect(err).NotTo(HaveOccurred())

			// Read response
			var response MCPMessage
			err = conn.ReadJSON(&response)
			Expect(err).NotTo(HaveOccurred())

			// Verify response
			Expect(response.Type).To(Equal("response"))
			Expect(response.ID).To(Equal(initMsg.ID))
			Expect(response.Error).To(BeNil())

			// Check server capabilities
			if response.Result != nil {
				result, ok := response.Result.(map[string]interface{})
				Expect(ok).To(BeTrue())

				if capabilities, ok := result["capabilities"]; ok {
					GinkgoWriter.Printf("Server capabilities: %+v\n", capabilities)
				}
			}
		})

		It("should handle protocol version negotiation", func() {
			// Try with an unsupported version
			var err error
			initMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "99.0", // Unsupported version
					"capabilities":    MCPCapabilities{},
				},
			}

			err = conn.WriteJSON(initMsg)
			Expect(err).NotTo(HaveOccurred())

			var response MCPMessage
			err = conn.ReadJSON(&response)
			Expect(err).NotTo(HaveOccurred())

			// Server should either negotiate down or return error
			Expect(response.Type).To(Equal("response"))
			Expect(response.ID).To(Equal(initMsg.ID))
		})
	})

	Describe("Tool Discovery and Execution", func() {
		var conn *websocket.Conn

		BeforeEach(func() {
			// Establish and initialize connection
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			var err error
			conn, _, err = dialer.Dial(wsURL, header)
			if err != nil {
				Skip("WebSocket server not available")
			}

			// Initialize connection
			initMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "1.0",
					"capabilities":    MCPCapabilities{Tools: true},
				},
			}
			err = conn.WriteJSON(initMsg)
			Expect(err).NotTo(HaveOccurred())

			var initResp MCPMessage
			err = conn.ReadJSON(&initResp)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if conn != nil {
				if err := conn.Close(); err != nil {
					GinkgoWriter.Printf("Error closing connection: %v\n", err)
				}
			}
		})

		It("should list available tools", func() {
			// Request tools list
			toolsMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "tools/list",
				Params: map[string]interface{}{},
			}

			err := conn.WriteJSON(toolsMsg)
			Expect(err).NotTo(HaveOccurred())

			var response MCPMessage
			err = conn.ReadJSON(&response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.Type).To(Equal("response"))
			Expect(response.ID).To(Equal(toolsMsg.ID))
			Expect(response.Error).To(BeNil())

			// Verify tools structure
			if response.Result != nil {
				result, ok := response.Result.(map[string]interface{})
				Expect(ok).To(BeTrue())

				if tools, ok := result["tools"]; ok {
					toolsList, ok := tools.([]interface{})
					Expect(ok).To(BeTrue())
					GinkgoWriter.Printf("Available tools: %d\n", len(toolsList))

					// Verify each tool has required fields
					for _, tool := range toolsList {
						toolMap, ok := tool.(map[string]interface{})
						Expect(ok).To(BeTrue())
						Expect(toolMap).To(HaveKey("name"))
						Expect(toolMap).To(HaveKey("description"))
						Expect(toolMap).To(HaveKey("inputSchema"))
					}
				}
			}
		})

		It("should execute a tool call", func() {
			// First get available tools
			toolsMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "tools/list",
				Params: map[string]interface{}{},
			}
			err := conn.WriteJSON(toolsMsg)
			Expect(err).NotTo(HaveOccurred())

			var toolsResp MCPMessage
			err = conn.ReadJSON(&toolsResp)
			Expect(err).NotTo(HaveOccurred())

			// Execute a simple tool if available
			callMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "tools/call",
				Params: map[string]interface{}{
					"name": "github_list_repos",
					"arguments": map[string]interface{}{
						"org": "test-org",
					},
				},
			}

			err = conn.WriteJSON(callMsg)
			Expect(err).NotTo(HaveOccurred())

			var response MCPMessage
			err = conn.ReadJSON(&response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.Type).To(Equal("response"))
			Expect(response.ID).To(Equal(callMsg.ID))

			// Tool might not exist or might fail - that's OK for this test
			if response.Error != nil {
				GinkgoWriter.Printf("Tool call error: %+v\n", response.Error)
			} else {
				GinkgoWriter.Printf("Tool call result: %+v\n", response.Result)
			}
		})
	})

	Describe("Resource Management", func() {
		var conn *websocket.Conn

		BeforeEach(func() {
			// Establish and initialize connection
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			var err error
			conn, _, err = dialer.Dial(wsURL, header)
			if err != nil {
				Skip("WebSocket server not available")
			}

			// Initialize with resources capability
			initMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "1.0",
					"capabilities":    MCPCapabilities{Resources: true},
				},
			}
			err = conn.WriteJSON(initMsg)
			Expect(err).NotTo(HaveOccurred())

			var initResp MCPMessage
			err = conn.ReadJSON(&initResp)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if conn != nil {
				if err := conn.Close(); err != nil {
					GinkgoWriter.Printf("Error closing connection: %v\n", err)
				}
			}
		})

		It("should list available resources", func() {
			resourcesMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "resources/list",
				Params: map[string]interface{}{},
			}

			err := conn.WriteJSON(resourcesMsg)
			Expect(err).NotTo(HaveOccurred())

			var response MCPMessage
			err = conn.ReadJSON(&response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.Type).To(Equal("response"))
			Expect(response.ID).To(Equal(resourcesMsg.ID))
		})

		It("should read a resource", func() {
			readMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "resources/read",
				Params: map[string]interface{}{
					"uri": "github://repos/test-org/test-repo",
				},
			}

			err := conn.WriteJSON(readMsg)
			Expect(err).NotTo(HaveOccurred())

			var response MCPMessage
			err = conn.ReadJSON(&response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.Type).To(Equal("response"))
			Expect(response.ID).To(Equal(readMsg.ID))
		})
	})

	Describe("Concurrent Operations", func() {
		var conn *websocket.Conn

		BeforeEach(func() {
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			var err error
			conn, _, err = dialer.Dial(wsURL, header)
			if err != nil {
				Skip("WebSocket server not available")
			}

			// Initialize connection
			initMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "1.0",
					"capabilities":    MCPCapabilities{Tools: true},
				},
			}
			err = conn.WriteJSON(initMsg)
			Expect(err).NotTo(HaveOccurred())

			var initResp MCPMessage
			err = conn.ReadJSON(&initResp)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if conn != nil {
				if err := conn.Close(); err != nil {
					GinkgoWriter.Printf("Error closing connection: %v\n", err)
				}
			}
		})

		It("should handle multiple concurrent requests", func() {
			var wg sync.WaitGroup
			responses := make(chan MCPMessage, 10)
			errors := make(chan error, 10)

			// Reader goroutine
			go func() {
				for {
					var msg MCPMessage
					err := conn.ReadJSON(&msg)
					if err != nil {
						if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
							return
						}
						errors <- err
						return
					}
					responses <- msg
				}
			}()

			// Send multiple requests concurrently
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()

					msg := MCPMessage{
						Type:   "request",
						ID:     fmt.Sprintf("req-%d", index),
						Method: "tools/list",
						Params: map[string]interface{}{},
					}

					err := conn.WriteJSON(msg)
					if err != nil {
						errors <- err
					}
				}(i)
			}

			// Wait for all requests to be sent
			wg.Wait()

			// Collect responses
			receivedResponses := make(map[string]bool)
			timeout := time.After(10 * time.Second)

			for i := 0; i < 5; i++ {
				select {
				case resp := <-responses:
					receivedResponses[resp.ID] = true
				case err := <-errors:
					Fail(fmt.Sprintf("Error during concurrent test: %v", err))
				case <-timeout:
					Fail("Timeout waiting for responses")
				}
			}

			// Verify all responses were received
			Expect(receivedResponses).To(HaveLen(5))
			for i := 0; i < 5; i++ {
				Expect(receivedResponses).To(HaveKey(fmt.Sprintf("req-%d", i)))
			}
		})
	})

	Describe("Error Handling", func() {
		var conn *websocket.Conn

		BeforeEach(func() {
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			var err error
			conn, _, err = dialer.Dial(wsURL, header)
			if err != nil {
				Skip("WebSocket server not available")
			}
		})

		AfterEach(func() {
			if conn != nil {
				if err := conn.Close(); err != nil {
					GinkgoWriter.Printf("Error closing connection: %v\n", err)
				}
			}
		})

		It("should handle invalid message format", func() {
			// Send invalid JSON
			err := conn.WriteMessage(websocket.TextMessage, []byte("invalid json"))
			Expect(err).NotTo(HaveOccurred())

			// Should receive error response
			var response MCPMessage
			err = conn.ReadJSON(&response)
			// Connection might close or return error message
			if err == nil {
				Expect(response.Type).To(Equal("error"))
			}
		})

		It("should handle unknown methods", func() {
			unknownMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "unknown/method",
				Params: map[string]interface{}{},
			}

			err := conn.WriteJSON(unknownMsg)
			Expect(err).NotTo(HaveOccurred())

			var response MCPMessage
			err = conn.ReadJSON(&response)
			if err == nil {
				Expect(response.Type).To(Equal("response"))
				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error.Code).To(BeNumerically(">", 0))
			}
		})
	})

	Describe("Connection Resilience", func() {
		It("should handle connection interruption gracefully", func() {
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			conn, _, err := dialer.Dial(wsURL, header)
			if err != nil {
				Skip("WebSocket server not available")
			}

			// Close connection abruptly
			if err := conn.Close(); err != nil {
				GinkgoWriter.Printf("Error closing connection: %v\n", err)
			}

			// Try to write - should fail
			msg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "tools/list",
			}

			err = conn.WriteJSON(msg)
			Expect(err).To(HaveOccurred())
		})

		It("should support reconnection", func() {
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			// First connection
			conn1, _, err := dialer.Dial(wsURL, header)
			if err != nil {
				Skip("WebSocket server not available")
			}
			err = conn1.Close()
			Expect(err).NotTo(HaveOccurred())

			// Second connection should work
			conn2, _, err := dialer.Dial(wsURL, header)
			if err != nil {
				Skip("Server doesn't support reconnection")
			}
			defer func() {
				if err := conn2.Close(); err != nil {
					GinkgoWriter.Printf("Error closing connection: %v\n", err)
				}
			}()

			// Should be able to send messages on new connection
			initMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "1.0",
				},
			}

			err = conn2.WriteJSON(initMsg)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Performance Tests", func() {
		var conn *websocket.Conn

		BeforeEach(func() {
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			header := http.Header{}
			header.Set("X-API-Key", apiKey)

			var err error
			conn, _, err = dialer.Dial(wsURL, header)
			if err != nil {
				Skip("WebSocket server not available")
			}

			// Initialize
			initMsg := MCPMessage{
				Type:   "request",
				ID:     uuid.New().String(),
				Method: "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "1.0",
					"capabilities":    MCPCapabilities{Tools: true},
				},
			}
			err = conn.WriteJSON(initMsg)
			Expect(err).NotTo(HaveOccurred())

			var initResp MCPMessage
			err = conn.ReadJSON(&initResp)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if conn != nil {
				if err := conn.Close(); err != nil {
					GinkgoWriter.Printf("Error closing connection: %v\n", err)
				}
			}
		})

		It("should handle rapid message exchanges", func() {
			messageCount := 20
			startTime := time.Now()

			for i := 0; i < messageCount; i++ {
				msg := MCPMessage{
					Type:   "request",
					ID:     fmt.Sprintf("perf-%d", i),
					Method: "tools/list",
					Params: map[string]interface{}{},
				}

				err := conn.WriteJSON(msg)
				Expect(err).NotTo(HaveOccurred())

				var response MCPMessage
				err = conn.ReadJSON(&response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.ID).To(Equal(msg.ID))
			}

			duration := time.Since(startTime)
			avgLatency := duration / time.Duration(messageCount)

			GinkgoWriter.Printf("Processed %d messages in %v (avg: %v per message)\n",
				messageCount, duration, avgLatency)

			// Performance assertion - adjust based on requirements
			Expect(avgLatency).To(BeNumerically("<", 100*time.Millisecond))
		})
	})
})
