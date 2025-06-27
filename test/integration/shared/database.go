package shared

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"github.com/google/uuid"
)

// TestDatabaseConfig holds test database configuration
type TestDatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

// GetTestDatabaseConfig returns test database configuration
func GetTestDatabaseConfig() TestDatabaseConfig {
	return TestDatabaseConfig{
		Host:     getEnv("TEST_DB_HOST", "localhost"),
		Port:     getEnv("TEST_DB_PORT", "5432"),
		User:     getEnv("TEST_DB_USER", "test"),
		Password: getEnv("TEST_DB_PASSWORD", "test"),
		Database: getEnv("TEST_DB_NAME", "mcp_test"),
		SSLMode:  getEnv("TEST_DB_SSLMODE", "disable"),
	}
}

// GetTestDatabase returns a test database connection
func GetTestDatabase(ctx context.Context) (*sql.DB, error) {
	config := GetTestDatabaseConfig()

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.Database, config.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for testing
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// RunMigrations runs database migrations
func RunMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	migrationsPath := getEnv("MIGRATIONS_PATH", "file://./migrations")
	m, err := migrate.NewWithDatabaseInstance(migrationsPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// CleanupTestData removes all test data for a tenant
func CleanupTestData(db *sql.DB, tenantID uuid.UUID) error {
	// Clean up in reverse order of dependencies
	tables := []string{
		"document_changes",
		"documents",
		"workspace_members",
		"workspace_activities",
		"workspaces",
		"workflow_step_executions",
		"workflow_executions",
		"workflow_steps",
		"workflows",
		"task_assignments",
		"task_delegations",
		"tasks",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table)
		if _, err := db.Exec(query, tenantID); err != nil {
			return fmt.Errorf("failed to clean up %s: %w", table, err)
		}
	}

	return nil
}

// CleanupTaskData removes task-related data for a tenant
func CleanupTaskData(db *sql.DB, tenantID uuid.UUID) error {
	tables := []string{
		"task_assignments",
		"task_delegations",
		"tasks",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table)
		if _, err := db.Exec(query, tenantID); err != nil {
			return fmt.Errorf("failed to clean up %s: %w", table, err)
		}
	}

	return nil
}

// CleanupWorkflowData removes workflow-related data for a tenant
func CleanupWorkflowData(db *sql.DB, tenantID uuid.UUID) error {
	tables := []string{
		"workflow_step_executions",
		"workflow_executions",
		"workflow_steps",
		"workflows",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table)
		if _, err := db.Exec(query, tenantID); err != nil {
			return fmt.Errorf("failed to clean up %s: %w", table, err)
		}
	}

	return nil
}

// CleanupWorkspaceData removes workspace-related data for a tenant
func CleanupWorkspaceData(db *sql.DB, tenantID uuid.UUID) error {
	tables := []string{
		"document_changes",
		"documents",
		"workspace_members",
		"workspace_activities",
		"workspaces",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table)
		if _, err := db.Exec(query, tenantID); err != nil {
			return fmt.Errorf("failed to clean up %s: %w", table, err)
		}
	}

	return nil
}

// CreateTestTenant creates a test tenant
func CreateTestTenant(db *sql.DB) (uuid.UUID, error) {
	tenantID := uuid.New()

	query := `
		INSERT INTO tenants (id, name, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`

	_, err := db.Exec(query, tenantID, fmt.Sprintf("Test Tenant %s", tenantID.String()[:8]))
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create test tenant: %w", err)
	}

	return tenantID, nil
}

// CreateTestUser creates a test user
func CreateTestUser(db *sql.DB, tenantID uuid.UUID) (uuid.UUID, error) {
	userID := uuid.New()

	query := `
		INSERT INTO users (id, tenant_id, email, name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', NOW(), NOW())
	`

	email := fmt.Sprintf("user-%s@test.com", userID.String()[:8])
	name := fmt.Sprintf("Test User %s", userID.String()[:8])

	_, err := db.Exec(query, userID, tenantID, email, name)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create test user: %w", err)
	}

	return userID, nil
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(timeout time.Duration, check func() bool) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if check() {
			return true
		}

		<-ticker.C
		if time.Now().After(deadline) {
			return false
		}
	}
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
