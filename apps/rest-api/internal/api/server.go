package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	contextAPI "rest-api/internal/api/context"
	"rest-api/internal/core"
	"rest-api/internal/repository"
)

// Global shutdown hooks
var shutdownHooks []func()

// Server represents the API server
type Server struct {
	router      *gin.Engine
	server      *http.Server
	engine      *core.Engine
	config      Config
	logger      observability.Logger
	db          *sqlx.DB
	metrics     observability.MetricsClient
	vectorDB    *database.VectorDatabase
	vectorRepo     repository.VectorAPIRepository
	cfg            *config.Config
	authMiddleware *auth.AuthMiddleware // Enhanced auth with rate limiting, metrics, and audit
	healthChecker  *HealthChecker
}

// NewServer creates a new API server
func NewServer(engine *core.Engine, cfg Config, db *sqlx.DB, metrics observability.MetricsClient, config *config.Config) *Server {
	// Defensive: fail fast if db is nil
	if db == nil {
		panic("[api.NewServer] FATAL: received nil *sqlx.DB. Check database initialization before calling NewServer.")
	}

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

	// Initialize logger first
	logger := observability.NewLogger("api-server")

	// Setup authentication configuration
	authConfig := &auth.AuthSystemConfig{
		Service: &auth.ServiceConfig{
			JWTSecret:         cfg.Auth.JWTSecret,
			JWTExpiration:     24 * time.Hour,
			APIKeyHeader:      "X-API-Key",
			EnableAPIKeys:     true,
			EnableJWT:         true,
			CacheEnabled:      false, // Disable cache for now
			MaxFailedAttempts: 5,
			LockoutDuration:   15 * time.Minute,
		},
		RateLimiter: auth.DefaultRateLimiterConfig(),
		APIKeys:     make(map[string]auth.APIKeySettings),
	}
	
	// Set JWT secret environment variable if provided
	if cfg.Auth.JWTSecret != "" {
		os.Setenv("JWT_SECRET", cfg.Auth.JWTSecret)
	}
	
	// Use the enhanced setup that gives us control over configuration
	authMiddleware, err := auth.SetupAuthenticationWithConfig(authConfig, db, nil, logger, metrics)
	if err != nil {
		logger.Error("Failed to setup enhanced authentication", map[string]interface{}{
			"error": err.Error(),
		})
		panic("Failed to setup authentication: " + err.Error())
	}
	
	logger.Info("Enhanced authentication initialized", map[string]interface{}{
		"environment": os.Getenv("ENVIRONMENT"),
		"api_key_source": os.Getenv("API_KEY_SOURCE"),
	})

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

	// Initialize vector database if enabled
	var vectorDB *database.VectorDatabase
	isVectorEnabled := false
	if config != nil {
		if vectorConfig, ok := config.Database.Vector.(map[string]any); ok {
			if enabled, ok := vectorConfig["enabled"].(bool); ok {
				isVectorEnabled = enabled
			}
		}
	}

	if isVectorEnabled {
		var err error
		vectorDB, err = database.NewVectorDatabase(db, config, logger.WithPrefix("vector_db"))
		if err != nil {
			logger.Warn("Failed to initialize vector database", map[string]any{
				"error": err.Error(),
			})
		}
	}

	// Initialize health checker
	healthChecker := NewHealthChecker(db)
	
	server := &Server{
		router:      router,
		engine:      engine,
		config:      cfg,
		logger:      logger,
		db:          db,
		metrics:     metrics,
		vectorDB:       vectorDB,
		cfg:            config,
		authMiddleware: authMiddleware,
		healthChecker:  healthChecker,
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
			s.logger.Warn("Vector database initialization failed", map[string]any{
				"error": err.Error(),
			})
			// Don't fail server startup if vector DB init fails
		}
	}

	// Ensure we have a valid context manager
	if s.engine != nil {
		// Always create a context manager as follows:
		// 1. First check if one is already set
		// 2. If not, check the environment to determine if we should use a mock
		// 3. Create and set either a real or mock context manager
		// 4. Verify that it was correctly set before proceeding

		// Get current context manager (if any)
		ctxManager := s.engine.GetContextManager()

		// Set a new context manager if none exists
		if ctxManager == nil {
			// Check environment variable to determine whether to use mock or real
			useMock := os.Getenv("USE_MOCK_CONTEXT_MANAGER")

			s.logger.Info("Context manager not found, initializing new one", map[string]any{
				"use_mock": useMock,
			})

			if strings.ToLower(useMock) == "true" {
				// Create mock context manager for development/testing
				s.logger.Info("Using mock context manager as specified by environment", nil)
				ctxManager = core.NewMockContextManager()
			} else {
				// Use our production-ready context manager implementation
				s.logger.Info("Initializing production-ready context manager", nil)

				// Pass existing components to the context manager
				s.logger.Info("Creating production context manager", nil)

				// Create the production context manager with available components
				// We're using an updated version of NewContextManager that accepts *sqlx.DB directly
				ctxManager = core.NewContextManager(s.db, s.logger, s.metrics)
				s.logger.Info("Production context manager initialized", nil)
			}

			// Set the context manager on the engine
			s.engine.SetContextManager(ctxManager)

			// Log the change
			s.logger.Info("Context manager set on engine", nil)
		} else {
			s.logger.Info("Using existing context manager", nil)
		}

		// Explicitly verify that a context manager is set before continuing
		if verifyCtx := s.engine.GetContextManager(); verifyCtx == nil {
			s.logger.Error("Context manager initialization failed - still nil after setting", nil)
			return fmt.Errorf("failed to initialize context manager, engine reports nil after setting")
		} else {
			s.logger.Info("Context manager initialization confirmed successful", nil)
		}
	} else {
		s.logger.Error("Engine is nil, cannot initialize context manager", nil)
		return fmt.Errorf("engine is nil, cannot initialize context manager")
	}

	// Initialize routes
	s.setupRoutes(ctx)

	// Mark server as ready
	s.healthChecker.SetReady(true)
	s.logger.Info("Server initialization complete and ready to serve requests", nil)

	return nil
}

// setupRoutes initializes all API routes
func (s *Server) setupRoutes(ctx context.Context) {
	// Public endpoints
	// Health check endpoints
	s.router.GET("/health", s.healthChecker.HealthHandler)
	s.router.GET("/healthz", s.healthChecker.LivenessHandler)  // Kubernetes liveness probe
	s.router.GET("/readyz", s.healthChecker.ReadinessHandler)  // Kubernetes readiness probe

	// Swagger API documentation
	if s.config.EnableSwagger {
		s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// Metrics endpoint - public (no authentication required)
	s.router.GET("/metrics", s.metricsHandler)

	// API v1 routes - require authentication
	v1 := s.router.Group("/api/v1")

	// Always use enhanced auth middleware - it includes all authentication features
	v1.Use(s.authMiddleware.GinMiddleware())
	s.logger.Info("Using enhanced authentication with rate limiting and audit logging", nil)

	// Keep the old middleware for backward compatibility during transition
	// This will be removed once all tests are updated
	testMode := os.Getenv("MCP_TEST_MODE")
	if testMode == "true" {
		fmt.Println("Test mode enabled, also applying legacy auth middleware")
		v1.Use(AuthMiddleware("api_key"))
	}

	// Root endpoint to provide API entry points (HATEOAS)
	v1.GET("/", func(c *gin.Context) {
		// Check for authentication result set by AuthMiddleware
		user, exists := c.Get("user")
		if !exists || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

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

	// --- Webhook Integration: Mount Gorilla Mux for webhook endpoints ---
	// Create a Gorilla Mux router and register webhook routes
	muxRouter := mux.NewRouter()
	s.RegisterWebhookRoutes(muxRouter)

	// Mount the mux router for all /api/webhooks/* paths
	// This allows the /api/webhooks/github endpoint to be handled by the correct handler
	s.router.Any("/api/webhooks/github", gin.WrapH(muxRouter))
	s.router.Any("/api/webhooks/github/", gin.WrapH(muxRouter))

	// Tool integration API - using resource-based approach
	adapterBridge, err := s.engine.GetAdapter("adapter_bridge")
	if err != nil {
		s.logger.Warn("Failed to get adapter bridge, using mock implementation", map[string]any{
			"error": err.Error(),
		})
		// Use a nil interface, the ToolAPI will use mock implementations
		adapterBridge = nil
	}
	toolAPI := NewToolAPI(adapterBridge)
	toolAPI.RegisterRoutes(v1)

	// Agent and Model APIs - create repositories first as they're needed by context API
	agentRepo := repository.NewAgentRepository(s.db.DB)
	agentAPI := NewAgentAPI(agentRepo)
	agentAPI.RegisterRoutes(v1)
	modelRepo := repository.NewModelRepository(s.db.DB)

	// Context API - register the context endpoints
	ctxAPI := contextAPI.NewAPI(
		s.engine.GetContextManager(),
		s.logger,
		s.metrics,
		s.db,
		modelRepo,
	)
	ctxAPI.RegisterRoutes(v1)
	modelAPI := NewModelAPI(modelRepo)
	modelAPI.RegisterRoutes(v1)

	// Setup Vector API if enabled
	if s.vectorDB != nil {
		if err := s.setupVectorAPI(ctx); err != nil {
			s.logger.Warn("Failed to setup vector API", map[string]any{
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
			s.logger.Warn("Failed to close vector database", map[string]any{
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
