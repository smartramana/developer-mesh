package core

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/cache/mocks"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockContextStorage mocks the S3 storage provider
type MockContextStorage struct {
	mock.Mock
}

func (m *MockContextStorage) StoreContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockContextStorage) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockContextStorage) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockContextStorage) ListContexts(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func TestNewS3ContextManager(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := NewS3ContextManager(mockDB, mockCache, mockStorage)
	assert.NotNil(t, cm)
	assert.Equal(t, mockDB, cm.db)
	assert.Equal(t, mockCache, cm.cache)
	assert.Equal(t, mockStorage, cm.s3Storage)
	assert.NotNil(t, cm.subscribers)
}

func TestS3ContextManagerCreateContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	
	// Test with valid input
	contextRequest := &mcp.Context{
		AgentID: "agent-123",
		ModelID: "model-123",
	}
	
	mockDB.On("CreateContextReference", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
	mockStorage.On("StoreContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	
	result, err := cm.CreateContext(ctx, contextRequest)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "agent-123", result.AgentID)
	assert.Equal(t, "model-123", result.ModelID)
	assert.False(t, result.CreatedAt.IsZero())
	assert.False(t, result.UpdatedAt.IsZero())
	
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test with missing required fields
	missingAgentID := &mcp.Context{
		ModelID: "model-123",
	}
	
	result, err = cm.CreateContext(ctx, missingAgentID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "agent_id is required")
	
	missingModelID := &mcp.Context{
		AgentID: "agent-123",
	}
	
	result, err = cm.CreateContext(ctx, missingModelID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "model_id is required")
	
	// Test with database error
	dbErrorContext := &mcp.Context{
		AgentID: "agent-err",
		ModelID: "model-err",
	}
	
	mockDB.On("CreateContextReference", ctx, mock.AnythingOfType("*mcp.Context")).Return(assert.AnError)
	
	result, err = cm.CreateContext(ctx, dbErrorContext)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create context reference")
	
	// Test with storage error
	storageErrorContext := &mcp.Context{
		AgentID: "agent-storage-err",
		ModelID: "model-storage-err",
	}
	
	mockDB.On("CreateContextReference", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
	mockStorage.On("StoreContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(assert.AnError)
	
	result, err = cm.CreateContext(ctx, storageErrorContext)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to store context in S3")
	
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestS3ContextManagerGetContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	contextID := "context-123"
	
	// Test cache hit
	expectedContext := &mcp.Context{
		ID:        contextID,
		AgentID:   "agent-123",
		ModelID:   "model-123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockCache.On("Get", mock.Anything, "context:"+contextID, mock.AnythingOfType("*mcp.Context")).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*mcp.Context)
			*arg = *expectedContext
		}).
		Return(nil)
	
	result, err := cm.GetContext(ctx, contextID)
	
	assert.NoError(t, err)
	assert.Equal(t, expectedContext, result)
	
	mockCache.AssertExpectations(t)
	
	// Test cache miss, S3 hit
	mockCache.On("Get", mock.Anything, "context:context-456", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	
	s3Context := &mcp.Context{
		ID:        "context-456",
		AgentID:   "agent-456",
		ModelID:   "model-456",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockStorage.On("GetContext", ctx, "context-456").Return(s3Context, nil)
	mockCache.On("Set", mock.Anything, "context:context-456", s3Context, mock.Anything).Return(nil)
	
	result, err = cm.GetContext(ctx, "context-456")
	
	assert.NoError(t, err)
	assert.Equal(t, s3Context, result)
	
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test not found
	mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	result, err = cm.GetContext(ctx, "not-found")
	
	assert.Error(t, err)
	assert.Nil(t, result)
	
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestS3ContextManagerUpdateContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	contextID := "context-123"
	
	// Mock the GetContext call
	existingContext := &mcp.Context{
		ID:            contextID,
		AgentID:       "agent-123",
		ModelID:       "model-123",
		Content:       []mcp.ContextItem{},
		CurrentTokens: 0,
		MaxTokens:     4000,
		Metadata:      map[string]interface{}{"existing": "value"},
		CreatedAt:     time.Now().Add(-1 * time.Hour),
		UpdatedAt:     time.Now().Add(-1 * time.Hour),
	}
	
	mockCache.On("Get", mock.Anything, "context:"+contextID, mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, contextID).Return(existingContext, nil)
	mockCache.On("Set", mock.Anything, "context:"+contextID, mock.AnythingOfType("*mcp.Context"), mock.Anything).
		Return(nil).Times(2)
	
	// Update request
	updateRequest := &mcp.Context{
		AgentID:  "agent-456",
		Metadata: map[string]interface{}{"new": "data"},
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "user",
				Content:   "Hello",
				Tokens:    1,
				Timestamp: time.Now(),
			},
		},
	}
	
	// Mock the database and storage update calls
	mockDB.On("UpdateContextReference", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
	mockStorage.On("StoreContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
	
	// Call the method
	result, err := cm.UpdateContext(ctx, contextID, updateRequest, nil)
	
	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, contextID, result.ID)
	assert.Equal(t, "agent-456", result.AgentID)
	assert.Equal(t, "model-123", result.ModelID)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, 1, result.CurrentTokens)
	assert.Equal(t, "value", result.Metadata["existing"])
	assert.Equal(t, "data", result.Metadata["new"])
	assert.False(t, result.UpdatedAt.Equal(existingContext.UpdatedAt))
	
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test error cases
	
	// GetContext error
	mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	result, err = cm.UpdateContext(ctx, "not-found", updateRequest, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	
	// Database update error
	mockCache.On("Get", mock.Anything, "context:db-err", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "db-err").Return(existingContext, nil)
	mockDB.On("UpdateContextReference", ctx, mock.AnythingOfType("*mcp.Context")).Return(assert.AnError)
	
	result, err = cm.UpdateContext(ctx, "db-err", updateRequest, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update context reference")
	
	// Storage update error
	mockCache.On("Get", mock.Anything, "context:storage-err", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "storage-err").Return(existingContext, nil)
	mockDB.On("UpdateContextReference", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
	mockStorage.On("StoreContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(assert.AnError)
	
	result, err = cm.UpdateContext(ctx, "storage-err", updateRequest, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update context in S3")
	
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestS3ContextManagerDeleteContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	contextID := "context-123"
	
	// Setup GetContext mock
	existingContext := &mcp.Context{
		ID:        contextID,
		AgentID:   "agent-123",
		ModelID:   "model-123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockCache.On("Get", mock.Anything, "context:"+contextID, mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, contextID).Return(existingContext, nil)
	mockCache.On("Set", mock.Anything, "context:"+contextID, existingContext, mock.Anything).Return(nil)
	
	// Setup delete mocks
	mockStorage.On("DeleteContext", ctx, contextID).Return(nil)
	mockDB.On("DeleteContextReference", ctx, contextID).Return(nil)
	mockCache.On("Delete", mock.Anything, "context:"+contextID).Return(nil)
	
	// Call the method
	err := cm.DeleteContext(ctx, contextID)
	
	// Assertions
	assert.NoError(t, err)
	
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test error cases
	
	// GetContext error
	mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	err = cm.DeleteContext(ctx, "not-found")
	assert.Error(t, err)
	
	// S3 delete error
	contextID2 := "context-s3-err"
	mockCache.On("Get", mock.Anything, "context:"+contextID2, mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, contextID2).Return(existingContext, nil)
	mockCache.On("Set", mock.Anything, "context:"+contextID2, existingContext, mock.Anything).Return(nil)
	mockStorage.On("DeleteContext", ctx, contextID2).Return(assert.AnError)
	
	err = cm.DeleteContext(ctx, contextID2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete context from S3")
	
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestS3ContextManagerListContexts(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	agentID := "agent-123"
	sessionID := "session-123"
	options := map[string]interface{}{
		"limit": 10,
	}
	
	// Reference objects from the database
	references := []mcp.ContextReference{
		{
			ID:        "context-1",
			AgentID:   agentID,
			SessionID: sessionID,
		},
		{
			ID:        "context-2",
			AgentID:   agentID,
			SessionID: sessionID,
		},
	}
	
	// Full context objects from S3
	context1 := &mcp.Context{
		ID:        "context-1",
		AgentID:   agentID,
		SessionID: sessionID,
		Content:   []mcp.ContextItem{},
	}
	
	context2 := &mcp.Context{
		ID:        "context-2",
		AgentID:   agentID,
		SessionID: sessionID,
		Content:   []mcp.ContextItem{},
	}
	
	mockDB.On("ListContextReferences", ctx, agentID, sessionID, options).Return(references, nil)
	
	// Setup GetContext mocks
	mockCache.On("Get", mock.Anything, "context:context-1", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "context-1").Return(context1, nil)
	mockCache.On("Set", mock.Anything, "context:context-1", context1, mock.Anything).Return(nil)
	
	mockCache.On("Get", mock.Anything, "context:context-2", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "context-2").Return(context2, nil)
	mockCache.On("Set", mock.Anything, "context:context-2", context2, mock.Anything).Return(nil)
	
	// Call the method
	result, err := cm.ListContexts(ctx, agentID, sessionID, options)
	
	// Assertions
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "context-1", result[0].ID)
	assert.Equal(t, "context-2", result[1].ID)
	
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test database error
	mockDB.On("ListContextReferences", ctx, "not-found", "", nil).Return(nil, assert.AnError)
	
	result, err = cm.ListContexts(ctx, "not-found", "", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list context references")
	
	mockDB.AssertExpectations(t)
}

func TestS3ContextManagerSummarizeContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	contextID := "context-123"
	
	// Mock the GetContext call
	existingContext := &mcp.Context{
		ID:            contextID,
		AgentID:       "agent-123",
		ModelID:       "model-123",
		CurrentTokens: 100,
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "user",
				Content:   "Hello",
				Tokens:    1,
				Timestamp: time.Now(),
			},
			{
				ID:        "item-2",
				Role:      "assistant",
				Content:   "Hi there",
				Tokens:    2,
				Timestamp: time.Now(),
			},
		},
	}
	
	mockCache.On("Get", mock.Anything, "context:"+contextID, mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, contextID).Return(existingContext, nil)
	mockCache.On("Set", mock.Anything, "context:"+contextID, existingContext, mock.Anything).Return(nil)
	
	// Call the method
	summary, err := cm.SummarizeContext(ctx, contextID)
	
	// Assertions
	assert.NoError(t, err)
	assert.Contains(t, summary, "2 messages")
	assert.Contains(t, summary, "100 tokens")
	
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test error case
	mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	summary, err = cm.SummarizeContext(ctx, "not-found")
	
	assert.Error(t, err)
	assert.Empty(t, summary)
	
	mockStorage.AssertExpectations(t)
}

func TestS3ContextManagerSearchInContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	contextID := "context-123"
	
	// Mock the GetContext call
	existingContext := &mcp.Context{
		ID:      contextID,
		AgentID: "agent-123",
		ModelID: "model-123",
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "user",
				Content:   "Hello world",
				Tokens:    2,
				Timestamp: time.Now(),
			},
			{
				ID:        "item-2",
				Role:      "assistant",
				Content:   "Hi there, how can I help you?",
				Tokens:    7,
				Timestamp: time.Now(),
			},
			{
				ID:        "item-3",
				Role:      "user",
				Content:   "I need information about the world",
				Tokens:    7,
				Timestamp: time.Now(),
			},
		},
	}
	
	mockCache.On("Get", mock.Anything, "context:"+contextID, mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, contextID).Return(existingContext, nil)
	mockCache.On("Set", mock.Anything, "context:"+contextID, existingContext, mock.Anything).Return(nil)
	
	// Test search with results
	results, err := cm.SearchInContext(ctx, contextID, "world")
	
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "Hello world", results[0].Content)
	assert.Equal(t, "I need information about the world", results[1].Content)
	
	// Test search with no results
	results, err = cm.SearchInContext(ctx, contextID, "nonexistent")
	
	assert.NoError(t, err)
	assert.Len(t, results, 0)
	
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test error case
	mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockStorage.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	results, err = cm.SearchInContext(ctx, "not-found", "world")
	
	assert.Error(t, err)
	assert.Nil(t, results)
	
	mockStorage.AssertExpectations(t)
}

func TestS3ContextManagerContainsTextCaseInsensitive(t *testing.T) {
	// Test with matching text
	assert.True(t, containsTextCaseInsensitive("Hello World", "world"))
	assert.True(t, containsTextCaseInsensitive("HELLO WORLD", "world"))
	assert.True(t, containsTextCaseInsensitive("hello world", "WORLD"))
	
	// Test with non-matching text
	assert.False(t, containsTextCaseInsensitive("Hello", "world"))
	assert.False(t, containsTextCaseInsensitive("", "world"))
}

func TestS3ContextManagerSubscribe(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	// Add a subscriber
	var receivedEvent mcp.Event
	cm.Subscribe("context_created", func(event mcp.Event) {
		receivedEvent = event
	})
	
	// Check that the subscriber was added
	assert.Len(t, cm.subscribers["context_created"], 1)
	
	// Add another subscriber for a different event type
	cm.Subscribe("context_updated", func(event mcp.Event) {
		// Do nothing
	})
	
	// Check that both subscribers exist
	assert.Len(t, cm.subscribers["context_created"], 1)
	assert.Len(t, cm.subscribers["context_updated"], 1)
	
	// Add a subscriber for all events
	cm.Subscribe("all", func(event mcp.Event) {
		// Do nothing
	})
	
	assert.Len(t, cm.subscribers["all"], 1)
}

func TestS3ContextManagerPublishEvent(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	// Add a subscriber for a specific event type
	specificCalled := false
	cm.subscribers["context_created"] = []func(mcp.Event){
		func(event mcp.Event) {
			specificCalled = true
			assert.Equal(t, "context_created", event.Type)
		},
	}
	
	// Add a subscriber for all events
	allCalled := false
	cm.subscribers["all"] = []func(mcp.Event){
		func(event mcp.Event) {
			allCalled = true
			assert.Equal(t, "context_created", event.Type)
		},
	}
	
	// Create an event
	event := mcp.Event{
		Source:    "test",
		Type:      "context_created",
		AgentID:   "agent-123",
		Timestamp: time.Now(),
	}
	
	// Publish the event
	cm.publishEvent(event)
	
	// Allow time for goroutines to execute
	time.Sleep(100 * time.Millisecond)
	
	// Both subscribers should have been called
	assert.True(t, specificCalled)
	assert.True(t, allCalled)
	
	// Test with no subscribers
	cm = &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	// This should not panic
	cm.publishEvent(event)
}

func TestS3ContextManagerTruncateContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	// Test different truncation strategies
	contextData := &mcp.Context{
		MaxTokens:     10,
		CurrentTokens: 15,
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "user",
				Content:   "First message",
				Tokens:    5,
				Timestamp: time.Now().Add(-2 * time.Hour),
			},
			{
				ID:        "item-2",
				Role:      "assistant",
				Content:   "Second message",
				Tokens:    5,
				Timestamp: time.Now().Add(-1 * time.Hour),
			},
			{
				ID:        "item-3",
				Role:      "user",
				Content:   "Third message",
				Tokens:    5,
				Timestamp: time.Now(),
			},
		},
	}
	
	// Test oldest first strategy
	err := cm.truncateContext(contextData, string(TruncateOldestFirst))
	assert.NoError(t, err)
	assert.Len(t, contextData.Content, 2)
	assert.Equal(t, "Second message", contextData.Content[0].Content)
	assert.Equal(t, "Third message", contextData.Content[1].Content)
	assert.Equal(t, 10, contextData.CurrentTokens)
	
	// Reset context
	contextData = &mcp.Context{
		MaxTokens:     10,
		CurrentTokens: 15,
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "user",
				Content:   "First message",
				Tokens:    5,
				Timestamp: time.Now().Add(-2 * time.Hour),
			},
			{
				ID:        "item-2",
				Role:      "assistant",
				Content:   "Second message",
				Tokens:    5,
				Timestamp: time.Now().Add(-1 * time.Hour),
			},
			{
				ID:        "item-3",
				Role:      "user",
				Content:   "Third message",
				Tokens:    5,
				Timestamp: time.Now(),
			},
		},
	}
	
	// Test relevance based strategy (should fallback to oldest first)
	err = cm.truncateContext(contextData, string(TruncateRelevanceBased))
	assert.NoError(t, err)
	assert.Len(t, contextData.Content, 2)
	assert.Equal(t, "Second message", contextData.Content[0].Content)
	assert.Equal(t, "Third message", contextData.Content[1].Content)
	
	// Reset context
	contextData = &mcp.Context{
		MaxTokens:     10,
		CurrentTokens: 15,
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "user",
				Content:   "First message",
				Tokens:    5,
				Timestamp: time.Now().Add(-2 * time.Hour),
			},
			{
				ID:        "item-2",
				Role:      "assistant",
				Content:   "Second message",
				Tokens:    5,
				Timestamp: time.Now().Add(-1 * time.Hour),
			},
			{
				ID:        "item-3",
				Role:      "user",
				Content:   "Third message",
				Tokens:    5,
				Timestamp: time.Now(),
			},
		},
	}
	
	// Test compression strategy (should fallback to oldest first)
	err = cm.truncateContext(contextData, string(TruncateCompression))
	assert.NoError(t, err)
	assert.Len(t, contextData.Content, 2)
	assert.Equal(t, "Second message", contextData.Content[0].Content)
	assert.Equal(t, "Third message", contextData.Content[1].Content)
	
	// Reset context
	contextData = &mcp.Context{
		MaxTokens:     10,
		CurrentTokens: 15,
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "user",
				Content:   "First message",
				Tokens:    5,
				Timestamp: time.Now().Add(-2 * time.Hour),
			},
			{
				ID:        "item-2",
				Role:      "assistant",
				Content:   "Second message",
				Tokens:    5,
				Timestamp: time.Now().Add(-1 * time.Hour),
			},
			{
				ID:        "item-3",
				Role:      "user",
				Content:   "Third message",
				Tokens:    5,
				Timestamp: time.Now(),
			},
		},
	}
	
	// Test invalid strategy (should fallback to oldest first)
	err = cm.truncateContext(contextData, "invalid_strategy")
	assert.NoError(t, err)
	assert.Len(t, contextData.Content, 2)
	assert.Equal(t, "Second message", contextData.Content[0].Content)
	assert.Equal(t, "Third message", contextData.Content[1].Content)
}

func TestS3ContextManagerTruncatePreservingUser(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		db:          mockDB,
		cache:       mockCache,
		s3Storage:   mockStorage,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	// Test with a complex context with different message types
	contextData := &mcp.Context{
		MaxTokens:     15,
		CurrentTokens: 25,
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "system",
				Content:   "System message",
				Tokens:    5,
				Timestamp: time.Now().Add(-3 * time.Hour),
			},
			{
				ID:        "item-2",
				Role:      "user",
				Content:   "User message 1",
				Tokens:    5,
				Timestamp: time.Now().Add(-2 * time.Hour),
			},
			{
				ID:        "item-3",
				Role:      "assistant",
				Content:   "Assistant message 1",
				Tokens:    5,
				Timestamp: time.Now().Add(-1 * time.Hour),
			},
			{
				ID:        "item-4",
				Role:      "user",
				Content:   "User message 2",
				Tokens:    5,
				Timestamp: time.Now().Add(-30 * time.Minute),
			},
			{
				ID:        "item-5",
				Role:      "assistant",
				Content:   "Assistant message 2",
				Tokens:    5,
				Timestamp: time.Now(),
			},
		},
	}
	
	err := cm.truncatePreservingUser(contextData)
	
	assert.NoError(t, err)
	// Check that token count is below max
	assert.LessOrEqual(t, contextData.CurrentTokens, contextData.MaxTokens)
	
	// Test with a small context (not exceeding minRecentMessages)
	smallContext := &mcp.Context{
		MaxTokens:     10,
		CurrentTokens: 15,
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "user",
				Content:   "User message",
				Tokens:    5,
				Timestamp: time.Now().Add(-1 * time.Hour),
			},
			{
				ID:        "item-2",
				Role:      "assistant",
				Content:   "Assistant message",
				Tokens:    5,
				Timestamp: time.Now().Add(-30 * time.Minute),
			},
			{
				ID:        "item-3",
				Role:      "user",
				Content:   "Another user message",
				Tokens:    5,
				Timestamp: time.Now(),
			},
		},
	}
	
	err = cm.truncatePreservingUser(smallContext)
	
	assert.NoError(t, err)
	// Small context should not be modified since it's <= minRecentMessages
	assert.Len(t, smallContext.Content, 3)
	
	// Test a case that would still be over the limit and need fallback to oldest first
	overLimitContext := &mcp.Context{
		MaxTokens:     5,
		CurrentTokens: 20,
		Content: []mcp.ContextItem{
			{
				ID:        "item-1",
				Role:      "system",
				Content:   "System message",
				Tokens:    5,
				Timestamp: time.Now().Add(-3 * time.Hour),
			},
			{
				ID:        "item-2",
				Role:      "user",
				Content:   "User message 1",
				Tokens:    5,
				Timestamp: time.Now().Add(-2 * time.Hour),
			},
			{
				ID:        "item-3",
				Role:      "user",
				Content:   "User message 2",
				Tokens:    5,
				Timestamp: time.Now().Add(-1 * time.Hour),
			},
			{
				ID:        "item-4",
				Role:      "user",
				Content:   "User message 3",
				Tokens:    5,
				Timestamp: time.Now(),
			},
		},
	}
	
	// This should fall back to truncateOldestFirst since all messages are user/system
	err = cm.truncatePreservingUser(overLimitContext)
	
	assert.NoError(t, err)
	assert.LessOrEqual(t, overLimitContext.CurrentTokens, overLimitContext.MaxTokens)
}

func TestS3ContextManagerCacheContext(t *testing.T) {
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		cache:     mockCache,
		s3Storage: mockStorage,
	}
	
	// Test with no expiration
	contextData := &mcp.Context{
		ID: "context-123",
	}
	
	mockCache.On("Set", mock.Anything, "context:context-123", contextData, mock.AnythingOfType("time.Duration")).Return(nil)
	
	err := cm.cacheContext(contextData)
	
	assert.NoError(t, err)
	mockCache.AssertExpectations(t)
	
	// Test with expiration
	tomorrow := time.Now().Add(24 * time.Hour)
	expiringContext := &mcp.Context{
		ID:        "context-expire",
		ExpiresAt: tomorrow,
	}
	
	mockCache.On("Set", mock.Anything, "context:context-expire", expiringContext, mock.AnythingOfType("time.Duration")).Return(nil)
	
	err = cm.cacheContext(expiringContext)
	
	assert.NoError(t, err)
	mockCache.AssertExpectations(t)
	
	// Test with already expired
	yesterday := time.Now().Add(-24 * time.Hour)
	expiredContext := &mcp.Context{
		ID:        "context-expired",
		ExpiresAt: yesterday,
	}
	
	// Should not call Set for expired contexts
	err = cm.cacheContext(expiredContext)
	
	assert.NoError(t, err)
	mockCache.AssertExpectations(t)
}

func TestS3ContextManagerGetCachedContext(t *testing.T) {
	mockCache := new(mocks.MockCache)
	mockStorage := new(MockContextStorage)
	
	cm := &S3ContextManager{
		cache:     mockCache,
		s3Storage: mockStorage,
	}
	
	contextID := "context-123"
	expectedContext := &mcp.Context{
		ID:      contextID,
		AgentID: "agent-123",
	}
	
	// Test successful cache get
	mockCache.On("Get", mock.Anything, "context:"+contextID, mock.AnythingOfType("*mcp.Context")).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*mcp.Context)
			*arg = *expectedContext
		}).
		Return(nil)
	
	result, err := cm.getCachedContext(contextID)
	
	assert.NoError(t, err)
	assert.Equal(t, expectedContext, result)
	
	mockCache.AssertExpectations(t)
	
	// Test cache miss
	mockCache.On("Get", mock.Anything, "context:cache-miss", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	
	result, err = cm.getCachedContext("cache-miss")
	
	assert.Error(t, err)
	assert.Nil(t, result)
	
	mockCache.AssertExpectations(t)
}
