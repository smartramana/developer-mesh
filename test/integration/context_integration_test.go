//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/cache"
	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextManagerIntegration tests the ContextManager with real database and cache
func TestContextManagerIntegration(t *testing.T) {
	// Skip test if ENABLE_INTEGRATION_TESTS is not set
	if os.Getenv("ENABLE_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set ENABLE_INTEGRATION_TESTS=true to run")
	}

	// Get database connection string from environment or use default
	dbConnStr := os.Getenv("TEST_DB_URL")
	if dbConnStr == "" {
		dbConnStr = "postgres://postgres:postgres@localhost:5432/test_db?sslmode=disable"
	}

	// Get Redis connection info from environment or use default
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Setup database connection
	db, err := sqlx.Connect("postgres", dbConnStr)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Setup Redis connection
	redisCache, err := cache.NewRedisCache(redisAddr, "", 0)
	require.NoError(t, err, "Failed to connect to Redis")

	// Create database wrapper
	dbWrapper := &database.PostgresDB{DB: db}

	// Create context manager
	cm := core.NewContextManager(dbWrapper, redisCache)
	require.NotNil(t, cm, "Context manager should not be nil")

	// Test CRUD operations
	t.Run("Create and retrieve context", func(t *testing.T) {
		// Create a test context
		testContext := &mcp.Context{
			AgentID:   "integration-test-agent",
			ModelID:   "integration-test-model",
			SessionID: "integration-test-session",
			Metadata: map[string]interface{}{
				"integration_test": true,
			},
		}

		// Create context
		ctx := context.Background()
		createdContext, err := cm.CreateContext(ctx, testContext)
		require.NoError(t, err, "Failed to create context")
		require.NotNil(t, createdContext, "Created context should not be nil")
		require.NotEmpty(t, createdContext.ID, "Created context should have an ID")

		// Test GetContext
		retrievedContext, err := cm.GetContext(ctx, createdContext.ID)
		assert.NoError(t, err, "Failed to retrieve context")
		assert.Equal(t, createdContext.ID, retrievedContext.ID, "Retrieved context should have the same ID")
		assert.Equal(t, "integration-test-agent", retrievedContext.AgentID, "AgentID should match")
		assert.Equal(t, "integration-test-model", retrievedContext.ModelID, "ModelID should match")
		assert.Equal(t, true, retrievedContext.Metadata["integration_test"], "Metadata should match")

		// Test UpdateContext
		updateRequest := &mcp.Context{
			Content: []mcp.ContextItem{
				{
					Role:      "user",
					Content:   "Test message",
					Tokens:    2,
					Timestamp: time.Now(),
				},
			},
		}

		updatedContext, err := cm.UpdateContext(ctx, createdContext.ID, updateRequest, nil)
		assert.NoError(t, err, "Failed to update context")
		assert.Len(t, updatedContext.Content, 1, "Updated context should have 1 content item")
		assert.Equal(t, "Test message", updatedContext.Content[0].Content, "Content should match")

		// Test ListContexts
		contexts, err := cm.ListContexts(ctx, "integration-test-agent", "integration-test-session", nil)
		assert.NoError(t, err, "Failed to list contexts")
		assert.NotEmpty(t, contexts, "List of contexts should not be empty")
		assert.Contains(t, extractIDs(contexts), createdContext.ID, "List should contain created context ID")

		// Test SearchInContext
		searchResults, err := cm.SearchInContext(ctx, createdContext.ID, "Test")
		assert.NoError(t, err, "Failed to search in context")
		assert.Len(t, searchResults, 1, "Search should return 1 result")
		assert.Contains(t, searchResults[0].Content, "Test", "Search result should contain the search term")

		// Test DeleteContext
		err = cm.DeleteContext(ctx, createdContext.ID)
		assert.NoError(t, err, "Failed to delete context")

		// Verify context is deleted
		_, err = cm.GetContext(ctx, createdContext.ID)
		assert.Error(t, err, "GetContext should return an error for deleted context")
	})
}

// Helper function to extract IDs from a slice of contexts
func extractIDs(contexts []*mcp.Context) []string {
	ids := make([]string, len(contexts))
	for i, c := range contexts {
		ids[i] = c.ID
	}
	return ids
}
