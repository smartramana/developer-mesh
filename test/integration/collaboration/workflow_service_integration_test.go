package collaboration

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/pkg/repository/postgres"
	"github.com/S-Corkum/devops-mcp/pkg/services"
	"github.com/S-Corkum/devops-mcp/test/integration/shared"
)

// WorkflowServiceIntegrationSuite tests workflow service with real database
type WorkflowServiceIntegrationSuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	db              *sql.DB
	workflowService services.WorkflowService
	taskService     services.TaskService
	logger          observability.Logger
	tenantID        uuid.UUID
}

// SetupSuite runs once before all tests
func (s *WorkflowServiceIntegrationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.logger = observability.NewLogger("workflow-service-test")
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
	workflowRepo := postgres.NewWorkflowRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan, metrics)
	taskRepo := postgres.NewTaskRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan, metrics)
	agentRepo := repository.NewAgentRepository(sqlxDB)

	// Create service config
	serviceConfig := services.ServiceConfig{
		Logger:  s.logger,
		Metrics: metrics,
		Tracer:  observability.NoopStartSpan,
	}

	// Create services
	agentService := services.NewAgentService(serviceConfig, agentRepo)
	s.taskService = services.NewTaskService(serviceConfig, taskRepo, nil, nil)
	notificationService := services.NewNotificationService(serviceConfig)
	s.workflowService = services.NewWorkflowService(serviceConfig, workflowRepo, s.taskService, agentService, notificationService)
}

// TearDownSuite runs once after all tests
func (s *WorkflowServiceIntegrationSuite) TearDownSuite() {
	s.cancel()
	if s.db != nil {
		// Clean up test data
		_ = shared.CleanupTestData(s.db, s.tenantID)
		_ = s.db.Close()
	}
}

// SetupTest runs before each test
func (s *WorkflowServiceIntegrationSuite) SetupTest() {
	// Clean up any data from previous test
	_ = shared.CleanupWorkflowData(s.db, s.tenantID)
}

// TestCreateAndRetrieveWorkflow tests basic workflow creation and retrieval
func (s *WorkflowServiceIntegrationSuite) TestCreateAndRetrieveWorkflow() {
	// Create workflow
	workflow := &models.Workflow{
		TenantID:    s.tenantID,
		Name:        "Test Workflow",
		Description: "Integration test workflow",
		Type:        models.WorkflowTypeStandard,
		IsActive:    true,
		CreatedBy:   uuid.New().String(),
		Steps: models.WorkflowSteps{
			{
				ID:   "step1",
				Name: "First Step",
				Type: "task",
			},
			{
				ID:   "step2",
				Name: "Second Step",
				Type: "task",
			},
		},
	}

	err := s.workflowService.CreateWorkflow(s.ctx, workflow)
	require.NoError(s.T(), err)
	assert.NotEqual(s.T(), uuid.Nil, workflow.ID)

	// Retrieve workflow
	retrieved, err := s.workflowService.GetWorkflow(s.ctx, workflow.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), workflow.Name, retrieved.Name)
	assert.Equal(s.T(), workflow.Description, retrieved.Description)
	assert.Equal(s.T(), workflow.Type, retrieved.Type)
	assert.True(s.T(), retrieved.IsActive)
}

// TestWorkflowExecution tests complete workflow execution
func (s *WorkflowServiceIntegrationSuite) TestWorkflowExecution() {
	// Create workflow with multiple steps
	workflow := &models.Workflow{
		TenantID:    s.tenantID,
		Name:        "Execution Test Workflow",
		Description: "Workflow to test execution flow",
		Type:        models.WorkflowTypeCollaborative,
		IsActive:    true,
		CreatedBy:   uuid.New().String(),
		Steps: models.WorkflowSteps{
			{
				ID:     "data_prep",
				Name:   "Data Preparation",
				Type:   "task",
				Config: map[string]interface{}{"timeout": 300},
			},
			{
				ID:     "processing",
				Name:   "Data Processing",
				Type:   "task",
				Config: map[string]interface{}{"parallel": true},
			},
			{
				ID:     "validation",
				Name:   "Result Validation",
				Type:   "task",
				Config: map[string]interface{}{"required": true},
			},
		},
	}

	err := s.workflowService.CreateWorkflow(s.ctx, workflow)
	require.NoError(s.T(), err)

	// Start workflow execution
	initiatorID := uuid.New().String()
	execution, err := s.workflowService.StartWorkflow(s.ctx, workflow.ID, initiatorID, map[string]interface{}{
		"input_data": "test data",
		"mode":       "test",
	})
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), execution)
	assert.Equal(s.T(), models.WorkflowExecutionStatusRunning, execution.Status)

	// Get current step
	currentStep, err := s.workflowService.GetCurrentStep(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), currentStep)
	assert.Equal(s.T(), "data_prep", currentStep.StepName)

	// Complete first step
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, currentStep.StepName, map[string]interface{}{
		"prepared_data": "processed test data",
		"record_count":  100,
	})
	require.NoError(s.T(), err)

	// Verify workflow moved to next step
	currentStep, err = s.workflowService.GetCurrentStep(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "processing", currentStep.StepName)

	// Complete remaining steps
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "processing", map[string]interface{}{
		"processed_records": 100,
		"errors":            0,
	})
	require.NoError(s.T(), err)

	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "validation", map[string]interface{}{
		"validation_passed": true,
		"warnings":          0,
	})
	require.NoError(s.T(), err)

	// Verify workflow is completed
	execution, err = s.workflowService.GetExecution(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkflowExecutionStatusCompleted, execution.Status)
	assert.NotNil(s.T(), execution.CompletedAt)
}

// TestParallelWorkflowExecution tests parallel step execution
func (s *WorkflowServiceIntegrationSuite) TestParallelWorkflowExecution() {
	// Create workflow with parallel steps
	workflow := &models.Workflow{
		TenantID:  s.tenantID,
		Name:      "Parallel Workflow",
		Type:      models.WorkflowTypeCollaborative,
		IsActive:  true,
		CreatedBy: uuid.New().String(),
		Steps: models.WorkflowSteps{
			{
				ID:   "setup",
				Name: "Setup",
				Type: "task",
			},
			{
				ID:     "parallel1",
				Name:   "Parallel Task 1",
				Type:   "task",
				Config: map[string]interface{}{"parallel": true},
			},
			{
				ID:     "parallel2",
				Name:   "Parallel Task 2",
				Type:   "task",
				Config: map[string]interface{}{"parallel": true},
			},
			{
				ID:     "parallel3",
				Name:   "Parallel Task 3",
				Type:   "task",
				Config: map[string]interface{}{"parallel": true},
			},
			{
				ID:   "finalize",
				Name: "Finalize",
				Type: "task",
			},
		},
	}

	err := s.workflowService.CreateWorkflow(s.ctx, workflow)
	require.NoError(s.T(), err)

	// Start execution
	execution, err := s.workflowService.StartWorkflow(s.ctx, workflow.ID, uuid.New().String(), nil)
	require.NoError(s.T(), err)

	// Complete setup step
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "setup", map[string]interface{}{"status": "ready"})
	require.NoError(s.T(), err)

	// Get all current steps (should be all parallel steps)
	steps, err := s.workflowService.GetPendingSteps(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), steps, 3) // All three parallel steps should be pending

	// Complete parallel steps concurrently
	var wg sync.WaitGroup
	errors := make(chan error, 3)

	for _, step := range steps {
		wg.Add(1)
		go func(stepID string) {
			defer wg.Done()
			err := s.workflowService.CompleteStep(s.ctx, execution.ID, stepID, map[string]interface{}{
				"completed": true,
				"step_id":   stepID,
			})
			if err != nil {
				errors <- err
			}
		}(step.StepName)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		assert.NoError(s.T(), err)
	}

	// Verify workflow moved to final step
	currentStep, err := s.workflowService.GetCurrentStep(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "finalize", currentStep.StepName)

	// Complete final step
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "finalize", map[string]interface{}{"status": "done"})
	require.NoError(s.T(), err)

	// Verify completion
	execution, err = s.workflowService.GetExecution(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkflowExecutionStatusCompleted, execution.Status)
}

// TestWorkflowErrorHandling tests workflow error scenarios
func (s *WorkflowServiceIntegrationSuite) TestWorkflowErrorHandling() {
	// Create workflow
	workflow := &models.Workflow{
		TenantID:  s.tenantID,
		Name:      "Error Test Workflow",
		Type:      models.WorkflowTypeStandard,
		IsActive:  true,
		CreatedBy: uuid.New().String(),
		Steps: models.WorkflowSteps{
			{
				ID:     "step1",
				Name:   "Step 1",
				Type:   "task",
				Config: map[string]interface{}{"required": true},
			},
			{
				ID:   "step2",
				Name: "Step 2",
				Type: "task",
			},
		},
	}

	err := s.workflowService.CreateWorkflow(s.ctx, workflow)
	require.NoError(s.T(), err)

	// Start execution
	execution, err := s.workflowService.StartWorkflow(s.ctx, workflow.ID, uuid.New().String(), nil)
	require.NoError(s.T(), err)

	// Fail the first step
	err = s.workflowService.FailStep(s.ctx, execution.ID, "step1", "Test failure", map[string]interface{}{
		"error_code": "TEST_ERROR",
		"details":    "Simulated failure for testing",
	})
	require.NoError(s.T(), err)

	// Verify workflow is failed
	execution, err = s.workflowService.GetExecution(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkflowExecutionStatusFailed, execution.Status)

	// Test workflow pause/resume
	workflow2 := &models.Workflow{
		TenantID:  s.tenantID,
		Name:      "Pause Test Workflow",
		Type:      models.WorkflowTypeStandard,
		IsActive:  true,
		CreatedBy: uuid.New().String(),
		Steps: models.WorkflowSteps{
			{ID: "step1", Name: "Step 1", Type: "task"},
			{ID: "step2", Name: "Step 2", Type: "task"},
		},
	}

	err = s.workflowService.CreateWorkflow(s.ctx, workflow2)
	require.NoError(s.T(), err)

	execution2, err := s.workflowService.StartWorkflow(s.ctx, workflow2.ID, uuid.New().String(), nil)
	require.NoError(s.T(), err)

	// Pause execution
	err = s.workflowService.PauseExecution(s.ctx, execution2.ID, "Testing pause functionality")
	require.NoError(s.T(), err)

	// Verify paused state
	execution2, err = s.workflowService.GetExecution(s.ctx, execution2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkflowExecutionStatusPaused, execution2.Status)

	// Resume execution
	err = s.workflowService.ResumeExecution(s.ctx, execution2.ID)
	require.NoError(s.T(), err)

	// Verify resumed
	execution2, err = s.workflowService.GetExecution(s.ctx, execution2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkflowExecutionStatusRunning, execution2.Status)

	// Cancel execution
	err = s.workflowService.CancelExecution(s.ctx, execution2.ID, "Testing cancellation")
	require.NoError(s.T(), err)

	// Verify cancelled
	execution2, err = s.workflowService.GetExecution(s.ctx, execution2.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkflowExecutionStatusCancelled, execution2.Status)
}

// TestWorkflowRetryMechanism tests workflow retry functionality
func (s *WorkflowServiceIntegrationSuite) TestWorkflowRetryMechanism() {
	// Create workflow with retry configuration
	workflow := &models.Workflow{
		TenantID:  s.tenantID,
		Name:      "Retry Test Workflow",
		Type:      models.WorkflowTypeStandard,
		IsActive:  true,
		CreatedBy: uuid.New().String(),
		Steps: models.WorkflowSteps{
			{
				ID:      "retry_step",
				Name:    "Retry Step",
				Type:    "task",
				Retries: 3,
				Config:  map[string]interface{}{"timeout": 60},
			},
		},
	}

	err := s.workflowService.CreateWorkflow(s.ctx, workflow)
	require.NoError(s.T(), err)

	// Start execution
	execution, err := s.workflowService.StartWorkflow(s.ctx, workflow.ID, uuid.New().String(), nil)
	require.NoError(s.T(), err)

	// Fail step first time
	err = s.workflowService.FailStep(s.ctx, execution.ID, "retry_step", "First failure", nil)
	require.NoError(s.T(), err)

	// Retry the step
	err = s.workflowService.RetryStep(s.ctx, execution.ID, "retry_step")
	require.NoError(s.T(), err)

	// Verify step is retrying
	stepExecution, err := s.workflowService.GetStepExecution(s.ctx, execution.ID, "retry_step")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, stepExecution.RetryCount)
	assert.Equal(s.T(), models.StepExecutionStatusRunning, stepExecution.Status)

	// Complete step on retry
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "retry_step", map[string]interface{}{
		"retry_successful": true,
	})
	require.NoError(s.T(), err)

	// Verify workflow completed
	execution, err = s.workflowService.GetExecution(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkflowExecutionStatusCompleted, execution.Status)
}

// TestWorkflowHistory tests workflow execution history
func (s *WorkflowServiceIntegrationSuite) TestWorkflowHistory() {
	// Create workflow
	workflow := &models.Workflow{
		TenantID:  s.tenantID,
		Name:      "History Test Workflow",
		Type:      models.WorkflowTypeStandard,
		IsActive:  true,
		CreatedBy: uuid.New().String(),
		Steps: models.WorkflowSteps{
			{ID: "step1", Name: "Step 1", Type: "task"},
			{ID: "step2", Name: "Step 2", Type: "task"},
		},
	}

	err := s.workflowService.CreateWorkflow(s.ctx, workflow)
	require.NoError(s.T(), err)

	// Run workflow multiple times
	numExecutions := 3
	executionIDs := make([]uuid.UUID, numExecutions)

	for i := 0; i < numExecutions; i++ {
		execution, err := s.workflowService.StartWorkflow(s.ctx, workflow.ID, uuid.New().String(), map[string]interface{}{
			"run": i + 1,
		})
		require.NoError(s.T(), err)
		executionIDs[i] = execution.ID

		// Complete workflow
		err = s.workflowService.CompleteStep(s.ctx, execution.ID, "step1", nil)
		require.NoError(s.T(), err)
		err = s.workflowService.CompleteStep(s.ctx, execution.ID, "step2", nil)
		require.NoError(s.T(), err)

		// Add small delay between executions
		time.Sleep(10 * time.Millisecond)
	}

	// Get execution history
	history, err := s.workflowService.GetWorkflowHistory(s.ctx, workflow.ID, 10, 0)
	require.NoError(s.T(), err)
	assert.Len(s.T(), history, numExecutions)

	// Verify history is in reverse chronological order
	for i := 0; i < len(history)-1; i++ {
		assert.True(s.T(), history[i].StartedAt.After(history[i+1].StartedAt))
	}
}

// TestWorkflowMetrics tests workflow metrics collection
func (s *WorkflowServiceIntegrationSuite) TestWorkflowMetrics() {
	// Create workflow
	workflow := &models.Workflow{
		TenantID:  s.tenantID,
		Name:      "Metrics Test Workflow",
		Type:      models.WorkflowTypeStandard,
		IsActive:  true,
		CreatedBy: uuid.New().String(),
		Steps: models.WorkflowSteps{
			{ID: "quick_step", Name: "Quick Step", Type: "task"},
			{ID: "slow_step", Name: "Slow Step", Type: "task"},
		},
	}

	err := s.workflowService.CreateWorkflow(s.ctx, workflow)
	require.NoError(s.T(), err)

	// Execute workflow
	execution, err := s.workflowService.StartWorkflow(s.ctx, workflow.ID, uuid.New().String(), nil)
	require.NoError(s.T(), err)

	// Complete steps with different durations
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "quick_step", map[string]interface{}{
		"duration_ms": 100,
	})
	require.NoError(s.T(), err)

	// Simulate slow step
	time.Sleep(200 * time.Millisecond)

	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "slow_step", map[string]interface{}{
		"duration_ms": 2000,
	})
	require.NoError(s.T(), err)

	// Get workflow metrics
	metrics, err := s.workflowService.GetWorkflowMetrics(s.ctx, workflow.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), metrics)
	assert.Equal(s.T(), int64(1), metrics.TotalExecutions)
	assert.Equal(s.T(), int64(1), metrics.SuccessfulRuns)
	assert.Equal(s.T(), int64(0), metrics.FailedRuns)
	assert.Greater(s.T(), metrics.AverageRunTime, time.Duration(0))
}

// TestWorkflowServiceIntegration runs the test suite
func TestWorkflowServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(WorkflowServiceIntegrationSuite))
}
