package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
)

type workflowService struct {
	BaseService

	// Dependencies
	repo         interfaces.WorkflowRepository
	taskService  TaskService
	agentService AgentService
	notifier     NotificationService

	// Caching
	workflowCache   cache.Cache
	executionCache  cache.Cache
	statsCache      cache.Cache

	// Execution management
	executionLocks sync.Map // map[uuid.UUID]*sync.Mutex
	activeExecutions sync.Map // map[uuid.UUID]*models.WorkflowExecution

	// Background workers
	executionMonitor *ExecutionMonitor
}

// NewWorkflowService creates a production-ready workflow service
func NewWorkflowService(
	config ServiceConfig,
	repo interfaces.WorkflowRepository,
	taskService TaskService,
	agentService AgentService,
	notifier NotificationService,
) WorkflowService {
	s := &workflowService{
		BaseService:     NewBaseService(config),
		repo:            repo,
		taskService:     taskService,
		agentService:    agentService,
		notifier:        notifier,
		workflowCache:   cache.NewMemoryCache(1000, 10*time.Minute),
		executionCache:  cache.NewMemoryCache(5000, 5*time.Minute),
		statsCache:      cache.NewMemoryCache(500, 1*time.Minute),
	}

	// Start background workers
	s.executionMonitor = NewExecutionMonitor(s)
	go s.executionMonitor.Start()

	return s
}

// CreateWorkflow creates a new workflow with validation
func (s *workflowService) CreateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.CreateWorkflow")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:create"); err != nil {
		return err
	}

	// Check quota
	if err := s.CheckQuota(ctx, "workflows", 1); err != nil {
		return err
	}

	// Sanitize input
	if err := s.sanitizeWorkflow(workflow); err != nil {
		return errors.Wrap(err, "input sanitization failed")
	}

	// Validate workflow definition
	if err := s.validateWorkflow(ctx, workflow); err != nil {
		return errors.Wrap(err, "workflow validation failed")
	}

	// Check authorization
	if err := s.authorizeWorkflowCreation(ctx, workflow); err != nil {
		return errors.Wrap(err, "authorization failed")
	}

	// Execute in transaction
	err := s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Set metadata
		workflow.ID = uuid.New()
		workflow.TenantID = auth.GetTenantID(ctx)
		workflow.CreatedBy = auth.GetAgentID(ctx)
		workflow.IsActive = true
		workflow.Version = 1

		// Set defaults
		if err := s.applyWorkflowDefaults(ctx, workflow); err != nil {
			return err
		}

		// Create workflow
		if err := s.repo.Create(ctx, workflow); err != nil {
			return errors.Wrap(err, "failed to create workflow")
		}

		// Publish event
		if err := s.PublishEvent(ctx, "WorkflowCreated", workflow, workflow); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Invalidate caches
	s.invalidateWorkflowCaches(ctx, workflow.TenantID)

	return nil
}

// ExecuteWorkflow starts a new workflow execution
func (s *workflowService) ExecuteWorkflow(ctx context.Context, workflowID uuid.UUID, input map[string]interface{}, idempotencyKey string) (*models.WorkflowExecution, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ExecuteWorkflow")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:execute"); err != nil {
		return nil, err
	}

	// Idempotency check
	if idempotencyKey != "" {
		existingID, err := s.checkExecutionIdempotency(ctx, idempotencyKey)
		if err == nil && existingID != uuid.Nil {
			return s.getExecution(ctx, existingID)
		}
	}

	// Get workflow
	workflow, err := s.getWorkflow(ctx, workflowID)
	if err != nil {
		return nil, errors.Wrap(err, "workflow not found")
	}

	// Validate workflow is active
	if !workflow.IsActive {
		return nil, WorkflowNotActiveError{WorkflowID: workflowID}
	}

	// Validate input
	if err := s.validateWorkflowInput(ctx, workflow, input); err != nil {
		return nil, errors.Wrap(err, "input validation failed")
	}

	// Check authorization
	if err := s.authorizeWorkflowExecution(ctx, workflow); err != nil {
		return nil, errors.Wrap(err, "authorization failed")
	}

	// Create execution
	execution := &models.WorkflowExecution{
		ID:          uuid.New(),
		WorkflowID:  workflowID,
		TenantID:    workflow.TenantID,
		InitiatedBy: auth.GetAgentID(ctx),
		Status:      models.WorkflowStatusPending,
		Context:     make(models.JSONMap),
		State:       make(models.JSONMap),
		StartedAt:   time.Now(),
	}

	// Store input in context
	execution.SetInput(input)

	// Initialize step statuses
	workflowSteps := workflow.GetSteps()
	execution.StepStatuses = make(map[string]*models.StepStatus)
	for _, step := range workflowSteps {
		execution.StepStatuses[step.ID] = &models.StepStatus{
			StepID:    step.ID,
			Status:    models.StepStatusPending,
			StartedAt: nil,
		}
	}

	// Start execution in transaction
	err = s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Create execution record
		if err := s.repo.CreateExecution(ctx, execution); err != nil {
			return errors.Wrap(err, "failed to create execution")
		}

		// Store idempotency key
		if idempotencyKey != "" {
			if err := s.storeExecutionIdempotencyKey(ctx, idempotencyKey, execution.ID); err != nil {
				return err
			}
		}

		// Update workflow last executed
		workflow.SetLastExecutedAt(execution.StartedAt)
		if err := s.repo.Update(ctx, workflow); err != nil {
			return err
		}

		// Publish event
		if err := s.PublishEvent(ctx, "WorkflowExecutionStarted", workflow, execution); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Start execution asynchronously
	s.startExecution(ctx, workflow, execution)

	return execution, nil
}

// ExecuteWorkflowStep executes a specific workflow step
func (s *workflowService) ExecuteWorkflowStep(ctx context.Context, executionID uuid.UUID, stepID string) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ExecuteWorkflowStep")
	defer span.End()

	// Get execution with lock
	execution, err := s.getExecutionWithLock(ctx, executionID)
	if err != nil {
		return err
	}

	// Get workflow
	workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
	if err != nil {
		return err
	}

	// Find step
	var step *models.WorkflowStep
	workflowSteps := workflow.GetSteps()
	for _, s := range workflowSteps {
		if s.ID == stepID {
			step = &s
			break
		}
	}

	if step == nil {
		return StepNotFoundError{StepID: stepID}
	}

	// Check dependencies
	if err := s.checkStepDependencies(ctx, execution, step); err != nil {
		return err
	}

	// Update step status
	stepStatus := execution.StepStatuses[stepID]
	stepStatus.Status = models.StepStatusRunning
	stepStatus.StartedAt = timePtr(time.Now())
	stepStatus.RetryCount++

	if err := s.repo.UpdateStepStatus(ctx, executionID, stepID, string(models.StepStatusRunning), nil); err != nil {
		return err
	}

	// Execute step based on type
	var output map[string]interface{}
	var stepErr error

	switch step.Type {
	case "task":
		output, stepErr = s.executeTaskStep(ctx, execution, step)
	case "approval":
		output, stepErr = s.executeApprovalStep(ctx, execution, step)
	case "parallel":
		output, stepErr = s.executeParallelStep(ctx, execution, step)
	case "conditional":
		output, stepErr = s.executeConditionalStep(ctx, execution, step)
	default:
		stepErr = fmt.Errorf("unsupported step type: %s", step.Type)
	}

	// Update step result
	if stepErr != nil {
		stepStatus.Status = models.StepStatusFailed
		stepStatus.Error = stepErr.Error()
	} else {
		stepStatus.Status = models.StepStatusCompleted
		stepStatus.Output = output
	}

	stepStatus.CompletedAt = timePtr(time.Now())

	if err := s.repo.UpdateStepStatus(ctx, executionID, stepID, string(stepStatus.Status), output); err != nil {
		return err
	}

	// Check if workflow is complete
	s.checkWorkflowCompletion(ctx, execution)

	return stepErr
}

// Helper methods

func (s *workflowService) sanitizeWorkflow(workflow *models.Workflow) error {
	if s.config.Sanitizer == nil {
		return nil
	}

	workflow.Name = s.config.Sanitizer.SanitizeString(workflow.Name)
	workflow.Description = s.config.Sanitizer.SanitizeString(workflow.Description)

	// Sanitize step names and descriptions if steps are stored as JSONMap
	// This would need proper implementation based on how steps are stored in JSONMap

	return nil
}

func (s *workflowService) validateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	// Basic validation
	if workflow.Name == "" {
		return ValidationError{Field: "name", Message: "required"}
	}

	if workflow.Type == "" {
		return ValidationError{Field: "type", Message: "required"}
	}

	workflowSteps := workflow.GetSteps()
	if len(workflowSteps) == 0 {
		return ValidationError{Field: "steps", Message: "at least one step required"}
	}

	// Validate steps
	stepIDs := make(map[string]bool)
	for _, step := range workflowSteps {
		if step.ID == "" {
			return ValidationError{Field: "step.id", Message: "required"}
		}

		if stepIDs[step.ID] {
			return ValidationError{Field: "step.id", Message: "duplicate step ID"}
		}
		stepIDs[step.ID] = true

		// Validate dependencies exist
		for _, dep := range step.Dependencies {
			if !stepIDs[dep] {
				return ValidationError{Field: "step.dependencies", Message: fmt.Sprintf("unknown dependency: %s", dep)}
			}
		}
	}

	// Check for circular dependencies
	if err := s.validateNoCycles(workflow); err != nil {
		return err
	}

	return nil
}

func (s *workflowService) validateNoCycles(workflow *models.Workflow) error {
	// Build adjacency list
	adj := make(map[string][]string)
	workflowSteps := workflow.GetSteps()
	for _, step := range workflowSteps {
		adj[step.ID] = step.Dependencies
	}

	// DFS to detect cycles
	visited := make(map[string]int) // 0=unvisited, 1=visiting, 2=visited
	var hasCycle bool

	var dfs func(node string)
	dfs = func(node string) {
		if hasCycle {
			return
		}

		visited[node] = 1
		for _, dep := range adj[node] {
			if visited[dep] == 1 {
				hasCycle = true
				return
			}
			if visited[dep] == 0 {
				dfs(dep)
			}
		}
		visited[node] = 2
	}

	for _, step := range workflowSteps {
		if visited[step.ID] == 0 {
			dfs(step.ID)
		}
	}

	if hasCycle {
		return ValidationError{Field: "steps", Message: "circular dependency detected"}
	}

	return nil
}

func (s *workflowService) authorizeWorkflowCreation(ctx context.Context, workflow *models.Workflow) error {
	if s.config.Authorizer == nil {
		return nil
	}

	decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
		Resource: "workflow",
		Action:   "create",
		Conditions: map[string]interface{}{
			"type": workflow.Type,
		},
	})

	if !decision.Allowed {
		return UnauthorizedError{
			Action: "create workflow",
			Reason: decision.Reason,
		}
	}

	return nil
}

func (s *workflowService) applyWorkflowDefaults(ctx context.Context, workflow *models.Workflow) error {
	if s.config.PolicyManager == nil {
		return nil
	}

	defaults, err := s.config.PolicyManager.GetDefaults(ctx, "workflow", string(workflow.Type))
	if err != nil {
		return err
	}

	// Apply default timeouts - need to implement proper step defaults handling
	// This would require modifying the workflow.Steps JSONMap directly
	// For now, return without modification
	_ = defaults

	return nil
}

func (s *workflowService) getWorkflow(ctx context.Context, id uuid.UUID) (*models.Workflow, error) {
	cacheKey := fmt.Sprintf("workflow:%s", id)
	var cached models.Workflow
	if err := s.workflowCache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	workflow, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	s.workflowCache.Set(ctx, cacheKey, workflow, 10*time.Minute)
	return workflow, nil
}

func (s *workflowService) getExecution(ctx context.Context, id uuid.UUID) (*models.WorkflowExecution, error) {
	cacheKey := fmt.Sprintf("execution:%s", id)
	var cached models.WorkflowExecution
	if err := s.executionCache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	execution, err := s.repo.GetExecution(ctx, id)
	if err != nil {
		return nil, err
	}

	s.executionCache.Set(ctx, cacheKey, execution, 5*time.Minute)
	return execution, nil
}

func (s *workflowService) getExecutionWithLock(ctx context.Context, executionID uuid.UUID) (*models.WorkflowExecution, error) {
	// Get or create lock for execution
	lockI, _ := s.executionLocks.LoadOrStore(executionID, &sync.Mutex{})
	lock := lockI.(*sync.Mutex)

	lock.Lock()
	defer lock.Unlock()

	return s.getExecution(ctx, executionID)
}

func (s *workflowService) checkExecutionIdempotency(ctx context.Context, key string) (uuid.UUID, error) {
	// TODO: Implement idempotency check
	return uuid.Nil, errors.New("not found")
}

func (s *workflowService) storeExecutionIdempotencyKey(ctx context.Context, key string, id uuid.UUID) error {
	// TODO: Implement idempotency key storage
	return nil
}

func (s *workflowService) validateWorkflowInput(ctx context.Context, workflow *models.Workflow, input map[string]interface{}) error {
	// TODO: Implement input validation based on workflow definition
	return nil
}

func (s *workflowService) authorizeWorkflowExecution(ctx context.Context, workflow *models.Workflow) error {
	if s.config.Authorizer == nil {
		return nil
	}

	decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
		Resource: "workflow",
		Action:   "execute",
		Conditions: map[string]interface{}{
			"workflow_id": workflow.ID,
			"type":        workflow.Type,
		},
	})

	if !decision.Allowed {
		return UnauthorizedError{
			Action: "execute workflow",
			Reason: decision.Reason,
		}
	}

	return nil
}

func (s *workflowService) startExecution(ctx context.Context, workflow *models.Workflow, execution *models.WorkflowExecution) {
	// Store active execution
	s.activeExecutions.Store(execution.ID, execution)

	// Start execution in goroutine
	go func() {
		ctx := context.Background() // New context for async execution
		defer s.activeExecutions.Delete(execution.ID)

		// Execute initial steps (those with no dependencies)
		workflowSteps := workflow.GetSteps()
		for _, step := range workflowSteps {
			if len(step.Dependencies) == 0 {
				if err := s.ExecuteWorkflowStep(ctx, execution.ID, step.ID); err != nil {
					s.config.Logger.Error("Failed to execute step", map[string]interface{}{
						"execution_id": execution.ID,
						"step_id":      step.ID,
						"error":        err.Error(),
					})
				}
			}
		}
	}()
}

func (s *workflowService) checkStepDependencies(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) error {
	for _, depID := range step.Dependencies {
		depStatus, exists := execution.StepStatuses[depID]
		if !exists {
			return fmt.Errorf("dependency step %s not found", depID)
		}

		if depStatus.Status != models.StepStatusCompleted {
			return StepDependencyError{
				StepID:       step.ID,
				DependencyID: depID,
				Status:       depStatus.Status,
			}
		}
	}

	return nil
}

func (s *workflowService) executeTaskStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	// Create task for step
	task, err := s.taskService.CreateWorkflowTask(ctx, execution.WorkflowID, uuid.MustParse(step.ID), step.Config)
	if err != nil {
		return nil, err
	}

	// Wait for task completion (with timeout)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(step.TimeoutSeconds)*time.Second)
	defer cancel()

	// Poll for task completion
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("step timeout exceeded")
		case <-ticker.C:
			taskStatus, err := s.taskService.Get(ctx, task.ID)
			if err != nil {
				return nil, err
			}

			switch taskStatus.Status {
			case models.TaskStatusCompleted:
				return map[string]interface{}{
					"task_id": task.ID,
					"result":  taskStatus.Result,
				}, nil
			case models.TaskStatusFailed:
				return nil, fmt.Errorf("task failed: %s", taskStatus.Error)
			}
		}
	}
}

func (s *workflowService) executeApprovalStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	// TODO: Implement approval step execution
	return nil, errors.New("approval steps not implemented")
}

func (s *workflowService) executeParallelStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	// TODO: Implement parallel step execution
	return nil, errors.New("parallel steps not implemented")
}

func (s *workflowService) executeConditionalStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	// TODO: Implement conditional step execution
	return nil, errors.New("conditional steps not implemented")
}

func (s *workflowService) checkWorkflowCompletion(ctx context.Context, execution *models.WorkflowExecution) {
	// Check if all steps are complete
	allComplete := true
	hasFailed := false

	for _, status := range execution.StepStatuses {
		if status.Status == models.StepStatusPending || status.Status == models.StepStatusRunning {
			allComplete = false
			break
		}
		if status.Status == models.StepStatusFailed {
			hasFailed = true
		}
	}

	if !allComplete {
		return
	}

	// Update execution status
	if hasFailed {
		execution.Status = models.WorkflowStatusFailed
	} else {
		execution.Status = models.WorkflowStatusCompleted
	}

	execution.CompletedAt = timePtr(time.Now())

	if err := s.repo.UpdateExecution(ctx, execution); err != nil {
		s.config.Logger.Error("Failed to update execution status", map[string]interface{}{
			"execution_id": execution.ID,
			"error":        err.Error(),
		})
	}

	// Publish completion event
	s.PublishEvent(ctx, "WorkflowExecutionCompleted", execution, execution)

	// Notify interested parties
	s.notifier.NotifyWorkflowCompleted(ctx, execution)
}

func (s *workflowService) invalidateWorkflowCaches(ctx context.Context, tenantID uuid.UUID) {
	// Invalidate tenant-specific caches
	s.workflowCache.Delete(ctx, fmt.Sprintf("tenant:%s:workflows", tenantID))
	s.statsCache.Delete(ctx, fmt.Sprintf("tenant:%s:workflow:stats", tenantID))
}

// Implement remaining interface methods with TODO placeholders

func (s *workflowService) GetWorkflow(ctx context.Context, id uuid.UUID) (*models.Workflow, error) {
	return s.getWorkflow(ctx, id)
}

func (s *workflowService) UpdateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	// TODO: Implement workflow update
	return errors.New("not implemented")
}

func (s *workflowService) DeleteWorkflow(ctx context.Context, id uuid.UUID) error {
	// TODO: Implement workflow deletion
	return errors.New("not implemented")
}

func (s *workflowService) ListWorkflows(ctx context.Context, filters interfaces.WorkflowFilters) ([]*models.Workflow, error) {
	// TODO: Implement workflow listing
	return nil, errors.New("not implemented")
}

func (s *workflowService) SearchWorkflows(ctx context.Context, query string) ([]*models.Workflow, error) {
	// TODO: Implement workflow search
	return nil, errors.New("not implemented")
}

func (s *workflowService) GetExecution(ctx context.Context, executionID uuid.UUID) (*models.WorkflowExecution, error) {
	return s.getExecution(ctx, executionID)
}

func (s *workflowService) ListExecutions(ctx context.Context, workflowID uuid.UUID, filters interfaces.ExecutionFilters) ([]*models.WorkflowExecution, error) {
	// TODO: Implement execution listing
	return nil, errors.New("not implemented")
}

func (s *workflowService) GetExecutionStatus(ctx context.Context, executionID uuid.UUID) (*models.ExecutionStatus, error) {
	// TODO: Implement execution status retrieval
	return nil, errors.New("not implemented")
}

func (s *workflowService) GetExecutionTimeline(ctx context.Context, executionID uuid.UUID) ([]*models.ExecutionEvent, error) {
	// TODO: Implement execution timeline retrieval
	return nil, errors.New("not implemented")
}

func (s *workflowService) UpdateExecution(ctx context.Context, execution *models.WorkflowExecution) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.UpdateExecution")
	defer span.End()

	// Validate execution
	if execution == nil {
		return ValidationError{Field: "execution", Message: "required"}
	}

	if execution.ID == uuid.Nil {
		return ValidationError{Field: "execution.id", Message: "required"}
	}

	// Check authorization
	existingExecution, err := s.getExecution(ctx, execution.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get existing execution")
	}

	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "update",
			Conditions: map[string]interface{}{
				"execution_id": execution.ID,
				"workflow_id":  existingExecution.WorkflowID,
				"tenant_id":    existingExecution.TenantID,
			},
		})

		if !decision.Allowed {
			return UnauthorizedError{
				Action: "update workflow execution",
				Reason: decision.Reason,
			}
		}
	}

	// Execute in transaction
	err = s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Update timestamp
		execution.UpdatedAt = time.Now()

		// Update execution
		if err := s.repo.UpdateExecution(ctx, execution); err != nil {
			return errors.Wrap(err, "failed to update execution")
		}

		// Publish event
		if err := s.PublishEvent(ctx, "WorkflowExecutionUpdated", execution, execution); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Invalidate caches
	s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", execution.ID))

	return nil
}

func (s *workflowService) PauseExecution(ctx context.Context, executionID uuid.UUID, reason string) error {
	// TODO: Implement execution pause
	return errors.New("not implemented")
}

func (s *workflowService) ResumeExecution(ctx context.Context, executionID uuid.UUID) error {
	// TODO: Implement execution resume
	return errors.New("not implemented")
}

func (s *workflowService) CancelExecution(ctx context.Context, executionID uuid.UUID, reason string) error {
	// TODO: Implement execution cancellation
	return errors.New("not implemented")
}

func (s *workflowService) RetryExecution(ctx context.Context, executionID uuid.UUID, fromStep string) error {
	// TODO: Implement execution retry
	return errors.New("not implemented")
}

func (s *workflowService) SubmitApproval(ctx context.Context, executionID uuid.UUID, stepID string, approval *models.ApprovalDecision) error {
	// TODO: Implement approval submission
	return errors.New("not implemented")
}

func (s *workflowService) GetPendingApprovals(ctx context.Context, approverID string) ([]*models.PendingApproval, error) {
	// TODO: Implement pending approvals retrieval
	return nil, errors.New("not implemented")
}

func (s *workflowService) CreateWorkflowTemplate(ctx context.Context, template *models.WorkflowTemplate) error {
	// TODO: Implement template creation
	return errors.New("not implemented")
}

func (s *workflowService) GetWorkflowTemplate(ctx context.Context, templateID uuid.UUID) (*models.WorkflowTemplate, error) {
	// TODO: Implement template retrieval
	return nil, errors.New("not implemented")
}

func (s *workflowService) ListWorkflowTemplates(ctx context.Context) ([]*models.WorkflowTemplate, error) {
	// TODO: Implement template listing
	return nil, errors.New("not implemented")
}

func (s *workflowService) CreateFromTemplate(ctx context.Context, templateID uuid.UUID, params map[string]interface{}) (*models.Workflow, error) {
	// TODO: Implement workflow creation from template
	return nil, errors.New("not implemented")
}

func (s *workflowService) ValidateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	return s.validateWorkflow(ctx, workflow)
}

func (s *workflowService) SimulateWorkflow(ctx context.Context, workflow *models.Workflow, input map[string]interface{}) (*models.SimulationResult, error) {
	// TODO: Implement workflow simulation
	return nil, errors.New("not implemented")
}

func (s *workflowService) GetWorkflowStats(ctx context.Context, workflowID uuid.UUID, period time.Duration) (*interfaces.WorkflowStats, error) {
	// TODO: Implement workflow statistics retrieval
	return nil, errors.New("not implemented")
}

func (s *workflowService) GenerateWorkflowReport(ctx context.Context, filters interfaces.WorkflowFilters, format string) ([]byte, error) {
	// TODO: Implement workflow report generation
	return nil, errors.New("not implemented")
}

func (s *workflowService) ArchiveCompletedExecutions(ctx context.Context, before time.Time) (int64, error) {
	// TODO: Implement execution archival
	return 0, errors.New("not implemented")
}

func (s *workflowService) CreateBranchingPath(ctx context.Context, executionID uuid.UUID, branchPoint string, conditions map[string]interface{}) error {
	// TODO: Implement branching path creation
	return errors.New("not implemented")
}

func (s *workflowService) MergeBranchingPaths(ctx context.Context, executionID uuid.UUID, branchIDs []string) error {
	// TODO: Implement branching path merge
	return errors.New("not implemented")
}

func (s *workflowService) CreateCompensation(ctx context.Context, executionID uuid.UUID, failedStep string, compensation *models.CompensationAction) error {
	// TODO: Implement compensation creation
	return errors.New("not implemented")
}

func (s *workflowService) ExecuteCompensation(ctx context.Context, executionID uuid.UUID) error {
	// TODO: Implement compensation execution
	return errors.New("not implemented")
}

// CreateExecution creates a new workflow execution record
func (s *workflowService) CreateExecution(ctx context.Context, execution *models.WorkflowExecution) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.CreateExecution")
	defer span.End()

	// Validate execution
	if execution.WorkflowID == uuid.Nil {
		return fmt.Errorf("workflow ID is required")
	}
	if execution.ID == uuid.Nil {
		execution.ID = uuid.New()
	}

	// Set defaults
	if execution.Status == "" {
		execution.Status = models.WorkflowStatusPending
	}
	if execution.StartedAt.IsZero() {
		execution.StartedAt = time.Now()
	}
	if execution.Context == nil {
		execution.Context = make(models.JSONMap)
	}
	if execution.State == nil {
		execution.State = make(models.JSONMap)
	}

	// Store in repository
	if err := s.repo.CreateExecution(ctx, execution); err != nil {
		return errors.Wrap(err, "failed to create execution")
	}

	// Cache execution
	s.executionCache.Set(ctx, execution.ID.String(), execution, 5*time.Minute)
	
	// Store in active executions
	s.activeExecutions.Store(execution.ID, execution)

	// Publish event
	s.PublishEvent(ctx, "ExecutionCreated", execution, execution)

	return nil
}

// StartWorkflow starts a new workflow execution
func (s *workflowService) StartWorkflow(ctx context.Context, workflowID uuid.UUID, initiatorID string, input map[string]interface{}) (*models.WorkflowExecution, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.StartWorkflow")
	defer span.End()

	// Get workflow
	workflow, err := s.getWorkflow(ctx, workflowID)
	if err != nil {
		return nil, errors.Wrap(err, "workflow not found")
	}

	// Create execution
	execution := &models.WorkflowExecution{
		ID:          uuid.New(),
		WorkflowID:  workflowID,
		TenantID:    workflow.TenantID,
		InitiatedBy: initiatorID,
		Status:      models.WorkflowExecutionStatusRunning,
		Context:     make(models.JSONMap),
		State:       make(models.JSONMap),
		StartedAt:   time.Now(),
	}

	// Store input
	if input != nil {
		execution.Context["input"] = input
	}

	// Initialize step statuses
	workflowSteps := workflow.GetSteps()
	execution.StepStatuses = make(map[string]*models.StepStatus)
	for _, step := range workflowSteps {
		execution.StepStatuses[step.ID] = &models.StepStatus{
			StepID:    step.ID,
			Status:    models.StepStatusPending,
			StartedAt: nil,
		}
	}

	// Create execution record
	if err := s.CreateExecution(ctx, execution); err != nil {
		return nil, err
	}

	// Start first step
	if len(workflowSteps) > 0 {
		firstStep := workflowSteps[0]
		execution.CurrentStepID = firstStep.ID
		execution.StepStatuses[firstStep.ID].Status = models.StepStatusRunning
		execution.StepStatuses[firstStep.ID].StartedAt = &execution.StartedAt
		
		if err := s.UpdateExecution(ctx, execution); err != nil {
			return nil, err
		}
	}

	// Notify
	s.notifier.NotifyWorkflowStarted(ctx, execution)

	return execution, nil
}

// CompleteStep marks a workflow step as completed
func (s *workflowService) CompleteStep(ctx context.Context, executionID uuid.UUID, stepID string, output map[string]interface{}) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.CompleteStep")
	defer span.End()

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Validate step
	stepStatus, exists := execution.StepStatuses[stepID]
	if !exists {
		return fmt.Errorf("step %s not found in execution", stepID)
	}

	if stepStatus.Status != models.StepStatusRunning {
		return fmt.Errorf("step %s is not running (status: %s)", stepID, stepStatus.Status)
	}

	// Update step status
	now := time.Now()
	stepStatus.Status = models.StepStatusCompleted
	stepStatus.CompletedAt = &now
	if output != nil {
		stepStatus.Output = output
	}

	// Find next step
	workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
	if err != nil {
		return err
	}

	workflowSteps := workflow.GetSteps()
	var currentStepIndex int
	for i, step := range workflowSteps {
		if step.ID == stepID {
			currentStepIndex = i
			break
		}
	}

	// Move to next step or complete workflow
	if currentStepIndex < len(workflowSteps)-1 {
		nextStep := workflowSteps[currentStepIndex+1]
		execution.CurrentStepID = nextStep.ID
		execution.StepStatuses[nextStep.ID].Status = models.StepStatusRunning
		execution.StepStatuses[nextStep.ID].StartedAt = &now
		
		// Notify step completed
		s.notifier.NotifyStepCompleted(ctx, executionID, stepID, output)
	} else {
		// All steps completed
		execution.Status = models.WorkflowExecutionStatusCompleted
		execution.CompletedAt = &now
		execution.CurrentStepID = ""
		
		// Notify workflow completed
		s.notifier.NotifyWorkflowCompleted(ctx, execution)
	}

	// Update execution
	if err := s.UpdateExecution(ctx, execution); err != nil {
		return err
	}

	return nil
}

// FailStep marks a workflow step as failed
func (s *workflowService) FailStep(ctx context.Context, executionID uuid.UUID, stepID string, reason string, details map[string]interface{}) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.FailStep")
	defer span.End()

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Update step status
	stepStatus, exists := execution.StepStatuses[stepID]
	if !exists {
		return fmt.Errorf("step %s not found", stepID)
	}

	now := time.Now()
	stepStatus.Status = models.StepStatusFailed
	stepStatus.CompletedAt = &now
	stepStatus.Error = reason
	if details != nil {
		stepStatus.Output = details
	}

	// Mark execution as failed
	execution.Status = models.WorkflowExecutionStatusFailed
	execution.CompletedAt = &now
	execution.Error = reason

	// Update execution
	if err := s.UpdateExecution(ctx, execution); err != nil {
		return err
	}

	// Notify
	s.notifier.NotifyWorkflowFailed(ctx, executionID, reason)

	return nil
}

// RetryStep retries a failed workflow step
func (s *workflowService) RetryStep(ctx context.Context, executionID uuid.UUID, stepID string) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.RetryStep")
	defer span.End()

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Validate step can be retried
	stepStatus, exists := execution.StepStatuses[stepID]
	if !exists {
		return fmt.Errorf("step %s not found", stepID)
	}

	if stepStatus.Status != models.StepStatusFailed {
		return fmt.Errorf("step %s is not failed (status: %s)", stepID, stepStatus.Status)
	}

	// Reset step status
	now := time.Now()
	stepStatus.Status = models.StepStatusRunning
	stepStatus.StartedAt = &now
	stepStatus.CompletedAt = nil
	stepStatus.Error = ""
	stepStatus.RetryCount++

	// Update execution status if needed
	if execution.Status == models.WorkflowExecutionStatusFailed {
		execution.Status = models.WorkflowExecutionStatusRunning
		execution.CompletedAt = nil
		execution.Error = ""
	}

	execution.CurrentStepID = stepID

	// Update execution
	if err := s.UpdateExecution(ctx, execution); err != nil {
		return err
	}

	return nil
}

// GetCurrentStep returns the currently executing step
func (s *workflowService) GetCurrentStep(ctx context.Context, executionID uuid.UUID) (*models.StepExecution, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetCurrentStep")
	defer span.End()

	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	if execution.CurrentStepID == "" {
		return nil, nil
	}

	stepStatus, exists := execution.StepStatuses[execution.CurrentStepID]
	if !exists {
		return nil, fmt.Errorf("current step not found")
	}

	return &models.StepExecution{
		ExecutionID: executionID,
		StepName:    execution.CurrentStepID,
		Status:      stepStatus.Status,
		StartedAt:   getTimeValue(stepStatus.StartedAt),
		CompletedAt: stepStatus.CompletedAt,
		RetryCount:  stepStatus.RetryCount,
		Result:      stepStatus.Output,
		Error:       getStringPtr(stepStatus.Error),
	}, nil
}

// GetPendingSteps returns all pending steps for parallel execution
func (s *workflowService) GetPendingSteps(ctx context.Context, executionID uuid.UUID) ([]*models.StepExecution, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetPendingSteps")
	defer span.End()

	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	var pendingSteps []*models.StepExecution
	for stepID, stepStatus := range execution.StepStatuses {
		if stepStatus.Status == models.StepStatusPending || stepStatus.Status == models.StepStatusRunning {
			pendingSteps = append(pendingSteps, &models.StepExecution{
				ExecutionID: executionID,
				StepName:    stepID,
				Status:      stepStatus.Status,
				StartedAt:   getTimeValue(stepStatus.StartedAt),
				RetryCount:  stepStatus.RetryCount,
			})
		}
	}

	return pendingSteps, nil
}

// GetStepExecution returns a specific step execution
func (s *workflowService) GetStepExecution(ctx context.Context, executionID uuid.UUID, stepID string) (*models.StepExecution, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetStepExecution")
	defer span.End()

	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	stepStatus, exists := execution.StepStatuses[stepID]
	if !exists {
		return nil, fmt.Errorf("step %s not found", stepID)
	}

	return &models.StepExecution{
		ExecutionID: executionID,
		StepName:    stepID,
		Status:      stepStatus.Status,
		StartedAt:   getTimeValue(stepStatus.StartedAt),
		CompletedAt: stepStatus.CompletedAt,
		RetryCount:  stepStatus.RetryCount,
		Result:      stepStatus.Output,
		Error:       getStringPtr(stepStatus.Error),
	}, nil
}

// GetWorkflowHistory returns execution history for a workflow
func (s *workflowService) GetWorkflowHistory(ctx context.Context, workflowID uuid.UUID, limit int, offset int) ([]*models.WorkflowExecution, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetWorkflowHistory")
	defer span.End()

	// Get from repository
	executions, err := s.repo.ListExecutions(ctx, workflowID, limit)
	if err != nil {
		return nil, err
	}

	return executions, nil
}

// GetWorkflowMetrics returns metrics for a workflow
func (s *workflowService) GetWorkflowMetrics(ctx context.Context, workflowID uuid.UUID) (*models.WorkflowMetrics, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetWorkflowMetrics")
	defer span.End()

	// Check cache
	cacheKey := fmt.Sprintf("metrics:%s", workflowID)
	var metrics models.WorkflowMetrics
	if err := s.statsCache.Get(ctx, cacheKey, &metrics); err == nil {
		return &metrics, nil
	}

	// Calculate metrics - get all executions
	executions, err := s.repo.ListExecutions(ctx, workflowID, 1000)
	if err != nil {
		return nil, err
	}

	metrics.WorkflowID = workflowID
	metrics.TotalExecutions = int64(len(executions))
	
	var totalDuration time.Duration
	for _, exec := range executions {
		if exec.Status == models.WorkflowExecutionStatusCompleted {
			metrics.SuccessfulRuns++
			if exec.CompletedAt != nil {
				duration := exec.CompletedAt.Sub(exec.StartedAt)
				totalDuration += duration
			}
		} else if exec.Status == models.WorkflowExecutionStatusFailed {
			metrics.FailedRuns++
		}
	}

	if metrics.SuccessfulRuns > 0 {
		metrics.AverageRunTime = totalDuration / time.Duration(metrics.SuccessfulRuns)
	}

	// Cache metrics
	s.statsCache.Set(ctx, cacheKey, &metrics, 1*time.Minute)

	return &metrics, nil
}

// GetExecutionHistory returns the execution history (same as GetWorkflowHistory for compatibility)
func (s *workflowService) GetExecutionHistory(ctx context.Context, workflowID uuid.UUID) ([]*models.WorkflowExecution, error) {
	return s.GetWorkflowHistory(ctx, workflowID, 100, 0)
}

// Helper functions
func getTimeValue(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func getStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ExecutionMonitor monitors running workflow executions
type ExecutionMonitor struct {
	service *workflowService
	ticker  *time.Ticker
	done    chan bool
}

func NewExecutionMonitor(service *workflowService) *ExecutionMonitor {
	return &ExecutionMonitor{
		service: service,
		ticker:  time.NewTicker(30 * time.Second),
		done:    make(chan bool),
	}
}

func (m *ExecutionMonitor) Start() {
	for {
		select {
		case <-m.ticker.C:
			m.checkExecutions()
		case <-m.done:
			m.ticker.Stop()
			return
		}
	}
}

func (m *ExecutionMonitor) Stop() {
	close(m.done)
}

func (m *ExecutionMonitor) checkExecutions() {
	// Monitor active executions
	m.service.activeExecutions.Range(func(key, value interface{}) bool {
		executionID := key.(uuid.UUID)
		execution := value.(*models.WorkflowExecution)

		// Check for timeouts, stuck steps, etc.
		if time.Since(execution.StartedAt) > 24*time.Hour {
			m.service.config.Logger.Warn("Long-running execution detected", map[string]interface{}{
				"execution_id": executionID,
				"duration":     time.Since(execution.StartedAt).String(),
			})
		}

		return true
	})
}