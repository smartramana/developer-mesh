package database

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMockDB(t *testing.T) (*Database, sqlmock.Sqlmock) {
	// Create a new SQL mock
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)

	// Create a sqlx DB
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")

	// Create a database instance with the mock
	db := &Database{
		db:         sqlxDB,
		config:     Config{},
		statements: make(map[string]*sqlx.Stmt),
	}

	return db, mock
}

func TestNewDatabase(t *testing.T) {
	t.Run("Invalid Driver", func(t *testing.T) {
		ctx := context.Background()
		config := Config{
			Driver: "invalid-driver",
			DSN:    "invalid-dsn",
		}

		db, err := NewDatabase(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

func TestTransaction(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	t.Run("Successful Transaction", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE mcp\\.events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := db.Transaction(context.Background(), func(tx *sqlx.Tx) error {
			_, err := tx.Exec("UPDATE mcp.events SET processed = true WHERE id = 1")
			return err
		})
		assert.NoError(t, err)
	})

	t.Run("Transaction Error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE mcp\\.events").WillReturnError(errors.New("database error"))
		mock.ExpectRollback()

		err := db.Transaction(context.Background(), func(tx *sqlx.Tx) error {
			_, err := tx.Exec("UPDATE mcp.events SET processed = true WHERE id = 1")
			return err
		})
		assert.Error(t, err)
		assert.Equal(t, "database error", err.Error())
	})

	t.Run("Begin Transaction Error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(errors.New("begin transaction error"))

		err := db.Transaction(context.Background(), func(tx *sqlx.Tx) error {
			return nil
		})
		assert.Error(t, err)
		assert.Equal(t, "begin transaction error", err.Error())
	})

	t.Run("Transaction Panic", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectRollback()

		assert.Panics(t, func() {
			_ = db.Transaction(context.Background(), func(tx *sqlx.Tx) error {
				panic("transaction panic")
			})
		})
	})
}

func TestPrepareErrorHandling(t *testing.T) {
	// Create a new SQL mock
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	// Create a sqlx DB
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")

	// Set up with an intentionally mismatched statement
	mock.ExpectPrepare("NOT-MATCHING-SQL")

	// Create a database instance with the mock
	db := &Database{
		db:         sqlxDB,
		config:     Config{},
		statements: make(map[string]*sqlx.Stmt),
	}

	// This should fail with a regexp mismatch error
	err = db.prepareStatements(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not match")
}

func TestClose(t *testing.T) {
	db, mock := setupMockDB(t)

	// Expect close
	mock.ExpectClose()

	err := db.Close()
	assert.NoError(t, err)
}

func TestPing(t *testing.T) {
	// Create a custom mock for ping test
	mockDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")

	db := &Database{
		db:         sqlxDB,
		config:     Config{},
		statements: make(map[string]*sqlx.Stmt),
	}
	defer db.Close()

	t.Run("Successful Ping", func(t *testing.T) {
		mock.ExpectPing()

		err := db.Ping()
		assert.NoError(t, err)
	})

	t.Run("Ping Error", func(t *testing.T) {
		pingErr := errors.New("ping error")
		mock.ExpectPing().WillReturnError(pingErr)

		err := db.Ping()
		assert.Error(t, err)
		assert.Equal(t, "ping error", err.Error())
	})
}
