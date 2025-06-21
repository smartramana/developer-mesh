package services

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Mock task service for testing
type mockTaskService struct {
	mock.Mock
}

func (m *mockTaskService) CreateWorkflowTask(ctx context.Context, workflowID, stepID uuid.UUID, params map[string]interface{}) (*models.Task, error) {
	args := m.Called(ctx, workflowID, stepID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func TestExecuteSequentialStep(t *testing.T) {
	tests := []struct {
		name           string
		step           *models.WorkflowStep
		execution      *models.WorkflowExecution
		setupMocks     func(*mockTaskService)
		expectedError  bool
		expectedOutput map[string]interface{}
	}{
		{
			name: "successful sequential execution",
			step: &models.WorkflowStep{
				ID:   "seq-step-1",
				Type: "sequential",
				Config: map[string]interface{}{
					"steps": []interface{}{
						map[string]interface{}{
							"id":      "step1",
							"command": "echo hello",
						},
						map[string]interface{}{
							"id":      "step2",
							"command": "echo world",
						},
					},
				},
			},
			execution: &models.WorkflowExecution{
				ID:          uuid.New(),
				WorkflowID:  uuid.New(),
				TenantID:    uuid.New(),
				InitiatedBy: "user1",
			},
			setupMocks: func(ts *mockTaskService) {
				// Mock task creation for both steps
				task1 := &models.Task{
					ID:     uuid.New(),
					Status: models.TaskStatusPending,
				}
				task2 := &models.Task{
					ID:     uuid.New(),
					Status: models.TaskStatusPending,
				}

				ts.On("CreateWorkflowTask", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(task1, nil).Once()
				ts.On("CreateWorkflowTask", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(task2, nil).Once()

				// Mock task completion
				completedTask1 := &models.Task{
					ID:     task1.ID,
					Status: models.TaskStatusCompleted,
					Result: map[string]interface{}{"output": "hello"},
				}
				completedTask2 := &models.Task{
					ID:     task2.ID,
					Status: models.TaskStatusCompleted,
					Result: map[string]interface{}{"output": "world"},
				}

				ts.On("Get", mock.Anything, task1.ID).Return(completedTask1, nil)
				ts.On("Get", mock.Anything, task2.ID).Return(completedTask2, nil)
			},
			expectedError: false,
			expectedOutput: map[string]interface{}{
				"steps_executed": 2,
			},
		},
		{
			name: "fail fast on error",
			step: &models.WorkflowStep{
				ID:   "seq-step-2",
				Type: "sequential",
				Config: map[string]interface{}{
					"fail_fast": true,
					"steps": []interface{}{
						map[string]interface{}{
							"id":      "step1",
							"command": "false",
						},
						map[string]interface{}{
							"id":      "step2",
							"command": "echo should-not-run",
						},
					},
				},
			},
			execution: &models.WorkflowExecution{
				ID:          uuid.New(),
				WorkflowID:  uuid.New(),
				TenantID:    uuid.New(),
				InitiatedBy: "user1",
			},
			setupMocks: func(ts *mockTaskService) {
				// First task creation succeeds
				task1 := &models.Task{
					ID:     uuid.New(),
					Status: models.TaskStatusPending,
				}
				ts.On("CreateWorkflowTask", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(task1, nil).Once()

				// First task fails
				failedTask := &models.Task{
					ID:     task1.ID,
					Status: models.TaskStatusFailed,
					Error:  "command failed",
				}
				ts.On("Get", mock.Anything, task1.ID).Return(failedTask, nil)
			},
			expectedError: true,
		},
		{
			name: "empty steps validation error",
			step: &models.WorkflowStep{
				ID:   "seq-step-3",
				Type: "sequential",
				Config: map[string]interface{}{
					"steps": []interface{}{},
				},
			},
			execution: &models.WorkflowExecution{
				ID:         uuid.New(),
				WorkflowID: uuid.New(),
				TenantID:   uuid.New(),
			},
			setupMocks:    func(ts *mockTaskService) {},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock services
			mockTS := new(mockTaskService)
			tt.setupMocks(mockTS)

			// Create workflow service with mocks
			logger := observability.NewNoopLogger()
			metrics := observability.NewNoOpMetricsClient()
			tracer := observability.NoopStartSpan

			config := ServiceConfig{
				Logger:  logger,
				Metrics: metrics,
				Tracer:  tracer,
			}

			service := &workflowService{
				BaseService: BaseService{
					config: config,
				},
				taskService: mockTS,
			}

			// Execute test
			ctx := context.Background()
			output, err := service.executeSequentialStep(ctx, tt.execution, tt.step)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectedOutput != nil {
					assert.Equal(t, tt.expectedOutput["steps_executed"], output["steps_executed"])
				}
			}

			mockTS.AssertExpectations(t)
		})
	}
}

func TestExecuteScriptStep(t *testing.T) {
	tests := []struct {
		name           string
		step           *models.WorkflowStep
		execution      *models.WorkflowExecution
		expectedError  bool
		validateOutput func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful bash script execution",
			step: &models.WorkflowStep{
				ID:   "script-step-1",
				Type: "script",
				Config: map[string]interface{}{
					"type":   "bash",
					"script": "echo 'Hello from script'",
				},
			},
			execution: &models.WorkflowExecution{
				ID:          uuid.New(),
				WorkflowID:  uuid.New(),
				TenantID:    uuid.New(),
				InitiatedBy: "user1",
			},
			expectedError: false,
			validateOutput: func(t *testing.T, output map[string]interface{}) {
				assert.Equal(t, 0, output["exit_code"])
				assert.Contains(t, output["stdout"], "Hello from script")
				assert.Equal(t, "bash", output["script_type"])
			},
		},
		{
			name: "script with environment variables",
			step: &models.WorkflowStep{
				ID:   "script-step-2",
				Type: "script",
				Config: map[string]interface{}{
					"type":   "bash",
					"script": "echo $TEST_VAR",
					"env": map[string]interface{}{
						"TEST_VAR": "test-value",
					},
				},
			},
			execution: &models.WorkflowExecution{
				ID:          uuid.New(),
				WorkflowID:  uuid.New(),
				TenantID:    uuid.New(),
				InitiatedBy: "user1",
			},
			expectedError: false,
			validateOutput: func(t *testing.T, output map[string]interface{}) {
				assert.Equal(t, 0, output["exit_code"])
				assert.Contains(t, output["stdout"], "test-value")
			},
		},
		{
			name: "script with non-zero exit code",
			step: &models.WorkflowStep{
				ID:   "script-step-3",
				Type: "script",
				Config: map[string]interface{}{
					"type":          "bash",
					"script":        "exit 42",
					"fail_on_error": true,
				},
			},
			execution: &models.WorkflowExecution{
				ID:          uuid.New(),
				WorkflowID:  uuid.New(),
				TenantID:    uuid.New(),
				InitiatedBy: "user1",
			},
			expectedError: true,
			validateOutput: func(t *testing.T, output map[string]interface{}) {
				assert.Equal(t, 42, output["exit_code"])
			},
		},
		{
			name: "script timeout",
			step: &models.WorkflowStep{
				ID:   "script-step-4",
				Type: "script",
				Config: map[string]interface{}{
					"type":            "bash",
					"script":          "sleep 10",
					"timeout_minutes": 0.01, // 0.6 seconds
				},
			},
			execution: &models.WorkflowExecution{
				ID:          uuid.New(),
				WorkflowID:  uuid.New(),
				TenantID:    uuid.New(),
				InitiatedBy: "user1",
			},
			expectedError: true,
			validateOutput: func(t *testing.T, output map[string]interface{}) {
				assert.Equal(t, -1, output["exit_code"])
				assert.Equal(t, "timeout", output["error"])
			},
		},
		{
			name: "missing script content",
			step: &models.WorkflowStep{
				ID:   "script-step-5",
				Type: "script",
				Config: map[string]interface{}{
					"type": "bash",
				},
			},
			execution: &models.WorkflowExecution{
				ID:         uuid.New(),
				WorkflowID: uuid.New(),
				TenantID:   uuid.New(),
			},
			expectedError: true,
		},
		{
			name: "unsupported script type",
			step: &models.WorkflowStep{
				ID:   "script-step-6",
				Type: "script",
				Config: map[string]interface{}{
					"type":   "perl",
					"script": "print 'Hello'",
				},
			},
			execution: &models.WorkflowExecution{
				ID:         uuid.New(),
				WorkflowID: uuid.New(),
				TenantID:   uuid.New(),
			},
			expectedError: true,
		},
		{
			name: "JSON output parsing",
			step: &models.WorkflowStep{
				ID:   "script-step-7",
				Type: "script",
				Config: map[string]interface{}{
					"type":              "bash",
					"script":            `echo '{"status": "ok", "value": 123}'`,
					"parse_json_output": true,
				},
			},
			execution: &models.WorkflowExecution{
				ID:          uuid.New(),
				WorkflowID:  uuid.New(),
				TenantID:    uuid.New(),
				InitiatedBy: "user1",
			},
			expectedError: false,
			validateOutput: func(t *testing.T, output map[string]interface{}) {
				assert.Equal(t, 0, output["exit_code"])
				parsed, ok := output["parsed_output"].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "ok", parsed["status"])
				assert.Equal(t, float64(123), parsed["value"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip script execution tests in CI environment
			if os.Getenv("CI") == "true" {
				t.Skip("Skipping script execution test in CI")
			}

			// Create workflow service
			logger := observability.NewNoopLogger()
			metrics := observability.NewNoOpMetricsClient()
			tracer := observability.NoopStartSpan

			config := ServiceConfig{
				Logger:  logger,
				Metrics: metrics,
				Tracer:  tracer,
			}

			service := &workflowService{
				BaseService: BaseService{
					config: config,
				},
			}

			// Execute test
			ctx := context.Background()
			output, err := service.executeScriptStep(ctx, tt.execution, tt.step)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.validateOutput != nil && output != nil {
				tt.validateOutput(t, output)
			}
		})
	}
}
