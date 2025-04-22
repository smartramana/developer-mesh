package migration

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

// DataMigration represents a migration that transforms data using Go code
type DataMigration struct {
	// Name is a descriptive name for the migration
	Name string
	
	// Version is the migration version number
	Version uint
	
	// Execute is the function that performs the data migration
	Execute func(ctx context.Context, tx *sqlx.Tx) error
	
	// Rollback is the function that reverts the data migration (optional)
	Rollback func(ctx context.Context, tx *sqlx.Tx) error
}

// DataMigrator handles data migrations using Go code
type DataMigrator struct {
	db         *sqlx.DB
	migrations []DataMigration
	logger     *log.Logger
}

// NewDataMigrator creates a new data migrator
func NewDataMigrator(db *sqlx.DB, logger *log.Logger) *DataMigrator {
	return &DataMigrator{
		db:         db,
		migrations: []DataMigration{},
		logger:     logger,
	}
}

// Register adds a migration to the registry
func (m *DataMigrator) Register(migration DataMigration) {
	m.migrations = append(m.migrations, migration)
}

// RunMigration applies a specific data migration
func (m *DataMigrator) RunMigration(ctx context.Context, version uint) error {
	for _, migration := range m.migrations {
		if migration.Version == version {
			startTime := time.Now()
			m.logger.Printf("Running data migration %s (version %d)...", migration.Name, migration.Version)
			
			// Execute migration in a transaction
			err := m.db.RunTxContext(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
				return migration.Execute(ctx, tx)
			})
			
			if err != nil {
				return fmt.Errorf("data migration %s failed: %w", migration.Name, err)
			}
			
			m.logger.Printf("Data migration %s completed in %s", migration.Name, time.Since(startTime))
			return nil
		}
	}
	
	return fmt.Errorf("data migration with version %d not found", version)
}

// RollbackMigration rolls back a specific data migration
func (m *DataMigrator) RollbackMigration(ctx context.Context, version uint) error {
	for _, migration := range m.migrations {
		if migration.Version == version {
			if migration.Rollback == nil {
				return fmt.Errorf("data migration %s does not support rollback", migration.Name)
			}
			
			startTime := time.Now()
			m.logger.Printf("Rolling back data migration %s (version %d)...", migration.Name, migration.Version)
			
			// Execute rollback in a transaction
			err := m.db.RunTxContext(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
				return migration.Rollback(ctx, tx)
			})
			
			if err != nil {
				return fmt.Errorf("data migration %s rollback failed: %w", migration.Name, err)
			}
			
			m.logger.Printf("Data migration %s rollback completed in %s", migration.Name, time.Since(startTime))
			return nil
		}
	}
	
	return fmt.Errorf("data migration with version %d not found", version)
}

// Example usage:
//
// migrator := migration.NewDataMigrator(db, log.Default())
//
// // Register a migration
// migrator.Register(migration.DataMigration{
//     Name:    "Add Default Tags",
//     Version: 1,
//     Execute: func(ctx context.Context, tx *sqlx.Tx) error {
//         // Add default tags to all existing posts
//         _, err := tx.ExecContext(ctx, `
//             UPDATE posts
//             SET tags = COALESCE(tags, '[]'::jsonb) || '["default"]'::jsonb
//             WHERE tags IS NULL OR NOT jsonb_exists(tags, 'default')
//         `)
//         return err
//     },
//     Rollback: func(ctx context.Context, tx *sqlx.Tx) error {
//         // Remove default tags from all posts
//         _, err := tx.ExecContext(ctx, `
//             UPDATE posts
//             SET tags = tags - 'default'
//         `)
//         return err
//     },
// })
//
// // Run the migration
// if err := migrator.RunMigration(ctx, 1); err != nil {
//     log.Fatalf("Migration failed: %v", err)
// }
