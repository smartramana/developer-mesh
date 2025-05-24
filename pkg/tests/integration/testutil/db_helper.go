// Package testutil provides shared utilities for integration testing
package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// TxKey defines a custom type for transaction context keys to avoid collisions
type TxKey string

// Known transaction context keys
const (
	// TransactionKey is the key used to store transaction in context
	// This matches what's expected in all repository implementations
	TransactionKey TxKey = "tx"
)

// WithTx adds a transaction to the context
func WithTx(ctx context.Context, tx *sqlx.Tx) context.Context {
	return context.WithValue(ctx, TransactionKey, tx)
}

// GetTx retrieves a transaction from the context
func GetTx(ctx context.Context) (*sqlx.Tx, bool) {
	tx, ok := ctx.Value(TransactionKey).(*sqlx.Tx)
	return tx, ok
}

// IsDatabaseSQLite determines if the database is using SQLite
func IsDatabaseSQLite(ctx context.Context, db *sqlx.DB) bool {
	if db == nil {
		return false
	}

	// Try to query SQLite version - this will only succeed on SQLite
	row := db.QueryRowContext(ctx, "SELECT sqlite_version()")
	var version string
	err := row.Scan(&version)
	return err == nil
}

// GetTablePrefix returns the appropriate table prefix based on database type
func GetTablePrefix(ctx context.Context, db *sqlx.DB) string {
	if IsDatabaseSQLite(ctx, db) {
		return ""
	}
	return "mcp."
}

// SetupTestDatabase creates a test database and initializes it with the standard schema
func SetupTestDatabase(t *testing.T) (*database.Database, context.Context) {
	ctx := context.Background()
	
	// Create test database with context
	db, err := database.NewTestDatabaseWithContext(ctx)
	require.NoError(t, err)
	require.NotNil(t, db)
	
	// Initialize tables with standard schema
	err = InitializeTestTables(ctx, db.DB())
	require.NoError(t, err, "Should be able to initialize test tables")
	
	return db, ctx
}

// InitializeTestTables creates the standard test tables in the database
func InitializeTestTables(ctx context.Context, db *sqlx.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Determine if we're using SQLite or PostgreSQL
	isSQLite := IsDatabaseSQLite(ctx, db)
	tablePrefix := ""
	
	// Configure database based on type
	if isSQLite {
		fmt.Println("Using SQLite database for tests")
		// For SQLite, enable foreign keys
		_, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
		if err != nil {
			return fmt.Errorf("failed to enable foreign keys in SQLite: %w", err)
		}
	} else {
		fmt.Println("Using PostgreSQL database for tests")
		// For PostgreSQL, create the schema if it doesn't exist
		// First drop existing schema to ensure clean test environment
		_, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS mcp CASCADE")
		if err != nil {
			return fmt.Errorf("failed to drop schema: %w", err)
		}
		
		// Create fresh schema
		_, err = db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS mcp")
		if err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
		tablePrefix = "mcp."
	}
	
	// Create standard tables for testing
	return createStandardTables(ctx, db, tablePrefix, isSQLite)
}

// createStandardTables creates the standard tables needed for integration tests
func createStandardTables(ctx context.Context, db *sqlx.DB, tablePrefix string, isSQLite bool) error {
	// Create models table
	modelsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %smodels (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`, tablePrefix)
	
	_, err := db.ExecContext(ctx, modelsTable)
	if err != nil {
		return fmt.Errorf("failed to create models table: %w", err)
	}
	
	// Create agents table
	agentsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %sagents (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		name TEXT NOT NULL,
		model_id TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (model_id) REFERENCES %smodels(id)
	)`, tablePrefix, tablePrefix)
	
	_, err = db.ExecContext(ctx, agentsTable)
	if err != nil {
		return fmt.Errorf("failed to create agents table: %w", err)
	}
	
	// Create relationships table
	relationshipsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %srelationships (
		id TEXT PRIMARY KEY,
		source_id TEXT NOT NULL,
		source_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		relationship_type TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`, tablePrefix)
	
	_, err = db.ExecContext(ctx, relationshipsTable)
	if err != nil {
		return fmt.Errorf("failed to create relationships table: %w", err)
	}
	
	// Create indices for faster lookups
	if !isSQLite {
		// Model indices
		modelIndices := []string{
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%smodels_tenant ON %smodels(tenant_id)`, 
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
		}
		
		for _, indexQuery := range modelIndices {
			_, err = db.ExecContext(ctx, indexQuery)
			if err != nil {
				return fmt.Errorf("failed to create model index: %w", err)
			}
		}
		
		// Agent indices
		agentIndices := []string{
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%sagents_tenant ON %sagents(tenant_id)`, 
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%sagents_model ON %sagents(model_id)`, 
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
		}
		
		for _, indexQuery := range agentIndices {
			_, err = db.ExecContext(ctx, indexQuery)
			if err != nil {
				return fmt.Errorf("failed to create agent index: %w", err)
			}
		}
		
		// Relationship indices
		relationshipIndices := []string{
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%srelationships_source ON %srelationships(source_id, source_type)`, 
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%srelationships_target ON %srelationships(target_id, target_type)`, 
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%srelationships_type ON %srelationships(relationship_type)`, 
				tablePrefix[0:len(tablePrefix)-1], tablePrefix),
		}
		
		for _, indexQuery := range relationshipIndices {
			_, err = db.ExecContext(ctx, indexQuery)
			if err != nil {
				return fmt.Errorf("failed to create relationship index: %w", err)
			}
		}
	}
	
	return nil
}

// CreateTestModel creates a test model with standard properties
func CreateTestModel(tenantID string) *models.Model {
	return &models.Model{
		ID:       uuid.New().String(),
		TenantID: tenantID,
		Name:     "Test Model " + uuid.New().String()[0:8],
	}
}

// CreateTestAgent creates a test agent with standard properties
func CreateTestAgent(tenantID, modelID string) *models.Agent {
	return &models.Agent{
		ID:       uuid.New().String(),
		TenantID: tenantID,
		Name:     "Test Agent " + uuid.New().String()[0:8],
		ModelID:  modelID,
	}
}
