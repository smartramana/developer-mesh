package core

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/cache/mocks"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDatabase is a mock for the database.Database struct
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockDatabase) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockDatabase) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockDatabase) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockDatabase) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

func TestNewContextManager(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := NewContextManager(nil, mockCache)
	assert.NotNil(t, cm)
	assert.NotNil(t, cm.subscribers)
}

func TestCreateContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		db:          mockDB,
		cache:       mockCache,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	
	// Test with valid input
	contextRequest := &mcp.Context{
		AgentID: "agent-123",
		ModelID: "model-123",
	}
	
	mockDB.On("CreateContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
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
}

func TestGetContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		db:          mockDB,
		cache:       mockCache,
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
	
	// Test cache miss, database hit
	mockCache.On("Get", mock.Anything, "context:context-456", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	
	dbContext := &mcp.Context{
		ID:        "context-456",
		AgentID:   "agent-456",
		ModelID:   "model-456",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockDB.On("GetContext", ctx, "context-456").Return(dbContext, nil)
	mockCache.On("Set", mock.Anything, "context:context-456", dbContext, mock.Anything).Return(nil)
	
	result, err = cm.GetContext(ctx, "context-456")
	
	assert.NoError(t, err)
	assert.Equal(t, dbContext, result)
	
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test not found
	mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockDB.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	result, err = cm.GetContext(ctx, "not-found")
	
	assert.Error(t, err)
	assert.Nil(t, result)
	
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestUpdateContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		db:          mockDB,
		cache:       mockCache,
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
	mockDB.On("GetContext", ctx, contextID).Return(existingContext, nil)
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
	
	// Mock the UpdateContext call
	mockDB.On("UpdateContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
	
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
	mockCache.AssertExpectations(t)
	
	// Test truncation
	largeContext := &mcp.Context{
		ID:            "context-large",
		AgentID:       "agent-123",
		ModelID:       "model-123",
		Content:       []mcp.ContextItem{},
		CurrentTokens: 0,
		MaxTokens:     100,
		CreatedAt:     time.Now().Add(-1 * time.Hour),
		UpdatedAt:     time.Now().Add(-1 * time.Hour),
	}
	
	mockCache.On("Get", mock.Anything, "context:context-large", mock.AnythingOfType("*mcp.Context")).
		Return(assert.AnError)
	mockDB.On("GetContext", ctx, "context-large").Return(largeContext, nil)
	
	// Large update that exceeds max tokens
	largeUpdate := &mcp.Context{
		Content: []mcp.ContextItem{
			{
				ID:        "item-large",
				Role:      "user",
				Content:   "This is a large message",
				Tokens:    200, // Exceeds max tokens
				Timestamp: time.Now(),
			},
		},
	}
	
	options := &mcp.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: string(TruncateOldestFirst),
	}
	
	mockDB.On("UpdateContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
	
	result, err = cm.UpdateContext(ctx, "context-large", largeUpdate, options)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 0) // Content should be truncated
	
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestDeleteContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		db:          mockDB,
		cache:       mockCache,
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
	mockDB.On("GetContext", ctx, contextID).Return(existingContext, nil)
	mockCache.On("Set", mock.Anything, "context:"+contextID, existingContext, mock.Anything).Return(nil)
	
	// Setup DeleteContext mock
	mockDB.On("DeleteContext", ctx, contextID).Return(nil)
	mockCache.On("Delete", mock.Anything, "context:"+contextID).Return(nil)
	
	// Call the method
	err := cm.DeleteContext(ctx, contextID)
	
	// Assertions
	assert.NoError(t, err)
	
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test error case
	mockDB.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	err = cm.DeleteContext(ctx, "not-found")
	
	assert.Error(t, err)
	
	mockDB.AssertExpectations(t)
}

func TestListContexts(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		db:          mockDB,
		cache:       mockCache,
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	ctx := context.Background()
	agentID := "agent-123"
	sessionID := "session-123"
	options := map[string]interface{}{
		"limit": 10,
	}
	
	// Expected contexts
	contexts := []*mcp.Context{
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
	
	mockDB.On("ListContexts", ctx, agentID, sessionID, options).Return(contexts, nil)
	
	// Call the method
	result, err := cm.ListContexts(ctx, agentID, sessionID, options)
	
	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, contexts, result)
	
	mockDB.AssertExpectations(t)
	
	// Test error case
	mockDB.On("ListContexts", ctx, "not-found", "", nil).Return(nil, assert.AnError)
	
	result, err = cm.ListContexts(ctx, "not-found", "", nil)
	
	assert.Error(t, err)
	assert.Nil(t, result)
	
	mockDB.AssertExpectations(t)
}

func TestSummarizeContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		db:          mockDB,
		cache:       mockCache,
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
	mockDB.On("GetContext", ctx, contextID).Return(existingContext, nil)
	mockCache.On("Set", mock.Anything, "context:"+contextID, existingContext, mock.Anything).Return(nil)
	
	// Call the method
	summary, err := cm.SummarizeContext(ctx, contextID)
	
	// Assertions
	assert.NoError(t, err)
	assert.Contains(t, summary, "2 messages")
	assert.Contains(t, summary, "100 tokens")
	
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test error case
	mockDB.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	summary, err = cm.SummarizeContext(ctx, "not-found")
	
	assert.Error(t, err)
	assert.Empty(t, summary)
	
	mockDB.AssertExpectations(t)
}

func TestSearchInContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		db:          mockDB,
		cache:       mockCache,
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
	mockDB.On("GetContext", ctx, contextID).Return(existingContext, nil)
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
	
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	
	// Test error case
	mockDB.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
	
	results, err = cm.SearchInContext(ctx, "not-found", "world")
	
	assert.Error(t, err)
	assert.Nil(t, results)
	
	mockDB.AssertExpectations(t)
}

func TestSubscribe(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		db:          mockDB,
		cache:       mockCache,
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

func TestTruncateContext(t *testing.T) {
	// Test truncateOldestFirst
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
	
	cm := &ContextManager{}
	err := cm.truncateOldestFirst(contextData)
	
	assert.NoError(t, err)
	assert.Len(t, contextData.Content, 2)
	assert.Equal(t, "Second message", contextData.Content[0].Content)
	assert.Equal(t, "Third message", contextData.Content[1].Content)
	assert.Equal(t, 10, contextData.CurrentTokens)
	
	// Test truncatePreservingUser
	contextData = &mcp.Context{
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
	
	err = cm.truncatePreservingUser(contextData)
	
	assert.NoError(t, err)
	// The last 4 messages should be kept since they are the recent ones
	assert.LessOrEqual(t, contextData.CurrentTokens, contextData.MaxTokens)
}

func TestCacheContext(t *testing.T) {
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		cache: mockCache,
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

func TestGetCachedContext(t *testing.T) {
	mockCache := new(mocks.MockCache)
	
	cm := &ContextManager{
		cache: mockCache,
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

func TestPublishEvent(t *testing.T) {
	cm := &ContextManager{
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
	cm = &ContextManager{
		subscribers: make(map[string][]func(mcp.Event)),
	}
	
	// This should not panic
	cm.publishEvent(event)
}
