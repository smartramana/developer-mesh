package adapters

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/stretchr/testify/mock"
)

// MockModelAdapter is a mock implementation of ModelAdapter for testing
type MockModelAdapter struct {
	mock.Mock
}

// CreateModel mocks the CreateModel method
func (m *MockModelAdapter) CreateModel(ctx context.Context, model *models.Model) (*models.Model, error) {
	args := m.Called(ctx, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}

// GetModel mocks the GetModel method
func (m *MockModelAdapter) GetModel(ctx context.Context, id string) (*models.Model, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}

// UpdateModel mocks the UpdateModel method
func (m *MockModelAdapter) UpdateModel(ctx context.Context, id string, model *models.Model) (*models.Model, error) {
	args := m.Called(ctx, id, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}

// DeleteModel mocks the DeleteModel method
func (m *MockModelAdapter) DeleteModel(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ListModels mocks the ListModels method
func (m *MockModelAdapter) ListModels(ctx context.Context, filter *models.ModelFilter) ([]*models.Model, *models.PaginationInfo, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*models.Model), args.Get(1).(*models.PaginationInfo), args.Error(2)
}
