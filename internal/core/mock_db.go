package core

import (
	"context"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/mock"
)

// MockDB is a mock implementation for database operations in tests
type MockDB struct {
	mock.Mock
}

// CreateContext mocks creating a context in the database
func (m *MockDB) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// GetContext mocks retrieving a context from the database
func (m *MockDB) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// UpdateContext mocks updating a context in the database
func (m *MockDB) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// DeleteContext mocks deleting a context from the database
func (m *MockDB) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// ListContexts mocks listing contexts from the database
func (m *MockDB) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// GetDB mocks getting the underlying database connection
func (m *MockDB) GetDB() interface{} {
	args := m.Called()
	return args.Get(0)
}
