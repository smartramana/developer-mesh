package collaboration

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/pkg/collaboration"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/postgres"
	"github.com/developer-mesh/developer-mesh/pkg/services"
	"github.com/developer-mesh/developer-mesh/test/integration/shared"
)

// MultiAgentIntegrationSuite tests multi-agent collaboration scenarios
type MultiAgentIntegrationSuite struct {
	suite.Suite
	ctx              context.Context
	cancel           context.CancelFunc
	db               *sql.DB
	taskService      services.TaskService
	workflowService  services.WorkflowService
	workspaceService services.WorkspaceService
	documentService  services.DocumentService
	logger           observability.Logger
	tenantID         uuid.UUID
	agents           map[string]uuid.UUID // agent name -> ID mapping
}

// SetupSuite runs once before all tests
func (s *MultiAgentIntegrationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.logger = observability.NewLogger("multi-agent-test")
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

	// Create repositories with proper parameters
	metrics := observability.NewNoOpMetricsClient()
	taskRepo := postgres.NewTaskRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan, metrics)
	workflowRepo := postgres.NewWorkflowRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan, metrics)
	workspaceRepo := postgres.NewWorkspaceRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan)
	documentRepo := postgres.NewDocumentRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan)

	// Create service config
	serviceConfig := services.ServiceConfig{
		Logger:  s.logger,
		Metrics: observability.NewNoOpMetricsClient(),
		Tracer:  observability.NoopStartSpan,
	}

	// Create notification service
	notificationService := services.NewNotificationService(serviceConfig)

	// Create agent repository and service
	agentRepo := repository.NewAgentRepository(sqlxDB)
	agentService := services.NewAgentService(serviceConfig, agentRepo)

	// Create services with proper constructor signatures
	s.taskService = services.NewTaskService(serviceConfig, taskRepo, nil, nil)
	s.workflowService = services.NewWorkflowService(serviceConfig, workflowRepo, s.taskService, agentService, notificationService)
	s.documentService = services.NewDocumentService(serviceConfig, documentRepo, mockCache)
	s.workspaceService = services.NewWorkspaceService(serviceConfig, workspaceRepo, documentRepo, mockCache)

	// Create test agents
	s.agents = map[string]uuid.UUID{
		"coordinator": uuid.New(),
		"developer1":  uuid.New(),
		"developer2":  uuid.New(),
		"tester1":     uuid.New(),
		"tester2":     uuid.New(),
		"reviewer":    uuid.New(),
		"documenter":  uuid.New(),
	}
}

// TearDownSuite runs once after all tests
func (s *MultiAgentIntegrationSuite) TearDownSuite() {
	s.cancel()
	if s.db != nil {
		// Clean up test data
		_ = shared.CleanupTestData(s.db, s.tenantID)
		_ = s.db.Close()
	}
}

// SetupTest runs before each test
func (s *MultiAgentIntegrationSuite) SetupTest() {
	// Clean up any data from previous test
	_ = shared.CleanupTestData(s.db, s.tenantID)
}

// TestMultiAgentTaskDelegation tests complex task delegation scenarios
func (s *MultiAgentIntegrationSuite) TestMultiAgentTaskDelegation() {
	// Coordinator creates a complex task
	mainTask := &models.Task{
		TenantID:    s.tenantID,
		Type:        "feature_development",
		Status:      models.TaskStatusPending,
		Priority:    models.TaskPriorityHigh,
		CreatedBy:   s.agents["coordinator"].String(),
		Title:       "Implement User Authentication Feature",
		Description: "Complete implementation including backend, frontend, and tests",
		Parameters: models.JSONMap{
			"feature_id":      "AUTH-001",
			"estimated_hours": 40,
			"requirements": []string{
				"OAuth 2.0 support",
				"Multi-factor authentication",
				"Session management",
				"Password reset flow",
			},
		},
		MaxRetries:     2,
		TimeoutSeconds: 172800, // 48 hours
	}

	err := s.taskService.Create(s.ctx, mainTask, uuid.New().String())
	require.NoError(s.T(), err)

	// Assign to developer1
	err = s.taskService.AssignTask(s.ctx, mainTask.ID, s.agents["developer1"].String())
	require.NoError(s.T(), err)

	// Developer1 accepts and starts the task
	err = s.taskService.AcceptTask(s.ctx, mainTask.ID, s.agents["developer1"].String())
	require.NoError(s.T(), err)
	err = s.taskService.StartTask(s.ctx, mainTask.ID, s.agents["developer1"].String())
	require.NoError(s.T(), err)

	// Developer1 creates subtasks for different components
	subtasks := []models.Task{
		{
			TenantID:     s.tenantID,
			Type:         "backend_development",
			Title:        "Implement OAuth backend",
			CreatedBy:    s.agents["developer1"].String(),
			ParentTaskID: &mainTask.ID,
		},
		{
			TenantID:     s.tenantID,
			Type:         "frontend_development",
			Title:        "Create login UI components",
			CreatedBy:    s.agents["developer1"].String(),
			ParentTaskID: &mainTask.ID,
		},
		{
			TenantID:     s.tenantID,
			Type:         "testing",
			Title:        "Write authentication tests",
			CreatedBy:    s.agents["developer1"].String(),
			ParentTaskID: &mainTask.ID,
		},
	}

	for i := range subtasks {
		err = s.taskService.Create(s.ctx, &subtasks[i], uuid.New().String())
		require.NoError(s.T(), err)
	}

	// Developer1 delegates frontend task to developer2
	frontendDelegation := &models.TaskDelegation{
		TaskID:         subtasks[1].ID,
		TaskCreatedAt:  subtasks[1].CreatedAt,
		FromAgentID:    s.agents["developer1"].String(),
		ToAgentID:      s.agents["developer2"].String(),
		Reason:         "Developer2 has more frontend expertise",
		DelegationType: models.DelegationManual,
	}
	err = s.taskService.DelegateTask(s.ctx, frontendDelegation)
	require.NoError(s.T(), err)

	// Developer1 delegates testing to tester1
	testDelegation := &models.TaskDelegation{
		TaskID:         subtasks[2].ID,
		TaskCreatedAt:  subtasks[2].CreatedAt,
		FromAgentID:    s.agents["developer1"].String(),
		ToAgentID:      s.agents["tester1"].String(),
		Reason:         "Specialized testing required",
		DelegationType: models.DelegationAutomatic,
	}
	err = s.taskService.DelegateTask(s.ctx, testDelegation)
	require.NoError(s.T(), err)

	// Verify delegations
	task1, err := s.taskService.Get(s.ctx, subtasks[1].ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), task1.AssignedTo)
	assert.Equal(s.T(), s.agents["developer2"].String(), *task1.AssignedTo)

	task2, err := s.taskService.Get(s.ctx, subtasks[2].ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), task2.AssignedTo)
	assert.Equal(s.T(), s.agents["tester1"].String(), *task2.AssignedTo)

	// Tester1 further delegates to tester2 for load testing
	loadTestDelegation := &models.TaskDelegation{
		TaskID:         subtasks[2].ID,
		TaskCreatedAt:  subtasks[2].CreatedAt,
		FromAgentID:    s.agents["tester1"].String(),
		ToAgentID:      s.agents["tester2"].String(),
		Reason:         "Load testing expertise needed",
		DelegationType: models.DelegationLoadBalance,
		Metadata: models.JSONMap{
			"test_type": "performance",
			"load_requirements": map[string]interface{}{
				"concurrent_users": 1000,
				"duration_minutes": 30,
			},
		},
	}
	err = s.taskService.DelegateTask(s.ctx, loadTestDelegation)
	require.NoError(s.T(), err)

	// Complete subtasks
	for i, subtask := range subtasks {
		var agentID string
		switch i {
		case 0:
			agentID = s.agents["developer1"].String()
		case 1:
			agentID = s.agents["developer2"].String()
		case 2:
			agentID = s.agents["tester2"].String()
		}

		// Accept and start
		err = s.taskService.AcceptTask(s.ctx, subtask.ID, agentID)
		require.NoError(s.T(), err)
		err = s.taskService.StartTask(s.ctx, subtask.ID, agentID)
		require.NoError(s.T(), err)

		// Complete with results
		result := map[string]interface{}{
			"completed_by": agentID,
			"status":       "success",
			"metrics": map[string]interface{}{
				"duration_hours": 8 + i*2,
				"quality_score":  0.95 - float64(i)*0.05,
			},
		}
		err = s.taskService.CompleteTask(s.ctx, subtask.ID, agentID, result)
		require.NoError(s.T(), err)
	}

	// Complete main task
	mainResult := map[string]interface{}{
		"feature_complete": true,
		"subtasks_count":   len(subtasks),
		"total_hours":      30,
	}
	err = s.taskService.CompleteTask(s.ctx, mainTask.ID, s.agents["developer1"].String(), mainResult)
	require.NoError(s.T(), err)

	// Verify task tree completion
	taskTree, err := s.taskService.GetTaskTree(s.ctx, mainTask.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), taskTree)
	assert.Equal(s.T(), models.TaskStatusCompleted, taskTree.Root.Status)
	for _, child := range taskTree.Children[mainTask.ID] {
		assert.Equal(s.T(), models.TaskStatusCompleted, child.Status)
	}
}

// TestMultiAgentWorkflowCoordination tests workflow execution across multiple agents
func (s *MultiAgentIntegrationSuite) TestMultiAgentWorkflowCoordination() {
	// Create a code review workflow
	workflow := &models.Workflow{
		TenantID:    s.tenantID,
		Name:        "Code Review Workflow",
		Description: "Multi-stage code review with different reviewers",
		Type:        models.WorkflowTypeCollaborative,
		IsActive:    true,
		CreatedBy:   s.agents["coordinator"].String(),
		Steps: models.WorkflowSteps{
			{
				ID:   "submit_pr",
				Name: "Submit Pull Request",
				Type: "task",
				Config: map[string]interface{}{
					"assignee": s.agents["developer1"].String(),
				},
			},
			{
				ID:   "code_review",
				Name: "Code Review",
				Type: "task",
				Config: map[string]interface{}{
					"assignee":           s.agents["reviewer"].String(),
					"review_type":        "technical",
					"required_approvals": 1,
				},
			},
			{
				ID:   "security_review",
				Name: "Security Review",
				Type: "task",
				Config: map[string]interface{}{
					"assignee":  s.agents["tester1"].String(),
					"scan_type": "security",
					"parallel":  true,
				},
			},
			{
				ID:   "performance_review",
				Name: "Performance Review",
				Type: "task",
				Config: map[string]interface{}{
					"assignee":  s.agents["tester2"].String(),
					"benchmark": true,
					"parallel":  true,
				},
			},
			{
				ID:   "documentation",
				Name: "Update Documentation",
				Type: "task",
				Config: map[string]interface{}{
					"assignee": s.agents["documenter"].String(),
				},
			},
			{
				ID:   "merge",
				Name: "Merge to Main",
				Type: "task",
				Config: map[string]interface{}{
					"assignee":   s.agents["developer1"].String(),
					"auto_merge": false,
				},
			},
		},
		Config: models.JSONMap{
			"coordination": map[string]interface{}{
				"mode":     "distributed",
				"strategy": "consensus",
			},
		},
	}

	err := s.workflowService.CreateWorkflow(s.ctx, workflow)
	require.NoError(s.T(), err)

	// Start workflow execution
	execution, err := s.workflowService.StartWorkflow(s.ctx, workflow.ID, s.agents["developer1"].String(), map[string]interface{}{
		"pr_number":     "PR-123",
		"branch":        "feature/auth",
		"files_changed": 25,
		"lines_added":   450,
		"lines_removed": 120,
	})
	require.NoError(s.T(), err)

	// Execute workflow steps
	// Complete submit_pr
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "submit_pr", map[string]interface{}{
		"pr_url":      "https://github.com/org/repo/pull/123",
		"commit_hash": "abc123def456",
	})
	require.NoError(s.T(), err)

	// Complete code_review
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "code_review", map[string]interface{}{
		"approved": true,
		"comments": 3,
		"suggestions": []string{
			"Consider adding more unit tests",
			"Update error handling in auth module",
		},
	})
	require.NoError(s.T(), err)

	// Complete parallel steps concurrently
	var wg sync.WaitGroup
	results := make(chan error, 2)

	// Security review
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond) // Simulate work
		err := s.workflowService.CompleteStep(s.ctx, execution.ID, "security_review", map[string]interface{}{
			"vulnerabilities": 0,
			"warnings":        2,
			"scan_duration":   "5m30s",
		})
		results <- err
	}()

	// Performance review
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(150 * time.Millisecond) // Simulate work
		err := s.workflowService.CompleteStep(s.ctx, execution.ID, "performance_review", map[string]interface{}{
			"response_time_ms": 45,
			"throughput_rps":   1200,
			"cpu_usage":        35.5,
			"memory_usage_mb":  256,
		})
		results <- err
	}()

	wg.Wait()
	close(results)

	// Check parallel execution results
	for err := range results {
		assert.NoError(s.T(), err)
	}

	// Complete documentation
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "documentation", map[string]interface{}{
		"docs_updated": []string{
			"README.md",
			"docs/authentication.md",
			"API.md",
		},
		"api_version": "v2.1.0",
	})
	require.NoError(s.T(), err)

	// Complete merge
	err = s.workflowService.CompleteStep(s.ctx, execution.ID, "merge", map[string]interface{}{
		"merged_at":    time.Now().Format(time.RFC3339),
		"merge_commit": "xyz789abc123",
		"deployed":     false,
	})
	require.NoError(s.T(), err)

	// Verify workflow completion
	finalExecution, err := s.workflowService.GetExecution(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkflowExecutionStatusCompleted, finalExecution.Status)
	assert.NotNil(s.T(), finalExecution.CompletedAt)

	// Verify all steps completed
	history, err := s.workflowService.GetExecutionHistory(s.ctx, execution.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), history, 6)
	for _, step := range history {
		assert.Equal(s.T(), models.StepExecutionStatusCompleted, step.Status)
	}
}

// TestMultiAgentWorkspaceCollaboration tests real-time collaboration in workspace
func (s *MultiAgentIntegrationSuite) TestMultiAgentWorkspaceCollaboration() {
	// Create collaborative workspace
	workspace := &models.Workspace{
		TenantID:    s.tenantID,
		Name:        "Project Planning Workspace",
		Description: "Collaborative space for project planning",
		OwnerID:     s.agents["coordinator"].String(),
		IsPublic:    false,
		Settings: models.WorkspaceSettings{
			Collaboration: models.CollaborationSettings{
				EnablePresence:     true,
				EnableTypingStatus: true,
				ConflictResolution: "auto_merge",
			},
			Preferences: map[string]interface{}{
				"version_control": true,
			},
		},
		Status: models.WorkspaceStatusActive,
	}

	err := s.workspaceService.Create(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Add all team members
	for name, agentID := range s.agents {
		if name != "coordinator" { // Owner is already a member
			role := models.MemberRole("member")
			if name == "reviewer" {
				role = models.MemberRole("admin")
			}
			member := &models.WorkspaceMember{
				WorkspaceID: workspace.ID,
				AgentID:     agentID.String(),
				Role:        role,
				TenantID:    s.tenantID,
				JoinedAt:    time.Now(),
			}
			err = s.workspaceService.AddMember(s.ctx, member)
			require.NoError(s.T(), err)
		}
	}

	// Create shared document
	workspaceID := workspace.ID
	planDoc := &models.SharedDocument{
		TenantID:    s.tenantID,
		WorkspaceID: workspaceID,
		Title:       "Q1 Project Plan",
		Content:     "# Q1 Project Plan\n\n## Overview\nInitial project planning document.\n\n## Goals\n- TBD\n\n## Timeline\n- TBD\n",
		Type:        string(models.DocumentTypeMarkdown),
		ContentType: "text/markdown",
		CreatedBy:   s.agents["coordinator"].String(),
		Metadata:    models.JSONMap{"tags": []string{"planning", "q1", "draft"}},
	}

	err = s.documentService.Create(s.ctx, planDoc)
	require.NoError(s.T(), err)

	// Simulate concurrent edits from multiple agents
	var editWg sync.WaitGroup
	editErrors := make(chan error, len(s.agents))

	// Each agent makes an edit
	edits := map[string]string{
		"developer1": "\n### Development Tasks\n- Implement authentication system\n- Create API endpoints\n",
		"developer2": "\n### Frontend Tasks\n- Design user interface\n- Implement responsive layout\n",
		"tester1":    "\n### Testing Strategy\n- Unit test coverage > 80%\n- Integration testing plan\n",
		"tester2":    "\n### Performance Goals\n- Response time < 100ms\n- Support 10k concurrent users\n",
		"reviewer":   "\n### Code Review Process\n- All PRs require 2 approvals\n- Security review mandatory\n",
		"documenter": "\n### Documentation Requirements\n- API documentation\n- User guides\n- Architecture diagrams\n",
	}

	for agentName, content := range edits {
		editWg.Add(1)
		go func(name string, editContent string) {
			defer editWg.Done()

			// Get current document
			doc, err := s.documentService.Get(s.ctx, planDoc.ID)
			if err != nil {
				editErrors <- err
				return
			}

			// Apply edit using document operation
			operation := &collaboration.DocumentOperation{
				ID:         uuid.New(),
				DocumentID: planDoc.ID,
				Type:       "insert",
				Path:       "/content",
				Value:      doc.Content + editContent,
				AgentID:    s.agents[name].String(),
				VectorClock: map[string]int{
					s.agents[name].String(): 1,
				},
				AppliedAt: time.Now(),
			}

			err = s.documentService.ApplyOperation(s.ctx, planDoc.ID, operation)
			editErrors <- err

			// Small delay to simulate real work
			time.Sleep(50 * time.Millisecond)
		}(agentName, content)
	}

	editWg.Wait()
	close(editErrors)

	// Check for edit errors
	for err := range editErrors {
		assert.NoError(s.T(), err)
	}

	// Verify final document contains all edits
	finalDoc, err := s.documentService.Get(s.ctx, planDoc.ID)
	require.NoError(s.T(), err)

	// Check that all sections were added
	assert.Contains(s.T(), finalDoc.Content, "Development Tasks")
	assert.Contains(s.T(), finalDoc.Content, "Frontend Tasks")
	assert.Contains(s.T(), finalDoc.Content, "Testing Strategy")
	assert.Contains(s.T(), finalDoc.Content, "Performance Goals")
	assert.Contains(s.T(), finalDoc.Content, "Code Review Process")
	assert.Contains(s.T(), finalDoc.Content, "Documentation Requirements")

	// Test workspace state management
	// Initialize shared counters
	err = s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
		Type: "set",
		Path: "/counters",
		Value: map[string]interface{}{
			"tasks_completed": 0,
			"reviews_pending": 0,
			"bugs_found":      0,
			"docs_updated":    0,
		},
	})
	require.NoError(s.T(), err)

	// Simulate agents updating counters concurrently
	var stateWg sync.WaitGroup
	stateErrors := make(chan error, 20)

	// Each agent updates different counters
	for i := 0; i < 5; i++ {
		// Developer completes tasks
		stateWg.Add(1)
		go func() {
			defer stateWg.Done()
			err := s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
				Type:  "increment",
				Path:  "/counters/tasks_completed",
				Value: 1,
			})
			stateErrors <- err
		}()

		// Reviewer adds pending reviews
		stateWg.Add(1)
		go func() {
			defer stateWg.Done()
			err := s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
				Type:  "increment",
				Path:  "/counters/reviews_pending",
				Value: 2,
			})
			stateErrors <- err
		}()

		// Tester finds bugs
		stateWg.Add(1)
		go func() {
			defer stateWg.Done()
			err := s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
				Type:  "increment",
				Path:  "/counters/bugs_found",
				Value: 3,
			})
			stateErrors <- err
		}()

		// Documenter updates docs
		stateWg.Add(1)
		go func() {
			defer stateWg.Done()
			err := s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
				Type:  "increment",
				Path:  "/counters/docs_updated",
				Value: 1,
			})
			stateErrors <- err
		}()
	}

	stateWg.Wait()
	close(stateErrors)

	// Check for state update errors
	for err := range stateErrors {
		assert.NoError(s.T(), err)
	}

	// Verify final state
	finalState, err := s.workspaceService.GetState(s.ctx, workspace.ID)
	require.NoError(s.T(), err)

	// Access the Data field of WorkspaceState
	stateData := finalState.Data
	counters, ok := stateData["counters"].(map[string]interface{})
	require.True(s.T(), ok, "counters should be a map")

	assert.Equal(s.T(), float64(5), counters["tasks_completed"])
	assert.Equal(s.T(), float64(10), counters["reviews_pending"])
	assert.Equal(s.T(), float64(15), counters["bugs_found"])
	assert.Equal(s.T(), float64(5), counters["docs_updated"])

	// Test member activity tracking
	memberActivity, err := s.workspaceService.GetMemberActivity(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), len(memberActivity), 5) // At least 5 active members
}

// TestMultiAgentFailoverScenario tests agent failure and task reassignment
func (s *MultiAgentIntegrationSuite) TestMultiAgentFailoverScenario() {
	// Create critical task
	criticalTask := &models.Task{
		TenantID:       s.tenantID,
		Type:           "critical_fix",
		Status:         models.TaskStatusPending,
		Priority:       models.TaskPriorityCritical,
		CreatedBy:      s.agents["coordinator"].String(),
		Title:          "Fix Production Issue",
		Description:    "Critical bug affecting production users",
		MaxRetries:     3,
		TimeoutSeconds: 3600, // 1 hour
	}

	err := s.taskService.Create(s.ctx, criticalTask, uuid.New().String())
	require.NoError(s.T(), err)

	// Use AssignTask to assign to developer1
	err = s.taskService.AssignTask(s.ctx, criticalTask.ID, s.agents["developer1"].String())
	require.NoError(s.T(), err)

	// Get assigned agent
	task, err := s.taskService.Get(s.ctx, criticalTask.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), task.AssignedTo)
	originalAgent := *task.AssignedTo

	// Simulate agent failure - reject task
	err = s.taskService.RejectTask(s.ctx, criticalTask.ID, originalAgent, "Agent going offline for maintenance")
	require.NoError(s.T(), err)

	// Task should be back to pending
	task, err = s.taskService.Get(s.ctx, criticalTask.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TaskStatusRejected, task.Status)

	// Trigger failover delegation
	failoverDelegation := &models.TaskDelegation{
		TaskID:         criticalTask.ID,
		TaskCreatedAt:  criticalTask.CreatedAt,
		FromAgentID:    originalAgent,
		ToAgentID:      s.agents["developer2"].String(),
		Reason:         "Original agent unavailable - failover",
		DelegationType: models.DelegationFailover,
		Metadata: models.JSONMap{
			"original_agent": originalAgent,
			"failover_time":  time.Now().Format(time.RFC3339),
		},
	}

	err = s.taskService.DelegateTask(s.ctx, failoverDelegation)
	require.NoError(s.T(), err)

	// New agent accepts and completes task
	err = s.taskService.AcceptTask(s.ctx, criticalTask.ID, s.agents["developer2"].String())
	require.NoError(s.T(), err)
	err = s.taskService.StartTask(s.ctx, criticalTask.ID, s.agents["developer2"].String())
	require.NoError(s.T(), err)

	// Complete with urgency
	err = s.taskService.CompleteTask(s.ctx, criticalTask.ID, s.agents["developer2"].String(), map[string]interface{}{
		"fix_applied":      true,
		"users_affected":   0,
		"downtime_minutes": 0,
		"failover_success": true,
	})
	require.NoError(s.T(), err)

	// Verify task completed despite initial failure
	finalTask, err := s.taskService.Get(s.ctx, criticalTask.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.TaskStatusCompleted, finalTask.Status)
	assert.Equal(s.T(), s.agents["developer2"].String(), *finalTask.AssignedTo)
}

// TestMultiAgentPerformance tests system performance with many concurrent agents
func (s *MultiAgentIntegrationSuite) TestMultiAgentPerformance() {
	// Create a shared workspace for performance testing
	perfWorkspace := &models.Workspace{
		TenantID:    s.tenantID,
		Name:        "Performance Test Workspace",
		Description: "Testing concurrent agent operations",
		OwnerID:     s.agents["coordinator"].String(),
	}

	err := s.workspaceService.Create(s.ctx, perfWorkspace)
	require.NoError(s.T(), err)

	// Create many virtual agents
	numAgents := 50
	virtualAgents := make([]uuid.UUID, numAgents)
	for i := 0; i < numAgents; i++ {
		virtualAgents[i] = uuid.New()
		member := &models.WorkspaceMember{
			WorkspaceID: perfWorkspace.ID,
			AgentID:     virtualAgents[i].String(),
			Role:        models.MemberRole("member"),
			TenantID:    s.tenantID,
			JoinedAt:    time.Now(),
		}
		err = s.workspaceService.AddMember(s.ctx, member)
		require.NoError(s.T(), err)
	}

	// Measure task creation performance
	startTime := time.Now()
	numTasks := 100
	tasks := make([]*models.Task, numTasks)

	for i := 0; i < numTasks; i++ {
		tasks[i] = &models.Task{
			TenantID:  s.tenantID,
			Type:      "performance_test",
			Status:    models.TaskStatusPending,
			Priority:  models.TaskPriorityNormal,
			CreatedBy: virtualAgents[i%numAgents].String(),
			Title:     fmt.Sprintf("Performance Test Task %d", i),
		}
	}

	// Batch create
	err = s.taskService.CreateBatch(s.ctx, tasks)
	require.NoError(s.T(), err)

	taskCreationTime := time.Since(startTime)
	s.T().Logf("Created %d tasks in %v (%.2f tasks/sec)", numTasks, taskCreationTime, float64(numTasks)/taskCreationTime.Seconds())

	// Measure concurrent task assignment and completion
	startTime = time.Now()
	var perfWg sync.WaitGroup
	errors := make(chan error, numTasks)

	for i, task := range tasks {
		perfWg.Add(1)
		go func(idx int, t *models.Task) {
			defer perfWg.Done()

			agentID := virtualAgents[idx%numAgents].String()

			// Assign
			if err := s.taskService.AssignTask(s.ctx, t.ID, agentID); err != nil {
				errors <- err
				return
			}

			// Accept
			if err := s.taskService.AcceptTask(s.ctx, t.ID, agentID); err != nil {
				errors <- err
				return
			}

			// Start
			if err := s.taskService.StartTask(s.ctx, t.ID, agentID); err != nil {
				errors <- err
				return
			}

			// Complete
			if err := s.taskService.CompleteTask(s.ctx, t.ID, agentID, map[string]interface{}{
				"iteration": idx,
				"agent":     agentID,
			}); err != nil {
				errors <- err
				return
			}
		}(i, task)

		// Limit concurrent goroutines
		if (i+1)%10 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	perfWg.Wait()
	close(errors)

	taskProcessingTime := time.Since(startTime)
	s.T().Logf("Processed %d tasks in %v (%.2f tasks/sec)", numTasks, taskProcessingTime, float64(numTasks)/taskProcessingTime.Seconds())

	// Check for errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			s.T().Logf("Error during processing: %v", err)
		}
	}
	assert.Equal(s.T(), 0, errorCount, "No errors should occur during concurrent processing")

	// Verify all tasks completed
	for _, task := range tasks {
		finalTask, err := s.taskService.Get(s.ctx, task.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.TaskStatusCompleted, finalTask.Status)
	}

	// Test concurrent state updates
	startTime = time.Now()

	// Initialize counter
	err = s.workspaceService.UpdateState(s.ctx, perfWorkspace.ID, &models.StateOperation{
		Type:  "set",
		Path:  "/performance_counter",
		Value: 0,
	})
	require.NoError(s.T(), err)

	// Concurrent increments
	numIncrements := 1000
	var stateWg sync.WaitGroup
	stateErrors := make(chan error, numIncrements)

	for i := 0; i < numIncrements; i++ {
		stateWg.Add(1)
		go func() {
			defer stateWg.Done()
			err := s.workspaceService.UpdateState(s.ctx, perfWorkspace.ID, &models.StateOperation{
				Type:  "increment",
				Path:  "/performance_counter",
				Value: 1,
			})
			stateErrors <- err
		}()

		// Throttle to avoid overwhelming the system
		if (i+1)%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	stateWg.Wait()
	close(stateErrors)

	stateUpdateTime := time.Since(startTime)
	s.T().Logf("Performed %d state updates in %v (%.2f updates/sec)", numIncrements, stateUpdateTime, float64(numIncrements)/stateUpdateTime.Seconds())

	// Verify final counter value
	finalState, err := s.workspaceService.GetState(s.ctx, perfWorkspace.ID)
	require.NoError(s.T(), err)

	// Access the Data field of WorkspaceState
	stateData := finalState.Data
	counterValue, ok := stateData["performance_counter"].(float64)
	require.True(s.T(), ok, "performance_counter should be a float64")
	assert.Equal(s.T(), float64(numIncrements), counterValue)
}

// TestMultiAgentIntegration runs the test suite
func TestMultiAgentIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(MultiAgentIntegrationSuite))
}

// mockCache is a simple in-memory cache for testing
type mockCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func (m *mockCache) Get(ctx context.Context, key string, value interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if v, ok := m.data[key]; ok {
		// In real implementation, would unmarshal into value
		_ = v
		return nil
	}
	return cache.ErrNotFound
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.data[key]
	return exists, nil
}

func (m *mockCache) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]interface{})
	return nil
}

func (m *mockCache) Close() error {
	return nil
}
