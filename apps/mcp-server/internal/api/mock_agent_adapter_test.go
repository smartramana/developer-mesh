package api

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/stretchr/testify/mock"
)

// MockAgentAdapter implements repository.AgentRepository for testing
type MockAgentAdapter struct {
	mock.Mock
}

// Core repository interface methods
func (m *MockAgentAdapter) Create(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentAdapter) Get(ctx context.Context, id string) (*models.Agent, error) {
	args := m.Called(ctx, id)
	if agent := args.Get(0); agent != nil {
		return agent.(*models.Agent), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAgentAdapter) List(ctx context.Context, filter map[string]interface{}) ([]*models.Agent, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*models.Agent), args.Error(1)
}

func (m *MockAgentAdapter) Update(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentAdapter) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// API-specific method names that the agent_api.go expects
func (m *MockAgentAdapter) CreateAgent(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentAdapter) GetAgentByID(ctx context.Context, tenantID string, id string) (*models.Agent, error) {
	args := m.Called(ctx, tenantID, id)
	if agent := args.Get(0); agent != nil {
		return agent.(*models.Agent), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAgentAdapter) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.Agent), args.Error(1)
}

func (m *MockAgentAdapter) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentAdapter) DeleteAgent(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
