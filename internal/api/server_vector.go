package api

import (
	"context"
	"fmt"
	
	// Use pkg/database/adapters for compatibility with legacy code
	"github.com/S-Corkum/devops-mcp/pkg/database" // Still needed for type compatibility during migration
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/database/adapters"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/gin-gonic/gin"
)

// loggerAdapter adapts the internal/observability.Logger to the simplified
// adapter.Logger interface used in our migration adapter.
// This follows the adapter pattern we've used throughout the migration.
type loggerAdapter struct {
	logger observability.Logger
}

// Info implements the adapter.Logger interface
func (l *loggerAdapter) Info(msg string, fields map[string]interface{}) {
	l.logger.Info(msg, fields)
}

// Warn implements the adapter.Logger interface
func (l *loggerAdapter) Warn(msg string, fields map[string]interface{}) {
	l.logger.Warn(msg, fields)
}

// Error implements the adapter.Logger interface
func (l *loggerAdapter) Error(msg string, fields map[string]interface{}) {
	l.logger.Error(msg, fields)
}

// setupVectorAPI initializes and registers the vector API routes
func (s *Server) setupVectorAPI(ctx context.Context) error {
	logger := s.logger.WithPrefix("vector_api")
	
	// Check if vector operations are enabled
	if !s.cfg.Database.Vector.Enabled {
		logger.Info("Vector operations are disabled", nil)
		return nil
	}
	
	// Initialize vector database using our simplified adapter during the migration phase
	if s.vectorDB == nil {
		// First, create an adapter wrapper for the observability.Logger to work with our simplified adapter
		adapterLogger := &loggerAdapter{logger: logger}

		// Create the LegacyVectorAdapter which conforms to our requirements
		// This is a temporary migration shim that implements the VectorDatabase interface
		// but doesn't have complex dependencies
		adapter, adapterErr := adapters.NewLegacyVectorAdapter(s.db, s.cfg, adapterLogger)
		if adapterErr != nil {
			return fmt.Errorf("failed to create vector database adapter: %w", adapterErr)
		}

		// Since our adapter implements the same interface as the internal VectorDatabase,
		// we can use it directly during the transition phase
		s.vectorDB = adapter
		
		// Log that we're using the simplified adapter pattern for migration
		logger.Info("Using simplified LegacyVectorAdapter for database migration", map[string]interface{}{
			"phase": "transition",
		})
	}
	
	// Initialize vector database
	if err := s.vectorDB.Initialize(ctx); err != nil {
		logger.Warn("Vector database initialization failed", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("vector database initialization failed: %w", err)
	}
	
	// Create repository using pkg/repository
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
