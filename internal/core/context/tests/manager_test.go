package tests

import (
	"context"
	"testing"
	"time"

	contextManager "github.com/S-Corkum/mcp-server/internal/core/context"
	"github.com/S-Corkum/mcp-server/internal/database/mocks"
	storageMocks "github.com/S-Corkum/mcp-server/internal/storage/providers/mocks"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupMockDependencies creates mocked dependencies for testing
func setupMockDependencies(t *testing.T) (*mocks.MockDatabase, *mocks.MockCache, *storageMocks.MockContextStorage) {
	mockDB := new(mocks.MockDatabase)
	mockCache := new(mocks.MockCache)
	mockStorage := new(storageMocks.MockContextStorage)
	
	return mockDB, mockCache, mockStorage
}

// TestCreateContext tests the CreateContext method
func TestCreateContext(t *testing.T) {
	// Create mock dependencies
	mockDB, mockCache, mockStorage := setupMockDependencies(t)
	
	// Create test context
	testContext := &mcp.Context{
		AgentID:  "test-agent",
		ModelID:  "test-model",
		MaxTokens: 1000,
	}
	
	// Set up expectations for database transaction and creating context
	mockDB.On("Transaction", mock.Anything, mock.AnythingOfType("func(*database.Tx) error")).
		Return(nil).
		Run(func(args mock.Arguments) {
			// Call the transaction function with nil to simulate success
			fn := args.Get(1).(func(*database.Tx) error)
			fn(nil)
		})
	
	// Set up expectations for cache
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).
		Return(nil)
	
	// Set up expectations for storage
	mockStorage.On("StoreContext", mock.Anything, mock.Anything).
		Return(nil)
	
	// Create context manager with mocks
	manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
	
	// Call CreateContext
	result, err := manager.CreateContext(context.Background(), testContext)
	
	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, testContext.AgentID, result.AgentID)
	assert.Equal(t, testContext.ModelID, result.ModelID)
	assert.NotEmpty(t, result.ID)
	assert.False(t, result.CreatedAt.IsZero())
	assert.False(t, result.UpdatedAt.IsZero())
	
	// Verify expectations
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

// TestGetContext tests the GetContext method
func TestGetContext(t *testing.T) {
	// Create mock dependencies
	mockDB, mockCache, mockStorage := setupMockDependencies(t)
	
	// Create test context ID
	testContextID := "test-context-123"
	
	// Create test context
	testContext := &mcp.Context{
		ID:       testContextID,
		AgentID:  "test-agent",
		ModelID:  "test-model",
		MaxTokens: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Test scenarios
	t.Run("get from cache", func(t *testing.T) {
		// Set up expectations for cache hit
		mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
			Run(func(args mock.Arguments) {
				// Set the output parameter (args[2]) to testContext
				outPtr := args.Get(2).(*mcp.Context)
				*outPtr = *testContext
			}).
			Return(nil)
		
		// Create context manager with mocks
		manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
		
		// Call GetContext
		result, err := manager.GetContext(context.Background(), testContextID)
		
		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testContextID, result.ID)
		assert.Equal(t, testContext.AgentID, result.AgentID)
		assert.Equal(t, testContext.ModelID, result.ModelID)
		
		// Verify expectations
		mockCache.AssertExpectations(t)
	})
	
	t.Run("get from database", func(t *testing.T) {
		// Reset mocks
		mockDB = new(mocks.MockDatabase)
		mockCache = new(mocks.MockCache)
		mockStorage = new(storageMocks.MockContextStorage)
		
		// Set up expectations for cache miss
		mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
			Return(assert.AnError)
		
		// Set up expectations for database transaction and getting context
		mockDB.On("Transaction", mock.Anything, mock.AnythingOfType("func(*database.Tx) error")).
			Run(func(args mock.Arguments) {
				// Call the transaction function with nil to simulate success
				fn := args.Get(1).(func(*database.Tx) error)
				fn(nil)
			}).
			Return(nil)
		
		// Set up expectations for database get
		mockDB.On("getContext", mock.Anything, mock.Anything, testContextID).
			Return(testContext, nil)
		
		// Set up expectations for caching the result
		mockCache.On("Set", mock.Anything, "context:"+testContextID, mock.Anything, mock.Anything).
			Return(nil)
		
		// Create context manager with mocks
		manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
		
		// Call GetContext
		result, err := manager.GetContext(context.Background(), testContextID)
		
		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testContextID, result.ID)
		
		// Verify expectations
		mockDB.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})
	
	t.Run("get from storage", func(t *testing.T) {
		// Reset mocks
		mockDB = new(mocks.MockDatabase)
		mockCache = new(mocks.MockCache)
		mockStorage = new(storageMocks.MockContextStorage)
		
		// Set up expectations for cache miss
		mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
			Return(assert.AnError)
		
		// Set up expectations for database miss
		mockDB.On("Transaction", mock.Anything, mock.AnythingOfType("func(*database.Tx) error")).
			Run(func(args mock.Arguments) {
				// Call the transaction function
				fn := args.Get(1).(func(*database.Tx) error)
				fn(nil)
			}).
			Return(assert.AnError)
		
		// Set up expectations for storage get
		mockStorage.On("GetContext", mock.Anything, testContextID).
			Return(testContext, nil)
		
		// Set up expectations for caching the result
		mockCache.On("Set", mock.Anything, "context:"+testContextID, mock.Anything, mock.Anything).
			Return(nil)
		
		// Create context manager with mocks
		manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
		
		// Call GetContext
		result, err := manager.GetContext(context.Background(), testContextID)
		
		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testContextID, result.ID)
		
		// Verify expectations
		mockStorage.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})
}

// TestUpdateContext tests the UpdateContext method
func TestUpdateContext(t *testing.T) {
	// Create mock dependencies
	mockDB, mockCache, mockStorage := setupMockDependencies(t)
	
	// Create test context ID
	testContextID := "test-context-123"
	
	// Create existing context
	existingContext := &mcp.Context{
		ID:       testContextID,
		AgentID:  "test-agent",
		ModelID:  "test-model",
		MaxTokens: 1000,
		CurrentTokens: 0,
		Content:  []mcp.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Create update data
	updateData := &mcp.Context{
		Content: []mcp.ContextItem{
			{
				Role:    "user",
				Content: "Hello, how are you?",
				Tokens:  5,
			},
		},
	}
	
	// Set up expectations for cache hit
	mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
		Run(func(args mock.Arguments) {
			// Set the output parameter (args[2]) to existingContext
			outPtr := args.Get(2).(*mcp.Context)
			*outPtr = *existingContext
		}).
		Return(nil)
	
	// Set up expectations for database transaction and updating context
	mockDB.On("Transaction", mock.Anything, mock.AnythingOfType("func(*database.Tx) error")).
		Return(nil).
		Run(func(args mock.Arguments) {
			// Call the transaction function with nil to simulate success
			fn := args.Get(1).(func(*database.Tx) error)
			fn(nil)
		})
	
	// Set up expectations for storage
	mockStorage.On("StoreContext", mock.Anything, mock.Anything).
		Return(nil)
	
	// Set up expectations for cache update
	mockCache.On("Set", mock.Anything, "context:"+testContextID, mock.Anything, mock.Anything).
		Return(nil)
	
	// Create context manager with mocks
	manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
	
	// Call UpdateContext
	result, err := manager.UpdateContext(context.Background(), testContextID, updateData, nil)
	
	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, testContextID, result.ID)
	assert.Equal(t, 1, len(result.Content))
	assert.Equal(t, 5, result.CurrentTokens)
	
	// Verify expectations
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

// TestDeleteContext tests the DeleteContext method
func TestDeleteContext(t *testing.T) {
	// Create mock dependencies
	mockDB, mockCache, mockStorage := setupMockDependencies(t)
	
	// Create test context ID
	testContextID := "test-context-123"
	
	// Create existing context
	existingContext := &mcp.Context{
		ID:       testContextID,
		AgentID:  "test-agent",
		ModelID:  "test-model",
		MaxTokens: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Set up expectations for cache hit
	mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
		Run(func(args mock.Arguments) {
			// Set the output parameter (args[2]) to existingContext
			outPtr := args.Get(2).(*mcp.Context)
			*outPtr = *existingContext
		}).
		Return(nil)
	
	// Set up expectations for database transaction and deleting context
	mockDB.On("Transaction", mock.Anything, mock.AnythingOfType("func(*database.Tx) error")).
		Return(nil).
		Run(func(args mock.Arguments) {
			// Call the transaction function with nil to simulate success
			fn := args.Get(1).(func(*database.Tx) error)
			fn(nil)
		})
	
	// Set up expectations for storage
	mockStorage.On("DeleteContext", mock.Anything, testContextID).
		Return(nil)
	
	// Set up expectations for cache delete
	mockCache.On("Delete", mock.Anything, "context:"+testContextID).
		Return(nil)
	
	// Create context manager with mocks
	manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
	
	// Call DeleteContext
	err := manager.DeleteContext(context.Background(), testContextID)
	
	// Assertions
	assert.NoError(t, err)
	
	// Verify expectations
	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

// TestListContexts tests the ListContexts method
func TestListContexts(t *testing.T) {
	// Create mock dependencies
	mockDB, mockCache, mockStorage := setupMockDependencies(t)
	
	// Create test agent ID
	testAgentID := "test-agent"
	
	// Create test session ID
	testSessionID := "test-session"
	
	// Create test contexts
	testContexts := []*mcp.Context{
		{
			ID:       "context-1",
			AgentID:  testAgentID,
			SessionID: testSessionID,
			ModelID:  "test-model",
			MaxTokens: 1000,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:       "context-2",
			AgentID:  testAgentID,
			SessionID: testSessionID,
			ModelID:  "test-model",
			MaxTokens: 1000,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	
	// Set up expectations for database transaction and listing contexts
	mockDB.On("Transaction", mock.Anything, mock.AnythingOfType("func(*database.Tx) error")).
		Return(nil).
		Run(func(args mock.Arguments) {
			// Call the transaction function with nil to simulate success
			fn := args.Get(1).(func(*database.Tx) error)
			fn(nil)
		})
	
	// Set up expectations for database list
	mockDB.On("listContexts", mock.Anything, mock.Anything, testAgentID, testSessionID, mock.Anything).
		Return(testContexts, nil)
	
	// Create context manager with mocks
	manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
	
	// Call ListContexts
	result, err := manager.ListContexts(context.Background(), testAgentID, testSessionID, nil)
	
	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "context-1", result[0].ID)
	assert.Equal(t, "context-2", result[1].ID)
	
	// Verify expectations
	mockDB.AssertExpectations(t)
}

// TestSummarizeContext tests the SummarizeContext method
func TestSummarizeContext(t *testing.T) {
	// Create mock dependencies
	mockDB, mockCache, mockStorage := setupMockDependencies(t)
	
	// Create test context ID
	testContextID := "test-context-123"
	
	// Create existing context with items
	existingContext := &mcp.Context{
		ID:       testContextID,
		AgentID:  "test-agent",
		ModelID:  "test-model",
		MaxTokens: 1000,
		CurrentTokens: 20,
		Content: []mcp.ContextItem{
			{
				Role:    "user",
				Content: "Hello, how are you?",
				Tokens:  5,
			},
			{
				Role:    "assistant",
				Content: "I'm doing well, thank you! How can I help you today?",
				Tokens:  10,
			},
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
				Tokens:  5,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Set up expectations for cache hit
	mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
		Run(func(args mock.Arguments) {
			// Set the output parameter (args[2]) to existingContext
			outPtr := args.Get(2).(*mcp.Context)
			*outPtr = *existingContext
		}).
		Return(nil)
	
	// Create context manager with mocks
	manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
	
	// Call SummarizeContext
	result, err := manager.SummarizeContext(context.Background(), testContextID)
	
	// Assertions
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "3 messages")
	assert.Contains(t, result, "1 user")
	assert.Contains(t, result, "1 assistant")
	assert.Contains(t, result, "1 system")
	assert.Contains(t, result, "20/1000 tokens")
	
	// Verify expectations
	mockCache.AssertExpectations(t)
}

// TestSearchInContext tests the SearchInContext method
func TestSearchInContext(t *testing.T) {
	// Create mock dependencies
	mockDB, mockCache, mockStorage := setupMockDependencies(t)
	
	// Create test context ID
	testContextID := "test-context-123"
	
	// Create existing context with items
	existingContext := &mcp.Context{
		ID:       testContextID,
		AgentID:  "test-agent",
		ModelID:  "test-model",
		MaxTokens: 1000,
		CurrentTokens: 20,
		Content: []mcp.ContextItem{
			{
				Role:    "user",
				Content: "Hello, how are you?",
				Tokens:  5,
			},
			{
				Role:    "assistant",
				Content: "I'm doing well, thank you! How can I help you today?",
				Tokens:  10,
			},
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
				Tokens:  5,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Set up expectations for cache hit
	mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
		Run(func(args mock.Arguments) {
			// Set the output parameter (args[2]) to existingContext
			outPtr := args.Get(2).(*mcp.Context)
			*outPtr = *existingContext
		}).
		Return(nil)
	
	// Create context manager with mocks
	manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
	
	// Test cases
	testCases := []struct {
		name     string
		query    string
		expected int
	}{
		{
			name:     "match all",
			query:    "how",
			expected: 2, // Matches both user and assistant messages
		},
		{
			name:     "match one",
			query:    "today",
			expected: 1, // Only matches assistant message
		},
		{
			name:     "match none",
			query:    "goodbye",
			expected: 0,
		},
		{
			name:     "empty query",
			query:    "",
			expected: 0,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call SearchInContext
			result, err := manager.SearchInContext(context.Background(), testContextID, tc.query)
			
			// Assertions
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, len(result))
		})
	}
	
	// Verify expectations (only once since mockCache.On is called once)
	mockCache.AssertExpectations(t)
}

// TestTruncateContext tests context truncation strategies
func TestTruncateContext(t *testing.T) {
	// Create mock dependencies
	mockDB, mockCache, mockStorage := setupMockDependencies(t)
	
	// Create test context ID
	testContextID := "test-context-123"
	
	// Test cases for truncation
	t.Run("oldest first", func(t *testing.T) {
		// Create a context that exceeds max tokens
		existingContext := &mcp.Context{
			ID:       testContextID,
			AgentID:  "test-agent",
			ModelID:  "test-model",
			MaxTokens: 15,
			CurrentTokens: 30,
			Content: []mcp.ContextItem{
				{
					Role:      "user",
					Content:   "First message",
					Tokens:    5,
					Timestamp: time.Now().Add(-3 * time.Hour),
				},
				{
					Role:      "assistant",
					Content:   "Second message",
					Tokens:    10,
					Timestamp: time.Now().Add(-2 * time.Hour),
				},
				{
					Role:      "user",
					Content:   "Third message",
					Tokens:    5,
					Timestamp: time.Now().Add(-1 * time.Hour),
				},
				{
					Role:      "assistant",
					Content:   "Fourth message",
					Tokens:    10,
					Timestamp: time.Now(),
				},
			},
		}
		
		// Set up expectations for cache hit
		mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
			Run(func(args mock.Arguments) {
				// Set the output parameter (args[2]) to existingContext
				outPtr := args.Get(2).(*mcp.Context)
				*outPtr = *existingContext
			}).
			Return(nil).Once()
		
		// Set up expectations for database transaction and updating context
		mockDB.On("Transaction", mock.Anything, mock.AnythingOfType("func(*database.Tx) error")).
			Return(nil).
			Run(func(args mock.Arguments) {
				// Call the transaction function with nil to simulate success
				fn := args.Get(1).(func(*database.Tx) error)
				fn(nil)
			}).Once()
		
		// Set up expectations for storage
		mockStorage.On("StoreContext", mock.Anything, mock.Anything).
			Return(nil).Once()
		
		// Set up expectations for cache update
		mockCache.On("Set", mock.Anything, "context:"+testContextID, mock.Anything, mock.Anything).
			Return(nil).Once()
		
		// Create context manager with mocks
		manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
		
		// Update options with truncation
		updateData := &mcp.Context{}
		options := &mcp.ContextUpdateOptions{
			Truncate:         true,
			TruncateStrategy: "oldest_first",
		}
		
		// Call UpdateContext (which will perform truncation)
		result, err := manager.UpdateContext(context.Background(), testContextID, updateData, options)
		
		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, len(result.Content)) // Should have removed the oldest 2 items
		assert.LessOrEqual(t, result.CurrentTokens, result.MaxTokens)
		assert.Equal(t, "Third message", result.Content[0].Content)
		assert.Equal(t, "Fourth message", result.Content[1].Content)
	})
	
	// Reset mocks
	mockDB = new(mocks.MockDatabase)
	mockCache = new(mocks.MockCache)
	mockStorage = new(storageMocks.MockContextStorage)
	
	t.Run("preserving user", func(t *testing.T) {
		// Create a context that exceeds max tokens
		existingContext := &mcp.Context{
			ID:       testContextID,
			AgentID:  "test-agent",
			ModelID:  "test-model",
			MaxTokens: 15,
			CurrentTokens: 30,
			Content: []mcp.ContextItem{
				{
					Role:      "user",
					Content:   "First user message",
					Tokens:    5,
					Timestamp: time.Now().Add(-3 * time.Hour),
				},
				{
					Role:      "assistant",
					Content:   "First assistant message",
					Tokens:    10,
					Timestamp: time.Now().Add(-2 * time.Hour),
				},
				{
					Role:      "user",
					Content:   "Second user message",
					Tokens:    5,
					Timestamp: time.Now().Add(-1 * time.Hour),
				},
				{
					Role:      "assistant",
					Content:   "Second assistant message",
					Tokens:    10,
					Timestamp: time.Now(),
				},
			},
		}
		
		// Set up expectations for cache hit
		mockCache.On("Get", mock.Anything, "context:"+testContextID, mock.Anything).
			Run(func(args mock.Arguments) {
				// Set the output parameter (args[2]) to existingContext
				outPtr := args.Get(2).(*mcp.Context)
				*outPtr = *existingContext
			}).
			Return(nil).Once()
		
		// Set up expectations for database transaction and updating context
		mockDB.On("Transaction", mock.Anything, mock.AnythingOfType("func(*database.Tx) error")).
			Return(nil).
			Run(func(args mock.Arguments) {
				// Call the transaction function with nil to simulate success
				fn := args.Get(1).(func(*database.Tx) error)
				fn(nil)
			}).Once()
		
		// Set up expectations for storage
		mockStorage.On("StoreContext", mock.Anything, mock.Anything).
			Return(nil).Once()
		
		// Set up expectations for cache update
		mockCache.On("Set", mock.Anything, "context:"+testContextID, mock.Anything, mock.Anything).
			Return(nil).Once()
		
		// Create context manager with mocks
		manager := contextManager.NewManager(mockDB, mockCache, mockStorage, nil, nil, nil)
		
		// Update options with truncation
		updateData := &mcp.Context{}
		options := &mcp.ContextUpdateOptions{
			Truncate:         true,
			TruncateStrategy: "preserving_user",
		}
		
		// Call UpdateContext (which will perform truncation)
		result, err := manager.UpdateContext(context.Background(), testContextID, updateData, options)
		
		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 3, len(result.Content)) // Should have removed the oldest assistant message
		assert.LessOrEqual(t, result.CurrentTokens, result.MaxTokens)
		
		// Check that user messages were preserved
		var userMessages int
		for _, item := range result.Content {
			if item.Role == "user" {
				userMessages++
			}
		}
		assert.Equal(t, 2, userMessages) // Both user messages should be preserved
	})
}
