package mocks

import (
	"context"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/stretchr/testify/mock"
)

// MockEngine is a mock implementation of the Engine interface for testing
type MockEngine struct {
	mock.Mock
}

// GetAdapter mocks the GetAdapter method
func (m *MockEngine) GetAdapter(name string) (adapters.Adapter, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(adapters.Adapter), args.Error(1)
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
