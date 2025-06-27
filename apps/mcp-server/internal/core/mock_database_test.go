package core

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock implementation of the database interface
// (moved from merged_mocks.go for test visibility)
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) CreateContext(ctx context.Context, contextData *models.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockDatabase) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Context), args.Error(1)
}

func (m *MockDatabase) UpdateContext(ctx context.Context, contextData *models.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockDatabase) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockDatabase) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*models.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Context), args.Error(1)
}

func (m *MockDatabase) GetDB() interface{} {
	args := m.Called()
	return args.Get(0)
}
