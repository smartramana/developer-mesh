// Package mocks contains mock implementations for testing the GitHub adapter
package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/S-Corkum/mcp-server/internal/events"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// MockEventBus is a mock implementation of the EventBus interface for testing
type MockEventBus struct {
	mock.Mock
}

// NewMockEventBus creates a new mock event bus
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{}
}

// Publish implements the EventBusIface.Publish method
func (m *MockEventBus) Publish(ctx context.Context, event *mcp.Event) {
	m.Called(ctx, event)
}

// Subscribe implements the EventBusIface.Subscribe method
func (m *MockEventBus) Subscribe(eventType events.EventType, handler events.Handler) {
	m.Called(eventType, handler)
}

// Unsubscribe implements the EventBusIface.Unsubscribe method
func (m *MockEventBus) Unsubscribe(eventType events.EventType, handler events.Handler) {
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

