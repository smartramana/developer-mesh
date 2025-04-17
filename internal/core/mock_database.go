package core

import (
	"context"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
)

// MockDB implements the database.Database interface
type MockDB struct {
	mock.Mock
}

func (m *MockDB) GetDB() *sqlx.DB {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*sqlx.DB)
}

func (m *MockDB) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDB) CreateContextReferenceTable(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Context operations interface for the ContextManager
func (m *MockDB) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockDB) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

func (m *MockDB) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

func (m *MockDB) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockDB) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}
