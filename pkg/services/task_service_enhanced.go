package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sony/gobreaker"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// TaskStateMachine defines valid state transitions
type TaskStateMachine struct {
	transitions map[models.TaskStatus][]models.TaskStatus
}

// NewTaskStateMachine creates a new state machine for task status transitions
func NewTaskStateMachine() *TaskStateMachine {
	return &TaskStateMachine{
		transitions: map[models.TaskStatus][]models.TaskStatus{
			models.TaskStatusPending: {
				models.TaskStatusAssigned,
				models.TaskStatusCancelled,
			},
			models.TaskStatusAssigned: {
				models.TaskStatusInProgress,
				models.TaskStatusAccepted,
				models.TaskStatusRejected,
				models.TaskStatusPending, // Unassign
				models.TaskStatusCancelled,
			},
			models.TaskStatusAccepted: {
				models.TaskStatusInProgress,
				models.TaskStatusAssigned, // Re-assign
				models.TaskStatusCancelled,
			},
			models.TaskStatusRejected: {
				models.TaskStatusAssigned, // Re-assign
				models.TaskStatusCancelled,
			},
			models.TaskStatusInProgress: {
				models.TaskStatusCompleted,
				models.TaskStatusFailed,
				models.TaskStatusCancelled,
			},
			models.TaskStatusCompleted: {
				// Terminal state - no transitions
			},
			models.TaskStatusFailed: {
				models.TaskStatusPending, // Retry
				models.TaskStatusCancelled,
			},
			models.TaskStatusCancelled: {
				// Terminal state - no transitions
			},
			models.TaskStatusTimeout: {
				models.TaskStatusPending, // Retry
				models.TaskStatusFailed,
				models.TaskStatusCancelled,
			},
		},
	}
}

// CanTransition checks if a status transition is valid
func (sm *TaskStateMachine) CanTransition(from, to models.TaskStatus) bool {
	validTransitions, exists := sm.transitions[from]
	if !exists {
		return false
	}

	for _, valid := range validTransitions {
		if valid == to {
			return true
		}
	}
	return false
}

// EnhancedTaskService implements TaskService with production features
type EnhancedTaskService struct {
	*taskService // Embed base service

	// Additional dependencies
	txManager      repository.TransactionManager
	uow            database.UnitOfWork
	eventPublisher events.Publisher
	stateMachine   *TaskStateMachine
	circuitBreaker *gobreaker.CircuitBreaker

	// Caches
	idempotencyCache cache.Cache
	delegationCache  cache.Cache
}

// NewEnhancedTaskService creates a production-ready task service
func NewEnhancedTaskService(
	baseService *taskService,
	txManager repository.TransactionManager,
	uow database.UnitOfWork,
	eventPublisher events.Publisher,
) TaskService {
	// Configure circuit breaker
	cbSettings := gobreaker.Settings{
		Name:        "task_service",
		MaxRequests: 100,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			baseService.config.Logger.Info("Circuit breaker state changed", map[string]interface{}{
				"name": name,
				"from": from.String(),
				"to":   to.String(),
			})
		},
	}

	return &EnhancedTaskService{
		taskService:      baseService,
		txManager:        txManager,
		uow:              uow,
		eventPublisher:   eventPublisher,
		stateMachine:     NewTaskStateMachine(),
		circuitBreaker:   gobreaker.NewCircuitBreaker(cbSettings),
		idempotencyCache: cache.NewMemoryCache(10000, 24*time.Hour),
		delegationCache:  cache.NewMemoryCache(5000, 1*time.Hour),
	}
}

// Create creates a task with idempotency support
func (s *EnhancedTaskService) Create(ctx context.Context, task *models.Task, idempotencyKey string) error {
	ctx, span := s.config.Tracer(ctx, "EnhancedTaskService.Create")
	defer span.End()

	// Record metrics
	startTime := time.Now()
	defer func() {
		s.config.Metrics.RecordHistogram("task_create_duration", time.Since(startTime).Seconds(), nil)
	}()

	// Check idempotency if key provided
	if idempotencyKey != "" {
		existingID, err := s.checkIdempotency(ctx, idempotencyKey, "create", task)
		if err == nil && existingID != uuid.Nil {
			task.ID = existingID
			s.config.Metrics.IncrementCounter("task_idempotency_hits", 1)
			return nil
		}
	}

	// Execute with circuit breaker
	_, err := s.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
			// Validate task
			if err := s.validateTask(task); err != nil {
				return errors.Wrap(err, "task validation failed")
			}

			// Set initial status
			if task.Status == "" {
				task.Status = models.TaskStatusPending
			}

			// Generate ID if not set
			if task.ID == uuid.Nil {
				task.ID = uuid.New()
			}

			// Set timestamps
			now := time.Now()
			task.CreatedAt = now
			task.UpdatedAt = now

			// Create task in database
			if err := s.createTaskInTx(ctx, tx, task); err != nil {
				return errors.Wrap(err, "failed to create task")
			}

			// Record state transition
			if err := s.recordStateTransition(ctx, tx, task.ID, "", task.Status, auth.GetAgentID(ctx), "Task created"); err != nil {
				return errors.Wrap(err, "failed to record state transition")
			}

			// Store idempotency key
			if idempotencyKey != "" {
				if err := s.storeIdempotencyKey(ctx, tx, idempotencyKey, "create", task); err != nil {
					return errors.Wrap(err, "failed to store idempotency key")
				}
			}

			// Publish event
			event := &events.TaskCreatedEvent{
				BaseEvent: events.BaseEvent{
					ID:        uuid.New().String(),
					Type:      "task.created",
					Timestamp: time.Now(),
					TenantID:  task.TenantID.String(),
					AgentID:   auth.GetAgentID(ctx),
				},
				TaskID:     task.ID.String(),
				TaskType:   task.Type,
				Title:      task.Title,
				Priority:   string(task.Priority),
				AssignedTo: task.AssignedTo,
			}

			if err := s.eventPublisher.Publish(ctx, event); err != nil {
				s.config.Logger.Error("Failed to publish task created event", map[string]interface{}{
					"error":   err.Error(),
					"task_id": task.ID,
				})
				// Don't fail the operation for event publishing errors
			}

			return nil
		})
	})

	if err != nil {
		return err
	}

	// Update metrics
	s.config.Metrics.IncrementCounter("tasks_created", 1)
	s.config.Metrics.IncrementCounter("task_status_"+string(task.Status), 1)

	return nil
}

// UpdateStatus updates task status with state machine validation
func (s *EnhancedTaskService) UpdateStatus(ctx context.Context, taskID uuid.UUID, newStatus models.TaskStatus, reason string) error {
	ctx, span := s.config.Tracer(ctx, "EnhancedTaskService.UpdateStatus")
	defer span.End()

	return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Get current task
		task, err := s.getTaskInTx(ctx, tx, taskID)
		if err != nil {
			return errors.Wrap(err, "failed to get task")
		}

		// Validate state transition
		if !s.stateMachine.CanTransition(task.Status, newStatus) {
			return errors.Errorf("invalid state transition from %s to %s", task.Status, newStatus)
		}

		// Update status
		oldStatus := task.Status
		task.Status = newStatus
		task.UpdatedAt = time.Now()

		// Update task
		if err := s.updateTaskInTx(ctx, tx, task); err != nil {
			return errors.Wrap(err, "failed to update task")
		}

		// Record state transition
		if err := s.recordStateTransition(ctx, tx, taskID, oldStatus, newStatus, auth.GetAgentID(ctx), reason); err != nil {
			return errors.Wrap(err, "failed to record state transition")
		}

		// Publish event
		event := &events.TaskStatusChangedEvent{
			BaseEvent: events.BaseEvent{
				ID:        uuid.New().String(),
				Type:      "task.status_changed",
				Timestamp: time.Now(),
				TenantID:  task.TenantID.String(),
				AgentID:   auth.GetAgentID(ctx),
			},
			TaskID:    taskID.String(),
			OldStatus: string(oldStatus),
			NewStatus: string(newStatus),
			Reason:    reason,
		}

		if err := s.eventPublisher.Publish(ctx, event); err != nil {
			s.config.Logger.Error("Failed to publish task status changed event", map[string]interface{}{
				"error":   err.Error(),
				"task_id": taskID,
			})
		}

		// Update metrics
		s.config.Metrics.IncrementCounter("task_state_transitions", 1)
		// Update counters instead of gauges
		s.config.Metrics.IncrementCounter("task_status_transitions_from_"+string(oldStatus), 1)
		s.config.Metrics.IncrementCounter("task_status_transitions_to_"+string(newStatus), 1)

		return nil
	})
}

// DelegateTask delegates a task with history tracking
func (s *EnhancedTaskService) DelegateTask(ctx context.Context, delegation *models.TaskDelegation) error {
	ctx, span := s.config.Tracer(ctx, "EnhancedTaskService.DelegateTask")
	defer span.End()

	taskID := delegation.TaskID
	fromAgentID := delegation.FromAgentID
	toAgentID := delegation.ToAgentID
	reason := delegation.Reason

	return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Get task
		task, err := s.getTaskInTx(ctx, tx, taskID)
		if err != nil {
			return errors.Wrap(err, "failed to get task")
		}

		// Check delegation limit from history
		delegationCount, err := s.getDelegationCount(ctx, tx, taskID)
		if err != nil {
			return errors.Wrap(err, "failed to get delegation count")
		}
		if delegationCount >= 5 { // Default max delegations
			return errors.New("task has reached maximum delegation limit")
		}

		// Validate state transition - delegated tasks should be assigned
		if !s.stateMachine.CanTransition(task.Status, models.TaskStatusAssigned) {
			return errors.Errorf("cannot delegate task in status %s", task.Status)
		}

		// Create delegation history record
		if err := s.recordDelegation(ctx, tx, delegation.ID, taskID, fromAgentID, toAgentID, string(delegation.DelegationType), reason); err != nil {
			return errors.Wrap(err, "failed to record delegation")
		}

		// Update task
		oldStatus := task.Status
		task.Status = models.TaskStatusAssigned // Use assigned status for delegated tasks
		task.AssignedTo = &toAgentID
		task.UpdatedAt = time.Now()

		if err := s.updateTaskInTx(ctx, tx, task); err != nil {
			return errors.Wrap(err, "failed to update task")
		}

		// Record state transition
		if err := s.recordStateTransition(ctx, tx, taskID, oldStatus, models.TaskStatusAssigned, fromAgentID, fmt.Sprintf("Delegated to %s: %s", toAgentID, reason)); err != nil {
			return errors.Wrap(err, "failed to record state transition")
		}

		// Publish event
		event := &events.TaskDelegatedEvent{
			BaseEvent: events.BaseEvent{
				ID:        uuid.New().String(),
				Type:      "task.delegated",
				Timestamp: time.Now(),
				TenantID:  task.TenantID.String(),
				AgentID:   fromAgentID,
			},
			TaskID:       taskID.String(),
			FromAgentID:  fromAgentID,
			ToAgentID:    toAgentID,
			DelegationID: delegation.ID.String(),
			Reason:       reason,
		}

		if err := s.eventPublisher.Publish(ctx, event); err != nil {
			s.config.Logger.Error("Failed to publish task delegated event", map[string]interface{}{
				"error":   err.Error(),
				"task_id": taskID,
			})
		}

		// Update metrics
		s.config.Metrics.IncrementCounter("tasks_delegated", 1)
		s.config.Metrics.RecordHistogram("task_delegation_count", float64(delegationCount+1), nil)

		// Notify the assigned agent
		if s.notifier != nil {
			_ = s.notifier.NotifyTaskAssigned(ctx, toAgentID, task)
		}

		return nil
	})
}

// AcceptTask accepts a delegated task with validation
func (s *EnhancedTaskService) AcceptTask(ctx context.Context, taskID uuid.UUID, agentID string) error {
	ctx, span := s.config.Tracer(ctx, "EnhancedTaskService.AcceptTask")
	defer span.End()

	return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Get task
		task, err := s.getTaskInTx(ctx, tx, taskID)
		if err != nil {
			return errors.Wrap(err, "failed to get task")
		}

		// Validate task is delegated to this agent
		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			return errors.New("task is not delegated to this agent")
		}

		// Validate status
		if task.Status != models.TaskStatusAssigned {
			return errors.Errorf("cannot accept task in status %s", task.Status)
		}

		// Update delegation history
		if err := s.acceptDelegation(ctx, tx, taskID, agentID); err != nil {
			return errors.Wrap(err, "failed to update delegation history")
		}

		// Update task status
		task.Status = models.TaskStatusAccepted
		task.UpdatedAt = time.Now()

		if err := s.updateTaskInTx(ctx, tx, task); err != nil {
			return errors.Wrap(err, "failed to update task")
		}

		// Record state transition
		if err := s.recordStateTransition(ctx, tx, taskID, models.TaskStatusAssigned, models.TaskStatusAccepted, agentID, "Task accepted"); err != nil {
			return errors.Wrap(err, "failed to record state transition")
		}

		// Publish event
		event := &events.TaskAcceptedEvent{
			BaseEvent: events.BaseEvent{
				ID:        uuid.New().String(),
				Type:      "task.accepted",
				Timestamp: time.Now(),
				TenantID:  task.TenantID.String(),
				AgentID:   agentID,
			},
			TaskID:  taskID.String(),
			AgentID: agentID,
		}

		if err := s.eventPublisher.Publish(ctx, event); err != nil {
			s.config.Logger.Error("Failed to publish task accepted event", map[string]interface{}{
				"error":   err.Error(),
				"task_id": taskID,
			})
		}

		// Update metrics
		s.config.Metrics.IncrementCounter("tasks_accepted", 1)

		return nil
	})
}

// CompleteTask completes a task with result storage
func (s *EnhancedTaskService) CompleteTask(ctx context.Context, taskID uuid.UUID, agentID string, result interface{}) error {
	ctx, span := s.config.Tracer(ctx, "EnhancedTaskService.CompleteTask")
	defer span.End()

	return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Get task
		task, err := s.getTaskInTx(ctx, tx, taskID)
		if err != nil {
			return errors.Wrap(err, "failed to get task")
		}

		// Validate agent
		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			return errors.New("task is not assigned to this agent")
		}

		// Validate state transition
		if !s.stateMachine.CanTransition(task.Status, models.TaskStatusCompleted) {
			return errors.Errorf("cannot complete task in status %s", task.Status)
		}

		// Store result
		if result != nil {
			task.Result = models.JSONMap{"data": result}
		}

		// Update task
		oldStatus := task.Status
		task.Status = models.TaskStatusCompleted
		now := time.Now()
		task.CompletedAt = &now
		task.UpdatedAt = now

		if err := s.updateTaskInTx(ctx, tx, task); err != nil {
			return errors.Wrap(err, "failed to update task")
		}

		// Record state transition
		if err := s.recordStateTransition(ctx, tx, taskID, oldStatus, models.TaskStatusCompleted, agentID, "Task completed"); err != nil {
			return errors.Wrap(err, "failed to record state transition")
		}

		// Publish event
		event := &events.TaskCompletedEvent{
			BaseEvent: events.BaseEvent{
				ID:        uuid.New().String(),
				Type:      "task.completed",
				Timestamp: time.Now(),
				TenantID:  task.TenantID.String(),
				AgentID:   agentID,
			},
			TaskID:      taskID.String(),
			AgentID:     agentID,
			Result:      result,
			CompletedAt: now,
		}

		if err := s.eventPublisher.Publish(ctx, event); err != nil {
			s.config.Logger.Error("Failed to publish task completed event", map[string]interface{}{
				"error":   err.Error(),
				"task_id": taskID,
			})
		}

		// Update metrics
		s.config.Metrics.IncrementCounter("tasks_completed", 1)
		if task.StartedAt != nil {
			duration := now.Sub(*task.StartedAt).Seconds()
			s.config.Metrics.RecordHistogram("task_completion_duration", duration, map[string]string{
				"task_type": task.Type,
				"priority":  string(task.Priority),
			})
		}

		// Notify completion
		if s.notifier != nil {
			_ = s.notifier.NotifyTaskCompleted(ctx, agentID, task)
		}

		return nil
	})
}

// Helper methods

func (s *EnhancedTaskService) checkIdempotency(ctx context.Context, key, operation string, task *models.Task) (uuid.UUID, error) {
	// Create hash of the request
	hash := s.createRequestHash(operation, task)
	cacheKey := fmt.Sprintf("idempotency:%s:%s", key, hash)

	// Check cache first
	var cachedID uuid.UUID
	if err := s.idempotencyCache.Get(ctx, cacheKey, &cachedID); err == nil {
		return cachedID, nil
	}

	// For now, return not found - this would need a proper repository method
	// TODO: Implement GetByIdempotencyKey in repository
	return uuid.Nil, errors.New("no existing task found")
}

func (s *EnhancedTaskService) storeIdempotencyKey(ctx context.Context, tx database.Transaction, key, operation string, task *models.Task) error {
	hash := s.createRequestHash(operation, task)

	query := `
		INSERT INTO task_idempotency_keys (idempotency_key, task_id, operation, request_hash, response, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (idempotency_key) DO NOTHING
	`

	response, _ := json.Marshal(map[string]interface{}{
		"task_id": task.ID,
		"status":  task.Status,
	})

	_, err := tx.ExecContext(ctx, query, key, task.ID, operation, hash, response, time.Now().Add(24*time.Hour))
	return err
}

func (s *EnhancedTaskService) createRequestHash(operation string, task *models.Task) string {
	data := fmt.Sprintf("%s:%s:%s:%s:%s", operation, task.Type, task.Title, task.Description, task.Priority)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *EnhancedTaskService) recordStateTransition(ctx context.Context, tx database.Transaction, taskID uuid.UUID, from, to models.TaskStatus, agentID, reason string) error {
	query := `
		INSERT INTO task_state_transitions (task_id, from_status, to_status, agent_id, reason, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	metadata := map[string]interface{}{
		"timestamp": time.Now().Unix(),
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err := tx.ExecContext(ctx, query, taskID, from, to, agentID, reason, metadataJSON)
	return err
}

func (s *EnhancedTaskService) recordDelegation(ctx context.Context, tx database.Transaction, delegationID, taskID uuid.UUID, fromAgent, toAgent, delegationType, reason string) error {
	query := `
		INSERT INTO task_delegation_history (id, task_id, from_agent_id, to_agent_id, delegation_type, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := tx.ExecContext(ctx, query, delegationID, taskID, fromAgent, toAgent, delegationType, reason)
	return err
}

func (s *EnhancedTaskService) acceptDelegation(ctx context.Context, tx database.Transaction, taskID uuid.UUID, agentID string) error {
	query := `
		UPDATE task_delegation_history 
		SET accepted_at = $1 
		WHERE task_id = $2 AND to_agent_id = $3 AND accepted_at IS NULL AND rejected_at IS NULL
		ORDER BY delegated_at DESC
		LIMIT 1
	`

	_, err := tx.ExecContext(ctx, query, time.Now(), taskID, agentID)
	return err
}

func (s *EnhancedTaskService) getTaskInTx(ctx context.Context, tx database.Transaction, taskID uuid.UUID) (*models.Task, error) {
	var task models.Task
	query := `SELECT * FROM tasks WHERE id = $1`
	if err := tx.Get(&task, query, taskID); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *EnhancedTaskService) createTaskInTx(ctx context.Context, tx database.Transaction, task *models.Task) error {
	query := `
		INSERT INTO tasks (
			id, tenant_id, type, title, description, status, priority, 
			assigned_to, delegated_from, delegation_count, max_delegations,
			auto_escalate, escalation_timeout, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	_, err := tx.ExecContext(ctx, query,
		task.ID, task.TenantID, task.Type, task.Title, task.Description,
		task.Status, task.Priority, task.AssignedTo, nil, // delegated_from will be tracked in history
		0, 5, false, // delegation_count, max_delegations, auto_escalate defaults
		nil, task.Parameters, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *EnhancedTaskService) updateTaskInTx(ctx context.Context, tx database.Transaction, task *models.Task) error {
	query := `
		UPDATE tasks SET
			status = $1, assigned_to = $2, delegated_from = $3, delegation_count = $4,
			result = $5, started_at = $6, completed_at = $7, accepted_at = $8,
			updated_at = $9
		WHERE id = $10
	`

	_, err := tx.ExecContext(ctx, query,
		task.Status, task.AssignedTo, nil, 0, // delegated_from and delegation_count tracked separately
		task.Result, task.StartedAt, task.CompletedAt, task.AssignedAt, // use AssignedAt instead of AcceptedAt
		task.UpdatedAt, task.ID,
	)
	return err
}

func (s *EnhancedTaskService) validateTask(task *models.Task) error {
	if task.Type == "" {
		return errors.New("task type is required")
	}
	if task.Title == "" {
		return errors.New("task title is required")
	}
	if task.TenantID == uuid.Nil {
		return errors.New("tenant ID is required")
	}
	if task.Priority == "" {
		task.Priority = models.TaskPriorityNormal
	}
	// Max delegations is handled at the database level with default value
	return nil
}

func (s *EnhancedTaskService) getDelegationCount(ctx context.Context, tx database.Transaction, taskID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM task_delegation_history WHERE task_id = $1`
	var count int
	if err := tx.Get(&count, query, taskID); err != nil {
		return 0, err
	}
	return count, nil
}
