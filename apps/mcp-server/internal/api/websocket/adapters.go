package websocket

import (
    "context"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "mcp-server/internal/core"
)

// contextManagerAdapter adapts core.ContextManagerInterface to websocket.ContextManager
type contextManagerAdapter struct {
    coreManager core.ContextManagerInterface
}

// NewContextManagerAdapter creates a new adapter
func NewContextManagerAdapter(coreManager core.ContextManagerInterface) ContextManager {
    return &contextManagerAdapter{
        coreManager: coreManager,
    }
}

// GetContext implements websocket.ContextManager
func (a *contextManagerAdapter) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
    return a.coreManager.GetContext(ctx, contextID)
}

// UpdateContext implements websocket.ContextManager with simplified signature
func (a *contextManagerAdapter) UpdateContext(ctx context.Context, contextID string, content string) (*models.Context, error) {
    // Create a context update with the new content
    updateData := &models.Context{
        Content: []models.ContextItem{
            {
                Content: content,
                Role:    "user",
            },
        },
    }
    
    // Use default options
    options := &models.ContextUpdateOptions{
        Truncate: false,
    }
    
    return a.coreManager.UpdateContext(ctx, contextID, updateData, options)
}

// TruncateContext implements websocket.ContextManager
func (a *contextManagerAdapter) TruncateContext(ctx context.Context, contextID string, maxTokens int, preserveRecent bool) (*TruncatedContext, int, error) {
    // Get current context
    context, err := a.coreManager.GetContext(ctx, contextID)
    if err != nil {
        return nil, 0, err
    }
    
    // Calculate tokens to remove
    removedTokens := 0
    if context.CurrentTokens > maxTokens {
        removedTokens = context.CurrentTokens - maxTokens
    }
    
    // Update context with truncation
    updateData := &models.Context{
        Content: context.Content, // This would be truncated in a real implementation
    }
    
    options := &models.ContextUpdateOptions{
        Truncate:         true,
        TruncateStrategy: "keep_recent",
    }
    
    updatedContext, err := a.coreManager.UpdateContext(ctx, contextID, updateData, options)
    if err != nil {
        return nil, 0, err
    }
    
    return &TruncatedContext{
        ID:         updatedContext.ID,
        TokenCount: maxTokens,
    }, removedTokens, nil
}

// CreateContext implements websocket.ContextManager
func (a *contextManagerAdapter) CreateContext(ctx context.Context, agentID, tenantID, name, content string) (*models.Context, error) {
    // Create a new context
    newContext := &models.Context{
        Name:    name,
        AgentID: agentID,
        Content: []models.ContextItem{
            {
                Content: content,
                Role:    "system",
            },
        },
    }
    
    return a.coreManager.CreateContext(ctx, newContext)
}