package repository

import (
	"context"
	"fmt"
	"testing"
	"time"
	
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreEmbedding(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name           string
		embedding      *Embedding
		setupMock      func(mock sqlmock.Sqlmock, embedding *Embedding)
		expectedID     string
		expectedError  bool
		errorContains  string
	}{
		{
			name: "successful storage",
			embedding: &Embedding{
				ContextID:    "context-123",
				ContentIndex: 2,
				Text:         "This is a test text for embedding",
				Embedding:    []float32{0.1, 0.2, 0.3, 0.4, 0.5},
				ModelID:      "test-model",
			},
			setupMock: func(mock sqlmock.Sqlmock, embedding *Embedding) {
				vectorStr := "[0.100000,0.200000,0.300000,0.400000,0.500000]"
				mock.ExpectQuery(`INSERT INTO mcp.embeddings`).
					WithArgs(embedding.ContextID, embedding.ContentIndex, embedding.Text, vectorStr, 5, embedding.ModelID).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("embedding-456"))
			},
			expectedID:    "embedding-456",
			expectedError: false,
		},
		{
			name: "database error",
			embedding: &Embedding{
				ContextID:    "context-error",
				ContentIndex: 1,
				Text:         "This will cause an error",
				Embedding:    []float32{0.1, 0.2, 0.3},
				ModelID:      "test-model",
			},
			setupMock: func(mock sqlmock.Sqlmock, embedding *Embedding) {
				vectorStr := "[0.100000,0.200000,0.300000]"
				mock.ExpectQuery(`INSERT INTO mcp.embeddings`).
					WithArgs(embedding.ContextID, embedding.ContentIndex, embedding.Text, vectorStr, 3, embedding.ModelID).
					WillReturnError(fmt.Errorf("database error"))
			},
			expectedID:    "",
			expectedError: true,
			errorContains: "failed to store embedding",
		},
		{
			name: "empty embedding vector",
			embedding: &Embedding{
				ContextID:    "context-empty",
				ContentIndex: 3,
				Text:         "Empty embedding vector",
				Embedding:    []float32{},
				ModelID:      "test-model",
			},
			setupMock: func(mock sqlmock.Sqlmock, embedding *Embedding) {
				vectorStr := "[]"
				mock.ExpectQuery(`INSERT INTO mcp.embeddings`).
					WithArgs(embedding.ContextID, embedding.ContentIndex, embedding.Text, vectorStr, 0, embedding.ModelID).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("embedding-empty"))
			},
			expectedID:    "embedding-empty",
			expectedError: false,
		},
	}
	
	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock database connection
			db, mock, err := sqlmock.New()
			require.NoError(t, err, "Error creating mock database")
			defer db.Close()
			
			// Create sqlx.DB from the mock connection
			sqlxDB := sqlx.NewDb(db, "sqlmock")
			
			// Create the repository with the mock database
			repo := NewEmbeddingRepository(sqlxDB)
			
			// Setup mock expectations
			tc.setupMock(mock, tc.embedding)
			
			// Call the method being tested
			err = repo.StoreEmbedding(context.Background(), tc.embedding)
			
			// Assert error expectations
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedID, tc.embedding.ID)
			}
			
			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSearchEmbeddings(t *testing.T) {
	// Create time for test data
	now := time.Now()
	
	// Define columns to match our custom query in the implementation
	columns := []string{"id", "context_id", "content_index", "text", "embedding", "vector_dimensions", "model_id", "created_at"}
	
	// Define test cases
	testCases := []struct {
		name           string
		contextID      string
		queryVector    []float32
		limit          int
		setupMock      func(mock sqlmock.Sqlmock, vectorStr string, contextID string, limit int)
		expectedError  bool
		errorContains  string
		validateResult func(t *testing.T, embeddings []*Embedding)
	}{
		{
			name:        "successful search with results",
			contextID:   "context-123",
			queryVector: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
			limit:       5,
			setupMock: func(mock sqlmock.Sqlmock, vectorStr string, contextID string, limit int) {
				rows := sqlmock.NewRows(columns).
					AddRow("embedding-1", contextID, 1, "Text 1", "{0.1,0.2,0.3,0.4,0.5}", 5, "model-1", now).
					AddRow("embedding-2", contextID, 2, "Text 2", "{0.5,0.4,0.3,0.2,0.1}", 5, "model-1", now)
				
				mock.ExpectQuery(`SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at FROM mcp.embeddings WHERE context_id = \$1 AND vector_dimensions = \$2 ORDER BY embedding <-> \$3 LIMIT \$4`).
					WithArgs(contextID, 5, vectorStr, limit).
					WillReturnRows(rows)
			},
			expectedError: false,
			validateResult: func(t *testing.T, embeddings []*Embedding) {
				assert.Len(t, embeddings, 2)
				assert.Equal(t, "embedding-1", embeddings[0].ID)
				assert.Equal(t, "embedding-2", embeddings[1].ID)
				
				// Check embeddings were properly parsed from the string format
				assert.NotEmpty(t, embeddings[0].Embedding)
				assert.InDelta(t, 0.1, float64(embeddings[0].Embedding[0]), 0.001)
			},
		},
		{
			name:        "search with no results",
			contextID:   "empty-context",
			queryVector: []float32{0.1, 0.2, 0.3},
			limit:       10,
			setupMock: func(mock sqlmock.Sqlmock, vectorStr string, contextID string, limit int) {
				rows := sqlmock.NewRows(columns) // Empty result set
				
				mock.ExpectQuery(`SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at FROM mcp.embeddings WHERE context_id = \$1 AND vector_dimensions = \$2 ORDER BY embedding <-> \$3 LIMIT \$4`).
					WithArgs(contextID, 3, vectorStr, limit).
					WillReturnRows(rows)
			},
			expectedError: false,
			validateResult: func(t *testing.T, embeddings []*Embedding) {
				assert.Empty(t, embeddings)
			},
		},
		{
			name:        "database error",
			contextID:   "error-context",
			queryVector: []float32{0.1, 0.2},
			limit:       5,
			setupMock: func(mock sqlmock.Sqlmock, vectorStr string, contextID string, limit int) {
				mock.ExpectQuery(`SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at FROM mcp.embeddings WHERE context_id = \$1 AND vector_dimensions = \$2 ORDER BY embedding <-> \$3 LIMIT \$4`).
					WithArgs(contextID, 2, vectorStr, limit).
					WillReturnError(fmt.Errorf("database error"))
			},
			expectedError: true,
			errorContains: "failed to search embeddings",
			validateResult: func(t *testing.T, embeddings []*Embedding) {
				assert.Nil(t, embeddings)
			},
		},
		{
			name:        "corrupted embedding data",
			contextID:   "bad-data-context",
			queryVector: []float32{0.1, 0.2},
			limit:       5,
			setupMock: func(mock sqlmock.Sqlmock, vectorStr string, contextID string, limit int) {
				rows := sqlmock.NewRows(columns).
					AddRow("embedding-bad", contextID, 1, "Text", "not-valid-json", 2, "model-1", now)
				
				mock.ExpectQuery(`SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at FROM mcp.embeddings WHERE context_id = \$1 AND vector_dimensions = \$2 ORDER BY embedding <-> \$3 LIMIT \$4`).
					WithArgs(contextID, 2, vectorStr, limit).
					WillReturnRows(rows)
			},
			expectedError: true,
			errorContains: "failed to parse embedding",
			validateResult: func(t *testing.T, embeddings []*Embedding) {
				assert.Nil(t, embeddings)
			},
		},
	}
	
	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock database connection
			db, mock, err := sqlmock.New()
			require.NoError(t, err, "Error creating mock database")
			defer db.Close()
			
			// Create sqlx.DB from the mock connection
			sqlxDB := sqlx.NewDb(db, "sqlmock")
			
			// Create the repository with the mock database
			repo := NewEmbeddingRepository(sqlxDB)
			
			// Calculate vector string format for the query
			var vectorStr string
			if len(tc.queryVector) > 0 {
				vectorStr = "["
				for i, v := range tc.queryVector {
					if i > 0 {
						vectorStr += ","
					}
					vectorStr += fmt.Sprintf("%f", v)
				}
				vectorStr += "]"
			}
			
			// Setup mock expectations
			tc.setupMock(mock, vectorStr, tc.contextID, tc.limit)
			
			// Call the method being tested
			embeddings, err := repo.SearchEmbeddings(context.Background(), tc.queryVector, tc.contextID, tc.limit)
			
			// Assert error expectations
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
			
			// Validate the result
			tc.validateResult(t, embeddings)
			
			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
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
