//go:build exclude_storage_tests
// +build exclude_storage_tests

package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/storage/providers"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockContextStorage mocks the ContextStorage interface
type MockContextStorage struct {
	mock.Mock
}

// Ensure MockContextStorage implements providers.ContextStorage
var _ providers.ContextStorage = (*MockContextStorage)(nil)

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

func (m *MockContextStorage) ListContexts(ctx context.Context, agentID string, sessionID string) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// MockDB mocks the database
type MockDB struct {
	mock.Mock
}

func (m *MockDB) CreateContextReference(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockDB) GetContextReference(ctx context.Context, contextID string) (*database.ContextReference, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.ContextReference), args.Error(1)
}

func (m *MockDB) UpdateContextReference(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockDB) DeleteContextReference(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockDB) ListContextReferences(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*database.ContextReference, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*database.ContextReference), args.Error(1)
}

func (m *MockDB) CreateContextReferenceTable(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockCache mocks the cache
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func TestNewS3ContextManager(t *testing.T) {
	mockDB := new(MockDB)
	mockCache := new(MockCache)
	mockStorage := new(MockContextStorage)

	manager := NewS3ContextManager(mockDB, mockCache, mockStorage)

	assert.NotNil(t, manager)
	assert.Equal(t, mockDB, manager.db)
	assert.Equal(t, mockCache, manager.cache)
	assert.Equal(t, mockStorage, manager.s3Storage)
}

func TestS3ContextManager_CreateContext(t *testing.T) {
	mockDB := new(MockDB)
	mockCache := new(MockCache)
	mockStorage := new(MockContextStorage)
	
	manager := NewS3ContextManager(mockDB, mockCache, mockStorage)
	ctx := context.Background()

	// Test successful create
	contextData := &mcp.Context{
		ID:      "test-id",
		AgentID: "test-agent",
		ModelID: "test-model",
	}

	mockDB.On("CreateContextReference", ctx, mock.MatchedBy(func(c *mcp.Context) bool {
		return c.ID == contextData.ID && c.AgentID == contextData.AgentID && c.ModelID == contextData.ModelID
	})).Return(nil).Once()

	mockStorage.On("StoreContext", ctx, mock.MatchedBy(func(c *mcp.Context) bool {
		return c.ID == contextData.ID && c.AgentID == contextData.AgentID && c.ModelID == contextData.ModelID
	})).Return(nil).Once()

	mockCache.On("Set", mock.Anything, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "context:test-id")
	}), mock.Anything, mock.Anything).Return(nil).Once()

	result, err := manager.CreateContext(ctx, contextData)

	assert.NoError(t, err)
	assert.Equal(t, contextData.ID, result.ID)
	assert.Equal(t, contextData.AgentID, result.AgentID)
	assert.Equal(t, contextData.ModelID, result.ModelID)
	assert.False(t, result.CreatedAt.IsZero())
	assert.False(t, result.UpdatedAt.IsZero())
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockCache.AssertExpectations(t)

	// Test missing required fields
	contextData = &mcp.Context{
		ID: "test-id",
	}

	result, err = manager.CreateContext(ctx, contextData)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "agent_id is required")

	contextData = &mcp.Context{
		ID:      "test-id",
		AgentID: "test-agent",
	}

	result, err = manager.CreateContext(ctx, contextData)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "model_id is required")

	// Test database error
	contextData = &mcp.Context{
		ID:      "test-id",
		AgentID: "test-agent",
		ModelID: "test-model",
	}

	mockDB.On("CreateContextReference", ctx, mock.Anything).Return(errors.New("db error")).Once()

	result, err = manager.CreateContext(ctx, contextData)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create context reference")
	mockDB.AssertExpectations(t)

	// Test storage error
	mockDB.On("CreateContextReference", ctx, mock.Anything).Return(nil).Once()
	mockStorage.On("StoreContext", ctx, mock.Anything).Return(errors.New("storage error")).Once()

	result, err = manager.CreateContext(ctx, contextData)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to store context in S3")
	mockDB.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestS3ContextManager_GetContext(t *testing.T) {
	mockDB := new(MockDB)
	mockCache := new(MockCache)
	mockStorage := new(MockContextStorage)
	
	manager := NewS3ContextManager(mockDB, mockCache, mockStorage)
	ctx := context.Background()

	// Test get from cache
	contextData := &mcp.Context{
		ID:      "test-id",
		AgentID: "test-agent",
		ModelID: "test-model",
	}

	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Run(func(args mock.Arguments) {
		// Set the context data in the provided pointer
		val := args.Get(2).(*mcp.Context)
		*val = *contextData
	}).Return(nil).Once()

	result, err := manager.GetContext(ctx, "test-id")

	assert.NoError(t, err)
	assert.Equal(t, contextData.ID, result.ID)
	assert.Equal(t, contextData.AgentID, result.AgentID)
	assert.Equal(t, contextData.ModelID, result.ModelID)
	mockCache.AssertExpectations(t)

	// Test get from storage when cache misses
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(contextData, nil).Once()
	mockCache.On("Set", mock.Anything, "context:test-id", mock.Anything, mock.Anything).Return(nil).Once()

	result, err = manager.GetContext(ctx, "test-id")

	assert.NoError(t, err)
	assert.Equal(t, contextData.ID, result.ID)
	assert.Equal(t, contextData.AgentID, result.AgentID)
	assert.Equal(t, contextData.ModelID, result.ModelID)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)

	// Test storage error
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(nil, errors.New("storage error")).Once()

	result, err = manager.GetContext(ctx, "test-id")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get context from S3")
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestS3ContextManager_UpdateContext(t *testing.T) {
	mockDB := new(MockDB)
	mockCache := new(MockCache)
	mockStorage := new(MockContextStorage)
	
	manager := NewS3ContextManager(mockDB, mockCache, mockStorage)
	ctx := context.Background()

	// Create existing context
	existingContext := &mcp.Context{
		ID:            "test-id",
		AgentID:       "test-agent",
		ModelID:       "test-model",
		CurrentTokens: 10,
		MaxTokens:     100,
		Content: []mcp.ContextItem{
			{
				Role:    "system",
				Content: "Test content",
				Tokens:  10,
			},
		},
		Metadata: map[string]interface{}{
			"key1": "value1",
		},
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	// Test successful update
	updateContext := &mcp.Context{
		ID:      "test-id",
		AgentID: "new-agent",
		Content: []mcp.ContextItem{
			{
				Role:    "user",
				Content: "New content",
				Tokens:  5,
			},
		},
		Metadata: map[string]interface{}{
			"key2": "value2",
		},
	}

	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(existingContext, nil).Once()
	mockDB.On("UpdateContextReference", ctx, mock.Anything).Return(nil).Once()
	mockStorage.On("StoreContext", ctx, mock.Anything).Return(nil).Once()
	mockCache.On("Set", mock.Anything, "context:test-id", mock.Anything, mock.Anything).Return(nil).Once()

	result, err := manager.UpdateContext(ctx, "test-id", updateContext, nil)

	assert.NoError(t, err)
	assert.Equal(t, "test-id", result.ID)
	assert.Equal(t, "new-agent", result.AgentID)
	assert.Equal(t, "test-model", result.ModelID)
	assert.Equal(t, 15, result.CurrentTokens) // 10 + 5
	assert.Equal(t, 2, len(result.Content))
	assert.Equal(t, "value1", result.Metadata["key1"])
	assert.Equal(t, "value2", result.Metadata["key2"])
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)

	// Test context not found
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(nil, errors.New("not found")).Once()

	result, err = manager.UpdateContext(ctx, "test-id", updateContext, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)

	// Test database update error
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(existingContext, nil).Once()
	mockDB.On("UpdateContextReference", ctx, mock.Anything).Return(errors.New("db error")).Once()

	result, err = manager.UpdateContext(ctx, "test-id", updateContext, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update context reference")
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)

	// Test storage update error
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(existingContext, nil).Once()
	mockDB.On("UpdateContextReference", ctx, mock.Anything).Return(nil).Once()
	mockStorage.On("StoreContext", ctx, mock.Anything).Return(errors.New("storage error")).Once()

	result, err = manager.UpdateContext(ctx, "test-id", updateContext, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update context in S3")
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)

	// Test truncation
	existingContext.CurrentTokens = 80
	updateContext.Content = []mcp.ContextItem{
		{
			Role:    "user",
			Content: "Content that exceeds max tokens",
			Tokens:  30, // This will push the total to 110, exceeding the 100 max
		},
	}
	options := &mcp.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	}

	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(existingContext, nil).Once()
	mockDB.On("UpdateContextReference", ctx, mock.Anything).Return(nil).Once()
	mockStorage.On("StoreContext", ctx, mock.Anything).Return(nil).Once()
	mockCache.On("Set", mock.Anything, "context:test-id", mock.Anything, mock.Anything).Return(nil).Once()

	result, err = manager.UpdateContext(ctx, "test-id", updateContext, options)

	assert.NoError(t, err)
	assert.LessOrEqual(t, result.CurrentTokens, result.MaxTokens) // Should be truncated
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

func TestS3ContextManager_DeleteContext(t *testing.T) {
	mockDB := new(MockDB)
	mockCache := new(MockCache)
	mockStorage := new(MockContextStorage)
	
	manager := NewS3ContextManager(mockDB, mockCache, mockStorage)
	ctx := context.Background()

	// Test successful delete
	contextData := &mcp.Context{
		ID:      "test-id",
		AgentID: "test-agent",
		ModelID: "test-model",
	}

	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(contextData, nil).Once()
	mockStorage.On("DeleteContext", ctx, "test-id").Return(nil).Once()
	mockDB.On("DeleteContextReference", ctx, "test-id").Return(nil).Once()
	mockCache.On("Delete", ctx, "context:test-id").Return(nil).Once()

	err := manager.DeleteContext(ctx, "test-id")

	assert.NoError(t, err)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)

	// Test context not found
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(nil, errors.New("not found")).Once()

	err = manager.DeleteContext(ctx, "test-id")
	assert.Error(t, err)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)

	// Test storage delete error
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(contextData, nil).Once()
	mockStorage.On("DeleteContext", ctx, "test-id").Return(errors.New("storage error")).Once()

	err = manager.DeleteContext(ctx, "test-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete context from S3")
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestS3ContextManager_ListContexts(t *testing.T) {
	mockDB := new(MockDB)
	mockCache := new(MockCache)
	mockStorage := new(MockContextStorage)
	
	manager := NewS3ContextManager(mockDB, mockCache, mockStorage)
	ctx := context.Background()

	// Test successful list
	refs := []*database.ContextReference{
		{ID: "context1", AgentID: "agent1"},
		{ID: "context2", AgentID: "agent1"},
	}

	contextData1 := &mcp.Context{
		ID:      "context1",
		AgentID: "agent1",
		ModelID: "model1",
	}

	contextData2 := &mcp.Context{
		ID:      "context2",
		AgentID: "agent1",
		ModelID: "model1",
	}

	options := map[string]interface{}{"limit": 10}

	mockDB.On("ListContextReferences", ctx, "agent1", "session1", options).Return(refs, nil).Once()
	mockCache.On("Get", mock.Anything, "context:context1", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "context1").Return(contextData1, nil).Once()
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	mockCache.On("Get", mock.Anything, "context:context2", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "context2").Return(contextData2, nil).Once()
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	results, err := manager.ListContexts(ctx, "agent1", "session1", options)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "context1", results[0].ID)
	assert.Equal(t, "context2", results[1].ID)
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)

	// Test database error
	mockDB.On("ListContextReferences", ctx, "agent1", "session1", options).Return(nil, errors.New("db error")).Once()

	results, err = manager.ListContexts(ctx, "agent1", "session1", options)
	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "failed to list context references")
	mockDB.AssertExpectations(t)

	// Test context retrieval error
	refs = []*database.ContextReference{
		{ID: "context1", AgentID: "agent1"},
	}

	mockDB.On("ListContextReferences", ctx, "agent1", "session1", options).Return(refs, nil).Once()
	mockCache.On("Get", mock.Anything, "context:context1", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "context1").Return(nil, errors.New("storage error")).Once()

	results, err = manager.ListContexts(ctx, "agent1", "session1", options)
	assert.NoError(t, err) // Should not error, just log warning and continue
	assert.Len(t, results, 0) // No results due to error
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestS3ContextManager_SearchInContext(t *testing.T) {
	mockDB := new(MockDB)
	mockCache := new(MockCache)
	mockStorage := new(MockContextStorage)
	
	manager := NewS3ContextManager(mockDB, mockCache, mockStorage)
	ctx := context.Background()

	// Create test context with content
	contextData := &mcp.Context{
		ID:      "test-id",
		AgentID: "test-agent",
		ModelID: "test-model",
		Content: []mcp.ContextItem{
			{Role: "system", Content: "System instruction"},
			{Role: "user", Content: "Search for this text"},
			{Role: "assistant", Content: "Response without search term"},
		},
	}

	// Test successful search with results
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(contextData, nil).Once()
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	results, err := manager.SearchInContext(ctx, "test-id", "search")

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "user", results[0].Role)
	assert.Equal(t, "Search for this text", results[0].Content)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)

	// Test search with no matches
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(contextData, nil).Once()
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	results, err = manager.SearchInContext(ctx, "test-id", "no match")

	assert.NoError(t, err)
	assert.Len(t, results, 0)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)

	// Test context not found
	mockCache.On("Get", mock.Anything, "context:test-id", mock.Anything).Return(errors.New("cache miss")).Once()
	mockStorage.On("GetContext", ctx, "test-id").Return(nil, errors.New("not found")).Once()

	results, err = manager.SearchInContext(ctx, "test-id", "search")
	assert.Error(t, err)
	assert.Nil(t, results)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}
