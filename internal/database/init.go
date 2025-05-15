package database

import (
	"context"
	"fmt"
	"log"
)

// InitializeTables initializes all required database tables
func (db *Database) InitializeTables(ctx context.Context) error {
	log.Println("Initializing database tables...")

	// Ensure context tables
	if err := db.ensureContextTables(ctx); err != nil {
		return fmt.Errorf("failed to initialize context tables: %w", err)
	}
	
	// Ensure GitHub content tables
	if err := db.ensureGitHubContentTables(ctx); err != nil {
		return fmt.Errorf("failed to initialize GitHub content tables: %w", err)
	}
	
	log.Println("Database tables initialized successfully")
	return nil
}
