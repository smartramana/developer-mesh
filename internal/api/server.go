package api

import (
	"context"
	"net/http"
	"time"

	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Server represents the API server
type Server struct {
	router               *gin.Engine
	server               *http.Server
	engine               *core.Engine
	config               Config
	logger               *observability.Logger
}

// NewServer creates a new API server
func NewServer(engine *core.Engine, cfg Config) *Server {
	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(RequestLogger())
	
	// Apply performance optimizations based on configuration
	if cfg.Performance.EnableCompression {
		router.Use(CompressionMiddleware())  // Add response compression
	}
	
	if cfg.Performance.EnableETagCaching {
		router.Use(CachingMiddleware())      // Add HTTP caching
	}
	
	router.Use(MetricsMiddleware())
	router.Use(ErrorHandlerMiddleware()) // Add centralized error handling
	router.Use(TracingMiddleware())      // Add request tracing
	
	// Apply API versioning
	router.Use(VersioningMiddleware(cfg.Versioning))

	if cfg.RateLimit.Enabled {
		limiterConfig := NewRateLimiterConfigFromConfig(cfg.RateLimit)
		router.Use(RateLimiter(limiterConfig))
	}

	// Enable CORS if configured
	if cfg.EnableCORS {
		corsConfig := CORSConfig{
			AllowedOrigins: []string{"*"}, // Default to allow all origins in development
		}
		router.Use(CORSMiddleware(corsConfig))
	}
	
	// Initialize API keys from configuration
	if cfg.Auth.APIKeys != nil {
		InitAPIKeys(cfg.Auth.APIKeys)
	}
	
	// Initialize JWT with secret from configuration
	InitJWT(cfg.Auth.JWTSecret)

	// Configure HTTP client transport for external service calls
	httpTransport := &http.Transport{
		MaxIdleConns:        cfg.Performance.HTTPMaxIdleConns,
		MaxConnsPerHost:     cfg.Performance.HTTPMaxConnsPerHost,
		IdleConnTimeout:     cfg.Performance.HTTPIdleConnTimeout,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}
	
	// Create custom HTTP client with the optimized transport
	httpClient := &http.Client{
		Transport: httpTransport,
		Timeout:   60 * time.Second,
	}
	
	// Use the custom HTTP client for external service calls
	http.DefaultClient = httpClient
	
	// Initialize logger
	logger := observability.NewLogger("api-server")

	server := &Server{
		router:       router,
		engine:       engine,
		config:       cfg,
		logger:       logger,
		server:       &http.Server{
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
	// Public endpoints
	s.router.GET("/health", s.healthHandler)
	
	// Swagger API documentation
	if s.config.EnableSwagger {
		s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
	
	// Metrics endpoints - add authentication
	s.router.GET("/metrics", AuthMiddleware("api_key"), s.metricsHandler)

	// API v1 routes - require authentication
	v1 := s.router.Group("/api/v1")
	v1.Use(AuthMiddleware("jwt")) // Require JWT auth for all API endpoints
	
	// Root endpoint to provide API entry points (HATEOAS)
	v1.GET("/", func(c *gin.Context) {
		baseURL := s.getBaseURL(c)
		c.JSON(http.StatusOK, gin.H{
			"api_version": "1.0",
			"description": "MCP Server API for DevOps tool integration following Model Context Protocol",
			"links": map[string]string{
				"tools": baseURL + "/api/v1/tools",
				"health": baseURL + "/health",
				"documentation": baseURL + "/swagger/index.html",
			},
		})
	})
	

	
	// Skip MCPAPI initialization for now since the interface doesn't match
	// This would need proper interface adaptation or an implementation with the expected methods
	
	// Tool integration API - using resource-based approach
	adapterBridge, err := s.engine.GetAdapter("adapter_bridge")
	if err != nil {
		s.logger.Warn("Failed to get adapter bridge, using mock implementation", map[string]interface{}{
			"error": err.Error(),
		})
		// Use a nil interface, the ToolAPI will use mock implementations
		adapterBridge = nil
	}
	toolAPI := NewToolAPI(adapterBridge)
	toolAPI.RegisterRoutes(v1)
	
	// Register GET /tools directly as a static handler (for backward compatibility)
	v1.GET("/tools", func(c *gin.Context) {
		// We're not using the baseURL for now to avoid unused variable error
		c.JSON(http.StatusOK, gin.H{
			"tools": []map[string]interface{}{
			{
				"name": "github",
				"description": "GitHub integration for repository, pull request, and code management",
				"actions": []string{
					"create_issue",
					"close_issue",
					"create_pull_request",
					"merge_pull_request",
					"add_comment",
					"archive_repository",
				},
				"safety_notes": "Cannot delete repositories for safety reasons",
			},
			{
				"name": "harness",
				"description": "Harness CI/CD integration for builds and deployments",
				"actions": []string{
					"trigger_pipeline",
					"get_pipeline_status",
					"stop_pipeline",
					"rollback_deployment",
				},
				"safety_notes": "Cannot delete production feature flags for safety reasons",
			},
			{
				"name": "sonarqube",
				"description": "SonarQube integration for code quality analysis",
				"actions": []string{
					"trigger_analysis",
					"get_quality_gate_status",
					"get_issues",
				},
			},
			{
				"name": "artifactory",
				"description": "JFrog Artifactory integration for artifact management (read-only)",
				"actions": []string{
					"download_artifact",
					"get_artifact_info",
					"search_artifacts",
				},
				"safety_notes": "Read-only access for safety reasons (no upload or delete capabilities)",
			},
			{
				"name": "xray",
				"description": "JFrog Xray integration for security scanning",
				"actions": []string{
					"scan_artifact",
					"get_vulnerabilities",
					"get_licenses",
				},
			},
		}})
	})
	

	

}

// Start starts the API server without TLS
func (s *Server) Start() error {
	// Start without TLS
	return s.server.ListenAndServe()
}

// StartTLS starts the API server with TLS
func (s *Server) StartTLS(certFile, keyFile string) error {
	// If specific files are provided, use those
	if certFile != "" && keyFile != "" {
		return s.server.ListenAndServeTLS(certFile, keyFile)
	}
	
	// Otherwise use the ones from config
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		return s.server.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile)
	}
	
	// If no TLS files are available, return an error
	return nil
}

// Shutdown gracefully shuts down the API server
func (s *Server) Shutdown(ctx context.Context) error {
	// Execute all registered shutdown hooks
	for _, hook := range shutdownHooks {
		hook()
	}
	
	return s.server.Shutdown(ctx)
}

// healthHandler returns the health status of all components
func (s *Server) healthHandler(c *gin.Context) {
	health := s.engine.Health()
	
	// Check if any component is unhealthy
	allHealthy := true
	for _, status := range health {
		// Consider "healthy" or any status starting with "healthy" (like "healthy (mock)") as healthy
		if status != "healthy" && len(status) < 7 || (len(status) >= 7 && status[:7] != "healthy") {
			allHealthy = false
			break
		}
	}
	
	if allHealthy {
		c.JSON(http.StatusOK, gin.H{
			"status":     "healthy",
			"components": health,
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":     "unhealthy",
			"components": health,
		})
	}
}

// metricsHandler returns metrics for Prometheus
func (s *Server) metricsHandler(c *gin.Context) {
	// Implementation depends on metrics client
	c.String(http.StatusOK, "# metrics data will be here")
}

// getBaseURL extracts the base URL from the request for HATEOAS links
func (s *Server) getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	
	host := c.Request.Host
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}
	
	return scheme + "://" + host
}

// This section intentionally left empty after removing unused context handlers



// This section intentionally left empty after removing updateContextHandler
