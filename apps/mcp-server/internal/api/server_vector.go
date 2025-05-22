package api

import (
	"context"
	"fmt"
	
	"github.com/gin-gonic/gin"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/api/proxies"
	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/api/handlers"
)

// SetupVectorAPI initializes and registers the vector API routes
func (s *Server) SetupVectorAPI(ctx context.Context) error {
	logger := s.logger.WithPrefix("vector_api")
	
	// Check if vector operations are enabled
	isEnabled := true

	// Access configuration fields directly since s.cfg is a *config.Config
	if s.cfg != nil {
		// Direct access to the concrete config struct fields
		isEnabled = s.cfg.Database.Vector.Enabled
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
	
	// Setup advanced vector search routes using handlers package
	if err := handlers.SetupSearchRoutes(apiV1, s.db.DB, s.cfg, logger); err != nil {
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

// Note: Vector routes are now handled by vectorAPI.RegisterRoutes() call in SetupVectorAPI

// Note: Vector routes are now handled by vectorAPI.RegisterRoutes() call
