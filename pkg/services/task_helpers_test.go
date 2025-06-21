package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Mock for task service methods used by helpers
type mockTaskServiceForHelpers struct {
	mock.Mock
}

func (m *mockTaskServiceForHelpers) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskServiceForHelpers) UpdateProgress(ctx context.Context, taskID uuid.UUID, progress int, message string) error {
	args := m.Called(ctx, taskID, progress, message)
	return args.Error(0)
}

func (m *mockTaskServiceForHelpers) Update(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func TestProgressTracker(t *testing.T) {
	t.Run("track and untrack tasks", func(t *testing.T) {
		logger := observability.NewNoopLogger()
		metrics := observability.NewNoOpMetricsClient()

		config := ServiceConfig{
			Logger:  logger,
			Metrics: metrics,
		}

		service := &taskService{
			BaseService: BaseService{
				config: config,
			},
		}

		tracker := NewProgressTracker(service)
		taskID1 := uuid.New()
		taskID2 := uuid.New()

		// Track tasks
		tracker.Track(taskID1)
		tracker.Track(taskID2)

		// Verify tasks are tracked
		count := 0
		tracker.tasks.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		assert.Equal(t, 2, count)

		// Untrack one task
		tracker.Untrack(taskID1)

		// Verify only one task remains
		count = 0
		tracker.tasks.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		assert.Equal(t, 1, count)
	})

	t.Run("check progress updates", func(t *testing.T) {
		mockService := new(mockTaskServiceForHelpers)
		logger := observability.NewNoopLogger()
		metrics := observability.NewNoOpMetricsClient()

		config := ServiceConfig{
			Logger:  logger,
			Metrics: metrics,
		}

		service := &taskService{
			BaseService: BaseService{
				config: config,
			},
		}
		// Inject mocked methods
		service.Get = mockService.Get
		service.UpdateProgress = mockService.UpdateProgress
		service.Update = mockService.Update

		tracker := &ProgressTracker{
			service: service,
		}

		taskID := uuid.New()
		tracker.Track(taskID)

		// Mock task in progress
		inProgressTask := &models.Task{
			ID:             taskID,
			TenantID:       uuid.New(),
			Status:         models.TaskStatusInProgress,
			TimeoutSeconds: 300, // 5 minutes
			Result:         map[string]interface{}{},
		}

		mockService.On("Get", mock.Anything, taskID).Return(inProgressTask, nil)
		mockService.On("UpdateProgress", mock.Anything, taskID, mock.AnythingOfType("int"), mock.AnythingOfType("string")).Return(nil)

		// Check progress
		tracker.checkProgress()

		// Should have called UpdateProgress since task is in progress
		mockService.AssertCalled(t, "UpdateProgress", mock.Anything, taskID, mock.AnythingOfType("int"), mock.AnythingOfType("string"))
	})

	t.Run("timeout handling", func(t *testing.T) {
		mockService := new(mockTaskServiceForHelpers)
		logger := observability.NewNoopLogger()
		metrics := observability.NewNoOpMetricsClient()

		config := ServiceConfig{
			Logger:  logger,
			Metrics: metrics,
		}

		service := &taskService{
			BaseService: BaseService{
				config: config,
			},
		}
		service.Get = mockService.Get
		service.Update = mockService.Update

		tracker := &ProgressTracker{
			service: service,
		}

		taskID := uuid.New()
		// Track task with past time to simulate timeout
		tracker.tasks.Store(taskID, time.Now().Add(-10*time.Minute))

		// Mock task that should timeout
		timeoutTask := &models.Task{
			ID:             taskID,
			TenantID:       uuid.New(),
			Status:         models.TaskStatusInProgress,
			TimeoutSeconds: 60, // 1 minute
		}

		mockService.On("Get", mock.Anything, taskID).Return(timeoutTask, nil)
		mockService.On("Update", mock.Anything, mock.AnythingOfType("*models.Task")).Return(nil)

		// Check progress
		tracker.checkProgress()

		// Should have called Update to mark as timed out
		mockService.AssertCalled(t, "Update", mock.Anything, mock.MatchedBy(func(task *models.Task) bool {
			return task.Status == models.TaskStatusTimeout && task.Error != ""
		}))
	})
}

func TestResultAggregator(t *testing.T) {
	t.Run("majority vote with clear majority", func(t *testing.T) {
		aggregator := NewResultAggregator()

		// Add results - 3 votes for "A", 1 for "B"
		results := map[uuid.UUID]interface{}{
			uuid.New(): "A",
			uuid.New(): "A",
			uuid.New(): "A",
			uuid.New(): "B",
		}

		result := aggregator.majorityVote(results)
		assert.Equal(t, "A", result)
	})

	t.Run("majority vote with no consensus", func(t *testing.T) {
		aggregator := NewResultAggregator()

		// All different results
		results := map[uuid.UUID]interface{}{
			uuid.New(): "A",
			uuid.New(): "B",
			uuid.New(): "C",
			uuid.New(): "D",
		}

		result := aggregator.majorityVote(results)
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.False(t, resultMap["consensus"].(bool))
		assert.Equal(t, "no majority agreement", resultMap["reason"])
	})

	t.Run("majority vote with tie", func(t *testing.T) {
		aggregator := NewResultAggregator()

		// 2 votes each for "A" and "B"
		results := map[uuid.UUID]interface{}{
			uuid.New(): "A",
			uuid.New(): "A",
			uuid.New(): "B",
			uuid.New(): "B",
		}

		result := aggregator.majorityVote(results)
		// Should return one of them deterministically
		assert.True(t, result == "A" || result == "B")
	})

	t.Run("majority vote with complex objects", func(t *testing.T) {
		aggregator := NewResultAggregator()

		// Same map values
		obj1 := map[string]interface{}{"status": "ok", "value": 42}
		obj2 := map[string]interface{}{"status": "ok", "value": 42}
		obj3 := map[string]interface{}{"status": "error", "value": 0}

		results := map[uuid.UUID]interface{}{
			uuid.New(): obj1,
			uuid.New(): obj2,
			uuid.New(): obj3,
		}

		result := aggregator.majorityVote(results)
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ok", resultMap["status"])
		assert.Equal(t, float64(42), resultMap["value"])
	})

	t.Run("aggregate results", func(t *testing.T) {
		aggregator := NewResultAggregator()

		parentTaskID := uuid.New()
		subtask1ID := uuid.New()
		subtask2ID := uuid.New()

		// Add results
		aggregator.AddResult(parentTaskID, subtask1ID, "result1")
		aggregator.AddResult(parentTaskID, subtask2ID, "result2")

		// Get aggregated results
		result := aggregator.GetAggregatedResult(parentTaskID, models.AggregationModeAllComplete)

		// Should have both results
		results, ok := result.(map[uuid.UUID]interface{})
		require.True(t, ok)
		assert.Equal(t, 2, len(results))
		assert.Equal(t, "result1", results[subtask1ID])
		assert.Equal(t, "result2", results[subtask2ID])

		// Clear results
		aggregator.Clear(parentTaskID)

		// Should be empty now
		result = aggregator.GetAggregatedResult(parentTaskID, models.AggregationModeAllComplete)
		assert.Nil(t, result)
	})
}

func TestTaskCreationSaga(t *testing.T) {
	t.Run("publish events with event publisher", func(t *testing.T) {
		logger := observability.NewNoopLogger()
		metrics := observability.NewNoOpMetricsClient()

		// Mock event publisher
		mockEventPublisher := new(mockEventPublisher)

		config := ServiceConfig{
			Logger:  logger,
			Metrics: metrics,
		}

		service := &taskService{
			BaseService: BaseService{
				config:         config,
				eventPublisher: mockEventPublisher,
			},
		}

		dt := &models.DistributedTask{
			Type:        "test",
			Title:       "Test Task",
			Description: "Test Description",
			Priority:    models.TaskPriorityNormal,
		}

		saga := NewTaskCreationSaga(service, dt)

		// Add some events
		task := &models.Task{ID: uuid.New(), Title: "Main Task"}
		saga.events = append(saga.events, models.TaskCreatedEvent{Task: task})

		// Mock publish expectations
		mockEventPublisher.On("Publish", mock.Anything, mock.AnythingOfType("*events.DomainEvent")).Return(nil)

		// Publish events
		err := saga.PublishEvents(context.Background())
		assert.NoError(t, err)

		// Verify event was published
		mockEventPublisher.AssertCalled(t, "Publish", mock.Anything, mock.MatchedBy(func(event interface{}) bool {
			domainEvent, ok := event.(*events.DomainEvent)
			return ok && domainEvent.Type == "task.created"
		}))
	})

	t.Run("publish events without event publisher", func(t *testing.T) {
		logger := observability.NewNoopLogger()
		metrics := observability.NewNoOpMetricsClient()

		config := ServiceConfig{
			Logger:  logger,
			Metrics: metrics,
		}

		service := &taskService{
			BaseService: BaseService{
				config:         config,
				eventPublisher: nil, // No event publisher
			},
		}

		dt := &models.DistributedTask{
			Type: "test",
		}

		saga := NewTaskCreationSaga(service, dt)

		// Add some events
		task := &models.Task{ID: uuid.New()}
		saga.events = append(saga.events, models.TaskCreatedEvent{Task: task})

		// Should not error when no publisher
		err := saga.PublishEvents(context.Background())
		assert.NoError(t, err)
	})
}

// Mock event publisher for testing
type mockEventPublisher struct {
	mock.Mock
}

func (m *mockEventPublisher) Publish(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockEventPublisher) Subscribe(eventType string, handler func(context.Context, interface{}) error) error {
	args := m.Called(eventType, handler)
	return args.Error(0)
}

func (m *mockEventPublisher) Unsubscribe(eventType string, handler func(context.Context, interface{}) error) error {
	args := m.Called(eventType, handler)
	return args.Error(0)
}
