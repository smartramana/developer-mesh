// Package adapters provides compatibility adapters for the API code
package adapters

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// ModelAdapter implements the legacy ModelRepository interface
// but delegates to the new repository.ModelRepository interface
type ModelAdapter struct {
	repo repository.ModelRepository
}

// NewModelAdapter creates a new ModelAdapter
func NewModelAdapter(repo repository.ModelRepository) *ModelAdapter {
	return &ModelAdapter{repo: repo}
}

// CreateModel creates a new model - adapter method that calls Create
func (a *ModelAdapter) CreateModel(ctx context.Context, model *models.Model) error {
	return a.repo.Create(ctx, model)
}

// GetModelByID retrieves a model by ID and tenant ID
// Adapter method that calls Get and then checks the tenant ID
func (a *ModelAdapter) GetModelByID(ctx context.Context, tenantID, id string) (*models.Model, error) {
	model, err := a.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// If tenantID is provided, verify that the model belongs to the tenant
	if tenantID != "" && model.TenantID != tenantID {
		return nil, nil // Not found for this tenant
	}

	return model, nil
}

// ListModels lists models for a tenant - adapter method that calls List with tenant filter
func (a *ModelAdapter) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	filter := map[string]any{
		"tenant_id": tenantID,
	}
	return a.repo.List(ctx, filter)
}

// UpdateModel updates a model - adapter method that calls Update
func (a *ModelAdapter) UpdateModel(ctx context.Context, model *models.Model) error {
	return a.repo.Update(ctx, model)
}

// DeleteModel deletes a model - adapter method that calls Delete
func (a *ModelAdapter) DeleteModel(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}
