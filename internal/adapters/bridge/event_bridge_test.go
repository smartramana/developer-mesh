package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/events/system"
	"github.com/S-Corkum/mcp-server/internal/observability"
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

// Version mocks the Version method
func (m *MockAdapter) Version() string {
	args := m.Called()
	return args.String(0)
}

// MockEventBus implements the events.EventBus interface
type MockEventBus struct {
	mock.Mock
}

// SubscribeAll mocks the SubscribeAll method
func (m *MockEventBus) SubscribeAll(listener events.EventListener) {
	m.Called(listener)
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

// Subscribe mocks the Subscribe method
func (m *MockEventBus) Subscribe(eventType events.EventType, listener events.EventListener) {
	m.Called(eventType, listener)
}

// Unsubscribe mocks the Unsubscribe method
func (m *MockEventBus) Unsubscribe(eventType events.EventType, listener events.EventListener) {
	m.Called(eventType, listener)
}

// UnsubscribeAll mocks the UnsubscribeAll method
func (m *MockEventBus) UnsubscribeAll(listener events.EventListener) {
	m.Called(listener)
}

// MockSystemEventBus implements the system.EventBus interface
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

// MockAdapterRegistry implements the core.AdapterRegistry interface
type MockAdapterRegistry struct {
	mock.Mock
}

// ListAdapters mocks the ListAdapters method
func (m *MockAdapterRegistry) ListAdapters() map[string]core.Adapter {
	args := m.Called()
	return args.Get(0).(map[string]core.Adapter)
}

// GetAdapter mocks the GetAdapter method
func (m *MockAdapterRegistry) GetAdapter(ctx context.Context, adapterType string) (core.Adapter, error) {
	args := m.Called(ctx, adapterType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(core.Adapter), args.Error(1)
}

// RegisterAdapter mocks the RegisterAdapter method
func (m *MockAdapterRegistry) RegisterAdapter(adapterType string, adapter core.Adapter) {
	m.Called(adapterType, adapter)
}

// DeregisterAdapter mocks the DeregisterAdapter method
func (m *MockAdapterRegistry) DeregisterAdapter(adapterType string) error {
	args := m.Called(adapterType)
	return args.Error(0)
}

func TestNewEventBridge(t *testing.T) {
	// Test creating a new event bridge with mocked event bus
	mockEventBus := &MockEventBus{}
	mockSystemEventBus := new(MockSystemEventBus)
	mockAdapterRegistry := new(MockAdapterRegistry)
	logger := observability.NewLogger("event-bridge-test")
	
	// Expect SubscribeAll to be called when creating a bridge with an event bus
	mockEventBus.On("SubscribeAll", mock.Anything).Return()
	
	bridge := NewEventBridge(mockEventBus, mockSystemEventBus, logger, mockAdapterRegistry)
	
	assert.NotNil(t, bridge)
	assert.Equal(t, mockEventBus, bridge.eventBus)
	assert.Equal(t, mockSystemEventBus, bridge.systemEventBus)
	assert.Equal(t, logger, bridge.logger)
	assert.Equal(t, mockAdapterRegistry, bridge.adapterRegistry)
	assert.NotNil(t, bridge.adapterHandlers)
	
	mockEventBus.AssertExpectations(t)
	
	// Test creating a bridge without event bus
	bridge = NewEventBridge(nil, mockSystemEventBus, logger, mockAdapterRegistry)
	
	assert.NotNil(t, bridge)
	assert.Nil(t, bridge.eventBus) // using interface{} so nil will work correctly
	assert.Equal(t, mockSystemEventBus, bridge.systemEventBus)
	assert.Equal(t, logger, bridge.logger)
	assert.Equal(t, mockAdapterRegistry, bridge.adapterRegistry)
}

func TestHandle(t *testing.T) {
	// Create test context and mocks
	ctx := context.Background()
	mockEventBus := new(MockEventBus)
	mockSystemEventBus := new(MockSystemEventBus)
	mockAdapterRegistry := new(MockAdapterRegistry)
	logger := observability.NewLogger("event-bridge-test")
	
	mockEventBus.On("SubscribeAll", mock.Anything).Return()
	
	bridge := NewEventBridge(mockEventBus, mockSystemEventBus, logger, mockAdapterRegistry)
	
	// Test cases
	testCases := []struct {
		name       string
		event      *events.AdapterEvent
		setupMocks func()
		expectErr  bool
	}{
		{
			name: "operation success event",
			event: &events.AdapterEvent{
				ID:          "event-123",
				AdapterType: "github",
				EventType:   events.EventTypeOperationSuccess,
				Timestamp:   time.Now(),
				Payload:     map[string]interface{}{"repos": []string{"repo1", "repo2"}},
				Metadata: map[string]interface{}{
					"operation": "list_repos",
					"contextId": "context-123",
				},
			},
			setupMocks: func() {
				// Expect the event to be mapped to a system event
				mockSystemEventBus.On("Publish", mock.Anything, mock.MatchedBy(func(e system.Event) bool {
					// Check if it's mapped to the right type
					opEvent, ok := e.(*system.AdapterOperationSuccessEvent)
					if !ok {
						return false
					}
					return opEvent.AdapterType == "github" && 
						   opEvent.Operation == "list_repos" &&
						   opEvent.ContextID == "context-123"
				})).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "operation failure event",
			event: &events.AdapterEvent{
				ID:          "event-456",
				AdapterType: "github",
				EventType:   events.EventTypeOperationFailure,
				Timestamp:   time.Now(),
				Payload:     nil,
				Metadata: map[string]interface{}{
					"operation": "get_repo",
					"contextId": "context-456",
					"error":     "repo not found",
				},
			},
			setupMocks: func() {
				// Expect the event to be mapped to a system event
				mockSystemEventBus.On("Publish", mock.Anything, mock.MatchedBy(func(e system.Event) bool {
					// Check if it's mapped to the right type
					failEvent, ok := e.(*system.AdapterOperationFailureEvent)
					if !ok {
						return false
					}
					return failEvent.AdapterType == "github" && 
						   failEvent.Operation == "get_repo" &&
						   failEvent.ContextID == "context-456" &&
						   failEvent.Error == "repo not found"
				})).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "webhook received event",
			event: &events.AdapterEvent{
				ID:          "event-789",
				AdapterType: "github",
				EventType:   events.EventTypeWebhookReceived,
				Timestamp:   time.Now(),
				Payload:     map[string]interface{}{"action": "push", "ref": "refs/heads/main"},
				Metadata: map[string]interface{}{
					"contextId": "context-789",
					"eventType": "push",
				},
			},
			setupMocks: func() {
				// Expect the event to be mapped to a system event
				mockSystemEventBus.On("Publish", mock.Anything, mock.MatchedBy(func(e system.Event) bool {
					// Check if it's mapped to the right type
					webhookEvent, ok := e.(*system.WebhookReceivedEvent)
					if !ok {
						return false
					}
					return webhookEvent.AdapterType == "github" && 
						   webhookEvent.EventType == "push" &&
						   webhookEvent.ContextID == "context-789"
				})).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "health changed event",
			event: &events.AdapterEvent{
				ID:          "event-health",
				AdapterType: "aws",
				EventType:   events.EventTypeAdapterHealthChanged,
				Timestamp:   time.Now(),
				Payload:     nil,
				Metadata: map[string]interface{}{
					"oldStatus": "healthy",
					"newStatus": "unhealthy: circuit breaker open",
				},
			},
			setupMocks: func() {
				// Expect the event to be mapped to a system event
				mockSystemEventBus.On("Publish", mock.Anything, mock.MatchedBy(func(e system.Event) bool {
					// Check if it's mapped to the right type
					healthEvent, ok := e.(*system.AdapterHealthChangedEvent)
					if !ok {
						return false
					}
					return healthEvent.AdapterType == "aws" && 
						   healthEvent.OldStatus == "healthy" &&
						   healthEvent.NewStatus == "unhealthy: circuit breaker open"
				})).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "generic event with no direct mapping",
			event: &events.AdapterEvent{
				ID:          "event-generic",
				AdapterType: "custom",
				EventType:   "custom.event",
				Timestamp:   time.Now(),
				Payload:     map[string]string{"data": "test"},
				Metadata:    map[string]interface{}{"key": "value"},
			},
			setupMocks: func() {
				// Expect the event to be mapped to a generic system event
				mockSystemEventBus.On("Publish", mock.Anything, mock.MatchedBy(func(e system.Event) bool {
					// Check if it's mapped to the right type
					genericEvent, ok := e.(*system.AdapterGenericEvent)
					if !ok {
						return false
					}
					return genericEvent.AdapterType == "custom" && 
						   genericEvent.EventType == "custom.event" 
				})).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "event with registered handler",
			event: &events.AdapterEvent{
				ID:          "event-handler",
				AdapterType: "github",
				EventType:   events.EventTypeWebhookReceived,
				Timestamp:   time.Now(),
				Payload:     map[string]string{"action": "push"},
				Metadata:    map[string]interface{}{"repo": "test-repo"},
			},
			setupMocks: func() {
				// Expect event to be published to system event bus
				mockSystemEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil)
				
				// Register a handler for this event type
				handlerCalled := false
				bridge.RegisterHandler("github", events.EventTypeWebhookReceived, func(ctx context.Context, event *events.AdapterEvent) error {
					handlerCalled = true
					assert.Equal(t, "github", event.AdapterType)
					assert.Equal(t, events.EventTypeWebhookReceived, event.EventType)
					return nil
				})
				
				// Check if the handler was called after test
				t.Cleanup(func() {
					assert.True(t, handlerCalled, "Event handler should have been called")
				})
			},
			expectErr: false,
		},
	}
	
	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up mocks for this test case
			tc.setupMocks()
			
			// Call the method
			err := bridge.Handle(ctx, tc.event)
			
			// Check results
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			// Verify mocks
			mockSystemEventBus.AssertExpectations(t)
		})
	}
}

func TestRegisterHandler(t *testing.T) {
	// Create test context and mocks
	ctx := context.Background()
	mockEventBus := new(MockEventBus)
	mockSystemEventBus := new(MockSystemEventBus)
	mockAdapterRegistry := new(MockAdapterRegistry)
	logger := observability.NewLogger("event-bridge-test")
	
	mockEventBus.On("SubscribeAll", mock.Anything).Return()
	
	bridge := NewEventBridge(mockEventBus, mockSystemEventBus, logger, mockAdapterRegistry)
	
	// Register a handler
	handlerCalled := false
	handler := func(ctx context.Context, event *events.AdapterEvent) error {
		handlerCalled = true
		return nil
	}
	
	bridge.RegisterHandler("github", events.EventTypeWebhookReceived, handler)
	
	// Create and handle an event that should trigger the handler
	event := &events.AdapterEvent{
		ID:          "test-event",
		AdapterType: "github",
		EventType:   events.EventTypeWebhookReceived,
		Timestamp:   time.Now(),
		Payload:     map[string]string{"action": "push"},
	}
	
	// Mock system event bus call
	mockSystemEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil)
	
	// Handle the event
	err := bridge.Handle(ctx, event)
	
	// Verify results
	assert.NoError(t, err)
	assert.True(t, handlerCalled, "Handler should have been called")
	mockSystemEventBus.AssertExpectations(t)
	
	// Reset for next test
	handlerCalled = false
	
	// Test that handler for different event type doesn't get called
	diffEvent := &events.AdapterEvent{
		ID:          "test-event-2",
		AdapterType: "github",
		EventType:   events.EventTypeOperationSuccess, // Different event type
		Timestamp:   time.Now(),
		Payload:     map[string]string{"result": "success"},
	}
	
	// Mock system event bus call for the different event
	mockSystemEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil)
	
	// Handle the event
	err = bridge.Handle(ctx, diffEvent)
	
	// Verify results
	assert.NoError(t, err)
	assert.False(t, handlerCalled, "Handler should not have been called for different event type")
	mockSystemEventBus.AssertExpectations(t)
}

func TestRegisterHandlerForAllAdapters(t *testing.T) {
	// Create test context and mocks
	mockEventBus := new(MockEventBus)
	mockSystemEventBus := new(MockSystemEventBus)
	mockAdapterRegistry := new(MockAdapterRegistry)
	logger := observability.NewLogger("event-bridge-test")
	
	// Setup the mock adapter registry to return some adapters
	adapters := map[string]core.Adapter{
		"github": &MockAdapter{},
		"aws":    &MockAdapter{},
	}
	mockAdapterRegistry.On("ListAdapters").Return(adapters)
	
	mockEventBus.On("SubscribeAll", mock.Anything).Return()
	
	bridge := NewEventBridge(mockEventBus, mockSystemEventBus, logger, mockAdapterRegistry)
	
	// Register a handler for all adapters
	handlerCalls := make(map[string]bool)
	handler := func(ctx context.Context, event *events.AdapterEvent) error {
		handlerCalls[event.AdapterType] = true
		return nil
	}
	
	bridge.RegisterHandlerForAllAdapters(events.EventTypeWebhookReceived, handler)
	
	// Create events for each adapter
	githubEvent := &events.AdapterEvent{
		AdapterType: "github",
		EventType:   events.EventTypeWebhookReceived,
		Timestamp:   time.Now(),
		Payload:     nil,
	}
	
	awsEvent := &events.AdapterEvent{
		AdapterType: "aws",
		EventType:   events.EventTypeWebhookReceived,
		Timestamp:   time.Now(),
		Payload:     nil,
	}
	
	// Mock system events publishing
	mockSystemEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil).Times(2)
	
	// Handle events
	err := bridge.Handle(context.Background(), githubEvent)
	require.NoError(t, err)
	
	err = bridge.Handle(context.Background(), awsEvent)
	require.NoError(t, err)
	
	// Verify handlers were called
	assert.True(t, handlerCalls["github"])
	assert.True(t, handlerCalls["aws"])
	
	mockAdapterRegistry.AssertExpectations(t)
	mockSystemEventBus.AssertExpectations(t)
}

func TestMapToSystemEvent(t *testing.T) {
	// Create bridge instance for testing
	bridge := &EventBridge{
		logger: observability.NewLogger("event-bridge-test"),
	}
	
	// Test cases for different event mappings
	testCases := []struct {
		name        string
		adapterEvent *events.AdapterEvent
		validateSystemEvent func(t *testing.T, event system.Event)
	}{
		{
			name: "map operation success event",
			adapterEvent: &events.AdapterEvent{
				ID:          "event-123",
				AdapterType: "github",
				EventType:   events.EventTypeOperationSuccess,
				Timestamp:   time.Now(),
				Payload:     map[string]string{"repos": "list"},
				Metadata: map[string]interface{}{
					"operation": "list_repos",
					"contextId": "context-123",
				},
			},
			validateSystemEvent: func(t *testing.T, event system.Event) {
				assert.IsType(t, &system.AdapterOperationSuccessEvent{}, event)
				
				typedEvent := event.(*system.AdapterOperationSuccessEvent)
				assert.Equal(t, "github", typedEvent.AdapterType)
				assert.Equal(t, "list_repos", typedEvent.Operation)
				assert.Equal(t, "context-123", typedEvent.ContextID)
				assert.NotNil(t, typedEvent.Result)
			},
		},
		{
			name: "map operation failure event",
			adapterEvent: &events.AdapterEvent{
				ID:          "event-456",
				AdapterType: "aws",
				EventType:   events.EventTypeOperationFailure,
				Timestamp:   time.Now(),
				Payload:     nil,
				Metadata: map[string]interface{}{
					"operation": "create_instance",
					"error":     "insufficient permissions",
					"contextId": "context-456",
				},
			},
			validateSystemEvent: func(t *testing.T, event system.Event) {
				assert.IsType(t, &system.AdapterOperationFailureEvent{}, event)
				
				typedEvent := event.(*system.AdapterOperationFailureEvent)
				assert.Equal(t, "aws", typedEvent.AdapterType)
				assert.Equal(t, "create_instance", typedEvent.Operation)
				assert.Equal(t, "context-456", typedEvent.ContextID)
				assert.Equal(t, "insufficient permissions", typedEvent.Error)
			},
		},
		{
			name: "map webhook received event",
			adapterEvent: &events.AdapterEvent{
				ID:          "event-789",
				AdapterType: "github",
				EventType:   events.EventTypeWebhookReceived,
				Timestamp:   time.Now(),
				Payload:     map[string]string{"action": "push"},
				Metadata: map[string]interface{}{
					"eventType": "push",
					"contextId": "context-789",
				},
			},
			validateSystemEvent: func(t *testing.T, event system.Event) {
				assert.IsType(t, &system.WebhookReceivedEvent{}, event)
				
				typedEvent := event.(*system.WebhookReceivedEvent)
				assert.Equal(t, "github", typedEvent.AdapterType)
				assert.Equal(t, "push", typedEvent.EventType)
				assert.Equal(t, "context-789", typedEvent.ContextID)
				assert.NotNil(t, typedEvent.Payload)
			},
		},
		{
			name: "map adapter health changed event",
			adapterEvent: &events.AdapterEvent{
				ID:          "event-health",
				AdapterType: "jira",
				EventType:   events.EventTypeAdapterHealthChanged,
				Timestamp:   time.Now(),
				Payload:     nil,
				Metadata: map[string]interface{}{
					"oldStatus": "healthy",
					"newStatus": "unhealthy",
				},
			},
			validateSystemEvent: func(t *testing.T, event system.Event) {
				assert.IsType(t, &system.AdapterHealthChangedEvent{}, event)
				
				typedEvent := event.(*system.AdapterHealthChangedEvent)
				assert.Equal(t, "jira", typedEvent.AdapterType)
				assert.Equal(t, "healthy", typedEvent.OldStatus)
				assert.Equal(t, "unhealthy", typedEvent.NewStatus)
			},
		},
		{
			name: "map generic event",
			adapterEvent: &events.AdapterEvent{
				ID:          "event-generic",
				AdapterType: "custom",
				EventType:   "custom.event",
				Timestamp:   time.Now(),
				Payload:     map[string]string{"key": "value"},
				Metadata:    map[string]interface{}{"meta": "data"},
			},
			validateSystemEvent: func(t *testing.T, event system.Event) {
				assert.IsType(t, &system.AdapterGenericEvent{}, event)
				
				typedEvent := event.(*system.AdapterGenericEvent)
				assert.Equal(t, "custom", typedEvent.AdapterType)
				assert.Equal(t, "custom.event", typedEvent.EventType)
				assert.NotNil(t, typedEvent.Payload)
				assert.NotNil(t, typedEvent.Metadata)
			},
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Map the adapter event to a system event
			result := bridge.mapToSystemEvent(tc.adapterEvent)
			
			// Validate the mapping
			assert.NotNil(t, result)
			assert.Equal(t, tc.adapterEvent.Timestamp, result.GetTimestamp())
			
			// Validate specific event type and fields
			tc.validateSystemEvent(t, result)
		})
	}
}

func TestCallEventHandlers(t *testing.T) {
	// Create bridge instance
	bridge := &EventBridge{
		logger:          observability.NewLogger("event-bridge-test"),
		adapterHandlers: make(map[string]map[string][]func(context.Context, *events.AdapterEvent) error),
	}
	
	// Setup test context
	ctx := context.Background()
	
	// Register handlers
	githubHandlerCalled := false
	awsHandlerCalled := false
	wildcardHandlerCalled := false
	errorHandler := false
	
	// GitHub webhook handler
	bridge.RegisterHandler("github", events.EventTypeWebhookReceived, func(ctx context.Context, event *events.AdapterEvent) error {
		githubHandlerCalled = true
		return nil
	})
	
	// AWS operation success handler
	bridge.RegisterHandler("aws", events.EventTypeOperationSuccess, func(ctx context.Context, event *events.AdapterEvent) error {
		awsHandlerCalled = true
		return nil
	})
	
	// Wildcard adapter, specific event type handler
	bridge.RegisterHandler("*", events.EventTypeOperationFailure, func(ctx context.Context, event *events.AdapterEvent) error {
		wildcardHandlerCalled = true
		return nil
	})
	
	// Error handler
	bridge.RegisterHandler("error", events.EventTypeOperationSuccess, func(ctx context.Context, event *events.AdapterEvent) error {
		errorHandler = true
		return assert.AnError
	})
	
	// Test cases
	testCases := []struct {
		name           string
		event          *events.AdapterEvent
		expectedGithub bool
		expectedAWS    bool
		expectedWildcard bool
		expectedError  bool
		shouldError    bool
	}{
		{
			name: "github webhook event",
			event: &events.AdapterEvent{
				AdapterType: "github",
				EventType:   events.EventTypeWebhookReceived,
			},
			expectedGithub: true,
			expectedAWS:    false,
			expectedWildcard: false,
			expectedError:  false,
			shouldError:    false,
		},
		{
			name: "aws operation success event",
			event: &events.AdapterEvent{
				AdapterType: "aws",
				EventType:   events.EventTypeOperationSuccess,
			},
			expectedGithub: false,
			expectedAWS:    true,
			expectedWildcard: false,
			expectedError:  false,
			shouldError:    false,
		},
		{
			name: "github operation failure event - wildcard match",
			event: &events.AdapterEvent{
				AdapterType: "github",
				EventType:   events.EventTypeOperationFailure,
			},
			expectedGithub: false,
			expectedAWS:    false,
			expectedWildcard: true,
			expectedError:  false,
			shouldError:    false,
		},
		{
			name: "error handler event",
			event: &events.AdapterEvent{
				AdapterType: "error",
				EventType:   events.EventTypeOperationSuccess,
			},
			expectedGithub: false,
			expectedAWS:    false,
			expectedWildcard: false,
			expectedError:  true,
			shouldError:    true,
		},
		{
			name: "no registered handler",
			event: &events.AdapterEvent{
				AdapterType: "unknown",
				EventType:   "unknown.event",
			},
			expectedGithub: false,
			expectedAWS:    false,
			expectedWildcard: false,
			expectedError:  false,
			shouldError:    false,
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset flags
			githubHandlerCalled = false
			awsHandlerCalled = false
			wildcardHandlerCalled = false
			errorHandler = false
			
			// Call the method
			err := bridge.callEventHandlers(ctx, tc.event)
			
			// Check results
			assert.Equal(t, tc.expectedGithub, githubHandlerCalled, "GitHub handler called")
			assert.Equal(t, tc.expectedAWS, awsHandlerCalled, "AWS handler called")
			assert.Equal(t, tc.expectedWildcard, wildcardHandlerCalled, "Wildcard handler called")
			assert.Equal(t, tc.expectedError, errorHandler, "Error handler called")
			
			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
