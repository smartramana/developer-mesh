package adapters

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"rest-api/internal/types"
)

// MockAgentAdapter is a mock implementation of AgentAdapter for testing
type MockAgentAdapter struct {
	mock.Mock
}

// CreateAgent mocks the CreateAgent method
func (m *MockAgentAdapter) CreateAgent(ctx context.Context, agent *models.Agent) (*models.Agent, error) {
	args := m.Called(ctx, agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

// GetAgent mocks the GetAgent method
func (m *MockAgentAdapter) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

// UpdateAgent mocks the UpdateAgent method
func (m *MockAgentAdapter) UpdateAgent(ctx context.Context, id string, agent *models.Agent) (*models.Agent, error) {
	args := m.Called(ctx, id, agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

// DeleteAgent mocks the DeleteAgent method
func (m *MockAgentAdapter) DeleteAgent(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ListAgents mocks the ListAgents method
func (m *MockAgentAdapter) ListAgents(ctx context.Context, filter *types.AgentFilter) ([]*models.Agent, *types.PaginationInfo, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*models.Agent), args.Get(1).(*types.PaginationInfo), args.Error(2)
}