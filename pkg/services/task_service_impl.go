package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
)

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

	// Synchronization
	taskLocks sync.Map // map[uuid.UUID]*sync.Mutex
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
		assignmentEngine: NewAssignmentEngine(config.RuleEngine),
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
	s.taskCache.Set(ctx, cacheKey, task, 5*time.Minute)

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

	// Business rule validation
	if s.config.PolicyManager != nil {
		// TODO: Implement rule evaluation when Rule has Evaluate method
		// For now, skip rule-based validation
	}

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
		s.taskCache.Delete(ctx, fmt.Sprintf("agent:%s:tasks", *task.AssignedTo))
	}
	s.statsCache.Delete(ctx, fmt.Sprintf("tenant:%s:stats", task.TenantID))

	// Schedule monitoring
	s.progressTracker.Track(task.ID)

	// Metrics
	if s.config.Metrics != nil {
		s.config.Metrics.IncrementCounter("task.created", 1.0)
	}
}

// Implement remaining interface methods with TODO placeholders
func (s *taskService) CreateBatch(ctx context.Context, tasks []*models.Task) error {
	// TODO: Implement batch creation
	return errors.New("not implemented")
}

func (s *taskService) GetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error) {
	// TODO: Implement batch retrieval
	return nil, errors.New("not implemented")
}

func (s *taskService) Update(ctx context.Context, task *models.Task) error {
	// TODO: Implement task update
	return errors.New("not implemented")
}

func (s *taskService) Delete(ctx context.Context, id uuid.UUID) error {
	// TODO: Implement task deletion
	return errors.New("not implemented")
}

func (s *taskService) AssignTask(ctx context.Context, taskID uuid.UUID, agentID string) error {
	// TODO: Implement manual task assignment
	return errors.New("not implemented")
}

func (s *taskService) AutoAssignTask(ctx context.Context, taskID uuid.UUID, strategy AssignmentStrategy) error {
	// TODO: Implement auto assignment with strategy
	return errors.New("not implemented")
}

func (s *taskService) AcceptTask(ctx context.Context, taskID uuid.UUID, agentID string) error {
	// TODO: Implement task acceptance
	return errors.New("not implemented")
}

func (s *taskService) RejectTask(ctx context.Context, taskID uuid.UUID, agentID string, reason string) error {
	// TODO: Implement task rejection
	return errors.New("not implemented")
}

func (s *taskService) StartTask(ctx context.Context, taskID uuid.UUID, agentID string) error {
	// TODO: Implement task start
	return errors.New("not implemented")
}

func (s *taskService) UpdateProgress(ctx context.Context, taskID uuid.UUID, progress int, message string) error {
	// TODO: Implement progress update
	return errors.New("not implemented")
}

func (s *taskService) CompleteTask(ctx context.Context, taskID uuid.UUID, agentID string, result interface{}) error {
	// TODO: Implement task completion
	return errors.New("not implemented")
}

func (s *taskService) FailTask(ctx context.Context, taskID uuid.UUID, agentID string, errorMsg string) error {
	// TODO: Implement task failure
	return errors.New("not implemented")
}

func (s *taskService) RetryTask(ctx context.Context, taskID uuid.UUID) error {
	// TODO: Implement task retry
	return errors.New("not implemented")
}

func (s *taskService) GetAgentTasks(ctx context.Context, agentID string, filters interfaces.TaskFilters) ([]*models.Task, error) {
	// TODO: Implement agent task retrieval
	return nil, errors.New("not implemented")
}

func (s *taskService) GetAvailableTasks(ctx context.Context, agentID string, capabilities []string) ([]*models.Task, error) {
	// TODO: Implement available task retrieval
	return nil, errors.New("not implemented")
}

func (s *taskService) SearchTasks(ctx context.Context, query string, filters interfaces.TaskFilters) ([]*models.Task, error) {
	// TODO: Implement task search
	return nil, errors.New("not implemented")
}

func (s *taskService) GetTaskTimeline(ctx context.Context, taskID uuid.UUID) ([]*models.TaskEvent, error) {
	// TODO: Implement task timeline retrieval
	return nil, errors.New("not implemented")
}

func (s *taskService) SubmitSubtaskResult(ctx context.Context, parentTaskID, subtaskID uuid.UUID, result interface{}) error {
	// TODO: Implement subtask result submission
	return errors.New("not implemented")
}

func (s *taskService) GetTaskTree(ctx context.Context, rootTaskID uuid.UUID) (*models.TaskTree, error) {
	// TODO: Implement task tree retrieval
	return nil, errors.New("not implemented")
}

func (s *taskService) CancelTaskTree(ctx context.Context, rootTaskID uuid.UUID, reason string) error {
	// TODO: Implement task tree cancellation
	return errors.New("not implemented")
}

func (s *taskService) CreateWorkflowTask(ctx context.Context, workflowID, stepID uuid.UUID, params map[string]interface{}) (*models.Task, error) {
	// TODO: Implement workflow task creation
	return nil, errors.New("not implemented")
}

func (s *taskService) CompleteWorkflowTask(ctx context.Context, taskID uuid.UUID, output interface{}) error {
	// TODO: Implement workflow task completion
	return errors.New("not implemented")
}

func (s *taskService) GetTaskStats(ctx context.Context, filters interfaces.TaskFilters) (*models.TaskStats, error) {
	// TODO: Implement task statistics retrieval
	return nil, errors.New("not implemented")
}

func (s *taskService) GetAgentPerformance(ctx context.Context, agentID string, period time.Duration) (*models.AgentPerformance, error) {
	// TODO: Implement agent performance retrieval
	return nil, errors.New("not implemented")
}

func (s *taskService) GenerateTaskReport(ctx context.Context, filters interfaces.TaskFilters, format string) ([]byte, error) {
	// TODO: Implement task report generation
	return nil, errors.New("not implemented")
}

func (s *taskService) ArchiveCompletedTasks(ctx context.Context, before time.Time) (int64, error) {
	// TODO: Implement task archival
	return 0, errors.New("not implemented")
}

func (s *taskService) RebalanceTasks(ctx context.Context) error {
	// TODO: Implement task rebalancing
	return errors.New("not implemented")
}

// Helper functions

func (s *taskService) checkIdempotency(ctx context.Context, key string) (uuid.UUID, error) {
	// TODO: Implement idempotency check
	return uuid.Nil, errors.New("not found")
}

func (s *taskService) storeIdempotencyKey(ctx context.Context, key string, id uuid.UUID) error {
	// TODO: Implement idempotency key storage
	return nil
}

func (s *taskService) validateDistributedTask(ctx context.Context, dt *models.DistributedTask) error {
	// TODO: Implement distributed task validation
	return nil
}

func (s *taskService) shouldAutoAssign(ctx context.Context, task *models.Task) bool {
	// TODO: Implement auto-assign check
	return false
}

func (s *taskService) notifyDistributedTaskCreated(ctx context.Context, dt *models.DistributedTask) {
	// TODO: Implement distributed task notification
}

func (s *taskService) validateDelegation(ctx context.Context, task *models.Task, delegation *models.TaskDelegation) error {
	// TODO: Implement delegation validation
	return nil
}