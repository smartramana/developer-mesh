package bridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockContextManager implements interfaces.ContextManager for testing
type MockContextManager struct {
	mock.Mock
}

// GetContext mocks the GetContext method
func (m *MockContextManager) GetContext(ctx context.Context, id string) (*mcp.Context, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// CreateContext mocks the CreateContext method
func (m *MockContextManager) CreateContext(ctx context.Context, contextData *mcp.Context) (*mcp.Context, error) {
	args := m.Called(ctx, contextData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// UpdateContext mocks the UpdateContext method
func (m *MockContextManager) UpdateContext(ctx context.Context, id string, contextData *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	args := m.Called(ctx, id, contextData, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Context), args.Error(1)
}

// ListContexts mocks the ListContexts method
func (m *MockContextManager) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	args := m.Called(ctx, agentID, sessionID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mcp.Context), args.Error(1)
}

// DeleteContext mocks the DeleteContext method
func (m *MockContextManager) DeleteContext(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// SearchInContext mocks the SearchInContext method
func (m *MockContextManager) SearchInContext(ctx context.Context, id string, query string) ([]mcp.ContextItem, error) {
	args := m.Called(ctx, id, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mcp.ContextItem), args.Error(1)
}

// SummarizeContext mocks the SummarizeContext method
func (m *MockContextManager) SummarizeContext(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

// MockEventBus implements a mock for events.EventBus
type MockEventBus struct {
	mock.Mock
}

// Subscribe mocks the Subscribe method
func (m *MockEventBus) Subscribe(eventType events.EventType, listener events.EventListener) {
	m.Called(eventType, listener)
}

// SubscribeAll mocks the SubscribeAll method
func (m *MockEventBus) SubscribeAll(listener events.EventListener) {
	m.Called(listener)
}

// Unsubscribe mocks the Unsubscribe method
func (m *MockEventBus) Unsubscribe(eventType events.EventType, listener events.EventListener) {
	m.Called(eventType, listener)
}

// UnsubscribeAll mocks the UnsubscribeAll method
func (m *MockEventBus) UnsubscribeAll(listener events.EventListener) {
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

func TestNewContextBridge(t *testing.T) {
	// Test creating a new context bridge
	mockContextManager := new(MockContextManager)
	mockEventBus := new(MockEventBus)
	logger := observability.NewLogger()
	
	// Expect SubscribeAll to be called when creating a bridge with an event bus
	mockEventBus.On("SubscribeAll", mock.Anything).Return()
	
	bridge := NewContextBridge(mockContextManager, logger, mockEventBus)
	
	assert.NotNil(t, bridge)
	assert.Equal(t, mockContextManager, bridge.contextManager)
	assert.Equal(t, logger, bridge.logger)
	assert.Equal(t, mockEventBus, bridge.eventBus)
	assert.Equal(t, resilience.DefaultRetryConfig(), bridge.retryConfig)
	
	mockEventBus.AssertExpectations(t)
	
	// Test creating a bridge without event bus
	bridge = NewContextBridge(mockContextManager, logger, nil)
	
	assert.NotNil(t, bridge)
	assert.Equal(t, mockContextManager, bridge.contextManager)
	assert.Equal(t, logger, bridge.logger)
	assert.Nil(t, bridge.eventBus)
}

func TestWithRetryConfig(t *testing.T) {
	// Test customizing retry configuration
	mockContextManager := new(MockContextManager)
	logger := observability.NewLogger()
	
	bridge := NewContextBridge(mockContextManager, logger, nil)
	
	// Create custom retry config
	customRetryConfig := resilience.RetryConfig{
		MaxRetries:      5,
		InitialInterval: 200 * time.Millisecond,
		MaxInterval:     20 * time.Second,
		Multiplier:      3.0,
		MaxElapsedTime:  60 * time.Second,
	}
	
	// Apply custom config and check it's applied
	modifiedBridge := bridge.WithRetryConfig(customRetryConfig)
	
	assert.Equal(t, bridge, modifiedBridge) // Should return the same bridge instance
	assert.Equal(t, customRetryConfig, modifiedBridge.retryConfig)
}

func TestHandle(t *testing.T) {
	// Create test context and mocks
	ctx := context.Background()
	mockContextManager := new(MockContextManager)
	mockEventBus := new(MockEventBus)
	logger := observability.NewLogger()
	
	bridge := NewContextBridge(mockContextManager, logger, mockEventBus)
	
	// Test cases
	testCases := []struct {
		name       string
		event      *events.AdapterEvent
		setupMocks func()
		expectErr  bool
	}{
		{
			name: "event without context ID",
			event: &events.AdapterEvent{
				ID:          "event-123",
				AdapterType: "test-adapter",
				EventType:   events.EventTypeOperationSuccess,
				Timestamp:   time.Now(),
				Metadata:    map[string]interface{}{}, // No context ID
			},
			setupMocks: func() {
				// No mocks needed, function should return early
			},
			expectErr: false,
		},
		{
			name: "event with context ID",
			event: &events.AdapterEvent{
				ID:          "event-456",
				AdapterType: "test-adapter",
				EventType:   events.EventTypeOperationSuccess,
				Timestamp:   time.Now(),
				Metadata: map[string]interface{}{
					"contextId": "context-123",
				},
			},
			setupMocks: func() {
				// Expect the bridge to record the event in the context
				existingContext := &mcp.Context{
					ID:            "context-123",
					Content:       []mcp.ContextItem{},
					CurrentTokens: 0,
				}
				
				mockContextManager.On("GetContext", mock.Anything, "context-123").Return(existingContext, nil)
				mockContextManager.On("UpdateContext", mock.Anything, "context-123", mock.AnythingOfType("*mcp.Context"), mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(existingContext, nil)
			},
			expectErr: false,
		},
		{
			name: "context manager error",
			event: &events.AdapterEvent{
				ID:          "event-789",
				AdapterType: "test-adapter",
				EventType:   events.EventTypeOperationSuccess,
				Timestamp:   time.Now(),
				Metadata: map[string]interface{}{
					"contextId": "error-context",
				},
			},
			setupMocks: func() {
				// Simulate an error when getting the context
				mockContextManager.On("GetContext", mock.Anything, "error-context").Return(nil, errors.New("context not found"))
				
				// Expect a fallback attempt to just append the item
				mockContextManager.On("GetContext", mock.Anything, "error-context").Return(nil, errors.New("context not found"))
			},
			expectErr: true,
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
			mockContextManager.AssertExpectations(t)
		})
		
		// Reset mocks for the next test
		mockContextManager = new(MockContextManager)
		bridge.contextManager = mockContextManager
	}
}

func TestRecordOperationInContext(t *testing.T) {
	// Create test context and mocks
	ctx := context.Background()
	mockContextManager := new(MockContextManager)
	logger := observability.NewLogger()
	
	bridge := NewContextBridge(mockContextManager, logger, nil)
	
	// Test cases
	testCases := []struct {
		name         string
		contextID    string
		adapterType  string
		operation    string
		request      interface{}
		response     interface{}
		err          error
		setupMocks   func()
		expectErr    bool
	}{
		{
			name:        "successful operation recording",
			contextID:   "context-123",
			adapterType: "github",
			operation:   "get_repo",
			request:     map[string]string{"repo": "test-repo"},
			response:    map[string]interface{}{"name": "test-repo", "stars": 100},
			err:         nil,
			setupMocks: func() {
				existingContext := &mcp.Context{
					ID:            "context-123",
					Content:       []mcp.ContextItem{},
					CurrentTokens: 0,
				}
				
				mockContextManager.On("GetContext", mock.Anything, "context-123").Return(existingContext, nil)
				
				// The content and tokens will be updated, so we need to use a matcher
				mockContextManager.On("UpdateContext", 
					mock.Anything, 
					"context-123", 
					mock.MatchedBy(func(c *mcp.Context) bool {
						// Check if the operation was recorded
						if len(c.Content) != 1 {
							return false
						}
						item := c.Content[0]
						return item.Role == "tool" && 
							   item.Metadata["adapter"] == "github" &&
							   item.Metadata["operation"] == "get_repo" &&
							   item.Metadata["status"] == "success"
					}),
					mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(existingContext, nil)
			},
			expectErr: false,
		},
		{
			name:        "failed operation recording",
			contextID:   "context-123",
			adapterType: "github",
			operation:   "get_repo",
			request:     map[string]string{"repo": "test-repo"},
			response:    nil,
			err:         errors.New("repo not found"),
			setupMocks: func() {
				existingContext := &mcp.Context{
					ID:            "context-123",
					Content:       []mcp.ContextItem{},
					CurrentTokens: 0,
				}
				
				mockContextManager.On("GetContext", mock.Anything, "context-123").Return(existingContext, nil)
				
				// The content and tokens will be updated, so we need to use a matcher
				mockContextManager.On("UpdateContext", 
					mock.Anything, 
					"context-123", 
					mock.MatchedBy(func(c *mcp.Context) bool {
						// Check if the operation was recorded
						if len(c.Content) != 1 {
							return false
						}
						item := c.Content[0]
						return item.Role == "tool" && 
							   item.Metadata["adapter"] == "github" &&
							   item.Metadata["operation"] == "get_repo" &&
							   item.Metadata["status"] == "failure" &&
							   item.Metadata["error"] == "repo not found"
					}),
					mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(existingContext, nil)
			},
			expectErr: false,
		},
		{
			name:        "context not found",
			contextID:   "not-found",
			adapterType: "github",
			operation:   "get_repo",
			request:     map[string]string{"repo": "test-repo"},
			response:    nil,
			err:         nil,
			setupMocks: func() {
				// Simulate context not found
				mockContextManager.On("GetContext", mock.Anything, "not-found").Return(nil, errors.New("context not found"))
				
				// Expect a fallback attempt to append just the operation item
				mockContextManager.On("GetContext", mock.Anything, "not-found").Return(nil, errors.New("context not found"))
			},
			expectErr: true,
		},
		{
			name:        "complex request and response",
			contextID:   "context-456",
			adapterType: "aws",
			operation:   "launch_instance",
			request: struct {
				InstanceType string
				AMI          string
				Tags         map[string]string
			}{
				InstanceType: "t2.micro",
				AMI:          "ami-12345",
				Tags:         map[string]string{"Name": "Test", "Environment": "Dev"},
			},
			response: struct {
				InstanceID string
				Status     string
				IP         string
			}{
				InstanceID: "i-abcdef",
				Status:     "pending",
				IP:         "10.0.0.1",
			},
			err: nil,
			setupMocks: func() {
				existingContext := &mcp.Context{
					ID:            "context-456",
					Content:       []mcp.ContextItem{},
					CurrentTokens: 0,
				}
				
				mockContextManager.On("GetContext", mock.Anything, "context-456").Return(existingContext, nil)
				mockContextManager.On("UpdateContext", 
					mock.Anything, 
					"context-456", 
					mock.MatchedBy(func(c *mcp.Context) bool {
						// Basic validation that content was added
						return len(c.Content) == 1 && c.Content[0].Role == "tool"
					}),
					mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(existingContext, nil)
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
			err := bridge.RecordOperationInContext(ctx, tc.contextID, tc.adapterType, tc.operation, tc.request, tc.response, tc.err)
			
			// Check results
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			// Verify mocks
			mockContextManager.AssertExpectations(t)
		})
		
		// Reset mocks for the next test
		mockContextManager = new(MockContextManager)
		bridge.contextManager = mockContextManager
	}
}

func TestRecordEventInContext(t *testing.T) {
	// Create test context and mocks
	ctx := context.Background()
	mockContextManager := new(MockContextManager)
	logger := observability.NewLogger()
	
	bridge := NewContextBridge(mockContextManager, logger, nil)
	
	// Create test data
	now := time.Now()
	
	// Test cases
	testCases := []struct {
		name         string
		contextID    string
		event        *events.AdapterEvent
		setupMocks   func()
		expectErr    bool
	}{
		{
			name:      "record simple event",
			contextID: "context-123",
			event: &events.AdapterEvent{
				ID:          "event-123",
				AdapterType: "github",
				EventType:   events.EventTypeWebhookReceived,
				Timestamp:   now,
				Payload:     map[string]interface{}{"action": "push", "repository": "test-repo"},
				Metadata:    map[string]interface{}{"branch": "main"},
			},
			setupMocks: func() {
				existingContext := &mcp.Context{
					ID:            "context-123",
					Content:       []mcp.ContextItem{},
					CurrentTokens: 0,
				}
				
				mockContextManager.On("GetContext", mock.Anything, "context-123").Return(existingContext, nil)
				
				// The content and tokens will be updated, so we need to use a matcher
				mockContextManager.On("UpdateContext", 
					mock.Anything, 
					"context-123", 
					mock.MatchedBy(func(c *mcp.Context) bool {
						// Check if the event was recorded
						if len(c.Content) != 1 {
							return false
						}
						item := c.Content[0]
						return item.Role == "event" && 
							   item.Metadata["adapter"] == "github" &&
							   item.Metadata["eventType"] == "webhook.received" &&
							   item.Metadata["eventId"] == "event-123" &&
							   item.Timestamp.Equal(now)
					}),
					mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(existingContext, nil)
			},
			expectErr: false,
		},
		{
			name:      "context not found",
			contextID: "not-found",
			event: &events.AdapterEvent{
				ID:          "event-456",
				AdapterType: "github",
				EventType:   events.EventTypeOperationSuccess,
				Timestamp:   now,
				Payload:     map[string]string{"status": "success"},
			},
			setupMocks: func() {
				// Simulate context not found
				mockContextManager.On("GetContext", mock.Anything, "not-found").Return(nil, errors.New("context not found"))
				
				// Expect a fallback attempt to append just the event item
				mockContextManager.On("GetContext", mock.Anything, "not-found").Return(nil, errors.New("context not found"))
			},
			expectErr: true,
		},
		{
			name:      "complex event payload",
			contextID: "context-456",
			event: &events.AdapterEvent{
				ID:          "event-789",
				AdapterType: "aws",
				EventType:   events.EventTypeOperationSuccess,
				Timestamp:   now,
				Payload: struct {
					Region     string
					Resources  []string
					Status     map[string]interface{}
				}{
					Region:    "us-west-2",
					Resources: []string{"instance-1", "instance-2"},
					Status: map[string]interface{}{
						"code":    200,
						"message": "success",
					},
				},
				Metadata: map[string]interface{}{"requestId": "req-12345"},
			},
			setupMocks: func() {
				existingContext := &mcp.Context{
					ID:            "context-456",
					Content:       []mcp.ContextItem{},
					CurrentTokens: 0,
				}
				
				mockContextManager.On("GetContext", mock.Anything, "context-456").Return(existingContext, nil)
				mockContextManager.On("UpdateContext", 
					mock.Anything, 
					"context-456", 
					mock.MatchedBy(func(c *mcp.Context) bool {
						// Basic validation that content was added
						return len(c.Content) == 1 && c.Content[0].Role == "event"
					}),
					mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(existingContext, nil)
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
			err := bridge.RecordEventInContext(ctx, tc.contextID, tc.event)
			
			// Check results
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			// Verify mocks
			mockContextManager.AssertExpectations(t)
		})
		
		// Reset mocks for the next test
		mockContextManager = new(MockContextManager)
		bridge.contextManager = mockContextManager
	}
}

func TestRecordWebhookInContext(t *testing.T) {
	// Create test context and mocks
	ctx := context.Background()
	mockContextManager := new(MockContextManager)
	mockEventBus := new(MockEventBus)
	logger := observability.NewLogger()
	
	bridge := NewContextBridge(mockContextManager, logger, mockEventBus)
	
	// Test cases
	testCases := []struct {
		name         string
		agentID      string
		adapterType  string
		eventType    string
		payload      interface{}
		setupMocks   func()
		expectContextID string
		expectErr    bool
	}{
		{
			name:        "existing context",
			agentID:     "agent-123",
			adapterType: "github",
			eventType:   "push",
			payload:     map[string]interface{}{"ref": "refs/heads/main", "repository": map[string]interface{}{"name": "test-repo"}},
			setupMocks: func() {
				// Return an existing context
				existingContexts := []*mcp.Context{
					{
						ID:            "context-123",
						AgentID:       "agent-123",
						Content:       []mcp.ContextItem{},
						CurrentTokens: 0,
					},
				}
				
				mockContextManager.On("ListContexts", mock.Anything, "agent-123", "", map[string]interface{}{"limit": 1}).Return(existingContexts, nil)
				
				// Expect context update
				mockContextManager.On("UpdateContext", 
					mock.Anything, 
					"context-123", 
					mock.MatchedBy(func(c *mcp.Context) bool {
						// Check if webhook was recorded
						if len(c.Content) != 1 {
							return false
						}
						item := c.Content[0]
						return item.Role == "webhook" && 
							   item.Metadata["adapter"] == "github" &&
							   item.Metadata["eventType"] == "push"
					}),
					mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(existingContexts[0], nil)
				
				// Expect event emission
				mockEventBus.On("Emit", mock.Anything, mock.MatchedBy(func(e *events.AdapterEvent) bool {
					return e.AdapterType == "github" && 
						   e.EventType == events.EventTypeWebhookReceived &&
						   e.Metadata["contextId"] == "context-123" &&
						   e.Metadata["eventType"] == "push"
				})).Return(nil)
			},
			expectContextID: "context-123",
			expectErr: false,
		},
		{
			name:        "new context creation",
			agentID:     "agent-456",
			adapterType: "jira",
			eventType:   "issue_created",
			payload:     map[string]interface{}{"key": "PROJ-123", "summary": "Test issue"},
			setupMocks: func() {
				// Return no existing contexts
				mockContextManager.On("ListContexts", mock.Anything, "agent-456", "", map[string]interface{}{"limit": 1}).Return([]*mcp.Context{}, nil)
				
				// Expect context creation
				mockContextManager.On("CreateContext", mock.Anything, mock.MatchedBy(func(c *mcp.Context) bool {
					return c.AgentID == "agent-456" && c.ModelID == "webhook"
				})).Return(&mcp.Context{
					ID:      "context-new",
					AgentID: "agent-456",
					ModelID: "webhook",
				}, nil)
				
				// Expect context update
				mockContextManager.On("UpdateContext", 
					mock.Anything, 
					"context-new", 
					mock.MatchedBy(func(c *mcp.Context) bool {
						return len(c.Content) == 1 && c.Content[0].Role == "webhook"
					}),
					mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(&mcp.Context{
						ID:      "context-new",
						AgentID: "agent-456",
					}, nil)
				
				// Expect event emission
				mockEventBus.On("Emit", mock.Anything, mock.MatchedBy(func(e *events.AdapterEvent) bool {
					return e.AdapterType == "jira" && 
						   e.EventType == events.EventTypeWebhookReceived &&
						   e.Metadata["contextId"] == "context-new" &&
						   e.Metadata["eventType"] == "issue_created"
				})).Return(nil)
			},
			expectContextID: "context-new",
			expectErr: false,
		},
		{
			name:        "error listing contexts",
			agentID:     "agent-error",
			adapterType: "github",
			eventType:   "push",
			payload:     map[string]string{"ref": "refs/heads/main"},
			setupMocks: func() {
				// Simulate error when listing contexts
				mockContextManager.On("ListContexts", mock.Anything, "agent-error", "", map[string]interface{}{"limit": 1}).Return(nil, errors.New("database error"))
			},
			expectContextID: "",
			expectErr: true,
		},
		{
			name:        "error creating context",
			agentID:     "agent-create-error",
			adapterType: "github",
			eventType:   "push",
			payload:     map[string]string{"ref": "refs/heads/main"},
			setupMocks: func() {
				// Return no existing contexts
				mockContextManager.On("ListContexts", mock.Anything, "agent-create-error", "", map[string]interface{}{"limit": 1}).Return([]*mcp.Context{}, nil)
				
				// Simulate error when creating context
				mockContextManager.On("CreateContext", mock.Anything, mock.Anything).Return(nil, errors.New("creation error"))
			},
			expectContextID: "",
			expectErr: true,
		},
	}
	
	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up mocks for this test case
			tc.setupMocks()
			
			// Call the method
			contextID, err := bridge.RecordWebhookInContext(ctx, tc.agentID, tc.adapterType, tc.eventType, tc.payload)
			
			// Check results
			if tc.expectErr {
				assert.Error(t, err)
				assert.Equal(t, tc.expectContextID, contextID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectContextID, contextID)
			}
			
			// Verify mocks
			mockContextManager.AssertExpectations(t)
			mockEventBus.AssertExpectations(t)
		})
		
		// Reset mocks for the next test
		mockContextManager = new(MockContextManager)
		mockEventBus = new(MockEventBus)
		bridge.contextManager = mockContextManager
		bridge.eventBus = mockEventBus
	}
}

func TestSafeJSONMarshal(t *testing.T) {
	bridge := &ContextBridge{
		logger: observability.NewLogger(),
	}

	// Test cases
	testCases := []struct {
		name        string
		input       interface{}
		expectError bool
		checkOutput func(t *testing.T, output []byte)
	}{
		{
			name:        "nil input",
			input:       nil,
			expectError: false,
			checkOutput: func(t *testing.T, output []byte) {
				assert.Equal(t, "null", string(output))
			},
		},
		{
			name:        "simple string",
			input:       "test string",
			expectError: false,
			checkOutput: func(t *testing.T, output []byte) {
				assert.Equal(t, "\"test string\"", string(output))
			},
		},
		{
			name:        "map input",
			input:       map[string]interface{}{"key": "value", "number": 123},
			expectError: false,
			checkOutput: func(t *testing.T, output []byte) {
				// Parse back to verify content
				var result map[string]interface{}
				err := json.Unmarshal(output, &result)
				require.NoError(t, err)
				
				assert.Equal(t, "value", result["key"])
				assert.Equal(t, float64(123), result["number"])
			},
		},
		{
			name: "struct input",
			input: struct {
				Name  string
				Count int
				Tags  []string
			}{
				Name:  "test",
				Count: 42,
				Tags:  []string{"tag1", "tag2"},
			},
			expectError: false,
			checkOutput: func(t *testing.T, output []byte) {
				// Verify structure
				var result struct {
					Name  string
					Count int
					Tags  []string
				}
				err := json.Unmarshal(output, &result)
				require.NoError(t, err)
				
				assert.Equal(t, "test", result.Name)
				assert.Equal(t, 42, result.Count)
				assert.Equal(t, []string{"tag1", "tag2"}, result.Tags)
			},
		},
		{
			name: "complex nested structure",
			input: map[string]interface{}{
				"data": map[string]interface{}{
					"user": struct {
						ID   int
						Name string
					}{
						ID:   1,
						Name: "John",
					},
					"items": []interface{}{1, "two", true},
				},
				"success": true,
			},
			expectError: false,
			checkOutput: func(t *testing.T, output []byte) {
				// Basic validation of JSON format
				var result map[string]interface{}
				err := json.Unmarshal(output, &result)
				require.NoError(t, err)
				
				assert.True(t, result["success"].(bool))
				assert.NotNil(t, result["data"])
			},
		},
		{
			name: "circular reference - should fail",
			input: func() interface{} {
				a := make(map[string]interface{})
				b := make(map[string]interface{})
				a["b"] = b
				b["a"] = a
				return a
			}(),
			expectError: true,
			checkOutput: func(t *testing.T, output []byte) {
				// Should not reach here, but handle the case
				assert.Nil(t, output)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := bridge.safeJSONMarshal(tc.input)
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tc.checkOutput(t, output)
			}
		})
	}
}

func TestHelperMethods(t *testing.T) {
	// Test createOperationContextItem and createEventContextItem
	bridge := &ContextBridge{
		logger: observability.NewLogger(),
	}
	
	// Test createOperationContextItem with success
	item := bridge.createOperationContextItem(
		"github", 
		"get_repo", 
		map[string]string{"repo": "test-repo"}, 
		map[string]interface{}{"name": "test-repo", "stars": 100}, 
		nil,
	)
	
	assert.Equal(t, "tool", item.Role)
	assert.Contains(t, item.Content, "Operation: get_repo")
	assert.Contains(t, item.Content, "Adapter: github")
	assert.Contains(t, item.Content, "Status: success")
	assert.Equal(t, "github", item.Metadata["adapter"])
	assert.Equal(t, "get_repo", item.Metadata["operation"])
	assert.Equal(t, "success", item.Metadata["status"])
	assert.NotZero(t, item.Tokens)
	
	// Test createOperationContextItem with error
	testErr := errors.New("not found")
	item = bridge.createOperationContextItem(
		"github", 
		"get_repo", 
		map[string]string{"repo": "missing-repo"}, 
		nil, 
		testErr,
	)
	
	assert.Equal(t, "tool", item.Role)
	assert.Contains(t, item.Content, "Status: failure")
	assert.Contains(t, item.Content, "Error: not found")
	assert.Equal(t, "failure", item.Metadata["status"])
	assert.Equal(t, "not found", item.Metadata["error"])
	
	// Test createEventContextItem
	now := time.Now()
	event := &events.AdapterEvent{
		ID:          "event-123",
		AdapterType: "github",
		EventType:   events.EventTypeWebhookReceived,
		Timestamp:   now,
		Payload:     map[string]interface{}{"action": "push", "repository": "test-repo"},
		Metadata:    map[string]interface{}{"branch": "main"},
	}
	
	item = bridge.createEventContextItem(event)
	
	assert.Equal(t, "event", item.Role)
	assert.Contains(t, item.Content, "Event: webhook.received")
	assert.Contains(t, item.Content, "Adapter: github")
	assert.Equal(t, now, item.Timestamp)
	assert.Equal(t, "github", item.Metadata["adapter"])
	assert.Equal(t, "webhook.received", item.Metadata["eventType"])
	assert.Equal(t, "event-123", item.Metadata["eventId"])
	assert.NotZero(t, item.Tokens)
}

func TestContextBridgeEndToEnd(t *testing.T) {
	// Create a more realistic end-to-end test
	ctx := context.Background()
	mockContextManager := new(MockContextManager)
	mockEventBus := new(MockEventBus)
	logger := observability.NewLogger()
	
	bridge := NewContextBridge(mockContextManager, logger, mockEventBus)
	
	// Mock for operation recording flow
	existingContext := &mcp.Context{
		ID:            "context-e2e",
		AgentID:       "agent-123",
		ModelID:       "model-456",
		Content:       []mcp.ContextItem{},
		CurrentTokens: 0,
		MaxTokens:     10000,
	}
	
	// Set up the initial context
	mockContextManager.On("GetContext", mock.Anything, "context-e2e").Return(existingContext, nil)
	
	// The update call should add a content item
	updatedContext := &mcp.Context{
		ID:            "context-e2e",
		AgentID:       "agent-123",
		ModelID:       "model-456",
		Content:       []mcp.ContextItem{}, // Will be updated
		CurrentTokens: 0,                  // Will be updated
		MaxTokens:     10000,
	}
	
	mockContextManager.On("UpdateContext", 
		mock.Anything, 
		"context-e2e", 
		mock.MatchedBy(func(c *mcp.Context) bool {
			// Should have added one item
			return len(c.Content) == 1
		}),
		mock.AnythingOfType("*mcp.ContextUpdateOptions")).Run(func(args mock.Arguments) {
			// Update the context for subsequent calls
			updatedContext = args.Get(2).(*mcp.Context)
		}).Return(updatedContext, nil)
	
	// Record an operation
	err := bridge.RecordOperationInContext(
		ctx,
		"context-e2e",
		"github",
		"list_repos",
		map[string]string{"owner": "octocat"},
		[]map[string]interface{}{
			{"name": "repo1", "stars": 100},
			{"name": "repo2", "stars": 200},
		},
		nil,
	)
	
	require.NoError(t, err)
	assert.Len(t, updatedContext.Content, 1)
	assert.Greater(t, updatedContext.CurrentTokens, 0)
	
	// Now set up for a second operation that builds on the first
	mockContextManager.On("GetContext", mock.Anything, "context-e2e").Return(updatedContext, nil)
	mockContextManager.On("UpdateContext", 
		mock.Anything, 
		"context-e2e", 
		mock.MatchedBy(func(c *mcp.Context) bool {
			// Should now have two items
			return len(c.Content) == 2
		}),
		mock.AnythingOfType("*mcp.ContextUpdateOptions")).Return(updatedContext, nil)
	
	// Record a second operation
	err = bridge.RecordOperationInContext(
		ctx,
		"context-e2e",
		"github",
		"get_issues",
		map[string]string{"repo": "repo1"},
		[]map[string]interface{}{
			{"number": 1, "title": "Bug report"},
			{"number": 2, "title": "Feature request"},
		},
		nil,
	)
	
	require.NoError(t, err)
	mockContextManager.AssertExpectations(t)
}
