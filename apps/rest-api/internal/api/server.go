package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/agents"
	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"rest-api/internal/adapters"
	contextAPI "rest-api/internal/api/context"
	"rest-api/internal/core"
	"rest-api/internal/repository"
)

// Helper function to extract string from map
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Helper function to get last N characters of a string
func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

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
	cfg            *config.Config
	authMiddleware *auth.AuthMiddleware // Enhanced auth with rate limiting, metrics, and audit
	healthChecker  *HealthChecker
	cache          cache.Cache
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
	
	// Parse API keys from configuration
	if apiKeysRaw, ok := cfg.Auth.APIKeys.(map[string]interface{}); ok {
		if staticKeys, ok := apiKeysRaw["static_keys"].(map[string]interface{}); ok {
			for key, settings := range staticKeys {
				if settingsMap, ok := settings.(map[string]interface{}); ok {
					apiKeySettings := auth.APIKeySettings{
						Role:     getStringFromMap(settingsMap, "role"),
						TenantID: getStringFromMap(settingsMap, "tenant_id"),
					}
					
					// Parse scopes
					if scopesRaw, ok := settingsMap["scopes"].([]interface{}); ok {
						scopes := make([]string, 0, len(scopesRaw))
						for _, s := range scopesRaw {
							if scope, ok := s.(string); ok {
								scopes = append(scopes, scope)
							}
						}
						apiKeySettings.Scopes = scopes
					}
					
					authConfig.APIKeys[key] = apiKeySettings
					
					// Debug logging
					logger.Info("API Key from config", map[string]interface{}{
						"key_suffix": lastN(key, 8),
						"role":       apiKeySettings.Role,
						"tenant_id":  apiKeySettings.TenantID,
						"scopes":     apiKeySettings.Scopes,
					})
				}
			}
		}
	}
	
	// Set JWT secret environment variable if provided
	if cfg.Auth.JWTSecret != "" {
		if err := os.Setenv("JWT_SECRET", cfg.Auth.JWTSecret); err != nil {
			logger.Warn("Failed to set JWT_SECRET environment variable", map[string]interface{}{"error": err})
		}
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


	// Initialize health checker
	healthChecker := NewHealthChecker(db)
	
	// Initialize cache based on configuration
	var cacheImpl cache.Cache
	if config != nil && config.Cache != nil {
		// Initialize cache with context
		var err error
		cacheImpl, err = cache.NewCache(context.Background(), config.Cache)
		if err != nil {
			logger.Warn("Failed to initialize cache, using no-op cache", map[string]any{
				"error": err.Error(),
			})
			// Fall back to no-op cache
			cacheImpl = cache.NewNoOpCache()
		}
	} else {
		// Use no-op cache if not configured
		cacheImpl = cache.NewNoOpCache()
	}
	
	server := &Server{
		router:      router,
		engine:      engine,
		config:      cfg,
		logger:      logger,
		db:          db,
		metrics:     metrics,
		cfg:            config,
		authMiddleware: authMiddleware,
		healthChecker:  healthChecker,
		cache:          cacheImpl,
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

	// Add tenant context extraction middleware AFTER authentication
	v1.Use(ExtractTenantContext())

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
				"embeddings":    baseURL + "/api/embeddings",
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

	// Embedding API v2 - Multi-agent embedding system
	// Initialize the embedding service with all configured providers
	embeddingService, embeddingErr := adapters.CreateEmbeddingService(s.cfg, *database.NewDatabaseWithConnection(s.db), s.cache)
	if embeddingErr != nil {
		s.logger.Error("Failed to create embedding service", map[string]any{
			"error": embeddingErr.Error(),
		})
		// Use mock or partial service if initialization fails
		s.logger.Warn("Embedding service initialization failed, some features may be limited", nil)
	} else {
		// Create agent repository and service using the PostgreSQL implementation
		agentPostgresRepo := agents.NewPostgresRepository(s.db, "mcp")
		agentService := agents.NewService(agentPostgresRepo)
		
		// Create and register embedding API
		embeddingAPI := NewEmbeddingAPI(embeddingService, agentService, s.logger)
		embeddingAPI.RegisterRoutes(v1)
		
		s.logger.Info("Embedding API v2 initialized successfully", nil)
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

	return s.server.Shutdown(ctx)
}

// healthHandler function removed - using health.HandleHealthCheck instead

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
