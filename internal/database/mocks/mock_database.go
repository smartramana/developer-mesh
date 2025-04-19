package mocks

import (
	"context"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock implementation of the Database interface
type MockDatabase struct {
	mock.Mock
}

// Transaction mocks the Transaction method
func (m *MockDatabase) Transaction(ctx context.Context, fn func(*Tx) error) error {
	args := m.Called(ctx, fn)
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

// getContext mocks the internal getContext method
func (m *MockDatabase) getContext(ctx context.Context, tx interface{}, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, tx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// CreateContext mocks the CreateContext method
func (m *MockDatabase) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// createContext mocks the internal createContext method
func (m *MockDatabase) createContext(ctx context.Context, tx interface{}, contextData *mcp.Context) error {
	args := m.Called(ctx, tx, contextData)
	return args.Error(0)
}

// UpdateContext mocks the UpdateContext method
func (m *MockDatabase) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// updateContext mocks the internal updateContext method
func (m *MockDatabase) updateContext(ctx context.Context, tx interface{}, contextData *mcp.Context) error {
	args := m.Called(ctx, tx, contextData)
	return args.Error(0)
}

// DeleteContext mocks the DeleteContext method
func (m *MockDatabase) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// deleteContext mocks the internal deleteContext method
func (m *MockDatabase) deleteContext(ctx context.Context, tx interface{}, contextID string) error {
	args := m.Called(ctx, tx, contextID)
	return args.Error(0)
}

// ListContexts mocks the ListContexts method
func (m *MockDatabase) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// listContexts mocks the internal listContexts method
func (m *MockDatabase) listContexts(ctx context.Context, tx interface{}, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, tx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// SearchContexts mocks the SearchContexts method
func (m *MockDatabase) SearchContexts(ctx context.Context, agentID string, query string, limit int) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// searchContexts mocks the internal searchContexts method
func (m *MockDatabase) searchContexts(ctx context.Context, tx interface{}, agentID string, query string, limit int) ([]*mcp.Context, error) {
	args := m.Called(ctx, tx, agentID, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// Tx is a mock database transaction
type Tx struct {
	mock.Mock
}
