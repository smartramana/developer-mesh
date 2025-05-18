// Package model provides interfaces and implementations for model entities
package model

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Repository defines operations for managing model entities
type Repository interface {
	// Core repository methods
	Create(ctx context.Context, model *models.Model) error
	Get(ctx context.Context, id string) (*models.Model, error)
	List(ctx context.Context, filter map[string]interface{}) ([]*models.Model, error)
	Update(ctx context.Context, model *models.Model) error
	Delete(ctx context.Context, id string) error
	
	// API-specific methods
	CreateModel(ctx context.Context, model *models.Model) error
	GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error)
	ListModels(ctx context.Context, tenantID string) ([]*models.Model, error)
	UpdateModel(ctx context.Context, model *models.Model) error
	DeleteModel(ctx context.Context, id string) error
}

// FilterFromTenantID creates a filter map from a tenant ID
func FilterFromTenantID(tenantID string) map[string]interface{} {
	return map[string]interface{}{
		"tenant_id": tenantID,
	}
}

// FilterFromIDs creates a filter map from tenant ID and model ID
func FilterFromIDs(tenantID, id string) map[string]interface{} {
	return map[string]interface{}{
		"tenant_id": tenantID,
		"id":        id,
	}
}
