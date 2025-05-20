package api

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/gin-gonic/gin"
)

// setupSearchRoutes registers the advanced vector search routes
func (s *Server) setupSearchRoutes(group *gin.RouterGroup) error {
	// Skip if vector operations are disabled
	isEnabled := true
	if s.cfg != nil {
		// Example of how to check if it's enabled in your config
		// Adjust this based on your actual config structure
		type VectorConfig struct {
			Enabled bool
		}
		type DatabaseConfig struct {
			Vector VectorConfig
		}
		type Config struct {
			Database DatabaseConfig
		}
		if cfg, ok := s.cfg.(Config); ok {
			isEnabled = cfg.Database.Vector.Enabled
		}
	}

	if !isEnabled {
		s.logger.Info("Vector search API routes not registered (vector operations disabled)", nil)
		return nil
	}

	// Get the raw SQL database connection
	sqlDB, err := s.db.DB()
	if err != nil {
		s.logger.Error("Failed to get sql.DB from sqlx.DB", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Create embedding service based on the default configured model
	embeddingService, err := s.createDefaultEmbeddingService()
	if err != nil {
		s.logger.Warn("Failed to create embedding service for search API", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Create the PostgreSQL search service
	pgSearchService, err := s.createSearchService(sqlDB, embeddingService)
	if err != nil {
		s.logger.Warn("Failed to create search service", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Create the search handler
	searchHandler := NewSearchHandler(pgSearchService)

	// Group for vector search endpoints
	searchGroup := group.Group("/search")

	// Register routes using the Gin router
	searchGroup.POST("/query", gin.WrapF(searchHandler.HandleSearch))
	searchGroup.GET("/query", gin.WrapF(searchHandler.HandleSearch))
	searchGroup.POST("/vector", gin.WrapF(searchHandler.HandleSearchByVector))
	searchGroup.POST("/similar", gin.WrapF(searchHandler.HandleSearchSimilar))
	searchGroup.GET("/similar", gin.WrapF(searchHandler.HandleSearchSimilar))

	// Add basic documentation for the API
	searchGroup.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"endpoints": []gin.H{
				{
					"path":        "/api/v1/search/query",
					"methods":     []string{"GET", "POST"},
					"description": "Search using a text query",
				},
				{
					"path":        "/api/v1/search/vector",
					"methods":     []string{"POST"},
					"description": "Search using a pre-computed vector",
				},
				{
					"path":        "/api/v1/search/similar",
					"methods":     []string{"GET", "POST"},
					"description": "Find content similar to an existing item",
				},
			},
		})
	})

	s.logger.Info("Vector search API routes registered", map[string]interface{}{
		"path": "/api/v1/search",
	})

	return nil
}

// createSearchService creates a new search service for vector similarity search
func (s *Server) createSearchService(db *sql.DB, embeddingService embedding.EmbeddingService) (embedding.SearchService, error) {
	// Create the search service configuration
	config := &embedding.PgSearchConfig{
		DB:               db,
		Schema:           "mcp", // Default schema, should match what's used elsewhere
		EmbeddingService: embeddingService,
		DefaultLimit:     50,     // Higher default limit for search results
		DefaultMinSimilarity: 0.65, // Lower threshold for better recall
	}

	// Create the search service
	searchService, err := embedding.NewPgSearchService(config)
	if err != nil {
		return nil, err
	}

	return searchService, nil
}

// createDefaultEmbeddingService creates an embedding service using the default configured model
func (s *Server) createDefaultEmbeddingService() (embedding.EmbeddingService, error) {
	// Default to OpenAI embedding model
	modelType := embedding.ModelTypeOpenAI
	modelName := "text-embedding-3-small"

	// Get model dimensions
	dimensions, err := embedding.GetEmbeddingModelDimensions(modelType, modelName)
	if err != nil {
		return nil, err
	}

	// Create a basic mock embedding service for development
	return &mockEmbeddingService{
		modelConfig: embedding.ModelConfig{
			Type:       modelType,
			Name:       modelName,
			Dimensions: dimensions,
		},
		dimensions: dimensions,
	}, nil
}

// mockEmbeddingService is a simple implementation of the EmbeddingService interface for testing
type mockEmbeddingService struct {
	modelConfig embedding.ModelConfig
	dimensions  int
}

// GenerateEmbedding creates a deterministic embedding for testing
func (m *mockEmbeddingService) GenerateEmbedding(ctx context.Context, text string, contentType string, contentID string) (*embedding.EmbeddingVector, error) {
	// Generate a deterministic vector based on the text for testing
	vector := make([]float32, m.dimensions)
	for i := 0; i < m.dimensions && i < len(text); i++ {
		vectorVal := float32(0)
		if i < len(text) {
			vectorVal = float32(text[i]) / 255.0
		}
		vector[i] = vectorVal
	}

	return &embedding.EmbeddingVector{
		Vector:      vector,
		Dimensions:  m.dimensions,
		ModelID:     m.modelConfig.Name,
		ContentType: contentType,
		ContentID:   contentID,
		Metadata:    make(map[string]interface{}),
	}, nil
}

// BatchGenerateEmbeddings creates multiple embeddings
func (m *mockEmbeddingService) BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*embedding.EmbeddingVector, error) {
	result := make([]*embedding.EmbeddingVector, len(texts))
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

// GetModelConfig returns model configuration
func (m *mockEmbeddingService) GetModelConfig() embedding.ModelConfig {
	return m.modelConfig
}

// GetModelDimensions returns the model dimensions
func (m *mockEmbeddingService) GetModelDimensions() int {
	return m.dimensions
}
