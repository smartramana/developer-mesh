package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// UnitOfWork represents a unit of work pattern for managing database transactions
type UnitOfWork interface {
	// BeginTx starts a new transaction with the given context and options
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Transaction, error)
	
	// Execute runs a function within a transaction, automatically handling commit/rollback
	Execute(ctx context.Context, fn func(tx Transaction) error) error
	
	// ExecuteWithOptions runs a function within a transaction with custom options
	ExecuteWithOptions(ctx context.Context, opts *sql.TxOptions, fn func(tx Transaction) error) error
}

// Transaction represents a database transaction with additional functionality
type Transaction interface {
	// Exec executes a query without returning any rows
	Exec(query string, args ...interface{}) (sql.Result, error)
	
	// ExecContext executes a query with context
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	
	// Query executes a query that returns rows
	Query(query string, args ...interface{}) (*sql.Rows, error)
	
	// QueryContext executes a query with context
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	
	// QueryRow executes a query that returns at most one row
	QueryRow(query string, args ...interface{}) *sql.Row
	
	// QueryRowContext executes a query with context that returns at most one row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	
	// Get executes a query and scans the result into dest (sqlx)
	Get(dest interface{}, query string, args ...interface{}) error
	
	// Select executes a query and scans the results into dest (sqlx)
	Select(dest interface{}, query string, args ...interface{}) error
	
	// NamedExec executes a named query (sqlx)
	NamedExec(query string, arg interface{}) (sql.Result, error)
	
	// Savepoint creates a savepoint with the given name
	Savepoint(name string) error
	
	// RollbackToSavepoint rolls back to the named savepoint
	RollbackToSavepoint(name string) error
	
	// ReleaseSavepoint releases the named savepoint
	ReleaseSavepoint(name string) error
	
	// Commit commits the transaction
	Commit() error
	
	// Rollback rolls back the transaction
	Rollback() error
	
	// ID returns the unique transaction ID for tracking
	ID() string
}

// unitOfWorkImpl implements the UnitOfWork interface
type unitOfWorkImpl struct {
	db      *sqlx.DB
	logger  observability.Logger
	metrics observability.MetricsClient
	
	// Transaction tracking
	activeTxns sync.Map
	txnCount   uint64
	mu         sync.Mutex
}

// NewUnitOfWork creates a new UnitOfWork instance
func NewUnitOfWork(db *sqlx.DB, logger observability.Logger, metrics observability.MetricsClient) UnitOfWork {
	return &unitOfWorkImpl{
		db:      db,
		logger:  logger,
		metrics: metrics,
	}
}

// BeginTx starts a new transaction
func (u *unitOfWorkImpl) BeginTx(ctx context.Context, opts *sql.TxOptions) (Transaction, error) {
	// Start observability span
	ctx, span := observability.StartSpan(ctx, "UnitOfWork.BeginTx")
	defer span.End()
	
	// Record metrics
	startTime := time.Now()
	defer func() {
		u.metrics.RecordHistogram("unit_of_work_begin_duration", time.Since(startTime).Seconds(), nil)
	}()
	
	// Set default options if not provided
	if opts == nil {
		opts = &sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
			ReadOnly:  false,
		}
	}
	
	// Begin transaction
	tx, err := u.db.BeginTxx(ctx, opts)
	if err != nil {
		u.metrics.IncrementCounter("unit_of_work_begin_errors", 1)
		return nil, errors.Wrap(err, "failed to begin transaction")
	}
	
	// Create transaction wrapper
	txn := newTransaction(tx, u.logger, u.metrics)
	
	// Track active transaction
	u.activeTxns.Store(txn.ID(), txn)
	u.mu.Lock()
	u.txnCount++
	u.mu.Unlock()
	
	u.metrics.IncrementCounter("unit_of_work_transactions_started", 1)
	u.metrics.RecordGauge("unit_of_work_active_transactions", float64(u.getActiveTransactionCount()), nil)
	
	u.logger.Debug("Transaction started", map[string]interface{}{
		"transaction_id": txn.ID(),
		"isolation":      opts.Isolation,
		"read_only":      opts.ReadOnly,
	})
	
	return txn, nil
}

// Execute runs a function within a transaction
func (u *unitOfWorkImpl) Execute(ctx context.Context, fn func(tx Transaction) error) error {
	return u.ExecuteWithOptions(ctx, nil, fn)
}

// ExecuteWithOptions runs a function within a transaction with custom options
func (u *unitOfWorkImpl) ExecuteWithOptions(ctx context.Context, opts *sql.TxOptions, fn func(tx Transaction) error) error {
	// Start observability span
	ctx, span := observability.StartSpan(ctx, "UnitOfWork.Execute")
	defer span.End()
	
	// Record metrics
	startTime := time.Now()
	success := false
	defer func() {
		duration := time.Since(startTime).Seconds()
		u.metrics.RecordHistogram("unit_of_work_execute_duration", duration, map[string]string{
			"success": fmt.Sprintf("%t", success),
		})
	}()
	
	// Begin transaction
	tx, err := u.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	
	// Ensure cleanup
	defer func() {
		if r := recover(); r != nil {
			// Rollback on panic
			if rbErr := tx.Rollback(); rbErr != nil {
				u.logger.Error("Failed to rollback transaction after panic", map[string]interface{}{
					"error":          rbErr.Error(),
					"panic":          r,
					"transaction_id": tx.ID(),
				})
			}
			panic(r) // Re-panic
		}
	}()
	
	// Execute function
	if err := fn(tx); err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			u.logger.Error("Failed to rollback transaction", map[string]interface{}{
				"error":          rbErr.Error(),
				"original_error": err.Error(),
				"transaction_id": tx.ID(),
			})
			return errors.Wrap(err, "transaction failed and rollback failed")
		}
		return errors.Wrap(err, "transaction failed")
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}
	
	success = true
	return nil
}

// getActiveTransactionCount returns the number of active transactions
func (u *unitOfWorkImpl) getActiveTransactionCount() int {
	count := 0
	u.activeTxns.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// transactionImpl implements the Transaction interface
type transactionImpl struct {
	tx      *sqlx.Tx
	id      string
	logger  observability.Logger
	metrics observability.MetricsClient
	
	// State tracking
	committed bool
	rolledBack bool
	mu        sync.Mutex
	
	// Savepoint tracking
	savepoints []string
}

// newTransaction creates a new transaction wrapper
func newTransaction(tx *sqlx.Tx, logger observability.Logger, metrics observability.MetricsClient) *transactionImpl {
	return &transactionImpl{
		tx:         tx,
		id:         generateTransactionID(),
		logger:     logger,
		metrics:    metrics,
		savepoints: make([]string, 0),
	}
}

// Implement all Transaction interface methods...

func (t *transactionImpl) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}

func (t *transactionImpl) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *transactionImpl) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.Query(query, args...)
}

func (t *transactionImpl) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *transactionImpl) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRow(query, args...)
}

func (t *transactionImpl) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *transactionImpl) Get(dest interface{}, query string, args ...interface{}) error {
	return t.tx.Get(dest, query, args...)
}

func (t *transactionImpl) Select(dest interface{}, query string, args ...interface{}) error {
	return t.tx.Select(dest, query, args...)
}

func (t *transactionImpl) NamedExec(query string, arg interface{}) (sql.Result, error) {
	return t.tx.NamedExec(query, arg)
}

// Savepoint creates a savepoint
func (t *transactionImpl) Savepoint(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.committed || t.rolledBack {
		return errors.New("transaction already completed")
	}
	
	query := fmt.Sprintf("SAVEPOINT %s", name)
	if _, err := t.tx.Exec(query); err != nil {
		return errors.Wrapf(err, "failed to create savepoint %s", name)
	}
	
	t.savepoints = append(t.savepoints, name)
	t.logger.Debug("Savepoint created", map[string]interface{}{
		"transaction_id": t.id,
		"savepoint":      name,
	})
	
	return nil
}

// RollbackToSavepoint rolls back to a savepoint
func (t *transactionImpl) RollbackToSavepoint(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.committed || t.rolledBack {
		return errors.New("transaction already completed")
	}
	
	query := fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", name)
	if _, err := t.tx.Exec(query); err != nil {
		return errors.Wrapf(err, "failed to rollback to savepoint %s", name)
	}
	
	t.logger.Debug("Rolled back to savepoint", map[string]interface{}{
		"transaction_id": t.id,
		"savepoint":      name,
	})
	
	return nil
}

// ReleaseSavepoint releases a savepoint
func (t *transactionImpl) ReleaseSavepoint(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.committed || t.rolledBack {
		return errors.New("transaction already completed")
	}
	
	query := fmt.Sprintf("RELEASE SAVEPOINT %s", name)
	if _, err := t.tx.Exec(query); err != nil {
		return errors.Wrapf(err, "failed to release savepoint %s", name)
	}
	
	// Remove from tracked savepoints
	newSavepoints := make([]string, 0, len(t.savepoints))
	for _, sp := range t.savepoints {
		if sp != name {
			newSavepoints = append(newSavepoints, sp)
		}
	}
	t.savepoints = newSavepoints
	
	t.logger.Debug("Savepoint released", map[string]interface{}{
		"transaction_id": t.id,
		"savepoint":      name,
	})
	
	return nil
}

// Commit commits the transaction
func (t *transactionImpl) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.committed {
		return errors.New("transaction already committed")
	}
	if t.rolledBack {
		return errors.New("transaction already rolled back")
	}
	
	startTime := time.Now()
	err := t.tx.Commit()
	duration := time.Since(startTime).Seconds()
	
	if err != nil {
		t.metrics.IncrementCounter("transaction_commit_errors", 1)
		t.metrics.RecordHistogram("transaction_commit_duration", duration, map[string]string{
			"success": "false",
		})
		return errors.Wrap(err, "failed to commit transaction")
	}
	
	t.committed = true
	t.metrics.IncrementCounter("transaction_commits", 1)
	t.metrics.RecordHistogram("transaction_commit_duration", duration, map[string]string{
		"success": "true",
	})
	
	t.logger.Debug("Transaction committed", map[string]interface{}{
		"transaction_id": t.id,
		"duration_ms":    duration * 1000,
	})
	
	return nil
}

// Rollback rolls back the transaction
func (t *transactionImpl) Rollback() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.committed {
		return errors.New("transaction already committed")
	}
	if t.rolledBack {
		return nil // Already rolled back
	}
	
	startTime := time.Now()
	err := t.tx.Rollback()
	duration := time.Since(startTime).Seconds()
	
	if err != nil && err != sql.ErrTxDone {
		t.metrics.IncrementCounter("transaction_rollback_errors", 1)
		t.metrics.RecordHistogram("transaction_rollback_duration", duration, map[string]string{
			"success": "false",
		})
		return errors.Wrap(err, "failed to rollback transaction")
	}
	
	t.rolledBack = true
	t.metrics.IncrementCounter("transaction_rollbacks", 1)
	t.metrics.RecordHistogram("transaction_rollback_duration", duration, map[string]string{
		"success": "true",
	})
	
	t.logger.Debug("Transaction rolled back", map[string]interface{}{
		"transaction_id": t.id,
		"duration_ms":    duration * 1000,
	})
	
	return nil
}

// ID returns the transaction ID
func (t *transactionImpl) ID() string {
	return t.id
}

// generateTransactionID generates a unique transaction ID
func generateTransactionID() string {
	return fmt.Sprintf("txn_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
}