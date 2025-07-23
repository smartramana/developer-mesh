package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLogger is a mock implementation of observability.Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Fatal(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) WithPrefix(prefix string) observability.Logger {
	args := m.Called(prefix)
	return args.Get(0).(observability.Logger)
}

func (m *MockLogger) With(fields map[string]interface{}) observability.Logger {
	args := m.Called(fields)
	return args.Get(0).(observability.Logger)
}

// MockMetricsClient is a mock implementation of observability.MetricsClient
type MockMetricsClient struct {
	mock.Mock
}

func (m *MockMetricsClient) IncrementCounter(name string, value float64) {
	m.Called(name, value)
}

func (m *MockMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func (m *MockMetricsClient) RecordHistogram(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMetricsClient) RecordGauge(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMetricsClient) RecordCounter(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMetricsClient) RecordEvent(source, eventType string) {
	m.Called(source, eventType)
}

func (m *MockMetricsClient) RecordLatency(operation string, duration time.Duration) {
	m.Called(operation, duration)
}

func (m *MockMetricsClient) RecordDuration(operation string, duration time.Duration) {
	m.Called(operation, duration)
}

func (m *MockMetricsClient) RecordOperation(operationName string, actionName string, success bool, duration float64, tags map[string]string) {
	m.Called(operationName, actionName, success, duration, tags)
}

func (m *MockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
	m.Called(name, duration, labels)
}

func (m *MockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
	m.Called(operation, success, durationSeconds)
}

func (m *MockMetricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
	m.Called(api, operation, success, durationSeconds)
}

func (m *MockMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
	m.Called(operation, success, durationSeconds)
}

func (m *MockMetricsClient) StartTimer(name string, labels map[string]string) func() {
	args := m.Called(name, labels)
	return args.Get(0).(func())
}

func (m *MockMetricsClient) RecordOperationWithContext(ctx context.Context, operation string, f func() error) error {
	args := m.Called(ctx, operation, f)
	return args.Error(0)
}

func (m *MockMetricsClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// setupTestContextManager creates a context manager with mocked dependencies for testing
func setupTestContextManager() (*ContextManager, *MockLogger, *MockMetricsClient) {
	logger := new(MockLogger)
	metrics := new(MockMetricsClient)

	// Set up expected calls for common operations
	logger.On("Debug", mock.Anything, mock.Anything).Return()
	logger.On("Info", mock.Anything, mock.Anything).Return()
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	logger.On("Error", mock.Anything, mock.Anything).Return()
	logger.On("WithPrefix", mock.Anything).Return(logger)

	metrics.On("IncrementCounter", mock.Anything, mock.Anything).Return()
	metrics.On("IncrementCounterWithLabels", mock.Anything, mock.Anything, mock.Anything).Return()
	metrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Return()
	metrics.On("RecordOperation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	metrics.On("RecordOperationWithContext", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Initialize with in-memory storage
	cm := &ContextManager{
		db:      nil, // No DB for unit tests
		cache:   make(map[string]*models.Context),
		mutex:   sync.RWMutex{},
		logger:  logger,
		metrics: metrics,
	}

	return cm, logger, metrics
}

// TestCreateContext tests the CreateContext method
func TestCreateContext(t *testing.T) {
	ctx := context.Background()
	cm, logger, metrics := setupTestContextManager()

	t.Run("Create valid context", func(t *testing.T) {
		// Arrange
		mockContext := &models.Context{
			ID:        "test-id",
			Name:      "Test Context",
			CreatedAt: time.Now(),
		}

		// Act
		result, err := cm.CreateContext(ctx, mockContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, mockContext, result)

		// Check cache
		cachedContext, found := cm.cache[mockContext.ID]
		assert.True(t, found)
		assert.Equal(t, mockContext, cachedContext)

		metrics.AssertCalled(t, "RecordHistogram", MetricContextCreationLatency, mock.Anything, mock.Anything)
		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "create", "status": "success"})
	})

	t.Run("Nil context", func(t *testing.T) {
		// Act
		result, err := cm.CreateContext(ctx, nil)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, "cannot create nil context", err.Error())

		logger.AssertCalled(t, "Error", "Attempted to create nil context", mock.Anything)
		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "create", "status": "error"})
	})

	t.Run("Empty ID", func(t *testing.T) {
		// Arrange
		mockContext := &models.Context{
			ID:        "", // Empty ID
			Name:      "Test Context",
			CreatedAt: time.Now(),
		}

		// Act
		result, err := cm.CreateContext(ctx, mockContext)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.ID, "Should generate an ID when empty ID is provided")
		assert.Equal(t, mockContext.Name, result.Name)

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "create", "status": "success"})
	})
}

// TestGetContext tests the GetContext method
func TestGetContext(t *testing.T) {
	ctx := context.Background()
	cm, logger, metrics := setupTestContextManager()

	t.Run("Get existing context from cache", func(t *testing.T) {
		// Arrange
		mockContext := &models.Context{
			ID:        "test-id",
			Name:      "Test Context",
			CreatedAt: time.Now(),
		}

		// Add to cache
		cm.cache[mockContext.ID] = mockContext

		// Act
		result, err := cm.GetContext(ctx, mockContext.ID)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, mockContext, result)

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "get", "status": "cache_hit"})
	})

	t.Run("Context not found", func(t *testing.T) {
		// Act
		result, err := cm.GetContext(ctx, "nonexistent-id")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "context not found")

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "get", "status": "not_found"})
	})

	t.Run("Empty ID", func(t *testing.T) {
		// Act
		result, err := cm.GetContext(ctx, "")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, "context ID cannot be empty", err.Error())

		logger.AssertCalled(t, "Error", "Attempted to get context with empty ID", mock.Anything)
		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "get", "status": "error"})
	})
}

// TestUpdateContext tests the UpdateContext method
func TestUpdateContext(t *testing.T) {
	ctx := context.Background()
	cm, logger, metrics := setupTestContextManager()
	_ = logger // Suppress unused variable warning

	t.Run("Update existing context", func(t *testing.T) {
		// Arrange
		origContext := &models.Context{
			ID:        "test-id",
			Name:      "Original Context",
			CreatedAt: time.Now().Add(-24 * time.Hour),
		}

		updatedContext := &models.Context{
			ID:        "test-id",
			Name:      "Updated Context",
			CreatedAt: origContext.CreatedAt,
		}

		// Add original to cache
		cm.cache[origContext.ID] = origContext

		// Act
		result, err := cm.UpdateContext(ctx, origContext.ID, updatedContext, nil)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, updatedContext.Name, result.Name)

		// Check cache was updated
		cachedContext := cm.cache[origContext.ID]
		assert.Equal(t, "Updated Context", cachedContext.Name)

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "update", "status": "success"})
	})

	t.Run("Update non-existent context", func(t *testing.T) {
		// Arrange
		updatedContext := &models.Context{
			ID:        "nonexistent-id",
			Name:      "Updated Context",
			CreatedAt: time.Now(),
		}

		// Act
		result, err := cm.UpdateContext(ctx, "nonexistent-id", updatedContext, nil)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "cannot update non-existent context")

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "update", "status": "error"})
	})

	t.Run("Empty ID", func(t *testing.T) {
		// Arrange
		updatedContext := &models.Context{
			ID:        "",
			Name:      "Updated Context",
			CreatedAt: time.Now(),
		}

		// Act
		result, err := cm.UpdateContext(ctx, "", updatedContext, nil)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid parameters for context update")

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "update", "status": "error"})
	})
}

// TestDeleteContext tests the DeleteContext method
func TestDeleteContext(t *testing.T) {
	ctx := context.Background()
	cm, logger, metrics := setupTestContextManager()

	t.Run("Delete existing context", func(t *testing.T) {
		// Arrange
		mockContext := &models.Context{
			ID:        "test-id",
			Name:      "Test Context",
			CreatedAt: time.Now(),
		}

		// Add to cache
		cm.cache[mockContext.ID] = mockContext

		// Act
		err := cm.DeleteContext(ctx, mockContext.ID)

		// Assert
		assert.NoError(t, err)

		// Check cache
		_, found := cm.cache[mockContext.ID]
		assert.False(t, found)

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "delete", "status": "success"})
	})

	t.Run("Empty ID", func(t *testing.T) {
		// Act
		err := cm.DeleteContext(ctx, "")

		// Assert
		assert.Error(t, err)
		assert.Equal(t, "context ID cannot be empty", err.Error())

		logger.AssertCalled(t, "Error", "Attempted to delete context with empty ID", mock.Anything)
		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "delete", "status": "error"})
	})
}

// TestListContexts tests the ListContexts method
func TestListContexts(t *testing.T) {
	ctx := context.Background()
	cm, logger, metrics := setupTestContextManager()
	_ = logger // Suppress unused variable warning

	t.Run("List contexts with no filter", func(t *testing.T) {
		// Arrange
		contexts := []*models.Context{
			{
				ID:        "ctx1",
				Name:      "Context 1",
				AgentID:   "agent1",
				CreatedAt: time.Now(),
			},
			{
				ID:        "ctx2",
				Name:      "Context 2",
				AgentID:   "agent1",
				CreatedAt: time.Now(),
			},
			{
				ID:        "ctx3",
				Name:      "Context 3",
				AgentID:   "agent2",
				CreatedAt: time.Now(),
			},
		}

		// Add to cache
		for _, c := range contexts {
			cm.cache[c.ID] = c
		}

		// Act
		results, err := cm.ListContexts(ctx, "", "", nil)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, results, 3)

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "list", "status": "success"})
	})

	t.Run("List contexts with agent filter", func(t *testing.T) {
		// Act
		results, err := cm.ListContexts(ctx, "agent1", "", nil)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, results, 2)
		for _, c := range results {
			assert.Equal(t, "agent1", c.AgentID)
		}

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "list", "status": "success"})
	})
}

// TestSearchInContext tests the SearchInContext method
func TestSearchInContext(t *testing.T) {
	ctx := context.Background()
	cm, _, metrics := setupTestContextManager()

	t.Run("Search with empty parameters", func(t *testing.T) {
		// Act
		results, err := cm.SearchInContext(ctx, "", "query")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "context ID and query cannot be empty")

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "search", "status": "error"})
	})

	t.Run("Search in non-existent context", func(t *testing.T) {
		// Act
		results, err := cm.SearchInContext(ctx, "nonexistent-id", "query")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "cannot search in non-existent context")

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "search", "status": "error"})
	})
}

// TestSummarizeContext tests the SummarizeContext method
func TestSummarizeContext(t *testing.T) {
	ctx := context.Background()
	cm, _, metrics := setupTestContextManager()

	t.Run("Summarize existing context", func(t *testing.T) {
		// Arrange
		mockTime := time.Date(2025, 5, 19, 10, 0, 0, 0, time.UTC)
		mockContext := &models.Context{
			ID:        "test-id",
			Name:      "Test Context",
			CreatedAt: mockTime,
		}

		// Add to cache
		cm.cache[mockContext.ID] = mockContext

		// Act
		summary, err := cm.SummarizeContext(ctx, mockContext.ID)

		// Assert
		assert.NoError(t, err)
		expectedSummary := "Context ID: test-id\nName: Test Context\nCreated At: " + mockTime.Format(time.RFC3339)
		assert.Equal(t, expectedSummary, summary)

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "summarize", "status": "success"})
	})

	t.Run("Empty ID", func(t *testing.T) {
		// Act
		summary, err := cm.SummarizeContext(ctx, "")

		// Assert
		assert.Error(t, err)
		assert.Equal(t, "", summary)
		assert.Equal(t, "context ID cannot be empty", err.Error())

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "summarize", "status": "error"})
	})

	t.Run("Non-existent context", func(t *testing.T) {
		// Act
		summary, err := cm.SummarizeContext(ctx, "nonexistent-id")

		// Assert
		assert.Error(t, err)
		assert.Equal(t, "", summary)
		assert.Contains(t, err.Error(), "cannot summarize non-existent context")

		metrics.AssertCalled(t, "IncrementCounterWithLabels", MetricContextOperationsTotal, float64(1),
			map[string]string{"operation": "summarize", "status": "error"})
	})
}
