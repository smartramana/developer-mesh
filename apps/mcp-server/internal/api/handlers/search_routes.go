// Package handlers provides RESTful API handlers for the MCP server
//
// DEPRECATED: This file contains the original search routes implementation
// which has been migrated to the new architecture. New code should use the
// SearchHandler in search_handlers.go directly with its RegisterRoutes method.
// This file is maintained only for backward compatibility during the migration
// and will be removed in a future release.
//
// Migration path:
// 1. Use the SearchHandler struct from search_handlers.go
// 2. Create a new instance with NewSearchHandler(searchService)
// 3. Register routes directly with searchHandler.RegisterRoutes(router)
//
// Example:
//   searchService := embedding.NewPgSearchService(config)
//   searchHandler := handlers.NewSearchHandler(searchService)
//   searchHandler.RegisterRoutes(apiV1)
package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
)

// SetupSearchRoutes registers the advanced vector search routes
func SetupSearchRoutes(
	group *gin.RouterGroup,
	db *sql.DB,
	cfg interface{},
	logger observability.Logger,
) error {
	// Skip if vector operations are disabled
	isEnabled := true
	if cfg != nil {
		// Configuration structure check
		type VectorConfig struct {
			Enabled bool
		}
		type DatabaseConfig struct {
			Vector VectorConfig
		}
		type Config struct {
			Database DatabaseConfig
		}
		if config, ok := cfg.(Config); ok {
			isEnabled = config.Database.Vector.Enabled
		}
	}

	if !isEnabled {
		logger.Info("Vector search API routes not registered (vector operations disabled)", nil)
		return nil
	}

	// Create embedding service based on the default configured model
	embeddingService, err := createDefaultEmbeddingService()
	if err != nil {
		logger.Warn("Failed to create embedding service for search API", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Create the PostgreSQL search service
	pgSearchService, err := createSearchService(db, embeddingService)
	if err != nil {
		logger.Warn("Failed to create search service", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Create the search handler
	searchHandler := NewSearchHandler(pgSearchService)

	// Register routes directly 
	searchHandler.RegisterRoutes(group)

	// Add basic documentation for the API
	searchGroup := group.Group("/search")
	searchGroup.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"endpoints": []gin.H{
				{
					"path":        "/api/v1/search",
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

	logger.Info("Vector search API routes registered", map[string]interface{}{
		"path": "/api/v1/search",
	})

	return nil
}

// createSearchService creates a new search service for vector similarity search
func createSearchService(db *sql.DB, embeddingService embedding.EmbeddingService) (embedding.SearchService, error) {
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
func createDefaultEmbeddingService() (embedding.EmbeddingService, error) {
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
