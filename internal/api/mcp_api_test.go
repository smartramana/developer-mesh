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
	// Setup
	gin.SetMode(gin.TestMode)
	mockContextManager := new(MockContextManager)
	api := NewMCPAPI(mockContextManager)
	router := gin.Default()
	
	// Register routes
	group := router.Group("/api/v1/mcp")
	{
		group.PUT("/context/:id", api.updateContext)
	}
	
	// Test data
	contextID := "test-id-1"
	existingContext := &mcp.Context{
		ID:        contextID,
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
		ID:        contextID,
		AgentID:   "test-agent",
		ModelID:   "gpt-4",
		Content:   newContent,
		CreatedAt: existingContext.CreatedAt,
		UpdatedAt: existingContext.UpdatedAt,
	}
	
	// Set expectations
	mockContextManager.On("GetContext", mock.Anything, contextID).Return(existingContext, nil)
	mockContextManager.On("UpdateContext", mock.Anything, contextID, mock.Anything, mock.Anything).Return(updatedContext, nil)
	
	// Create request
	requestBody := `{"content": [
		{"role": "user", "content": "Hello, how are you?", "tokens": 5},
		{"role": "assistant", "content": "I'm doing well, thank you!", "tokens": 6}
	]}`
	req, _ := http.NewRequest("PUT", "/api/v1/mcp/context/"+contextID, strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	
	// Perform request
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	// Assert
	assert.Equal(t, http.StatusOK, resp.Code)
	mockContextManager.AssertExpectations(t)
}
