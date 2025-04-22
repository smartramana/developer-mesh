package core

import (
	"context"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock implementation of the database.Database for testing
type MockDatabase struct {
	mock.Mock
}

// CreateContext mocks the database.Database.CreateContext method
func (m *MockDatabase) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// GetContext mocks the database.Database.GetContext method
func (m *MockDatabase) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// UpdateContext mocks the database.Database.UpdateContext method
func (m *MockDatabase) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// DeleteContext mocks the database.Database.DeleteContext method
func (m *MockDatabase) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// ListContexts mocks the database.Database.ListContexts method
func (m *MockDatabase) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// CreateContextReference mocks the database.Database.CreateContextReference method
func (m *MockDatabase) CreateContextReference(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// UpdateContextReference mocks the database.Database.UpdateContextReference method
func (m *MockDatabase) UpdateContextReference(ctx context.Context, contextData *mcp.Context) error {
	args := m.Called(ctx, contextData)
	return args.Error(0)
}

// DeleteContextReference mocks the database.Database.DeleteContextReference method
func (m *MockDatabase) DeleteContextReference(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// ListContextReferences mocks the database.Database.ListContextReferences method
func (m *MockDatabase) ListContextReferences(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// GetDB mocks the database.Database.GetDB method
func (m *MockDatabase) GetDB() interface{} {
	args := m.Called()
	return args.Get(0)
}

// Close mocks the database.Database.Close method
func (m *MockDatabase) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockCache is a mock implementation of the cache.Cache for testing
type MockCache struct {
	mock.Mock
}

// Get mocks the cache.Cache.Get method
func (m *MockCache) Get(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

// Set mocks the cache.Cache.Set method
func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

// Delete mocks the cache.Cache.Delete method
func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

// Close mocks the cache.Cache.Close method
func (m *MockCache) Close() error {
	args := m.Called()
	return args.Error(0)
}
