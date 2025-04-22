package integration

import (
	"os"
	"testing"
)

// getTestDatabaseDSN returns the database DSN from environment variables
func getTestDatabaseDSN() string {
	// Look for the environment variable MCP_DATABASE_DSN
	dsn := os.Getenv("MCP_DATABASE_DSN")
	if dsn == "" {
		// Default test database DSN for development
		dsn = "postgres://postgres:postgres@localhost:5432/mcp_test?sslmode=disable"
	}
	return dsn
}

// SkipIfNoDatabase skips the test if database connection cannot be established
func SkipIfNoDatabase(t *testing.T) {
	if os.Getenv("ENABLE_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test as ENABLE_INTEGRATION_TESTS is not set to true")
	}
}
