package embedding

import (
    "context"
    "encoding/json"
    "testing"
    "time"
    
    "github.com/DATA-DOG/go-sqlmock"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// MockProvider implements the Provider interface for testing
type MockProvider struct {
    GenerateEmbeddingFunc func(ctx context.Context, content string, model string) ([]float32, error)
    SupportedModels       []string
    ValidateAPIKeyFunc    func() error
}

func (m *MockProvider) GenerateEmbedding(ctx context.Context, content string, model string) ([]float32, error) {
    if m.GenerateEmbeddingFunc != nil {
        return m.GenerateEmbeddingFunc(ctx, content, model)
    }
    // Return a dummy embedding
    return make([]float32, 1536), nil
}

func (m *MockProvider) GetSupportedModels() []string {
    if m.SupportedModels != nil {
        return m.SupportedModels
    }
    return []string{"test-model"}
}

func (m *MockProvider) ValidateAPIKey() error {
    if m.ValidateAPIKeyFunc != nil {
        return m.ValidateAPIKeyFunc()
    }
    return nil
}

func TestService_CreateEmbedding(t *testing.T) {
    // Create mock database
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer func() {
        if err := db.Close(); err != nil {
            t.Errorf("Failed to close database: %v", err)
        }
    }()
    
    // Create repository and service
    repo := NewRepository(db)
    service := NewService(repo)
    
    // Register mock provider
    mockProvider := &MockProvider{
        GenerateEmbeddingFunc: func(ctx context.Context, content string, model string) ([]float32, error) {
            return make([]float32, 1536), nil
        },
    }
    service.RegisterProvider("openai", mockProvider)
    
    // Test data
    ctx := context.Background()
    contextID := uuid.New()
    tenantID := uuid.New()
    modelID := uuid.New()
    embeddingID := uuid.New()
    
    req := CreateEmbeddingRequest{
        ContextID:    contextID,
        Content:      "Test content",
        ModelName:    "text-embedding-ada-002",
        TenantID:     tenantID,
        Source:       "test",
        ContentIndex: 0,
        ChunkIndex:   0,
    }
    
    // Mock GetModelByName
    modelRows := sqlmock.NewRows([]string{
        "id", "provider", "model_name", "model_version", "dimensions",
        "max_tokens", "supports_binary", "supports_dimensionality_reduction",
        "min_dimensions", "cost_per_million_tokens", "model_id", "model_type",
        "is_active", "capabilities", "created_at",
    }).AddRow(
        modelID, "openai", "text-embedding-ada-002", "v2", 1536,
        8191, false, false, nil, 0.10, nil, "text",
        true, json.RawMessage("{}"), time.Now(),
    )
    
    mock.ExpectQuery("SELECT (.+) FROM mcp.embedding_models").
        WithArgs("text-embedding-ada-002").
        WillReturnRows(modelRows)
    
    // Mock InsertEmbedding
    mock.ExpectQuery("SELECT mcp.insert_embedding").
        WithArgs(
            contextID,
            "Test content",
            sqlmock.AnyArg(), // embedding array
            "text-embedding-ada-002",
            tenantID,
            sqlmock.AnyArg(), // metadata
            0,                // content_index
            0,                // chunk_index
            nil,              // configured_dimensions
        ).
        WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(embeddingID))
    
    // Execute
    id, err := service.CreateEmbedding(ctx, req)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, embeddingID, id)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestService_SearchSimilar(t *testing.T) {
    // Create mock database
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer func() {
        if err := db.Close(); err != nil {
            t.Errorf("Failed to close database: %v", err)
        }
    }()
    
    // Create repository and service
    repo := NewRepository(db)
    service := NewService(repo)
    
    // Register mock provider
    mockProvider := &MockProvider{
        GenerateEmbeddingFunc: func(ctx context.Context, content string, model string) ([]float32, error) {
            return make([]float32, 1536), nil
        },
    }
    service.RegisterProvider("openai", mockProvider)
    
    // Test data
    ctx := context.Background()
    tenantID := uuid.New()
    contextID := uuid.New()
    modelID := uuid.New()
    resultID := uuid.New()
    
    req := SearchSimilarRequest{
        Query:     "Search query",
        ModelName: "text-embedding-ada-002",
        TenantID:  tenantID,
        ContextID: &contextID,
        Limit:     10,
        Threshold: 0.8,
    }
    
    // Mock GetModelByName
    modelRows := sqlmock.NewRows([]string{
        "id", "provider", "model_name", "model_version", "dimensions",
        "max_tokens", "supports_binary", "supports_dimensionality_reduction",
        "min_dimensions", "cost_per_million_tokens", "model_id", "model_type",
        "is_active", "capabilities", "created_at",
    }).AddRow(
        modelID, "openai", "text-embedding-ada-002", "v2", 1536,
        8191, false, false, nil, 0.10, nil, "text",
        true, json.RawMessage("{}"), time.Now(),
    )
    
    mock.ExpectQuery("SELECT (.+) FROM mcp.embedding_models").
        WithArgs("text-embedding-ada-002").
        WillReturnRows(modelRows)
    
    // Mock SearchEmbeddings
    searchRows := sqlmock.NewRows([]string{
        "id", "context_id", "content", "similarity", "metadata", "model_provider",
    }).AddRow(
        resultID, contextID, "Result content", 0.95,
        json.RawMessage(`{"source": "test"}`), "openai",
    )
    
    mock.ExpectQuery("SELECT \\* FROM mcp.search_embeddings").
        WithArgs(
            sqlmock.AnyArg(), // query embedding
            "text-embedding-ada-002",
            tenantID,
            contextID,
            10,
            0.8,
            sqlmock.AnyArg(), // metadata filter - can be nil or empty slice
        ).
        WillReturnRows(searchRows)
    
    // Execute
    results, err := service.SearchSimilar(ctx, req)
    
    // Assert
    assert.NoError(t, err)
    assert.Len(t, results, 1)
    assert.Equal(t, resultID, results[0].ID)
    assert.Equal(t, "Result content", results[0].Content)
    assert.Equal(t, 0.95, results[0].Similarity)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestService_GetAvailableModels(t *testing.T) {
    // Create mock database
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer func() {
        if err := db.Close(); err != nil {
            t.Errorf("Failed to close database: %v", err)
        }
    }()
    
    // Create repository and service
    repo := NewRepository(db)
    service := NewService(repo)
    
    // Test data
    ctx := context.Background()
    provider := "openai"
    
    filter := ModelFilter{
        Provider: &provider,
    }
    
    // Mock query
    rows := sqlmock.NewRows([]string{
        "provider", "model_name", "model_version", "dimensions", "max_tokens",
        "model_type", "supports_dimensionality_reduction", "min_dimensions", "is_active",
    }).
        AddRow("openai", "text-embedding-3-small", "v3", 1536, 8191, "text", true, 512, true).
        AddRow("openai", "text-embedding-3-large", "v3", 3072, 8191, "text", true, 256, true)
    
    mock.ExpectQuery("SELECT (.+) FROM mcp.get_available_models").
        WithArgs(&provider, nil).
        WillReturnRows(rows)
    
    // Execute
    models, err := service.GetAvailableModels(ctx, filter)
    
    // Assert
    assert.NoError(t, err)
    assert.Len(t, models, 2)
    assert.Equal(t, "text-embedding-3-small", models[0].ModelName)
    assert.Equal(t, 1536, models[0].Dimensions)
    assert.NoError(t, mock.ExpectationsWereMet())
}