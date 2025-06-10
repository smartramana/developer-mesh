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