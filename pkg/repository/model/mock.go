package model

import (
	"context"
	"errors"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// MockRepository is a mock implementation of the Repository interface
type MockRepository struct{}

// NewMockRepository creates a new mock repository for testing
func NewMockRepository() Repository {
	return &MockRepository{}
}

// Create implements the Repository interface
func (m *MockRepository) Create(ctx context.Context, model *models.Model) error {
	// Mock implementation that does nothing but return success
	return nil
}

// Get implements the Repository interface
func (m *MockRepository) Get(ctx context.Context, id string) (*models.Model, error) {
	// Mock implementation that returns a dummy model
	return &models.Model{
		ID:       id,
		Name:     "Mock Model",
		TenantID: "mock-tenant",
	}, nil
}

// List implements the Repository interface
func (m *MockRepository) List(ctx context.Context, filter map[string]interface{}) ([]*models.Model, error) {
	// Mock implementation that returns an empty list
	return []*models.Model{}, nil
}

// Update implements the Repository interface
func (m *MockRepository) Update(ctx context.Context, model *models.Model) error {
	// Mock implementation that does nothing but return success
	return nil
}

// Delete implements the Repository interface
func (m *MockRepository) Delete(ctx context.Context, id string) error {
	// Mock implementation that does nothing but return success
	return nil
}

// CreateModel implements the API-specific method
func (m *MockRepository) CreateModel(ctx context.Context, model *models.Model) error {
	return m.Create(ctx, model)
}

// GetModelByID implements the API-specific method
func (m *MockRepository) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	model, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// If found, verify tenant ID matches
	if model != nil && model.TenantID != tenantID {
		return nil, errors.New("model not found for tenant")
	}
	
	return model, nil
}

// ListModels implements the API-specific method
func (m *MockRepository) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	filter := map[string]interface{}{"tenant_id": tenantID}
	return m.List(ctx, filter)
}

// UpdateModel implements the API-specific method
func (m *MockRepository) UpdateModel(ctx context.Context, model *models.Model) error {
	return m.Update(ctx, model)
}

// DeleteModel implements the API-specific method
func (m *MockRepository) DeleteModel(ctx context.Context, id string) error {
	return m.Delete(ctx, id)
}
