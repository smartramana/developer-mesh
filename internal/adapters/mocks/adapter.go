package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockAdapter is a mock implementation of the Adapter interface for testing
type MockAdapter struct {
	mock.Mock
}

// Initialize mocks the Initialize method
func (m *MockAdapter) Initialize(ctx context.Context, config interface{}) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

// GetData mocks the GetData method
func (m *MockAdapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	args := m.Called(ctx, query)
	return args.Get(0), args.Error(1)
}

// Subscribe mocks the Subscribe method
func (m *MockAdapter) Subscribe(eventType string, callback func(interface{})) error {
	args := m.Called(eventType, callback)
	return args.Error(0)
}

// HandleWebhook mocks the HandleWebhook method
func (m *MockAdapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	args := m.Called(ctx, eventType, payload)
	return args.Error(0)
}

// Health mocks the Health method
func (m *MockAdapter) Health() string {
	args := m.Called()
	return args.String(0)
}

// Close mocks the Close method
func (m *MockAdapter) Close() error {
	args := m.Called()
	return args.Error(0)
}
