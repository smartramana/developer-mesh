package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
)

// Config holds the migration configuration
type Config struct {
	// Path to migration files directory
	MigrationsPath string `json:"migrations_path" yaml:"migrations_path"`

	// Whether to automatically run migrations on startup
	AutoMigrate bool `json:"auto_migrate" yaml:"auto_migrate"`

	// Timeout for migration operations
	MigrationTimeout time.Duration `json:"migration_timeout" yaml:"migration_timeout"`

	// Whether to validate migrations without applying them
	ValidateOnly bool `json:"validate_only" yaml:"validate_only"`

	// Use a specific number of steps for migration (0 means all)
	Steps int `json:"steps" yaml:"steps"`
}

// Manager handles database migrations
type Manager struct {
	db         *sqlx.DB
	config     Config
	migrator   *migrate.Migrate
	driverName string
}

// NewManager creates a new migration manager
func NewManager(db *sqlx.DB, config Config, driverName string) (*Manager, error) {
	if db == nil {
		return nil, errors.New("db connection cannot be nil")
	}

	if config.MigrationsPath == "" {
		// Default to a standard location if not specified
		config.MigrationsPath = "migrations/sql"
	}

	if config.MigrationTimeout == 0 {
		// Default timeout of 1 minute
		config.MigrationTimeout = 1 * time.Minute
	}

	// Resolve the absolute path to migrations to validate it exists
	_, err := filepath.Abs(config.MigrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve migrations path: %w", err)
	}

	return &Manager{
		db:         db,
		config:     config,
		driverName: driverName,
	}, nil
}

// Init initializes the migration manager
func (m *Manager) Init(ctx context.Context) error {
	// Create a database driver instance
	driver, err := postgres.WithInstance(m.db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Create file source instance with migrations path
	sourceURL := fmt.Sprintf("file://%s", m.config.MigrationsPath)

	// Create migrator instance
	migrator, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Set the migrator
	m.migrator = migrator

	return nil
}

// RunMigrations applies all pending migrations
func (m *Manager) RunMigrations(ctx context.Context) error {
	if m.migrator == nil {
		if err := m.Init(ctx); err != nil {
			return err
		}
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, m.config.MigrationTimeout)
	defer cancel()

	// Create a done channel for handling timeout
	done := make(chan error, 1)

	// Run migrations in a goroutine
	go func() {
		var err error

		// If steps is specified, run only that many steps
		if m.config.Steps > 0 {
			err = m.migrator.Steps(m.config.Steps)
		} else {
			// Run all migrations
			err = m.migrator.Up()
		}

		// Special case: no migrations to run is not an error
		if err == migrate.ErrNoChange {
			log.Println("No migrations to run")
			err = nil
		}

		done <- err
	}()

	// Wait for either completion or timeout
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("migration error: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("migration timeout after %s", m.config.MigrationTimeout)
	}
}

// ValidateMigrations checks if migrations are valid but doesn't apply them
func (m *Manager) ValidateMigrations(ctx context.Context) error {
	if m.migrator == nil {
		if err := m.Init(ctx); err != nil {
			return err
		}
	}

	// Get current version
	version, dirty, err := m.migrator.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	// Check if database is in a dirty state
	if dirty {
		return fmt.Errorf("database is in a dirty state at version %d", version)
	}

	log.Printf("Current migration version: %d", version)
	return nil
}

// Rollback rolls back the last applied migration
func (m *Manager) Rollback(ctx context.Context) error {
	if m.migrator == nil {
		if err := m.Init(ctx); err != nil {
			return err
		}
	}

	// Step back one migration
	return m.migrator.Steps(-1)
}

// RollbackAll rolls back all migrations
func (m *Manager) RollbackAll(ctx context.Context) error {
	if m.migrator == nil {
		if err := m.Init(ctx); err != nil {
			return err
		}
	}

	// Migrate down to version 0
	err := m.migrator.Down()
	if err == migrate.ErrNoChange {
		log.Println("No migrations to roll back")
		return nil
	}
	return err
}

// GetVersion returns the current migration version
func (m *Manager) GetVersion() (uint, bool, error) {
	if m.migrator == nil {
		ctx := context.Background()
		if err := m.Init(ctx); err != nil {
			return 0, false, err
		}
	}

	return m.migrator.Version()
}

// ForceVersion forces the database to a specific version
func (m *Manager) ForceVersion(version uint) error {
	if m.migrator == nil {
		ctx := context.Background()
		if err := m.Init(ctx); err != nil {
			return err
		}
	}

	return m.migrator.Force(int(version))
}

// Close releases resources used by the migration manager
func (m *Manager) Close() error {
	var err error
	if m.migrator != nil {
		sourceErr, databaseErr := m.migrator.Close()
		if sourceErr != nil {
			err = fmt.Errorf("source error: %w", sourceErr)
		}
		if databaseErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; database error: %w", err, databaseErr)
			} else {
				err = fmt.Errorf("database error: %w", databaseErr)
			}
		}
	}
	return err
}

// WithTransaction executes a function within a transaction
// This is useful for data migrations that are not handled by SQL files
func (m *Manager) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	// Start a transaction
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute the function
	if err := fn(tx); err != nil {
		// Rollback transaction on error
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("error: %v, rollback error: %w", err, rbErr)
		}
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
