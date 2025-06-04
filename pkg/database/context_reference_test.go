package database

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func TestContextReference(t *testing.T) {
	// Create a mock database connection
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer func() {
		mock.ExpectClose()
		if err := mockDB.Close(); err != nil {
			t.Errorf("Failed to close mock database: %v", err)
		}
	}()

	// Wrap in sqlx
	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")

	// Create the database instance
	db := &Database{
		db:         sqlxDB,
		statements: make(map[string]*sqlx.Stmt),
	}

	// Create test data
	ctx := context.Background()
	now := time.Now()
	testContext := &models.Context{
		ID:            "test-id",
		AgentID:       "test-agent",
		ModelID:       "test-model",
		SessionID:     "test-session",
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(24 * time.Hour),
		CurrentTokens: 100,
		Content:       make([]models.ContextItem, 5),
	}

	t.Run("CreateContextReference", func(t *testing.T) {
		// Set up expectations
		mock.ExpectExec("INSERT INTO mcp.context_references").
			WithArgs(
				testContext.ID,
				testContext.AgentID,
				testContext.ModelID,
				testContext.SessionID,
				testContext.CreatedAt,
				testContext.UpdatedAt,
				testContext.ExpiresAt,
				testContext.CurrentTokens,
				len(testContext.Content),
				"contexts/test-id.json",
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Call the function
		err := db.CreateContextReference(ctx, testContext)
		assert.NoError(t, err)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("GetContextReference", func(t *testing.T) {
		// Set up expectations
		columns := []string{"id", "agent_id", "model_id", "session_id", "created_at", "updated_at", "expires_at", "token_count", "message_count", "storage_path"}
		mock.ExpectQuery("SELECT (.+) FROM mcp.context_references WHERE id = \\$1").
			WithArgs("test-id").
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow("test-id", "test-agent", "test-model", "test-session", now, now, now.Add(24*time.Hour), 100, 5, "contexts/test-id.json"))

		// Call the function
		ref, err := db.GetContextReference(ctx, "test-id")
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		assert.Equal(t, "test-id", ref.ID)
		assert.Equal(t, "test-agent", ref.AgentID)
		assert.Equal(t, "test-model", ref.ModelID)
		assert.Equal(t, "test-session", ref.SessionID)
		assert.Equal(t, 100, ref.TokenCount)
		assert.Equal(t, 5, ref.MessageCount)
		assert.Equal(t, "contexts/test-id.json", ref.StoragePath)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("UpdateContextReference", func(t *testing.T) {
		// Set up expectations
		mock.ExpectExec("UPDATE mcp.context_references SET").
			WithArgs(
				testContext.ID,
				testContext.AgentID,
				testContext.ModelID,
				testContext.SessionID,
				testContext.UpdatedAt,
				testContext.ExpiresAt,
				testContext.CurrentTokens,
				len(testContext.Content),
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Call the function
		err := db.UpdateContextReference(ctx, testContext)
		assert.NoError(t, err)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("DeleteContextReference", func(t *testing.T) {
		// Set up expectations
		mock.ExpectExec("DELETE FROM mcp.context_references WHERE id = \\$1").
			WithArgs("test-id").
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Call the function
		err := db.DeleteContextReference(ctx, "test-id")
		assert.NoError(t, err)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("ListContextReferences", func(t *testing.T) {
		// Test with agent ID only
		columns := []string{"id", "agent_id", "model_id", "session_id", "created_at", "updated_at", "expires_at", "token_count", "message_count", "storage_path"}
		mock.ExpectQuery("SELECT (.+) FROM mcp.context_references WHERE 1=1 AND agent_id = \\$1 ORDER BY created_at DESC").
			WithArgs("test-agent").
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow("test-id1", "test-agent", "test-model", "test-session", now, now, now.Add(24*time.Hour), 100, 5, "contexts/test-id1.json").
				AddRow("test-id2", "test-agent", "test-model", "test-session", now, now, now.Add(24*time.Hour), 200, 10, "contexts/test-id2.json"))

		// Call the function
		refs, err := db.ListContextReferences(ctx, "test-agent", "", nil)
		assert.NoError(t, err)
		assert.Len(t, refs, 2)
		assert.Equal(t, "test-id1", refs[0].ID)
		assert.Equal(t, "test-id2", refs[1].ID)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)

		// Test with agent ID and session ID
		mock.ExpectQuery("SELECT (.+) FROM mcp.context_references WHERE 1=1 AND agent_id = \\$1 AND session_id = \\$2 ORDER BY created_at DESC").
			WithArgs("test-agent", "test-session").
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow("test-id1", "test-agent", "test-model", "test-session", now, now, now.Add(24*time.Hour), 100, 5, "contexts/test-id1.json"))

		// Call the function
		refs, err = db.ListContextReferences(ctx, "test-agent", "test-session", nil)
		assert.NoError(t, err)
		assert.Len(t, refs, 1)
		assert.Equal(t, "test-id1", refs[0].ID)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)

		// Test with options
		options := map[string]interface{}{
			"model_id": "test-model",
			"limit":    10,
		}
		mock.ExpectQuery("SELECT (.+) FROM mcp.context_references WHERE 1=1 AND agent_id = \\$1 AND model_id = \\$2 ORDER BY created_at DESC LIMIT \\$3").
			WithArgs("test-agent", "test-model", 10).
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow("test-id1", "test-agent", "test-model", "test-session", now, now, now.Add(24*time.Hour), 100, 5, "contexts/test-id1.json"))

		// Call the function
		refs, err = db.ListContextReferences(ctx, "test-agent", "", options)
		assert.NoError(t, err)
		assert.Len(t, refs, 1)
		assert.Equal(t, "test-id1", refs[0].ID)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)

		// Test with created_after
		createdAfter := now.Add(-1 * time.Hour)
		options = map[string]interface{}{
			"created_after": createdAfter,
		}
		mock.ExpectQuery("SELECT (.+) FROM mcp.context_references WHERE 1=1 AND agent_id = \\$1 AND created_at > \\$2 ORDER BY created_at DESC").
			WithArgs("test-agent", createdAfter).
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow("test-id1", "test-agent", "test-model", "test-session", now, now, now.Add(24*time.Hour), 100, 5, "contexts/test-id1.json"))

		// Call the function
		refs, err = db.ListContextReferences(ctx, "test-agent", "", options)
		assert.NoError(t, err)
		assert.Len(t, refs, 1)
		assert.Equal(t, "test-id1", refs[0].ID)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("CreateContextReferenceTable", func(t *testing.T) {
		// Set up expectations
		mock.ExpectExec("CREATE SCHEMA IF NOT EXISTS mcp").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE TABLE IF NOT EXISTS mcp.context_references").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_context_agent_id").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_context_session_id").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_context_created_at").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_context_expires_at").
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Call the function
		err := db.CreateContextReferenceTable(ctx)
		assert.NoError(t, err)

		// Verify expectations
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}
