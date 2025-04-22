package migration

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	// Create a mock DB
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Test with valid configuration
	config := Config{
		MigrationsPath:   "test/migrations",
		AutoMigrate:      true,
		MigrationTimeout: 30 * time.Second,
	}
	
	manager, err := NewManager(db, config, "postgres")
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, config, manager.config)
	
	// Test with nil DB
	_, err = NewManager(nil, config, "postgres")
	assert.Error(t, err)
	
	// Test with empty migrations path (should use default)
	emptyConfig := Config{
		AutoMigrate:      true,
		MigrationTimeout: 30 * time.Second,
	}
	
	manager, err = NewManager(db, emptyConfig, "postgres")
	require.NoError(t, err)
	assert.Equal(t, "migrations/sql", manager.config.MigrationsPath)
	
	// Test with zero migration timeout (should use default)
	zeroTimeoutConfig := Config{
		MigrationsPath: "test/migrations",
		AutoMigrate:    true,
	}
	
	manager, err = NewManager(db, zeroTimeoutConfig, "postgres")
	require.NoError(t, err)
	assert.Equal(t, 1*time.Minute, manager.config.MigrationTimeout)
}

func TestRunMigrationsWithTimeout(t *testing.T) {
	// Create a mock DB
	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create manager with very short timeout to test timeout handling
	config := Config{
		MigrationsPath:   "nonexistent/path", // This will cause the migration to hang
		AutoMigrate:      true,
		MigrationTimeout: 50 * time.Millisecond, // Very short timeout
	}
	
	manager, err := NewManager(db, config, "postgres")
	require.NoError(t, err)
	
	// This should timeout
	ctx := context.Background()
	err = manager.RunMigrations(ctx)
	
	// We expect an error that contains "timeout"
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestWithTransaction(t *testing.T) {
	// Create a mock DB
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create manager
	config := Config{
		MigrationsPath:   "test/migrations",
		AutoMigrate:      true,
		MigrationTimeout: 30 * time.Second,
	}
	
	manager, err := NewManager(db, config, "postgres")
	require.NoError(t, err)
	
	// Test successful transaction
	mock.ExpectBegin()
	mock.ExpectCommit()
	
	err = manager.WithTransaction(context.Background(), func(tx *sqlmock.Tx) error {
		return nil
	})
	assert.NoError(t, err)
	
	// Test transaction with error
	mock.ExpectBegin()
	mock.ExpectRollback()
	
	err = manager.WithTransaction(context.Background(), func(tx *sqlmock.Tx) error {
		return assert.AnError
	})
	assert.Error(t, err)
	
	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}
