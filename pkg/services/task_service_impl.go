package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
)

// AssignmentStrategyLeastLoad represents the least load assignment strategy
var AssignmentStrategyLeastLoad = &LeastLoadedStrategy{}

type taskService struct {
	BaseService

	// Dependencies
	repo            interfaces.TaskRepository
	agentService    AgentService
	notifier        NotificationService
	workflowService WorkflowService

	// Configuration
	assignmentEngine *AssignmentEngine
	aggregator       *ResultAggregator

	// Caching
	taskCache  cache.Cache
	statsCache cache.Cache

	// Background workers
	progressTracker *ProgressTracker
	taskRebalancer  *TaskRebalancer
}

// NewTaskService creates a production-ready task service
func NewTaskService(
	config ServiceConfig,
	repo interfaces.TaskRepository,
	agentService AgentService,
	notifier NotificationService,
) TaskService {
	s := &taskService{
		BaseService:      NewBaseService(config),
		repo:             repo,
		agentService:     agentService,
		notifier:         notifier,
		assignmentEngine: NewAssignmentEngine(config.RuleEngine, agentService, config.Logger, config.Metrics),
		aggregator:       NewResultAggregator(),
		taskCache:        cache.NewMemoryCache(10000, 5*time.Minute),
		statsCache:       cache.NewMemoryCache(1000, 1*time.Minute),
	}

	// Start background workers
	s.progressTracker = NewProgressTracker(s)
	s.taskRebalancer = NewTaskRebalancer(s, config.RuleEngine)

	go s.progressTracker.Start()
	go s.taskRebalancer.Start()

	return s
}

// isValidStatusTransition checks if a status transition is valid
func (s *taskService) isValidStatusTransition(from, to models.TaskStatus) bool {
	validTransitions := map[models.TaskStatus][]models.TaskStatus{
		models.TaskStatusPending:    {models.TaskStatusAssigned, models.TaskStatusCancelled},
		models.TaskStatusAssigned:   {models.TaskStatusAccepted, models.TaskStatusRejected, models.TaskStatusCancelled},
		models.TaskStatusAccepted:   {models.TaskStatusInProgress, models.TaskStatusCancelled},
		models.TaskStatusRejected:   {models.TaskStatusPending, models.TaskStatusCancelled},
		models.TaskStatusInProgress: {models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusCancelled},
		models.TaskStatusCompleted:  {},                         // Terminal state
		models.TaskStatusFailed:     {models.TaskStatusPending}, // Can retry
		models.TaskStatusCancelled:  {},                         // Terminal state
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, validTo := range allowed {
		if validTo == to {
			return true
		}
	}
	return false
}

// Create creates a task with full validation and idempotency
func (s *taskService) Create(ctx context.Context, task *models.Task, idempotencyKey string) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.Create")
	defer span.End()

	// Check rate limit
	if err := s.CheckRateLimit(ctx, "task:create"); err != nil {
		return err
	}

	// Check quota
	if err := s.CheckQuota(ctx, "tasks", 1); err != nil {
		return err
	}

	// Idempotency check
	if idempotencyKey != "" {
		existingID, err := s.checkIdempotency(ctx, idempotencyKey)
		if err == nil && existingID != uuid.Nil {
			task.ID = existingID
			return nil
		}
	}

	// Sanitize input
	if err := s.sanitizeTask(task); err != nil {
		return errors.Wrap(err, "input sanitization failed")
	}

	// Validate with business rules
	if err := s.validateTask(ctx, task); err != nil {
		return errors.Wrap(err, "task validation failed")
	}

	// Check authorization
	if err := s.authorizeTaskCreation(ctx, task); err != nil {
		return errors.Wrap(err, "authorization failed")
	}

	// Execute in transaction
	err := s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Set metadata
		task.ID = uuid.New()
		task.TenantID = auth.GetTenantID(ctx)
		task.CreatedBy = auth.GetAgentID(ctx)
		task.Status = models.TaskStatusPending
		task.Version = 1

		// Set defaults from policy
		if err := s.applyTaskDefaults(ctx, task); err != nil {
			return err
		}

		// Auto-assign if enabled
		if task.AssignedTo == nil && s.shouldAutoAssign(ctx, task) {
			agent, err := s.assignmentEngine.FindBestAgent(ctx, task)
			if err == nil && agent != nil {
				task.AssignedTo = &agent.ID
				task.Status = models.TaskStatusAssigned
				task.AssignedAt = timePtr(time.Now())
			}
		}

		// Create task
		if err := s.repo.Create(ctx, task); err != nil {
			return errors.Wrap(err, "failed to create task")
		}

		// Store idempotency key
		if idempotencyKey != "" {
			if err := s.storeIdempotencyKey(ctx, idempotencyKey, task.ID); err != nil {
				return err
			}
		}

		// Publish event
		if err := s.PublishEvent(ctx, "TaskCreated", task, task); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Post-creation actions (outside transaction)
	s.executePostCreationActions(ctx, task)

	return nil
}

// CreateDistributedTask creates a task with subtasks using saga pattern
func (s *taskService) CreateDistributedTask(ctx context.Context, dt *models.DistributedTask) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.CreateDistributedTask")
	defer span.End()

	// Validate distributed task
	if err := s.validateDistributedTask(ctx, dt); err != nil {
		return err
	}

	// Start saga
	saga := NewTaskCreationSaga(s, dt)
	compensator := NewCompensator(s.config.Logger)

	// Execute saga steps
	err := s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Step 1: Create main task
		mainTask, err := saga.CreateMainTask(ctx, tx)
		if err != nil {
			return err
		}
		compensator.AddCompensation(func() error {
			return saga.DeleteMainTask(ctx, mainTask.ID)
		})

		// Step 2: Validate agent availability
		agentMap, err := saga.ValidateAgents(ctx, dt.Subtasks)
		if err != nil {
			return err
		}

		// Step 3: Create subtasks in parallel
		g, gctx := errgroup.WithContext(ctx)
		subtaskIDs := make([]uuid.UUID, len(dt.Subtasks))
		var mu sync.Mutex

		for i, subtaskDef := range dt.Subtasks {
			i := i
			subtaskDef := subtaskDef

			g.Go(func() error {
				subtask, err := saga.CreateSubtask(gctx, tx, mainTask, subtaskDef, agentMap[subtaskDef.AgentID])
				if err != nil {
					return err
				}

				mu.Lock()
				subtaskIDs[i] = subtask.ID
				mu.Unlock()

				compensator.AddCompensation(func() error {
					return saga.DeleteSubtask(ctx, subtask.ID)
				})

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		// Step 4: Update main task with subtask references
		mainTask.Parameters["subtask_ids"] = subtaskIDs
		if err := s.repo.Update(ctx, mainTask); err != nil {
			return err
		}

		// Step 5: Publish events
		if err := saga.PublishEvents(ctx); err != nil {
			return err
		}

		dt.ID = mainTask.ID
		dt.SubtaskIDs = subtaskIDs

		return nil
	})

	if err != nil {
		// Run compensations
		if compErr := compensator.Compensate(ctx); compErr != nil {
			s.config.Logger.Error("Saga compensation failed", map[string]interface{}{
				"error":          compErr.Error(),
				"original_error": err.Error(),
			})
		}
		return err
	}

	// Notify agents
	s.notifyDistributedTaskCreated(ctx, dt)

	return nil
}

// DelegateTask handles task delegation with policy enforcement
func (s *taskService) DelegateTask(ctx context.Context, delegation *models.TaskDelegation) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.DelegateTask")
	defer span.End()

	// Rate limiting
	if err := s.CheckRateLimit(ctx, "task:delegate"); err != nil {
		return err
	}

	// Get task with lock
	task, err := s.repo.GetForUpdate(ctx, delegation.TaskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Validate delegation
	if err := s.validateDelegation(ctx, task, delegation); err != nil {
		return err
	}

	// Check delegation policy
	if s.config.RuleEngine != nil {
		decision, err := s.config.RuleEngine.Evaluate(ctx, "task.delegation", map[string]interface{}{
			"task":       task,
			"delegation": delegation,
			"from_agent": delegation.FromAgentID,
			"to_agent":   delegation.ToAgentID,
		})

		if err != nil {
			return errors.Wrap(err, "policy evaluation failed")
		}

		if !decision.Allowed {
			return DelegationError{Reason: decision.Reason}
		}
	}

	// Apply delegation
	return s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Create delegation record
		delegation.ID = uuid.New()
		delegation.DelegatedAt = time.Now()

		if err := s.repo.CreateDelegation(ctx, delegation); err != nil {
			return errors.Wrap(err, "failed to create delegation")
		}

		// Update task
		previousAssignee := task.AssignedTo
		task.AssignedTo = &delegation.ToAgentID
		task.Status = models.TaskStatusAssigned
		task.AssignedAt = timePtr(time.Now())
		task.Version++

		if err := s.repo.UpdateWithVersion(ctx, task, task.Version-1); err != nil {
			if errors.Is(err, interfaces.ErrOptimisticLock) {
				return ErrConcurrentModification
			}
			return errors.Wrap(err, "failed to update task")
		}

		// Publish event
		prevAgent := ""
		if previousAssignee != nil {
			prevAgent = *previousAssignee
		}
		event := &models.TaskDelegatedEvent{
			Task:          task,
			Delegation:    delegation,
			PreviousAgent: prevAgent,
		}

		if err := s.PublishEvent(ctx, "TaskDelegated", task, event); err != nil {
			return err
		}

		return nil
	})
}

// Get retrieves a task by ID
func (s *taskService) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.Get")
	defer span.End()

	// Check cache first
	cacheKey := fmt.Sprintf("task:%s", id)
	var cached models.Task
	if err := s.taskCache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	// Get from repository
	task, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the result
	_ = s.taskCache.Set(ctx, cacheKey, task, 5*time.Minute) // Best effort caching

	return task, nil
}

// Helper methods

func (s *taskService) sanitizeTask(task *models.Task) error {
	if s.config.Sanitizer == nil {
		return nil
	}

	// Sanitize string fields
	task.Title = s.config.Sanitizer.SanitizeString(task.Title)
	task.Description = s.config.Sanitizer.SanitizeString(task.Description)

	// Sanitize parameters
	if task.Parameters != nil {
		sanitized, err := s.config.Sanitizer.SanitizeJSON(task.Parameters)
		if err != nil {
			return err
		}
		if params, ok := sanitized.(models.JSONMap); ok {
			task.Parameters = params
		}
	}

	return nil
}

func (s *taskService) validateTask(ctx context.Context, task *models.Task) error {
	// Basic validation
	if task.Type == "" {
		return ValidationError{Field: "type", Message: "required"}
	}

	if task.Title == "" {
		return ValidationError{Field: "title", Message: "required"}
	}

	if len(task.Title) > 500 {
		return ValidationError{Field: "title", Message: "exceeds maximum length"}
	}

	// Business rule validation is handled by the policy manager if configured

	return nil
}

func (s *taskService) authorizeTaskCreation(ctx context.Context, task *models.Task) error {
	if s.config.Authorizer == nil {
		return nil
	}

	decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
		Resource: "task",
		Action:   "create",
		Conditions: map[string]interface{}{
			"type":     task.Type,
			"priority": task.Priority,
		},
	})

	if !decision.Allowed {
		return UnauthorizedError{
			Action: "create task",
			Reason: decision.Reason,
		}
	}

	return nil
}

func (s *taskService) applyTaskDefaults(ctx context.Context, task *models.Task) error {
	if s.config.PolicyManager == nil {
		return nil
	}

	defaults, err := s.config.PolicyManager.GetDefaults(ctx, "task", task.Type)
	if err != nil {
		return err
	}

	if task.Priority == "" {
		task.Priority = models.TaskPriority(defaults.GetString("priority", string(models.TaskPriorityNormal)))
	}

	if task.MaxRetries == 0 {
		task.MaxRetries = defaults.GetInt("max_retries", 3)
	}

	if task.TimeoutSeconds == 0 {
		task.TimeoutSeconds = defaults.GetInt("timeout_seconds", 3600)
	}

	// Apply default parameters
	if defaultParams := defaults.GetMap("parameters"); defaultParams != nil {
		if task.Parameters == nil {
			task.Parameters = make(models.JSONMap)
		}

		for k, v := range defaultParams {
			if _, exists := task.Parameters[k]; !exists {
				task.Parameters[k] = v
			}
		}
	}

	return nil
}

func (s *taskService) executePostCreationActions(ctx context.Context, task *models.Task) {
	// Notify assigned agent
	if task.AssignedTo != nil && *task.AssignedTo != "" {
		if err := s.notifier.NotifyTaskAssigned(ctx, *task.AssignedTo, task); err != nil {
			s.config.Logger.Error("Failed to notify agent", map[string]interface{}{
				"task_id":  task.ID,
				"agent_id": *task.AssignedTo,
				"error":    err.Error(),
			})
		}
	}

	// Update caches
	if task.AssignedTo != nil {
		_ = s.taskCache.Delete(ctx, fmt.Sprintf("agent:%s:tasks", *task.AssignedTo))
	}
	_ = s.statsCache.Delete(ctx, fmt.Sprintf("tenant:%s:stats", task.TenantID))

	// Schedule monitoring
	s.progressTracker.Track(task.ID)

	// Metrics
	if s.config.Metrics != nil {
		s.config.Metrics.IncrementCounter("task.created", 1.0)
	}
}

// CreateBatch creates multiple tasks in a single operation
func (s *taskService) CreateBatch(ctx context.Context, tasks []*models.Task) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.CreateBatch")
	defer span.End()

	// Validate input
	if len(tasks) == 0 {
		return errors.New("no tasks provided")
	}

	// Check authorization for batch creation
	if s.config.Authorizer != nil {
		for range tasks {
			if !s.config.Authorizer.CheckPermission(ctx, "task", "create") {
				return errors.New("unauthorized to create tasks")
			}
		}
	}

	// Validate each task
	for i, task := range tasks {
		if err := s.validateTask(ctx, task); err != nil {
			return errors.Wrapf(err, "invalid task at index %d", i)
		}
		// Set defaults
		if task.ID == uuid.Nil {
			task.ID = uuid.New()
		}
		if task.Status == "" {
			task.Status = models.TaskStatusPending
		}
		task.CreatedAt = time.Now()
		task.UpdatedAt = time.Now()
	}

	// Create in repository
	if err := s.repo.CreateBatch(ctx, tasks); err != nil {
		s.config.Logger.Error("Failed to create batch tasks", map[string]interface{}{
			"error": err.Error(),
			"count": len(tasks),
		})
		return errors.Wrap(err, "failed to create batch tasks")
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.batch_created", float64(len(tasks)))
	s.config.Logger.Info("Batch tasks created", map[string]interface{}{
		"count": len(tasks),
	})

	return nil
}

func (s *taskService) GetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.GetBatch")
	defer span.End()

	// Validate input
	if len(ids) == 0 {
		return []*models.Task{}, nil
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "read") {
		return nil, errors.New("unauthorized to read tasks")
	}

	// Try to get from cache first
	tasks := make([]*models.Task, 0, len(ids))
	uncachedIDs := make([]uuid.UUID, 0)

	for _, id := range ids {
		cacheKey := fmt.Sprintf("task:%s", id.String())
		var task models.Task
		if err := s.taskCache.Get(ctx, cacheKey, &task); err == nil {
			// Create a copy to avoid G601 implicit memory aliasing
			taskCopy := task
			tasks = append(tasks, &taskCopy)
		} else {
			uncachedIDs = append(uncachedIDs, id)
		}
	}

	// Get uncached tasks from repository
	if len(uncachedIDs) > 0 {
		repTasks, err := s.repo.GetBatch(ctx, uncachedIDs)
		if err != nil {
			s.config.Logger.Error("Failed to get batch tasks", map[string]interface{}{
				"error": err.Error(),
				"count": len(uncachedIDs),
			})
			return nil, errors.Wrap(err, "failed to get batch tasks")
		}

		// Cache the retrieved tasks
		for _, task := range repTasks {
			cacheKey := fmt.Sprintf("task:%s", task.ID.String())
			_ = s.taskCache.Set(ctx, cacheKey, task, 5*time.Minute)
			tasks = append(tasks, task)
		}
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.batch_retrieved", float64(len(tasks)))

	return tasks, nil
}

func (s *taskService) Update(ctx context.Context, task *models.Task) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.Update")
	defer span.End()

	// Validate input
	if task == nil {
		return errors.New("task is required")
	}
	if task.ID == uuid.Nil {
		return errors.New("task ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "update") {
		return errors.New("unauthorized to update task")
	}

	// Get existing task to validate state transition
	existing, err := s.repo.Get(ctx, task.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get existing task")
	}

	// Validate status transition if status changed
	if existing.Status != task.Status {
		if !s.isValidStatusTransition(existing.Status, task.Status) {
			return errors.Errorf("invalid status transition from %s to %s", existing.Status, task.Status)
		}
	}

	// Update timestamp
	task.UpdatedAt = time.Now()

	// Update in repository
	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to update task", map[string]interface{}{
			"error":   err.Error(),
			"task_id": task.ID.String(),
		})
		return errors.Wrap(err, "failed to update task")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", task.ID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Record metrics
	s.config.Metrics.IncrementCounter("task.updated", 1.0)

	return nil
}

func (s *taskService) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.Delete")
	defer span.End()

	// Validate input
	if id == uuid.Nil {
		return errors.New("task ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "delete") {
		return errors.New("unauthorized to delete task")
	}

	// Get task to check if it can be deleted
	task, err := s.repo.Get(ctx, id)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Don't allow deletion of in-progress tasks
	if task.Status == models.TaskStatusInProgress {
		return errors.New("cannot delete in-progress task")
	}

	// Delete from repository
	if err := s.repo.Delete(ctx, id); err != nil {
		s.config.Logger.Error("Failed to delete task", map[string]interface{}{
			"error":   err.Error(),
			"task_id": id.String(),
		})
		return errors.Wrap(err, "failed to delete task")
	}

	// Remove from cache
	cacheKey := fmt.Sprintf("task:%s", id.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Record metrics
	s.config.Metrics.IncrementCounter("task.deleted", 1.0)

	return nil
}

func (s *taskService) AssignTask(ctx context.Context, taskID uuid.UUID, agentID string) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.AssignTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if agentID == "" {
		return errors.New("agent ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "assign") {
		return errors.New("unauthorized to assign task")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Check if task can be assigned
	if task.Status != models.TaskStatusPending && task.Status != models.TaskStatusAssigned {
		return errors.Errorf("task cannot be assigned in status %s", task.Status)
	}

	// Check if agent exists and is available
	agent, err := s.agentService.GetAgent(ctx, agentID)
	if err != nil {
		return errors.Wrap(err, "failed to get agent")
	}
	if models.AgentStatus(agent.Status) != models.AgentStatusActive {
		return errors.Errorf("agent is not active: %s", agent.Status)
	}

	// Update task assignment
	task.AssignedTo = &agentID
	task.Status = models.TaskStatusAssigned
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to assign task", map[string]interface{}{
			"error":    err.Error(),
			"task_id":  taskID.String(),
			"agent_id": agentID,
		})
		return errors.Wrap(err, "failed to assign task")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", taskID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Record metrics
	s.config.Metrics.IncrementCounter("task.assigned", 1.0)

	return nil
}

func (s *taskService) AutoAssignTask(ctx context.Context, taskID uuid.UUID, strategy AssignmentStrategy) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.AutoAssignTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "assign") {
		return errors.New("unauthorized to assign task")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Check if task can be assigned
	if task.Status != models.TaskStatusPending && task.Status != models.TaskStatusAssigned {
		return errors.Errorf("task cannot be assigned in status %s", task.Status)
	}

	// Use assignment engine to find suitable agent
	selectedAgent, err := s.assignmentEngine.FindBestAgent(ctx, task)
	if err != nil {
		return errors.Wrap(err, "failed to auto-assign task")
	}

	if selectedAgent == nil {
		return errors.New("no suitable agent found")
	}

	// Assign task to selected agent
	return s.AssignTask(ctx, taskID, selectedAgent.ID)
}

func (s *taskService) AcceptTask(ctx context.Context, taskID uuid.UUID, agentID string) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.AcceptTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if agentID == "" {
		return errors.New("agent ID is required")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Verify task is assigned to this agent
	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		return errors.New("task is not assigned to this agent")
	}

	// Verify task can be accepted
	if task.Status != models.TaskStatusAssigned {
		return errors.Errorf("task cannot be accepted in status %s", task.Status)
	}

	// Update task status
	task.Status = models.TaskStatusAccepted
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to accept task", map[string]interface{}{
			"error":    err.Error(),
			"task_id":  taskID.String(),
			"agent_id": agentID,
		})
		return errors.Wrap(err, "failed to accept task")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", taskID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Send notification
	if s.notifier != nil {
		_ = s.notifier.NotifyTaskAssigned(ctx, agentID, task)
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.accepted", 1.0)

	return nil
}

func (s *taskService) RejectTask(ctx context.Context, taskID uuid.UUID, agentID string, reason string) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.RejectTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if agentID == "" {
		return errors.New("agent ID is required")
	}
	if reason == "" {
		return errors.New("rejection reason is required")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Verify task is assigned to this agent
	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		return errors.New("task is not assigned to this agent")
	}

	// Verify task can be rejected
	if task.Status != models.TaskStatusAssigned {
		return errors.Errorf("task cannot be rejected in status %s", task.Status)
	}

	// Update task status
	task.Status = models.TaskStatusRejected
	task.AssignedTo = nil // Unassign the task
	task.UpdatedAt = time.Now()
	// Store rejection info in result field
	if task.Result == nil {
		task.Result = make(models.JSONMap)
	}
	task.Result["rejection_reason"] = reason
	task.Result["rejected_by"] = agentID
	task.Result["rejected_at"] = time.Now().Format(time.RFC3339)

	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to reject task", map[string]interface{}{
			"error":    err.Error(),
			"task_id":  taskID.String(),
			"agent_id": agentID,
			"reason":   reason,
		})
		return errors.Wrap(err, "failed to reject task")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", taskID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Send notification of failure
	if s.notifier != nil {
		_ = s.notifier.NotifyTaskFailed(ctx, taskID, agentID, reason)
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.rejected", 1.0)

	return nil
}

func (s *taskService) StartTask(ctx context.Context, taskID uuid.UUID, agentID string) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.StartTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if agentID == "" {
		return errors.New("agent ID is required")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Verify task is assigned to this agent
	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		return errors.New("task is not assigned to this agent")
	}

	// Verify task can be started
	if task.Status != models.TaskStatusAccepted {
		return errors.Errorf("task cannot be started in status %s", task.Status)
	}

	// Update task status
	task.Status = models.TaskStatusInProgress
	task.StartedAt = timePtr(time.Now())
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to start task", map[string]interface{}{
			"error":    err.Error(),
			"task_id":  taskID.String(),
			"agent_id": agentID,
		})
		return errors.Wrap(err, "failed to start task")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", taskID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Send notification
	if s.notifier != nil {
		_ = s.notifier.NotifyTaskAssigned(ctx, agentID, task)
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.started", 1.0)

	return nil
}

func (s *taskService) UpdateProgress(ctx context.Context, taskID uuid.UUID, progress int, message string) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.UpdateProgress")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if progress < 0 || progress > 100 {
		return errors.New("progress must be between 0 and 100")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Verify task is in progress
	if task.Status != models.TaskStatusInProgress {
		return errors.Errorf("cannot update progress for task in status %s", task.Status)
	}

	// Store progress in result field
	if task.Result == nil {
		task.Result = make(models.JSONMap)
	}
	task.Result["progress"] = progress
	task.Result["last_progress_update"] = time.Now().Format(time.RFC3339)
	if message != "" {
		task.Result["progress_message"] = message
	}
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to update task progress", map[string]interface{}{
			"error":    err.Error(),
			"task_id":  taskID.String(),
			"progress": progress,
		})
		return errors.Wrap(err, "failed to update task progress")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", taskID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Send notification for major milestones
	if s.notifier != nil && progress == 100 {
		// Extract agent ID from task
		if task.AssignedTo != nil {
			_ = s.notifier.NotifyTaskCompleted(ctx, *task.AssignedTo, task)
		}
	}

	// Record metrics
	s.config.Metrics.RecordGauge("task.progress", float64(progress), map[string]string{
		"task_id": taskID.String(),
	})

	return nil
}

func (s *taskService) CompleteTask(ctx context.Context, taskID uuid.UUID, agentID string, result interface{}) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.CompleteTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if agentID == "" {
		return errors.New("agent ID is required")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Verify task is assigned to this agent
	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		return errors.New("task is not assigned to this agent")
	}

	// Verify task can be completed
	if task.Status != models.TaskStatusInProgress {
		return errors.Errorf("task cannot be completed in status %s", task.Status)
	}

	// Update task status
	task.Status = models.TaskStatusCompleted
	task.CompletedAt = timePtr(time.Now())
	task.UpdatedAt = time.Now()

	// Store result
	if resultMap, ok := result.(map[string]interface{}); ok {
		task.Result = models.JSONMap(resultMap)
	} else if task.Result == nil {
		task.Result = make(models.JSONMap)
		task.Result["data"] = result
	}

	// Add completion metadata
	task.Result["progress"] = 100
	task.Result["completed_by"] = agentID

	// Calculate duration if started
	if task.StartedAt != nil {
		duration := time.Since(*task.StartedAt)
		task.Result["duration_seconds"] = duration.Seconds()
	}

	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to complete task", map[string]interface{}{
			"error":    err.Error(),
			"task_id":  taskID.String(),
			"agent_id": agentID,
		})
		return errors.Wrap(err, "failed to complete task")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", taskID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Send notification
	if s.notifier != nil {
		_ = s.notifier.NotifyTaskCompleted(ctx, agentID, task)
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.completed", 1.0)
	if task.StartedAt != nil {
		duration := time.Since(*task.StartedAt)
		s.config.Metrics.RecordHistogram("task.duration", duration.Seconds(), map[string]string{
			"type": task.Type,
		})
	}

	return nil
}

func (s *taskService) FailTask(ctx context.Context, taskID uuid.UUID, agentID string, errorMsg string) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.FailTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if agentID == "" {
		return errors.New("agent ID is required")
	}
	if errorMsg == "" {
		return errors.New("error message is required")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Verify task is assigned to this agent
	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		return errors.New("task is not assigned to this agent")
	}

	// Verify task can be failed
	if task.Status != models.TaskStatusInProgress {
		return errors.Errorf("task cannot be failed in status %s", task.Status)
	}

	// Update task status
	task.Status = models.TaskStatusFailed
	task.UpdatedAt = time.Now()
	task.Error = errorMsg

	// Record failure metadata in result
	if task.Result == nil {
		task.Result = make(models.JSONMap)
	}
	task.Result["failed_at"] = time.Now().Format(time.RFC3339)
	task.Result["failed_by"] = agentID
	task.Result["error_message"] = errorMsg

	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to fail task", map[string]interface{}{
			"error":         err.Error(),
			"task_id":       taskID.String(),
			"agent_id":      agentID,
			"error_message": errorMsg,
		})
		return errors.Wrap(err, "failed to fail task")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", taskID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Send notification
	if s.notifier != nil {
		_ = s.notifier.NotifyTaskFailed(ctx, taskID, agentID, errorMsg)
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.failed", 1.0)

	return nil
}

func (s *taskService) RetryTask(ctx context.Context, taskID uuid.UUID) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.RetryTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "retry") {
		return errors.New("unauthorized to retry task")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Verify task can be retried
	if task.Status != models.TaskStatusFailed && task.Status != models.TaskStatusCancelled {
		return errors.Errorf("task cannot be retried in status %s", task.Status)
	}

	// Increment retry count
	if task.RetryCount >= task.MaxRetries {
		return errors.New("task has reached max retry limit")
	}

	// Update task for retry
	task.Status = models.TaskStatusPending
	task.AssignedTo = nil
	task.Error = ""
	task.RetryCount++
	task.UpdatedAt = time.Now()
	task.StartedAt = nil
	task.CompletedAt = nil

	// Clear previous results but keep retry history
	if task.Result == nil {
		task.Result = make(models.JSONMap)
	}
	task.Result["retry_count"] = task.RetryCount
	task.Result["retried_at"] = time.Now().Format(time.RFC3339)

	if err := s.repo.Update(ctx, task); err != nil {
		s.config.Logger.Error("Failed to retry task", map[string]interface{}{
			"error":   err.Error(),
			"task_id": taskID.String(),
		})
		return errors.Wrap(err, "failed to retry task")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("task:%s", taskID.String())
	_ = s.taskCache.Delete(ctx, cacheKey)

	// Auto-assign if configured
	if s.shouldAutoAssign(ctx, task) {
		_ = s.AutoAssignTask(ctx, taskID, AssignmentStrategyLeastLoad)
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.retried", 1.0)

	return nil
}

func (s *taskService) GetAgentTasks(ctx context.Context, agentID string, filters interfaces.TaskFilters) ([]*models.Task, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.GetAgentTasks")
	defer span.End()

	// Validate input
	if agentID == "" {
		return nil, errors.New("agent ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "read") {
		return nil, errors.New("unauthorized to read tasks")
	}

	// Try cache first
	cacheKey := fmt.Sprintf("agent:%s:tasks", agentID)
	var tasks []*models.Task
	if err := s.taskCache.Get(ctx, cacheKey, &tasks); err == nil {
		return tasks, nil
	}

	// Get from repository using ListByAgent
	page, err := s.repo.ListByAgent(ctx, agentID, types.TaskFilters{
		Status:   filters.Status,
		Priority: filters.Priority,
		Types:    filters.Types,
		Limit:    filters.Limit,
		Offset:   filters.Offset,
	})
	if err != nil {
		s.config.Logger.Error("Failed to get agent tasks", map[string]interface{}{
			"error":    err.Error(),
			"agent_id": agentID,
		})
		return nil, errors.Wrap(err, "failed to get agent tasks")
	}

	tasks = page.Tasks

	// Cache the results
	_ = s.taskCache.Set(ctx, cacheKey, tasks, 1*time.Minute)

	// Record metrics
	s.config.Metrics.IncrementCounter("task.agent_tasks_retrieved", float64(len(tasks)))

	return tasks, nil
}

func (s *taskService) GetAvailableTasks(ctx context.Context, agentID string, capabilities []string) ([]*models.Task, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.GetAvailableTasks")
	defer span.End()

	// Validate input
	if agentID == "" {
		return nil, errors.New("agent ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "read") {
		return nil, errors.New("unauthorized to read tasks")
	}

	// Get agent to check status
	agent, err := s.agentService.GetAgent(ctx, agentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get agent")
	}

	if models.AgentStatus(agent.Status) != models.AgentStatusActive {
		return []*models.Task{}, nil // No tasks for inactive agents
	}

	// Build filters for unassigned tasks
	taskFilters := types.TaskFilters{
		Status: []string{string(models.TaskStatusPending)},
		Limit:  100,
	}

	// Get pending tasks for tenant
	page, err := s.repo.ListByTenant(ctx, agent.TenantID, taskFilters)
	if err != nil {
		s.config.Logger.Error("Failed to get available tasks", map[string]interface{}{
			"error":    err.Error(),
			"agent_id": agentID,
		})
		return nil, errors.Wrap(err, "failed to get available tasks")
	}

	tasks := page.Tasks

	// Filter tasks by agent capabilities if provided
	if len(capabilities) > 0 {
		filteredTasks := make([]*models.Task, 0)
		for _, task := range tasks {
			// Check if task type matches agent capabilities
			for _, cap := range capabilities {
				if task.Type == cap {
					filteredTasks = append(filteredTasks, task)
					break
				}
			}
		}
		tasks = filteredTasks
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.available_tasks_retrieved", float64(len(tasks)))

	return tasks, nil
}

func (s *taskService) SearchTasks(ctx context.Context, query string, filters interfaces.TaskFilters) ([]*models.Task, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.SearchTasks")
	defer span.End()

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "read") {
		return nil, errors.New("unauthorized to search tasks")
	}

	// Search in repository
	result, err := s.repo.SearchTasks(ctx, query, types.TaskFilters{
		Status:     filters.Status,
		Priority:   filters.Priority,
		Types:      filters.Types,
		AssignedTo: filters.AssignedTo,
		Limit:      filters.Limit,
		Offset:     filters.Offset,
	})
	if err != nil {
		s.config.Logger.Error("Failed to search tasks", map[string]interface{}{
			"error": err.Error(),
			"query": query,
		})
		return nil, errors.Wrap(err, "failed to search tasks")
	}

	tasks := result.Tasks

	// Record metrics
	s.config.Metrics.IncrementCounter("task.search", 1.0)
	s.config.Metrics.RecordGauge("task.search_results", float64(len(tasks)), nil)

	return tasks, nil
}

func (s *taskService) GetTaskTimeline(ctx context.Context, taskID uuid.UUID) ([]*models.TaskEvent, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.GetTaskTimeline")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return nil, errors.New("task ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "read") {
		return nil, errors.New("unauthorized to read task timeline")
	}

	// Get task to verify it exists
	_, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get task")
	}

	// Get timeline from repository
	events, err := s.repo.GetTaskTimeline(ctx, taskID)
	if err != nil {
		s.config.Logger.Error("Failed to get task timeline", map[string]interface{}{
			"error":   err.Error(),
			"task_id": taskID.String(),
		})
		return nil, errors.Wrap(err, "failed to get task timeline")
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.timeline_retrieved", 1.0)

	// Convert to models.TaskEvent
	modelEvents := make([]*models.TaskEvent, len(events))
	for i, e := range events {
		modelEvents[i] = &models.TaskEvent{
			ID:        uuid.New(),
			TaskID:    taskID,
			Type:      e.EventType,
			Timestamp: e.Timestamp,
			AgentID:   e.AgentID,
			Data:      e.Details,
		}
	}

	return modelEvents, nil
}

func (s *taskService) SubmitSubtaskResult(ctx context.Context, parentTaskID, subtaskID uuid.UUID, result interface{}) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.SubmitSubtaskResult")
	defer span.End()

	// Validate input
	if parentTaskID == uuid.Nil {
		return errors.New("parent task ID is required")
	}
	if subtaskID == uuid.Nil {
		return errors.New("subtask ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "update") {
		return errors.New("unauthorized to submit subtask result")
	}

	// Get parent task
	parentTask, err := s.repo.Get(ctx, parentTaskID)
	if err != nil {
		return errors.Wrap(err, "failed to get parent task")
	}

	// Get subtask
	subtask, err := s.repo.Get(ctx, subtaskID)
	if err != nil {
		return errors.Wrap(err, "failed to get subtask")
	}

	// Verify subtask belongs to parent
	if subtask.ParentTaskID == nil || *subtask.ParentTaskID != parentTaskID {
		return errors.New("subtask does not belong to parent task")
	}

	// Update subtask result
	if subtask.Result == nil {
		subtask.Result = make(models.JSONMap)
	}
	if resultMap, ok := result.(map[string]interface{}); ok {
		for k, v := range resultMap {
			subtask.Result[k] = v
		}
	} else {
		subtask.Result["data"] = result
	}

	// Update parent task with subtask result
	if parentTask.Result == nil {
		parentTask.Result = make(models.JSONMap)
	}
	subtaskResults, ok := parentTask.Result["subtask_results"].(map[string]interface{})
	if !ok {
		subtaskResults = make(map[string]interface{})
		parentTask.Result["subtask_results"] = subtaskResults
	}
	subtaskResults[subtaskID.String()] = subtask.Result

	// Update parent task
	if err := s.repo.Update(ctx, parentTask); err != nil {
		return errors.Wrap(err, "failed to update parent task")
	}

	// Invalidate caches
	parentCacheKey := fmt.Sprintf("task:%s", parentTaskID.String())
	subtaskCacheKey := fmt.Sprintf("task:%s", subtaskID.String())
	_ = s.taskCache.Delete(ctx, parentCacheKey)
	_ = s.taskCache.Delete(ctx, subtaskCacheKey)

	// Record metrics
	s.config.Metrics.IncrementCounter("task.subtask_result_submitted", 1.0)

	return nil
}

func (s *taskService) GetTaskTree(ctx context.Context, rootTaskID uuid.UUID) (*models.TaskTree, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.GetTaskTree")
	defer span.End()

	// Validate input
	if rootTaskID == uuid.Nil {
		return nil, errors.New("root task ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "read") {
		return nil, errors.New("unauthorized to read task tree")
	}

	// Get root task
	rootTask, err := s.repo.Get(ctx, rootTaskID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root task")
	}

	// Get all subtasks recursively
	tree, err := s.repo.GetTaskTree(ctx, rootTaskID, 10)
	if err != nil {
		s.config.Logger.Error("Failed to get task tree", map[string]interface{}{
			"error":        err.Error(),
			"root_task_id": rootTaskID.String(),
		})
		return nil, errors.Wrap(err, "failed to get task tree")
	}

	// If repository doesn't return a proper tree, build it
	if tree == nil {
		tree = &models.TaskTree{
			Root:     rootTask,
			Children: make(map[uuid.UUID][]*models.Task),
		}
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.tree_retrieved", 1.0)

	return tree, nil
}

func (s *taskService) CancelTaskTree(ctx context.Context, rootTaskID uuid.UUID, reason string) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.CancelTaskTree")
	defer span.End()

	// Validate input
	if rootTaskID == uuid.Nil {
		return errors.New("root task ID is required")
	}
	if reason == "" {
		return errors.New("cancellation reason is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "cancel") {
		return errors.New("unauthorized to cancel task tree")
	}

	// Get task tree
	tree, err := s.GetTaskTree(ctx, rootTaskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task tree")
	}

	// Cancel all tasks in tree
	var eg errgroup.Group
	cancelledCount := 0

	// Cancel root task
	eg.Go(func() error {
		if tree.Root.Status != models.TaskStatusCompleted && tree.Root.Status != models.TaskStatusCancelled {
			tree.Root.Status = models.TaskStatusCancelled
			tree.Root.Error = reason
			tree.Root.UpdatedAt = time.Now()
			if err := s.repo.Update(ctx, tree.Root); err != nil {
				return err
			}
			cancelledCount++
		}
		return nil
	})

	// Cancel all children
	for _, children := range tree.Children {
		for _, child := range children {
			child := child // Capture loop variable
			eg.Go(func() error {
				if child.Status != models.TaskStatusCompleted && child.Status != models.TaskStatusCancelled {
					child.Status = models.TaskStatusCancelled
					child.Error = reason
					child.UpdatedAt = time.Now()
					if err := s.repo.Update(ctx, child); err != nil {
						return err
					}
					cancelledCount++
				}
				return nil
			})
		}
	}

	// Wait for all cancellations
	if err := eg.Wait(); err != nil {
		s.config.Logger.Error("Failed to cancel task tree", map[string]interface{}{
			"error":        err.Error(),
			"root_task_id": rootTaskID.String(),
		})
		return errors.Wrap(err, "failed to cancel task tree")
	}

	// Invalidate caches
	_ = s.taskCache.Delete(ctx, fmt.Sprintf("task:%s", rootTaskID.String()))

	// Record metrics
	s.config.Metrics.IncrementCounter("task.tree_cancelled", float64(cancelledCount))

	return nil
}

func (s *taskService) CreateWorkflowTask(ctx context.Context, workflowID, stepID uuid.UUID, params map[string]interface{}) (*models.Task, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.CreateWorkflowTask")
	defer span.End()

	// Validate input
	if workflowID == uuid.Nil {
		return nil, errors.New("workflow ID is required")
	}
	if stepID == uuid.Nil {
		return nil, errors.New("step ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "create") {
		return nil, errors.New("unauthorized to create workflow task")
	}

	// Get workflow and step from workflow service
	if s.workflowService == nil {
		return nil, errors.New("workflow service not configured")
	}

	// Create task for workflow step
	task := &models.Task{
		ID:          uuid.New(),
		TenantID:    auth.GetTenantID(ctx),
		Type:        "workflow_step",
		Title:       fmt.Sprintf("Workflow Step %s", stepID.String()),
		Description: "Task created from workflow step",
		Status:      models.TaskStatusPending,
		Priority:    models.TaskPriorityNormal,
		Parameters:  params,
		Result:      make(models.JSONMap),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Add workflow metadata
	task.Result["workflow_id"] = workflowID.String()
	task.Result["step_id"] = stepID.String()

	// Create task
	if err := s.Create(ctx, task, ""); err != nil {
		s.config.Logger.Error("Failed to create workflow task", map[string]interface{}{
			"error":       err.Error(),
			"workflow_id": workflowID.String(),
			"step_id":     stepID.String(),
		})
		return nil, errors.Wrap(err, "failed to create workflow task")
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.workflow_task_created", 1.0)

	return task, nil
}

func (s *taskService) CompleteWorkflowTask(ctx context.Context, taskID uuid.UUID, output interface{}) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.CompleteWorkflowTask")
	defer span.End()

	// Validate input
	if taskID == uuid.Nil {
		return errors.New("task ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "complete") {
		return errors.New("unauthorized to complete workflow task")
	}

	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to get task")
	}

	// Verify this is a workflow task
	if task.Type != "workflow_step" {
		return errors.New("task is not a workflow task")
	}

	// Get agent ID from assigned agent
	agentID := ""
	if task.AssignedTo != nil {
		agentID = *task.AssignedTo
	}

	// Complete the task
	if err := s.CompleteTask(ctx, taskID, agentID, output); err != nil {
		return errors.Wrap(err, "failed to complete workflow task")
	}

	// Workflow service notification handled elsewhere

	// Record metrics
	s.config.Metrics.IncrementCounter("task.workflow_task_completed", 1.0)

	return nil
}

func (s *taskService) GetTaskStats(ctx context.Context, filters interfaces.TaskFilters) (*models.TaskStats, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.GetTaskStats")
	defer span.End()

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "read") {
		return nil, errors.New("unauthorized to read task stats")
	}

	// Try cache first
	cacheKey := fmt.Sprintf("tenant:%s:stats", auth.GetTenantID(ctx))
	var stats models.TaskStats
	if err := s.statsCache.Get(ctx, cacheKey, &stats); err == nil {
		return &stats, nil
	}

	// Get stats from repository
	statsPtr, err := s.repo.GetTaskStats(ctx, auth.GetTenantID(ctx), 30*24*time.Hour)
	if err != nil {
		s.config.Logger.Error("Failed to get task stats", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, errors.Wrap(err, "failed to get task stats")
	}

	// Convert to models.TaskStats
	stats = models.TaskStats{
		TotalTasks:      statsPtr.TotalCount,
		TasksByStatus:   make(map[models.TaskStatus]int64),
		TasksByPriority: make(map[models.TaskPriority]int64),
		TasksByType:     make(map[string]int64),
		AverageTime:     float64(statsPtr.AverageCompletion.Seconds()),
		SuccessRate:     float64(statsPtr.CompletedCount) / float64(statsPtr.TotalCount),
	}

	// Convert status map
	for status, count := range statsPtr.ByStatus {
		stats.TasksByStatus[models.TaskStatus(status)] = count
	}

	// Convert priority map
	for priority, count := range statsPtr.ByPriority {
		stats.TasksByPriority[models.TaskPriority(priority)] = count
	}

	// Copy type map
	stats.TasksByType = statsPtr.ByAgent

	// Cache the results
	_ = s.statsCache.Set(ctx, cacheKey, stats, 1*time.Minute)

	// Record metrics
	s.config.Metrics.IncrementCounter("task.stats_retrieved", 1.0)

	return &stats, nil
}

func (s *taskService) GetAgentPerformance(ctx context.Context, agentID string, period time.Duration) (*models.AgentPerformance, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.GetAgentPerformance")
	defer span.End()

	// Validate input
	if agentID == "" {
		return nil, errors.New("agent ID is required")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "read") {
		return nil, errors.New("unauthorized to read agent performance")
	}

	// Get agent workload first
	workloadMap, err := s.repo.GetAgentWorkload(ctx, []string{agentID})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get agent workload")
	}

	workload := workloadMap[agentID]
	if workload == nil {
		workload = &interfaces.AgentWorkload{}
	}

	// Build performance metrics
	perf := &models.AgentPerformance{
		AgentID:               agentID,
		TasksCompleted:        int64(workload.CompletedToday),
		TasksFailed:           0, // Not available in workload
		AverageCompletionTime: workload.AverageTime.Seconds(),
		SuccessRate:           workload.CurrentCapacity,
		LoadFactor:            1.0 - workload.CurrentCapacity,
		SpeedScore:            workload.CurrentCapacity,
		TaskTypeMetrics:       make(map[string]models.TaskMetrics),
	}
	// Removed error handling for workload - it's handled above

	// Calculate additional metrics if not provided
	if perf.SuccessRate == 0 && perf.TasksCompleted > 0 {
		totalTasks := perf.TasksCompleted + perf.TasksFailed
		if totalTasks > 0 {
			perf.SuccessRate = float64(perf.TasksCompleted) / float64(totalTasks)
		}
	}

	// Get current workload for load factor
	if s.agentService != nil {
		workload, err := s.agentService.GetAgentWorkload(ctx, agentID)
		if err == nil && workload != nil {
			perf.LoadFactor = workload.LoadScore
		}
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.agent_performance_retrieved", 1.0)

	return perf, nil
}

func (s *taskService) GenerateTaskReport(ctx context.Context, filters interfaces.TaskFilters, format string) ([]byte, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.GenerateTaskReport")
	defer span.End()

	// Validate format
	if format != "json" && format != "csv" {
		return nil, errors.New("unsupported format, use 'json' or 'csv'")
	}

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "report") {
		return nil, errors.New("unauthorized to generate task report")
	}

	// Get tasks for report based on tenant
	page, err := s.repo.ListByTenant(ctx, auth.GetTenantID(ctx), types.TaskFilters{
		Status:   filters.Status,
		Priority: filters.Priority,
		Types:    filters.Types,
		Limit:    filters.Limit,
		Offset:   filters.Offset,
	})
	if err != nil {
		s.config.Logger.Error("Failed to get tasks for report", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, errors.Wrap(err, "failed to get tasks for report")
	}

	tasks := page.Tasks

	// Get stats
	stats, err := s.GetTaskStats(ctx, filters)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get task stats")
	}

	// Generate report based on format
	var reportData []byte
	switch format {
	case "json":
		report := map[string]interface{}{
			"generated_at": time.Now().Format(time.RFC3339),
			"filters":      filters,
			"stats":        stats,
			"tasks":        tasks,
		}
		reportData, err = json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal JSON report")
		}
	case "csv":
		// Simple CSV implementation
		var csvData string
		csvData = "ID,Type,Title,Status,Priority,AssignedTo,CreatedAt,CompletedAt\n"
		for _, task := range tasks {
			assignedTo := ""
			if task.AssignedTo != nil {
				assignedTo = *task.AssignedTo
			}
			completedAt := ""
			if task.CompletedAt != nil {
				completedAt = task.CompletedAt.Format(time.RFC3339)
			}
			csvData += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s\n",
				task.ID.String(),
				task.Type,
				task.Title,
				task.Status,
				task.Priority,
				assignedTo,
				task.CreatedAt.Format(time.RFC3339),
				completedAt,
			)
		}
		reportData = []byte(csvData)
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.report_generated", 1.0)

	return reportData, nil
}

func (s *taskService) ArchiveCompletedTasks(ctx context.Context, before time.Time) (int64, error) {
	ctx, span := s.config.Tracer(ctx, "TaskService.ArchiveCompletedTasks")
	defer span.End()

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "archive") {
		return 0, errors.New("unauthorized to archive tasks")
	}

	// Archive tasks in repository
	count, err := s.repo.ArchiveTasks(ctx, before)
	if err != nil {
		s.config.Logger.Error("Failed to archive completed tasks", map[string]interface{}{
			"error":  err.Error(),
			"before": before.Format(time.RFC3339),
		})
		return 0, errors.Wrap(err, "failed to archive completed tasks")
	}

	// Clear caches after archiving
	if count > 0 {
		// Clear stats cache as counts have changed
		statsKey := fmt.Sprintf("tenant:%s:stats", auth.GetTenantID(ctx))
		_ = s.statsCache.Delete(ctx, statsKey)
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.archived", float64(count))
	s.config.Logger.Info("Archived completed tasks", map[string]interface{}{
		"count":  count,
		"before": before.Format(time.RFC3339),
	})

	return count, nil
}

func (s *taskService) RebalanceTasks(ctx context.Context) error {
	ctx, span := s.config.Tracer(ctx, "TaskService.RebalanceTasks")
	defer span.End()

	// Check authorization
	if s.config.Authorizer != nil && !s.config.Authorizer.CheckPermission(ctx, "task", "rebalance") {
		return errors.New("unauthorized to rebalance tasks")
	}

	// Use task rebalancer if available - skip to avoid circular call
	// The rebalancer calls this method, so we don't call it back

	// Manual rebalancing logic
	// Get all assigned but not started tasks
	rebalanceFilters := types.TaskFilters{
		Status: []string{string(models.TaskStatusAssigned)},
		Limit:  1000,
	}

	page, err := s.repo.ListByTenant(ctx, auth.GetTenantID(ctx), rebalanceFilters)
	if err != nil {
		return errors.Wrap(err, "failed to get assigned tasks")
	}

	tasks := page.Tasks

	// Get agent workloads
	agentWorkloads := make(map[string]*models.AgentWorkload)
	for _, task := range tasks {
		if task.AssignedTo != nil {
			if _, exists := agentWorkloads[*task.AssignedTo]; !exists {
				workload, err := s.agentService.GetAgentWorkload(ctx, *task.AssignedTo)
				if err == nil {
					agentWorkloads[*task.AssignedTo] = workload
				}
			}
		}
	}

	// Rebalance tasks from overloaded agents
	rebalancedCount := 0
	for _, task := range tasks {
		if task.AssignedTo != nil {
			workload := agentWorkloads[*task.AssignedTo]
			if workload != nil && workload.LoadScore > 0.8 { // Overloaded threshold
				// Try to reassign
				if err := s.AutoAssignTask(ctx, task.ID, AssignmentStrategyLeastLoad); err == nil {
					rebalancedCount++
				}
			}
		}
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("task.rebalanced", float64(rebalancedCount))
	s.config.Logger.Info("Rebalanced tasks", map[string]interface{}{
		"count": rebalancedCount,
	})

	return nil
}

// Helper functions

func (s *taskService) checkIdempotency(ctx context.Context, key string) (uuid.UUID, error) {
	if key == "" {
		return uuid.Nil, errors.New("not found")
	}

	// Check cache for idempotency key
	var taskID uuid.UUID
	if err := s.taskCache.Get(ctx, "idem:"+key, &taskID); err == nil {
		return taskID, nil
	}

	return uuid.Nil, errors.New("not found")
}

func (s *taskService) storeIdempotencyKey(ctx context.Context, key string, id uuid.UUID) error {
	if key == "" {
		return nil
	}

	// Store in cache with TTL
	return s.taskCache.Set(ctx, "idem:"+key, id, 24*time.Hour)
}

func (s *taskService) validateDistributedTask(ctx context.Context, dt *models.DistributedTask) error {
	// Set defaults
	dt.SetDefaults()

	// Validate the distributed task
	if err := dt.Validate(); err != nil {
		return errors.Wrap(err, "invalid distributed task")
	}

	// Validate subtasks
	if len(dt.Subtasks) == 0 {
		return errors.New("distributed task must have at least one subtask")
	}

	// Validate aggregation config
	if dt.Aggregation.Method == "" {
		dt.Aggregation.Method = "combine_results"
	}

	return nil
}

func (s *taskService) shouldAutoAssign(ctx context.Context, task *models.Task) bool {
	// Auto-assign based on task priority and type
	if task.Priority == models.TaskPriorityHigh {
		return true
	}

	// Check if rule engine has auto-assign rules
	if s.config.RuleEngine != nil {
		rules, err := s.config.RuleEngine.GetRules(ctx, "task.auto_assign", map[string]interface{}{
			"task_type": task.Type,
			"priority":  task.Priority,
		})
		if err == nil && len(rules) > 0 {
			return true
		}
	}

	return false
}

func (s *taskService) notifyDistributedTaskCreated(ctx context.Context, dt *models.DistributedTask) {
	if s.notifier != nil && dt.Task != nil {
		// Notify about distributed task creation
		if dt.Task.AssignedTo != nil {
			_ = s.notifier.NotifyTaskAssigned(ctx, *dt.Task.AssignedTo, dt.Task)
		}

		// Record metrics
		s.config.Metrics.IncrementCounter("task.distributed_created", 1.0)
		s.config.Metrics.RecordGauge("task.distributed_subtasks", float64(len(dt.Subtasks)), nil)
	}
}

func (s *taskService) validateDelegation(ctx context.Context, task *models.Task, delegation *models.TaskDelegation) error {
	// Validate task can be delegated
	if task.Status == models.TaskStatusCompleted || task.Status == models.TaskStatusCancelled {
		return errors.New("cannot delegate completed or cancelled task")
	}

	// Validate delegation fields
	if delegation.TaskID != task.ID {
		return errors.New("delegation task ID must match task ID")
	}
	if delegation.FromAgentID == "" {
		return errors.New("delegation from agent is required")
	}
	if delegation.ToAgentID == "" {
		return errors.New("delegation to agent is required")
	}
	if delegation.FromAgentID == delegation.ToAgentID {
		return errors.New("cannot delegate to same agent")
	}

	// Check if from agent is assigned to task
	if task.AssignedTo == nil || *task.AssignedTo != delegation.FromAgentID {
		return errors.New("task is not assigned to delegating agent")
	}

	// Verify target agent exists and is active
	if s.agentService != nil {
		agent, err := s.agentService.GetAgent(ctx, delegation.ToAgentID)
		if err != nil {
			return errors.Wrap(err, "failed to get target agent")
		}
		if models.AgentStatus(agent.Status) != models.AgentStatusActive {
			return errors.New("target agent is not active")
		}
	}

	return nil
}
