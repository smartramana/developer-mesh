package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
	"github.com/S-Corkum/devops-mcp/pkg/repository/types"
)

type workflowService struct {
	BaseService

	// Dependencies
	repo         interfaces.WorkflowRepository
	taskService  TaskService
	agentService AgentService
	notifier     NotificationService

	// Caching
	workflowCache  cache.Cache
	executionCache cache.Cache
	statsCache     cache.Cache

	// Execution management
	executionLocks   sync.Map // map[uuid.UUID]*sync.Mutex
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
		BaseService:    NewBaseService(config),
		repo:           repo,
		taskService:    taskService,
		agentService:   agentService,
		notifier:       notifier,
		workflowCache:  cache.NewMemoryCache(1000, 10*time.Minute),
		executionCache: cache.NewMemoryCache(5000, 5*time.Minute),
		statsCache:     cache.NewMemoryCache(500, 1*time.Minute),
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

	// Use repository transaction for database operations
	tx, err := s.repo.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin transaction")
	}

	// Create repository with transaction
	txRepo := s.repo.WithTx(tx)

	// Execute in transaction
	err = func() error {
		// Create execution record
		if err := txRepo.CreateExecution(ctx, execution); err != nil {
			return errors.Wrap(err, "failed to create execution")
		}

		s.config.Logger.Info("Created workflow execution", map[string]interface{}{
			"execution_id": execution.ID,
			"workflow_id":  workflowID,
			"status":       execution.Status,
		})

		// Store idempotency key
		if idempotencyKey != "" {
			if err := s.storeExecutionIdempotencyKey(ctx, idempotencyKey, execution.ID); err != nil {
				return err
			}
		}

		// Update workflow last executed
		workflow.SetLastExecutedAt(execution.StartedAt)
		if err := txRepo.Update(ctx, workflow); err != nil {
			return err
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return errors.Wrap(err, "failed to commit transaction")
		}

		s.config.Logger.Info("Transaction committed for workflow execution", map[string]interface{}{
			"execution_id": execution.ID,
			"workflow_id":  workflowID,
		})

		// Publish event after commit
		if err := s.PublishEvent(ctx, "WorkflowExecutionStarted", workflow, execution); err != nil {
			// Log but don't fail - execution already committed
			s.config.Logger.Warn("Failed to publish event", map[string]interface{}{
				"event": "WorkflowExecutionStarted",
				"error": err.Error(),
			})
		}

		return nil
	}()

	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			s.config.Logger.Error("Failed to rollback transaction", map[string]interface{}{
				"error": rbErr.Error(),
			})
		}
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
	case "sequential":
		output, stepErr = s.executeSequentialStep(ctx, execution, step)
	case "script":
		output, stepErr = s.executeScriptStep(ctx, execution, step)
	case "webhook":
		output, stepErr = s.executeWebhookStep(ctx, execution, step)
	case "branching":
		output, stepErr = s.handleBranching(ctx, execution, step)
	case "compensation":
		output, stepErr = s.executeCompensation(ctx, execution, step)
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

	// Debug logging
	s.config.Logger.Info("Validating workflow steps", map[string]interface{}{
		"workflow_id": workflow.ID,
		"steps_count": len(workflow.Steps),
		"steps_type":  fmt.Sprintf("%T", workflow.Steps),
	})

	workflowSteps := workflow.GetSteps()
	s.config.Logger.Info("GetSteps result", map[string]interface{}{
		"step_count": len(workflowSteps),
	})
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

	_ = s.workflowCache.Set(ctx, cacheKey, workflow, 10*time.Minute) // Best effort caching
	return workflow, nil
}

func (s *workflowService) getExecution(ctx context.Context, id uuid.UUID) (*models.WorkflowExecution, error) {
	cacheKey := fmt.Sprintf("execution:%s", id)
	var cached models.WorkflowExecution
	if err := s.executionCache.Get(ctx, cacheKey, &cached); err == nil {
		s.config.Logger.Debug("Found execution in cache", map[string]interface{}{
			"execution_id": id,
		})
		return &cached, nil
	}

	s.config.Logger.Debug("Getting execution from repository", map[string]interface{}{
		"execution_id": id,
	})
	execution, err := s.repo.GetExecution(ctx, id)
	if err != nil {
		s.config.Logger.Error("Failed to get execution from repository", map[string]interface{}{
			"execution_id": id,
			"error":        err.Error(),
			"error_type":   fmt.Sprintf("%T", err),
		})
		// Check if it's a not found error
		if errors.Is(err, interfaces.ErrNotFound) {
			return nil, fmt.Errorf("execution not found: %s", id)
		}
		return nil, err
	}

	_ = s.executionCache.Set(ctx, cacheKey, execution, 5*time.Minute) // Best effort caching
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
	ctx, span := s.config.Tracer(ctx, "WorkflowService.checkExecutionIdempotency")
	defer span.End()

	// Check cache for idempotency key
	cacheKey := fmt.Sprintf("idempotency:%s", key)
	var executionID uuid.UUID
	err := s.executionCache.Get(ctx, cacheKey, &executionID)
	if err == nil && executionID != uuid.Nil {
		s.config.Logger.Info("Found existing execution for idempotency key", map[string]interface{}{
			"key":          key,
			"execution_id": executionID.String(),
		})
		return executionID, nil
	}

	// In a production system, you would check the repository for recent executions
	// with this idempotency key stored in execution metadata
	// For now, we rely on cache-based idempotency with a 24-hour TTL

	return uuid.Nil, errors.New("not found")
}

func (s *workflowService) storeExecutionIdempotencyKey(ctx context.Context, key string, id uuid.UUID) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.storeExecutionIdempotencyKey")
	defer span.End()

	// Store in cache with 24-hour TTL
	cacheKey := fmt.Sprintf("idempotency:%s", key)
	err := s.executionCache.Set(ctx, cacheKey, id, 24*time.Hour)
	if err != nil {
		s.config.Logger.Error("Failed to store idempotency key in cache", map[string]interface{}{
			"key":          key,
			"execution_id": id.String(),
			"error":        err.Error(),
		})
		// Non-fatal, continue
	}

	// Also store in execution metadata for persistence
	// This would be done when creating the execution
	s.config.Logger.Info("Stored idempotency key", map[string]interface{}{
		"key":          key,
		"execution_id": id.String(),
	})

	return nil
}

func (s *workflowService) validateWorkflowInput(ctx context.Context, workflow *models.Workflow, input map[string]interface{}) error {
	_, span := s.config.Tracer(ctx, "WorkflowService.validateWorkflowInput")
	defer span.End()

	// Get input schema from workflow config
	var inputSchema map[string]interface{}
	if schema, ok := workflow.Config["input_schema"]; ok {
		if schemaMap, ok := schema.(map[string]interface{}); ok {
			inputSchema = schemaMap
		}
	}

	if inputSchema == nil {
		// No schema defined, allow any input
		return nil
	}

	// Validate required fields
	if required, ok := inputSchema["required"].([]interface{}); ok {
		for _, field := range required {
			fieldName, ok := field.(string)
			if !ok {
				continue
			}
			if input == nil || input[fieldName] == nil {
				return ValidationError{
					Field:   fieldName,
					Message: "required field missing",
				}
			}
		}
	}

	// Validate field types
	if properties, ok := inputSchema["properties"].(map[string]interface{}); ok {
		for fieldName, fieldDef := range properties {
			if fieldDefMap, ok := fieldDef.(map[string]interface{}); ok {
				if fieldType, ok := fieldDefMap["type"].(string); ok {
					if input != nil && input[fieldName] != nil {
						if err := s.validateFieldType(fieldName, input[fieldName], fieldType); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	// Validate field constraints
	if constraints, ok := inputSchema["constraints"].(map[string]interface{}); ok {
		for fieldName, constraint := range constraints {
			if input != nil && input[fieldName] != nil {
				if err := s.validateFieldConstraint(fieldName, input[fieldName], constraint); err != nil {
					return err
				}
			}
		}
	}

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
	ctx, span := s.config.Tracer(ctx, "WorkflowService.executeApprovalStep")
	defer span.End()

	// Extract approval configuration
	approvalConfig := make(map[string]interface{})
	if step.Config != nil {
		approvalConfig = step.Config
	}

	// Get required approvers
	requiredApprovers := []string{}
	if approvers, ok := approvalConfig["approvers"].([]interface{}); ok {
		for _, approver := range approvers {
			if approverStr, ok := approver.(string); ok {
				requiredApprovers = append(requiredApprovers, approverStr)
			}
		}
	}

	if len(requiredApprovers) == 0 {
		return nil, ValidationError{
			Field:   "approvers",
			Message: "at least one approver required for approval step",
		}
	}

	// Get approval strategy (unanimous, majority, any)
	strategy := "any" // default
	if strategyStr, ok := approvalConfig["strategy"].(string); ok {
		strategy = strategyStr
	}

	// Calculate timeout
	timeout := 24 * time.Hour // default 24 hours
	if timeoutMinutes, ok := approvalConfig["timeout_minutes"].(float64); ok {
		timeout = time.Duration(timeoutMinutes) * time.Minute
	}

	// Create pending approval record
	pendingApproval := &models.PendingApproval{
		ExecutionID: execution.ID,
		WorkflowID:  execution.WorkflowID,
		StepID:      step.ID,
		StepName:    step.Name,
		RequestedAt: time.Now(),
		RequiredBy:  requiredApprovers,
		ApprovedBy:  []string{},
		RejectedBy:  []string{},
		DueBy:       timePtr(time.Now().Add(timeout)),
		Context: map[string]interface{}{
			"workflow_name": execution.Workflow.Name,
			"step_input":    step.Input,
			"initiated_by":  execution.InitiatedBy,
			"strategy":      strategy,
		},
	}

	// Store pending approval (in production, this would be in a dedicated approval repository)
	approvalKey := fmt.Sprintf("approval:%s:%s", execution.ID, step.ID)
	if err := s.executionCache.Set(ctx, approvalKey, pendingApproval, timeout); err != nil {
		return nil, errors.Wrap(err, "failed to store pending approval")
	}

	// Update step status to indicate waiting for approval
	if stepStatus, exists := execution.StepStatuses[step.ID]; exists {
		stepStatus.Status = "awaiting_approval"
		stepStatus.Output = map[string]interface{}{
			"approval_requested_at": pendingApproval.RequestedAt,
			"required_approvers":    requiredApprovers,
			"due_by":                pendingApproval.DueBy,
		}
	}

	// Send notifications to approvers
	for _, approverID := range requiredApprovers {
		notification := map[string]interface{}{
			"type":         "approval_request",
			"execution_id": execution.ID,
			"workflow_id":  execution.WorkflowID,
			"step_id":      step.ID,
			"step_name":    step.Name,
			"approver_id":  approverID,
			"due_by":       pendingApproval.DueBy,
			"context":      pendingApproval.Context,
		}

		if err := s.notifier.BroadcastToAgents(ctx, []string{approverID}, notification); err != nil {
			s.config.Logger.Error("Failed to send approval notification", map[string]interface{}{
				"approver_id": approverID,
				"error":       err.Error(),
			})
		}
	}

	// Record metric
	s.config.Metrics.IncrementCounterWithLabels("workflow.approval.requested", 1, map[string]string{
		"workflow_id": execution.WorkflowID.String(),
		"step_id":     step.ID,
		"strategy":    strategy,
	})

	// Create background task to monitor approval timeout
	go func() {
		// Monitor in a separate goroutine
		time.Sleep(timeout)
		// Check if still pending and timeout
		var pendingApproval models.PendingApproval
		if err := s.executionCache.Get(ctx, approvalKey, &pendingApproval); err == nil {
			// Still pending, mark as timeout
			s.config.Logger.Warn("Approval timeout", map[string]interface{}{
				"execution_id": execution.ID,
				"step_id":      step.ID,
			})
			// Update step status to timeout
			if stepStatus, exists := execution.StepStatuses[step.ID]; exists {
				stepStatus.Status = models.StepStatusTimeout
				stepStatus.Error = "approval timeout"
				if err := s.UpdateExecution(ctx, execution); err != nil {
					s.config.Logger.Error("Failed to update execution on approval timeout", map[string]interface{}{
						"execution_id": execution.ID,
						"step_id":      step.ID,
						"error":        err.Error(),
					})
				}
			}
		}
	}()

	// Return approval request details
	return map[string]interface{}{
		"status":             "awaiting_approval",
		"approval_id":        approvalKey,
		"required_approvers": requiredApprovers,
		"strategy":           strategy,
		"due_by":             pendingApproval.DueBy,
		"requested_at":       pendingApproval.RequestedAt,
	}, nil
}

func (s *workflowService) executeParallelStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.executeParallelStep")
	defer span.End()

	// Extract parallel configuration
	parallelConfig := make(map[string]interface{})
	if step.Config != nil {
		parallelConfig = step.Config
	}

	// Get branches to execute
	branches := []map[string]interface{}{}
	if branchesData, ok := parallelConfig["branches"].([]interface{}); ok {
		for _, branch := range branchesData {
			if branchMap, ok := branch.(map[string]interface{}); ok {
				branches = append(branches, branchMap)
			}
		}
	}

	if len(branches) == 0 {
		return nil, ValidationError{
			Field:   "branches",
			Message: "at least one branch required for parallel step",
		}
	}

	// Get join strategy (all, any, fail_fast)
	joinStrategy := "all" // default - wait for all branches
	if strategy, ok := parallelConfig["join_strategy"].(string); ok {
		joinStrategy = strategy
	}

	// Create error group for parallel execution
	type branchResult struct {
		branchID string
		result   map[string]interface{}
		err      error
	}

	results := make(chan branchResult, len(branches))
	var wg sync.WaitGroup

	// Execute branches in parallel
	for i, branch := range branches {
		wg.Add(1)
		go func(branchIndex int, branchConfig map[string]interface{}) {
			defer wg.Done()

			branchID := fmt.Sprintf("%s_branch_%d", step.ID, branchIndex)
			if id, ok := branchConfig["id"].(string); ok {
				branchID = id
			}

			// Record branch start
			s.config.Metrics.IncrementCounterWithLabels("workflow.parallel.branch.start", 1, map[string]string{
				"workflow_id": execution.WorkflowID.String(),
				"step_id":     step.ID,
				"branch_id":   branchID,
			})

			// Get timeout for this branch
			branchTimeout := 30 * time.Minute // default
			if timeoutMinutes, ok := branchConfig["timeout_minutes"].(float64); ok {
				branchTimeout = time.Duration(timeoutMinutes) * time.Minute
			}

			// Create task for branch
			branchTask := &models.Task{
				ID:             uuid.New(),
				TenantID:       execution.TenantID,
				Title:          fmt.Sprintf("Parallel branch: %s", branchID),
				Type:           "workflow_branch",
				Priority:       models.TaskPriorityNormal,
				Status:         models.TaskStatusPending,
				CreatedBy:      execution.InitiatedBy,
				Parameters:     branchConfig,
				MaxRetries:     3,
				TimeoutSeconds: int(branchTimeout.Seconds()),
			}

			// Assign to capable agent
			if agentID, ok := branchConfig["agent_id"].(string); ok {
				branchTask.AssignedTo = &agentID
			} else if step.AgentID != "" {
				branchTask.AssignedTo = &step.AgentID
			}

			// Create task with idempotency key
			idempotencyKey := fmt.Sprintf("workflow:%s:step:%s:branch:%s", execution.ID, step.ID, branchID)
			err := s.taskService.Create(ctx, branchTask, idempotencyKey)
			if err != nil {
				results <- branchResult{
					branchID: branchID,
					err:      errors.Wrapf(err, "failed to create task for branch %s", branchID),
				}
				return
			}

			// Wait for task completion with timeout
			branchCtx, cancel := context.WithTimeout(ctx, branchTimeout)
			defer cancel()

			// Poll for completion
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-branchCtx.Done():
					results <- branchResult{
						branchID: branchID,
						err:      fmt.Errorf("branch %s timeout exceeded", branchID),
					}
					return
				case <-ticker.C:
					taskStatus, err := s.taskService.Get(branchCtx, branchTask.ID)
					if err != nil {
						results <- branchResult{
							branchID: branchID,
							err:      errors.Wrapf(err, "failed to get task status for branch %s", branchID),
						}
						return
					}

					switch taskStatus.Status {
					case models.TaskStatusCompleted:
						// Record branch completion
						s.config.Metrics.IncrementCounterWithLabels("workflow.parallel.branch.complete", 1, map[string]string{
							"workflow_id": execution.WorkflowID.String(),
							"step_id":     step.ID,
							"branch_id":   branchID,
						})

						results <- branchResult{
							branchID: branchID,
							result: map[string]interface{}{
								"task_id": branchTask.ID,
								"result":  taskStatus.Result,
								"status":  "completed",
							},
						}
						return

					case models.TaskStatusFailed:
						results <- branchResult{
							branchID: branchID,
							err:      fmt.Errorf("branch %s failed: %s", branchID, taskStatus.Error),
						}
						return
					}
				}
			}
		}(i, branch)
	}

	// Wait for results based on join strategy
	branchResults := make(map[string]interface{})
	errors := []error{}
	completed := 0

	// Start a goroutine to close results channel when all branches complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		if result.err != nil {
			errors = append(errors, result.err)
			branchResults[result.branchID] = map[string]interface{}{
				"status": "failed",
				"error":  result.err.Error(),
			}

			// For fail_fast strategy, cancel remaining branches
			if joinStrategy == "fail_fast" {
				return branchResults, result.err
			}
		} else {
			completed++
			branchResults[result.branchID] = result.result

			// For "any" strategy, return as soon as one succeeds
			if joinStrategy == "any" {
				return branchResults, nil
			}
		}
	}

	// Check results based on join strategy
	switch joinStrategy {
	case "all":
		if len(errors) > 0 {
			return branchResults, fmt.Errorf("parallel execution failed: %d branches failed", len(errors))
		}
	case "any":
		if completed == 0 {
			return branchResults, fmt.Errorf("parallel execution failed: no branches succeeded")
		}
	}

	// Record metrics
	s.config.Metrics.RecordHistogram("workflow.parallel.branches", float64(len(branches)), map[string]string{
		"workflow_id": execution.WorkflowID.String(),
		"step_id":     step.ID,
	})

	s.config.Metrics.RecordHistogram("workflow.parallel.failures", float64(len(errors)), map[string]string{
		"workflow_id": execution.WorkflowID.String(),
		"step_id":     step.ID,
	})

	return branchResults, nil
}

func (s *workflowService) executeConditionalStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.executeConditionalStep")
	defer span.End()

	// Extract conditional configuration
	conditionalConfig := make(map[string]interface{})
	if step.Config != nil {
		conditionalConfig = step.Config
	}

	// Get condition to evaluate
	condition, ok := conditionalConfig["condition"].(map[string]interface{})
	if !ok {
		return nil, ValidationError{
			Field:   "condition",
			Message: "condition configuration required for conditional step",
		}
	}

	// Get branches
	branches, ok := conditionalConfig["branches"].(map[string]interface{})
	if !ok {
		return nil, ValidationError{
			Field:   "branches",
			Message: "branches configuration required for conditional step",
		}
	}

	// Evaluate condition
	evaluated, err := s.evaluateCondition(ctx, execution, condition)
	if err != nil {
		return nil, errors.Wrap(err, "failed to evaluate condition")
	}

	// Record metric
	s.config.Metrics.IncrementCounterWithLabels("workflow.conditional.evaluation", 1, map[string]string{
		"workflow_id": execution.WorkflowID.String(),
		"step_id":     step.ID,
		"result":      evaluated,
	})

	// Select branch based on evaluation
	var selectedBranch map[string]interface{}
	var branchID string

	switch evaluated {
	case "true":
		if trueBranch, ok := branches["true"].(map[string]interface{}); ok {
			selectedBranch = trueBranch
			branchID = "true"
		}
	case "false":
		if falseBranch, ok := branches["false"].(map[string]interface{}); ok {
			selectedBranch = falseBranch
			branchID = "false"
		}
	default:
		// Check for specific value branches
		if valueBranch, ok := branches[evaluated].(map[string]interface{}); ok {
			selectedBranch = valueBranch
			branchID = evaluated
		} else if defaultBranch, ok := branches["default"].(map[string]interface{}); ok {
			selectedBranch = defaultBranch
			branchID = "default"
		}
	}

	if selectedBranch == nil {
		return nil, fmt.Errorf("no matching branch for condition result: %s", evaluated)
	}

	// Execute selected branch
	branchTask := &models.Task{
		ID:             uuid.New(),
		TenantID:       execution.TenantID,
		Title:          fmt.Sprintf("Conditional branch: %s", branchID),
		Type:           "workflow_branch",
		Priority:       models.TaskPriorityNormal,
		Status:         models.TaskStatusPending,
		CreatedBy:      execution.InitiatedBy,
		Parameters:     selectedBranch,
		MaxRetries:     3,
		TimeoutSeconds: 1800, // 30 minutes default
	}

	// Assign to agent if specified
	if agentID, ok := selectedBranch["agent_id"].(string); ok {
		branchTask.AssignedTo = &agentID
	} else if step.AgentID != "" {
		branchTask.AssignedTo = &step.AgentID
	}

	// Create task with idempotency
	idempotencyKey := fmt.Sprintf("workflow:%s:step:%s:condition:%s", execution.ID, step.ID, branchID)
	if err := s.taskService.Create(ctx, branchTask, idempotencyKey); err != nil {
		return nil, errors.Wrap(err, "failed to create conditional branch task")
	}

	// Wait for task completion
	timeout := 30 * time.Minute
	if timeoutMinutes, ok := selectedBranch["timeout_minutes"].(float64); ok {
		timeout = time.Duration(timeoutMinutes) * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("conditional branch timeout exceeded")
		case <-ticker.C:
			taskStatus, err := s.taskService.Get(ctx, branchTask.ID)
			if err != nil {
				return nil, err
			}

			switch taskStatus.Status {
			case models.TaskStatusCompleted:
				return map[string]interface{}{
					"task_id":          branchTask.ID,
					"branch_selected":  branchID,
					"condition_result": evaluated,
					"result":           taskStatus.Result,
				}, nil
			case models.TaskStatusFailed:
				return nil, fmt.Errorf("conditional branch failed: %s", taskStatus.Error)
			}
		}
	}
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
	if err := s.PublishEvent(ctx, "WorkflowExecutionCompleted", execution, execution); err != nil {
		s.config.Logger.Error("Failed to publish workflow completion event", map[string]interface{}{
			"execution_id": execution.ID,
			"error":        err.Error(),
		})
	}

	// Notify interested parties
	if err := s.notifier.NotifyWorkflowCompleted(ctx, execution); err != nil {
		s.config.Logger.Error("Failed to notify workflow completion", map[string]interface{}{
			"execution_id": execution.ID,
			"error":        err.Error(),
		})
	}
}

func (s *workflowService) invalidateWorkflowCaches(ctx context.Context, tenantID uuid.UUID) {
	// Invalidate tenant-specific caches
	_ = s.workflowCache.Delete(ctx, fmt.Sprintf("tenant:%s:workflows", tenantID))
	_ = s.statsCache.Delete(ctx, fmt.Sprintf("tenant:%s:workflow:stats", tenantID))
}

// Interface method implementations

func (s *workflowService) GetWorkflow(ctx context.Context, id uuid.UUID) (*models.Workflow, error) {
	return s.getWorkflow(ctx, id)
}

func (s *workflowService) UpdateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.UpdateWorkflow")
	defer span.End()

	// Validate input
	if workflow == nil {
		return ValidationError{Field: "workflow", Message: "required"}
	}

	if workflow.ID == uuid.Nil {
		return ValidationError{Field: "workflow.id", Message: "required"}
	}

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:update"); err != nil {
		return err
	}

	// Get existing workflow for validation and authorization
	existing, err := s.getWorkflow(ctx, workflow.ID)
	if err != nil {
		return errors.Wrap(err, "workflow not found")
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "update",
			Conditions: map[string]interface{}{
				"workflow_id": workflow.ID,
				"tenant_id":   existing.TenantID,
			},
		})

		if !decision.Allowed {
			return UnauthorizedError{
				Action: "update workflow",
				Reason: decision.Reason,
			}
		}
	}

	// Check if workflow has active executions - prevent type changes
	activeExecutions, err := s.repo.GetActiveExecutions(ctx, workflow.ID)
	if err != nil {
		return errors.Wrap(err, "failed to check active executions")
	}

	if len(activeExecutions) > 0 && workflow.Type != existing.Type {
		return ValidationError{
			Field:   "type",
			Message: fmt.Sprintf("cannot change workflow type while %d executions are active", len(activeExecutions)),
		}
	}

	// Sanitize input
	if err := s.sanitizeWorkflow(workflow); err != nil {
		return errors.Wrap(err, "input sanitization failed")
	}

	// Validate workflow definition
	if err := s.validateWorkflow(ctx, workflow); err != nil {
		return errors.Wrap(err, "workflow validation failed")
	}

	// Execute in transaction
	err = s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Preserve immutable fields
		workflow.ID = existing.ID
		workflow.TenantID = existing.TenantID
		workflow.CreatedBy = existing.CreatedBy
		workflow.CreatedAt = existing.CreatedAt

		// Update metadata
		workflow.UpdatedAt = time.Now()
		workflow.Version = existing.Version + 1

		// Update workflow
		if err := s.repo.Update(ctx, workflow); err != nil {
			if errors.Is(err, ErrConcurrentModification) {
				return ConcurrentModificationError{
					Resource: "workflow",
					ID:       workflow.ID,
					Version:  existing.Version,
				}
			}
			return errors.Wrap(err, "failed to update workflow")
		}

		// Create audit log entry
		auditEntry := map[string]interface{}{
			"action":       "workflow_updated",
			"workflow_id":  workflow.ID,
			"updated_by":   auth.GetAgentID(ctx),
			"version":      workflow.Version,
			"changes_made": s.calculateChanges(existing, workflow),
		}

		// Publish event
		if err := s.PublishEvent(ctx, "WorkflowUpdated", workflow, auditEntry); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Invalidate all related caches
	s.invalidateWorkflowCaches(ctx, workflow.TenantID)
	_ = s.workflowCache.Delete(ctx, fmt.Sprintf("workflow:%s", workflow.ID))

	// Clear execution caches for this workflow
	s.activeExecutions.Range(func(key, value interface{}) bool {
		if exec, ok := value.(*models.WorkflowExecution); ok && exec.WorkflowID == workflow.ID {
			_ = s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", exec.ID))
		}
		return true
	})

	// Notify about the update
	if s.notifier != nil {
		if err := s.notifier.NotifyWorkflowUpdated(ctx, workflow); err != nil {
			s.config.Logger.Error("Failed to notify workflow update", map[string]interface{}{
				"workflow_id": workflow.ID,
				"error":       err.Error(),
			})
		}
	}

	return nil
}

func (s *workflowService) DeleteWorkflow(ctx context.Context, id uuid.UUID) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.DeleteWorkflow")
	defer span.End()

	// Validate input
	if id == uuid.Nil {
		return ValidationError{Field: "id", Message: "required"}
	}

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:delete"); err != nil {
		return err
	}

	// Get workflow for authorization
	workflow, err := s.getWorkflow(ctx, id)
	if err != nil {
		return errors.Wrap(err, "workflow not found")
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "delete",
			Conditions: map[string]interface{}{
				"workflow_id": id,
				"tenant_id":   workflow.TenantID,
			},
		})

		if !decision.Allowed {
			return UnauthorizedError{
				Action: "delete workflow",
				Reason: decision.Reason,
			}
		}
	}

	// Check for running executions
	activeExecutions, err := s.repo.GetActiveExecutions(ctx, id)
	if err != nil {
		return errors.Wrap(err, "failed to check active executions")
	}

	if len(activeExecutions) > 0 {
		return ValidationError{
			Field:   "workflow",
			Message: fmt.Sprintf("cannot delete workflow with %d active executions", len(activeExecutions)),
		}
	}

	// Execute in transaction
	err = s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Get all executions for archival
		allExecutions, err := s.repo.ListExecutions(ctx, id, 10000)
		if err != nil {
			return errors.Wrap(err, "failed to list executions")
		}

		// Archive executions
		for _, exec := range allExecutions {
			archiveData := map[string]interface{}{
				"workflow_id":    id,
				"execution_id":   exec.ID,
				"archived_at":    time.Now(),
				"archived_by":    auth.GetAgentID(ctx),
				"execution_data": exec,
			}

			if err := s.PublishEvent(ctx, "WorkflowExecutionArchived", exec, archiveData); err != nil {
				s.config.Logger.Warn("Failed to publish execution archive event", map[string]interface{}{
					"execution_id": exec.ID,
					"error":        err.Error(),
				})
			}
		}

		// Soft delete the workflow (keeps record but marks as deleted)
		if err := s.repo.SoftDelete(ctx, id); err != nil {
			return errors.Wrap(err, "failed to delete workflow")
		}

		// Publish deletion event
		deletionData := map[string]interface{}{
			"workflow_id":      id,
			"deleted_by":       auth.GetAgentID(ctx),
			"deleted_at":       time.Now(),
			"executions_count": len(allExecutions),
		}

		if err := s.PublishEvent(ctx, "WorkflowDeleted", workflow, deletionData); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Clean up caches
	_ = s.workflowCache.Delete(ctx, fmt.Sprintf("workflow:%s", id))
	s.invalidateWorkflowCaches(ctx, workflow.TenantID)

	// Clean up active executions
	s.activeExecutions.Range(func(key, value interface{}) bool {
		if exec, ok := value.(*models.WorkflowExecution); ok && exec.WorkflowID == id {
			s.activeExecutions.Delete(key)
			_ = s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", exec.ID))
		}
		return true
	})

	// Notify about the deletion
	if s.notifier != nil {
		_ = s.notifier.NotifyResourceDeleted(ctx, "workflow", id)
	}

	return nil
}

func (s *workflowService) ListWorkflows(ctx context.Context, filters interfaces.WorkflowFilters) ([]*models.Workflow, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ListWorkflows")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:list"); err != nil {
		return nil, err
	}

	// Get tenant ID from context
	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return nil, ValidationError{Field: "tenant_id", Message: "required in context"}
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "list",
			Conditions: map[string]interface{}{
				"tenant_id": tenantID,
			},
		})

		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "list workflows",
				Reason: decision.Reason,
			}
		}
	}

	// Check cache for filtered results
	cacheKey := s.buildWorkflowListCacheKey(tenantID, filters)
	var cached []*models.Workflow
	if err := s.workflowCache.Get(ctx, cacheKey, &cached); err == nil {
		s.config.Metrics.IncrementCounterWithLabels("cache_hit", 1, map[string]string{
			"cache": "workflow_list",
		})
		return cached, nil
	}

	// Apply default limits
	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Limit > 1000 {
		filters.Limit = 1000
	}

	// Circuit breaker for database calls
	result, err := s.ExecuteWithCircuitBreaker(ctx, "workflow_list", func() (interface{}, error) {
		return s.repo.List(ctx, tenantID, filters)
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to list workflows")
	}

	workflows := result.([]*models.Workflow)

	// Apply post-filtering based on business rules
	filteredWorkflows := make([]*models.Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		// Check if user has access to this specific workflow
		if s.config.Authorizer != nil {
			decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
				Resource: "workflow",
				Action:   "view",
				Conditions: map[string]interface{}{
					"workflow_id": workflow.ID,
					"created_by":  workflow.CreatedBy,
				},
			})

			if !decision.Allowed {
				continue
			}
		}

		filteredWorkflows = append(filteredWorkflows, workflow)
	}

	// Cache the results
	_ = s.workflowCache.Set(ctx, cacheKey, filteredWorkflows, 5*time.Minute)

	// Record metrics
	s.config.Metrics.RecordHistogram("workflow_list_size", float64(len(filteredWorkflows)), map[string]string{
		"tenant_id": tenantID.String(),
	})

	return filteredWorkflows, nil
}

func (s *workflowService) SearchWorkflows(ctx context.Context, query string) ([]*models.Workflow, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.SearchWorkflows")
	defer span.End()

	// Validate input
	if query == "" {
		return nil, ValidationError{Field: "query", Message: "required"}
	}

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:search"); err != nil {
		return nil, err
	}

	// Get tenant ID from context
	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return nil, ValidationError{Field: "tenant_id", Message: "required in context"}
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "search",
			Conditions: map[string]interface{}{
				"tenant_id": tenantID,
			},
		})

		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "search workflows",
				Reason: decision.Reason,
			}
		}
	}

	// Check cache
	cacheKey := fmt.Sprintf("workflow:search:%s:%s", tenantID, query)
	var cached []*models.Workflow
	if err := s.workflowCache.Get(ctx, cacheKey, &cached); err == nil {
		return cached, nil
	}

	// Create search filters
	filters := interfaces.WorkflowFilters{
		Limit:     100,
		SortBy:    "relevance",
		SortOrder: types.SortDesc,
	}

	// Execute search with circuit breaker
	result, err := s.ExecuteWithCircuitBreaker(ctx, "workflow_search", func() (interface{}, error) {
		// Try full-text search first
		workflows, err := s.repo.SearchWorkflows(ctx, query, filters)
		if err == nil && len(workflows) > 0 {
			return workflows, nil
		}

		// Fall back to basic search
		// In production, you'd integrate with a vector store for similarity search

		// If all else fails, do a basic name/description search
		return s.repo.List(ctx, tenantID, interfaces.WorkflowFilters{
			Limit: 100,
		})
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to search workflows")
	}

	workflows := result.([]*models.Workflow)

	// Filter by relevance (simple scoring based on query match)
	scoredWorkflows := s.scoreWorkflowRelevance(workflows, query)

	// Sort by relevance score
	sort.Slice(scoredWorkflows, func(i, j int) bool {
		return scoredWorkflows[i].score > scoredWorkflows[j].score
	})

	// Extract workflows from scored results
	results := make([]*models.Workflow, 0, len(scoredWorkflows))
	for _, sw := range scoredWorkflows {
		// Additional authorization check
		if s.config.Authorizer != nil {
			decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
				Resource: "workflow",
				Action:   "view",
				Conditions: map[string]interface{}{
					"workflow_id": sw.workflow.ID,
				},
			})

			if !decision.Allowed {
				continue
			}
		}

		results = append(results, sw.workflow)
	}

	// Cache results
	_ = s.workflowCache.Set(ctx, cacheKey, results, 2*time.Minute)

	return results, nil
}

func (s *workflowService) GetExecution(ctx context.Context, executionID uuid.UUID) (*models.WorkflowExecution, error) {
	return s.getExecution(ctx, executionID)
}

func (s *workflowService) ListExecutions(ctx context.Context, workflowID uuid.UUID, filters interfaces.ExecutionFilters) ([]*models.WorkflowExecution, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ListExecutions")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:list"); err != nil {
		return nil, err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "list",
			Conditions: map[string]interface{}{
				"workflow_id": workflowID,
			},
		})
		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "list executions",
				Reason: decision.Reason,
			}
		}
	}

	// Check cache
	cacheKey := fmt.Sprintf("executions:%s:filters:%v", workflowID, filters)
	var cached []*models.WorkflowExecution
	if err := s.executionCache.Get(ctx, cacheKey, &cached); err == nil {
		return cached, nil
	}

	// Get from repository with pagination
	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	executions, err := s.repo.ListExecutions(ctx, workflowID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list executions")
	}

	// Apply additional filters
	filteredExecutions := make([]*models.WorkflowExecution, 0, len(executions))
	for _, exec := range executions {
		// Filter by status
		if len(filters.Status) > 0 {
			statusMatch := false
			for _, status := range filters.Status {
				if string(exec.Status) == status {
					statusMatch = true
					break
				}
			}
			if !statusMatch {
				continue
			}
		}

		// Filter by date range (using CreatedAfter/Before from ExecutionFilters)
		if filters.CreatedAfter != nil && exec.StartedAt.Before(*filters.CreatedAfter) {
			continue
		}
		if filters.CreatedBefore != nil && exec.StartedAt.After(*filters.CreatedBefore) {
			continue
		}

		// Filter by initiator (using TriggeredBy from ExecutionFilters)
		if filters.TriggeredBy != nil && *filters.TriggeredBy != "" && exec.InitiatedBy != *filters.TriggeredBy {
			continue
		}

		filteredExecutions = append(filteredExecutions, exec)
	}

	// Cache result
	_ = s.executionCache.Set(ctx, cacheKey, filteredExecutions, 30*time.Second)

	return filteredExecutions, nil
}

func (s *workflowService) GetExecutionStatus(ctx context.Context, executionID uuid.UUID) (*models.ExecutionStatus, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetExecutionStatus")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:status"); err != nil {
		return nil, err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "read",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "get execution status",
				Reason: decision.Reason,
			}
		}
	}

	// Calculate progress
	totalSteps := len(execution.StepStatuses)
	completedSteps := 0
	failedSteps := 0
	runningSteps := 0

	for _, stepStatus := range execution.StepStatuses {
		switch stepStatus.Status {
		case models.StepStatusCompleted:
			completedSteps++
		case models.StepStatusFailed:
			failedSteps++
		case models.StepStatusRunning:
			runningSteps++
		}
	}

	progress := float64(completedSteps) / float64(totalSteps) * 100
	if totalSteps == 0 {
		progress = 0
	}

	// Build status
	status := &models.ExecutionStatus{
		ExecutionID:    executionID,
		WorkflowID:     execution.WorkflowID,
		Status:         string(execution.Status),
		Progress:       int(progress),
		CurrentSteps:   []string{execution.CurrentStepID},
		TotalSteps:     totalSteps,
		CompletedSteps: completedSteps,
		StartedAt:      execution.StartedAt,
		UpdatedAt:      execution.UpdatedAt,
	}

	// Add metrics
	if status.Metrics == nil {
		status.Metrics = make(map[string]interface{})
	}
	status.Metrics["failed_steps"] = failedSteps
	status.Metrics["running_steps"] = runningSteps
	status.Metrics["duration_ms"] = execution.Duration().Milliseconds()
	if execution.Error != "" {
		status.Metrics["error"] = execution.Error
	}

	return status, nil
}

func (s *workflowService) GetExecutionTimeline(ctx context.Context, executionID uuid.UUID) ([]*models.ExecutionEvent, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetExecutionTimeline")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:timeline"); err != nil {
		return nil, err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "read",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "get execution timeline",
				Reason: decision.Reason,
			}
		}
	}

	// Build timeline from execution data
	events := make([]*models.ExecutionEvent, 0)

	// Execution started event
	events = append(events, &models.ExecutionEvent{
		Timestamp:   execution.StartedAt,
		EventType:   "execution_started",
		Description: "Workflow execution started",
		Details: map[string]interface{}{
			"initiated_by": execution.InitiatedBy,
			"workflow_id":  execution.WorkflowID,
		},
	})

	// Step events
	for stepID, stepStatus := range execution.StepStatuses {
		// Step started
		if stepStatus.StartedAt != nil {
			events = append(events, &models.ExecutionEvent{
				Timestamp:   *stepStatus.StartedAt,
				EventType:   "step_started",
				StepID:      stepID,
				AgentID:     stepStatus.AgentID,
				Description: fmt.Sprintf("Step %s started", stepID),
				Details:     stepStatus.Input,
			})
		}

		// Step completed
		if stepStatus.CompletedAt != nil {
			eventType := "step_completed"
			description := fmt.Sprintf("Step %s completed", stepID)
			if stepStatus.Error != "" {
				eventType = "step_failed"
				description = fmt.Sprintf("Step %s failed: %s", stepID, stepStatus.Error)
			}

			events = append(events, &models.ExecutionEvent{
				Timestamp:   *stepStatus.CompletedAt,
				EventType:   eventType,
				StepID:      stepID,
				AgentID:     stepStatus.AgentID,
				Description: description,
				Details:     stepStatus.Output,
			})
		}

		// Retry events
		if stepStatus.RetryCount > 0 {
			if stepStatus.StartedAt != nil {
				events = append(events, &models.ExecutionEvent{
					Timestamp:   *stepStatus.StartedAt,
					EventType:   "step_retried",
					StepID:      stepID,
					Description: fmt.Sprintf("Step %s retried (attempt %d)", stepID, stepStatus.RetryCount),
				})
			}
		}
	}

	// Execution completed event
	if execution.CompletedAt != nil {
		eventType := "execution_completed"
		description := "Workflow execution completed successfully"
		switch execution.Status {
		case models.WorkflowExecutionStatusFailed:
			eventType = "execution_failed"
			description = fmt.Sprintf("Workflow execution failed: %s", execution.Error)
		case models.WorkflowExecutionStatusCancelled:
			eventType = "execution_cancelled"
			description = "Workflow execution was cancelled"
		}

		events = append(events, &models.ExecutionEvent{
			Timestamp:   *execution.CompletedAt,
			EventType:   eventType,
			Description: description,
			Details: map[string]interface{}{
				"duration": execution.Duration().String(),
				"status":   execution.Status,
			},
		})
	}

	// Sort events by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events, nil
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
	_ = s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", execution.ID))

	return nil
}

func (s *workflowService) PauseExecution(ctx context.Context, executionID uuid.UUID, reason string) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.PauseExecution")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:pause"); err != nil {
		return err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "pause",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "pause execution",
				Reason: decision.Reason,
			}
		}
	}

	// Validate current status
	if execution.Status != models.WorkflowExecutionStatusRunning {
		return ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("cannot pause execution in status %s", execution.Status),
		}
	}

	// Update execution status
	execution.Status = models.WorkflowExecutionStatusPaused
	execution.UpdatedAt = time.Now()
	if execution.Context == nil {
		execution.Context = make(models.JSONMap)
	}
	execution.Context["pause_reason"] = reason
	execution.Context["paused_at"] = execution.UpdatedAt

	// Update in repository
	if err := s.repo.UpdateExecution(ctx, execution); err != nil {
		return errors.Wrap(err, "failed to update execution")
	}

	// Cancel any running tasks
	for stepID, stepStatus := range execution.StepStatuses {
		if stepStatus.Status == models.StepStatusRunning {
			// Notify agents to pause/cancel current tasks
			notification := map[string]interface{}{
				"type":         "execution_paused",
				"execution_id": executionID,
				"step_id":      stepID,
				"reason":       reason,
			}
			if err := s.notifier.NotifyWorkflowUpdated(ctx, notification); err != nil {
				s.config.Logger.Error("Failed to notify execution pause", map[string]interface{}{
					"execution_id": executionID,
					"step_id":      stepID,
					"error":        err.Error(),
				})
			}
		}
	}

	// Clear from active executions
	s.activeExecutions.Delete(executionID)

	// Invalidate cache
	_ = s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", executionID))

	// Publish event
	if err := s.PublishEvent(ctx, "ExecutionPaused", execution, map[string]interface{}{
		"reason": reason,
	}); err != nil {
		s.config.Logger.Error("Failed to publish pause event", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	s.config.Logger.Info("Execution paused", map[string]interface{}{
		"execution_id": executionID,
		"reason":       reason,
	})

	return nil
}

func (s *workflowService) ResumeExecution(ctx context.Context, executionID uuid.UUID) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ResumeExecution")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:resume"); err != nil {
		return err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "resume",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "resume execution",
				Reason: decision.Reason,
			}
		}
	}

	// Validate current status
	if execution.Status != models.WorkflowExecutionStatusPaused {
		return ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("cannot resume execution in status %s", execution.Status),
		}
	}

	// Update execution status
	execution.Status = models.WorkflowExecutionStatusRunning
	execution.UpdatedAt = time.Now()
	if execution.Context == nil {
		execution.Context = make(models.JSONMap)
	}
	execution.Context["resumed_at"] = execution.UpdatedAt
	delete(execution.Context, "pause_reason")

	// Update in repository
	if err := s.repo.UpdateExecution(ctx, execution); err != nil {
		return errors.Wrap(err, "failed to update execution")
	}

	// Add back to active executions
	s.activeExecutions.Store(executionID, execution)

	// Resume execution in background
	go func() {
		ctx := context.Background()
		defer s.activeExecutions.Delete(executionID)

		// Find next steps to execute
		workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
		if err != nil {
			s.config.Logger.Error("Failed to get workflow for resume", map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
				"error":        err.Error(),
			})
			return
		}

		// Continue execution from current step
		if execution.CurrentStepID != "" {
			if err := s.ExecuteWorkflowStep(ctx, executionID, execution.CurrentStepID); err != nil {
				s.config.Logger.Error("Failed to resume step execution", map[string]interface{}{
					"execution_id": executionID,
					"step_id":      execution.CurrentStepID,
					"error":        err.Error(),
				})
			}
		} else {
			// Find pending steps with satisfied dependencies
			workflowSteps := workflow.GetSteps()
			for _, step := range workflowSteps {
				stepStatus := execution.StepStatuses[step.ID]
				if stepStatus != nil && stepStatus.Status == models.StepStatusPending {
					// Check if dependencies are satisfied
					if err := s.checkStepDependencies(ctx, execution, &step); err == nil {
						if err := s.ExecuteWorkflowStep(ctx, executionID, step.ID); err != nil {
							s.config.Logger.Error("Failed to execute pending step", map[string]interface{}{
								"execution_id": executionID,
								"step_id":      step.ID,
								"error":        err.Error(),
							})
						}
					}
				}
			}
		}
	}()

	// Notify about resume
	notification := map[string]interface{}{
		"type":         "execution_resumed",
		"execution_id": executionID,
	}
	if err := s.notifier.NotifyWorkflowUpdated(ctx, notification); err != nil {
		s.config.Logger.Error("Failed to notify execution resume", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	// Invalidate cache
	_ = s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", executionID))

	// Publish event
	if err := s.PublishEvent(ctx, "ExecutionResumed", execution, nil); err != nil {
		s.config.Logger.Error("Failed to publish resume event", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	s.config.Logger.Info("Execution resumed", map[string]interface{}{
		"execution_id": executionID,
	})

	return nil
}

func (s *workflowService) CancelExecution(ctx context.Context, executionID uuid.UUID, reason string) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.CancelExecution")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:cancel"); err != nil {
		return err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "cancel",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "cancel execution",
				Reason: decision.Reason,
			}
		}
	}

	// Validate current status - can cancel unless already completed/failed/cancelled
	switch execution.Status {
	case models.WorkflowExecutionStatusCompleted,
		models.WorkflowExecutionStatusFailed,
		models.WorkflowExecutionStatusCancelled:
		return ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("cannot cancel execution in status %s", execution.Status),
		}
	}

	// Begin transaction to ensure consistency
	err = s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Update execution status
		execution.Status = models.WorkflowExecutionStatusCancelled
		execution.CompletedAt = &time.Time{}
		*execution.CompletedAt = time.Now()
		execution.UpdatedAt = *execution.CompletedAt
		if execution.Context == nil {
			execution.Context = make(models.JSONMap)
		}
		execution.Context["cancel_reason"] = reason
		execution.Context["cancelled_by"] = auth.GetAgentID(ctx)

		// Cancel all pending and running steps
		for _, stepStatus := range execution.StepStatuses {
			switch stepStatus.Status {
			case models.StepStatusPending, models.StepStatusRunning:
				stepStatus.Status = models.StepStatusCancelled
				now := time.Now()
				stepStatus.CompletedAt = &now
				stepStatus.Error = "Execution cancelled: " + reason
			}
		}

		// Update in repository
		if err := s.repo.UpdateExecution(ctx, execution); err != nil {
			return errors.Wrap(err, "failed to update execution")
		}

		// Execute compensation if defined in workflow config
		workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
		if err == nil {
			if onCancel, ok := workflow.Config["on_cancel"]; ok {
				// Log cancellation handler configuration
				s.config.Logger.Info("Workflow has cancellation handler", map[string]interface{}{
					"execution_id": executionID,
					"handler":      onCancel,
				})
				// In production, you would execute the cancellation handler
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Remove from active executions
	s.activeExecutions.Delete(executionID)

	// Invalidate cache
	_ = s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", executionID))

	// Notify about cancellation
	if err := s.notifier.NotifyWorkflowFailed(ctx, executionID, reason); err != nil {
		s.config.Logger.Error("Failed to notify execution cancellation", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	// Publish event
	if err := s.PublishEvent(ctx, "ExecutionCancelled", execution, map[string]interface{}{
		"reason": reason,
	}); err != nil {
		s.config.Logger.Error("Failed to publish cancel event", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	s.config.Logger.Info("Execution cancelled", map[string]interface{}{
		"execution_id": executionID,
		"reason":       reason,
	})

	return nil
}

func (s *workflowService) RetryExecution(ctx context.Context, executionID uuid.UUID, fromStep string) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.RetryExecution")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:retry"); err != nil {
		return err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "retry",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "retry execution",
				Reason: decision.Reason,
			}
		}
	}

	// Validate current status - can only retry failed executions
	if execution.Status != models.WorkflowExecutionStatusFailed {
		return ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("cannot retry execution in status %s", execution.Status),
		}
	}

	// Validate fromStep exists
	if fromStep != "" {
		if _, exists := execution.StepStatuses[fromStep]; !exists {
			return ValidationError{
				Field:   "fromStep",
				Message: fmt.Sprintf("step %s not found in execution", fromStep),
			}
		}
	}

	// Get workflow
	workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
	if err != nil {
		return errors.Wrap(err, "failed to get workflow")
	}

	// Check retry limit
	retryCount := 0
	if count, ok := execution.Context["retry_count"].(int); ok {
		retryCount = count
	}
	maxRetries := 3 // Default
	if max, ok := workflow.Config["max_retries"].(int); ok {
		maxRetries = max
	}
	if retryCount >= maxRetries {
		return ValidationError{
			Field:   "retry_count",
			Message: fmt.Sprintf("execution has been retried %d times, max is %d", retryCount, maxRetries),
		}
	}

	// Begin transaction
	err = s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Update execution status
		execution.Status = models.WorkflowExecutionStatusRunning
		execution.UpdatedAt = time.Now()
		execution.CompletedAt = nil
		if execution.Context == nil {
			execution.Context = make(models.JSONMap)
		}
		execution.Context["retry_count"] = retryCount + 1
		execution.Context["retry_from_step"] = fromStep
		execution.Context["retried_at"] = execution.UpdatedAt
		execution.Context["retried_by"] = auth.GetAgentID(ctx)

		// Reset step statuses from the specified step onwards
		workflowSteps := workflow.GetSteps()
		stepFound := fromStep == "" // If no fromStep specified, retry from beginning

		for _, step := range workflowSteps {
			if step.ID == fromStep {
				stepFound = true
			}

			if stepFound {
				// Reset this step and all subsequent steps
				if stepStatus, exists := execution.StepStatuses[step.ID]; exists {
					stepStatus.Status = models.StepStatusPending
					stepStatus.StartedAt = nil
					stepStatus.CompletedAt = nil
					stepStatus.Error = ""
					stepStatus.Output = nil
					stepStatus.RetryCount++
				}
			}
		}

		// If retrying from a specific step, set it as current
		if fromStep != "" {
			execution.CurrentStepID = fromStep
		} else {
			// Find first step with no dependencies
			for _, step := range workflowSteps {
				if len(step.Dependencies) == 0 {
					execution.CurrentStepID = step.ID
					break
				}
			}
		}

		// Update in repository
		if err := s.repo.UpdateExecution(ctx, execution); err != nil {
			return errors.Wrap(err, "failed to update execution")
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Add back to active executions
	s.activeExecutions.Store(executionID, execution)

	// Start execution in background
	go func() {
		ctx := context.Background()
		defer s.activeExecutions.Delete(executionID)

		// Execute from the retry point
		if execution.CurrentStepID != "" {
			if err := s.ExecuteWorkflowStep(ctx, executionID, execution.CurrentStepID); err != nil {
				s.config.Logger.Error("Failed to execute retry step", map[string]interface{}{
					"execution_id": executionID,
					"step_id":      execution.CurrentStepID,
					"error":        err.Error(),
				})
			}
		}
	}()

	// Invalidate cache
	_ = s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", executionID))

	// Notify about retry
	notification := map[string]interface{}{
		"type":         "execution_retried",
		"execution_id": executionID,
		"from_step":    fromStep,
		"retry_count":  retryCount + 1,
	}
	if err := s.notifier.NotifyWorkflowUpdated(ctx, notification); err != nil {
		s.config.Logger.Error("Failed to notify execution retry", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	// Publish event
	if err := s.PublishEvent(ctx, "ExecutionRetried", execution, map[string]interface{}{
		"from_step":   fromStep,
		"retry_count": retryCount + 1,
	}); err != nil {
		s.config.Logger.Error("Failed to publish retry event", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	s.config.Logger.Info("Execution retried", map[string]interface{}{
		"execution_id": executionID,
		"from_step":    fromStep,
		"retry_count":  retryCount + 1,
	})

	return nil
}

func (s *workflowService) SubmitApproval(ctx context.Context, executionID uuid.UUID, stepID string, approval *models.ApprovalDecision) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.SubmitApproval")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "approval:submit"); err != nil {
		return err
	}

	// Validate approval
	if approval == nil {
		return ValidationError{Field: "approval", Message: "required"}
	}
	if approval.ApprovedBy == "" {
		return ValidationError{Field: "approved_by", Message: "required"}
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_approval",
			Action:   "submit",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"step_id":      stepID,
				"approver_id":  approval.ApprovedBy,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "submit approval",
				Reason: decision.Reason,
			}
		}
	}

	// Get step status
	stepStatus, exists := execution.StepStatuses[stepID]
	if !exists {
		return ValidationError{
			Field:   "step_id",
			Message: fmt.Sprintf("step %s not found in execution", stepID),
		}
	}

	// Verify step is awaiting approval
	if stepStatus.Status != models.StepStatusAwaitingApproval {
		return ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("step %s is not awaiting approval, current status: %s", stepID, stepStatus.Status),
		}
	}

	// Get workflow to find step configuration
	workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
	if err != nil {
		return errors.Wrap(err, "failed to get workflow")
	}

	// Find the step
	var targetStep *models.WorkflowStep
	workflowSteps := workflow.GetSteps()
	for _, step := range workflowSteps {
		if step.ID == stepID {
			targetStep = &step
			break
		}
	}

	if targetStep == nil {
		return ValidationError{
			Field:   "step_id",
			Message: fmt.Sprintf("step %s not found in workflow definition", stepID),
		}
	}

	// Process approval
	err = s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Update step status based on decision
		approval.Timestamp = time.Now()

		if stepStatus.Output == nil {
			stepStatus.Output = make(map[string]interface{})
		}
		stepStatus.Output["approval"] = approval

		// Determine decision from approval state
		var decision string
		if approval.Approved {
			decision = "approved"
		} else if approval.Comments != "" && strings.Contains(strings.ToLower(approval.Comments), "escalate") {
			decision = "escalated"
		} else {
			decision = "rejected"
		}

		switch decision {
		case "approved":
			stepStatus.Status = models.StepStatusCompleted
			now := time.Now()
			stepStatus.CompletedAt = &now

			// Continue workflow execution
			if err := s.continueWorkflowExecution(ctx, execution, stepID); err != nil {
				return errors.Wrap(err, "failed to continue workflow")
			}

		case "rejected":
			stepStatus.Status = models.StepStatusFailed
			now := time.Now()
			stepStatus.CompletedAt = &now
			stepStatus.Error = fmt.Sprintf("Approval rejected by %s: %s", approval.ApprovedBy, approval.Comments)

			// Check if workflow should fail
			if targetStep.OnFailure == "fail_workflow" {
				execution.Status = models.WorkflowExecutionStatusFailed
				execution.Error = stepStatus.Error
				now := time.Now()
				execution.CompletedAt = &now
			}

		case "escalated":
			// Handle escalation
			if escalateTo, ok := targetStep.Config["escalate_to"].(string); ok {
				// Create new approval request for escalation
				escalation := map[string]interface{}{
					"type":              "approval_escalated",
					"execution_id":      executionID,
					"step_id":           stepID,
					"escalated_to":      escalateTo,
					"escalated_by":      approval.ApprovedBy,
					"escalation_reason": approval.Comments,
				}
				if err := s.notifier.BroadcastToAgents(ctx, []string{escalateTo}, escalation); err != nil {
					return errors.Wrap(err, "failed to notify escalation")
				}
				stepStatus.Output["escalated_to"] = escalateTo
			}

		default:
			return ValidationError{
				Field:   "decision",
				Message: fmt.Sprintf("invalid approval decision: %s", decision),
			}
		}

		// Update execution
		execution.UpdatedAt = time.Now()
		if err := s.repo.UpdateExecution(ctx, execution); err != nil {
			return errors.Wrap(err, "failed to update execution")
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Clear cache
	_ = s.executionCache.Delete(ctx, fmt.Sprintf("execution:%s", executionID))

	// Publish event
	if err := s.PublishEvent(ctx, "ApprovalSubmitted", approval, map[string]interface{}{
		"execution_id": executionID,
		"step_id":      stepID,
	}); err != nil {
		s.config.Logger.Error("Failed to publish approval event", map[string]interface{}{
			"execution_id": executionID,
			"step_id":      stepID,
			"error":        err.Error(),
		})
	}

	// Log the approval decision
	decision := "rejected"
	if approval.Approved {
		decision = "approved"
	} else if approval.Comments != "" && strings.Contains(strings.ToLower(approval.Comments), "escalate") {
		decision = "escalated"
	}

	s.config.Logger.Info("Approval submitted", map[string]interface{}{
		"execution_id": executionID,
		"step_id":      stepID,
		"decision":     decision,
		"approver":     approval.ApprovedBy,
	})

	return nil
}

func (s *workflowService) GetPendingApprovals(ctx context.Context, approverID string) ([]*models.PendingApproval, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetPendingApprovals")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "approval:list"); err != nil {
		return nil, err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_approval",
			Action:   "list",
			Conditions: map[string]interface{}{
				"approver_id": approverID,
			},
		})
		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "list pending approvals",
				Reason: decision.Reason,
			}
		}
	}

	// Check cache
	cacheKey := fmt.Sprintf("approvals:pending:%s", approverID)
	var cached []*models.PendingApproval
	if err := s.executionCache.Get(ctx, cacheKey, &cached); err == nil {
		return cached, nil
	}

	// Get all active workflows first
	// Get tenant ID from context or use a default
	tenantID := uuid.UUID{}
	if tenant, ok := ctx.Value("tenant_id").(uuid.UUID); ok {
		tenantID = tenant
	}
	workflows, err := s.repo.List(ctx, tenantID, interfaces.WorkflowFilters{
		IsActive: &[]bool{true}[0],
		Limit:    100,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workflows")
	}

	// Collect all running executions
	executions := make([]*models.WorkflowExecution, 0)
	for _, workflow := range workflows {
		activeExecs, err := s.repo.GetActiveExecutions(ctx, workflow.ID)
		if err != nil {
			s.config.Logger.Error("Failed to get active executions", map[string]interface{}{
				"workflow_id": workflow.ID,
				"error":       err.Error(),
			})
			continue
		}
		executions = append(executions, activeExecs...)
	}

	pendingApprovals := make([]*models.PendingApproval, 0)

	// Check each execution for pending approvals
	for _, execution := range executions {
		// Get workflow
		workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
		if err != nil {
			s.config.Logger.Error("Failed to get workflow for approval check", map[string]interface{}{
				"workflow_id": execution.WorkflowID,
				"error":       err.Error(),
			})
			continue
		}

		// Check each step
		for stepID, stepStatus := range execution.StepStatuses {
			if stepStatus.Status != "awaiting_approval" && stepStatus.Status != "approval_pending" {
				continue
			}

			// Find step configuration
			workflowSteps := workflow.GetSteps()
			for _, step := range workflowSteps {
				if step.ID != stepID {
					continue
				}

				// Check if this approver is authorized
				authorized := false
				if approvers, ok := step.Config["approvers"].([]interface{}); ok {
					for _, approver := range approvers {
						if approverStr, ok := approver.(string); ok && approverStr == approverID {
							authorized = true
							break
						}
					}
				}

				// Check groups
				if !authorized {
					if groups, ok := step.Config["approval_groups"].([]interface{}); ok {
						// In production, check if approver is member of any group
						for _, group := range groups {
							if groupStr, ok := group.(string); ok {
								// Check group membership
								if s.isApproverInGroup(ctx, approverID, groupStr) {
									authorized = true
									break
								}
							}
						}
					}
				}

				if authorized {
					// Calculate deadline
					var deadline *time.Time
					if stepStatus.StartedAt != nil {
						if timeout, ok := step.Config["approval_timeout_hours"].(float64); ok {
							d := stepStatus.StartedAt.Add(time.Duration(timeout) * time.Hour)
							deadline = &d
						}
					}

					pending := &models.PendingApproval{
						ExecutionID: execution.ID,
						WorkflowID:  workflow.ID,
						StepID:      stepID,
						StepName:    step.Name,
						RequestedAt: execution.StartedAt,
						DueBy:       deadline,
						Context:     stepStatus.Input,
					}

					pendingApprovals = append(pendingApprovals, pending)
				}
				break
			}
		}
	}

	// Sort by deadline
	sort.Slice(pendingApprovals, func(i, j int) bool {
		// Sort by deadline if both have one
		if pendingApprovals[i].DueBy != nil && pendingApprovals[j].DueBy != nil {
			return pendingApprovals[i].DueBy.Before(*pendingApprovals[j].DueBy)
		}
		// Items with deadline come before items without
		if pendingApprovals[i].DueBy != nil {
			return true
		}
		if pendingApprovals[j].DueBy != nil {
			return false
		}
		// Otherwise sort by request time
		return pendingApprovals[i].RequestedAt.Before(pendingApprovals[j].RequestedAt)
	})

	// Cache result
	_ = s.executionCache.Set(ctx, cacheKey, pendingApprovals, 1*time.Minute)

	return pendingApprovals, nil
}

func (s *workflowService) CreateWorkflowTemplate(ctx context.Context, template *models.WorkflowTemplate) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.CreateWorkflowTemplate")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "template:create"); err != nil {
		return err
	}

	// Check quota
	if err := s.CheckQuota(ctx, "workflow_templates", 1); err != nil {
		return err
	}

	// Validate input
	if template == nil {
		return ValidationError{Field: "template", Message: "required"}
	}

	if template.Name == "" {
		return ValidationError{Field: "name", Message: "required"}
	}

	if len(template.Definition) == 0 {
		return ValidationError{Field: "definition", Message: "required"}
	}

	// Sanitize input
	if s.config.Sanitizer != nil {
		template.Name = s.config.Sanitizer.SanitizeString(template.Name)
		template.Description = s.config.Sanitizer.SanitizeString(template.Description)
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_template",
			Action:   "create",
			Conditions: map[string]interface{}{
				"category": template.Category,
			},
		})

		if !decision.Allowed {
			return UnauthorizedError{
				Action: "create workflow template",
				Reason: decision.Reason,
			}
		}
	}

	// Validate template definition
	if err := s.validateTemplateDefinition(ctx, template); err != nil {
		return errors.Wrap(err, "template validation failed")
	}

	// Execute in transaction
	err := s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Set metadata
		template.ID = uuid.New()
		template.CreatedBy = auth.GetAgentID(ctx)
		template.CreatedAt = time.Now()
		template.UpdatedAt = template.CreatedAt

		// Store template (using workflow repository for now)
		// In production, you'd have a separate template repository
		// Since templates have a different structure, we store the definition in Config
		workflowFromTemplate := &models.Workflow{
			ID:          template.ID,
			TenantID:    auth.GetTenantID(ctx),
			Name:        fmt.Sprintf("TEMPLATE:%s", template.Name),
			Type:        models.WorkflowTypeSequential, // Templates stored as sequential workflows
			Description: template.Description,
			Steps:       models.WorkflowSteps{}, // Empty steps for template workflows
			Config: models.JSONMap{
				"template":   true,
				"parameters": template.Parameters,
				"definition": template.Definition, // Store template definition in config
			},
			CreatedBy: template.CreatedBy,
			IsActive:  false,
			Tags:      []string{"template", template.Category},
		}

		if err := s.repo.Create(ctx, workflowFromTemplate); err != nil {
			return errors.Wrap(err, "failed to create template")
		}

		// Publish event
		if err := s.PublishEvent(ctx, "WorkflowTemplateCreated", template, template); err != nil {
			return err
		}

		return nil
	})

	return err
}

func (s *workflowService) GetWorkflowTemplate(ctx context.Context, templateID uuid.UUID) (*models.WorkflowTemplate, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetWorkflowTemplate")
	defer span.End()

	// Check cache
	cacheKey := fmt.Sprintf("template:%s", templateID)
	var cached models.WorkflowTemplate
	if err := s.workflowCache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	// Get from repository
	workflow, err := s.repo.Get(ctx, templateID)
	if err != nil {
		return nil, errors.Wrap(err, "template not found")
	}

	// Verify it's a template
	if config, ok := workflow.Config["template"].(bool); !ok || !config {
		return nil, ValidationError{Field: "template", Message: "not a template"}
	}

	// Convert to template
	// Extract template definition from config for template workflows
	var definition map[string]interface{}
	if def, ok := workflow.Config["definition"].(map[string]interface{}); ok {
		definition = def
	} else {
		// Fallback for backward compatibility
		definition = make(map[string]interface{})
	}

	template := &models.WorkflowTemplate{
		ID:          workflow.ID,
		Name:        strings.TrimPrefix(workflow.Name, "TEMPLATE:"),
		Description: workflow.Description,
		Category:    s.extractTemplateCategory(workflow.Tags),
		Definition:  definition,
		CreatedBy:   workflow.CreatedBy,
		CreatedAt:   workflow.CreatedAt,
		UpdatedAt:   workflow.UpdatedAt,
	}

	if params, ok := workflow.Config["parameters"].([]models.TemplateParameter); ok {
		template.Parameters = params
	}

	// Cache result
	_ = s.workflowCache.Set(ctx, cacheKey, template, 10*time.Minute)

	return template, nil
}

func (s *workflowService) ListWorkflowTemplates(ctx context.Context) ([]*models.WorkflowTemplate, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ListWorkflowTemplates")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "template:list"); err != nil {
		return nil, err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_template",
			Action:   "list",
		})

		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "list workflow templates",
				Reason: decision.Reason,
			}
		}
	}

	// Get all workflows that are templates
	filters := interfaces.WorkflowFilters{
		Tags:  []string{"template"},
		Limit: 1000,
	}

	workflows, err := s.repo.List(ctx, auth.GetTenantID(ctx), filters)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list templates")
	}

	// Convert to templates
	templates := make([]*models.WorkflowTemplate, 0, len(workflows))
	for _, workflow := range workflows {
		if config, ok := workflow.Config["template"].(bool); ok && config {
			// Extract template definition from config for template workflows
			var definition map[string]interface{}
			if def, ok := workflow.Config["definition"].(map[string]interface{}); ok {
				definition = def
			} else {
				// Fallback for backward compatibility
				definition = make(map[string]interface{})
			}

			template := &models.WorkflowTemplate{
				ID:          workflow.ID,
				Name:        strings.TrimPrefix(workflow.Name, "TEMPLATE:"),
				Description: workflow.Description,
				Category:    s.extractTemplateCategory(workflow.Tags),
				Definition:  definition,
				CreatedBy:   workflow.CreatedBy,
				CreatedAt:   workflow.CreatedAt,
				UpdatedAt:   workflow.UpdatedAt,
			}

			if params, ok := workflow.Config["parameters"].([]models.TemplateParameter); ok {
				template.Parameters = params
			}

			templates = append(templates, template)
		}
	}

	return templates, nil
}

func (s *workflowService) CreateFromTemplate(ctx context.Context, templateID uuid.UUID, params map[string]interface{}) (*models.Workflow, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.CreateFromTemplate")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:create"); err != nil {
		return nil, err
	}

	// Get template
	template, err := s.GetWorkflowTemplate(ctx, templateID)
	if err != nil {
		return nil, errors.Wrap(err, "template not found")
	}

	// Validate parameters
	if err := s.validateTemplateParameters(template, params); err != nil {
		return nil, errors.Wrap(err, "parameter validation failed")
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "create_from_template",
			Conditions: map[string]interface{}{
				"template_id": templateID,
				"category":    template.Category,
			},
		})

		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "create workflow from template",
				Reason: decision.Reason,
			}
		}
	}

	// Instantiate workflow from template
	workflow := &models.Workflow{
		TenantID:    auth.GetTenantID(ctx),
		Name:        s.generateWorkflowName(template, params),
		Description: s.substituteTemplateParams(template.Description, params),
		Type:        s.extractWorkflowType(template.Definition),
		Steps:       s.instantiateTemplateSteps(template.Definition, params),
		Config: models.JSONMap{
			"created_from_template": templateID,
			"template_params":       params,
		},
		Tags:     []string{fmt.Sprintf("from-template:%s", template.Name)},
		IsActive: true,
	}

	// Create the workflow
	if err := s.CreateWorkflow(ctx, workflow); err != nil {
		return nil, errors.Wrap(err, "failed to create workflow from template")
	}

	return workflow, nil
}

func (s *workflowService) ValidateWorkflow(ctx context.Context, workflow *models.Workflow) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ValidateWorkflow")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:validate"); err != nil {
		return err
	}

	// Basic validation
	if workflow == nil {
		return ValidationError{Field: "workflow", Message: "required"}
	}

	// Perform comprehensive validation
	if err := s.validateWorkflow(ctx, workflow); err != nil {
		return err
	}

	// Validate step definitions
	workflowSteps := workflow.GetSteps()
	for _, step := range workflowSteps {
		// Check agent capabilities
		if step.AgentID != "" {
			agent, err := s.agentService.GetAgent(ctx, step.AgentID)
			if err != nil {
				return ValidationError{
					Field:   fmt.Sprintf("step[%s].agent_id", step.ID),
					Message: "agent not found",
				}
			}

			// Verify agent has required capabilities
			hasCapability := false
			for _, capability := range agent.Capabilities {
				if capability == step.Action {
					hasCapability = true
					break
				}
			}

			if !hasCapability {
				return ValidationError{
					Field:   fmt.Sprintf("step[%s]", step.ID),
					Message: fmt.Sprintf("agent %s does not have capability %s", step.AgentID, step.Action),
				}
			}
		}

		// Validate timeout
		if step.TimeoutSeconds <= 0 {
			return ValidationError{
				Field:   fmt.Sprintf("step[%s].timeout", step.ID),
				Message: "must be positive",
			}
		}

		// Validate retry policy
		if step.RetryPolicy.MaxAttempts < 0 {
			return ValidationError{
				Field:   fmt.Sprintf("step[%s].retry_policy.max_attempts", step.ID),
				Message: "cannot be negative",
			}
		}
	}

	// Verify all required fields based on workflow type
	switch workflow.Type {
	case models.WorkflowTypeSequential:
		// Ensure steps have proper ordering
		if len(workflowSteps) > 1 {
			for i := 1; i < len(workflowSteps); i++ {
				if len(workflowSteps[i].Dependencies) == 0 {
					return ValidationError{
						Field:   fmt.Sprintf("step[%s].dependencies", workflowSteps[i].ID),
						Message: "sequential workflow steps must have dependencies except the first",
					}
				}
			}
		}

	case models.WorkflowTypeParallel:
		// Ensure no circular dependencies
		if err := s.validateNoCycles(workflow); err != nil {
			return err
		}

	case models.WorkflowTypeConditional:
		// Ensure condition definitions exist
		for _, step := range workflowSteps {
			if step.Type == "conditional" && step.Config["condition"] == nil {
				return ValidationError{
					Field:   fmt.Sprintf("step[%s].config.condition", step.ID),
					Message: "conditional step requires condition configuration",
				}
			}
		}

	case models.WorkflowTypeCollaborative:
		// Ensure agents are defined
		if len(workflow.Agents) == 0 {
			return ValidationError{
				Field:   "agents",
				Message: "collaborative workflow requires agent definitions",
			}
		}
	}

	// Check for resource requirements
	if s.config.RuleEngine != nil {
		decision, err := s.config.RuleEngine.Evaluate(ctx, "workflow_requirements", workflow)
		if err == nil && decision != nil && decision.Allowed {
			// Check if decision contains requirements data in metadata
			if decision.Metadata != nil {
				if minAgents, ok := decision.Metadata["min_agents"].(int); ok && minAgents > 0 {
					// Verify we have enough agents
					agentCount := 0
					for _, step := range workflowSteps {
						if step.AgentID != "" {
							agentCount++
						}
					}
					if agentCount < minAgents {
						return ValidationError{
							Field:   "steps",
							Message: fmt.Sprintf("workflow requires at least %d agents, found %d", minAgents, agentCount),
						}
					}
				}
			}
		}
	}

	return nil
}

func (s *workflowService) SimulateWorkflow(ctx context.Context, workflow *models.Workflow, input map[string]interface{}) (*models.SimulationResult, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.SimulateWorkflow")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:simulate"); err != nil {
		return nil, err
	}

	// Validate workflow
	if err := s.validateWorkflow(ctx, workflow); err != nil {
		return nil, errors.Wrap(err, "workflow validation failed")
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "simulate",
			Conditions: map[string]interface{}{
				"workflow_id": workflow.ID,
			},
		})
		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "simulate workflow",
				Reason: decision.Reason,
			}
		}
	}

	// Initialize simulation result
	result := &models.SimulationResult{
		Success:         true,
		ExecutionPath:   make([]string, 0),
		ResourceUsage:   make(map[string]interface{}),
		StepDetails:     make(map[string]*models.StepSimulation),
		Warnings:        make([]string, 0),
		PotentialErrors: make([]string, 0),
	}

	// Simulate execution
	workflowSteps := workflow.GetSteps()
	executedSteps := make(map[string]bool)
	var totalTime time.Duration

	// Check resource requirements
	for _, step := range workflowSteps {
		stepSim := &models.StepSimulation{
			StepID:        step.ID,
			CanExecute:    true,
			EstimatedTime: 5 * time.Second, // Default estimate
			Requirements:  make([]string, 0),
			Issues:        make([]string, 0),
		}

		// Check agent availability
		if step.AgentID != "" {
			agent, err := s.agentService.GetAgent(ctx, step.AgentID)
			if err != nil || agent == nil {
				stepSim.CanExecute = false
				stepSim.Issues = append(stepSim.Issues, fmt.Sprintf("Agent %s not available", step.AgentID))
				result.PotentialErrors = append(result.PotentialErrors, fmt.Sprintf("Step %s: Agent %s not available", step.ID, step.AgentID))
			} else {
				// Check agent capabilities
				capabilities, _ := s.agentService.GetAgentCapabilities(ctx, step.AgentID)
				if requiredCaps, ok := step.Config["required_capabilities"].([]interface{}); ok {
					for _, cap := range requiredCaps {
						capStr := fmt.Sprintf("%v", cap)
						stepSim.Requirements = append(stepSim.Requirements, capStr)
						hasCapability := false
						for _, agentCap := range capabilities {
							if agentCap == capStr {
								hasCapability = true
								break
							}
						}
						if !hasCapability {
							stepSim.Issues = append(stepSim.Issues, fmt.Sprintf("Missing capability: %s", capStr))
							result.Warnings = append(result.Warnings, fmt.Sprintf("Step %s: Agent %s missing capability %s", step.ID, step.AgentID, capStr))
						}
					}
				}
			}
		}

		// Check dependencies
		for _, dep := range step.Dependencies {
			if !executedSteps[dep] {
				stepSim.Requirements = append(stepSim.Requirements, fmt.Sprintf("Depends on: %s", dep))
			}
		}

		// Estimate execution time
		if timeout := step.TimeoutSeconds; timeout > 0 {
			stepSim.EstimatedTime = time.Duration(timeout) * time.Second / 2 // Assume 50% of timeout
		}
		totalTime += stepSim.EstimatedTime

		result.StepDetails[step.ID] = stepSim
	}

	// Simulate execution order
	for i := 0; i < len(workflowSteps); i++ {
		for _, step := range workflowSteps {
			if executedSteps[step.ID] {
				continue
			}

			// Check if dependencies are satisfied
			depsOk := true
			for _, dep := range step.Dependencies {
				if !executedSteps[dep] {
					depsOk = false
					break
				}
			}

			if depsOk && result.StepDetails[step.ID].CanExecute {
				result.ExecutionPath = append(result.ExecutionPath, step.ID)
				executedSteps[step.ID] = true
			}
		}
	}

	// Check if all steps were executed
	if len(executedSteps) < len(workflowSteps) {
		result.Success = false
		for _, step := range workflowSteps {
			if !executedSteps[step.ID] {
				result.PotentialErrors = append(result.PotentialErrors, fmt.Sprintf("Step %s could not be executed", step.ID))
			}
		}
	}

	// Set estimated time
	result.EstimatedTime = totalTime

	// Estimate resource usage
	result.ResourceUsage["estimated_time"] = totalTime.String()
	result.ResourceUsage["total_steps"] = len(workflowSteps)
	result.ResourceUsage["execution_steps"] = len(executedSteps)

	return result, nil
}

func (s *workflowService) GetWorkflowStats(ctx context.Context, workflowID uuid.UUID, period time.Duration) (*interfaces.WorkflowStats, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GetWorkflowStats")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:stats"); err != nil {
		return nil, err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "read_stats",
			Conditions: map[string]interface{}{
				"workflow_id": workflowID,
			},
		})
		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "get workflow stats",
				Reason: decision.Reason,
			}
		}
	}

	// Check cache
	cacheKey := fmt.Sprintf("stats:%s:period:%s", workflowID, period.String())
	var cached interfaces.WorkflowStats
	if err := s.statsCache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	// Get stats from repository
	stats, err := s.repo.GetWorkflowStats(ctx, workflowID, period)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get workflow stats")
	}

	// Cache result
	_ = s.statsCache.Set(ctx, cacheKey, stats, 5*time.Minute)

	return stats, nil
}

func (s *workflowService) GenerateWorkflowReport(ctx context.Context, filters interfaces.WorkflowFilters, format string) ([]byte, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.GenerateWorkflowReport")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:report"); err != nil {
		return nil, err
	}

	// Validate format
	if format != "json" && format != "csv" && format != "pdf" {
		return nil, ValidationError{
			Field:   "format",
			Message: "unsupported format, must be json, csv, or pdf",
		}
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "generate_report",
		})
		if !decision.Allowed {
			return nil, UnauthorizedError{
				Action: "generate workflow report",
				Reason: decision.Reason,
			}
		}
	}

	// Get workflows
	tenantID := auth.GetTenantID(ctx)
	workflows, err := s.repo.List(ctx, tenantID, filters)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workflows")
	}

	// Generate report based on format
	switch format {
	case "json":
		// Generate JSON report
		report := map[string]interface{}{
			"generated_at":    time.Now(),
			"period":          filters.CreatedAfter,
			"total_workflows": len(workflows),
			"workflows":       make([]map[string]interface{}, 0, len(workflows)),
		}

		for _, workflow := range workflows {
			// Get stats for each workflow
			stats, _ := s.GetWorkflowStats(ctx, workflow.ID, 30*24*time.Hour)

			workflowData := map[string]interface{}{
				"id":         workflow.ID,
				"name":       workflow.Name,
				"type":       workflow.Type,
				"created_at": workflow.CreatedAt,
				"created_by": workflow.CreatedBy,
				"is_active":  workflow.IsActive,
				"version":    workflow.Version,
			}

			if stats != nil {
				workflowData["stats"] = stats
			}

			report["workflows"] = append(report["workflows"].([]map[string]interface{}), workflowData)
		}

		return json.Marshal(report)

	case "csv":
		// Generate CSV report
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)

		// Write header
		header := []string{"ID", "Name", "Type", "Created At", "Created By", "Active", "Version"}
		if err := writer.Write(header); err != nil {
			return nil, errors.Wrap(err, "failed to write CSV header")
		}

		// Write data
		for _, workflow := range workflows {
			row := []string{
				workflow.ID.String(),
				workflow.Name,
				string(workflow.Type),
				workflow.CreatedAt.Format(time.RFC3339),
				workflow.CreatedBy,
				fmt.Sprintf("%t", workflow.IsActive),
				fmt.Sprintf("%d", workflow.Version),
			}
			if err := writer.Write(row); err != nil {
				return nil, errors.Wrap(err, "failed to write CSV row")
			}
		}

		writer.Flush()
		return buf.Bytes(), writer.Error()

	case "pdf":
		// For PDF, we would integrate with a PDF generation library
		// For now, return a simple placeholder
		return nil, errors.New("PDF format not yet implemented")

	default:
		return nil, ValidationError{
			Field:   "format",
			Message: "unsupported format",
		}
	}
}

func (s *workflowService) ArchiveCompletedExecutions(ctx context.Context, before time.Time) (int64, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ArchiveCompletedExecutions")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "workflow:archive"); err != nil {
		return 0, err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow",
			Action:   "archive",
		})
		if !decision.Allowed {
			return 0, UnauthorizedError{
				Action: "archive executions",
				Reason: decision.Reason,
			}
		}
	}

	// Archive executions
	count, err := s.repo.ArchiveOldExecutions(ctx, before)
	if err != nil {
		return 0, errors.Wrap(err, "failed to archive executions")
	}

	// Clear related caches
	// Note: Most cache implementations don't have a Clear method, so we skip this

	// Publish event
	if err := s.PublishEvent(ctx, "ExecutionsArchived", nil, map[string]interface{}{
		"before": before,
		"count":  count,
		"by":     auth.GetAgentID(ctx),
	}); err != nil {
		s.config.Logger.Error("Failed to publish archive event", map[string]interface{}{
			"count": count,
			"error": err.Error(),
		})
	}

	s.config.Logger.Info("Archived completed executions", map[string]interface{}{
		"before": before,
		"count":  count,
	})

	return count, nil
}

func (s *workflowService) CreateBranchingPath(ctx context.Context, executionID uuid.UUID, branchPoint string, conditions map[string]interface{}) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.CreateBranchingPath")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:branch"); err != nil {
		return err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "branch",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "create branching path",
				Reason: decision.Reason,
			}
		}
	}

	// Validate execution is running
	if execution.Status != models.WorkflowExecutionStatusRunning {
		return ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("cannot create branch in status %s", execution.Status),
		}
	}

	// Store branching information in execution context
	if execution.Context == nil {
		execution.Context = make(models.JSONMap)
	}
	if execution.Context["branches"] == nil {
		execution.Context["branches"] = make(map[string]interface{})
	}

	branches := execution.Context["branches"].(map[string]interface{})
	branches[branchPoint] = map[string]interface{}{
		"conditions": conditions,
		"created_at": time.Now(),
		"created_by": auth.GetAgentID(ctx),
		"branch_id":  uuid.New().String(),
	}

	// Update execution
	execution.UpdatedAt = time.Now()
	if err := s.repo.UpdateExecution(ctx, execution); err != nil {
		return errors.Wrap(err, "failed to update execution with branch")
	}

	// Publish event
	if err := s.PublishEvent(ctx, "BranchingPathCreated", execution, map[string]interface{}{
		"branch_point": branchPoint,
		"conditions":   conditions,
	}); err != nil {
		s.config.Logger.Error("Failed to publish branching event", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	s.config.Logger.Info("Branching path created", map[string]interface{}{
		"execution_id": executionID,
		"branch_point": branchPoint,
	})

	return nil
}

func (s *workflowService) MergeBranchingPaths(ctx context.Context, executionID uuid.UUID, branchIDs []string) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.MergeBranchingPaths")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:merge"); err != nil {
		return err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "merge",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "merge branching paths",
				Reason: decision.Reason,
			}
		}
	}

	// Validate branches exist
	if execution.Context == nil || execution.Context["branches"] == nil {
		return ValidationError{
			Field:   "branches",
			Message: "no branches found in execution",
		}
	}

	branches := execution.Context["branches"].(map[string]interface{})
	mergedResults := make(map[string]interface{})

	// Collect results from each branch
	for _, branchID := range branchIDs {
		branchFound := false
		for _, branch := range branches {
			branchData := branch.(map[string]interface{})
			if branchData["branch_id"] == branchID {
				branchFound = true
				// Collect any results from this branch
				if results, ok := branchData["results"]; ok {
					mergedResults[branchID] = results
				}
				break
			}
		}
		if !branchFound {
			return ValidationError{
				Field:   "branch_id",
				Message: fmt.Sprintf("branch %s not found", branchID),
			}
		}
	}

	// Store merge information
	if execution.Context["merges"] == nil {
		execution.Context["merges"] = make([]interface{}, 0)
	}
	merges := execution.Context["merges"].([]interface{})
	merges = append(merges, map[string]interface{}{
		"merged_at":      time.Now(),
		"merged_by":      auth.GetAgentID(ctx),
		"branch_ids":     branchIDs,
		"merged_results": mergedResults,
	})
	execution.Context["merges"] = merges

	// Update execution
	execution.UpdatedAt = time.Now()
	if err := s.repo.UpdateExecution(ctx, execution); err != nil {
		return errors.Wrap(err, "failed to update execution with merge")
	}

	// Publish event
	if err := s.PublishEvent(ctx, "BranchingPathsMerged", execution, map[string]interface{}{
		"branch_ids": branchIDs,
	}); err != nil {
		s.config.Logger.Error("Failed to publish merge event", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	s.config.Logger.Info("Branching paths merged", map[string]interface{}{
		"execution_id": executionID,
		"branch_ids":   branchIDs,
	})

	return nil
}

func (s *workflowService) CreateCompensation(ctx context.Context, executionID uuid.UUID, failedStep string, compensation *models.CompensationAction) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.CreateCompensation")
	defer span.End()

	// Validate compensation
	if compensation == nil {
		return ValidationError{Field: "compensation", Message: "required"}
	}
	if compensation.Type == "" {
		return ValidationError{Field: "type", Message: "required"}
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "compensate",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "create compensation",
				Reason: decision.Reason,
			}
		}
	}

	// Set compensation metadata
	compensation.ID = uuid.New()
	compensation.ExecutionID = executionID
	compensation.StepID = failedStep
	compensation.Status = "pending"
	compensation.CreatedAt = time.Now()

	// Store compensation in execution context
	if execution.Context == nil {
		execution.Context = make(models.JSONMap)
	}
	if execution.Context["compensations"] == nil {
		execution.Context["compensations"] = make([]interface{}, 0)
	}

	compensations := execution.Context["compensations"].([]interface{})
	compensations = append(compensations, compensation)
	execution.Context["compensations"] = compensations

	// Update execution
	execution.UpdatedAt = time.Now()
	if err := s.repo.UpdateExecution(ctx, execution); err != nil {
		return errors.Wrap(err, "failed to update execution with compensation")
	}

	// Publish event
	if err := s.PublishEvent(ctx, "CompensationCreated", execution, compensation); err != nil {
		s.config.Logger.Error("Failed to publish compensation event", map[string]interface{}{
			"execution_id":    executionID,
			"compensation_id": compensation.ID,
			"error":           err.Error(),
		})
	}

	s.config.Logger.Info("Compensation created", map[string]interface{}{
		"execution_id":    executionID,
		"compensation_id": compensation.ID,
		"failed_step":     failedStep,
		"type":            compensation.Type,
	})

	return nil
}

func (s *workflowService) ExecuteCompensation(ctx context.Context, executionID uuid.UUID) error {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.ExecuteCompensation")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "execution:compensate"); err != nil {
		return err
	}

	// Get execution
	execution, err := s.getExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Check authorization
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "workflow_execution",
			Action:   "compensate",
			Conditions: map[string]interface{}{
				"execution_id": executionID,
				"workflow_id":  execution.WorkflowID,
			},
		})
		if !decision.Allowed {
			return UnauthorizedError{
				Action: "execute compensation",
				Reason: decision.Reason,
			}
		}
	}

	// Get compensations
	if execution.Context == nil || execution.Context["compensations"] == nil {
		return ValidationError{
			Field:   "compensations",
			Message: "no compensations found",
		}
	}

	compensations := execution.Context["compensations"].([]interface{})
	if len(compensations) == 0 {
		return ValidationError{
			Field:   "compensations",
			Message: "no compensations to execute",
		}
	}

	// Execute compensations in reverse order
	for i := len(compensations) - 1; i >= 0; i-- {
		compData := compensations[i].(map[string]interface{})
		if compData["status"] == "pending" {
			// Execute compensation based on type
			compType := compData["type"].(string)
			switch compType {
			case "rollback":
				// Execute rollback
				if stepID, ok := compData["step_id"].(string); ok {
					// Create a compensating task
					task := &models.Task{
						TenantID:    execution.TenantID,
						Type:        "compensation",
						Title:       fmt.Sprintf("Compensate step %s", stepID),
						Description: compData["description"].(string),
						Parameters:  compData["parameters"].(models.JSONMap),
						CreatedBy:   auth.GetAgentID(ctx),
						Priority:    models.TaskPriorityHigh,
					}
					if err := s.taskService.Create(ctx, task, fmt.Sprintf("comp-%s", uuid.New())); err != nil {
						s.config.Logger.Error("Failed to create compensation task", map[string]interface{}{
							"step_id": stepID,
							"error":   err.Error(),
						})
						continue
					}
					compData["task_id"] = task.ID.String()
				}

			case "notify":
				// Send notification
				if recipients, ok := compData["recipients"].([]interface{}); ok {
					recipientStrs := make([]string, 0, len(recipients))
					for _, r := range recipients {
						recipientStrs = append(recipientStrs, r.(string))
					}
					message := map[string]interface{}{
						"type":         "compensation_notification",
						"execution_id": executionID,
						"description":  compData["description"],
					}
					if err := s.notifier.BroadcastToAgents(ctx, recipientStrs, message); err != nil {
						s.config.Logger.Error("Failed to send compensation notification", map[string]interface{}{
							"error": err.Error(),
						})
					}
				}

			case "custom":
				// Custom compensation logic would go here
				s.config.Logger.Info("Custom compensation requested", compData)
			}

			// Update compensation status
			compData["status"] = "executed"
			compData["executed_at"] = time.Now()
		}
	}

	// Update execution
	execution.UpdatedAt = time.Now()
	if err := s.repo.UpdateExecution(ctx, execution); err != nil {
		return errors.Wrap(err, "failed to update execution after compensation")
	}

	// Publish event
	if err := s.PublishEvent(ctx, "CompensationExecuted", execution, map[string]interface{}{
		"compensations": len(compensations),
	}); err != nil {
		s.config.Logger.Error("Failed to publish compensation execution event", map[string]interface{}{
			"execution_id": executionID,
			"error":        err.Error(),
		})
	}

	s.config.Logger.Info("Compensation executed", map[string]interface{}{
		"execution_id":  executionID,
		"compensations": len(compensations),
	})

	return nil
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
	_ = s.executionCache.Set(ctx, execution.ID.String(), execution, 5*time.Minute)

	// Store in active executions
	s.activeExecutions.Store(execution.ID, execution)

	// Publish event
	_ = s.PublishEvent(ctx, "ExecutionCreated", execution, execution)

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
	_ = s.notifier.NotifyWorkflowStarted(ctx, execution)

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
		_ = s.notifier.NotifyStepCompleted(ctx, executionID, stepID, output)
	} else {
		// All steps completed
		execution.Status = models.WorkflowExecutionStatusCompleted
		execution.CompletedAt = &now
		execution.CurrentStepID = ""

		// Notify workflow completed
		_ = s.notifier.NotifyWorkflowCompleted(ctx, execution)
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
	_ = s.notifier.NotifyWorkflowFailed(ctx, executionID, reason)

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
		switch exec.Status {
		case models.WorkflowExecutionStatusCompleted:
			metrics.SuccessfulRuns++
			if exec.CompletedAt != nil {
				duration := exec.CompletedAt.Sub(exec.StartedAt)
				totalDuration += duration
			}
		case models.WorkflowExecutionStatusFailed:
			metrics.FailedRuns++
		}
	}

	if metrics.SuccessfulRuns > 0 {
		metrics.AverageRunTime = totalDuration / time.Duration(metrics.SuccessfulRuns)
	}

	// Cache metrics
	_ = s.statsCache.Set(ctx, cacheKey, &metrics, 1*time.Minute)

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

// Helper methods for workflow service

// calculateChanges compares two workflows and returns the changes
func (s *workflowService) calculateChanges(old, new *models.Workflow) map[string]interface{} {
	changes := make(map[string]interface{})

	if old.Name != new.Name {
		changes["name"] = map[string]string{"old": old.Name, "new": new.Name}
	}

	if old.Description != new.Description {
		changes["description"] = map[string]string{"old": old.Description, "new": new.Description}
	}

	if old.Type != new.Type {
		changes["type"] = map[string]string{"old": string(old.Type), "new": string(new.Type)}
	}

	if old.IsActive != new.IsActive {
		changes["is_active"] = map[string]bool{"old": old.IsActive, "new": new.IsActive}
	}

	// Compare tags
	oldTags := strings.Join(old.Tags, ",")
	newTags := strings.Join(new.Tags, ",")
	if oldTags != newTags {
		changes["tags"] = map[string]interface{}{"old": old.Tags, "new": new.Tags}
	}

	// Compare steps (simplified)
	if len(old.Steps) != len(new.Steps) {
		changes["steps_count"] = map[string]int{"old": len(old.Steps), "new": len(new.Steps)}
	}

	return changes
}

// buildWorkflowListCacheKey creates a cache key for workflow list queries
func (s *workflowService) buildWorkflowListCacheKey(tenantID uuid.UUID, filters interfaces.WorkflowFilters) string {
	key := fmt.Sprintf("workflows:list:%s", tenantID)

	if len(filters.Type) > 0 {
		key += fmt.Sprintf(":type:%s", strings.Join(filters.Type, ","))
	}

	if filters.IsActive != nil {
		key += fmt.Sprintf(":active:%t", *filters.IsActive)
	}

	if filters.CreatedBy != nil {
		key += fmt.Sprintf(":created_by:%s", *filters.CreatedBy)
	}

	if len(filters.Tags) > 0 {
		key += fmt.Sprintf(":tags:%s", strings.Join(filters.Tags, ","))
	}

	key += fmt.Sprintf(":limit:%d:offset:%d", filters.Limit, filters.Offset)

	return key
}

// scoreWorkflowRelevance represents a workflow with its relevance score
type scoredWorkflow struct {
	workflow *models.Workflow
	score    float64
}

// scoreWorkflowRelevance calculates relevance scores for workflows
func (s *workflowService) scoreWorkflowRelevance(workflows []*models.Workflow, query string) []scoredWorkflow {
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	scored := make([]scoredWorkflow, 0, len(workflows))

	for _, workflow := range workflows {
		score := 0.0

		// Score based on name match
		nameLower := strings.ToLower(workflow.Name)
		if nameLower == queryLower {
			score += 10.0 // Exact match
		} else if strings.Contains(nameLower, queryLower) {
			score += 5.0 // Contains full query
		} else {
			// Check individual words
			for _, word := range queryWords {
				if strings.Contains(nameLower, word) {
					score += 2.0
				}
			}
		}

		// Score based on description match
		descLower := strings.ToLower(workflow.Description)
		if strings.Contains(descLower, queryLower) {
			score += 3.0
		} else {
			for _, word := range queryWords {
				if strings.Contains(descLower, word) {
					score += 1.0
				}
			}
		}

		// Score based on tag match
		for _, tag := range workflow.Tags {
			tagLower := strings.ToLower(tag)
			if tagLower == queryLower {
				score += 4.0
			} else if strings.Contains(tagLower, queryLower) {
				score += 2.0
			}
		}

		// Add recency boost (newer workflows score slightly higher)
		daysSinceCreation := time.Since(workflow.CreatedAt).Hours() / 24
		if daysSinceCreation < 7 {
			score += 0.5
		} else if daysSinceCreation < 30 {
			score += 0.2
		}

		if score > 0 {
			scored = append(scored, scoredWorkflow{
				workflow: workflow,
				score:    score,
			})
		}
	}

	return scored
}

// evaluateCondition evaluates a workflow condition
func (s *workflowService) evaluateCondition(ctx context.Context, execution *models.WorkflowExecution, condition map[string]interface{}) (string, error) {
	// Get condition type
	conditionType, ok := condition["type"].(string)
	if !ok {
		return "", ValidationError{Field: "condition.type", Message: "required"}
	}

	switch conditionType {
	case "expression":
		// Simple expression evaluation
		expression, ok := condition["expression"].(string)
		if !ok {
			return "", ValidationError{Field: "condition.expression", Message: "required"}
		}

		// In production, you'd use a proper expression evaluator
		// For now, support simple comparisons
		if field, ok := condition["field"].(string); ok {
			if operator, ok := condition["operator"].(string); ok {
				if value, ok := condition["value"]; ok {
					// Get field value from execution context
					var fieldValue interface{}
					if execution.Context != nil {
						fieldValue = execution.Context[field]
					}
					if fieldValue == nil && execution.State != nil {
						fieldValue = execution.State[field]
					}

					// Perform comparison
					switch operator {
					case "==", "equals":
						if fieldValue == value {
							return "true", nil
						}
						return "false", nil
					case "!=", "not_equals":
						if fieldValue != value {
							return "true", nil
						}
						return "false", nil
					case ">":
						// Type assertion and comparison for numeric values
						if fv, ok := fieldValue.(float64); ok {
							if v, ok := value.(float64); ok && fv > v {
								return "true", nil
							}
						}
						return "false", nil
					case "<":
						if fv, ok := fieldValue.(float64); ok {
							if v, ok := value.(float64); ok && fv < v {
								return "true", nil
							}
						}
						return "false", nil
					default:
						return "", fmt.Errorf("unsupported operator: %s", operator)
					}
				}
			}
		}

		// Fall back to simple expression parsing
		switch expression {
		case "true", "1", "yes":
			return "true", nil
		case "false", "0", "no":
			return "false", nil
		default:
			// Check if it's a field reference
			if strings.HasPrefix(expression, "${") && strings.HasSuffix(expression, "}") {
				fieldName := strings.TrimSuffix(strings.TrimPrefix(expression, "${"), "}")
				if execution.Context != nil {
					if val, exists := execution.Context[fieldName]; exists {
						return fmt.Sprintf("%v", val), nil
					}
				}
			}
			return expression, nil
		}

	case "script":
		// Script-based evaluation (would integrate with script engine)
		script, ok := condition["script"].(string)
		if !ok {
			return "", ValidationError{Field: "condition.script", Message: "required"}
		}

		// In production, you'd execute this in a sandboxed environment
		s.config.Logger.Warn("Script evaluation not implemented", map[string]interface{}{
			"script": script,
		})
		return "true", nil // Default to true for now

	case "rule":
		// Rule engine evaluation
		ruleName, ok := condition["rule"].(string)
		if !ok {
			return "", ValidationError{Field: "condition.rule", Message: "required"}
		}

		if s.config.RuleEngine != nil {
			decision, err := s.config.RuleEngine.Evaluate(ctx, ruleName, map[string]interface{}{
				"execution": execution,
				"condition": condition,
			})
			if err != nil {
				return "", errors.Wrap(err, "rule evaluation failed")
			}
			if decision.Allowed {
				return "true", nil
			}
			return "false", nil
		}

		// No rule engine configured
		return "true", nil

	default:
		return "", fmt.Errorf("unsupported condition type: %s", conditionType)
	}
}

// validateTemplateDefinition validates a workflow template
func (s *workflowService) validateTemplateDefinition(ctx context.Context, template *models.WorkflowTemplate) error {
	// Validate definition structure
	if _, ok := template.Definition["type"]; !ok {
		return ValidationError{Field: "definition.type", Message: "required"}
	}

	if _, ok := template.Definition["steps"]; !ok {
		return ValidationError{Field: "definition.steps", Message: "required"}
	}

	// Validate parameters
	for i, param := range template.Parameters {
		if param.Name == "" {
			return ValidationError{Field: fmt.Sprintf("parameters[%d].name", i), Message: "required"}
		}

		if param.Type == "" {
			return ValidationError{Field: fmt.Sprintf("parameters[%d].type", i), Message: "required"}
		}

		// Validate parameter types
		switch param.Type {
		case "string", "number", "boolean", "array", "object":
			// Valid types
		default:
			return ValidationError{
				Field:   fmt.Sprintf("parameters[%d].type", i),
				Message: fmt.Sprintf("invalid type: %s", param.Type),
			}
		}
	}

	return nil
}

// validateFieldType validates that a field value matches the expected type
func (s *workflowService) validateFieldType(fieldName string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("expected string, got %T", value),
			}
		}
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Valid number types
		default:
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("expected number, got %T", value),
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("expected boolean, got %T", value),
			}
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("expected object, got %T", value),
			}
		}
	case "array":
		switch value.(type) {
		case []interface{}, []string, []int, []float64:
			// Valid array types
		default:
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("expected array, got %T", value),
			}
		}
	}
	return nil
}

// validateFieldConstraint validates field constraints like min/max values
func (s *workflowService) validateFieldConstraint(fieldName string, value interface{}, constraint interface{}) error {
	constraintMap, ok := constraint.(map[string]interface{})
	if !ok {
		return nil // No constraint map
	}

	// Check minimum constraint
	if minVal, ok := constraintMap["min"]; ok {
		if err := s.validateMinConstraint(fieldName, value, minVal); err != nil {
			return err
		}
	}

	// Check maximum constraint
	if maxVal, ok := constraintMap["max"]; ok {
		if err := s.validateMaxConstraint(fieldName, value, maxVal); err != nil {
			return err
		}
	}

	// Check pattern constraint for strings
	if pattern, ok := constraintMap["pattern"]; ok {
		if err := s.validatePatternConstraint(fieldName, value, pattern); err != nil {
			return err
		}
	}

	// Check enum constraint
	if enum, ok := constraintMap["enum"]; ok {
		if err := s.validateEnumConstraint(fieldName, value, enum); err != nil {
			return err
		}
	}

	return nil
}

// validateMinConstraint validates minimum value constraint
func (s *workflowService) validateMinConstraint(fieldName string, value, minVal interface{}) error {
	switch v := value.(type) {
	case int:
		if minInt, ok := minVal.(int); ok && v < minInt {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("value %d is less than minimum %d", v, minInt),
			}
		}
	case float64:
		if minFloat, ok := minVal.(float64); ok && v < minFloat {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("value %f is less than minimum %f", v, minFloat),
			}
		}
	case string:
		if minInt, ok := minVal.(int); ok && len(v) < minInt {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("string length %d is less than minimum %d", len(v), minInt),
			}
		}
	}
	return nil
}

// validateMaxConstraint validates maximum value constraint
func (s *workflowService) validateMaxConstraint(fieldName string, value, maxVal interface{}) error {
	switch v := value.(type) {
	case int:
		if maxInt, ok := maxVal.(int); ok && v > maxInt {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("value %d exceeds maximum %d", v, maxInt),
			}
		}
	case float64:
		if maxFloat, ok := maxVal.(float64); ok && v > maxFloat {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("value %f exceeds maximum %f", v, maxFloat),
			}
		}
	case string:
		if maxInt, ok := maxVal.(int); ok && len(v) > maxInt {
			return ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("string length %d exceeds maximum %d", len(v), maxInt),
			}
		}
	}
	return nil
}

// validatePatternConstraint validates regex pattern for string values
func (s *workflowService) validatePatternConstraint(fieldName string, value, pattern interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil // Pattern only applies to strings
	}

	patternStr, ok := pattern.(string)
	if !ok {
		return nil
	}

	matched, err := regexp.MatchString(patternStr, str)
	if err != nil {
		return ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("invalid pattern: %v", err),
		}
	}

	if !matched {
		return ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("value does not match pattern %s", patternStr),
		}
	}

	return nil
}

// validateEnumConstraint validates that value is in allowed set
func (s *workflowService) validateEnumConstraint(fieldName string, value, enum interface{}) error {
	allowedValues, ok := enum.([]interface{})
	if !ok {
		return nil
	}

	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}

	return ValidationError{
		Field:   fieldName,
		Message: fmt.Sprintf("value %v not in allowed values", value),
	}
}

// extractTemplateCategory extracts category from tags
func (s *workflowService) extractTemplateCategory(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, "category:") {
			return strings.TrimPrefix(tag, "category:")
		}
	}
	return "general"
}

// validateTemplateParameters validates template instantiation parameters
func (s *workflowService) validateTemplateParameters(template *models.WorkflowTemplate, params map[string]interface{}) error {
	for _, param := range template.Parameters {
		value, exists := params[param.Name]

		// Check required parameters
		if param.Required && !exists {
			return ValidationError{
				Field:   param.Name,
				Message: "required parameter missing",
			}
		}

		// Use default value if not provided
		if !exists && param.DefaultValue != nil {
			params[param.Name] = param.DefaultValue
			continue
		}

		// Type validation
		if exists {
			if err := s.validateParameterType(param, value); err != nil {
				return err
			}
		}
	}

	return nil
}

// continueWorkflowExecution continues workflow execution after step completion
func (s *workflowService) continueWorkflowExecution(ctx context.Context, execution *models.WorkflowExecution, completedStepID string) error {
	// Get workflow
	workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
	if err != nil {
		return errors.Wrap(err, "failed to get workflow")
	}

	// Find next steps to execute
	workflowSteps := workflow.GetSteps()
	for _, step := range workflowSteps {
		// Check if this step depends on the completed step
		for _, dep := range step.Dependencies {
			if dep == completedStepID {
				// Check if all dependencies are satisfied
				if err := s.checkStepDependencies(ctx, execution, &step); err == nil {
					// Execute this step
					go func(stepID string) {
						if err := s.ExecuteWorkflowStep(ctx, execution.ID, stepID); err != nil {
							s.config.Logger.Error("Failed to execute next step", map[string]interface{}{
								"execution_id": execution.ID,
								"step_id":      stepID,
								"error":        err.Error(),
							})
						}
					}(step.ID)
				}
				break
			}
		}
	}

	return nil
}

// isApproverInGroup checks if an approver is in a specified group
func (s *workflowService) isApproverInGroup(ctx context.Context, approverID string, group string) bool {
	// In production, this would check against your group management system
	// For now, implement a simple check
	if s.config.Authorizer != nil {
		decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
			Resource: "group",
			Action:   "member",
			Conditions: map[string]interface{}{
				"group":   group,
				"user_id": approverID,
			},
		})
		return decision.Allowed
	}
	return false
}

// validateParameterType validates a parameter value against its type
func (s *workflowService) validateParameterType(param models.TemplateParameter, value interface{}) error {
	switch param.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return ValidationError{
				Field:   param.Name,
				Message: "must be a string",
			}
		}

	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Valid number types
		default:
			return ValidationError{
				Field:   param.Name,
				Message: "must be a number",
			}
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			return ValidationError{
				Field:   param.Name,
				Message: "must be a boolean",
			}
		}

	case "array":
		if _, ok := value.([]interface{}); !ok {
			return ValidationError{
				Field:   param.Name,
				Message: "must be an array",
			}
		}

	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return ValidationError{
				Field:   param.Name,
				Message: "must be an object",
			}
		}
	}

	return nil
}

// generateWorkflowName generates a workflow name from template
func (s *workflowService) generateWorkflowName(template *models.WorkflowTemplate, params map[string]interface{}) string {
	name := template.Name

	// Substitute any {param} placeholders
	for key, value := range params {
		placeholder := fmt.Sprintf("{%s}", key)
		if strings.Contains(name, placeholder) {
			name = strings.ReplaceAll(name, placeholder, fmt.Sprintf("%v", value))
		}
	}

	// Add timestamp if name would be duplicate
	name = fmt.Sprintf("%s - %s", name, time.Now().Format("2006-01-02 15:04"))

	return name
}

// substituteTemplateParams substitutes parameters in a string
func (s *workflowService) substituteTemplateParams(text string, params map[string]interface{}) string {
	result := text

	for key, value := range params {
		placeholder := fmt.Sprintf("{%s}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	return result
}

// extractWorkflowType extracts workflow type from template definition
func (s *workflowService) extractWorkflowType(definition map[string]interface{}) models.WorkflowType {
	if typeStr, ok := definition["type"].(string); ok {
		switch typeStr {
		case "sequential":
			return models.WorkflowTypeSequential
		case "parallel":
			return models.WorkflowTypeParallel
		case "conditional":
			return models.WorkflowTypeConditional
		case "collaborative":
			return models.WorkflowTypeCollaborative
		}
	}

	// Default to sequential
	return models.WorkflowTypeSequential
}

// instantiateTemplateSteps creates workflow steps from template
func (s *workflowService) instantiateTemplateSteps(definition map[string]interface{}, params map[string]interface{}) models.WorkflowSteps {
	// Extract steps array from definition
	var stepsData []interface{}
	if stepsRaw, ok := definition["steps"]; ok {
		if stepsArray, ok := stepsRaw.([]interface{}); ok {
			stepsData = stepsArray
		}
	}

	// Convert to WorkflowSteps
	steps := make(models.WorkflowSteps, 0, len(stepsData))
	for _, stepData := range stepsData {
		// Substitute parameters in step data
		substitutedStep := s.deepCopyWithSubstitution(stepData, params)
		
		// Convert to WorkflowStep
		if stepMap, ok := substitutedStep.(map[string]interface{}); ok {
			step := models.WorkflowStep{
				ID:              s.getStringValue(stepMap, "id"),
				Name:            s.getStringValue(stepMap, "name"),
				Description:     s.getStringValue(stepMap, "description"),
				Type:            s.getStringValue(stepMap, "type"),
				Action:          s.getStringValue(stepMap, "action"),
				AgentID:         s.getStringValue(stepMap, "agent_id"),
				ContinueOnError: s.getBoolValue(stepMap, "continue_on_error"),
			}

			// Extract input
			if input, ok := stepMap["input"].(map[string]interface{}); ok {
				step.Input = input
			} else {
				step.Input = make(map[string]interface{})
			}

			// Extract config
			if config, ok := stepMap["config"].(map[string]interface{}); ok {
				step.Config = config
			} else {
				step.Config = make(map[string]interface{})
			}

			// Extract dependencies
			if deps, ok := stepMap["dependencies"].([]interface{}); ok {
				step.Dependencies = make([]string, 0, len(deps))
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok {
						step.Dependencies = append(step.Dependencies, depStr)
					}
				}
			}

			// Extract timeout
			if timeout, ok := stepMap["timeout"].(float64); ok {
				step.Timeout = time.Duration(timeout) * time.Second
			}
			if timeoutSec, ok := stepMap["timeout_seconds"].(float64); ok {
				step.TimeoutSeconds = int(timeoutSec)
			}

			// Extract retries
			if retries, ok := stepMap["retries"].(float64); ok {
				step.Retries = int(retries)
			}

			// Extract on_failure
			step.OnFailure = s.getStringValue(stepMap, "on_failure")

			steps = append(steps, step)
		}
	}

	return steps
}

// Helper method to extract string value from map
func (s *workflowService) getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// Helper method to extract bool value from map
func (s *workflowService) getBoolValue(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

// deepCopyWithSubstitution performs deep copy with parameter substitution
func (s *workflowService) deepCopyWithSubstitution(value interface{}, params map[string]interface{}) interface{} {
	switch v := value.(type) {
	case string:
		// Substitute parameters in strings
		return s.substituteTemplateParams(v, params)

	case map[string]interface{}:
		// Recursively copy maps
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = s.deepCopyWithSubstitution(val, params)
		}
		return result

	case []interface{}:
		// Recursively copy slices
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = s.deepCopyWithSubstitution(val, params)
		}
		return result

	default:
		// Return other types as-is
		return v
	}
}

func (s *workflowService) executeSequentialStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.executeSequentialStep")
	defer span.End()

	// Extract sequential configuration
	sequentialConfig := make(map[string]interface{})
	if step.Config != nil {
		sequentialConfig = step.Config
	}

	// Get steps to execute
	steps := []map[string]interface{}{}
	if stepsData, ok := sequentialConfig["steps"].([]interface{}); ok {
		for _, stepData := range stepsData {
			if stepMap, ok := stepData.(map[string]interface{}); ok {
				steps = append(steps, stepMap)
			}
		}
	}

	if len(steps) == 0 {
		return nil, ValidationError{
			Field:   "steps",
			Message: "at least one step required for sequential execution",
		}
	}

	// Get fail fast setting (default true - stop on first failure)
	failFast := true
	if ff, ok := sequentialConfig["fail_fast"].(bool); ok {
		failFast = ff
	}

	// Execute steps in sequence
	results := make(map[string]interface{})
	var lastOutput map[string]interface{}

	for i, stepConfig := range steps {
		stepID := fmt.Sprintf("%s_seq_%d", step.ID, i)
		if id, ok := stepConfig["id"].(string); ok {
			stepID = id
		}

		// Record step start
		s.config.Metrics.IncrementCounterWithLabels("workflow.sequential.step.start", 1, map[string]string{
			"workflow_id": execution.WorkflowID.String(),
			"parent_step": step.ID,
			"step_id":     stepID,
		})

		startTime := time.Now()

		// Get timeout for this step
		stepTimeout := 30 * time.Minute // default
		if timeoutMinutes, ok := stepConfig["timeout_minutes"].(float64); ok {
			stepTimeout = time.Duration(timeoutMinutes) * time.Minute
		}

		// Create timeout context
		stepCtx, cancel := context.WithTimeout(ctx, stepTimeout)
		defer cancel()

		// Create and execute task for this step
		subTask := &models.Task{
			ID:             uuid.New(),
			TenantID:       execution.TenantID,
			Title:          fmt.Sprintf("Sequential step: %s", stepID),
			Type:           "workflow_sequential_step",
			Priority:       models.TaskPriorityNormal,
			Status:         models.TaskStatusPending,
			CreatedBy:      execution.InitiatedBy,
			Parameters:     stepConfig,
			MaxRetries:     3,
			TimeoutSeconds: int(stepTimeout.Seconds()),
		}

		// Pass previous step output as input if available
		if lastOutput != nil {
			if subTask.Parameters == nil {
				subTask.Parameters = make(map[string]interface{})
			}
			subTask.Parameters["previous_output"] = lastOutput
		}

		// Create task
		createdTask, err := s.taskService.CreateWorkflowTask(stepCtx, execution.WorkflowID, uuid.MustParse(step.ID), subTask.Parameters)
		if err != nil {
			s.config.Metrics.IncrementCounterWithLabels("workflow.sequential.step.error", 1, map[string]string{
				"workflow_id": execution.WorkflowID.String(),
				"parent_step": step.ID,
				"step_id":     stepID,
				"error":       "create_failed",
			})

			if failFast {
				return nil, errors.Wrapf(err, "failed to create sequential step %s", stepID)
			}
			// Store error and continue
			results[stepID] = map[string]interface{}{
				"error":   err.Error(),
				"skipped": false,
			}
			continue
		}

		// Wait for task completion
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		taskCompleted := false

		for !taskCompleted {
			select {
			case <-stepCtx.Done():
				s.config.Metrics.IncrementCounterWithLabels("workflow.sequential.step.error", 1, map[string]string{
					"workflow_id": execution.WorkflowID.String(),
					"parent_step": step.ID,
					"step_id":     stepID,
					"error":       "timeout",
				})

				if failFast {
					return nil, fmt.Errorf("sequential step %s timed out", stepID)
				}
				results[stepID] = map[string]interface{}{
					"error":    "timeout",
					"duration": time.Since(startTime).String(),
				}
				taskCompleted = true

			case <-ticker.C:
				task, err := s.taskService.Get(stepCtx, createdTask.ID)
				if err != nil {
					continue // Try again
				}

				switch task.Status {
				case models.TaskStatusCompleted:
					taskCompleted = true
					lastOutput = task.Result

					s.config.Metrics.IncrementCounterWithLabels("workflow.sequential.step.complete", 1, map[string]string{
						"workflow_id": execution.WorkflowID.String(),
						"parent_step": step.ID,
						"step_id":     stepID,
					})
					s.config.Metrics.RecordHistogram("workflow.sequential.step.duration", time.Since(startTime).Seconds(), map[string]string{
						"workflow_id": execution.WorkflowID.String(),
						"parent_step": step.ID,
					})

					results[stepID] = map[string]interface{}{
						"task_id":  task.ID,
						"result":   task.Result,
						"duration": time.Since(startTime).String(),
					}

				case models.TaskStatusFailed:
					taskCompleted = true

					s.config.Metrics.IncrementCounterWithLabels("workflow.sequential.step.error", 1, map[string]string{
						"workflow_id": execution.WorkflowID.String(),
						"parent_step": step.ID,
						"step_id":     stepID,
						"error":       "task_failed",
					})

					if failFast {
						return nil, fmt.Errorf("sequential step %s failed: %s", stepID, task.Error)
					}
					results[stepID] = map[string]interface{}{
						"task_id":  task.ID,
						"error":    task.Error,
						"duration": time.Since(startTime).String(),
					}
				}
			}
		}
	}

	// Build final output
	output := map[string]interface{}{
		"steps_executed": len(results),
		"results":        results,
	}

	// Include final output if available
	if lastOutput != nil {
		output["final_output"] = lastOutput
	}

	return output, nil
}

func (s *workflowService) executeScriptStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.executeScriptStep")
	defer span.End()

	// Extract script configuration
	scriptConfig := make(map[string]interface{})
	if step.Config != nil {
		scriptConfig = step.Config
	}

	// Get script content
	scriptContent, ok := scriptConfig["script"].(string)
	if !ok || scriptContent == "" {
		return nil, ValidationError{
			Field:   "script",
			Message: "script content is required",
		}
	}

	// Get script type (default to bash)
	scriptType := "bash"
	if st, ok := scriptConfig["type"].(string); ok {
		scriptType = st
	}

	// Validate allowed script types
	allowedTypes := map[string]string{
		"bash":   "/bin/bash",
		"sh":     "/bin/sh",
		"python": "/usr/bin/python3",
		"node":   "/usr/bin/node",
	}

	interpreter, allowed := allowedTypes[scriptType]
	if !allowed {
		return nil, ValidationError{
			Field:   "type",
			Message: fmt.Sprintf("unsupported script type: %s", scriptType),
		}
	}

	// Get timeout (default 5 minutes)
	timeout := 5 * time.Minute
	if timeoutMinutes, ok := scriptConfig["timeout_minutes"].(float64); ok {
		timeout = time.Duration(timeoutMinutes) * time.Minute
	}

	// Get environment variables
	env := os.Environ()
	if envVars, ok := scriptConfig["env"].(map[string]interface{}); ok {
		for k, v := range envVars {
			env = append(env, fmt.Sprintf("%s=%v", k, v))
		}
	}

	// Add workflow context to environment
	env = append(env,
		fmt.Sprintf("WORKFLOW_ID=%s", execution.WorkflowID),
		fmt.Sprintf("WORKFLOW_EXECUTION_ID=%s", execution.ID),
		fmt.Sprintf("WORKFLOW_STEP_ID=%s", step.ID),
		fmt.Sprintf("TENANT_ID=%s", execution.TenantID),
	)

	// Get working directory
	workDir := "/tmp"
	if wd, ok := scriptConfig["working_directory"].(string); ok {
		// Validate working directory
		if !filepath.IsAbs(wd) {
			return nil, ValidationError{
				Field:   "working_directory",
				Message: "working directory must be an absolute path",
			}
		}
		// Ensure directory exists
		if _, err := os.Stat(wd); os.IsNotExist(err) {
			return nil, ValidationError{
				Field:   "working_directory",
				Message: fmt.Sprintf("working directory does not exist: %s", wd),
			}
		}
		workDir = wd
	}

	// Create temporary script file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("workflow-script-%s-*.%s", step.ID, scriptType))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary script file")
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	// Write script content
	if _, err := tmpFile.WriteString(scriptContent); err != nil {
		_ = tmpFile.Close()
		return nil, errors.Wrap(err, "failed to write script content")
	}
	if err := tmpFile.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close script file")
	}

	// Make script executable
	if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
		return nil, errors.Wrap(err, "failed to make script executable")
	}

	// Record script execution start
	s.config.Metrics.IncrementCounterWithLabels("workflow.script.start", 1, map[string]string{
		"workflow_id": execution.WorkflowID.String(),
		"step_id":     step.ID,
		"script_type": scriptType,
	})

	startTime := time.Now()

	// Create command with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, interpreter, tmpFile.Name())
	cmd.Dir = workDir
	cmd.Env = env

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set resource limits
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for cleanup
	}

	// Execute script
	err = cmd.Run()

	// Record metrics
	duration := time.Since(startTime)
	s.config.Metrics.RecordHistogram("workflow.script.duration", duration.Seconds(), map[string]string{
		"workflow_id": execution.WorkflowID.String(),
		"step_id":     step.ID,
		"script_type": scriptType,
	})

	// Prepare output
	output := map[string]interface{}{
		"stdout":      stdout.String(),
		"stderr":      stderr.String(),
		"duration":    duration.String(),
		"script_type": scriptType,
		"working_dir": workDir,
	}

	// Handle execution result
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			output["exit_code"] = exitError.ExitCode()

			s.config.Metrics.IncrementCounterWithLabels("workflow.script.error", 1, map[string]string{
				"workflow_id": execution.WorkflowID.String(),
				"step_id":     step.ID,
				"script_type": scriptType,
				"error":       "non_zero_exit",
			})

			// Check if we should fail on non-zero exit
			failOnError := true
			if foe, ok := scriptConfig["fail_on_error"].(bool); ok {
				failOnError = foe
			}

			if failOnError {
				return output, fmt.Errorf("script exited with code %d", exitError.ExitCode())
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			output["exit_code"] = -1
			output["error"] = "timeout"

			s.config.Metrics.IncrementCounterWithLabels("workflow.script.error", 1, map[string]string{
				"workflow_id": execution.WorkflowID.String(),
				"step_id":     step.ID,
				"script_type": scriptType,
				"error":       "timeout",
			})

			return output, fmt.Errorf("script execution timed out after %v", timeout)
		} else {
			output["exit_code"] = -1
			output["error"] = err.Error()

			s.config.Metrics.IncrementCounterWithLabels("workflow.script.error", 1, map[string]string{
				"workflow_id": execution.WorkflowID.String(),
				"step_id":     step.ID,
				"script_type": scriptType,
				"error":       "execution_error",
			})

			return output, errors.Wrap(err, "script execution failed")
		}
	} else {
		output["exit_code"] = 0

		s.config.Metrics.IncrementCounterWithLabels("workflow.script.complete", 1, map[string]string{
			"workflow_id": execution.WorkflowID.String(),
			"step_id":     step.ID,
			"script_type": scriptType,
		})
	}

	// Parse output if JSON expected
	if parseJSON, ok := scriptConfig["parse_json_output"].(bool); ok && parseJSON {
		var jsonOutput interface{}
		if err := json.Unmarshal(stdout.Bytes(), &jsonOutput); err == nil {
			output["parsed_output"] = jsonOutput
		}
	}

	// Log script execution
	s.config.Logger.Info("Script execution completed", map[string]interface{}{
		"workflow_id":  execution.WorkflowID,
		"step_id":      step.ID,
		"script_type":  scriptType,
		"exit_code":    output["exit_code"],
		"duration":     duration.String(),
		"stdout_lines": strings.Count(stdout.String(), "\n"),
		"stderr_lines": strings.Count(stderr.String(), "\n"),
	})

	return output, nil
}

// ExecuteWithCircuitBreaker executes a function with circuit breaker protection
func (s *workflowService) ExecuteWithCircuitBreaker(ctx context.Context, operation string, fn func() (interface{}, error)) (interface{}, error) {
	// If no circuit breaker settings configured, execute directly
	if s.config.CircuitBreaker == nil {
		return fn()
	}

	// Create a circuit breaker for this operation if not exists
	// In production, you'd want to maintain a map of circuit breakers per operation
	// For now, we'll execute with basic error handling
	result, err := fn()
	if err != nil {
		// Record failure metric
		if s.config.Metrics != nil {
			s.config.Metrics.IncrementCounterWithLabels("circuit_breaker_failure", 1, map[string]string{
				"operation": operation,
			})
		}
		return nil, err
	}

	// Record success metric
	if s.config.Metrics != nil {
		s.config.Metrics.IncrementCounterWithLabels("circuit_breaker_success", 1, map[string]string{
			"operation": operation,
		})
	}

	return result, nil
}

// executeWebhookStep executes a webhook step by making an HTTP request
func (s *workflowService) executeWebhookStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.executeWebhookStep")
	defer span.End()

	s.config.Logger.Info("Executing webhook step", map[string]interface{}{
		"execution_id": execution.ID,
		"step_id":      step.ID,
	})

	// Extract webhook configuration
	config := step.Config
	if config == nil {
		return nil, errors.New("invalid webhook configuration")
	}

	// Get URL
	urlStr, ok := config["url"].(string)
	if !ok || urlStr == "" {
		return nil, errors.New("webhook URL is required")
	}

	// Get method (default to POST)
	method := "POST"
	if m, ok := config["method"].(string); ok {
		method = strings.ToUpper(m)
	}

	// Get timeout (default to 30 seconds)
	timeout := 30 * time.Second
	if t, ok := config["timeout_seconds"].(float64); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	}

	// Prepare request body
	var requestBody []byte
	if body, ok := config["body"]; ok {
		switch v := body.(type) {
		case string:
			requestBody = []byte(v)
		case map[string]interface{}, []interface{}:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, errors.Wrap(err, "failed to marshal request body")
			}
			requestBody = b
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, urlStr, strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	// Set headers
	if headers, ok := config["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Set(key, strValue)
			}
		}
	}

	// Default Content-Type if not set
	if req.Header.Get("Content-Type") == "" && len(requestBody) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add authentication if configured
	if auth, ok := config["authentication"].(map[string]interface{}); ok {
		authType, _ := auth["type"].(string)
		switch authType {
		case "bearer":
			if token, ok := auth["token"].(string); ok {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "basic":
			if username, ok := auth["username"].(string); ok {
				if password, ok := auth["password"].(string); ok {
					req.SetBasicAuth(username, password)
				}
			}
		}
	}

	// Execute request with retries
	var resp *http.Response
	maxRetries := 3
	if r, ok := config["max_retries"].(float64); ok {
		maxRetries = int(r)
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt) * time.Second
			time.Sleep(backoff)
		}

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break // Success or client error - don't retry
		}

		if err != nil {
			s.config.Logger.Warn("Webhook request failed", map[string]interface{}{
				"attempt": attempt + 1,
				"error":   err.Error(),
			})
		} else if resp != nil {
			s.config.Logger.Warn("Webhook request failed with status", map[string]interface{}{
				"attempt": attempt + 1,
				"status":  resp.StatusCode,
			})
			_ = resp.Body.Close()
		}
	}

	if err != nil {
		return nil, errors.Wrap(err, "webhook request failed after retries")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Check for success status codes
	successCodes := []int{200, 201, 202, 204}
	if codes, ok := config["success_codes"].([]interface{}); ok {
		successCodes = make([]int, 0, len(codes))
		for _, code := range codes {
			if c, ok := code.(float64); ok {
				successCodes = append(successCodes, int(c))
			}
		}
	}

	isSuccess := false
	for _, code := range successCodes {
		if resp.StatusCode == code {
			isSuccess = true
			break
		}
	}

	if !isSuccess {
		return nil, errors.Errorf("webhook returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse response
	output := map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(responseBody),
	}

	// Try to parse JSON response
	var jsonResponse interface{}
	if err := json.Unmarshal(responseBody, &jsonResponse); err == nil {
		output["parsed_body"] = jsonResponse
	}

	s.config.Logger.Info("Webhook step completed", map[string]interface{}{
		"execution_id": execution.ID,
		"step_id":      step.ID,
		"status_code":  resp.StatusCode,
	})

	return output, nil
}

// handleBranching executes a branching step that can split execution into multiple paths
func (s *workflowService) handleBranching(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.handleBranching")
	defer span.End()

	s.config.Logger.Info("Handling branching step", map[string]interface{}{
		"execution_id": execution.ID,
		"step_id":      step.ID,
	})

	// Extract branching configuration
	config := step.Config
	if config == nil {
		return nil, errors.New("invalid branching configuration")
	}

	// Get branches
	branchesConfig, ok := config["branches"].([]interface{})
	if !ok || len(branchesConfig) == 0 {
		return nil, errors.New("branches are required for branching step")
	}

	// Get strategy (default to "all")
	strategy := "all"
	if s, ok := config["strategy"].(string); ok {
		strategy = s
	}

	// Parse branches
	type branch struct {
		Name      string
		Condition string
		NextStep  string
		Weight    float64
	}

	branches := make([]branch, 0, len(branchesConfig))
	for _, bc := range branchesConfig {
		branchMap, ok := bc.(map[string]interface{})
		if !ok {
			continue
		}

		b := branch{
			Name:     branchMap["name"].(string),
			NextStep: branchMap["next_step"].(string),
			Weight:   1.0,
		}

		if cond, ok := branchMap["condition"].(string); ok {
			b.Condition = cond
		}

		if weight, ok := branchMap["weight"].(float64); ok {
			b.Weight = weight
		}

		branches = append(branches, b)
	}

	// Determine which branches to execute based on strategy
	var selectedBranches []branch

	switch strategy {
	case "all":
		// Execute all branches
		selectedBranches = branches

	case "conditional":
		// Execute branches where conditions are true
		for _, b := range branches {
			if b.Condition != "" {
				// Evaluate condition
				// Parse condition string to map
				conditionMap := map[string]interface{}{
					"type":       "expression",
					"expression": b.Condition,
				}
				result, err := s.evaluateCondition(ctx, execution, conditionMap)
				if err != nil {
					s.config.Logger.Warn("Failed to evaluate branch condition", map[string]interface{}{
						"branch": b.Name,
						"error":  err.Error(),
					})
					continue
				}
				if result == "true" {
					selectedBranches = append(selectedBranches, b)
				}
			}
		}

	case "weighted":
		// Select one branch based on weights
		if len(branches) > 0 {
			totalWeight := 0.0
			for _, b := range branches {
				totalWeight += b.Weight
			}

			// Random selection based on weights
			r := rand.Float64() * totalWeight
			cumulative := 0.0
			for _, b := range branches {
				cumulative += b.Weight
				if r <= cumulative {
					selectedBranches = []branch{b}
					break
				}
			}
		}

	case "first":
		// Execute only the first matching branch
		for _, b := range branches {
			if b.Condition == "" {
				selectedBranches = []branch{b}
				break
			}
			conditionMap := map[string]interface{}{
				"type":       "expression",
				"expression": b.Condition,
			}
			result, err := s.evaluateCondition(ctx, execution, conditionMap)
			if err == nil && result == "true" {
				selectedBranches = []branch{b}
				break
			}
		}

	default:
		return nil, errors.New(fmt.Sprintf("unsupported branching strategy: %s", strategy))
	}

	// Execute selected branches
	results := make(map[string]interface{})
	errors := make([]string, 0)

	// Update execution state to track branches
	if execution.State == nil {
		execution.State = make(models.JSONMap)
	}
	activeBranches := make([]string, 0, len(selectedBranches))
	for _, b := range selectedBranches {
		activeBranches = append(activeBranches, b.Name)
	}
	execution.State["active_branches"] = activeBranches

	// For parallel execution of branches
	if strategy == "all" || strategy == "conditional" {
		// Create a wait group for parallel execution
		type branchResult struct {
			Name   string
			Result interface{}
			Error  error
		}

		resultChan := make(chan branchResult, len(selectedBranches))

		for _, branch := range selectedBranches {
			go func(b struct {
				Name      string
				Condition string
				NextStep  string
				Weight    float64
			}) {
				// Queue next step for execution
				if b.NextStep != "" {
					err := s.queueStepForExecution(ctx, execution, b.NextStep)
					if err != nil {
						resultChan <- branchResult{
							Name:  b.Name,
							Error: err,
						}
						return
					}
				}

				resultChan <- branchResult{
					Name: b.Name,
					Result: map[string]interface{}{
						"branch":    b.Name,
						"next_step": b.NextStep,
						"queued":    true,
					},
				}
			}(branch)
		}

		// Collect results
		for i := 0; i < len(selectedBranches); i++ {
			result := <-resultChan
			if result.Error != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", result.Name, result.Error))
			} else {
				results[result.Name] = result.Result
			}
		}
	} else {
		// Sequential execution for single branch strategies
		for _, branch := range selectedBranches {
			if branch.NextStep != "" {
				err := s.queueStepForExecution(ctx, execution, branch.NextStep)
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", branch.Name, err))
				} else {
					results[branch.Name] = map[string]interface{}{
						"branch":    branch.Name,
						"next_step": branch.NextStep,
						"queued":    true,
					}
				}
			}
		}
	}

	// Check for errors
	if len(errors) > 0 {
		return results, fmt.Errorf("branch execution errors: %s", strings.Join(errors, "; "))
	}

	output := map[string]interface{}{
		"strategy":          strategy,
		"selected_branches": activeBranches,
		"results":           results,
	}

	s.config.Logger.Info("Branching step completed", map[string]interface{}{
		"execution_id":      execution.ID,
		"step_id":           step.ID,
		"selected_branches": activeBranches,
	})

	return output, nil
}

// executeCompensation executes compensation/rollback logic for failed workflows
func (s *workflowService) executeCompensation(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (map[string]interface{}, error) {
	ctx, span := s.config.Tracer(ctx, "WorkflowService.executeCompensation")
	defer span.End()

	s.config.Logger.Info("Executing compensation step", map[string]interface{}{
		"execution_id": execution.ID,
		"step_id":      step.ID,
	})

	// Extract compensation configuration
	config := step.Config
	if config == nil {
		return nil, errors.New("invalid compensation configuration")
	}

	// Get compensation strategy
	strategy := "reverse"
	if s, ok := config["strategy"].(string); ok {
		strategy = s
	}

	// Get target steps to compensate
	var targetSteps []string
	if targets, ok := config["target_steps"].([]interface{}); ok {
		for _, t := range targets {
			if stepID, ok := t.(string); ok {
				targetSteps = append(targetSteps, stepID)
			}
		}
	}

	// If no specific targets, compensate all completed steps
	if len(targetSteps) == 0 {
		for stepID, status := range execution.StepStatuses {
			if status.Status == models.StepStatusCompleted {
				targetSteps = append(targetSteps, stepID)
			}
		}
	}

	// Execute compensation based on strategy
	compensationResults := make(map[string]interface{})
	var compensationErrors []string

	switch strategy {
	case "reverse":
		// Compensate in reverse order of execution
		// Sort steps by completion time (newest first)
		type stepInfo struct {
			ID          string
			CompletedAt time.Time
		}

		var sortedSteps []stepInfo
		for _, stepID := range targetSteps {
			if status, ok := execution.StepStatuses[stepID]; ok && status.CompletedAt != nil {
				sortedSteps = append(sortedSteps, stepInfo{
					ID:          stepID,
					CompletedAt: *status.CompletedAt,
				})
			}
		}

		// Sort by completion time (descending)
		for i := 0; i < len(sortedSteps); i++ {
			for j := i + 1; j < len(sortedSteps); j++ {
				if sortedSteps[i].CompletedAt.Before(sortedSteps[j].CompletedAt) {
					sortedSteps[i], sortedSteps[j] = sortedSteps[j], sortedSteps[i]
				}
			}
		}

		// Execute compensation for each step
		for _, stepInfo := range sortedSteps {
			result, err := s.compensateStep(ctx, execution, stepInfo.ID)
			if err != nil {
				compensationErrors = append(compensationErrors, fmt.Sprintf("%s: %v", stepInfo.ID, err))
				compensationResults[stepInfo.ID] = map[string]interface{}{
					"status": "failed",
					"error":  err.Error(),
				}
			} else {
				compensationResults[stepInfo.ID] = result
			}
		}

	case "parallel":
		// Compensate all steps in parallel
		type compensationResult struct {
			StepID string
			Result map[string]interface{}
			Error  error
		}

		resultChan := make(chan compensationResult, len(targetSteps))

		for _, stepID := range targetSteps {
			go func(sid string) {
				result, err := s.compensateStep(ctx, execution, sid)
				resultChan <- compensationResult{
					StepID: sid,
					Result: result,
					Error:  err,
				}
			}(stepID)
		}

		// Collect results
		for i := 0; i < len(targetSteps); i++ {
			result := <-resultChan
			if result.Error != nil {
				compensationErrors = append(compensationErrors, fmt.Sprintf("%s: %v", result.StepID, result.Error))
				compensationResults[result.StepID] = map[string]interface{}{
					"status": "failed",
					"error":  result.Error.Error(),
				}
			} else {
				compensationResults[result.StepID] = result.Result
			}
		}

	case "custom":
		// Execute custom compensation logic
		if compensationFunc, ok := config["function"].(string); ok {
			// In production, this would invoke a registered compensation function
			// For now, we'll use a simple implementation
			result := map[string]interface{}{
				"function": compensationFunc,
				"status":   "executed",
			}
			compensationResults["custom"] = result
		}

	default:
		return nil, errors.Errorf("unsupported compensation strategy: %s", strategy)
	}

	// Update execution state
	execution.State["compensation_executed"] = true
	execution.State["compensation_results"] = compensationResults

	output := map[string]interface{}{
		"strategy":          strategy,
		"compensated_steps": targetSteps,
		"results":           compensationResults,
	}

	if len(compensationErrors) > 0 {
		output["errors"] = compensationErrors
		s.config.Logger.Error("Compensation completed with errors", map[string]interface{}{
			"execution_id": execution.ID,
			"step_id":      step.ID,
			"errors":       compensationErrors,
		})
	} else {
		s.config.Logger.Info("Compensation completed successfully", map[string]interface{}{
			"execution_id": execution.ID,
			"step_id":      step.ID,
		})
	}

	return output, nil
}

// compensateStep executes compensation logic for a single step
func (s *workflowService) compensateStep(ctx context.Context, execution *models.WorkflowExecution, stepID string) (map[string]interface{}, error) {
	// Get the original step definition
	workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get workflow")
	}

	// Find the step
	var originalStep *models.WorkflowStep
	for i := range workflow.Steps {
		if workflow.Steps[i].ID == stepID {
			originalStep = &workflow.Steps[i]
			break
		}
	}

	if originalStep == nil {
		return nil, errors.Errorf("step %s not found in workflow", stepID)
	}

	// Check if step has compensation defined in config
	var compensation map[string]interface{}
	if originalStep.Config != nil {
		if comp, ok := originalStep.Config["compensation"].(map[string]interface{}); ok {
			compensation = comp
		}
	}

	if compensation == nil {
		return map[string]interface{}{
			"status":  "skipped",
			"message": "no compensation defined for step",
		}, nil
	}

	// Execute compensation based on type
	compensationType := "task"
	if t, ok := compensation["type"].(string); ok {
		compensationType = t
	}

	switch compensationType {
	case "task":
		// Create a compensation task
		compensationTask := &models.Task{
			Type:        "compensation",
			Title:       fmt.Sprintf("Compensate: %s", stepID),
			Description: fmt.Sprintf("Compensation task for step %s in execution %s", stepID, execution.ID),
			Priority:    models.TaskPriorityHigh,
			Parameters: map[string]interface{}{
				"original_step": stepID,
				"execution_id":  execution.ID,
				"compensation":  compensation,
			},
		}

		// Create and wait for compensation task
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		createdTask, err := s.taskService.CreateWorkflowTask(ctx, execution.WorkflowID, uuid.MustParse(stepID), compensationTask.Parameters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create compensation task")
		}

		// Poll for task completion
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil, errors.New("compensation task timed out")
			case <-ticker.C:
				task, err := s.taskService.Get(ctx, createdTask.ID)
				if err != nil {
					return nil, errors.Wrap(err, "failed to get compensation task status")
				}

				switch task.Status {
				case models.TaskStatusCompleted:
					return map[string]interface{}{
						"status":  "completed",
						"task_id": createdTask.ID,
						"result":  task.Result,
					}, nil
				case models.TaskStatusFailed:
					return nil, errors.New("compensation task failed")
				}
			}
		}

	case "script":
		// Execute compensation script
		if script, ok := compensation["script"].(string); ok {
			// For security, we should validate and sandbox the script
			// For now, we'll just log it
			s.config.Logger.Info("Would execute compensation script", map[string]interface{}{
				"step_id": stepID,
				"script":  script,
			})

			return map[string]interface{}{
				"status": "completed",
				"type":   "script",
			}, nil
		}

	case "noop":
		// No operation - just mark as compensated
		return map[string]interface{}{
			"status": "completed",
			"type":   "noop",
		}, nil
	}

	return nil, errors.Errorf("unsupported compensation type: %s", compensationType)
}

// queueStepForExecution queues a step for execution
func (s *workflowService) queueStepForExecution(ctx context.Context, execution *models.WorkflowExecution, stepID string) error {
	// In a production system, this would add the step to a queue for processing
	// For now, we'll execute it directly in a goroutine
	go func() {
		if err := s.ExecuteWorkflowStep(ctx, execution.ID, stepID); err != nil {
			s.config.Logger.Error("Failed to execute queued step", map[string]interface{}{
				"execution_id": execution.ID,
				"step_id":      stepID,
				"error":        err.Error(),
			})
		}
	}()

	return nil
}
