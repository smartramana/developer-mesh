package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/apps/worker/internal/queue"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// MockLogger is a simple mock for the Logger interface
type MockLogger struct {
	InfoCalls  int
	ErrorCalls int
	WarnCalls  int
	DebugCalls int
	FatalCalls int
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.InfoCalls++
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.ErrorCalls++
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.WarnCalls++
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.DebugCalls++
}

func (m *MockLogger) Fatal(msg string, fields map[string]interface{}) {
	m.FatalCalls++
}

func (m *MockLogger) WithPrefix(prefix string) observability.Logger {
	return m
}

// MockMetricsClient is a simple mock for the MetricsClient interface
type MockMetricsClient struct {
	CounterCalls     int
	HistogramCalls   int
	GaugeCalls       int
	EventCalls       int
	LatencyCalls     int
	OperationCalls   int
	LastCounterName  string
	LastCounterValue float64
}

func (m *MockMetricsClient) IncrementCounter(name string, value float64, tags map[string]string) {
	m.CounterCalls++
	m.LastCounterName = name
	m.LastCounterValue = value
}

func (m *MockMetricsClient) RecordHistogram(name string, value float64, tags map[string]string) {
	m.HistogramCalls++
}

func (m *MockMetricsClient) RecordGauge(name string, value float64, tags map[string]string) {
	m.GaugeCalls++
}

func (m *MockMetricsClient) RecordCounter(name string, value float64, tags map[string]string) {
	m.CounterCalls++
}

func (m *MockMetricsClient) RecordEvent(source, eventType string) {
	m.EventCalls++
}

func (m *MockMetricsClient) RecordLatency(operation string, duration time.Duration) {
	m.LatencyCalls++
}

func (m *MockMetricsClient) RecordDuration(operation string, duration time.Duration) {
	m.LatencyCalls++
}

func (m *MockMetricsClient) RecordOperation(operationName string, actionName string, success bool, duration float64, tags map[string]string) {
	m.OperationCalls++
}

func (m *MockMetricsClient) RecordOperationWithContext(ctx context.Context, operation string, f func() error) error {
	m.OperationCalls++
	return f()
}

func (m *MockMetricsClient) Close() error {
	return nil
}

func TestPushProcessor_Process(t *testing.T) {
	// Setup
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsClient{}
	processor := &PushProcessor{
		BaseProcessor: BaseProcessor{
			Logger:  mockLogger,
			Metrics: mockMetrics,
		},
	}

	// Valid push event
	validPayload := map[string]interface{}{
		"ref": "refs/heads/main",
		"head_commit": map[string]interface{}{
			"id":      "abcdef123456",
			"message": "Test commit",
			"author": map[string]interface{}{
				"name": "Test Author",
			},
		},
	}

	event := queue.SQSEvent{
		DeliveryID: "123",
		EventType:  "push",
		RepoName:   "test-repo",
		SenderName: "test-sender",
	}

	// Test
	ctx := context.Background()
	err := processor.Process(ctx, event, validPayload)

	// Assertions
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	if mockLogger.InfoCalls == 0 {
		t.Error("Expected logger.Info to be called")
	}
	if mockMetrics.CounterCalls == 0 {
		t.Error("Expected metrics.IncrementCounter to be called")
	}

	// Invalid push event (missing required fields)
	invalidPayload := map[string]interface{}{}
	err = processor.Process(ctx, event, invalidPayload)
	if err == nil {
		t.Error("Expected error for invalid payload, got nil")
	}
}

func TestPullRequestProcessor_Process(t *testing.T) {
	// Setup
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsClient{}
	processor := &PullRequestProcessor{
		BaseProcessor: BaseProcessor{
			Logger:  mockLogger,
			Metrics: mockMetrics,
		},
	}

	// Valid PR event
	validPayload := map[string]interface{}{
		"action": "opened",
		"pull_request": map[string]interface{}{
			"number": float64(42),
			"title":  "Test PR",
			"state":  "open",
			"user": map[string]interface{}{
				"login": "test-user",
			},
		},
	}

	event := queue.SQSEvent{
		DeliveryID: "456",
		EventType:  "pull_request",
		RepoName:   "test-repo",
		SenderName: "test-sender",
	}

	// Test
	ctx := context.Background()
	err := processor.Process(ctx, event, validPayload)

	// Assertions
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	if mockLogger.InfoCalls == 0 {
		t.Error("Expected logger.Info to be called")
	}
	if mockMetrics.CounterCalls == 0 {
		t.Error("Expected metrics.IncrementCounter to be called")
	}

	// Invalid PR event (missing required fields)
	invalidPayload := map[string]interface{}{
		"action": "opened",
	}
	err = processor.Process(ctx, event, invalidPayload)
	if err == nil {
		t.Error("Expected error for invalid payload, got nil")
	}
}

func TestIssuesProcessor_Process(t *testing.T) {
	// Setup
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsClient{}
	processor := &IssuesProcessor{
		BaseProcessor: BaseProcessor{
			Logger:  mockLogger,
			Metrics: mockMetrics,
		},
	}

	// Valid issue event
	validPayload := map[string]interface{}{
		"action": "opened",
		"issue": map[string]interface{}{
			"number": float64(123),
			"title":  "Test Issue",
			"state":  "open",
		},
	}

	event := queue.SQSEvent{
		DeliveryID: "789",
		EventType:  "issues",
		RepoName:   "test-repo",
		SenderName: "test-sender",
	}

	// Test
	ctx := context.Background()
	err := processor.Process(ctx, event, validPayload)

	// Assertions
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	if mockLogger.InfoCalls == 0 {
		t.Error("Expected logger.Info to be called")
	}
	if mockMetrics.CounterCalls == 0 {
		t.Error("Expected metrics.IncrementCounter to be called")
	}
}

func TestEventProcessor_ProcessEvent(t *testing.T) {
	// Setup
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsClient{}
	processor := NewEventProcessor(mockLogger, mockMetrics)

	// Sample event for push
	pushEvent := queue.SQSEvent{
		DeliveryID: "push-event-123",
		EventType:  "push",
		RepoName:   "test-repo",
		SenderName: "test-sender",
		Payload:    json.RawMessage(`{"ref": "refs/heads/main", "head_commit": {"id": "abc123", "message": "test commit", "author": {"name": "test author"}}}`),
	}

	// Process SQS Event
	ctx := context.Background()
	err := processor.ProcessSQSEvent(ctx, pushEvent)

	// Assertions
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}

	// Test default processor for unknown event type
	unknownEvent := queue.SQSEvent{
		DeliveryID: "unknown-event-456",
		EventType:  "unknown_event_type",
		RepoName:   "test-repo",
		SenderName: "test-sender",
		Payload:    json.RawMessage(`{"foo": "bar"}`),
	}

	// Process unknown event 
	err = processor.ProcessSQSEvent(ctx, unknownEvent)

	// Assertions - should use default processor without error
	if err != nil {
		t.Errorf("Expected nil error from default processor, got: %v", err)
	}
}

func TestProcessSQSEventIntegration(t *testing.T) {
	// This tests the public ProcessSQSEvent function that's used for backward compatibility
	event := queue.SQSEvent{
		DeliveryID: "integration-test-123",
		EventType:  "pull_request",
		RepoName:   "test-repo",
		SenderName: "test-sender",
		Payload:    json.RawMessage(`{"action": "opened", "pull_request": {"number": 42, "title": "Test PR", "state": "open", "user": {"login": "test-user"}}}`),
	}

	// Process the event
	err := ProcessSQSEvent(event)

	// Should process successfully
	if err != nil {
		t.Errorf("Integration test failed: %v", err)
	}
}
