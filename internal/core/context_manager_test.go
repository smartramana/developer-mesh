package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/cache/mocks"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)



func TestNewContextManager(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	// Mock necessary functions
	mockDB.On("GetDB").Return(&sqlx.DB{})
	
	cm := NewContextManager(mockDB, mockCache)
	assert.NotNil(t, cm)
	assert.NotNil(t, cm.subscribers)
}

func TestCreateContext(t *testing.T) {
	// Create test context
	ctx := context.Background()
	
	// Define test cases
	testCases := []struct {
		name           string
		contextRequest *mcp.Context
		setupMocks     func(mockDB *MockDB, mockCache *mocks.MockCache)
		expectedError  bool
		errorMessage   string
		validateResult func(t *testing.T, result *mcp.Context)
	}{
		{
			name: "valid context",
			contextRequest: &mcp.Context{
				AgentID: "agent-123",
				ModelID: "model-123",
			},
			setupMocks: func(mockDB *MockDB, mockCache *mocks.MockCache) {
				mockDB.On("CreateContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(nil)
				mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *mcp.Context) {
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.ID)
				assert.Equal(t, "agent-123", result.AgentID)
				assert.Equal(t, "model-123", result.ModelID)
				assert.False(t, result.CreatedAt.IsZero())
				assert.False(t, result.UpdatedAt.IsZero())
			},
		},
		{
			name: "missing agent ID",
			contextRequest: &mcp.Context{
				ModelID: "model-123",
			},
			setupMocks:    func(mockDB *MockDB, mockCache *mocks.MockCache) {},
			expectedError: true,
			errorMessage:  "agent_id is required",
			validateResult: func(t *testing.T, result *mcp.Context) {
				assert.Nil(t, result)
			},
		},
		{
			name: "missing model ID",
			contextRequest: &mcp.Context{
				AgentID: "agent-123",
			},
			setupMocks:    func(mockDB *MockDB, mockCache *mocks.MockCache) {},
			expectedError: true,
			errorMessage:  "model_id is required",
			validateResult: func(t *testing.T, result *mcp.Context) {
				assert.Nil(t, result)
			},
		},
		{
			name: "database error",
			contextRequest: &mcp.Context{
				AgentID: "agent-error",
				ModelID: "model-error",
			},
			setupMocks: func(mockDB *MockDB, mockCache *mocks.MockCache) {
				mockDB.On("CreateContext", ctx, mock.AnythingOfType("*mcp.Context")).Return(assert.AnError)
			},
			expectedError: true,
			errorMessage:  "failed to create context",
			validateResult: func(t *testing.T, result *mcp.Context) {
				assert.Nil(t, result)
			},
		},
	}
	
	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			mockDB := new(MockDB)
			mockCache := new(mocks.MockCache)
			mockDB.On("GetDB").Return(&sqlx.DB{})
			
			// Setup specific mocks for this test case
			tc.setupMocks(mockDB, mockCache)
			
			// Create context manager
			cm := &ContextManager{
				db:          mockDB,
				cache:       mockCache,
				subscribers: make(map[string][]func(mcp.Event)),
			}
			
			// Call the method being tested
			result, err := cm.CreateContext(ctx, tc.contextRequest)
			
			// Verify error expectations
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorMessage != "" {
					assert.Contains(t, err.Error(), tc.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
			
			// Validate the result
			tc.validateResult(t, result)
			
			// Verify that all expected mock calls were made
			mockDB.AssertExpectations(t)
			mockCache.AssertExpectations(t)
		})
	}
}

func TestGetContext(t *testing.T) {
	// Create test context
	ctx := context.Background()
	testTime := time.Now()
	
	// Define test cases
	testCases := []struct {
		name           string
		contextID      string
		setupMocks     func(mockDB *MockDatabase, mockCache *mocks.MockCache)
		expectedError  bool
		errorMessage   string
		validateResult func(t *testing.T, result *mcp.Context)
	}{
		{
			name:      "cache hit",
			contextID: "context-123",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				expectedContext := &mcp.Context{
					ID:        "context-123",
					AgentID:   "agent-123",
					ModelID:   "model-123",
					CreatedAt: testTime,
					UpdatedAt: testTime,
				}
				
				mockCache.On("Get", mock.Anything, "context:context-123", mock.AnythingOfType("*mcp.Context")).
					Run(func(args mock.Arguments) {
						arg := args.Get(2).(*mcp.Context)
						*arg = *expectedContext
					}).
					Return(nil)
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *mcp.Context) {
				assert.NotNil(t, result)
				assert.Equal(t, "context-123", result.ID)
				assert.Equal(t, "agent-123", result.AgentID)
				assert.Equal(t, "model-123", result.ModelID)
			},
		},
		{
			name:      "cache miss with database hit",
			contextID: "context-456",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				dbContext := &mcp.Context{
					ID:        "context-456",
					AgentID:   "agent-456",
					ModelID:   "model-456",
					CreatedAt: testTime,
					UpdatedAt: testTime,
				}
				
				mockCache.On("Get", mock.Anything, "context:context-456", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "context-456").Return(dbContext, nil)
				mockCache.On("Set", mock.Anything, "context:context-456", dbContext, mock.Anything).Return(nil)
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *mcp.Context) {
				assert.NotNil(t, result)
				assert.Equal(t, "context-456", result.ID)
				assert.Equal(t, "agent-456", result.AgentID)
				assert.Equal(t, "model-456", result.ModelID)
			},
		},
		{
			name:      "context not found",
			contextID: "not-found",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
			},
			expectedError: true,
			validateResult: func(t *testing.T, result *mcp.Context) {
				assert.Nil(t, result)
			},
		},
	}
	
	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			mockDB := new(MockDatabase)
			mockCache := new(mocks.MockCache)
			mockDB.On("GetDB").Return(&sqlx.DB{})
			
			// Setup specific mocks for this test case
			tc.setupMocks(mockDB, mockCache)
			
			// Create context manager
			cm := &ContextManager{
				db:          mockDB,
				cache:       mockCache,
				subscribers: make(map[string][]func(mcp.Event)),
			}
			
			// Call the method being tested
			result, err := cm.GetContext(ctx, tc.contextID)
			
			// Verify error expectations
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorMessage != "" {
					assert.Contains(t, err.Error(), tc.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
			
			// Validate the result
			tc.validateResult(t, result)
			
			// Verify that all expected mock calls were made
			mockDB.AssertExpectations(t)
			mockCache.AssertExpectations(t)
		})
	}
}

func TestUpdateContext(t *testing.T) {
	mockDB := new(MockDatabase)
	mockCache := new(mocks.MockCache)
	
	// Add necessary mock DB functions
	mockDB.On("GetDB").Return(&sqlx.DB{})
	
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
	
	// Add necessary mock DB functions
	mockDB.On("GetDB").Return(&sqlx.DB{})
	
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
	
	// Add necessary mock DB functions
	mockDB.On("GetDB").Return(&sqlx.DB{})
	
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
	// Create test context
	ctx := context.Background()
	testTime := time.Now()
	
	// Define test cases
	testCases := []struct {
		name           string
		contextID      string
		setupMocks     func(mockDB *MockDatabase, mockCache *mocks.MockCache)
		expectedError  bool
		errorMessage   string
		expectedResult string
	}{
		{
			name:      "valid context with messages",
			contextID: "context-123",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				existingContext := &mcp.Context{
					ID:            "context-123",
					AgentID:       "agent-123",
					ModelID:       "model-123",
					CurrentTokens: 100,
					Content: []mcp.ContextItem{
						{
							ID:        "item-1",
							Role:      "user",
							Content:   "Hello",
							Tokens:    1,
							Timestamp: testTime,
						},
						{
							ID:        "item-2",
							Role:      "assistant",
							Content:   "Hi there",
							Tokens:    2,
							Timestamp: testTime,
						},
					},
				}
				
				mockCache.On("Get", mock.Anything, "context:context-123", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "context-123").Return(existingContext, nil)
				mockCache.On("Set", mock.Anything, "context:context-123", existingContext, mock.Anything).Return(nil)
			},
			expectedError:  false,
			expectedResult: "2 messages",
		},
		{
			name:      "context with no messages",
			contextID: "empty-context",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				emptyContext := &mcp.Context{
					ID:            "empty-context",
					AgentID:       "agent-123",
					ModelID:       "model-123",
					CurrentTokens: 0,
					Content:       []mcp.ContextItem{},
				}
				
				mockCache.On("Get", mock.Anything, "context:empty-context", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "empty-context").Return(emptyContext, nil)
				mockCache.On("Set", mock.Anything, "context:empty-context", emptyContext, mock.Anything).Return(nil)
			},
			expectedError:  false,
			expectedResult: "0 messages",
		},
		{
			name:      "context not found",
			contextID: "not-found",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
			},
			expectedError: true,
			errorMessage:  "failed to get context",
			expectedResult: "",
		},
	}
	
	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			mockDB := new(MockDatabase)
			mockCache := new(mocks.MockCache)
			mockDB.On("GetDB").Return(&sqlx.DB{})
			
			// Setup specific mocks for this test case
			tc.setupMocks(mockDB, mockCache)
			
			// Create context manager
			cm := &ContextManager{
				db:          mockDB,
				cache:       mockCache,
				subscribers: make(map[string][]func(mcp.Event)),
			}
			
			// Call the method being tested
			summary, err := cm.SummarizeContext(ctx, tc.contextID)
			
			// Verify error expectations
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorMessage != "" {
					assert.Contains(t, err.Error(), tc.errorMessage)
				}
				assert.Empty(t, summary)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, summary, tc.expectedResult)
			}
			
			// Verify that all expected mock calls were made
			mockDB.AssertExpectations(t)
			mockCache.AssertExpectations(t)
		})
	}
}

func TestSearchInContext(t *testing.T) {
	// Create test context
	ctx := context.Background()
	testTime := time.Now()
	
	// Define test context with messages
	existingContext := &mcp.Context{
		ID:      "context-123",
		AgentID: "agent-123",
		ModelID: "model-123",
		Content: []mcp.ContextItem{
			{
				Role:      "user",
				Content:   "Hello world",
				Tokens:    2,
				Timestamp: testTime,
			},
			{
				Role:      "assistant",
				Content:   "Hi there, how can I help you?",
				Tokens:    7,
				Timestamp: testTime,
			},
			{
				Role:      "user",
				Content:   "I need information about the world",
				Tokens:    7,
				Timestamp: testTime,
			},
		},
	}
	
	// Define test cases
	testCases := []struct {
		name             string
		contextID        string
		searchQuery      string
		setupMocks       func(mockDB *MockDatabase, mockCache *mocks.MockCache)
		expectedError    bool
		errorMessage     string
		expectedResults  int
		validateResults  func(t *testing.T, results []mcp.ContextItem)
	}{
		{
			name:        "search with multiple results",
			contextID:   "context-123",
			searchQuery: "world",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				mockCache.On("Get", mock.Anything, "context:context-123", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "context-123").Return(existingContext, nil)
				mockCache.On("Set", mock.Anything, "context:context-123", existingContext, mock.Anything).Return(nil)
			},
			expectedError:   false,
			expectedResults: 2,
			validateResults: func(t *testing.T, results []mcp.ContextItem) {
				assert.Equal(t, "Hello world", results[0].Content)
				assert.Equal(t, "I need information about the world", results[1].Content)
			},
		},
		{
			name:        "search with no results",
			contextID:   "context-123",
			searchQuery: "nonexistent",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				mockCache.On("Get", mock.Anything, "context:context-123", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "context-123").Return(existingContext, nil)
				mockCache.On("Set", mock.Anything, "context:context-123", existingContext, mock.Anything).Return(nil)
			},
			expectedError:   false,
			expectedResults: 0,
			validateResults: func(t *testing.T, results []mcp.ContextItem) {
				// No results to validate
			},
		},
		{
			name:        "context not found",
			contextID:   "not-found",
			searchQuery: "world",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				mockCache.On("Get", mock.Anything, "context:not-found", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "not-found").Return(nil, assert.AnError)
			},
			expectedError:   true,
			errorMessage:    "failed to get context",
			expectedResults: 0,
			validateResults: func(t *testing.T, results []mcp.ContextItem) {
				assert.Nil(t, results)
			},
		},
		{
			name:        "empty search query",
			contextID:   "context-123",
			searchQuery: "",
			setupMocks: func(mockDB *MockDatabase, mockCache *mocks.MockCache) {
				mockCache.On("Get", mock.Anything, "context:context-123", mock.AnythingOfType("*mcp.Context")).
					Return(assert.AnError)
				mockDB.On("GetContext", ctx, "context-123").Return(existingContext, nil)
				mockCache.On("Set", mock.Anything, "context:context-123", existingContext, mock.Anything).Return(nil)
			},
			expectedError:   false,
			expectedResults: 0,
			validateResults: func(t *testing.T, results []mcp.ContextItem) {
				assert.Empty(t, results)
			},
		},
	}
	
	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			mockDB := new(MockDatabase)
			mockCache := new(mocks.MockCache)
			mockDB.On("GetDB").Return(&sqlx.DB{})
			
			// Setup specific mocks for this test case
			tc.setupMocks(mockDB, mockCache)
			
			// Create context manager
			cm := &ContextManager{
				db:          mockDB,
				cache:       mockCache,
				subscribers: make(map[string][]func(mcp.Event)),
			}
			
			// Call the method being tested
			results, err := cm.SearchInContext(ctx, tc.contextID, tc.searchQuery)
			
			// Verify error expectations
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorMessage != "" {
					assert.Contains(t, err.Error(), tc.errorMessage)
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tc.expectedResults)
			}
			
			// Validate the results
			tc.validateResults(t, results)
			
			// Verify that all expected mock calls were made
			mockDB.AssertExpectations(t)
			mockCache.AssertExpectations(t)
		})
	}
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
	// Define reference time
	now := time.Now()
	
	// Define test cases
	testCases := []struct {
		name             string
		truncateStrategy string
		context          *mcp.Context
		expectedError    bool
		validateResult   func(t *testing.T, context *mcp.Context)
	}{
		{
			name:             "truncate oldest first",
			truncateStrategy: string(TruncateOldestFirst),
			context: &mcp.Context{
				MaxTokens:     10,
				CurrentTokens: 15,
				Content: []mcp.ContextItem{
					{
						Role:      "user",
						Content:   "First message",
						Tokens:    5,
						Timestamp: now.Add(-2 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Second message",
						Tokens:    5,
						Timestamp: now.Add(-1 * time.Hour),
					},
					{
						Role:      "user",
						Content:   "Third message",
						Tokens:    5,
						Timestamp: now,
					},
				},
			},
			expectedError: false,
			validateResult: func(t *testing.T, context *mcp.Context) {
				assert.Len(t, context.Content, 2)
				assert.Equal(t, "Second message", context.Content[0].Content)
				assert.Equal(t, "Third message", context.Content[1].Content)
				assert.Equal(t, 10, context.CurrentTokens)
			},
		},
		{
			name:             "truncate preserving user",
			truncateStrategy: string(TruncatePreservingUser),
			context: &mcp.Context{
				MaxTokens:     15,
				CurrentTokens: 25,
				Content: []mcp.ContextItem{
					{
						Role:      "system",
						Content:   "System message",
						Tokens:    5,
						Timestamp: now.Add(-3 * time.Hour),
					},
					{
						Role:      "user",
						Content:   "User message 1",
						Tokens:    5,
						Timestamp: now.Add(-2 * time.Hour),
					},
					{
						Role:      "assistant",
						Content:   "Assistant message 1",
						Tokens:    5,
						Timestamp: now.Add(-1 * time.Hour),
					},
					{
						Role:      "user",
						Content:   "User message 2",
						Tokens:    5,
						Timestamp: now.Add(-30 * time.Minute),
					},
					{
						Role:      "assistant",
						Content:   "Assistant message 2",
						Tokens:    5,
						Timestamp: now,
					},
				},
			},
			expectedError: false,
			validateResult: func(t *testing.T, context *mcp.Context) {
				assert.LessOrEqual(t, context.CurrentTokens, context.MaxTokens)
			},
		},
		{
			name:             "truncate with no content",
			truncateStrategy: string(TruncateOldestFirst),
			context: &mcp.Context{
				MaxTokens:     10,
				CurrentTokens: 0,
				Content:       []mcp.ContextItem{},
			},
			expectedError: false,
			validateResult: func(t *testing.T, context *mcp.Context) {
				assert.Len(t, context.Content, 0)
				assert.Equal(t, 0, context.CurrentTokens)
			},
		},
		{
			name:             "truncate with content under max tokens",
			truncateStrategy: string(TruncateOldestFirst),
			context: &mcp.Context{
				MaxTokens:     20,
				CurrentTokens: 10,
				Content: []mcp.ContextItem{
					{
						Role:      "user",
						Content:   "Message under limit",
						Tokens:    10,
						Timestamp: now,
					},
				},
			},
			expectedError: false,
			validateResult: func(t *testing.T, context *mcp.Context) {
				assert.Len(t, context.Content, 1)
				assert.Equal(t, 10, context.CurrentTokens)
			},
		},
		{
			name:             "truncate with invalid strategy",
			truncateStrategy: "invalid-strategy",
			context: &mcp.Context{
				MaxTokens:     10,
				CurrentTokens: 15,
				Content: []mcp.ContextItem{
					{
						Role:      "user",
						Content:   "Test message",
						Tokens:    15,
						Timestamp: now,
					},
				},
			},
			expectedError: true,
			validateResult: func(t *testing.T, context *mcp.Context) {
				// Context should remain unchanged
				assert.Len(t, context.Content, 1)
				assert.Equal(t, 15, context.CurrentTokens)
			},
		},
	}
	
	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a copy of the context to avoid modifying the original
			contextData := &mcp.Context{
				MaxTokens:     tc.context.MaxTokens,
				CurrentTokens: tc.context.CurrentTokens,
				Content:       make([]mcp.ContextItem, len(tc.context.Content)),
			}
			
			// Copy content items
			for i, item := range tc.context.Content {
				contextData.Content[i] = item
			}
			
			// Create context manager
			cm := &ContextManager{}
			
			// Call the method being tested
			var err error
			switch tc.truncateStrategy {
			case string(TruncateOldestFirst):
				err = cm.truncateOldestFirst(contextData)
			case string(TruncatePreservingUser):
				err = cm.truncatePreservingUser(contextData)
			default:
				// For invalid strategy, call truncateContext directly
				err = cm.truncateContext(contextData, TruncateStrategy(tc.truncateStrategy))
			}
			
			// Verify error expectations
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			// Validate the results
			tc.validateResult(t, contextData)
		})
	}
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
// FuzzTruncateOldestFirst is a property-based test that verifies the truncateOldestFirst
// function behaves correctly with various inputs
func FuzzTruncateOldestFirst(f *testing.F) {
	// Add seed corpus
	f.Add(10, 15, 3)  // maxTokens, currentTokens, numItems
	f.Add(100, 100, 5)  // maxTokens, currentTokens, numItems - exact match
	f.Add(10, 30, 10)  // maxTokens, currentTokens, numItems - large truncation needed
	f.Add(100, 0, 0)  // maxTokens, currentTokens, numItems - empty context
	
	// Define the fuzzing function
	f.Fuzz(func(t *testing.T, maxTokens, currentTokens, numItems int) {
		// Ensure sane ranges for input values
		maxTokens = maxTokens % 1000  // Limit max tokens to 0-999
		if maxTokens < 0 {
			maxTokens = -maxTokens
		}
		
		currentTokens = currentTokens % 2000  // Limit current tokens to 0-1999
		if currentTokens < 0 {
			currentTokens = -currentTokens
		}
		
		numItems = numItems % 50  // Limit number of items to 0-49
		if numItems < 0 {
			numItems = -numItems
		}
		
		// Create test context with random items
		contextData := &mcp.Context{
			MaxTokens:     maxTokens,
			CurrentTokens: currentTokens,
			Content:       make([]mcp.ContextItem, numItems),
		}
		
		// If we have items, populate them with random content
		tokensPerItem := 1
		if numItems > 0 && currentTokens > 0 {
			tokensPerItem = currentTokens / numItems
			if tokensPerItem < 1 {
				tokensPerItem = 1
			}
		}
		
		now := time.Now()
		for i := 0; i < numItems; i++ {
			contextData.Content[i] = mcp.ContextItem{
				ID:        fmt.Sprintf("item-%d", i),
				Role:      "user",
				Content:   fmt.Sprintf("Message %d", i),
				Tokens:    tokensPerItem,
				Timestamp: now.Add(time.Duration(i) * time.Minute),
			}
		}
		
		// Create context manager
		cm := &ContextManager{}
		
		// Call the method being tested
		err := cm.truncateOldestFirst(contextData)
		
		// Verify invariants
		assert.NoError(t, err)
		
		// Check that current tokens <= max tokens after truncation
		assert.LessOrEqual(t, contextData.CurrentTokens, contextData.MaxTokens)
		
		// Check that content items are consistent with current tokens
		actualTokens := 0
		for _, item := range contextData.Content {
			actualTokens += item.Tokens
		}
		assert.Equal(t, actualTokens, contextData.CurrentTokens)
		
		// Check that items are kept in chronological order (newer items are kept)
		for i := 1; i < len(contextData.Content); i++ {
			assert.True(t, !contextData.Content[i].Timestamp.Before(contextData.Content[i-1].Timestamp),
				"Items should be in chronological order")
		}
	})
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
