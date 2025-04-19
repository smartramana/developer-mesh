package database

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// MockDatabase provides a mock implementation of Database for testing
type MockDatabase struct {
	TransactionFn func(ctx context.Context, fn func(*sqlx.Tx) error) error
}

// Transaction executes the mock transaction function
func (m *MockDatabase) Transaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	if m.TransactionFn != nil {
		return m.TransactionFn(ctx, fn)
	}
	return nil
}

// GetDB returns nil for mock
func (m *MockDatabase) GetDB() *sqlx.DB {
	return nil
}

// Close is a no-op for mock
func (m *MockDatabase) Close() error {
	return nil
}

// Ping is a no-op for mock
func (m *MockDatabase) Ping() error {
	return nil
}
