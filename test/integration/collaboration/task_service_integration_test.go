package collaboration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
	"github.com/S-Corkum/devops-mcp/pkg/repository/postgres"
	"github.com/S-Corkum/devops-mcp/pkg/services"
	"github.com/S-Corkum/devops-mcp/test/integration/shared"
)

// TaskServiceIntegrationSuite tests task service with real database
type TaskServiceIntegrationSuite struct {
	suite.Suite
	ctx         context.Context
	cancel      context.CancelFunc
	db          *sql.DB
	taskService services.TaskService
	logger      observability.Logger
	tenantID    uuid.UUID
}

// SetupSuite runs once before all tests
func (s *TaskServiceIntegrationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.logger = observability.NewLogger("task-service-test")
	s.tenantID = uuid.New()

	// Get test database connection
	db, err := shared.GetTestDatabase(s.ctx)
	require.NoError(s.T(), err)
	s.db = db

	// Run migrations
	err = shared.RunMigrations(s.db)
	require.NoError(s.T(), err)

	// Create sqlx DB
	sqlxDB := sqlx.NewDb(db, "postgres")

	// Create mock cache
	mockCache := &mockCache{data: make(map[string]interface{})}

	// Create repositories
	metrics := observability.NewNoOpMetricsClient()
	taskRepo := postgres.NewTaskRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan, metrics)

	// Create service config
	serviceConfig := services.ServiceConfig{
		Logger:  s.logger,
		Metrics: metrics,
		Tracer:  observability.NoopStartSpan,
	}

	// Create service
	s.taskService = services.NewTaskService(serviceConfig, taskRepo, nil, nil)
}

// TearDownSuite runs once after all tests
func (s *TaskServiceIntegrationSuite) TearDownSuite() {
	s.cancel()
	if s.db != nil {
		// Clean up test data
		_ = shared.CleanupTestData(s.db, s.tenantID)
		_ = s.db.Close()
	}
}

// SetupTest runs before each test
func (s *TaskServiceIntegrationSuite) SetupTest() {
	// Clean up any data from previous test
	_ = shared.CleanupTaskData(s.db, s.tenantID)
}

// TestCreateAndRetrieveTask tests basic task creation and retrieval
func (s *TaskServiceIntegrationSuite) TestCreateAndRetrieveTask() {
	// Create task
	task := &models.Task{
		TenantID:    s.tenantID,
		Type:        "test_task",
		Status:      models.TaskStatusPending,
		Priority:    models.TaskPriorityNormal,
		CreatedBy:   "agent1",
		Title:       "Integration Test Task",
		Description: "Task created in integration test",
		Parameters: models.JSONMap{
			"key": "value",
		},
		MaxRetries:     3,
		TimeoutSeconds: 300,
	}

	// Create with idempotency key
	idempotencyKey := uuid.New().String()
	err := s.taskService.Create(s.ctx, task, idempotencyKey)
	require.NoError(s.T(), err)
	assert.NotEqual(s.T(), uuid.Nil, task.ID)

	// Retrieve task
	retrieved, err := s.taskService.Get(s.ctx, task.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), task.Title, retrieved.Title)
	assert.Equal(s.T(), task.Description, retrieved.Description)
	assert.Equal(s.T(), task.CreatedBy, retrieved.CreatedBy)

	// Test idempotency - creating with same key should not error
	task2 := &models.Task{
		TenantID:  s.tenantID,
		Type:      "test_task",
		Title:     "Different Task",
		CreatedBy: "agent1",
	}
	err = s.taskService.Create(s.ctx, task2, idempotencyKey)
	require.NoError(s.T(), err)
	// Should get the same ID as the first task
	assert.Equal(s.T(), task.ID, task2.ID)
}

// TestTaskAssignmentFlow tests the complete task assignment workflow
func (s *TaskServiceIntegrationSuite) TestTaskAssignmentFlow() {
	// Create task
	task := &models.Task{
		TenantID:  s.tenantID,
		Type:      "coding",
		Status:    models.TaskStatusPending,
		Priority:  models.TaskPriorityHigh,
		CreatedBy: "manager",
		Title:     "Implement Feature X",
	}

	err := s.taskService.Create(s.ctx, task, uuid.New().String())
	require.NoError(s.T(), err)

	// Assign task
	agentID := "developer1"
	err = s.taskService.AssignTask(s.ctx, task.ID, agentID)
	require.NoError(s.T(), err)

	// Verify assignment
	retrieved, err := s.taskService.Get(s.ctx, task.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TaskStatusAssigned, retrieved.Status)
	assert.NotNil(s.T(), retrieved.AssignedTo)
	assert.Equal(s.T(), agentID, *retrieved.AssignedTo)
	assert.NotNil(s.T(), retrieved.AssignedAt)

	// Accept task
	err = s.taskService.AcceptTask(s.ctx, task.ID, agentID)
	require.NoError(s.T(), err)

	// Start task
	err = s.taskService.StartTask(s.ctx, task.ID, agentID)
	require.NoError(s.T(), err)

	// Verify task is in progress
	retrieved, err = s.taskService.Get(s.ctx, task.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TaskStatusInProgress, retrieved.Status)
	assert.NotNil(s.T(), retrieved.StartedAt)

	// Complete task
	result := map[string]interface{}{
		"output": "Feature implemented successfully",
		"metrics": map[string]interface{}{
			"lines_of_code": 150,
			"test_coverage": 85.5,
		},
	}
	err = s.taskService.CompleteTask(s.ctx, task.ID, agentID, result)
	require.NoError(s.T(), err)

	// Verify completion
	retrieved, err = s.taskService.Get(s.ctx, task.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TaskStatusCompleted, retrieved.Status)
	assert.NotNil(s.T(), retrieved.CompletedAt)
	assert.NotNil(s.T(), retrieved.Result)
}

// TestTaskDelegation tests task delegation between agents
func (s *TaskServiceIntegrationSuite) TestTaskDelegation() {
	// Create task assigned to agent1
	task := &models.Task{
		TenantID:  s.tenantID,
		Type:      "review",
		Status:    models.TaskStatusPending,
		CreatedBy: "manager",
		Title:     "Code Review Task",
	}

	err := s.taskService.Create(s.ctx, task, uuid.New().String())
	require.NoError(s.T(), err)

	// Assign to first agent
	agent1 := "reviewer1"
	err = s.taskService.AssignTask(s.ctx, task.ID, agent1)
	require.NoError(s.T(), err)

	// Accept and start task
	err = s.taskService.AcceptTask(s.ctx, task.ID, agent1)
	require.NoError(s.T(), err)
	err = s.taskService.StartTask(s.ctx, task.ID, agent1)
	require.NoError(s.T(), err)

	// Delegate to another agent
	delegation := &models.TaskDelegation{
		TaskID:         task.ID,
		TaskCreatedAt:  task.CreatedAt,
		FromAgentID:    agent1,
		ToAgentID:      "reviewer2",
		Reason:         "More expertise in this area",
		DelegationType: models.DelegationManual,
		Metadata: models.JSONMap{
			"priority": "urgent",
		},
	}

	err = s.taskService.DelegateTask(s.ctx, delegation)
	require.NoError(s.T(), err)

	// Verify task is reassigned
	retrieved, err := s.taskService.Get(s.ctx, task.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), retrieved.AssignedTo)
	assert.Equal(s.T(), "reviewer2", *retrieved.AssignedTo)
	assert.Equal(s.T(), models.TaskStatusAssigned, retrieved.Status)
}

// TestConcurrentTaskOperations tests concurrent access to tasks
func (s *TaskServiceIntegrationSuite) TestConcurrentTaskOperations() {
	// Create a task
	task := &models.Task{
		TenantID:  s.tenantID,
		Type:      "concurrent_test",
		Status:    models.TaskStatusPending,
		CreatedBy: "system",
		Title:     "Concurrent Operations Test",
	}

	err := s.taskService.Create(s.ctx, task, uuid.New().String())
	require.NoError(s.T(), err)

	// Run concurrent updates
	numGoroutines := 10
	errors := make(chan error, numGoroutines)
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer func() { done <- true }()

			// Try to update progress
			progress := (idx + 1) * 10
			message := "Progress update from goroutine"
			err := s.taskService.UpdateProgress(s.ctx, task.ID, progress, message)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	close(errors)

	// Check for errors
	for err := range errors {
		assert.NoError(s.T(), err)
	}
}

// TestTaskBatchOperations tests batch task operations
func (s *TaskServiceIntegrationSuite) TestTaskBatchOperations() {
	// Create multiple tasks
	numTasks := 5
	tasks := make([]*models.Task, numTasks)
	taskIDs := make([]uuid.UUID, numTasks)

	for i := 0; i < numTasks; i++ {
		tasks[i] = &models.Task{
			TenantID:  s.tenantID,
			Type:      "batch_test",
			Status:    models.TaskStatusPending,
			Priority:  models.TaskPriorityNormal,
			CreatedBy: "batch_creator",
			Title:     "Batch Task " + string(rune(i)),
		}
	}

	// Create batch
	err := s.taskService.CreateBatch(s.ctx, tasks)
	require.NoError(s.T(), err)

	// Collect IDs
	for i, task := range tasks {
		assert.NotEqual(s.T(), uuid.Nil, task.ID)
		taskIDs[i] = task.ID
	}

	// Get batch
	retrieved, err := s.taskService.GetBatch(s.ctx, taskIDs)
	require.NoError(s.T(), err)
	assert.Len(s.T(), retrieved, numTasks)

	// Verify all tasks were created
	for i, task := range retrieved {
		assert.Equal(s.T(), tasks[i].Title, task.Title)
	}
}

// TestTaskFiltering tests task search and filtering
func (s *TaskServiceIntegrationSuite) TestTaskFiltering() {
	// Create tasks with different properties
	agent1 := "agent1"
	agent2 := "agent2"

	tasks := []*models.Task{
		{
			TenantID:  s.tenantID,
			Type:      "coding",
			Status:    models.TaskStatusPending,
			Priority:  models.TaskPriorityHigh,
			CreatedBy: agent1,
			Title:     "High Priority Coding Task",
			Tags:      []string{"backend", "api"},
		},
		{
			TenantID:   s.tenantID,
			Type:       "testing",
			Status:     models.TaskStatusInProgress,
			Priority:   models.TaskPriorityNormal,
			CreatedBy:  agent2,
			Title:      "Testing Task",
			Tags:       []string{"qa", "automated"},
			AssignedTo: &agent1,
		},
		{
			TenantID:  s.tenantID,
			Type:      "documentation",
			Status:    models.TaskStatusCompleted,
			Priority:  models.TaskPriorityLow,
			CreatedBy: agent1,
			Title:     "Documentation Task",
			Tags:      []string{"docs"},
		},
	}

	// Create all tasks
	for i, task := range tasks {
		err := s.taskService.Create(s.ctx, task, uuid.New().String())
		require.NoError(s.T(), err, "Failed to create task %d", i)
	}

	// Test: Get tasks by agent
	agentTasks, err := s.taskService.GetAgentTasks(s.ctx, agent1, interfaces.TaskFilters{})
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), len(agentTasks), 1)

	// Test: Search tasks
	searchResults, err := s.taskService.SearchTasks(s.ctx, "Coding", interfaces.TaskFilters{
		Types:  []string{"coding"},
		Status: []string{string(models.TaskStatusPending)},
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), searchResults, 1)
	assert.Contains(s.T(), searchResults[0].Title, "Coding")
}

// TestTaskRetryMechanism tests task retry functionality
func (s *TaskServiceIntegrationSuite) TestTaskRetryMechanism() {
	// Create task with retry configuration
	task := &models.Task{
		TenantID:       s.tenantID,
		Type:           "retry_test",
		Status:         models.TaskStatusPending,
		Priority:       models.TaskPriorityNormal,
		CreatedBy:      "system",
		Title:          "Retry Test Task",
		MaxRetries:     3,
		RetryCount:     0,
		TimeoutSeconds: 60,
	}

	err := s.taskService.Create(s.ctx, task, uuid.New().String())
	require.NoError(s.T(), err)

	// Assign and start task
	agentID := "worker1"
	err = s.taskService.AssignTask(s.ctx, task.ID, agentID)
	require.NoError(s.T(), err)
	err = s.taskService.AcceptTask(s.ctx, task.ID, agentID)
	require.NoError(s.T(), err)
	err = s.taskService.StartTask(s.ctx, task.ID, agentID)
	require.NoError(s.T(), err)

	// Fail the task
	err = s.taskService.FailTask(s.ctx, task.ID, agentID, "Temporary failure")
	require.NoError(s.T(), err)

	// Verify task is failed
	retrieved, err := s.taskService.Get(s.ctx, task.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TaskStatusFailed, retrieved.Status)

	// Retry the task
	err = s.taskService.RetryTask(s.ctx, task.ID)
	require.NoError(s.T(), err)

	// Verify task is back to pending with incremented retry count
	retrieved, err = s.taskService.Get(s.ctx, task.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TaskStatusPending, retrieved.Status)
	assert.Equal(s.T(), 1, retrieved.RetryCount)
}

// TestDistributedTaskCreation tests creating tasks with subtasks
func (s *TaskServiceIntegrationSuite) TestDistributedTaskCreation() {
	// Create parent task with subtasks
	parentTask := &models.DistributedTask{
		Task: &models.Task{
			TenantID:    s.tenantID,
			Type:        "parent_task",
			Status:      models.TaskStatusPending,
			Priority:    models.TaskPriorityHigh,
			CreatedBy:   "orchestrator",
			Title:       "Parent Task with Subtasks",
			Description: "This task will be split into subtasks",
		},
		Subtasks: []models.Subtask{
			{
				ID:          "subtask-1",
				Description: "First subtask",
				Parameters:  map[string]interface{}{"step": 1},
			},
			{
				ID:          "subtask-2",
				Description: "Second subtask",
				Parameters:  map[string]interface{}{"step": 2},
			},
			{
				ID:          "subtask-3",
				Description: "Third subtask",
				Parameters:  map[string]interface{}{"step": 3},
			},
		},
		CoordinationMode: "parallel",
		CompletionMode:   "all",
	}

	err := s.taskService.CreateDistributedTask(s.ctx, parentTask)
	require.NoError(s.T(), err)
	assert.NotEqual(s.T(), uuid.Nil, parentTask.ID)

	// Get task tree
	taskTree, err := s.taskService.GetTaskTree(s.ctx, parentTask.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), taskTree)
	assert.Equal(s.T(), parentTask.ID, taskTree.Root.ID)
	assert.Len(s.T(), taskTree.Children[parentTask.ID], 3)

	// Submit result for one subtask
	subtaskID := taskTree.Children[parentTask.ID][0].ID
	result := map[string]interface{}{
		"status": "completed",
		"output": "Subtask 1 completed",
	}
	err = s.taskService.SubmitSubtaskResult(s.ctx, parentTask.ID, subtaskID, result)
	require.NoError(s.T(), err)
}

// TestTaskServiceIntegration runs the test suite
func TestTaskServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(TaskServiceIntegrationSuite))
}
