package migrations

import (
    "database/sql"
    "fmt"
    "os"
    "testing"
    
    _ "github.com/lib/pq"
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMigrations(t *testing.T) {
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
    
    // Database connection string for testing
    dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/postgres?sslmode=disable", dbUser, dbPassword, dbHost)
    
    // Open database connection
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        t.Skip("Cannot connect to test database:", err)
    }
    defer db.Close()
    
    // Test connection
    if err := db.Ping(); err != nil {
        t.Skip("Cannot connect to test database:", err)
    }
    
    // Create test database
    _, err = db.Exec("CREATE DATABASE test_migrations")
    if err != nil && !isAlreadyExistsError(err) {
        t.Skip("Failed to create test database (may need permissions):", err)
    }
    
    // Connect to test database
    testDSN := fmt.Sprintf("postgres://%s:%s@%s:5432/test_migrations?sslmode=disable", dbUser, dbPassword, dbHost)
    testDB, err := sql.Open("postgres", testDSN)
    require.NoError(t, err)
    defer testDB.Close()
    
    // Clean up after test
    defer func() {
        testDB.Close()
        _, _ = db.Exec("DROP DATABASE test_migrations")
    }()
    
    // Create migrate instance
    driver, err := postgres.WithInstance(testDB, &postgres.Config{})
    require.NoError(t, err)
    
    m, err := migrate.NewWithDatabaseInstance(
        "file://sql",
        "postgres", driver)
    require.NoError(t, err)
    
    // Test up migrations
    t.Run("Up", func(t *testing.T) {
        err := m.Up()
        assert.NoError(t, err)
        
        // Verify schema exists
        var schemaExists bool
        err = testDB.QueryRow(`
            SELECT EXISTS (
                SELECT 1 FROM information_schema.schemata 
                WHERE schema_name = 'mcp'
            )
        `).Scan(&schemaExists)
        assert.NoError(t, err)
        assert.True(t, schemaExists, "mcp schema should exist")
        
        // Verify tables exist
        tables := []string{
            "contexts",
            "users", 
            "api_keys",
            "api_key_usage",
            "embeddings",
            "embedding_models",
            "embedding_searches",
        }
        
        for _, table := range tables {
            var tableExists bool
            err = testDB.QueryRow(`
                SELECT EXISTS (
                    SELECT 1 FROM information_schema.tables 
                    WHERE table_schema = 'mcp' AND table_name = $1
                )
            `, table).Scan(&tableExists)
            assert.NoError(t, err)
            assert.True(t, tableExists, fmt.Sprintf("Table %s should exist", table))
        }
        
        // Verify functions exist
        functions := []string{
            "insert_embedding",
            "search_embeddings",
            "get_available_models",
        }
        
        for _, function := range functions {
            var functionExists bool
            err = testDB.QueryRow(`
                SELECT EXISTS (
                    SELECT 1 FROM pg_proc p
                    JOIN pg_namespace n ON p.pronamespace = n.oid
                    WHERE n.nspname = 'mcp' AND p.proname = $1
                )
            `, function).Scan(&functionExists)
            assert.NoError(t, err)
            assert.True(t, functionExists, fmt.Sprintf("Function %s should exist", function))
        }
        
        // Verify embedding models are populated
        var modelCount int
        err = testDB.QueryRow("SELECT COUNT(*) FROM mcp.embedding_models").Scan(&modelCount)
        assert.NoError(t, err)
        assert.Equal(t, 14, modelCount, "Should have 14 embedding models")
    })
    
    // Test down migrations
    t.Run("Down", func(t *testing.T) {
        err := m.Down()
        assert.NoError(t, err)
        
        // Verify schema is gone
        var schemaExists bool
        err = testDB.QueryRow(`
            SELECT EXISTS (
                SELECT 1 FROM information_schema.schemata 
                WHERE schema_name = 'mcp'
            )
        `).Scan(&schemaExists)
        assert.NoError(t, err)
        assert.False(t, schemaExists, "mcp schema should not exist after down migration")
    })
}

func isAlreadyExistsError(err error) bool {
    return err != nil && err.Error() == "pq: database \"test_migrations\" already exists"
}