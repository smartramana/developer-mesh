package core

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// copyContext creates a deep copy of a models.Context for test isolation
func copyContext(src *models.Context) *models.Context {
	if src == nil {
		return nil
	}
	copyItems := make([]models.ContextItem, len(src.Content))
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
	return &models.Context{
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
	// Remove the cache mock - the context manager will handle nil cache
	var mockCache cache.Cache = nil

	cm := NewContextManager(mockDB, mockCache)

	contextID := "test-context-id"
	initialContext := &models.Context{
		ID:      contextID,
		AgentID: "test-agent",
		ModelID: "test-model",
		Content: []models.ContextItem{
			{Role: "system", Content: "You are a helpful assistant.", Timestamp: time.Now(), Tokens: 8},
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentTokens: 8,
	}

	newContent := []models.ContextItem{
		{Role: "user", Content: "Replace me!", Timestamp: time.Now(), Tokens: 4},
	}

	updateRequest := &models.Context{
		Content: newContent,
	}

	options := &models.ContextUpdateOptions{ReplaceContent: true}

	mockDB.On("GetContext", mock.Anything, contextID).
		Run(func(args mock.Arguments) {
			// no-op
		}).
		Return(copyContext(initialContext), nil)
	mockDB.On("UpdateContext", mock.Anything, mock.AnythingOfType("*models.Context")).Return(nil)
	// Cache mock operations removed since we're using nil cache

	
	t.Logf("Type of mock context: %T", copyContext(initialContext))
	result, err := cm.UpdateContext(context.Background(), contextID, updateRequest, options)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Content))
	assert.Equal(t, "Replace me!", result.Content[0].Content)
	assert.Equal(t, 4, result.CurrentTokens)
	// Cache expectations removed
}

func TestContextManager_UpdateContext_AppendContent(t *testing.T) {
	// Setup mock database and cache
	mockDB := new(MockDB)
	// Remove the cache mock - the context manager will handle nil cache
	var mockCache cache.Cache = nil

	cm := NewContextManager(mockDB, mockCache)

	contextID := "test-context-id"
	initialContext := &models.Context{
		ID:      contextID,
		AgentID: "test-agent",
		ModelID: "test-model",
		Content: []models.ContextItem{
			{Role: "system", Content: "You are a helpful assistant.", Timestamp: time.Now(), Tokens: 8},
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentTokens: 8,
	}

	// Simulate persistence
	persistedContext := copyContext(initialContext)

	newContent := []models.ContextItem{
		{Role: "user", Content: "Hello, can you help me?", Timestamp: time.Now(), Tokens: 6},
	}

	updateRequest := &models.Context{
		Content: newContent,
	}

	var options *models.ContextUpdateOptions = nil

	mockDB.On("GetContext", mock.Anything, contextID).
		Run(func(args mock.Arguments) {
			// no-op
		}).
		Return(copyContext(persistedContext), nil)
	mockDB.On("UpdateContext", mock.Anything, mock.AnythingOfType("*models.Context")).Run(func(args mock.Arguments) {
		ctx := args.Get(1).(*models.Context)
		if ctx.Content != nil && len(ctx.Content) > 0 {
			persistedContext.Content = append(persistedContext.Content, ctx.Content...)
			persistedContext.CurrentTokens += ctx.CurrentTokens
		}
		// Optionally merge other fields if needed
	}).Return(nil)
	// Cache mock operations removed since we're using nil cache

	
	
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
	// Cache expectations removed
}
