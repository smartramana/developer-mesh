package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockEngine is a mock implementation of the Engine interface for testing
type MockEngine struct {
	mock.Mock
}

// GetAdapter mocks the GetAdapter method
func (m *MockEngine) GetAdapter(adapterType string) (interface{}, error) {
	args := m.Called(adapterType)
	return args.Get(0), args.Error(1)
}

// ExecuteAdapterAction mocks the ExecuteAdapterAction method
func (m *MockEngine) ExecuteAdapterAction(ctx context.Context, contextID string, adapterType string, action string, params map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, contextID, adapterType, action, params)
	return args.Get(0), args.Error(1)
}

// HandleAdapterWebhook mocks the HandleAdapterWebhook method
func (m *MockEngine) HandleAdapterWebhook(ctx context.Context, adapterType string, eventType string, payload []byte) error {
	args := m.Called(ctx, adapterType, eventType, payload)
	return args.Error(0)
}

// RecordWebhookInContext mocks the RecordWebhookInContext method
func (m *MockEngine) RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error) {
	args := m.Called(ctx, agentID, adapterType, eventType, payload)
	return args.String(0), args.Error(1)
}

// Health mocks the Health method
func (m *MockEngine) Health() map[string]string {
	args := m.Called()
	return args.Get(0).(map[string]string)
}

// Shutdown mocks the Shutdown method
func (m *MockEngine) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
