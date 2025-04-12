package api

import (
	"context"
	"net/http"

	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/gin-gonic/gin"
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

	// Enable CORS if configured
	if cfg.EnableCORS {
		router.Use(CORSMiddleware())
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
			// Setup GitHub webhook endpoint if enabled
			if s.config.Webhooks.GitHub.Enabled {
				path := "/github"
				if s.config.Webhooks.GitHub.Path != "" {
					path = s.config.Webhooks.GitHub.Path
				}
				webhook.POST(path, s.githubWebhookHandler)
			}

			// Setup Harness webhook endpoint if enabled
			if s.config.Webhooks.Harness.Enabled {
				path := "/harness"
				if s.config.Webhooks.Harness.Path != "" {
					path = s.config.Webhooks.Harness.Path
				}
				webhook.POST(path, s.harnessWebhookHandler)
			}

			// Setup SonarQube webhook endpoint if enabled
			if s.config.Webhooks.SonarQube.Enabled {
				path := "/sonarqube"
				if s.config.Webhooks.SonarQube.Path != "" {
					path = s.config.Webhooks.SonarQube.Path
				}
				webhook.POST(path, s.sonarqubeWebhookHandler)
			}

			// Setup Artifactory webhook endpoint if enabled
			if s.config.Webhooks.Artifactory.Enabled {
				path := "/artifactory"
				if s.config.Webhooks.Artifactory.Path != "" {
					path = s.config.Webhooks.Artifactory.Path
				}
				webhook.POST(path, s.artifactoryWebhookHandler)
			}

			// Setup Xray webhook endpoint if enabled
			if s.config.Webhooks.Xray.Enabled {
				path := "/xray"
				if s.config.Webhooks.Xray.Path != "" {
					path = s.config.Webhooks.Xray.Path
				}
				webhook.POST(path, s.xrayWebhookHandler)
			}
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

// metricsHandler returns metrics for Prometheus
func (s *Server) metricsHandler(c *gin.Context) {
	// Implementation depends on metrics client
	c.String(http.StatusOK, "# metrics data will be here")
}

// contextHandler creates a new MCP context
func (s *Server) contextHandler(c *gin.Context) {
	// To be implemented
	c.JSON(http.StatusOK, gin.H{"message": "context created"})
}

// getContextHandler gets an MCP context by ID
func (s *Server) getContextHandler(c *gin.Context) {
	id := c.Param("id")
	// To be implemented
	c.JSON(http.StatusOK, gin.H{"id": id, "message": "context retrieved"})
}
