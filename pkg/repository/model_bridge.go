// Package repository provides a bridge to the new model package
package repository

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/model"
)

// Using type alias to reference the interface defined in interfaces.go
// to avoid redeclaration errors in the codebase.
// This maintains compatibility with existing code while consolidating interface definitions.

// CreateMockModelRepository creates a mock model repository for testing
func CreateMockModelRepository() ModelRepository {
	return &mockModelRepository{}
}

// mockModelRepository provides a simple implementation for testing
type mockModelRepository struct {
	models map[string]*models.Model
}

// Create implements the Create method for mockModelRepository
func (m *mockModelRepository) Create(ctx context.Context, model *models.Model) error {
	if m.models == nil {
		m.models = make(map[string]*models.Model)
	}
	m.models[model.ID] = model
	return nil
}

// Get implements the Get method for mockModelRepository
func (m *mockModelRepository) Get(ctx context.Context, id string) (*models.Model, error) {
	if m.models == nil {
		return nil, nil
	}
	return m.models[id], nil
}

// List implements the List method for mockModelRepository
func (m *mockModelRepository) List(ctx context.Context, filter model.Filter) ([]*models.Model, error) {
	var result []*models.Model
	
	if m.models == nil {
		return result, nil
	}
	
	for _, model := range m.models {
		matches := true
		for key, value := range filter {
			switch key {
			case "tenant_id":
				if model.TenantID != value.(string) {
					matches = false
				}
			case "id":
				if model.ID != value.(string) {
					matches = false
				}
			}
		}
		
		if matches {
			result = append(result, model)
		}
	}
	
	return result, nil
}

// Update implements the Update method for mockModelRepository
func (m *mockModelRepository) Update(ctx context.Context, model *models.Model) error {
	if m.models == nil {
		m.models = make(map[string]*models.Model)
	}
	m.models[model.ID] = model
	return nil
}

// Delete implements the Delete method for mockModelRepository
func (m *mockModelRepository) Delete(ctx context.Context, id string) error {
	if m.models != nil {
		delete(m.models, id)
	}
	return nil
}

// CreateModel implements the API-specific method
func (m *mockModelRepository) CreateModel(ctx context.Context, model *models.Model) error {
	return m.Create(ctx, model)
}

// GetModelByID implements the API-specific method 
func (m *mockModelRepository) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	model, _ := m.Get(ctx, id)
	if model != nil && model.TenantID != tenantID {
		return nil, nil
	}
	return model, nil
}

// ListModels implements the API-specific method
func (m *mockModelRepository) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	filter := model.FilterFromTenantID(tenantID)
	return m.List(ctx, filter)
}

// UpdateModel implements the API-specific method
func (m *mockModelRepository) UpdateModel(ctx context.Context, model *models.Model) error {
	return m.Update(ctx, model)
}

// DeleteModel implements the API-specific method
func (m *mockModelRepository) DeleteModel(ctx context.Context, id string) error {
	return m.Delete(ctx, id)
}
