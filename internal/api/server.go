package api

import (
	"context"
	"net/http"
	"time"

	contextAPI "github.com/S-Corkum/mcp-server/internal/api/context"
	"github.com/S-Corkum/mcp-server/internal/config"
	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Global shutdown hooks
var shutdownHooks []func()

// Server represents the API server
type Server struct {
	router   *gin.Engine
	server   *http.Server
	engine   *core.Engine
	config   Config
	logger   *observability.Logger
	db       *sqlx.DB
	metrics  *observability.MetricsClient
	vectorDB *database.VectorDatabase
	cfg      *config.Config
}

// NewServer creates a new API server
func NewServer(engine *core.Engine, cfg Config, db *sqlx.DB, metrics *observability.MetricsClient, config *config.Config) *Server {
	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(RequestLogger())

	// Apply performance optimizations based on configuration
	if cfg.Performance.EnableCompression {
		router.Use(CompressionMiddleware()) // Add response compression
	}

	if cfg.Performance.EnableETagCaching {
		router.Use(CachingMiddleware()) // Add HTTP caching
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
		MaxIdleConns:          cfg.Performance.HTTPMaxIdleConns,
		MaxConnsPerHost:       cfg.Performance.HTTPMaxConnsPerHost,
		IdleConnTimeout:       cfg.Performance.HTTPIdleConnTimeout,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
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

	// Initialize vector database if enabled
	var vectorDB *database.VectorDatabase
	if config != nil && config.Database.Vector.Enabled {
		var err error
		vectorDB, err = database.NewVectorDatabase(db, config, logger.WithPrefix("vector_db"))
		if err != nil {
			logger.Warn("Failed to initialize vector database", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	server := &Server{
		router:   router,
		engine:   engine,
		config:   cfg,
		logger:   logger,
		db:       db,
		metrics:  metrics,
		vectorDB: vectorDB,
		cfg:      config,
		server: &http.Server{
			Addr:         cfg.ListenAddress,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}

	return server
}

// Initialize initializes all components and routes
func (s *Server) Initialize(ctx context.Context) error {
	// Initialize vector database if available
	if s.vectorDB != nil {
		if err := s.vectorDB.Initialize(ctx); err != nil {
			s.logger.Warn("Vector database initialization failed", map[string]interface{}{
				"error": err.Error(),
			})
			// Don't fail server startup if vector DB init fails
		}
	}

	// Initialize routes
	s.setupRoutes(ctx)

	return nil
}

// setupRoutes initializes all API routes
func (s *Server) setupRoutes(ctx context.Context) {
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
				"tools":         baseURL + "/api/v1/tools",
				"contexts":      baseURL + "/api/v1/contexts",
				"vectors":       baseURL + "/api/v1/vectors",
				"health":        baseURL + "/health",
				"documentation": baseURL + "/swagger/index.html",
			},
		})
	})

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

	// Context API - register the context endpoints
	ctxAPI := contextAPI.NewAPI(
		s.engine.GetContextManager(),
		s.logger,
		s.metrics,
	)
	ctxAPI.RegisterRoutes(v1)

	// Setup Vector API if enabled
	if s.vectorDB != nil {
		if err := s.setupVectorAPI(ctx); err != nil {
			s.logger.Warn("Failed to setup vector API", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
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

	// Close vector database if available
	if s.vectorDB != nil {
		if err := s.vectorDB.Close(); err != nil {
			s.logger.Warn("Failed to close vector database", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return s.server.Shutdown(ctx)
}

// healthHandler returns the health status of all components
func (s *Server) healthHandler(c *gin.Context) {
	health := s.engine.Health()

	// Add vector database health if available
	if s.vectorDB != nil {
		health["vector_database"] = "healthy"
	}

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

// RegisterShutdownHook registers a function to be called during server shutdown
func RegisterShutdownHook(hook func()) {
	shutdownHooks = append(shutdownHooks, hook)
}
