package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/username/mcp-server/internal/core"
)

// Server represents the API server
type Server struct {
	router *gin.Engine
	server *http.Server
	engine *core.Engine
	config Config
}

// NewServer creates a new API server
func NewServer(engine *core.Engine, cfg Config) *Server {
	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(RequestLogger())
	router.Use(MetricsMiddleware())

	if cfg.RateLimit.Enabled {
		router.Use(RateLimiter(cfg.RateLimit))
	}

	server := &Server{
		router: router,
		engine: engine,
		config: cfg,
		server: &http.Server{
			Addr:         cfg.ListenAddress,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}

	// Initialize routes
	server.setupRoutes()

	return server
}

// setupRoutes initializes all API routes
func (s *Server) setupRoutes() {
	// Health and metrics endpoints
	s.router.GET("/health", s.healthHandler)
	s.router.GET("/metrics", s.metricsHandler)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// MCP protocol endpoints
		mcp := v1.Group("/mcp")
		{
			mcp.POST("/context", s.contextHandler)
			mcp.GET("/context/:id", s.getContextHandler)
			// More MCP endpoints...
		}

		// Webhook endpoints
		webhook := v1.Group("/webhook")
		{
			webhook.POST("/github", s.githubWebhookHandler)
			webhook.POST("/harness", s.harnessWebhookHandler)
			// More webhook endpoints...
		}
	}
}

// Start starts the API server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the API server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// healthHandler returns the health status of all components
func (s *Server) healthHandler(c *gin.Context) {
	health := s.engine.Health()

	// If any component is unhealthy, return 503
	for _, status := range health {
		if status != "healthy" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":     "unhealthy",
				"components": health,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"components": health,
	})
}
