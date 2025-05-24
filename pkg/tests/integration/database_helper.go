package integration

import (
	"context"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

// DatabaseHelper provides utilities for database integration tests
type DatabaseHelper struct {
	t        *testing.T
	db       *sqlx.DB
	testHelper *TestHelper
}

// NewDatabaseHelper creates a new database helper
func NewDatabaseHelper(t *testing.T) *DatabaseHelper {
	testHelper := NewTestHelper(t)
	return &DatabaseHelper{
		t:        t,
		testHelper: testHelper,
	}
}

// SetupTestDatabase connects to the test database
func (h *DatabaseHelper) SetupTestDatabase(ctx context.Context, config database.Config) *sqlx.DB {
	// Initialize database connection
	dbInstance, err := database.NewDatabase(ctx, config)
	require.NoError(h.t, err, "Failed to connect to test database")
	require.NotNil(h.t, dbInstance, "Database instance should not be nil")
	
	// Get the underlying sqlx.DB
	db := dbInstance.GetDB()
	require.NotNil(h.t, db, "Database connection should not be nil")
	h.db = db
	return db
}

// SetupTestDatabaseWithConnection sets up the database helper with an existing connection
func (h *DatabaseHelper) SetupTestDatabaseWithConnection(ctx context.Context, db *sqlx.DB) {
	require.NotNil(h.t, db, "Database connection should not be nil")
	h.db = db
}

// CreateTransaction creates a new transaction for testing
func (h *DatabaseHelper) CreateTransaction(ctx context.Context) (*sqlx.Tx, context.Context) {
	require.NotNil(h.t, h.db, "Database must be initialized before creating transaction")
	
	// Start a new transaction
	tx, err := h.db.BeginTxx(ctx, nil)
	require.NoError(h.t, err, "Failed to begin transaction")
	
	// Create a new context with the transaction
	txCtx := context.WithValue(ctx, TxKey("tx"), tx)
	return tx, txCtx
}

// Rollback safely rolls back a transaction, ignoring if already committed
func (h *DatabaseHelper) Rollback(tx *sqlx.Tx) {
	if tx != nil {
		_ = tx.Rollback() // Ignore error if already committed
	}
}

// Commit commits a transaction
func (h *DatabaseHelper) Commit(tx *sqlx.Tx) {
	require.NotNil(h.t, tx, "Transaction cannot be nil")
	err := tx.Commit()
	require.NoError(h.t, err, "Failed to commit transaction")
}

// CleanupDatabase closes the database connection
func (h *DatabaseHelper) CleanupDatabase() {
	if h.db != nil {
		h.db.Close()
	}
}

// GetTransactionFromContext extracts a transaction from context
func GetTransactionFromContext(ctx context.Context) *sqlx.Tx {
	tx, ok := ctx.Value(TxKey("tx")).(*sqlx.Tx)
	if !ok {
		return nil
	}
	return tx
}

// TxKey defines a custom type for transaction context keys to avoid collisions
type TxKey string
