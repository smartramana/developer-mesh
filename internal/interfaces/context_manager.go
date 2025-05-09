package interfaces

import (
	"context"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
)

// ContextManager defines the interface for managing contexts
type ContextManager interface {
	// CreateContext creates a new context
	CreateContext(ctx context.Context, contextData *mcp.Context) (*mcp.Context, error)
	
	// GetContext retrieves a context by ID
	GetContext(ctx context.Context, contextID string) (*mcp.Context, error)
	
	// UpdateContext updates an existing context
	UpdateContext(ctx context.Context, contextID string, updateData *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error)
	
	// DeleteContext deletes a context
	DeleteContext(ctx context.Context, contextID string) error
	
	// ListContexts lists contexts for an agent
	ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error)
	
	// SummarizeContext generates a summary of a context
	SummarizeContext(ctx context.Context, contextID string) (string, error)
	
	// SearchInContext searches for text within a context
	SearchInContext(ctx context.Context, contextID string, query string) ([]mcp.ContextItem, error)
}
