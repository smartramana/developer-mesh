package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test the updateContext handler in MCPAPI

func TestUpdateContextHandler(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	
	// Create mock context manager
	mockManager := new(MockContextManager)
	
	// Test data
	contextID := "test-context-id"
	existingContext := &mcp.Context{
		ID:       contextID,
		AgentID:  "test-agent",
		ModelID:  "gpt-4",
		Content:  []mcp.ContextItem{},
	}
	
	updatedContent := []mcp.ContextItem{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello, how are you?"},
		{Role: "assistant", Content: "I'm doing well, thank you! How can I help you today?"},
	}
	
	updatedContext := &mcp.Context{
		ID:       contextID,
		AgentID:  "test-agent",
		ModelID:  "gpt-4",
		Content:  updatedContent,
	}
	
	// Mock expectations
	mockManager.On("GetContext", mock.Anything, contextID).Return(existingContext, nil)
	mockManager.On("UpdateContext", mock.Anything, contextID, mock.Anything, mock.Anything).Return(updatedContext, nil)
	
	// Create router with the MCP API handlers
	router := gin.New()
	mcpAPI := NewMCPAPI(mockManager)
	
	// Register routes
	mcpGroup := router.Group("/api/v1/mcp")
	mcpGroup.PUT("/context/:id", mcpAPI.updateContext)
	
	// Create test request
	reqBody := struct {
		Content []mcp.ContextItem      `json:"content"`
		Options mcp.ContextUpdateOptions `json:"options,omitempty"`
	}{
		Content: updatedContent,
	}
	
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/mcp/context/"+contextID, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	// Perform the request
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response mcp.Context
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, contextID, response.ID)
	assert.Equal(t, len(updatedContent), len(response.Content))
	
	// Verify our expectations were met
	mockManager.AssertExpectations(t)
}

// Test error conditions for the updateContext handler

func TestUpdateContextHandlerErrors(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	
	// Create mock context manager
	mockManager := new(MockContextManager)
	
	// Test data
	contextID := "test-context-id"
	existingContext := &mcp.Context{
		ID:       contextID,
		AgentID:  "test-agent",
		ModelID:  "gpt-4",
		Content:  []mcp.ContextItem{},
	}
	
	// Create router with the MCP API handlers
	router := gin.New()
	mcpAPI := NewMCPAPI(mockManager)
	
	// Register routes
	mcpGroup := router.Group("/api/v1/mcp")
	mcpGroup.PUT("/context/:id", mcpAPI.updateContext)
	
	// Test cases
	tests := []struct {
		name               string
		contextID          string
		requestBody        interface{}
		setupMocks         func()
		expectedStatusCode int
		expectedMessage    string
	}{
		{
			name:      "Invalid request body",
			contextID: contextID,
			requestBody: map[string]string{
				"invalid": "this is not a valid request",
			},
			setupMocks: func() {
				// Mock GetContext to return error for invalid request body test
				mockManager.On("GetContext", mock.Anything, contextID).Return(existingContext, nil).Once()
				// Also mock UpdateContext since it will be called even with invalid request body
				mockManager.On("UpdateContext", mock.Anything, contextID, mock.Anything, mock.Anything).Return(existingContext, nil).Once()
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedMessage:    "error",
		},
		{
			name:      "Context not found",
			contextID: contextID,
			requestBody: struct {
				Content []mcp.ContextItem `json:"content"`
			}{
				Content: []mcp.ContextItem{
					{Role: "system", Content: "Test"},
				},
			},
			setupMocks: func() {
				var nilContext *mcp.Context = nil
				mockManager.On("GetContext", mock.Anything, contextID).Return(nilContext, assert.AnError).Once()
			},
			expectedStatusCode: http.StatusNotFound,
			expectedMessage:    "context not found",
		},
		{
			name:      "Error updating context",
			contextID: contextID,
			requestBody: struct {
				Content []mcp.ContextItem `json:"content"`
			}{
				Content: []mcp.ContextItem{
					{Role: "system", Content: "Test"},
				},
			},
			setupMocks: func() {
				mockManager.On("GetContext", mock.Anything, contextID).Return(existingContext, nil).Once()
				var nilContext *mcp.Context = nil
				mockManager.On("UpdateContext", mock.Anything, contextID, mock.Anything, mock.Anything).Return(nilContext, assert.AnError).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedMessage:    "error",
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks for this test case
			tc.setupMocks()
			
			// Create request
			jsonBody, _ := json.Marshal(tc.requestBody)
			req, _ := http.NewRequest(http.MethodPut, "/api/v1/mcp/context/"+tc.contextID, bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			
			// Perform the request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			// Check response
			assert.Equal(t, tc.expectedStatusCode, w.Code)
			assert.Contains(t, w.Body.String(), tc.expectedMessage)
		})
	}
	
	// Verify all expectations were met
	mockManager.AssertExpectations(t)
}
