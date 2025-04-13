package mocks

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock implementation of the Database interface for testing
type MockDatabase struct {
	mock.Mock
}

// Get mocks the Get method
func (m *MockDatabase) Get(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	callArgs := m.Called(ctx, dest, query, args)
	return callArgs.Error(0)
}

// Select mocks the Select method
func (m *MockDatabase) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	callArgs := m.Called(ctx, dest, query, args)
	return callArgs.Error(0)
}

// Exec mocks the Exec method
func (m *MockDatabase) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	callArgs := m.Called(ctx, query, args)
	return callArgs.Get(0).(sql.Result), callArgs.Error(1)
}

// NamedExec mocks the NamedExec method
func (m *MockDatabase) NamedExec(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	callArgs := m.Called(ctx, query, arg)
	return callArgs.Get(0).(sql.Result), callArgs.Error(1)
}

// BeginTx mocks the BeginTx method
func (m *MockDatabase) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	callArgs := m.Called(ctx)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(*sqlx.Tx), callArgs.Error(1)
}

// Close mocks the Close method
func (m *MockDatabase) Close() error {
	callArgs := m.Called()
	return callArgs.Error(0)
}

// Ping mocks the Ping method
func (m *MockDatabase) Ping(ctx context.Context) error {
	callArgs := m.Called(ctx)
	return callArgs.Error(0)
}
