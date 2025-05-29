// Package model provides interfaces and implementations for model entities
package model

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Filter defines a filter map for repository operations
// This avoids importing pkg/repository to prevent import cycles
type Filter map[string]any

// FilterFromTenantID creates a filter for tenant ID
func FilterFromTenantID(tenantID string) Filter {
	return Filter{"tenant_id": tenantID}
}

// FilterFromIDs creates a filter for tenant ID and model ID
func FilterFromIDs(tenantID, id string) Filter {
	return Filter{
		"tenant_id": tenantID,
		"id":        id,
	}
}

// Repository defines operations for managing model entities
// It follows the generic repository pattern while preserving API-specific methods
type Repository interface {
	// Core repository methods - aligned with generic Repository[T] interface
	Create(ctx context.Context, model *models.Model) error
	Get(ctx context.Context, id string) (*models.Model, error)
	List(ctx context.Context, filter Filter) ([]*models.Model, error)
	Update(ctx context.Context, model *models.Model) error
	Delete(ctx context.Context, id string) error

	// API-specific methods - preserved for backward compatibility
	// These methods delegate to the core methods in the implementation
	CreateModel(ctx context.Context, model *models.Model) error
	GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error)
	ListModels(ctx context.Context, tenantID string) ([]*models.Model, error)
	UpdateModel(ctx context.Context, model *models.Model) error
	DeleteModel(ctx context.Context, id string) error
}
