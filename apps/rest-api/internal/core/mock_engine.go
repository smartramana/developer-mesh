package core

import (
	"context"

	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/types"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/stretchr/testify/mock"
)

// MockEngine is a mock implementation of Engine for testing
type MockEngine struct {
	mock.Mock
}

// CreateAgent mocks the CreateAgent method
func (m *MockEngine) CreateAgent(ctx context.Context, agent *models.Agent) (*models.Agent, error) {
	args := m.Called(ctx, agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

// GetAgent mocks the GetAgent method
func (m *MockEngine) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

// UpdateAgent mocks the UpdateAgent method
func (m *MockEngine) UpdateAgent(ctx context.Context, id string, agent *models.Agent) (*models.Agent, error) {
	args := m.Called(ctx, id, agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

// DeleteAgent mocks the DeleteAgent method
func (m *MockEngine) DeleteAgent(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ListAgents mocks the ListAgents method
func (m *MockEngine) ListAgents(ctx context.Context, filter *types.AgentFilter) ([]*models.Agent, *types.PaginationInfo, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*models.Agent), args.Get(1).(*types.PaginationInfo), args.Error(2)
}

// CreateModel mocks the CreateModel method
func (m *MockEngine) CreateModel(ctx context.Context, model *models.Model) (*models.Model, error) {
	args := m.Called(ctx, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}

// GetModel mocks the GetModel method
func (m *MockEngine) GetModel(ctx context.Context, id string) (*models.Model, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}

// UpdateModel mocks the UpdateModel method
func (m *MockEngine) UpdateModel(ctx context.Context, id string, model *models.Model) (*models.Model, error) {
	args := m.Called(ctx, id, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Model), args.Error(1)
}

// DeleteModel mocks the DeleteModel method
func (m *MockEngine) DeleteModel(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ListModels mocks the ListModels method
func (m *MockEngine) ListModels(ctx context.Context, filter *types.ModelFilter) ([]*models.Model, *types.PaginationInfo, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*models.Model), args.Get(1).(*types.PaginationInfo), args.Error(2)
}

// CreateContext mocks the CreateContext method
func (m *MockEngine) CreateContext(ctx context.Context, context *models.Context) (*models.Context, error) {
	args := m.Called(ctx, context)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

// GetContext mocks the GetContext method
func (m *MockEngine) GetContext(ctx context.Context, id string) (*models.Context, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

// UpdateContext mocks the UpdateContext method
func (m *MockEngine) UpdateContext(ctx context.Context, id string, context *models.Context) (*models.Context, error) {
	args := m.Called(ctx, id, context)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

// DeleteContext mocks the DeleteContext method
func (m *MockEngine) DeleteContext(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ListContexts mocks the ListContexts method
func (m *MockEngine) ListContexts(ctx context.Context, filter *types.ContextFilter) ([]*models.Context, *types.PaginationInfo, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*models.Context), args.Get(1).(*types.PaginationInfo), args.Error(2)
}

// VectorSearch mocks the VectorSearch method
func (m *MockEngine) VectorSearch(ctx context.Context, query *types.VectorSearchQuery) ([]*types.VectorSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.VectorSearchResult), args.Error(1)
}

// Health mocks the Health method
func (m *MockEngine) Health(ctx context.Context) (*types.HealthStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.HealthStatus), args.Error(1)
}
