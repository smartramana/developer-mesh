package mocks

import (
	"context"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
)

// MockContextDatabase is a mock implementation of the Database interface
type MockContextDatabase struct {
	mock.Mock
}

// Transaction mocks the Transaction method
func (m *MockContextDatabase) Transaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

// GetContext mocks the GetContext method
func (m *MockContextDatabase) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// getContext mocks the internal getContext method
func (m *MockContextDatabase) getContext(ctx context.Context, tx interface{}, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, tx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// CreateContext mocks the CreateContext method
func (m *MockContextDatabase) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// createContext mocks the internal createContext method
func (m *MockContextDatabase) createContext(ctx context.Context, tx interface{}, contextData *mcp.Context) error {
	args := m.Called(ctx, tx, contextData)
	return args.Error(0)
}

// UpdateContext mocks the UpdateContext method
func (m *MockContextDatabase) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// updateContext mocks the internal updateContext method
func (m *MockContextDatabase) updateContext(ctx context.Context, tx interface{}, contextData *mcp.Context) error {
	args := m.Called(ctx, tx, contextData)
	return args.Error(0)
}

// DeleteContext mocks the DeleteContext method
func (m *MockContextDatabase) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// deleteContext mocks the internal deleteContext method
func (m *MockContextDatabase) deleteContext(ctx context.Context, tx interface{}, contextID string) error {
	args := m.Called(ctx, tx, contextID)
	return args.Error(0)
}

// ListContexts mocks the ListContexts method
func (m *MockContextDatabase) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// listContexts mocks the internal listContexts method
func (m *MockContextDatabase) listContexts(ctx context.Context, tx interface{}, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, tx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// SearchContexts mocks the SearchContexts method
func (m *MockContextDatabase) SearchContexts(ctx context.Context, agentID string, query string, limit int) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// searchContexts mocks the internal searchContexts method
func (m *MockContextDatabase) searchContexts(ctx context.Context, tx interface{}, agentID string, query string, limit int) ([]*mcp.Context, error) {
	args := m.Called(ctx, tx, agentID, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// MockTx is a mock database transaction
type MockTx struct {
	mock.Mock
}
