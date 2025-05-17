package migration

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
)

// AutoMigrateOptions contains options for automatic migration
type AutoMigrateOptions struct {
	// Whether to automatically run migrations on startup
	Enabled bool
	
	// Path to migration files
	Path string
	
	// Whether to fail startup if migrations fail
	FailOnError bool
	
	// Timeout for migration operations
	Timeout time.Duration
	
	// Whether to validate migrations without applying them
	ValidateOnly bool
	
	// Logger to use for migration messages
	Logger *log.Logger
}

// DefaultOptions returns the default migration options
func DefaultOptions() AutoMigrateOptions {
	return AutoMigrateOptions{
		Enabled:     true,
		Path:        "migrations/sql",
		FailOnError: true,
		Timeout:     1 * time.Minute,
		ValidateOnly: false,
		Logger:      log.New(os.Stdout, "[DB Migration] ", log.LstdFlags),
	}
}

// AutoMigrate performs automatic database migration on startup
func AutoMigrate(ctx context.Context, db *sqlx.DB, driverName string, options AutoMigrateOptions) error {
	if !options.Enabled {
		options.Logger.Println("Automatic migrations disabled")
		return nil
	}

	options.Logger.Printf("Starting database migration from %s", options.Path)
	
	// Create migration manager
	manager, err := NewManager(db, Config{
		MigrationsPath:   options.Path,
		AutoMigrate:      options.Enabled,
		MigrationTimeout: options.Timeout,
		ValidateOnly:     options.ValidateOnly,
	}, driverName)
	
	if err != nil {
		return fmt.Errorf("failed to create migration manager: %w", err)
	}
	defer manager.Close()
	
	// Initialize the migration manager
	if err := manager.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize migration manager: %w", err)
	}
	
	// Get current version before migration
	version, dirty, err := manager.GetVersion()
	if err == nil {
		options.Logger.Printf("Current migration version: %d, dirty: %t", version, dirty)
	}
	
	// If validation only, just validate without running migrations
	if options.ValidateOnly {
		options.Logger.Println("Validating migrations without applying them")
		if err := manager.ValidateMigrations(ctx); err != nil {
			return fmt.Errorf("migration validation failed: %w", err)
		}
		options.Logger.Println("Migration validation succeeded")
		return nil
	}
	
	// Apply migrations
	startTime := time.Now()
	if err := manager.RunMigrations(ctx); err != nil {
		options.Logger.Printf("Migration failed: %v", err)
		if options.FailOnError {
			return fmt.Errorf("migration failed: %w", err)
		}
		// Log but don't return error if FailOnError is false
		options.Logger.Printf("Continuing despite migration failure due to FailOnError=false")
		return nil
	}
	
	// Get new version after migration
	newVersion, dirty, err := manager.GetVersion()
	if err == nil && newVersion != version {
		options.Logger.Printf("Migrated from version %d to %d in %s", 
			version, newVersion, time.Since(startTime))
	} else {
		options.Logger.Printf("Migrations completed in %s", time.Since(startTime))
	}
	
	return nil
}
