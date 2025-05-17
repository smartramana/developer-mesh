// Package migrations provides database schema migrations and versioning utilities.
package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MigrationOptions represents configuration options for database migrations
type MigrationOptions struct {
	// Path to migration files
	Path string
	// Whether to fail on migration errors
	FailOnError bool
	// Directory containing migration scripts
	MigrationsFS fs.FS
}

// DefaultOptions returns the default migration options
func DefaultOptions() MigrationOptions {
	return MigrationOptions{
		Path:        "migrations",
		FailOnError: true,
	}
}

// Migrate applies database migrations to the given database
func Migrate(ctx context.Context, db *sql.DB, driver string, opts MigrationOptions) error {
	log.Printf("Running migrations from path: %s", opts.Path)
	
	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(ctx, db, driver); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}
	
	// Get applied migrations
	applied, err := getAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}
	
	// Get migration files
	migrations, err := getMigrationFiles(opts.Path)
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}
	
	// Sort migrations by version
	sort.Strings(migrations)
	
	// Apply migrations
	for _, migration := range migrations {
		// Skip if already applied
		if contains(applied, migration) {
			log.Printf("Migration %s already applied, skipping", migration)
			continue
		}
		
		log.Printf("Applying migration: %s", migration)
		
		// Read migration file
		content, err := os.ReadFile(filepath.Join(opts.Path, migration))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migration, err)
		}
		
		// Begin transaction
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", migration, err)
		}
		
		// Execute migration
		_, err = tx.ExecContext(ctx, string(content))
		if err != nil {
			tx.Rollback()
			if opts.FailOnError {
				return fmt.Errorf("failed to execute migration %s: %w", migration, err)
			}
			log.Printf("Warning: Failed to execute migration %s: %v", migration, err)
			continue
		}
		
		// Record migration
		_, err = tx.ExecContext(ctx, "INSERT INTO _migrations (version, applied_at) VALUES ($1, $2)",
			migration, time.Now())
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", migration, err)
		}
		
		// Commit transaction
		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", migration, err)
		}
		
		log.Printf("Successfully applied migration: %s", migration)
	}
	
	return nil
}

// AutoMigrate applies migrations if enabled in the configuration
func AutoMigrate(ctx context.Context, db *sql.DB, driver string, opts MigrationOptions) error {
	return Migrate(ctx, db, driver, opts)
}

// createMigrationsTable creates the migrations tracking table if it doesn't exist
func createMigrationsTable(ctx context.Context, db *sql.DB, driver string) error {
	var query string
	
	switch driver {
	case "postgres":
		query = `
		CREATE TABLE IF NOT EXISTS _migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL
		);
		`
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}
	
	_, err := db.ExecContext(ctx, query)
	return err
}

// getAppliedMigrations gets a list of already applied migrations
func getAppliedMigrations(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM _migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var migrations []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		migrations = append(migrations, version)
	}
	
	return migrations, rows.Err()
}

// getMigrationFiles gets a list of migration files from the migrations directory
func getMigrationFiles(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	
	return files, nil
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
