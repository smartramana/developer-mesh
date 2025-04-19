package core

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)



// Using the MockCache defined in mock_database.go

// Using the MockMetricsClient defined in engine.go

// MockAdapterTest mocks the adapter interface
type MockAdapterTest struct {
	mock.Mock
}

func (m *MockAdapterTest) Initialize(ctx context.Context, config interface{}) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockAdapterTest) Health() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAdapterTest) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, contextID, action, params)
	return args.Get(0), args.Error(1)
}

func (m *MockAdapterTest) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	args := m.Called(ctx, query)
	return args.Get(0), args.Error(1)
}

func (m *MockAdapterTest) Subscribe(eventType string, callback func(interface{})) error {
	args := m.Called(eventType, callback)
	return args.Error(0)
}

func (m *MockAdapterTest) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAdapterTest) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	args := m.Called(action, params)
	return args.Bool(0), args.Error(1)
}

func (m *MockAdapterTest) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	args := m.Called(ctx, eventType, payload)
	return args.Error(0)
}

func TestSetupGithubEventHandlers(t *testing.T) {
	// Create mock adapter
	mockAdapter := new(MockAdapterTest)
	
	// Set up expectations for our event subscriptions
	mockAdapter.On("Subscribe", "pull_request", mock.AnythingOfType("func(interface {})")).Return(nil)
	mockAdapter.On("Subscribe", "push", mock.AnythingOfType("func(interface {})")).Return(nil)
	
	// Create an engine with an event channel
	engine := &Engine{
		events: make(chan mcp.Event, 10),
	}
	
	// Call the function
	err := engine.setupGithubEventHandlers(mockAdapter)
	
	// Verify
	assert.NoError(t, err)
	mockAdapter.AssertExpectations(t)
	
	// Test error handling
	mockFailingAdapter := new(MockAdapterTest)
	mockFailingAdapter.On("Subscribe", "pull_request", mock.Anything).Return(assert.AnError)
	
	err = engine.setupGithubEventHandlers(mockFailingAdapter)
	assert.Error(t, err)
	mockFailingAdapter.AssertExpectations(t)
}

func TestEngineHealth(t *testing.T) {
	// Create mock dependencies
	mockAdapter := new(MockAdapterTest)
	mockAdapter.On("Health").Return("healthy")
	
	// Create an engine with the mock adapter
	engine := &Engine{
		adapters: map[string]core.Adapter{
			"test-adapter": mockAdapter,
		},
	}
	
	// Call the Health method
	health := engine.Health()
	
	// Verify
	assert.Equal(t, "healthy", health["engine"])
	assert.Equal(t, "healthy", health["test-adapter"])
	mockAdapter.AssertExpectations(t)
}

func TestGetAdapter(t *testing.T) {
	// Create mock adapter
	mockAdapter := new(MockAdapterTest)
	
	// Create an engine with the mock adapter
	engine := &Engine{
		adapters: map[string]interfaces.Adapter{
			"test-adapter": mockAdapter,
		},
	}
	
	// Test getting an existing adapter
	adapter, err := engine.GetAdapter("test-adapter")
	assert.NoError(t, err)
	assert.Equal(t, mockAdapter, adapter)
	
	// Test getting a non-existent adapter
	adapter, err = engine.GetAdapter("nonexistent-adapter")
	assert.Error(t, err)
	assert.Nil(t, adapter)
}

func TestListAdapters(t *testing.T) {
	// Create mock adapters
	mockAdapter1 := new(MockAdapterTest)
	mockAdapter2 := new(MockAdapterTest)
	
	// Create an engine with the mock adapters
	engine := &Engine{
		adapters: map[string]core.Adapter{
			"adapter1": mockAdapter1,
			"adapter2": mockAdapter2,
		},
	}
	
	// Test listing adapters
	adapterList := engine.ListAdapters()
	
	// Verify
	assert.Contains(t, adapterList, "adapter1")
	assert.Contains(t, adapterList, "adapter2")
	assert.Len(t, adapterList, 2)
}

func TestProcessEvent(t *testing.T) {
	// Create mock deps
	mockContextManager := new(MockContextManager)
	
	// Create an engine with a buffered events channel
	engine := &Engine{
		events:         make(chan mcp.Event, 10),
		ContextManager: mockContextManager,
	}
	
	// Create a test event
	event := mcp.Event{
		Source:    "test-source",
		Type:      "test-type",
		Timestamp: time.Time{}, // Zero timestamp to test auto-setting
	}
	
	// Process the event
	engine.ProcessEvent(event)
	
	// Verify that the event was placed in the events channel with a timestamp
	receivedEvent := <-engine.events
	assert.Equal(t, "test-source", receivedEvent.Source)
	assert.Equal(t, "test-type", receivedEvent.Type)
	assert.False(t, receivedEvent.Timestamp.IsZero()) // Timestamp should be set
}

func TestEngineShutdown(t *testing.T) {
	// Create mock adapter
	mockAdapter := new(MockAdapterTest)
	mockAdapter.On("Close").Return(nil)
	
	// Create context for shutdown
	ctx := context.Background()
	
	// Create an engine with the mock adapter
	engineCtx, cancel := context.WithCancel(ctx)
	engine := &Engine{
		ctx:     engineCtx,
		cancel:  cancel,
		adapters: map[string]core.Adapter{
			"test-adapter": mockAdapter,
		},
		events:  make(chan mcp.Event, 10),
	}
	
	// Call shutdown
	err := engine.Shutdown(ctx)
	
	// Verify
	assert.NoError(t, err)
	mockAdapter.AssertExpectations(t)
	
	// Check that the events channel is closed
	_, ok := <-engine.events
	assert.False(t, ok, "Events channel should be closed")
}

func TestExecuteAdapterAction(t *testing.T) {
	// Create mock adapter
	mockAdapter := new(MockAdapterTest)
	mockAdapter.On("IsSafeOperation", "test-action", mock.Anything).Return(true, nil)
	mockAdapter.On("ExecuteAction", mock.Anything, "test-context", "test-action", mock.Anything).Return("test-result", nil)
	
	// Create test context
	testContext := &mcp.Context{
		ID:        "test-context",
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []mcp.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Create mock context manager
	mockContextManager := new(MockContextManager)
	mockContextManager.On("GetContext", mock.Anything, "test-context").Return(testContext, nil)
	mockContextManager.On("UpdateContext", mock.Anything, "test-context", mock.Anything, mock.Anything).Return(testContext, nil)
	
	// Create an engine with the mock adapter
	engine := &Engine{
		adapters: map[string]core.Adapter{
			"test-adapter": mockAdapter,
		},
		ContextManager: mockContextManager,
	}
	
	// Test executing an action
	ctx := context.Background()
	params := map[string]interface{}{"param": "value"}
	result, err := engine.ExecuteAdapterAction(ctx, "test-adapter", "test-context", "test-action", params)
	
	// Verify
	assert.NoError(t, err)
	assert.Equal(t, "test-result", result)
	mockAdapter.AssertExpectations(t)
}
