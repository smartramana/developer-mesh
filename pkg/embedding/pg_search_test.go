package embedding

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPgSearchService(t *testing.T) {
	// Create a mock DB
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Create a mock embedding service
	mockEmbService := &mockEmbeddingService{
		dimensions: 3,
	}

	// Test case: pgvector extension is not installed
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	_, err = NewPgSearchService(&PgSearchConfig{
		DB:               db,
		Schema:           "mcp",
		EmbeddingService: mockEmbService,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pgvector extension is not installed")

	// Test case: successful creation
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	svc, err := NewPgSearchService(&PgSearchConfig{
		DB:               db,
		Schema:           "mcp",
		EmbeddingService: mockEmbService,
	})
	assert.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, "mcp", svc.schema)
	assert.Equal(t, mockEmbService, svc.embeddingService)
}

func TestPgSearchService_Search(t *testing.T) {
	// Create a mock DB
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup the mock embedding service
	mockEmbService := &mockEmbeddingService{
		dimensions: 3,
		mockVector: []float32{0.1, 0.2, 0.3},
	}

	// Setup the mock for extension check
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Create the search service
	svc, err := NewPgSearchService(&PgSearchConfig{
		DB:               db,
		Schema:           "mcp",
		EmbeddingService: mockEmbService,
	})
	require.NoError(t, err)

	// Setup the mock for search query
	rows := sqlmock.NewRows([]string{
		"id", "context_id", "content_index", "text", "embedding",
		"vector_dimensions", "model_id", "metadata", "content_type", "similarity",
	}).AddRow(
		"code:test123", sql.NullString{String: "", Valid: false}, 0,
		sql.NullString{String: "", Valid: false}, "[0.1,0.2,0.3]",
		3, "test-model", sql.NullString{String: "{}", Valid: true},
		"code", 0.95,
	)

	// We need to use MatchAny because the actual SQL is complex and we just want to test the flow
	mock.ExpectQuery("SELECT (.+) FROM mcp.embeddings").WillReturnRows(rows)

	// Perform the search
	ctx := context.Background()
	results, err := svc.Search(ctx, "test query", nil)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, "test123", results.Results[0].Content.ContentID)
	assert.Equal(t, "code", results.Results[0].Content.ContentType)
	assert.InDelta(t, 0.95, results.Results[0].Score, 0.001)
}

func TestPgSearchService_SearchByVector(t *testing.T) {
	// Create a mock DB
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup the mock embedding service
	mockEmbService := &mockEmbeddingService{
		dimensions: 3,
	}

	// Setup the mock for extension check
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Create the search service
	svc, err := NewPgSearchService(&PgSearchConfig{
		DB:               db,
		Schema:           "mcp",
		EmbeddingService: mockEmbService,
	})
	require.NoError(t, err)

	// Setup the mock for search query
	rows := sqlmock.NewRows([]string{
		"id", "context_id", "content_index", "text", "embedding",
		"vector_dimensions", "model_id", "metadata", "content_type", "similarity",
	}).AddRow(
		"issue:123", sql.NullString{String: "", Valid: false}, 0,
		sql.NullString{String: "", Valid: false}, "[0.4,0.5,0.6]",
		3, "test-model", sql.NullString{String: "{\"title\":\"Test Issue\"}", Valid: true},
		"issue", 0.85,
	)

	// Mock the query execution
	mock.ExpectQuery("SELECT (.+) FROM mcp.embeddings").WillReturnRows(rows)

	// Perform the search with a vector
	ctx := context.Background()
	vector := []float32{0.4, 0.5, 0.6}
	options := &SearchOptions{
		ContentTypes:  []string{"issue"},
		MinSimilarity: 0.7,
		Limit:         10,
	}

	results, err := svc.SearchByVector(ctx, vector, options)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, "123", results.Results[0].Content.ContentID)
	assert.Equal(t, "issue", results.Results[0].Content.ContentType)
	assert.InDelta(t, 0.85, results.Results[0].Score, 0.001)
}

func TestPgSearchService_SearchByContentID(t *testing.T) {
	// Create a mock DB
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup the mock embedding service
	mockEmbService := &mockEmbeddingService{
		dimensions: 3,
	}

	// Setup the mock for extension check
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Create the search service
	svc, err := NewPgSearchService(&PgSearchConfig{
		DB:               db,
		Schema:           "mcp",
		EmbeddingService: mockEmbService,
	})
	require.NoError(t, err)

	// Mock for getting the embedding for the content ID
	mock.ExpectQuery("SELECT embedding::text").WillReturnRows(
		sqlmock.NewRows([]string{"embedding"}).AddRow("[0.7,0.8,0.9]"),
	)

	// Setup the mock for search query
	rows := sqlmock.NewRows([]string{
		"id", "context_id", "content_index", "text", "embedding",
		"vector_dimensions", "model_id", "metadata", "content_type", "similarity",
	}).AddRow(
		"pr:456", sql.NullString{String: "", Valid: false}, 0,
		sql.NullString{String: "", Valid: false}, "[0.7,0.8,0.9]",
		3, "test-model", sql.NullString{String: "{\"title\":\"Similar PR\"}", Valid: true},
		"pr", 0.92,
	)

	// Mock the query execution for the search
	mock.ExpectQuery("SELECT (.+) FROM mcp.embeddings").WillReturnRows(rows)

	// Perform the search by content ID
	ctx := context.Background()
	options := &SearchOptions{
		MinSimilarity: 0.8,
		Limit:         5,
	}

	results, err := svc.SearchByContentID(ctx, "code:original123", options)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, "456", results.Results[0].Content.ContentID)
	assert.Equal(t, "pr", results.Results[0].Content.ContentType)
	assert.InDelta(t, 0.92, results.Results[0].Score, 0.001)
}

// mockEmbeddingService is a mock implementation of EmbeddingService for testing
type mockEmbeddingService struct {
	dimensions int
	mockVector []float32
	config     ModelConfig
}

func (m *mockEmbeddingService) GenerateEmbedding(ctx context.Context, text string, contentType string, contentID string) (*EmbeddingVector, error) {
	vector := m.mockVector
	if vector == nil {
		// Generate a simple deterministic vector based on the text
		vector = make([]float32, m.dimensions)
		for i := 0; i < m.dimensions && i < len(text); i++ {
			vector[i] = float32(text[i%len(text)]) / 255.0
		}
	}

	return &EmbeddingVector{
		Vector:      vector,
		Dimensions:  m.dimensions,
		ModelID:     "test-model",
		ContentType: contentType,
		ContentID:   contentID,
		Metadata:    make(map[string]interface{}),
	}, nil
}

func (m *mockEmbeddingService) BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error) {
	result := make([]*EmbeddingVector, len(texts))
	for i, text := range texts {
		contentID := ""
		if i < len(contentIDs) {
			contentID = contentIDs[i]
		}
		emb, _ := m.GenerateEmbedding(ctx, text, contentType, contentID)
		result[i] = emb
	}
	return result, nil
}

func (m *mockEmbeddingService) GetModelConfig() ModelConfig {
	if m.config.Name == "" {
		return ModelConfig{
			Type:       ModelTypeOpenAI,
			Name:       "text-embedding-3-small",
			Dimensions: m.dimensions,
		}
	}
	return m.config
}

func (m *mockEmbeddingService) GetModelDimensions() int {
	return m.dimensions
}
