package api

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)


// SetupVectorAPI initializes and registers the vector API routes
func (s *Server) SetupVectorAPI(ctx context.Context) error {
	logger := s.logger.WithPrefix("vector_api")

	// Check if vector operations are enabled
	isEnabled := false
	if s.cfg != nil {
		// Check if Vector is enabled in the config
		if vectorConfig, ok := s.cfg.Database.Vector.(map[string]interface{}); ok {
			if enabled, ok := vectorConfig["enabled"].(bool); ok {
				isEnabled = enabled
			}
		}
	}

	if !isEnabled {
		logger.Info("Vector operations are disabled", nil)
		return nil
	}

	// Initialize vector database
	var err error
	if s.vectorDB == nil {
		s.vectorDB, err = database.NewVectorDatabase(s.db, s.cfg, logger)
		if err != nil {
			return fmt.Errorf("failed to create vector database: %w", err)
		}
	}

	// Initialize vector database
	if err := s.vectorDB.Initialize(ctx); err != nil {
		logger.Warn("Vector database initialization failed", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("vector database initialization failed: %w", err)
	}

	// Create repository
	embedRepo := repository.NewEmbeddingRepository(s.db)

	// Store repository in server for use in other components
	s.embeddingRepo = embedRepo

	// Setup vector routes directly on the server
	apiV1 := s.router.Group("/api/v1")
	s.setupVectorRoutes(apiV1)

	logger.Info("Vector API routes registered", map[string]interface{}{
		"path": "/api/v1/vectors",
	})

	// Setup advanced vector search routes
	if err := s.setupSearchRoutes(apiV1); err != nil {
		logger.Warn("Failed to setup vector search API", map[string]interface{}{
			"error": err.Error(),
		})
		// Non-fatal, continue with other routes
	} else {
		logger.Info("Vector search API routes registered", map[string]interface{}{
			"path": "/api/v1/search",
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

// These method signatures need to be implemented based on the actual routes
// and handlers in your vector_handlers.go file
func (s *Server) setupVectorRoutes(group *gin.RouterGroup) {
	// This would be implemented in vector_handlers.go
}

func (s *Server) setupSearchRoutes(group *gin.RouterGroup) error {
	// This would be implemented in search_handlers.go
	return nil
}
