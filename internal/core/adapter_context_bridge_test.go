package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockContextManager mocks the ContextManager interface
type MockContextManager struct {
	mock.Mock
}

func (m *MockContextManager) CreateContext(ctx context.Context, request *mcp.Context) (*mcp.Context, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) UpdateContext(ctx context.Context, contextID string, updateRequest *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	args := m.Called(ctx, contextID, mock.Anything, mock.Anything)
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockContextManager) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

func (m *MockContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	args := m.Called(ctx, contextID)
	return args.String(0), args.Error(1)
}

func (m *MockContextManager) SearchInContext(ctx context.Context, contextID string, query string) ([]mcp.ContextItem, error) {
	args := m.Called(ctx, contextID, query)
	return args.Get(0).([]mcp.ContextItem), args.Error(1)
}

// MockAdapter mocks the core.Adapter interface
type MockAdapter struct {
	mock.Mock
}

func (m *MockAdapter) Type() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAdapter) Health() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, contextID, action, params)
	return args.Get(0), args.Error(1)
}

func (m *MockAdapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	args := m.Called(ctx, eventType, payload)
	return args.Error(0)
}

func (m *MockAdapter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAdapter) Version() string {
	args := m.Called()
	return args.String(0)
}

// Additional methods needed for tests but not in core.Adapter interface
func (m *MockAdapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	args := m.Called(ctx, query)
	return args.Get(0), args.Error(1)
}

func TestExecuteToolAction(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)
	
	// Create the bridge
	adapters := map[string]core.Adapter{
		"test-tool": mockAdapter,
	}
	
	bridge := NewAdapterContextBridge(mockContextManager, adapters)
	
	// Test data
	ctx := context.Background()
	contextID := "test-context"
	tool := "test-tool"
	action := "test-action"
	params := map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}
	
	// Test context
	testContext := &mcp.Context{
		ID:        contextID,
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []mcp.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Expected result
	expectedResult := map[string]interface{}{
		"status": "success",
		"data":   "test-data",
	}
	
	// Set up expectations for context manager
	mockContextManager.On("GetContext", ctx, contextID).Return(testContext, nil)
	mockContextManager.On("UpdateContext", ctx, contextID, mock.Anything, mock.Anything).Return(testContext, nil).Times(2)
	
	// Set up expectations for adapter
	mockAdapter.On("Type").Return("test-tool")
	mockAdapter.On("ExecuteAction", ctx, contextID, action, params).Return(expectedResult, nil)
	
	// Execute the action
	result, err := bridge.ExecuteToolAction(ctx, contextID, tool, action, params)
	
	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	
	// Verify that all expectations were met
	mockContextManager.AssertExpectations(t)
	mockAdapter.AssertExpectations(t)
}

func TestGetToolData(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)
	
	// Create the bridge
	adapters := map[string]core.Adapter{
		"test-tool": mockAdapter,
	}
	
	bridge := NewAdapterContextBridge(mockContextManager, adapters)
	
	// Test data
	ctx := context.Background()
	contextID := "test-context"
	tool := "test-tool"
	query := map[string]interface{}{
		"filter": "test-filter",
		"limit":  10,
	}
	
	// Test context
	testContext := &mcp.Context{
		ID:        contextID,
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []mcp.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Expected result
	expectedResult := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": 1, "name": "Item 1"},
			map[string]interface{}{"id": 2, "name": "Item 2"},
		},
	}
	
	// Set up expectations for context manager
	mockContextManager.On("GetContext", ctx, contextID).Return(testContext, nil)
	mockContextManager.On("UpdateContext", ctx, contextID, mock.Anything, mock.Anything).Return(testContext, nil).Times(2)
	
	// Set up expectations for adapter
	mockAdapter.On("Type").Return("test-tool")
	mockAdapter.On("GetData", ctx, query).Return(expectedResult, nil)
	
	// Get the data
	result, err := bridge.GetToolData(ctx, contextID, tool, query)
	
	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	
	// Verify that all expectations were met
	mockContextManager.AssertExpectations(t)
	mockAdapter.AssertExpectations(t)
}

func TestHandleToolWebhook(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)
	
	// Create the bridge
	adapters := map[string]core.Adapter{
		"test-tool": mockAdapter,
	}
	
	bridge := NewAdapterContextBridge(mockContextManager, adapters)
	
	// Test data
	ctx := context.Background()
	tool := "test-tool"
	eventType := "test-event"
	
	// Create a webhook payload with context IDs
	payload := map[string]interface{}{
		"event": "some-event",
		"data": map[string]interface{}{
			"key": "value",
		},
		"metadata": map[string]interface{}{
			"context_ids": []interface{}{"context-1", "context-2"},
		},
	}
	
	jsonPayload, _ := json.Marshal(payload)
	
	// Test contexts
	testContext1 := &mcp.Context{
		ID:        "context-1",
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []mcp.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	testContext2 := &mcp.Context{
		ID:        "context-2",
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []mcp.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Set up expectations for context manager
	mockContextManager.On("GetContext", ctx, "context-1").Return(testContext1, nil)
	mockContextManager.On("GetContext", ctx, "context-2").Return(testContext2, nil)
	mockContextManager.On("UpdateContext", ctx, "context-1", mock.Anything, nil).Return(testContext1, nil)
	mockContextManager.On("UpdateContext", ctx, "context-2", mock.Anything, nil).Return(testContext2, nil)
	
	// Set up expectations for adapter
	mockAdapter.On("Type").Return("test-tool")
	mockAdapter.On("HandleWebhook", ctx, eventType, jsonPayload).Return(nil)
	
	// Handle the webhook
	err := bridge.HandleToolWebhook(ctx, tool, eventType, jsonPayload)
	
	// Assertions
	assert.NoError(t, err)
	
	// Verify that all expectations were met
	mockContextManager.AssertExpectations(t)
	mockAdapter.AssertExpectations(t)
}

func TestExecuteToolAction_Error(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)
	
	// Create the bridge
	adapters := map[string]core.Adapter{
		"test-tool": mockAdapter,
	}
	
	bridge := NewAdapterContextBridge(mockContextManager, adapters)
	
	// Test data
	ctx := context.Background()
	contextID := "test-context"
	tool := "test-tool"
	action := "test-action"
	params := map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}
	
	// Test context
	testContext := &mcp.Context{
		ID:        contextID,
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []mcp.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Set up expectations for context manager
	mockContextManager.On("GetContext", ctx, contextID).Return(testContext, nil)
	mockContextManager.On("UpdateContext", ctx, contextID, mock.Anything, mock.Anything).Return(testContext, nil).Times(2)
	
	// Set up expectations for adapter to return an error
	mockAdapter.On("Type").Return("test-tool")
	mockAdapter.On("ExecuteAction", ctx, contextID, action, params).
		Return(nil, assert.AnError)
	
	// Execute the action
	result, err := bridge.ExecuteToolAction(ctx, contextID, tool, action, params)
	
	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)
	
	// Verify that all expectations were met
	mockContextManager.AssertExpectations(t)
	mockAdapter.AssertExpectations(t)
}

func TestGetToolData_Error(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)
	
	// Create the bridge
	adapters := map[string]core.Adapter{
		"test-tool": mockAdapter,
	}
	
	bridge := NewAdapterContextBridge(mockContextManager, adapters)
	
	// Test data
	ctx := context.Background()
	contextID := "test-context"
	tool := "test-tool"
	query := map[string]interface{}{
		"filter": "test-filter",
		"limit":  10,
	}
	
	// Test context
	testContext := &mcp.Context{
		ID:        contextID,
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []mcp.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Set up expectations for context manager
	mockContextManager.On("GetContext", ctx, contextID).Return(testContext, nil)
	mockContextManager.On("UpdateContext", ctx, contextID, mock.Anything, mock.Anything).Return(testContext, nil).Times(2)
	
	// Set up expectations for adapter to return an error
	mockAdapter.On("Type").Return("test-tool")
	mockAdapter.On("GetData", ctx, query).Return(nil, assert.AnError)
	
	// Get the data
	result, err := bridge.GetToolData(ctx, contextID, tool, query)
	
	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)
	
	// Verify that all expectations were met
	mockContextManager.AssertExpectations(t)
	mockAdapter.AssertExpectations(t)
}
