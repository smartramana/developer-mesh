package events

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// ContextManager defines the interface for managing contexts
type ContextManager interface {
	// GetContext retrieves a context by ID
	GetContext(ctx context.Context, contextID string) (*models.Context, error)

	// UpdateContext updates an existing context
	UpdateContext(ctx context.Context, contextID string, updatedContext *models.Context, options interface{}) (*models.Context, error)
}
