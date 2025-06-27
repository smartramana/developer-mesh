package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationsIsolated tests migrations in a completely isolated environment
func TestMigrationsIsolated(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping migration test in short mode")
	}

	// Get database credentials from environment or use defaults
	dbUser := os.Getenv("DATABASE_USER")
	if dbUser == "" {
		dbUser = "dev"
	}
	dbPassword := os.Getenv("DATABASE_PASSWORD")
	if dbPassword == "" {
		dbPassword = "dev"
	}
	dbHost := os.Getenv("DATABASE_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		dbName = "dev"
	}

	// Connect to database
	dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skip("Cannot connect to database:", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close database: %v", err)
		}
	}()

	// Test connection
	if err := db.Ping(); err != nil {
		t.Skip("Cannot connect to database:", err)
	}

	// Create a unique test schema
	testSchema := fmt.Sprintf("test_mig_%d", time.Now().UnixNano())

	// Create test schema
	_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", testSchema))
	if err != nil {
		t.Skip("Cannot create test schema:", err)
	}

	// Ensure cleanup
	defer func() {
		_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA %s CASCADE", testSchema))
	}()

	// Create temporary migration files that use our test schema
	tempDir := t.TempDir()
	err = createTestMigrations(tempDir, testSchema)
	require.NoError(t, err)

	// Create a new connection with the test schema as search path
	testDSN := fmt.Sprintf("%s&search_path=%s", dsn, testSchema)
	testDB, err := sql.Open("postgres", testDSN)
	require.NoError(t, err)
	defer func() {
		if err := testDB.Close(); err != nil {
			t.Errorf("Failed to close test database: %v", err)
		}
	}()

	// Create migrate instance
	driver, err := postgres.WithInstance(testDB, &postgres.Config{
		MigrationsTable: "schema_migrations",
	})
	require.NoError(t, err)

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", tempDir),
		"postgres", driver)
	require.NoError(t, err)

	// Test up migrations
	t.Run("Up", func(t *testing.T) {
		err := m.Up()
		assert.NoError(t, err)

		// Verify tables exist in our test schema
		tables := []string{
			"contexts",
			"users",
			"api_keys",
			"api_key_usage",
		}

		for _, table := range tables {
			var exists bool
			err = testDB.QueryRow(`
				SELECT EXISTS (
					SELECT 1 FROM information_schema.tables 
					WHERE table_schema = $1 AND table_name = $2
				)
			`, testSchema, table).Scan(&exists)
			assert.NoError(t, err)
			assert.True(t, exists, fmt.Sprintf("Table %s.%s should exist", testSchema, table))
		}
	})

	// Test down migrations
	t.Run("Down", func(t *testing.T) {
		err := m.Down()
		assert.NoError(t, err)

		// Verify tables are gone (except schema_migrations)
		var tableCount int
		err = testDB.QueryRow(`
			SELECT COUNT(*) FROM information_schema.tables 
			WHERE table_schema = $1 AND table_name != 'schema_migrations'
		`, testSchema).Scan(&tableCount)
		assert.NoError(t, err)
		assert.Equal(t, 0, tableCount, "No tables should exist after down migration (except schema_migrations)")
	})
}

// createTestMigrations creates simplified migration files for testing
func createTestMigrations(dir, schema string) error {
	// Create a simple up migration
	upContent := fmt.Sprintf(`-- Test migration
CREATE TABLE %s.contexts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE %s.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE %s.api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) UNIQUE NOT NULL,
    user_id UUID REFERENCES %s.users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE %s.api_key_usage (
    api_key_id UUID NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (api_key_id, used_at)
);
`, schema, schema, schema, schema, schema)

	// Create a simple down migration
	downContent := fmt.Sprintf(`-- Undo test migration
DROP TABLE IF EXISTS %s.api_key_usage;
DROP TABLE IF EXISTS %s.api_keys;
DROP TABLE IF EXISTS %s.users;
DROP TABLE IF EXISTS %s.contexts;
`, schema, schema, schema, schema)

	// Write migration files
	err := os.WriteFile(filepath.Join(dir, "000001_test.up.sql"), []byte(upContent), 0644)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "000001_test.down.sql"), []byte(downContent), 0644)
}
