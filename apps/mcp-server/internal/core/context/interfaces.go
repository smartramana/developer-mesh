// Package context provides context management for the MCP application
package context

import (
	"context"
	
	"mcp-server/internal/core/models"
)

// ContextManager defines the interface for managing contexts
type ContextManager interface {
	// CreateContext creates a new context
	CreateContext(ctx context.Context, contextData *models.Context) (*models.Context, error)
	
	// GetContext retrieves a context by ID
	GetContext(ctx context.Context, contextID string) (*models.Context, error)
	
	// UpdateContext updates an existing context
	UpdateContext(ctx context.Context, contextID string, updateData *models.Context, options *models.ContextUpdateOptions) (*models.Context, error)
	
	// DeleteContext deletes a context
	DeleteContext(ctx context.Context, contextID string) error
	
	// ListContexts lists contexts for an agent
	ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*models.Context, error)
	
	// SummarizeContext generates a summary of a context
	SummarizeContext(ctx context.Context, contextID string) (string, error)
	
	// SearchInContext searches for text within a context
	SearchInContext(ctx context.Context, contextID string, query string) ([]models.ContextItem, error)
}

// Ensure Manager implements ContextManager
var _ ContextManager = (*Manager)(nil)
