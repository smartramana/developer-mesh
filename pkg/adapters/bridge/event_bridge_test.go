package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/core"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	"github.com/S-Corkum/devops-mcp/pkg/events/system"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAdapter implements core.Adapter for testing
type MockAdapter struct {
	mock.Mock
}

// Type mocks the Type method
func (m *MockAdapter) Type() string {
	args := m.Called()
	return args.String(0)
}

// ExecuteAction mocks the ExecuteAction method
func (m *MockAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, contextID, action, params)
	return args.Get(0), args.Error(1)
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

// MockEventBus implements events.EventBus for testing
type MockEventBus struct {
	mock.Mock
}

// Emit mocks the Emit method
func (m *MockEventBus) Emit(ctx context.Context, event *events.AdapterEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// EmitWithCallback mocks the EmitWithCallback method
func (m *MockEventBus) EmitWithCallback(ctx context.Context, event *events.AdapterEvent, callback func(error)) error {
	args := m.Called(ctx, event, callback)
	return args.Error(0)
}

// SubscribeListener mocks the SubscribeListener method
func (m *MockEventBus) SubscribeListener(eventType events.EventType, listener events.EventListener) {
	m.Called(eventType, listener)
}

// SubscribeAll mocks the SubscribeAll method
func (m *MockEventBus) SubscribeAll(listener events.EventListener) {
	m.Called(listener)
}

// UnsubscribeListener mocks the UnsubscribeListener method
func (m *MockEventBus) UnsubscribeListener(eventType events.EventType, listener events.EventListener) {
	m.Called(eventType, listener)
}

// UnsubscribeAll mocks the UnsubscribeAll method
func (m *MockEventBus) UnsubscribeAll(listener events.EventListener) {
	m.Called(listener)
}

// MockSystemEventBus implements system.EventBus for testing
type MockSystemEventBus struct {
	mock.Mock
}

// Publish mocks the Publish method
func (m *MockSystemEventBus) Publish(ctx context.Context, event system.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// Subscribe mocks the Subscribe method
func (m *MockSystemEventBus) Subscribe(eventType system.EventType, handler func(ctx context.Context, event system.Event) error) {
	m.Called(eventType, handler)
}

// Unsubscribe mocks the Unsubscribe method
func (m *MockSystemEventBus) Unsubscribe(eventType system.EventType, handler func(ctx context.Context, event system.Event) error) {
	m.Called(eventType, handler)
}

// TestNewEventBridge tests the creation of a new event bridge
func TestNewEventBridge(t *testing.T) {
	// Create mocks
	mockEventBus := new(MockEventBus)
	mockSystemEventBus := new(MockSystemEventBus)
	logger := observability.NewLogger()
	mockRegistry := &MockAdapterRegistry{}

	// Set expectations
	mockEventBus.On("SubscribeAll", mock.Anything).Return()

	// Create bridge
	bridge := NewEventBridge(mockEventBus, mockSystemEventBus, logger, mockRegistry)

	// Assertions
	assert.NotNil(t, bridge)
	assert.Equal(t, mockEventBus, bridge.eventBus)
	assert.Equal(t, mockSystemEventBus, bridge.systemEventBus)
	assert.Equal(t, logger, bridge.logger)
	assert.Equal(t, mockRegistry, bridge.adapterRegistry)
	assert.NotNil(t, bridge.adapterHandlers)

	// Verify expectations
	mockEventBus.AssertExpectations(t)
}

// MockAdapterRegistry implements a mock adapter registry for testing
type MockAdapterRegistry struct {
	mock.Mock
}

// ListAdapters mocks the ListAdapters method
func (m *MockAdapterRegistry) ListAdapters() map[string]core.Adapter {
	args := m.Called()
	return args.Get(0).(map[string]core.Adapter)
}

// TestHandleAdapterEvent tests the Handle method of the event bridge
func TestHandleAdapterEvent(t *testing.T) {
	// Create mocks
	mockEventBus := new(MockEventBus)
	mockSystemEventBus := new(MockSystemEventBus)
	logger := observability.NewLogger()
	mockRegistry := &MockAdapterRegistry{}

	// Set expectations for event bus subscription
	mockEventBus.On("SubscribeAll", mock.Anything).Return()

	// Set expectations for system event bus publishing
	mockSystemEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil)

	// Create bridge
	bridge := NewEventBridge(mockEventBus, mockSystemEventBus, mockRegistry, logger)

	// Create a test event
	event := events.NewAdapterEvent("test-adapter", events.EventTypeOperationSuccess, "test payload")
	event.WithMetadata("contextId", "test-context")
	event.WithMetadata("operation", "test-operation")

	// Handle the event
	err := bridge.Handle(context.Background(), event)

	// Assertions
	require.NoError(t, err)
	mockSystemEventBus.AssertCalled(t, "Publish", mock.Anything, mock.MatchedBy(func(e system.Event) bool {
		// Verify the event was mapped correctly
		if e.GetType() != system.EventTypeAdapterOperationSuccess {
			return false
		}
		
		// Type assertion to check specific fields
		if successEvent, ok := e.(*system.AdapterOperationSuccessEvent); ok {
			return successEvent.AdapterType == "test-adapter" &&
				successEvent.Operation == "test-operation" &&
				successEvent.ContextID == "test-context"
		}
		
		return false
	}))
}

// TestRegisterHandler tests the RegisterHandler method
func TestRegisterHandler(t *testing.T) {
	// Create mocks
	mockEventBus := new(MockEventBus)
	mockSystemEventBus := new(MockSystemEventBus)
	logger := observability.NewLogger()
	mockRegistry := &MockAdapterRegistry{}

	// Set expectations
	mockEventBus.On("SubscribeAll", mock.Anything).Return()

	// Create bridge
	bridge := NewEventBridge(mockEventBus, mockSystemEventBus, logger, mockRegistry)

	// Define handler
	handlerCalled := false
	handler := func(ctx context.Context, event *events.AdapterEvent) error {
		handlerCalled = true
		return nil
	}

	// Register handler
	bridge.RegisterHandler("test-adapter", events.EventTypeOperationSuccess, handler)

	// Create a test event
	event := events.NewAdapterEvent("test-adapter", events.EventTypeOperationSuccess, "test payload")

	// Handle the event
	err := bridge.Handle(context.Background(), event)

	// Assertions
	require.NoError(t, err)
	assert.True(t, handlerCalled, "Handler should have been called")
}

// TestRegisterHandlerForAllAdapters tests the RegisterHandlerForAllAdapters method
func TestRegisterHandlerForAllAdapters(t *testing.T) {
	// Create mocks
	mockEventBus := new(MockEventBus)
	mockSystemEventBus := new(MockSystemEventBus)
	logger := observability.NewLogger()
	mockRegistry := &MockAdapterRegistry{}

	// Set up adapters
	adapters := map[string]core.Adapter{
		"adapter1": &MockAdapter{},
		"adapter2": &MockAdapter{},
	}
	mockRegistry.On("ListAdapters").Return(adapters)

	// Set expectations
	mockEventBus.On("SubscribeAll", mock.Anything).Return()

	// Create bridge
	bridge := NewEventBridge(mockEventBus, mockSystemEventBus, logger, mockRegistry)

	// Define handler
	handlerCalledCount := 0
	handler := func(ctx context.Context, event *events.AdapterEvent) error {
		handlerCalledCount++
		return nil
	}

	// Register handler for all adapters
	bridge.RegisterHandlerForAllAdapters(events.EventTypeOperationSuccess, handler)

	// Create test events for each adapter
	event1 := events.NewAdapterEvent("adapter1", events.EventTypeOperationSuccess, "test payload")
	event2 := events.NewAdapterEvent("adapter2", events.EventTypeOperationSuccess, "test payload")
	eventWildcard := events.NewAdapterEvent("adapter3", events.EventTypeOperationSuccess, "test payload")

	// Handle the events
	bridge.Handle(context.Background(), event1)
	bridge.Handle(context.Background(), event2)
	bridge.Handle(context.Background(), eventWildcard)

	// Assertions
	assert.Equal(t, 3, handlerCalledCount, "Handler should have been called for all events")
}
