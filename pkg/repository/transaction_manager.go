package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/pkg/errors"
)

// TransactionManager manages database transactions across repositories
type TransactionManager interface {
	// WithTransaction executes a function within a transaction
	WithTransaction(ctx context.Context, fn func(ctx context.Context, tx database.Transaction) error) error

	// WithTransactionOptions executes a function within a transaction with options
	WithTransactionOptions(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context, tx database.Transaction) error) error

	// WithNestedTransaction executes a function within a nested transaction (savepoint)
	WithNestedTransaction(ctx context.Context, parentTx database.Transaction, name string, fn func(ctx context.Context) error) error

	// GetCurrentTransaction retrieves the current transaction from context
	GetCurrentTransaction(ctx context.Context) (database.Transaction, bool)
}

// TransactionKey is the context key for storing transactions
type transactionKey struct{}

// transactionManagerImpl implements TransactionManager
type transactionManagerImpl struct {
	uow     database.UnitOfWork
	logger  observability.Logger
	metrics observability.MetricsClient

	// Transaction tracking
	activeTransactions sync.Map
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(uow database.UnitOfWork, logger observability.Logger, metrics observability.MetricsClient) TransactionManager {
	return &transactionManagerImpl{
		uow:     uow,
		logger:  logger,
		metrics: metrics,
	}
}

// WithTransaction executes a function within a transaction
func (tm *transactionManagerImpl) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx database.Transaction) error) error {
	return tm.WithTransactionOptions(ctx, nil, fn)
}

// WithTransactionOptions executes a function within a transaction with options
func (tm *transactionManagerImpl) WithTransactionOptions(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context, tx database.Transaction) error) error {
	// Start observability span
	ctx, span := observability.StartSpan(ctx, "TransactionManager.WithTransaction")
	defer span.End()

	// Check if we're already in a transaction
	if existingTx, ok := tm.GetCurrentTransaction(ctx); ok {
		tm.logger.Debug("Using existing transaction", map[string]interface{}{
			"transaction_id": existingTx.ID(),
		})
		// Use existing transaction
		return fn(ctx, existingTx)
	}

	// Record metrics
	startTime := time.Now()
	success := false
	defer func() {
		duration := time.Since(startTime).Seconds()
		tm.metrics.RecordHistogram("transaction_manager_duration", duration, map[string]string{
			"success": fmt.Sprintf("%t", success),
		})
	}()

	// Execute within new transaction
	err := tm.uow.ExecuteWithOptions(ctx, opts, func(tx database.Transaction) error {
		// Store transaction in context
		txCtx := context.WithValue(ctx, transactionKey{}, tx)

		// Track active transaction
		tm.activeTransactions.Store(tx.ID(), &transactionInfo{
			ID:        tx.ID(),
			StartTime: time.Now(),
			Context:   ctx,
		})
		defer tm.activeTransactions.Delete(tx.ID())

		// Execute function
		return fn(txCtx, tx)
	})

	if err == nil {
		success = true
	}

	return err
}

// WithNestedTransaction executes a function within a nested transaction using savepoints
func (tm *transactionManagerImpl) WithNestedTransaction(ctx context.Context, parentTx database.Transaction, name string, fn func(ctx context.Context) error) error {
	// Start observability span
	ctx, span := observability.StartSpan(ctx, "TransactionManager.WithNestedTransaction")
	defer span.End()
	// Add span attributes if supported by the observability package
	// span.SetAttributes(
	//     observability.String("savepoint_name", name),
	//     observability.String("parent_transaction_id", parentTx.ID()),
	// )

	// Validate parent transaction
	if parentTx == nil {
		return errors.New("parent transaction is required for nested transaction")
	}

	// Create savepoint
	if err := parentTx.Savepoint(name); err != nil {
		tm.metrics.IncrementCounter("nested_transaction_savepoint_errors", 1)
		return errors.Wrapf(err, "failed to create savepoint %s", name)
	}

	tm.logger.Debug("Nested transaction started", map[string]interface{}{
		"savepoint":      name,
		"transaction_id": parentTx.ID(),
	})

	// Execute function
	err := fn(ctx)

	if err != nil {
		// Rollback to savepoint on error
		if rbErr := parentTx.RollbackToSavepoint(name); rbErr != nil {
			tm.logger.Error("Failed to rollback to savepoint", map[string]interface{}{
				"savepoint":      name,
				"transaction_id": parentTx.ID(),
				"error":          rbErr.Error(),
				"original_error": err.Error(),
			})
			tm.metrics.IncrementCounter("nested_transaction_rollback_errors", 1)
			return errors.Wrapf(err, "operation failed and rollback to savepoint %s failed", name)
		}

		tm.metrics.IncrementCounter("nested_transaction_rollbacks", 1)
		tm.logger.Debug("Rolled back to savepoint", map[string]interface{}{
			"savepoint":      name,
			"transaction_id": parentTx.ID(),
		})

		return err
	}

	// Release savepoint on success
	if err := parentTx.ReleaseSavepoint(name); err != nil {
		tm.logger.Warn("Failed to release savepoint", map[string]interface{}{
			"savepoint":      name,
			"transaction_id": parentTx.ID(),
			"error":          err.Error(),
		})
		// Non-fatal error - the transaction can still continue
	}

	tm.metrics.IncrementCounter("nested_transaction_commits", 1)
	return nil
}

// GetCurrentTransaction retrieves the current transaction from context
func (tm *transactionManagerImpl) GetCurrentTransaction(ctx context.Context) (database.Transaction, bool) {
	tx, ok := ctx.Value(transactionKey{}).(database.Transaction)
	return tx, ok
}

// transactionInfo holds information about an active transaction
type transactionInfo struct {
	ID        string
	StartTime time.Time
	Context   context.Context
}

// TransactionalRepository is a base interface for repositories that support transactions
type TransactionalRepository interface {
	// WithTx returns a new instance of the repository that uses the given transaction
	WithTx(tx database.Transaction) interface{}
}

// RepositoryFactory creates repository instances with transaction support
type RepositoryFactory interface {
	// CreateTaskRepository creates a task repository
	CreateTaskRepository(tx database.Transaction) interface{}

	// CreateWorkflowRepository creates a workflow repository
	CreateWorkflowRepository(tx database.Transaction) interface{}

	// CreateWorkspaceRepository creates a workspace repository
	CreateWorkspaceRepository(tx database.Transaction) interface{}

	// CreateDocumentRepository creates a document repository
	CreateDocumentRepository(tx database.Transaction) interface{}

	// CreateAgentRepository creates an agent repository
	CreateAgentRepository(tx database.Transaction) interface{}
}

// CompensationFunc is a function that can be used to compensate for a failed operation
type CompensationFunc func(ctx context.Context) error

// CompensationManager manages compensation logic for failed transactions
type CompensationManager interface {
	// RegisterCompensation registers a compensation function
	RegisterCompensation(name string, fn CompensationFunc)

	// ExecuteWithCompensation executes an operation with compensation
	ExecuteWithCompensation(ctx context.Context, operation func(ctx context.Context) error, compensations ...string) error
}

// compensationManagerImpl implements CompensationManager
type compensationManagerImpl struct {
	compensations map[string]CompensationFunc
	logger        observability.Logger
	metrics       observability.MetricsClient
	mu            sync.RWMutex
}

// NewCompensationManager creates a new compensation manager
func NewCompensationManager(logger observability.Logger, metrics observability.MetricsClient) CompensationManager {
	return &compensationManagerImpl{
		compensations: make(map[string]CompensationFunc),
		logger:        logger,
		metrics:       metrics,
	}
}

// RegisterCompensation registers a compensation function
func (cm *compensationManagerImpl) RegisterCompensation(name string, fn CompensationFunc) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.compensations[name] = fn
	cm.logger.Debug("Compensation registered", map[string]interface{}{
		"name": name,
	})
}

// ExecuteWithCompensation executes an operation with compensation
func (cm *compensationManagerImpl) ExecuteWithCompensation(ctx context.Context, operation func(ctx context.Context) error, compensations ...string) error {
	// Execute the operation
	err := operation(ctx)
	if err == nil {
		return nil // Success, no compensation needed
	}

	// Operation failed, execute compensations
	cm.logger.Warn("Operation failed, executing compensations", map[string]interface{}{
		"error":         err.Error(),
		"compensations": compensations,
	})

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var compensationErrors []error
	for _, name := range compensations {
		if fn, ok := cm.compensations[name]; ok {
			cm.logger.Debug("Executing compensation", map[string]interface{}{
				"name": name,
			})

			if compErr := fn(ctx); compErr != nil {
				cm.logger.Error("Compensation failed", map[string]interface{}{
					"name":  name,
					"error": compErr.Error(),
				})
				compensationErrors = append(compensationErrors, compErr)
				cm.metrics.IncrementCounter("compensation_errors", 1)
			} else {
				cm.metrics.IncrementCounter("compensation_success", 1)
			}
		} else {
			cm.logger.Warn("Compensation not found", map[string]interface{}{
				"name": name,
			})
		}
	}

	if len(compensationErrors) > 0 {
		return errors.Wrapf(err, "operation failed and %d compensations also failed", len(compensationErrors))
	}

	return errors.Wrap(err, "operation failed but compensations succeeded")
}
