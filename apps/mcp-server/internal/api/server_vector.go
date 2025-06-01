package api

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"mcp-server/internal/api/handlers"
	"mcp-server/internal/api/proxies"
)

// SetupVectorAPI initializes and registers the vector API routes
func (s *Server) SetupVectorAPI(ctx context.Context) error {
	logger := s.logger.WithPrefix("vector_api")

	// Check if vector operations are enabled
	isEnabled := true

	// Access configuration fields directly since s.cfg is a *config.Config
	if s.cfg != nil {
		// Type assert Vector field to access Enabled property
		if vectorConfig, ok := s.cfg.Database.Vector.(map[string]interface{}); ok {
			if enabled, ok := vectorConfig["enabled"].(bool); ok {
				isEnabled = enabled
			}
		}
		logger.Info(fmt.Sprintf("Vector operations enabled: %v", isEnabled), nil)
	}

	if !isEnabled {
		logger.Info("Vector operations are disabled", nil)
		return nil
	}

	// Initialize vector database
	var err error
	var vectorDB *database.VectorDatabase

	// Create a vector database connection
	vectorDB, err = database.NewVectorDatabase(s.db, s.cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create vector database: %w", err)
	}

	// Initialize vector database
	if err := vectorDB.Initialize(ctx); err != nil {
		logger.Warn("Vector database initialization failed", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("vector database initialization failed: %w", err)
	}

	// Create repository
	embedRepo := repository.NewEmbeddingRepository(s.db)

	// Create vector API handler
	vectorAPI := proxies.NewVectorAPI(embedRepo, logger)

	// Setup vector routes
	apiV1 := s.router.Group("/api/v1")
	vectorAPI.RegisterRoutes(apiV1)

	logger.Info("Vector API routes registered", map[string]interface{}{
		"path": "/api/v1/vectors",
	})

	// Setup advanced vector search routes using the new SearchHandler pattern
	// Create embedding service - in a real environment this would use the appropriate service
	// This implementation keeps the same mock embedding service that was in the deprecated code
	dimensions := 1536 // Hardcoded for the example, normally would be from embedding.GetEmbeddingModelDimensions

	// Create embedding service for search
	embeddingService := embedding.NewMockEmbeddingService(dimensions)

	// Create a search service
	searchConfig := &embedding.PgSearchConfig{
		DB:                   s.db.DB,
		Schema:               "mcp", // Default schema, should match what's used elsewhere
		EmbeddingService:     embeddingService,
		DefaultLimit:         50,   // Higher default limit for search results
		DefaultMinSimilarity: 0.65, // Lower threshold for better recall
	}

	// Create the search service
	searchService, err := embedding.NewPgSearchService(searchConfig)
	if err != nil {
		logger.Warn("Failed to create search service", map[string]interface{}{
			"error": err.Error(),
		})
		// Non-fatal, continue with other routes
	} else {
		// Create the search handler and register routes
		searchHandler := handlers.NewSearchHandler(searchService)
		searchHandler.RegisterRoutes(apiV1)

		logger.Info("Vector search API routes registered using modern pattern", map[string]interface{}{
			"path":    "/api/v1/search",
			"pattern": "SearchHandler",
		})
	}

	// Add metrics middleware
	vectorRoutes := apiV1.Group("/vectors")
	vectorRoutes.Use(createVectorMetricsMiddleware(s.metrics))

	logger.Info("Vector metrics middleware added", nil)

	return nil
}

// createVectorMetricsMiddleware creates a middleware for vector metrics
func createVectorMetricsMiddleware(metrics observability.MetricsClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process the request
		c.Next()

		// Record metrics
		if metrics != nil {
			operation := "unknown"
			path := c.Request.URL.Path

			if c.Request.Method == "POST" && path == "/api/v1/vectors/store" {
				operation = "store"
			} else if c.Request.Method == "POST" && path == "/api/v1/vectors/search" {
				operation = "search"
			} else if c.Request.Method == "GET" {
				operation = "get"
			} else if c.Request.Method == "DELETE" {
				operation = "delete"
			}

			metrics.RecordCounter("vector_operations_total", 1, map[string]string{
				"operation": operation,
				"status":    fmt.Sprintf("%d", c.Writer.Status()),
			})
		}
	}
}

// Note: Vector routes are now handled by vectorAPI.RegisterRoutes() call in SetupVectorAPI

// Note: Vector routes are now handled by vectorAPI.RegisterRoutes() call
