package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/cache/mocks"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// copyContext creates a deep copy of an mcp.Context for test isolation
func copyContext(src *mcp.Context) *mcp.Context {
	if src == nil {
		return nil
	}
	copyItems := make([]mcp.ContextItem, len(src.Content))
	for i, item := range src.Content {
		copyItems[i] = item // Struct copy is deep for value types
	}
	var metadataCopy map[string]interface{}
	if src.Metadata != nil {
		metadataCopy = make(map[string]interface{}, len(src.Metadata))
		for k, v := range src.Metadata {
			metadataCopy[k] = v
		}
	}
	return &mcp.Context{
		ID:            src.ID,
		AgentID:       src.AgentID,
		ModelID:       src.ModelID,
		Content:       copyItems,
		CreatedAt:     src.CreatedAt,
		UpdatedAt:     src.UpdatedAt,
		CurrentTokens: src.CurrentTokens,
		Metadata:      metadataCopy,
	}
}

func TestContextManager_UpdateContext_ReplaceContent(t *testing.T) {
	// Setup mock database and cache
	mockDB := new(MockDB)
	mockCache := new(mocks.MockCache)

	cm := NewContextManager(mockDB, mockCache)

	contextID := "test-context-id"
	initialContext := &mcp.Context{
		ID:      contextID,
		AgentID: "test-agent",
		ModelID: "test-model",
		Content: []mcp.ContextItem{
			{Role: "system", Content: "You are a helpful assistant.", Timestamp: time.Now(), Tokens: 8},
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentTokens: 8,
	}

	newContent := []mcp.ContextItem{
		{Role: "user", Content: "Replace me!", Timestamp: time.Now(), Tokens: 4},
	}

	updateRequest := &mcp.Context{
		Content: newContent,
	}

	options := &mcp.ContextUpdateOptions{ReplaceContent: true}

	mockDB.On("GetContext", mock.Anything, contextID).
		Run(func(args mock.Arguments) {
			// no-op
		}).
		Return(copyContext(initialContext), nil)
	mockDB.On("UpdateContext", mock.Anything, mock.AnythingOfType("*mcp.Context")).Return(nil)
	mockCache.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*mcp.Context")).Return(fmt.Errorf("cache miss"))
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	
	t.Logf("Type of mock context: %T", copyContext(initialContext))
	result, err := cm.UpdateContext(context.Background(), contextID, updateRequest, options)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Content))
	assert.Equal(t, "Replace me!", result.Content[0].Content)
	assert.Equal(t, 4, result.CurrentTokens)
	mockCache.AssertExpectations(t)
}

func TestContextManager_UpdateContext_AppendContent(t *testing.T) {
	// Setup mock database and cache
	mockDB := new(MockDB)
	mockCache := new(mocks.MockCache)

	cm := NewContextManager(mockDB, mockCache)

	contextID := "test-context-id"
	initialContext := &mcp.Context{
		ID:      contextID,
		AgentID: "test-agent",
		ModelID: "test-model",
		Content: []mcp.ContextItem{
			{Role: "system", Content: "You are a helpful assistant.", Timestamp: time.Now(), Tokens: 8},
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentTokens: 8,
	}

	// Simulate persistence
	persistedContext := copyContext(initialContext)

	newContent := []mcp.ContextItem{
		{Role: "user", Content: "Hello, can you help me?", Timestamp: time.Now(), Tokens: 6},
	}

	updateRequest := &mcp.Context{
		Content: newContent,
	}

	var options *mcp.ContextUpdateOptions = nil

	mockDB.On("GetContext", mock.Anything, contextID).
		Run(func(args mock.Arguments) {
			// no-op
		}).
		Return(copyContext(persistedContext), nil)
	mockDB.On("UpdateContext", mock.Anything, mock.AnythingOfType("*mcp.Context")).Run(func(args mock.Arguments) {
		ctx := args.Get(1).(*mcp.Context)
		if ctx.Content != nil && len(ctx.Content) > 0 {
			persistedContext.Content = append(persistedContext.Content, ctx.Content...)
			persistedContext.CurrentTokens += ctx.CurrentTokens
		}
		// Optionally merge other fields if needed
	}).Return(nil)
	mockCache.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*mcp.Context")).Return(fmt.Errorf("cache miss"))
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	
	
	result, err := cm.UpdateContext(context.Background(), contextID, updateRequest, options)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	if len(result.Content) != 2 {
		t.Errorf("Expected 2 items in result.Content, got %d: %+v", len(result.Content), result.Content)
	}
	if len(result.Content) == 2 {
		assert.Equal(t, "system", result.Content[0].Role)
		assert.Equal(t, "user", result.Content[1].Role)
		assert.Equal(t, 14, result.CurrentTokens)
	} else if len(result.Content) == 1 {
		// Only appended one item, so only user role present
		assert.Equal(t, "user", result.Content[0].Role)
	}
	mockCache.AssertExpectations(t)
}
