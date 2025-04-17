package adapters

import (
	"context"
	"testing"

	"github.com/S-Corkum/mcp-server/internal/interfaces"
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) UpdateContext(ctx context.Context, contextID string, updateRequest *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	args := m.Called(ctx, contextID, updateRequest, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextManager) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockContextManager) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

func (m *MockContextManager) RegisterEventSubscriber(eventType string, callback func(mcp.Event)) {
	m.Called(eventType, callback)
}

func (m *MockContextManager) EmitEvent(event mcp.Event) {
	m.Called(event)
}

// Ensure the mock implements the interface
var _ interfaces.ContextManager = (*MockContextManager)(nil)

func TestNewContextAwareAdapter(t *testing.T) {
	mockCtxManager := new(MockContextManager)
	adapter := NewContextAwareAdapter(mockCtxManager, "test-adapter")
	
	assert.NotNil(t, adapter)
	assert.Equal(t, mockCtxManager, adapter.contextManager)
	assert.Equal(t, "test-adapter", adapter.adapterName)
}

func TestRecordOperationInContext(t *testing.T) {
	// Setup
	mockCtxManager := new(MockContextManager)
	adapter := NewContextAwareAdapter(mockCtxManager, "test-adapter")
	ctx := context.Background()
	contextID := "test-context-id"
	
	// Test data
	testRequest := map[string]string{"action": "test"}
	testResponse := map[string]string{"result": "success"}
	
	// Mock context
	mockContext := &mcp.Context{
		ID:            contextID,
		AgentID:       "test-agent",
		Content:       []mcp.ContextItem{},
		CurrentTokens: 0,
	}
	
	// Setup expectations
	mockCtxManager.On("GetContext", ctx, contextID).Return(mockContext, nil)
	mockCtxManager.On("UpdateContext", ctx, contextID, mock.Anything, mock.Anything).Return(mockContext, nil)
	
	// Execute
	err := adapter.RecordOperationInContext(ctx, contextID, "test-operation", testRequest, testResponse)
	
	// Assert
	assert.NoError(t, err)
	mockCtxManager.AssertExpectations(t)
	
	// Verify the context was updated with the operation
	mockCtxManager.AssertCalled(t, "UpdateContext", ctx, contextID, mock.MatchedBy(func(ctx *mcp.Context) bool {
		// Check that the context has one item
		if len(ctx.Content) != 1 {
			return false
		}
		
		// Check the item has the correct data
		item := ctx.Content[0]
		assert.Equal(t, "tool", item.Role)
		assert.Contains(t, item.Content, "Operation: test-operation")
		assert.Contains(t, item.Content, "Adapter: test-adapter")
		
		// Check metadata
		assert.Equal(t, "test-adapter", item.Metadata["adapter"])
		assert.Equal(t, "test-operation", item.Metadata["operation"])
		
		return true
	}), mock.Anything)
}

func TestRecordOperationInContext_GetContextError(t *testing.T) {
	// Setup
	mockCtxManager := new(MockContextManager)
	adapter := NewContextAwareAdapter(mockCtxManager, "test-adapter")
	ctx := context.Background()
	contextID := "test-context-id"
	
	// Test data
	testRequest := map[string]string{"action": "test"}
	testResponse := map[string]string{"result": "success"}
	
	// Setup expectations with error
	mockCtxManager.On("GetContext", ctx, contextID).Return(nil, assert.AnError)
	
	// Execute
	err := adapter.RecordOperationInContext(ctx, contextID, "test-operation", testRequest, testResponse)
	
	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get context")
	mockCtxManager.AssertExpectations(t)
	mockCtxManager.AssertNotCalled(t, "UpdateContext")
}

func TestRecordWebhookInContext(t *testing.T) {
	// Setup
	mockCtxManager := new(MockContextManager)
	adapter := NewContextAwareAdapter(mockCtxManager, "test-adapter")
	ctx := context.Background()
	agentID := "test-agent-id"
	
	// Test data
	testPayload := map[string]string{"event": "test-event-data"}
	
	// Mock existing context
	existingContextID := "existing-context-id"
	mockContext := &mcp.Context{
		ID:            existingContextID,
		AgentID:       agentID,
		Content:       []mcp.ContextItem{},
		CurrentTokens: 0,
	}
	
	// Setup expectations
	mockCtxManager.On("ListContexts", ctx, agentID, "", mock.Anything).
		Return([]*mcp.Context{mockContext}, nil)
	mockCtxManager.On("UpdateContext", ctx, existingContextID, mock.Anything, mock.Anything).
		Return(mockContext, nil)
	
	// Execute
	resultContextID, err := adapter.RecordWebhookInContext(ctx, agentID, "test-event", testPayload)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, existingContextID, resultContextID)
	mockCtxManager.AssertExpectations(t)
	
	// Verify the context was updated with the webhook
	mockCtxManager.AssertCalled(t, "UpdateContext", ctx, existingContextID, mock.MatchedBy(func(ctx *mcp.Context) bool {
		// Check that the context has one item
		if len(ctx.Content) != 1 {
			return false
		}
		
		// Check the item has the correct data
		item := ctx.Content[0]
		assert.Equal(t, "event", item.Role)
		assert.Contains(t, item.Content, "Event: test-event")
		assert.Contains(t, item.Content, "Adapter: test-adapter")
		
		// Check metadata
		assert.Equal(t, "test-adapter", item.Metadata["adapter"])
		assert.Equal(t, "test-event", item.Metadata["eventType"])
		
		return true
	}), mock.Anything)
}

func TestRecordWebhookInContext_NoExistingContext(t *testing.T) {
	// Setup
	mockCtxManager := new(MockContextManager)
	adapter := NewContextAwareAdapter(mockCtxManager, "test-adapter")
	ctx := context.Background()
	agentID := "test-agent-id"
	
	// Test data
	testPayload := map[string]string{"event": "test-event-data"}
	
	// Mock new context
	newContextID := "new-context-id"
	mockNewContext := &mcp.Context{
		ID:            newContextID,
		AgentID:       agentID,
		ModelID:       "webhook",
		Content:       []mcp.ContextItem{},
		CurrentTokens: 0,
		MaxTokens:     100000,
		Metadata: map[string]interface{}{
			"source": "webhook",
		},
	}
	
	// Setup expectations
	mockCtxManager.On("ListContexts", ctx, agentID, "", mock.Anything).
		Return([]*mcp.Context{}, nil)
	mockCtxManager.On("CreateContext", ctx, mock.AnythingOfType("*mcp.Context")).
		Return(mockNewContext, nil)
	mockCtxManager.On("UpdateContext", ctx, newContextID, mock.Anything, mock.Anything).
		Return(mockNewContext, nil)
	
	// Execute
	resultContextID, err := adapter.RecordWebhookInContext(ctx, agentID, "test-event", testPayload)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, newContextID, resultContextID)
	mockCtxManager.AssertExpectations(t)
	
	// Verify a new context was created
	mockCtxManager.AssertCalled(t, "CreateContext", ctx, mock.MatchedBy(func(ctx *mcp.Context) bool {
		return ctx.AgentID == agentID && 
			   ctx.ModelID == "webhook" && 
			   ctx.MaxTokens == 100000 &&
			   ctx.Metadata["source"] == "webhook"
	}))
	
	// Verify the context was updated with the webhook
	mockCtxManager.AssertCalled(t, "UpdateContext", ctx, newContextID, mock.Anything, mock.Anything)
}

func TestRecordWebhookInContext_ListContextsError(t *testing.T) {
	// Setup
	mockCtxManager := new(MockContextManager)
	adapter := NewContextAwareAdapter(mockCtxManager, "test-adapter")
	ctx := context.Background()
	agentID := "test-agent-id"
	
	// Test data
	testPayload := map[string]string{"event": "test-event-data"}
	
	// Setup expectations with error
	mockCtxManager.On("ListContexts", ctx, agentID, "", mock.Anything).
		Return(nil, assert.AnError)
	
	// Execute
	_, err := adapter.RecordWebhookInContext(ctx, agentID, "test-event", testPayload)
	
	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list contexts")
	mockCtxManager.AssertExpectations(t)
	mockCtxManager.AssertNotCalled(t, "CreateContext")
	mockCtxManager.AssertNotCalled(t, "UpdateContext")
}

func TestRecordWebhookInContext_CreateContextError(t *testing.T) {
	// Setup
	mockCtxManager := new(MockContextManager)
	adapter := NewContextAwareAdapter(mockCtxManager, "test-adapter")
	ctx := context.Background()
	agentID := "test-agent-id"
	
	// Test data
	testPayload := map[string]string{"event": "test-event-data"}
	
	// Setup expectations with error
	mockCtxManager.On("ListContexts", ctx, agentID, "", mock.Anything).
		Return([]*mcp.Context{}, nil)
	mockCtxManager.On("CreateContext", ctx, mock.AnythingOfType("*mcp.Context")).
		Return(nil, assert.AnError)
	
	// Execute
	_, err := adapter.RecordWebhookInContext(ctx, agentID, "test-event", testPayload)
	
	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create context")
	mockCtxManager.AssertExpectations(t)
	mockCtxManager.AssertNotCalled(t, "UpdateContext")
}
