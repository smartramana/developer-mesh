package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// TransactionalWorkflowService extends the workflow service with proper transaction management
type TransactionalWorkflowService struct {
	*workflowService
	uow             database.UnitOfWork
	txManager       repository.TransactionManager
	compensationMgr repository.CompensationManager
}

// NewTransactionalWorkflowService creates a workflow service with transaction support
func NewTransactionalWorkflowService(
	baseService *workflowService,
	uow database.UnitOfWork,
	txManager repository.TransactionManager,
	compensationMgr repository.CompensationManager,
) WorkflowService {
	return &TransactionalWorkflowService{
		workflowService: baseService,
		uow:             uow,
		txManager:       txManager,
		compensationMgr: compensationMgr,
	}
}

// ExecuteWorkflow executes a workflow with full transaction support
func (s *TransactionalWorkflowService) ExecuteWorkflow(ctx context.Context, workflowID uuid.UUID, input map[string]interface{}, idempotencyKey string) (*models.WorkflowExecution, error) {
	ctx, span := s.config.Tracer(ctx, "TransactionalWorkflowService.ExecuteWorkflow")
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

	var execution *models.WorkflowExecution

	// Execute within transaction with proper isolation
	err := s.txManager.WithTransactionOptions(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	}, func(ctx context.Context, tx database.Transaction) error {
		// Get workflow within transaction
		workflow, err := s.getWorkflowInTx(ctx, tx, workflowID)
		if err != nil {
			return errors.Wrap(err, "workflow not found")
		}

		// Validate workflow is active
		if !workflow.IsActive {
			return WorkflowNotActiveError{WorkflowID: workflowID}
		}

		// Validate input
		if err := s.validateWorkflowInput(ctx, workflow, input); err != nil {
			return errors.Wrap(err, "input validation failed")
		}

		// Check authorization
		if err := s.authorizeWorkflowExecution(ctx, workflow); err != nil {
			return errors.Wrap(err, "authorization failed")
		}

		// Create execution
		execution = &models.WorkflowExecution{
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

		// Create execution record using transaction
		if err := s.createExecutionInTx(ctx, tx, execution); err != nil {
			return errors.Wrap(err, "failed to create execution")
		}

		// Store idempotency key
		if idempotencyKey != "" {
			if err := s.storeExecutionIdempotencyKeyInTx(ctx, tx, idempotencyKey, execution.ID); err != nil {
				return err
			}
		}

		// Update workflow metadata with last execution time
		if workflow.Config == nil {
			workflow.Config = make(models.JSONMap)
		}
		workflow.Config["last_executed_at"] = execution.StartedAt
		if err := s.updateWorkflowInTx(ctx, tx, workflow); err != nil {
			return err
		}

		// Track active execution
		s.activeExecutions.Store(execution.ID, execution)

		// Send notification
		if s.notifier != nil {
			_ = s.notifier.NotifyWorkflowStarted(ctx, map[string]interface{}{
				"workflow_id":   workflowID,
				"workflow_name": workflow.Name,
				"execution_id":  execution.ID,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Start asynchronous execution with compensation support
	go s.executeWorkflowAsync(context.Background(), execution)

	return execution, nil
}

// ExecuteWorkflowStep executes a single workflow step with transaction and savepoint support
func (s *TransactionalWorkflowService) ExecuteWorkflowStep(ctx context.Context, executionID uuid.UUID, stepID string) error {
	ctx, span := s.config.Tracer(ctx, "TransactionalWorkflowService.ExecuteWorkflowStep")
	defer span.End()

	// Get or create transaction
	var tx database.Transaction
	existingTx, hasTx := s.txManager.GetCurrentTransaction(ctx)

	if hasTx {
		tx = existingTx
	} else {
		// Create new transaction if not in one
		return s.txManager.WithTransaction(ctx, func(ctx context.Context, newTx database.Transaction) error {
			tx = newTx
			return s.executeStepInTransaction(ctx, tx, executionID, stepID)
		})
	}

	// Execute with savepoint for nested transaction support
	savepointName := fmt.Sprintf("step_%s", stepID)
	return s.txManager.WithNestedTransaction(ctx, tx, savepointName, func(ctx context.Context) error {
		return s.executeStepInTransaction(ctx, tx, executionID, stepID)
	})
}

// executeStepInTransaction executes a step within a transaction
func (s *TransactionalWorkflowService) executeStepInTransaction(ctx context.Context, tx database.Transaction, executionID uuid.UUID, stepID string) error {
	// Get execution
	execution, err := s.getExecutionInTx(ctx, tx, executionID)
	if err != nil {
		return errors.Wrap(err, "execution not found")
	}

	// Get workflow
	workflow, err := s.getWorkflowInTx(ctx, tx, execution.WorkflowID)
	if err != nil {
		return errors.Wrap(err, "workflow not found")
	}

	// Find step
	step, err := s.findStep(workflow, stepID)
	if err != nil {
		return err
	}

	// Validate step can be executed
	if err := s.validateStepExecution(execution, step); err != nil {
		return err
	}

	// Update step status to running
	stepStatus := execution.StepStatuses[stepID]
	stepStatus.Status = models.StepStatusRunning
	now := time.Now()
	stepStatus.StartedAt = &now

	// Save execution state
	if err := s.updateExecutionInTx(ctx, tx, execution); err != nil {
		return errors.Wrap(err, "failed to update execution status")
	}

	// Register compensation for this step
	s.compensationMgr.RegisterCompensation(
		fmt.Sprintf("compensate_step_%s", stepID),
		func(compensateCtx context.Context) error {
			return s.compensateStep(compensateCtx, executionID, stepID)
		},
	)

	// Execute step action
	result, err := s.executeStepAction(ctx, tx, execution, step)
	if err != nil {
		// Update step status to failed
		stepStatus.Status = models.StepStatusFailed
		stepStatus.Error = err.Error()
		completedAt := time.Now()
		stepStatus.CompletedAt = &completedAt

		// Save failure state
		if updateErr := s.updateExecutionInTx(ctx, tx, execution); updateErr != nil {
			s.config.Logger.Error("Failed to update step failure status", map[string]interface{}{
				"error":     updateErr.Error(),
				"step_id":   stepID,
				"execution": executionID,
			})
		}

		return errors.Wrapf(err, "step '%s' execution failed", step.Name)
	}

	// Update step status to completed
	stepStatus.Status = models.StepStatusCompleted
	// Store result in output field
	if result != nil {
		if resultMap, ok := result.(map[string]interface{}); ok {
			stepStatus.Output = resultMap
		} else {
			stepStatus.Output = map[string]interface{}{
				"result": result,
			}
		}
	}
	completedAt := time.Now()
	stepStatus.CompletedAt = &completedAt

	// Store step result in execution context
	execution.Context[fmt.Sprintf("step_%s_result", stepID)] = result

	// Save execution state
	if err := s.updateExecutionInTx(ctx, tx, execution); err != nil {
		return errors.Wrap(err, "failed to update execution after step completion")
	}

	s.config.Logger.Info("Workflow step completed", map[string]interface{}{
		"execution_id": executionID,
		"step_id":      stepID,
		"status":       stepStatus.Status,
	})

	return nil
}

// executeWorkflowAsync executes workflow steps asynchronously with compensation
func (s *TransactionalWorkflowService) executeWorkflowAsync(ctx context.Context, execution *models.WorkflowExecution) {
	// Create compensation list for rollback
	compensations := make([]string, 0)

	// Execute workflow with compensation support
	err := s.compensationMgr.ExecuteWithCompensation(ctx, func(ctx context.Context) error {
		// Get workflow
		workflow, err := s.getWorkflow(ctx, execution.WorkflowID)
		if err != nil {
			return err
		}

		// Execute each step
		for _, step := range workflow.GetSteps() {
			// Check if execution was cancelled
			if s.isExecutionCancelled(execution.ID) {
				return errors.New("execution cancelled")
			}

			// Execute step within transaction
			if err := s.ExecuteWorkflowStep(ctx, execution.ID, step.ID); err != nil {
				return errors.Wrapf(err, "failed to execute step %s", step.ID)
			}

			// Add compensation for this step
			compensations = append(compensations, fmt.Sprintf("compensate_step_%s", step.ID))
		}

		// Update execution status to completed
		return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
			execution.Status = models.WorkflowStatusCompleted
			execution.CompletedAt = &time.Time{}
			*execution.CompletedAt = time.Now()
			return s.updateExecutionInTx(ctx, tx, execution)
		})
	}, compensations...)

	if err != nil {
		// Update execution status to failed
		_ = s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
			execution.Status = models.WorkflowStatusFailed
			execution.Error = err.Error()
			return s.updateExecutionInTx(ctx, tx, execution)
		})
	}

	// Remove from active executions
	s.activeExecutions.Delete(execution.ID)

	// Send completion notification
	if s.notifier != nil {
		if err != nil {
			_ = s.notifier.NotifyWorkflowFailed(context.Background(), execution.WorkflowID, fmt.Sprintf("Execution failed: %v", err))
		} else {
			_ = s.notifier.NotifyWorkflowCompleted(context.Background(), map[string]interface{}{
				"workflow_id":  execution.WorkflowID,
				"execution_id": execution.ID,
				"status":       execution.Status,
				"completed_at": execution.CompletedAt,
			})
		}
	}
}

// compensateStep compensates for a failed step
func (s *TransactionalWorkflowService) compensateStep(ctx context.Context, executionID uuid.UUID, stepID string) error {
	return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
		execution, err := s.getExecutionInTx(ctx, tx, executionID)
		if err != nil {
			return err
		}

		// Update step status to compensated (or rolled back)
		if stepStatus, ok := execution.StepStatuses[stepID]; ok {
			stepStatus.Status = models.StepStatusFailed
			// Mark as compensated in output
			if stepStatus.Output == nil {
				stepStatus.Output = make(map[string]interface{})
			}
			stepStatus.Output["compensated"] = true
			return s.updateExecutionInTx(ctx, tx, execution)
		}

		return nil
	})
}

// Transaction-aware helper methods

func (s *TransactionalWorkflowService) getWorkflowInTx(ctx context.Context, tx database.Transaction, workflowID uuid.UUID) (*models.Workflow, error) {
	// Implementation would use the transaction to query the workflow
	// This is a placeholder - actual implementation depends on repository structure
	var workflow models.Workflow
	query := `SELECT * FROM workflows WHERE id = $1`
	if err := tx.Get(&workflow, query, workflowID); err != nil {
		return nil, err
	}
	return &workflow, nil
}

func (s *TransactionalWorkflowService) createExecutionInTx(ctx context.Context, tx database.Transaction, execution *models.WorkflowExecution) error {
	query := `
		INSERT INTO workflow_executions (id, workflow_id, tenant_id, initiated_by, status, context, state, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := tx.ExecContext(ctx, query,
		execution.ID,
		execution.WorkflowID,
		execution.TenantID,
		execution.InitiatedBy,
		execution.Status,
		execution.Context,
		execution.State,
		execution.StartedAt,
	)
	return err
}

func (s *TransactionalWorkflowService) updateWorkflowInTx(ctx context.Context, tx database.Transaction, workflow *models.Workflow) error {
	query := `
		UPDATE workflows 
		SET config = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := tx.ExecContext(ctx, query, workflow.Config, time.Now(), workflow.ID)
	return err
}

func (s *TransactionalWorkflowService) getExecutionInTx(ctx context.Context, tx database.Transaction, executionID uuid.UUID) (*models.WorkflowExecution, error) {
	var execution models.WorkflowExecution
	query := `SELECT * FROM workflow_executions WHERE id = $1`
	if err := tx.Get(&execution, query, executionID); err != nil {
		return nil, err
	}
	return &execution, nil
}

func (s *TransactionalWorkflowService) updateExecutionInTx(ctx context.Context, tx database.Transaction, execution *models.WorkflowExecution) error {
	query := `
		UPDATE workflow_executions 
		SET status = $1, context = $2, state = $3, 
		    completed_at = $4, error = $5, updated_at = $6
		WHERE id = $7
	`
	_, err := tx.ExecContext(ctx, query,
		execution.Status,
		execution.Context,
		execution.State,
		execution.CompletedAt,
		execution.Error,
		time.Now(),
		execution.ID,
	)
	return err
}

func (s *TransactionalWorkflowService) storeExecutionIdempotencyKeyInTx(ctx context.Context, tx database.Transaction, key string, executionID uuid.UUID) error {
	query := `
		INSERT INTO workflow_execution_idempotency (idempotency_key, execution_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (idempotency_key) DO NOTHING
	`
	_, err := tx.ExecContext(ctx, query, key, executionID, time.Now())
	return err
}

func (s *TransactionalWorkflowService) executeStepAction(ctx context.Context, tx database.Transaction, execution *models.WorkflowExecution, step *models.WorkflowStep) (interface{}, error) {
	// This would execute the actual step logic
	// For now, return a placeholder
	return map[string]interface{}{
		"status": "completed",
		"output": fmt.Sprintf("Step %s executed successfully", step.Name),
	}, nil
}

func (s *TransactionalWorkflowService) findStep(workflow *models.Workflow, stepID string) (*models.WorkflowStep, error) {
	for _, step := range workflow.GetSteps() {
		if step.ID == stepID {
			return &step, nil
		}
	}
	return nil, fmt.Errorf("step '%s' not found in workflow", stepID)
}

func (s *TransactionalWorkflowService) validateStepExecution(execution *models.WorkflowExecution, step *models.WorkflowStep) error {
	stepStatus, ok := execution.StepStatuses[step.ID]
	if !ok {
		return fmt.Errorf("step '%s' not found in execution", step.ID)
	}

	if stepStatus.Status != models.StepStatusPending {
		return fmt.Errorf("step '%s' is not in pending state (current: %s)", step.ID, stepStatus.Status)
	}

	return nil
}

func (s *TransactionalWorkflowService) isExecutionCancelled(executionID uuid.UUID) bool {
	// Check if execution has been cancelled
	// This could check a cancellation flag in the database or a context cancellation
	return false
}
