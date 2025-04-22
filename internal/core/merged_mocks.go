package core

import (
	"context"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock implementation of the database interface
type MockDatabase struct {
	mock.Mock
}

// CreateContext mocks the CreateContext method
func (m *MockDatabase) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// GetContext mocks the GetContext method
func (m *MockDatabase) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// UpdateContext mocks the UpdateContext method
func (m *MockDatabase) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// DeleteContext mocks the DeleteContext method
func (m *MockDatabase) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// ListContexts mocks the ListContexts method
func (m *MockDatabase) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// GetDB mocks the GetDB method
func (m *MockDatabase) GetDB() interface{} {
	args := m.Called()
	return args.Get(0)
}

// MockContextManager is a mock implementation of the ContextManager interface.
type MockContextManager struct {
	mock.Mock
}

// CreateContext mocks the CreateContext method.
func (m *MockContextManager) CreateContext(ctx context.Context, contextData *mcp.Context) (*mcp.Context, error) {
	args := m.Called(ctx, contextData)
	
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// GetContext mocks the GetContext method.
func (m *MockContextManager) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// UpdateContext mocks the UpdateContext method.
func (m *MockContextManager) UpdateContext(ctx context.Context, contextID string, updateData *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	args := m.Called(ctx, contextID, updateData, options)
	
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// DeleteContext mocks the DeleteContext method.
func (m *MockContextManager) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// ListContexts mocks the ListContexts method.
func (m *MockContextManager) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// Close mocks the Close method.
func (m *MockContextManager) Close() error {
	args := m.Called()
	return args.Error(0)
}
