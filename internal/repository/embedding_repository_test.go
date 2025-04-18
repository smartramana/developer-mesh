package repository

import (
	"context"
	"testing"
	"time"
	
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func TestStoreEmbedding(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()
	
	// Create sqlx.DB from the mock connection
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	
	// Create the repository with the mock database
	repo := NewEmbeddingRepository(sqlxDB)
	
	// Test embedding
	testEmbedding := &Embedding{
		ContextID:    "context-123",
		ContentIndex: 2,
		Text:         "This is a test text for embedding",
		Embedding:    []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		ModelID:      "test-model",
	}
	
	// Expected vector string (approximate format)
	vectorStr := "[0.100000,0.200000,0.300000,0.400000,0.500000]"
	
	// Set up the expected SQL query and result
	mock.ExpectQuery(`INSERT INTO mcp.embeddings`).
		WithArgs(testEmbedding.ContextID, testEmbedding.ContentIndex, testEmbedding.Text, vectorStr, 5, testEmbedding.ModelID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("embedding-456"))
	
	// Call the method being tested
	err = repo.StoreEmbedding(context.Background(), testEmbedding)
	
	// Assert expectations
	assert.NoError(t, err)
	assert.Equal(t, "embedding-456", testEmbedding.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSearchEmbeddings(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()
	
	// Create sqlx.DB from the mock connection
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	
	// Create the repository with the mock database
	repo := NewEmbeddingRepository(sqlxDB)
	
	// Test query parameters
	contextID := "context-123"
	queryVector := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	limit := 5
	
	// Expected vector string (approximate format)
	vectorStr := "[0.100000,0.200000,0.300000,0.400000,0.500000]"
	
	// Create time for test data
	now := time.Now()
	
	// Create columns to match our custom query in the implementation
	columns := []string{"id", "context_id", "content_index", "text", "embedding", "vector_dimensions", "model_id", "created_at"}
	
	// Create mock rows
	rows := sqlmock.NewRows(columns).
		AddRow("embedding-1", "context-123", 1, "Text 1", "{0.1,0.2,0.3,0.4,0.5}", 5, "model-1", now).
		AddRow("embedding-2", "context-123", 2, "Text 2", "{0.5,0.4,0.3,0.2,0.1}", 5, "model-1", now)
	
	// Use QueryxContext for the updated implementation
	mock.ExpectQuery(`SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at FROM mcp.embeddings WHERE context_id = \$1 AND vector_dimensions = \$2 ORDER BY embedding <-> \$3 LIMIT \$4`).
		WithArgs(contextID, 5, vectorStr, limit).
		WillReturnRows(rows)
	
	// Call the method being tested
	embeddings, err := repo.SearchEmbeddings(context.Background(), queryVector, contextID, limit)
	
	// Add a specific check to avoid panic if no embeddings are returned
	if assert.NoError(t, err, "Search should not return an error") {
		if assert.NotEmpty(t, embeddings, "Should have returned embeddings") {
			// Only proceed with assertions if we have embeddings
			assert.Equal(t, "embedding-1", embeddings[0].ID)
			assert.Equal(t, "embedding-2", embeddings[1].ID)
			
			// Check embeddings were properly parsed from the string format
			if assert.NotEmpty(t, embeddings[0].Embedding) {
				assert.InDelta(t, 0.1, float64(embeddings[0].Embedding[0]), 0.001)
			}
		}
	}
	
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetContextEmbeddings(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()
	
	// Create sqlx.DB from the mock connection
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	
	// Create the repository with the mock database
	repo := NewEmbeddingRepository(sqlxDB)
	
	// Test parameters
	contextID := "context-123"
	
	// Create time for test data
	now := time.Now()
	
	// Create columns to match our custom query in the implementation
	columns := []string{"id", "context_id", "content_index", "text", "embedding", "vector_dimensions", "model_id", "created_at"}
	
	// Create mock rows with properly formatted embeddings
	rows := sqlmock.NewRows(columns).
		AddRow("embedding-1", contextID, 1, "Text 1", "{0.1,0.2,0.3,0.4,0.5}", 5, "model-1", now).
		AddRow("embedding-2", contextID, 2, "Text 2", "{0.5,0.4,0.3,0.2,0.1}", 5, "model-1", now)
	
	// Set up the expected SQL query and result
	mock.ExpectQuery(`SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at FROM mcp.embeddings WHERE context_id = \$1`).
		WithArgs(contextID).
		WillReturnRows(rows)
	
	// Call the method being tested
	embeddings, err := repo.GetContextEmbeddings(context.Background(), contextID)
	
	// Assert expectations
	assert.NoError(t, err)
	assert.Len(t, embeddings, 2)
	assert.Equal(t, contextID, embeddings[0].ContextID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteContextEmbeddings(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()
	
	// Create sqlx.DB from the mock connection
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	
	// Create the repository with the mock database
	repo := NewEmbeddingRepository(sqlxDB)
	
	// Test parameters
	contextID := "context-123"
	
	// Set up the expected SQL query and result
	mock.ExpectExec(`DELETE FROM mcp.embeddings WHERE context_id = (.*)`).
		WithArgs(contextID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	
	// Call the method being tested
	err = repo.DeleteContextEmbeddings(context.Background(), contextID)
	
	// Assert expectations
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
