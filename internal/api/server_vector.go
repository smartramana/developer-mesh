package api

import (
	"context"
	"fmt"
	
	"github.com/S-Corkum/mcp-server/internal/config"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/gin-gonic/gin"
)

// setupVectorAPI initializes and registers the vector API routes
func (s *Server) setupVectorAPI(ctx context.Context) error {
	logger := s.logger.WithPrefix("vector_api")
	
	// Check if vector operations are enabled
	if !s.cfg.Database.Vector.Enabled {
		logger.Info("Vector operations are disabled", nil)
		return nil
	}
	
	// Initialize vector database
	vectorDB, err := database.NewVectorDatabase(s.db, s.cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create vector database: %w", err)
	}
	
	// Initialize vector database
	if err := vectorDB.Initialize(ctx); err != nil {
		logger.Warn("Vector database initialization failed; vector search will be disabled", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}
	
	// Create repository
	embedRepo := repository.NewEmbeddingRepository(s.db)
	
	// Create API handler
	vectorAPI := NewVectorAPI(embedRepo, logger)
	
	// Register routes
	apiV1 := s.router.Group("/api/v1")
	vectorAPI.RegisterRoutes(apiV1)
	
	logger.Info("Vector API routes registered", map[string]interface{}{
		"path": "/api/v1/vectors",
	})
	
	// Add metrics collecting middleware for vector operations
	if s.cfg.Monitoring.Prometheus.VectorMetrics.Enabled {
		vectorMetricsMiddleware := createVectorMetricsMiddleware(s.metrics)
		apiV1.Use(vectorMetricsMiddleware)
		
		logger.Info("Vector metrics middleware added", nil)
	}
	
	return nil
}

// createVectorMetricsMiddleware creates a middleware that collects metrics for vector operations
func createVectorMetricsMiddleware(metrics observability.MetricsClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process the request
		c.Next()
		
		// Check if this is a vector operation
		if c.Request.URL.Path == "/api/v1/vectors/search" {
			// Record the request count
			metrics.RecordCounter("vector_search_requests_total", 1, map[string]string{
				"status": fmt.Sprintf("%d", c.Writer.Status()),
			})
		} else if c.Request.URL.Path == "/api/v1/vectors/store" {
			// Record the request count
			metrics.RecordCounter("vector_store_requests_total", 1, map[string]string{
				"status": fmt.Sprintf("%d", c.Writer.Status()),
			})
		}
	}
}
