package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// NewTestDatabase is a helper that creates a new test database with a background context
// Maintained for backward compatibility with existing code
func NewTestDatabase() (*Database, error) {
	return NewTestDatabaseWithContext(context.Background())
}

// NewTestDatabaseWithContext creates a new database instance for testing purposes.
func NewTestDatabaseWithContext(ctx context.Context) (*Database, error) {
	// Default to SQLite in-memory database for testing
	dbType := os.Getenv("TEST_DB_TYPE")
	if dbType == "" {
		dbType = "sqlite3"
	}

	var db *sqlx.DB
	var err error

	if dbType == "postgres" {
		// Use PostgreSQL for testing if specified
		postgresDSN := os.Getenv("TEST_POSTGRES_DSN")
		if postgresDSN == "" {
			postgresDSN = "host=localhost port=5432 user=postgres password=postgres dbname=mcp_test sslmode=disable"
		}

		// Open direct connection to PostgreSQL
		db, err = sqlx.Connect("postgres", postgresDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to test database: %w", err)
		}

		// Drop schema if it exists to ensure clean test environment
		_, err = db.ExecContext(ctx, "DROP SCHEMA IF EXISTS mcp CASCADE")
		if err != nil {
			return nil, fmt.Errorf("failed to drop schema: %w", err)
		}

		// Create schema
		_, err = db.ExecContext(ctx, "CREATE SCHEMA mcp")
		if err != nil {
			return nil, fmt.Errorf("failed to create schema: %w", err)
		}
	} else {
		// Use SQLite in-memory database
		db, err = sqlx.Connect("sqlite3", ":memory:")
		if err != nil {
			return nil, fmt.Errorf("failed to connect to in-memory SQLite database: %w", err)
		}

		// Enable foreign keys for SQLite
		_, err = db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
		if err != nil {
			return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
		}
	}

	// Create the Database wrapper
	testDb := &Database{
		db:         db,
		statements: make(map[string]*sqlx.Stmt),
		config: Config{
			Driver: dbType,
		},
	}

	// Create all the tables needed for testing
	// We'll use the test-specific methods that have been renamed to avoid conflicts

	// Create context tables
	if err := testDb.ensureTestContextTables(ctx); err != nil {
		testDb.Close()
		return nil, fmt.Errorf("failed to create context tables: %w", err)
	}

	// Create GitHub content tables
	if err := testDb.ensureTestGitHubContentTables(ctx); err != nil {
		testDb.Close()
		return nil, fmt.Errorf("failed to create GitHub content tables: %w", err)
	}

	// Create model tables
	if err := testDb.ensureTestModelTables(ctx); err != nil {
		testDb.Close()
		return nil, fmt.Errorf("failed to create model tables: %w", err)
	}

	// Create agent tables
	if err := testDb.ensureTestAgentTables(ctx); err != nil {
		testDb.Close()
		return nil, fmt.Errorf("failed to create agent tables: %w", err)
	}

	// Create vector tables
	if err := testDb.ensureTestVectorTables(ctx); err != nil {
		testDb.Close()
		return nil, fmt.Errorf("failed to create vector tables: %w", err)
	}

	// Create relationship tables
	if err := testDb.ensureTestRelationshipTables(ctx); err != nil {
		testDb.Close()
		return nil, fmt.Errorf("failed to create relationship tables: %w", err)
	}

	// Create additional tables needed for integration tests
	// These might not be covered by the standard table creation methods

	// Determine JSON data type based on database type
	jsonType := "TEXT"
	if dbType == "postgres" {
		jsonType = "JSONB"
	}

	// Schema prefix for table names
	schemaPrefix := ""
	if dbType == "postgres" {
		schemaPrefix = "mcp."
	}

	// Create events table (needed for worker processing)
	eventsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %sevents (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		type TEXT NOT NULL,
		status TEXT NOT NULL,
		payload %s,
		metadata %s,
		processed_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`, schemaPrefix, jsonType, jsonType)

	if _, err := testDb.db.ExecContext(ctx, eventsTable); err != nil {
		testDb.Close()
		return nil, fmt.Errorf("failed to create events table: %w", err)
	}

	// Create integrations table (needed for API tests)
	integrationsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %sintegrations (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		configuration %s,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`, schemaPrefix, jsonType)

	if _, err := testDb.db.ExecContext(ctx, integrationsTable); err != nil {
		testDb.Close()
		return nil, fmt.Errorf("failed to create integrations table: %w", err)
	}

	return testDb, nil
}

// ensureContextTables creates tables for storing context data
func (d *Database) ensureTestContextTables(ctx context.Context) error {
	var queries []string

	// Handle SQL dialect differences
	if d.config.Driver == "sqlite3" {
		// SQLite-compatible queries
		queries = []string{
			// Create contexts table
			`CREATE TABLE IF NOT EXISTS contexts (
				id TEXT PRIMARY KEY,
				agent_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				max_items INTEGER NOT NULL DEFAULT 100,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			// Create context_items table
			`CREATE TABLE IF NOT EXISTS context_items (
				id TEXT PRIMARY KEY,
				context_id TEXT NOT NULL,
				role TEXT NOT NULL,
				content TEXT NOT NULL,
				tokens INTEGER,
				timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				metadata TEXT, -- JSON stored as text in SQLite
				FOREIGN KEY (context_id) REFERENCES contexts(id) ON DELETE CASCADE
			)`,

			// Create integrations table
			`CREATE TABLE IF NOT EXISTS integrations (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				name TEXT NOT NULL,
				type TEXT NOT NULL,
				configuration TEXT, -- JSON stored as text in SQLite
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
		}
	} else {
		// PostgreSQL-compatible queries
		queries = []string{
			// Create contexts table
			`CREATE TABLE IF NOT EXISTS mcp.contexts (
				id TEXT PRIMARY KEY,
				agent_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				max_items INTEGER NOT NULL DEFAULT 100,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			// Create context_items table
			`CREATE TABLE IF NOT EXISTS mcp.context_items (
				id TEXT PRIMARY KEY,
				context_id TEXT NOT NULL,
				role TEXT NOT NULL,
				content TEXT NOT NULL,
				tokens INTEGER,
				timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				metadata JSONB,
				FOREIGN KEY (context_id) REFERENCES mcp.contexts(id) ON DELETE CASCADE
			)`,

			// Create integrations table
			`CREATE TABLE IF NOT EXISTS mcp.integrations (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				name TEXT NOT NULL,
				type TEXT NOT NULL,
				configuration JSONB,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
		}
	}

	for _, query := range queries {
		if _, err := d.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %s: %w", query, err)
		}
	}

	return nil
}

// ensureGitHubContentTables creates tables for storing GitHub content
func (d *Database) ensureTestGitHubContentTables(ctx context.Context) error {
	var queries []string

	// Handle SQL dialect differences
	if d.config.Driver == "sqlite3" {
		// SQLite-compatible queries
		queries = []string{
			// Create github_content table
			`CREATE TABLE IF NOT EXISTS github_content (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				owner TEXT NOT NULL,
				repo TEXT NOT NULL,
				path TEXT NOT NULL,
				ref TEXT NOT NULL,
				content TEXT NOT NULL,
				checksum TEXT NOT NULL,
				size INTEGER,
				metadata TEXT, -- JSON stored as text in SQLite
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			// Create index on checksum for fast lookups
			`CREATE INDEX IF NOT EXISTS idx_github_content_checksum ON github_content(checksum)`,
		}
	} else {
		// PostgreSQL-compatible queries
		queries = []string{
			// Create github_content table
			`CREATE TABLE IF NOT EXISTS mcp.github_content (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				owner TEXT NOT NULL,
				repo TEXT NOT NULL,
				path TEXT NOT NULL,
				ref TEXT NOT NULL,
				content TEXT NOT NULL,
				checksum TEXT NOT NULL,
				size INTEGER,
				metadata JSONB,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			// Create index on checksum for fast lookups
			`CREATE INDEX IF NOT EXISTS idx_github_content_checksum ON mcp.github_content(checksum)`,
		}
	}

	for _, query := range queries {
		if _, err := d.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %s: %w", query, err)
		}
	}

	return nil
}

// ensureModelTables creates tables for models
func (d *Database) ensureTestModelTables(ctx context.Context) error {
	var queries []string

	// Handle SQL dialect differences
	if d.config.Driver == "sqlite3" {
		// SQLite-compatible queries
		queries = []string{
			// Create models table - simplified schema matching pkg/models/model.go
			`CREATE TABLE IF NOT EXISTS models (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				name TEXT NOT NULL
			)`,

			// Create index on tenant_id for models
			`CREATE INDEX IF NOT EXISTS idx_models_tenant ON models(tenant_id)`,
		}
	} else {
		// PostgreSQL-compatible queries
		queries = []string{
			// Create models table - simplified schema matching pkg/models/model.go
			`CREATE TABLE IF NOT EXISTS mcp.models (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				name TEXT NOT NULL
			)`,

			// Create index on tenant_id for models
			`CREATE INDEX IF NOT EXISTS idx_models_tenant ON mcp.models(tenant_id)`,
		}
	}

	for _, query := range queries {
		if _, err := d.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %s: %w", query, err)
		}
	}

	return nil
}

// ensureAgentTables creates tables for agents
func (d *Database) ensureTestAgentTables(ctx context.Context) error {
	var queries []string

	// Handle SQL dialect differences
	if d.config.Driver == "sqlite3" {
		// SQLite-compatible queries
		queries = []string{
			// Create agents table with simplified schema matching pkg/models/agent.go
			`CREATE TABLE IF NOT EXISTS agents (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				name TEXT NOT NULL,
				model_id TEXT NOT NULL,
				FOREIGN KEY (model_id) REFERENCES models(id)
			)`,

			// Create index on tenant_id for agents
			`CREATE INDEX IF NOT EXISTS idx_agents_tenant ON agents(tenant_id)`,
		}
	} else {
		// PostgreSQL-compatible queries
		queries = []string{
			// Create agents table with simplified schema matching pkg/models/agent.go
			`CREATE TABLE IF NOT EXISTS mcp.agents (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				name TEXT NOT NULL,
				model_id TEXT NOT NULL,
				FOREIGN KEY (model_id) REFERENCES mcp.models(id)
			)`,

			// Create index on tenant_id for agents
			`CREATE INDEX IF NOT EXISTS idx_agents_tenant ON mcp.agents(tenant_id)`,
		}
	}

	for _, query := range queries {
		if _, err := d.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %s: %w", query, err)
		}
	}

	return nil
}

// ensureVectorTables creates tables for vector storage
func (d *Database) ensureTestVectorTables(ctx context.Context) error {
	var queries []string

	// Handle SQL dialect differences
	if d.config.Driver == "sqlite3" {
		// SQLite-compatible queries
		queries = []string{
			// Create vectors table
			`CREATE TABLE IF NOT EXISTS vectors (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				content_id TEXT NOT NULL,
				embedding_model TEXT NOT NULL,
				embedding TEXT, -- Store vector data as JSON text in SQLite
				metadata TEXT, -- JSON stored as text in SQLite
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			// Create index on tenant_id for vectors
			`CREATE INDEX IF NOT EXISTS idx_vectors_tenant ON vectors(tenant_id)`,
			`CREATE INDEX IF NOT EXISTS idx_vectors_content ON vectors(content_id)`,
		}
	} else {
		// PostgreSQL-compatible queries
		queries = []string{
			// Create vectors table
			`CREATE TABLE IF NOT EXISTS mcp.vectors (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				content_id TEXT NOT NULL,
				embedding_model TEXT NOT NULL,
				embedding JSONB, -- Store vector data as JSON array
				metadata JSONB,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			// Create index on tenant_id for vectors
			`CREATE INDEX IF NOT EXISTS idx_vectors_tenant ON mcp.vectors(tenant_id)`,
			`CREATE INDEX IF NOT EXISTS idx_vectors_content ON mcp.vectors(content_id)`,
		}
	}

	for _, query := range queries {
		if _, err := d.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %s: %w", query, err)
		}
	}

	return nil
}

// EnsureRelationshipTables creates tables for storing entity relationships
func (d *Database) ensureTestRelationshipTables(ctx context.Context) error {
	var queries []string

	// Handle SQL dialect differences
	if d.config.Driver == "sqlite3" {
		// SQLite-compatible queries
		queries = []string{
			// Create relationships table
			`CREATE TABLE IF NOT EXISTS relationships (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				source_id TEXT NOT NULL,
				source_type TEXT NOT NULL,
				target_id TEXT NOT NULL,
				target_type TEXT NOT NULL,
				relationship_type TEXT NOT NULL,
				metadata TEXT, -- JSON stored as text in SQLite
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			// Create indices for relationship lookups
			`CREATE INDEX IF NOT EXISTS idx_relationships_source ON relationships(source_id, source_type)`,
			`CREATE INDEX IF NOT EXISTS idx_relationships_target ON relationships(target_id, target_type)`,
			`CREATE INDEX IF NOT EXISTS idx_relationships_tenant ON relationships(tenant_id)`,
		}
	} else {
		// PostgreSQL-compatible queries
		queries = []string{
			// Create relationships table
			`CREATE TABLE IF NOT EXISTS mcp.relationships (
				id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				source_id TEXT NOT NULL,
				source_type TEXT NOT NULL,
				target_id TEXT NOT NULL,
				target_type TEXT NOT NULL,
				relationship_type TEXT NOT NULL,
				metadata JSONB,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			// Create indices for relationship lookups
			`CREATE INDEX IF NOT EXISTS idx_relationships_source ON mcp.relationships(source_id, source_type)`,
			`CREATE INDEX IF NOT EXISTS idx_relationships_target ON mcp.relationships(target_id, target_type)`,
			`CREATE INDEX IF NOT EXISTS idx_relationships_tenant ON mcp.relationships(tenant_id)`,
		}
	}

	for _, query := range queries {
		if _, err := d.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %s: %w", query, err)
		}
	}

	return nil
}
