package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockContextManager is a mock implementation of ContextManagerInterface
type MockContextManager struct {
	mock.Mock
}

func (m *MockContextManager) CreateContext(ctx context.Context, context *mcp.Context) (*mcp.Context, error) {
	args := m.Called(ctx, context)
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) UpdateContext(ctx context.Context, contextID string, context *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	args := m.Called(ctx, contextID, context, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockContextManager) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

func (m *MockContextManager) SearchInContext(ctx context.Context, contextID, query string) ([]mcp.ContextItem, error) {
	args := m.Called(ctx, contextID, query)
	return args.Get(0).([]mcp.ContextItem), args.Error(1)
}

func (m *MockContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	args := m.Called(ctx, contextID)
	return args.String(0), args.Error(1)
}

func TestUpdateContext(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Define test cases
	testCases := []struct {
		name               string
		contextID          string
		requestBody        string
		setupMocks         func(mockContextManager *MockContextManager)
		expectedStatusCode int
		expectedResponse   string
	}{
		{
			name:      "successful update with new content",
			contextID: "test-id-1",
			requestBody: `{"content": [
				{"role": "user", "content": "Hello, how are you?", "tokens": 5},
				{"role": "assistant", "content": "I'm doing well, thank you!", "tokens": 6}
			]}`,
			setupMocks: func(mockContextManager *MockContextManager) {
				existingContext := &mcp.Context{
					ID:        "test-id-1",
					AgentID:   "test-agent",
					ModelID:   "gpt-4",
					Content:   []mcp.ContextItem{},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				
				newContent := []mcp.ContextItem{
					{
						Role:    "user",
						Content: "Hello, how are you?",
						Tokens:  5,
					},
					{
						Role:    "assistant",
						Content: "I'm doing well, thank you!",
						Tokens:  6,
					},
				}
				
				updatedContext := &mcp.Context{
					ID:        "test-id-1",
					AgentID:   "test-agent",
					ModelID:   "gpt-4",
					Content:   newContent,
					CreatedAt: existingContext.CreatedAt,
					UpdatedAt: existingContext.UpdatedAt,
				}
				
				mockContextManager.On("GetContext", mock.Anything, "test-id-1").Return(existingContext, nil)
				mockContextManager.On("UpdateContext", mock.Anything, "test-id-1", mock.Anything, mock.Anything).Return(updatedContext, nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   "",
		},
		{
			name:      "context not found",
			contextID: "nonexistent-id",
			requestBody: `{"content": [
				{"role": "user", "content": "Hello", "tokens": 1}
			]}`,
			setupMocks: func(mockContextManager *MockContextManager) {
				mockContextManager.On("GetContext", mock.Anything, "nonexistent-id").Return(nil, assert.AnError)
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "Context not found",
		},
		{
			name:      "invalid request body",
			contextID: "test-id-1",
			requestBody: `{"invalid json`,
			setupMocks: func(mockContextManager *MockContextManager) {
				// No mocks needed as request parsing will fail
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "Invalid request",
		},
		{
			name:      "update failure",
			contextID: "error-id",
			requestBody: `{"content": [
				{"role": "user", "content": "Content that causes error", "tokens": 5}
			]}`,
			setupMocks: func(mockContextManager *MockContextManager) {
				existingContext := &mcp.Context{
					ID:        "error-id",
					AgentID:   "test-agent",
					ModelID:   "gpt-4",
					Content:   []mcp.ContextItem{},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				
				mockContextManager.On("GetContext", mock.Anything, "error-id").Return(existingContext, nil)
				mockContextManager.On("UpdateContext", mock.Anything, "error-id", mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "Failed to update context",
		},
	}
	
	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new mock context manager for each test
			mockContextManager := new(MockContextManager)
			
			// Setup mocks for this test
			tc.setupMocks(mockContextManager)
			
			// Create API and router
			api := NewMCPAPI(mockContextManager)
			router := gin.Default()
			
			// Register routes
			group := router.Group("/api/v1/mcp")
			{
				group.PUT("/context/:id", api.updateContext)
			}
			
			// Create request
			req, _ := http.NewRequest("PUT", "/api/v1/mcp/context/"+tc.contextID, strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			
			// Perform request
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			
			// Assert status code
			assert.Equal(t, tc.expectedStatusCode, resp.Code)
			
			// Assert response content if expected
			if tc.expectedResponse != "" {
				assert.Contains(t, resp.Body.String(), tc.expectedResponse)
			}
			
			// Verify that all expected mock calls were made
			mockContextManager.AssertExpectations(t)
		})
	}
}
