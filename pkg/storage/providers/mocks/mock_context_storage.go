package mocks

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/stretchr/testify/mock"
)

// MockContextStorage is a mock implementation of the ContextStorage interface
type MockContextStorage struct {
	mock.Mock
}

// StoreContext mocks the StoreContext method
func (m *MockContextStorage) StoreContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// GetContext mocks the GetContext method
func (m *MockContextStorage) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// DeleteContext mocks the DeleteContext method
func (m *MockContextStorage) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// ListContexts mocks the ListContexts method
func (m *MockContextStorage) ListContexts(ctx context.Context, agentID string, sessionID string) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}
