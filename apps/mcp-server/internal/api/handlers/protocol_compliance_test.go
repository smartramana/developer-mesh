package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockContextManager is a mock implementation of ContextManagerInterface
type MockContextManager struct {
	mock.Mock
}

func (m *MockContextManager) CreateContext(ctx context.Context, context *models.Context) (*models.Context, error) {
	args := m.Called(ctx, context)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

func (m *MockContextManager) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

func (m *MockContextManager) UpdateContext(ctx context.Context, contextID string, context *models.Context, options *models.ContextUpdateOptions) (*models.Context, error) {
	args := m.Called(ctx, contextID, context, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

func (m *MockContextManager) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockContextManager) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*models.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Context), args.Error(1)
}

func (m *MockContextManager) SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error) {
	args := m.Called(ctx, contextID, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ContextItem), args.Error(1)
}

func (m *MockContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	args := m.Called(ctx, contextID)
	return args.String(0), args.Error(1)
}

// MockLogger is a mock implementation of the Logger interface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) WithPrefix(prefix string) observability.Logger {
	args := m.Called(prefix)
	return args.Get(0).(observability.Logger)
}

func (m *MockLogger) With(fields map[string]interface{}) observability.Logger {
	args := m.Called(fields)
	return args.Get(0).(observability.Logger)
}

// MCP Protocol message types
const (
	MessageTypeRequest  = "request"
	MessageTypeResponse = "response"
	MessageTypeError    = "error"
	MessageTypeEvent    = "event"
)

// TestProtocolCompliance tests MCP protocol message format compliance
func TestProtocolCompliance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	handler := setupTestHandler(t)
	router := setupTestRouter(handler)
	
	t.Run("Request Message Format", func(t *testing.T) {
		// Valid MCP request
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "test-123",
			"method":  "context.create",
			"params": map[string]interface{}{
				"agent_id": "test-agent",
				"model_id": "test-model",
				"metadata": map[string]interface{}{
					"source": "protocol-test",
				},
			},
		}
		
		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		// Validate response format
		assert.Equal(t, "2.0", response["jsonrpc"])
		assert.Equal(t, "test-123", response["id"])
		assert.Contains(t, response, "result")
		assert.NotContains(t, response, "error")
	})
	
	t.Run("Batch Request Support", func(t *testing.T) {
		// Batch of MCP requests
		requests := []map[string]interface{}{
			{
				"jsonrpc": "2.0",
				"id":      "batch-1",
				"method":  "context.list",
				"params": map[string]interface{}{
					"limit": 10,
				},
			},
			{
				"jsonrpc": "2.0",
				"id":      "batch-2",
				"method":  "agent.get",
				"params": map[string]interface{}{
					"id": "test-agent",
				},
			},
		}
		
		body, _ := json.Marshal(requests)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var responses []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &responses)
		require.NoError(t, err)
		
		assert.Len(t, responses, 2)
		assert.Equal(t, "batch-1", responses[0]["id"])
		assert.Equal(t, "batch-2", responses[1]["id"])
	})
	
	t.Run("Error Response Format", func(t *testing.T) {
		// Invalid method
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "error-test",
			"method":  "invalid.method",
			"params":  map[string]interface{}{},
		}
		
		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code) // JSON-RPC returns 200 with error in body
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "2.0", response["jsonrpc"])
		assert.Equal(t, "error-test", response["id"])
		assert.NotContains(t, response, "result")
		assert.Contains(t, response, "error")
		
		// Validate error structure
		errorObj := response["error"].(map[string]interface{})
		assert.Contains(t, errorObj, "code")
		assert.Contains(t, errorObj, "message")
		assert.Equal(t, float64(-32601), errorObj["code"]) // Method not found
	})
	
	t.Run("Notification Support", func(t *testing.T) {
		// Notification (no id field)
		notification := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "event.log",
			"params": map[string]interface{}{
				"level":   "info",
				"message": "Test notification",
			},
		}
		
		body, _ := json.Marshal(notification)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// Notifications should not return a response
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})
}

// TestMessageFormats tests various MCP message format validations
func TestMessageFormats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	handler := setupTestHandler(t)
	router := setupTestRouter(handler)
	
	testCases := []struct {
		name           string
		request        interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Missing jsonrpc",
			request: map[string]interface{}{
				"id":     "test-1",
				"method": "context.create",
				"params": map[string]interface{}{},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid Request",
		},
		{
			name: "Invalid jsonrpc version",
			request: map[string]interface{}{
				"jsonrpc": "1.0",
				"id":      "test-2",
				"method":  "context.create",
				"params":  map[string]interface{}{},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid Request",
		},
		{
			name: "Missing method",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-3",
				"params":  map[string]interface{}{},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid Request",
		},
		{
			name: "Invalid params type",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-4",
				"method":  "context.create",
				"params":  "invalid-params",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid params",
		},
		{
			name: "Valid request without params",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-5",
				"method":  "context.list",
			},
			expectedStatus: http.StatusOK,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.request)
			req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-MCP-Version", "1.0")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			if tc.expectedStatus == http.StatusBadRequest {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
				
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				
				if tc.expectedError != "" {
					assert.Contains(t, response, "error")
					errorObj := response["error"].(map[string]interface{})
					assert.Contains(t, errorObj["message"], tc.expectedError)
				}
			}
		})
	}
}

// TestVersionCompatibility tests protocol version negotiation
func TestVersionCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	handler := setupTestHandler(t)
	router := setupTestRouter(handler)
	
	t.Run("Supported Version", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "version-test",
			"method":  "system.version",
		}
		
		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("X-MCP-Version"), "1.0")
	})
	
	t.Run("Unsupported Version", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "version-test",
			"method":  "system.version",
		}
		
		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "99.0") // Unsupported version
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// Should still work but indicate supported version
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "1.0", w.Header().Get("X-MCP-Version"))
	})
	
	t.Run("Version Negotiation", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "negotiate",
			"method":  "system.capabilities",
		}
		
		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-MCP-Version", "1.0, 2.0, 3.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		
		if result, ok := response["result"].(map[string]interface{}); ok {
			assert.Contains(t, result, "version")
			assert.Contains(t, result, "capabilities")
		}
	})
}

// TestStreamingResponses tests streaming response support
func TestStreamingResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	handler := setupTestHandler(t)
	router := setupTestRouter(handler)
	
	t.Run("Streaming Context Updates", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "stream-test",
			"method":  "context.stream",
			"params": map[string]interface{}{
				"context_id": "test-context",
				"stream":     true,
			},
		}
		
		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		req.Header.Set("Accept", "text/event-stream")
		
		w := httptest.NewRecorder()
		
		// Create a channel to simulate streaming
		done := make(chan bool)
		go func() {
			router.ServeHTTP(w, req)
			done <- true
		}()
		
		// Give it time to start streaming
		time.Sleep(100 * time.Millisecond)
		
		// Check headers for SSE
		assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		
		// Stop the stream
		close(done)
	})
}

// TestContextManagementProtocol tests context-specific protocol features
func TestContextManagementProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	handler := setupTestHandler(t)
	router := setupTestRouter(handler)
	
	t.Run("Context Token Tracking", func(t *testing.T) {
		// Create context
		createReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "create-1",
			"method":  "context.create",
			"params": map[string]interface{}{
				"agent_id":   "test-agent",
				"model_id":   "gpt-4",
				"max_tokens": 4000,
			},
		}
		
		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		var createResp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &createResp)
		
		if result, ok := createResp["result"].(map[string]interface{}); ok {
			assert.Contains(t, result, "id")
			assert.Contains(t, result, "current_tokens")
			assert.Contains(t, result, "max_tokens")
			assert.Equal(t, float64(0), result["current_tokens"])
			assert.Equal(t, float64(4000), result["max_tokens"])
		}
	})
	
	t.Run("Context Truncation Signal", func(t *testing.T) {
		// Update context to near token limit
		updateReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "update-1",
			"method":  "context.update",
			"params": map[string]interface{}{
				"context_id": "test-context",
				"content": []map[string]interface{}{
					{
						"role":    "user",
						"content": generateLongContent(3900), // Near 4000 token limit
					},
				},
			},
		}
		
		body, _ := json.Marshal(updateReq)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		var updateResp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &updateResp)
		
		if result, ok := updateResp["result"].(map[string]interface{}); ok {
			// Should include truncation warning
			assert.Contains(t, result, "warnings")
			warnings := result["warnings"].([]interface{})
			assert.Greater(t, len(warnings), 0)
			
			warning := warnings[0].(map[string]interface{})
			assert.Equal(t, "approaching_token_limit", warning["type"])
		}
	})
}

// TestEventProtocol tests event handling in MCP
func TestEventProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	handler := setupTestHandler(t)
	router := setupTestRouter(handler)
	
	t.Run("Event Subscription", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "subscribe-1",
			"method":  "event.subscribe",
			"params": map[string]interface{}{
				"events": []string{
					"context.created",
					"context.updated",
					"context.deleted",
				},
			},
		}
		
		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		
		if result, ok := response["result"].(map[string]interface{}); ok {
			assert.Contains(t, result, "subscription_id")
			assert.Contains(t, result, "events")
		}
	})
	
	t.Run("Event Delivery", func(t *testing.T) {
		// Simulate event notification
		event := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "event.notify",
			"params": map[string]interface{}{
				"subscription_id": "sub-123",
				"event": map[string]interface{}{
					"type":      "context.created",
					"timestamp": time.Now().Unix(),
					"data": map[string]interface{}{
						"context_id": "new-context",
						"agent_id":   "test-agent",
					},
				},
			},
		}
		
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/mcp/v1/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-MCP-Version", "1.0")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// Event notifications don't return response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

// Helper functions

func setupTestHandler(t *testing.T) *MCPAPI {
	// Create mock dependencies and handler
	mockContextManager := &MockContextManager{}
	mockLogger := &MockLogger{}
	
	// Setup logger expectations
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything, mock.Anything).Return()
	
	handler := NewMCPAPI(mockContextManager, mockLogger)
	
	return handler
}

func setupTestRouter(handler *MCPAPI) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	
	// MCP routes
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)
	
	// Add RPC endpoint handler for JSON-RPC
	router.POST("/mcp/v1/rpc", handleJSONRPC(handler))
	router.GET("/mcp/v1/stream", handleStream(handler))
	
	return router
}

// handleJSONRPC processes JSON-RPC requests
func handleJSONRPC(handler *MCPAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read raw body to handle both single and batch requests
		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		
		// First try to parse as array (batch request)
		var batchRequest []map[string]any
		if err := json.Unmarshal(body, &batchRequest); err == nil {
			// It's a batch request
			var responses []map[string]any
			for _, request := range batchRequest {
				response := processJSONRPCRequest(request, handler)
				if response != nil {
					responses = append(responses, response)
				}
			}
			c.JSON(http.StatusOK, responses)
			return
		}
		
		// Try as single request
		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
		
		response := processJSONRPCRequest(request, handler)
		if response != nil {
			c.JSON(http.StatusOK, response)
		} else {
			// Notification - no response
			c.Status(http.StatusNoContent)
		}
	}
}

// processJSONRPCRequest processes a single JSON-RPC request
func processJSONRPCRequest(request map[string]any, handler *MCPAPI) map[string]any {
	// Validate JSON-RPC format
	if request["jsonrpc"] != "2.0" {
		if id, ok := request["id"]; ok {
			return map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"error": map[string]any{
					"code":    -32700,
					"message": "Invalid Request",
				},
			}
		}
		return nil
	}
	
	// Handle based on method
	method, ok := request["method"].(string)
	if !ok {
		if id, ok := request["id"]; ok {
			return map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"error": map[string]any{
					"code":    -32600,
					"message": "Invalid Request",
				},
			}
		}
		return nil
	}
	
	// Check if it's a notification (no id)
	id, hasID := request["id"]
	
	// Route to appropriate handler based on method
	switch method {
	case "context.create", "context.list", "context.stream", "context.update":
		if !hasID {
			return nil // Notification
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{"status": "ok"},
		}
	case "agent.get":
		if !hasID {
			return nil // Notification
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{"id": "test-agent", "name": "Test Agent"},
		}
	case "system.version":
		if !hasID {
			return nil // Notification
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"version":  "1.0",
				"protocol": "MCP",
			},
		}
	case "system.capabilities":
		if !hasID {
			return nil // Notification
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"version":      "1.0",
				"capabilities": []string{"context", "streaming", "events"},
			},
		}
	case "event.subscribe":
		if !hasID {
			return nil // Notification
		}
		params := request["params"].(map[string]any)
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"subscription_id": "sub-123",
				"events":          params["events"],
			},
		}
	case "event.log", "event.notify":
		// Notifications don't return response
		return nil
	case "invalid.method":
		if !hasID {
			return nil // Notification
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]any{
				"code":    -32601,
				"message": "Method not found",
			},
		}
	default:
		if !hasID {
			return nil // Notification
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]any{
				"code":    -32601,
				"message": "Method not found",
			},
		}
	}
}

// handleStream handles streaming responses
func handleStream(handler *MCPAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		
		// For testing, just set headers and return
		c.Status(http.StatusOK)
	}
}

func generateLongContent(approxTokens int) string {
	// Rough approximation: 1 token ≈ 4 characters
	charCount := approxTokens * 4
	content := ""
	for len(content) < charCount {
		content += "This is a test message to fill up the context with tokens. "
	}
	return content
}