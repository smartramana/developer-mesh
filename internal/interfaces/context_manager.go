package interfaces

import (
	"context"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// ContextManager defines the interface for context management operations
type ContextManager interface {
	// GetContext retrieves a context by ID
	GetContext(ctx context.Context, id string) (*mcp.Context, error)

	// CreateContext creates a new context
	CreateContext(ctx context.Context, contextData *mcp.Context) (*mcp.Context, error)

	// UpdateContext updates an existing context
	UpdateContext(ctx context.Context, id string, contextData *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error)

	// ListContexts lists contexts with optional filtering
	ListContexts(ctx context.Context, agentID string, modelID string, options map[string]interface{}) ([]*mcp.Context, error)

	// DeleteContext deletes a context by ID
	DeleteContext(ctx context.Context, id string) error
}
