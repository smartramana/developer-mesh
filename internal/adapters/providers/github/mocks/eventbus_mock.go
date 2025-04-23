// Package mocks contains mock implementations for testing the GitHub adapter
package mocks

import (
	"context"
	
	"github.com/stretchr/testify/mock"
	"github.com/S-Corkum/mcp-server/internal/events/system"
)

// MockEventBus is a mock implementation of the EventBus interface for testing
type MockEventBus struct {
	mock.Mock
}

// NewMockEventBus creates a new mock event bus
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{}
}

// Publish mocks the Publish method
func (m *MockEventBus) Publish(ctx context.Context, event system.Event) {
	m.Called(ctx, event)
}

// Subscribe mocks the Subscribe method
func (m *MockEventBus) Subscribe(eventType string, handler system.EventHandler) {
	m.Called(eventType, handler)
}

// Close mocks the Close method
func (m *MockEventBus) Close() {
	m.Called()
}

// On mocks the On method for setting up expectations
func (m *MockEventBus) On(method string, args ...interface{}) *mock.Call {
	return m.Mock.On(method, args...)
}
