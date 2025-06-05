package embedding

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSearchServiceV2(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repository := &Repository{}
	dimensionAdapter := NewDimensionAdapter()

	service := NewSearchServiceV2(db, repository, dimensionAdapter)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.db)
	assert.Equal(t, repository, service.repository)
	assert.Equal(t, dimensionAdapter, service.dimensionAdapter)
}

func TestCrossModelSearchValidation(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	service := NewSearchServiceV2(db, &Repository{}, NewDimensionAdapter())

	t.Run("empty query and embedding", func(t *testing.T) {
		req := CrossModelSearchRequest{
			TenantID: uuid.New(),
		}

		_, err := service.CrossModelSearch(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query or query_embedding must be provided")
	})

	t.Run("valid request with defaults", func(t *testing.T) {
		req := CrossModelSearchRequest{
			Query:    "test query",
			TenantID: uuid.New(),
		}

		// Mock database response
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() {
			_ = db.Close()
		}()

		service := NewSearchServiceV2(db, &Repository{}, NewDimensionAdapter())

		// Expect the search query
		mock.ExpectQuery("WITH normalized_embeddings AS").
			WithArgs(StandardDimension, pq.Array([]float32{}), req.TenantID, 0.7, 10).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "context_id", "content", "original_model", "original_dimension",
				"embedding", "similarity", "agent_id", "metadata", "created_at",
			}))

		_, err = service.CrossModelSearch(context.Background(), req)
		assert.NoError(t, err)
	})
}

func TestCrossModelSearchResults(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	service := NewSearchServiceV2(db, &Repository{}, NewDimensionAdapter())

	tenantID := uuid.New()
	contextID := uuid.New()
	req := CrossModelSearchRequest{
		Query:         "test query",
		QueryEmbedding: []float32{0.1, 0.2, 0.3},
		TenantID:      tenantID,
		Limit:         5,
		MinSimilarity: 0.8,
	}

	// Mock search results
	rows := sqlmock.NewRows([]string{
		"id", "context_id", "content", "original_model", "original_dimension",
		"embedding", "similarity", "agent_id", "metadata", "created_at",
	}).
		AddRow(
			uuid.New(), contextID, "Test content 1", "text-embedding-3-small", 1536,
			pq.Array([]float32{0.1, 0.2, 0.3}), 0.95, "agent-001",
			`{"key": "value1"}`, "2025-01-06T10:00:00Z",
		).
		AddRow(
			uuid.New(), contextID, "Test content 2", "text-embedding-3-large", 3072,
			pq.Array([]float32{0.2, 0.3, 0.4}), 0.90, "agent-002",
			`{"key": "value2"}`, "2025-01-06T11:00:00Z",
		)

	mock.ExpectQuery("WITH normalized_embeddings AS").
		WithArgs(StandardDimension, pq.Array(req.QueryEmbedding), req.TenantID, req.MinSimilarity, req.Limit).
		WillReturnRows(rows)

	results, err := service.CrossModelSearch(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Check first result
	assert.Equal(t, "Test content 1", results[0].Content)
	assert.Equal(t, "text-embedding-3-small", results[0].OriginalModel)
	assert.Equal(t, 1536, results[0].OriginalDimension)
	assert.Equal(t, 0.95, results[0].RawSimilarity)
	assert.Equal(t, "agent-001", results[0].AgentID)
	assert.Equal(t, "value1", results[0].Metadata["key"])

	// Check second result
	assert.Equal(t, "Test content 2", results[1].Content)
	assert.Equal(t, "text-embedding-3-large", results[1].OriginalModel)
	assert.Equal(t, 3072, results[1].OriginalDimension)
	assert.Equal(t, 0.90, results[1].RawSimilarity)
	assert.Equal(t, "agent-002", results[1].AgentID)

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildCrossModelSearchQuery(t *testing.T) {
	service := &SearchServiceV2{}

	t.Run("basic query", func(t *testing.T) {
		req := CrossModelSearchRequest{
			QueryEmbedding: []float32{0.1, 0.2, 0.3},
			TenantID:       uuid.New(),
			MinSimilarity:  0.7,
			Limit:          10,
		}

		query, args := service.buildCrossModelSearchQuery(req, 1536)

		assert.Contains(t, query, "WITH normalized_embeddings AS")
		assert.Contains(t, query, "WHERE e.tenant_id = $3")
		assert.Contains(t, query, "WHERE similarity >= $")
		assert.Contains(t, query, "ORDER BY similarity DESC")
		assert.Contains(t, query, "LIMIT $")

		assert.Len(t, args, 5)
		assert.Equal(t, 1536, args[0])
		assert.Equal(t, req.TenantID, args[2])
		assert.Equal(t, req.MinSimilarity, args[3])
		assert.Equal(t, req.Limit, args[4])
	})

	t.Run("with filters", func(t *testing.T) {
		contextID := uuid.New()
		req := CrossModelSearchRequest{
			QueryEmbedding: []float32{0.1, 0.2, 0.3},
			TenantID:       uuid.New(),
			ContextID:      &contextID,
			IncludeModels:  []string{"model1", "model2"},
			ExcludeModels:  []string{"model3"},
			IncludeAgents:  []string{"agent1"},
			ExcludeAgents:  []string{"agent2"},
			MetadataFilter: map[string]interface{}{"key": "value"},
			TimeRangeFilter: &TimeRangeFilter{
				StartTime: "2025-01-01T00:00:00Z",
				EndTime:   "2025-01-31T23:59:59Z",
			},
			MinSimilarity: 0.8,
			Limit:         20,
		}

		query, args := service.buildCrossModelSearchQuery(req, 768)

		// Check that all filters are included
		assert.Contains(t, query, "AND e.context_id = $")
		assert.Contains(t, query, "AND e.model_name = ANY($")
		assert.Contains(t, query, "AND e.model_name != ALL($")
		assert.Contains(t, query, "AND e.metadata->>'agent_id' = ANY($")
		assert.Contains(t, query, "AND e.metadata->>'agent_id' != ALL($")
		assert.Contains(t, query, "AND e.metadata @> $")
		assert.Contains(t, query, "AND e.created_at >= $")
		assert.Contains(t, query, "AND e.created_at <= $")

		// Verify args count
		assert.Greater(t, len(args), 10)
	})
}

func TestNormalizeScore(t *testing.T) {
	service := &SearchServiceV2{}

	t.Run("same model and dimension", func(t *testing.T) {
		score := service.normalizeScore(0.95, "text-embedding-3-small", "text-embedding-3-small", 1536, 1536)
		assert.Equal(t, 0.95, score)
	})

	t.Run("different dimensions penalty", func(t *testing.T) {
		score := service.normalizeScore(0.95, "text-embedding-3-large", "text-embedding-3-small", 3072, 1536)
		// Should apply dimension penalty
		assert.Less(t, score, 0.95)
		assert.Greater(t, score, 0.85)
	})

	t.Run("cross-model calibration", func(t *testing.T) {
		score := service.normalizeScore(0.90, "text-embedding-3-small", "voyage-2", 1536, 1536)
		// Should apply cross-model calibration
		assert.Less(t, score, 0.90)
		assert.Greater(t, score, 0.80)
	})

	t.Run("clamp to valid range", func(t *testing.T) {
		// Test upper bound
		score := service.normalizeScore(1.5, "model1", "model2", 1536, 1536)
		assert.LessOrEqual(t, score, 1.0)

		// Test lower bound
		score = service.normalizeScore(-0.5, "model1", "model2", 1536, 1536)
		assert.GreaterOrEqual(t, score, 0.0)
	})
}

func TestGetModelQualityScore(t *testing.T) {
	service := &SearchServiceV2{}

	tests := []struct {
		model         string
		expectedScore float64
	}{
		{"text-embedding-3-large", 0.95},
		{"text-embedding-3-small", 0.90},
		{"text-embedding-ada-002", 0.85},
		{"voyage-code-2", 0.92},
		{"amazon.titan-embed-text-v2:0", 0.87},
		{"unknown-model", 0.80},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			score := service.getModelQualityScore(tt.model)
			assert.Equal(t, tt.expectedScore, score)
		})
	}
}

func TestCalculateFinalScore(t *testing.T) {
	service := &SearchServiceV2{}

	t.Run("research task type", func(t *testing.T) {
		score := service.calculateFinalScore(0.9, 0.8, "research")
		// Research: 60% similarity, 40% quality
		expected := 0.6*0.9 + 0.4*0.8
		assert.Equal(t, expected, score)
	})

	t.Run("code analysis task type", func(t *testing.T) {
		score := service.calculateFinalScore(0.9, 0.8, "code_analysis")
		// Code: 70% similarity, 30% quality
		expected := 0.7*0.9 + 0.3*0.8
		assert.Equal(t, expected, score)
	})

	t.Run("default task type", func(t *testing.T) {
		score := service.calculateFinalScore(0.9, 0.8, "unknown")
		// Default: 80% similarity, 20% quality
		expected := 0.8*0.9 + 0.2*0.8
		assert.Equal(t, expected, score)
	})
}

func TestHybridSearch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	service := NewSearchServiceV2(db, &Repository{}, NewDimensionAdapter())

	req := HybridSearchRequest{
		Query:        "test query",
		Keywords:     []string{"test", "query"},
		TenantID:     uuid.New(),
		Limit:        10,
		HybridWeight: 0.7, // 70% semantic, 30% keyword
	}

	// Mock semantic search results
	semanticRows := sqlmock.NewRows([]string{
		"id", "context_id", "content", "original_model", "original_dimension",
		"embedding", "similarity", "agent_id", "metadata", "created_at",
	}).
		AddRow(
			uuid.New(), uuid.New(), "Semantic result 1", "text-embedding-3-small", 1536,
			pq.Array([]float32{0.1, 0.2}), 0.95, "agent-001", `{}`, "2025-01-06T10:00:00Z",
		)

	// Mock keyword search results
	keywordRows := sqlmock.NewRows([]string{
		"id", "context_id", "content", "model_name", "model_dimensions",
		"metadata", "created_at", "agent_id", "rank",
	}).
		AddRow(
			uuid.New(), uuid.New(), "Keyword result 1", "text-embedding-3-small", 1536,
			`{}`, "2025-01-06T10:00:00Z", "agent-002", 3.5,
		)

	// Expect semantic search query
	mock.ExpectQuery("WITH normalized_embeddings AS").
		WillReturnRows(semanticRows)

	// Expect keyword search query
	mock.ExpectQuery("SELECT(.|\n)*FROM mcp.embeddings e").
		WithArgs("test & query", req.TenantID, req.Limit*2).
		WillReturnRows(keywordRows)

	results, err := service.HybridSearch(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Check hybrid scores are calculated
	for _, result := range results {
		assert.True(t, result.HybridScore >= 0 && result.HybridScore <= 1)
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMergeResults(t *testing.T) {
	service := &SearchServiceV2{}

	t.Run("merge without duplicates", func(t *testing.T) {
		semantic := []HybridSearchResult{
			{
				CrossModelSearchResult: CrossModelSearchResult{
					ID:         uuid.New(),
					Content:    "Result 1",
					FinalScore: 0.9,
				},
				SemanticScore: 0.9,
			},
		}

		keyword := []HybridSearchResult{
			{
				CrossModelSearchResult: CrossModelSearchResult{
					ID:      uuid.New(),
					Content: "Result 2",
				},
				KeywordScore: 0.8,
			},
		}

		merged := service.mergeResults(semantic, keyword, 0.6)

		assert.Len(t, merged, 2)
		// Check hybrid scores
		assert.Equal(t, 0.6*0.9, merged[0].HybridScore)      // Semantic only
		assert.Equal(t, (1-0.6)*0.8, merged[1].HybridScore)  // Keyword only
	})

	t.Run("merge with duplicates", func(t *testing.T) {
		sharedID := uuid.New()

		semantic := []HybridSearchResult{
			{
				CrossModelSearchResult: CrossModelSearchResult{
					ID:         sharedID,
					Content:    "Shared result",
					FinalScore: 0.9,
				},
				SemanticScore: 0.9,
			},
		}

		keyword := []HybridSearchResult{
			{
				CrossModelSearchResult: CrossModelSearchResult{
					ID:      sharedID,
					Content: "Shared result",
				},
				KeywordScore: 0.8,
			},
		}

		merged := service.mergeResults(semantic, keyword, 0.5)

		assert.Len(t, merged, 1)
		// Combined score
		assert.Equal(t, 0.5*0.9+0.5*0.8, merged[0].HybridScore)
		assert.Equal(t, 0.9, merged[0].SemanticScore)
		assert.Equal(t, 0.8, merged[0].KeywordScore)
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("getModelFamily", func(t *testing.T) {
		tests := []struct {
			model    string
			expected string
		}{
			{"text-embedding-3-small", "openai"},
			{"text-embedding-ada-002", "openai"},
			{"voyage-2", "voyage"},
			{"voyage-code-2", "voyage"},
			{"amazon.titan-embed-text-v2:0", "bedrock"},
			{"anthropic.claude-instant-v1", "bedrock"},
			{"cohere.embed-english-v3", "cohere"},
			{"unknown-model", "unknown"},
		}

		for _, tt := range tests {
			t.Run(tt.model, func(t *testing.T) {
				family := getModelFamily(tt.model)
				assert.Equal(t, tt.expected, family)
			})
		}
	})

	t.Run("buildTsQuery", func(t *testing.T) {
		tests := []struct {
			keywords []string
			expected string
		}{
			{[]string{}, ""},
			{[]string{"test"}, "test"},
			{[]string{"test", "query"}, "test & query"},
			{[]string{"foo", "bar", "baz"}, "foo & bar & baz"},
		}

		for _, tt := range tests {
			t.Run(tt.expected, func(t *testing.T) {
				query := buildTsQuery(tt.keywords)
				assert.Equal(t, tt.expected, query)
			})
		}
	})
}

// Benchmark tests
func BenchmarkCrossModelSearch(b *testing.B) {
	db, mock, _ := sqlmock.New()
	defer func() {
		_ = db.Close()
	}()

	service := NewSearchServiceV2(db, &Repository{}, NewDimensionAdapter())

	req := CrossModelSearchRequest{
		Query:         "benchmark query",
		QueryEmbedding: make([]float32, 1536),
		TenantID:      uuid.New(),
		Limit:         50,
		MinSimilarity: 0.7,
	}

	// Mock empty results to avoid complex setup
	rows := sqlmock.NewRows([]string{
		"id", "context_id", "content", "original_model", "original_dimension",
		"embedding", "similarity", "agent_id", "metadata", "created_at",
	})

	mock.ExpectQuery("WITH normalized_embeddings AS").
		WillReturnRows(rows).
		RowsWillBeClosed()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.CrossModelSearch(ctx, req)
	}
}

func BenchmarkNormalizeScore(b *testing.B) {
	service := &SearchServiceV2{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.normalizeScore(0.95, "text-embedding-3-large", "text-embedding-3-small", 3072, 1536)
	}
}