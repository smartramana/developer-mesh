package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// pgTransaction wraps sqlx.Tx with additional features
type pgTransaction struct {
	tx         *sqlx.Tx
	logger     observability.Logger
	startTime  time.Time
	savepoints []string
	closed     bool
}

// Execute runs a function within the transaction
func (t *pgTransaction) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if t.closed {
		return errors.New("transaction already closed")
	}

	return fn(ctx)
}

// Savepoint creates a savepoint for nested transactions
func (t *pgTransaction) Savepoint(ctx context.Context, name string) error {
	if t.closed {
		return errors.New("transaction already closed")
	}

	if name == "" {
		name = fmt.Sprintf("sp_%d", len(t.savepoints))
	}

	_, err := t.tx.ExecContext(ctx, "SAVEPOINT "+name)
	if err != nil {
		return errors.Wrap(err, "failed to create savepoint")
	}

	t.savepoints = append(t.savepoints, name)
	return nil
}

// RollbackToSavepoint rolls back to a specific savepoint
func (t *pgTransaction) RollbackToSavepoint(ctx context.Context, name string) error {
	if t.closed {
		return errors.New("transaction already closed")
	}

	_, err := t.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name)
	if err != nil {
		return errors.Wrap(err, "failed to rollback to savepoint")
	}

	// Remove savepoints after this one
	for i := len(t.savepoints) - 1; i >= 0; i-- {
		if t.savepoints[i] == name {
			t.savepoints = t.savepoints[:i+1]
			break
		}
	}

	return nil
}

// Commit commits the transaction with timing metrics
func (t *pgTransaction) Commit() error {
	if t.closed {
		return errors.New("transaction already closed")
	}

	duration := time.Since(t.startTime)
	err := t.tx.Commit()
	t.closed = true

	if err != nil {
		t.logger.Error("Transaction commit failed", map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"error":       err.Error(),
		})
		return errors.Wrap(err, "failed to commit transaction")
	}

	t.logger.Debug("Transaction committed", map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
		"savepoints":  len(t.savepoints),
	})

	return nil
}

// Rollback rolls back the transaction
func (t *pgTransaction) Rollback() error {
	if t.closed {
		return nil
	}

	err := t.tx.Rollback()
	t.closed = true

	if err != nil && err != sql.ErrTxDone {
		return errors.Wrap(err, "failed to rollback transaction")
	}

	return nil
}
