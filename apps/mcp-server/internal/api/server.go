package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/client/rest"
	commonLogging "github.com/S-Corkum/devops-mcp/pkg/common/logging"
	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/pkg/repository/agent"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"mcp-server/internal/api/proxies"
	"mcp-server/internal/api/websocket"
	"mcp-server/internal/core"
)

// Global shutdown hooks
var shutdownHooks []func()

// Server represents the API server
type Server struct {
	router            *gin.Engine
	server            *http.Server
	engine            *core.Engine
	config            Config
	logger            observability.Logger
	loggerAdapter     *commonLogging.Logger // For compatibility with old code
	loggerObsAdapter  observability.Logger  // Adapter that wraps commonLogging.Logger as observability.Logger
	db                *sqlx.DB
	metrics           observability.MetricsClient
	metricsAdapter    observability.Client // For compatibility with old code
	cfg               *config.Config
	restClientFactory *rest.Factory // REST API client factory for communication with REST API
	authService       *auth.Service
	authMiddleware    *auth.AuthMiddleware // Enhanced auth with rate limiting, metrics, and audit
	cache             cache.Cache
	// API proxies that delegate to REST API
	vectorAPIProxy  repository.VectorAPIRepository // Proxy for vector operations
	agentAPIProxy   agent.Repository               // Proxy for agent operations
	modelAPIProxy   repository.ModelRepository     // Proxy for model operations
	contextAPIProxy repository.ContextRepository   // Proxy for context operations
	searchAPIProxy  repository.SearchRepository    // Proxy for search operations
	// WebSocket server
	wsServer        *websocket.Server
	webhookAPIProxy proxies.WebhookRepository      // Proxy for webhook operations
}

// NewServer creates a new API server
func NewServer(engine *core.Engine, cfg Config, db *sqlx.DB, cacheClient cache.Cache, metrics observability.MetricsClient, config *config.Config) *Server {
	// Create adapter objects for compatibility with existing code
	loggerAdapter := observability.NewLoggerAdapter(observability.DefaultLogger)
	// Create an adapter that wraps commonLogging.Logger as observability.Logger
	loggerObsAdapter := observability.NewCommonLoggerAdapter(loggerAdapter)
	metricsAdapter := observability.NewMetricsAdapter(metrics)
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

	// Initialize observability if not already done
	if observability.DefaultLogger == nil {
		observability.DefaultLogger = observability.NewStandardLogger("mcp-server")
	}
	
	// Initialize auth service with cache
	authConfig := auth.DefaultConfig()
	authConfig.JWTSecret = cfg.Auth.JWTSecret
	authConfig.EnableAPIKeys = true
	authConfig.EnableJWT = true

	// Create auth service with cache
	authService := auth.NewService(authConfig, db, cacheClient, observability.DefaultLogger)
	
	// Setup enhanced authentication with rate limiting, metrics, and audit logging
	authMiddleware, err := auth.SetupAuthentication(db, cacheClient, observability.DefaultLogger, metrics)
	if err != nil {
		observability.DefaultLogger.Error("Failed to setup enhanced authentication", map[string]interface{}{
			"error": err.Error(),
		})
		// Fall back to basic auth service
		authMiddleware = nil
	}

	// Initialize API keys from configuration
	if cfg.Auth.APIKeys != nil {
		fmt.Printf("API Keys from config: %+v\n", cfg.Auth.APIKeys)

		// Use the new method that handles full configuration including tenant IDs
		if apiKeys, ok := cfg.Auth.APIKeys.(map[string]interface{}); ok {
			authService.InitializeAPIKeysWithConfig(apiKeys)
		} else {
			// Fall back to simple string map if needed
			if apiKeys, ok := cfg.Auth.APIKeys.(map[string]string); ok {
				authService.InitializeDefaultAPIKeys(apiKeys)
			}
		}
	} else {
		fmt.Println("No API keys defined in config")
	}

	// JWT is now handled by the auth service configuration

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
	// We don't initialize the vector database directly anymore
	// Vector operations are now handled by the REST API client
	var restClientFactory *rest.Factory
	var vectorProxy repository.VectorAPIRepository
	var agentProxy agent.Repository
	var modelProxy repository.ModelRepository

	if cfg.RestAPI.Enabled {
		// Create the REST client factory
		restClientFactory = rest.NewFactory(
			cfg.RestAPI.BaseURL,
			cfg.RestAPI.APIKey,
			observability.DefaultLogger,
		)

		// Create proxies that implement repository interfaces but delegate to REST API
		vectorProxy = proxies.NewVectorAPIProxy(restClientFactory, observability.DefaultLogger)
		agentProxy = proxies.NewAgentAPIProxy(restClientFactory, observability.DefaultLogger)
		modelProxy = proxies.NewModelAPIProxy(restClientFactory, observability.DefaultLogger)

		observability.DefaultLogger.Info("REST API client initialized", map[string]interface{}{
			"base_url": cfg.RestAPI.BaseURL,
			"timeout":  cfg.RestAPI.Timeout,
		})
	} else {
		observability.DefaultLogger.Warn("REST API client disabled - data operations will not be available", nil)
	}

	// Create the server instance
	s := &Server{
		router:            router,
		server:            &http.Server{
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second, // Prevent Slowloris attacks
		},
		engine:            engine,
		config:            cfg,  // Set the API config
		logger:            observability.DefaultLogger,
		loggerAdapter:     loggerAdapter,
		loggerObsAdapter:  loggerObsAdapter,
		db:                db,
		cache:             cacheClient,
		metrics:           metrics,
		metricsAdapter:    metricsAdapter,
		cfg:               config,
		restClientFactory: restClientFactory,
		authService:       authService,
		authMiddleware:    authMiddleware,
		// Store the proxies
		vectorAPIProxy: vectorProxy,
		agentAPIProxy:  agentProxy,
		modelAPIProxy:  modelProxy,
	}
	
	// Initialize WebSocket server if enabled
	if cfg.WebSocket.Enabled {
		wsConfig := websocket.Config{
			MaxConnections:  cfg.WebSocket.MaxConnections,
			ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
			WriteBufferSize: cfg.WebSocket.WriteBufferSize,
			PingInterval:    cfg.WebSocket.PingInterval,
			PongTimeout:     cfg.WebSocket.PongTimeout,
			MaxMessageSize:  cfg.WebSocket.MaxMessageSize,
			Security:        cfg.WebSocket.Security,
			RateLimit:       cfg.WebSocket.RateLimit,
		}
		
		s.wsServer = websocket.NewServer(authService, metrics, observability.DefaultLogger, wsConfig)
		
		// Set dependencies (will be properly implemented in full integration)
		if engine != nil {
			// Use adapter to bridge the interface differences
			contextAdapter := websocket.NewContextManagerAdapter(engine.GetContextManager())
			s.wsServer.SetContextManager(contextAdapter)
			// TODO: Set tool registry and event bus when available
		}
		
		observability.DefaultLogger.Info("WebSocket server initialized", map[string]interface{}{
			"enabled":         true,
			"max_connections": cfg.WebSocket.MaxConnections,
		})
	}
	
	s.server.Addr = cfg.ListenAddress
	s.server.ReadTimeout = cfg.ReadTimeout
	s.server.WriteTimeout = cfg.WriteTimeout
	s.server.IdleTimeout = cfg.IdleTimeout

	return s
}

// Initialize initializes all components and routes
func (s *Server) Initialize(ctx context.Context) error {
	// Setup HTTP server if not done already
	if s.server == nil {
		s.server = &http.Server{
			Addr:              s.config.ListenAddress,
			Handler:           s.router,
			ReadHeaderTimeout: 5 * time.Second, // Prevent Slowloris attacks
		}
	}

	// Initialize API proxies
	if s.config.RestAPI.Enabled {
		// Context repository initialization
		if s.contextAPIProxy == nil {
			// Using mock implementation until import issues are resolved
			s.contextAPIProxy = proxies.NewMockContextRepository(s.logger)
			s.logger.Info("Initialized Mock Context Repository for temporary use", nil)
		}

		// Search repository initialization
		if s.searchAPIProxy == nil {
			// Using mock implementation until import issues are resolved
			s.searchAPIProxy = proxies.NewMockSearchRepository(s.logger)
			s.logger.Info("Initialized Mock Search Repository for temporary use", nil)
		}

		// Webhook repository initialization
		if s.webhookAPIProxy == nil {
			// Using mock implementation until rest client is fully integrated
			s.webhookAPIProxy = proxies.NewMockWebhookRepository(s.logger)
			s.logger.Info("Initialized Mock Webhook Repository for temporary use", nil)
		}
	}

	// Initialize routes
	s.setupRoutes()

	return nil
}

// setupRoutes sets up all API routes
func (s *Server) setupRoutes() {
	// Setup base routes
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "MCP REST API is running"})
	})
	s.router.GET("/health", s.healthHandler)
	
	// Setup WebSocket endpoint
	s.logger.Info("WebSocket route registration check", map[string]interface{}{
		"enabled": s.config.WebSocket.Enabled,
		"wsServer_nil": s.wsServer == nil,
	})
	
	if s.config.WebSocket.Enabled && s.wsServer != nil {
		// Convert gin handler to http.HandlerFunc
		s.router.GET("/ws", func(c *gin.Context) {
			s.wsServer.HandleWebSocket(c.Writer, c.Request)
		})
		s.logger.Info("WebSocket endpoint enabled at /ws", map[string]interface{}{
			"max_connections": s.config.WebSocket.MaxConnections,
		})
	} else {
		s.logger.Warn("WebSocket endpoint NOT enabled", map[string]interface{}{
			"enabled": s.config.WebSocket.Enabled,
			"wsServer_nil": s.wsServer == nil,
		})
	}

	// Setup API documentation
	// Create API versioned routes
	baseURL := ""
	if s.config.ListenAddress != "" {
		baseURL = fmt.Sprintf("http://%s", s.config.ListenAddress)
	}

	url := ginSwagger.URL(fmt.Sprintf("%s/swagger/doc.json", baseURL))
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

	// API v1 routes
	v1 := s.router.Group("/api/v1")

	// Use enhanced auth middleware if available, otherwise fall back to basic auth
	if s.authMiddleware != nil {
		// Use enhanced auth with rate limiting, metrics, and audit logging
		v1.Use(s.authMiddleware.GinMiddleware())
		s.logger.Info("Using enhanced authentication with rate limiting and audit logging", nil)
	} else {
		// Fall back to basic centralized auth middleware
		v1.Use(s.authService.GinMiddleware(auth.TypeAPIKey, auth.TypeJWT))
		s.logger.Warn("Using basic authentication - enhanced features not available", nil)
	}

	// Add credential extraction middleware
	v1.Use(auth.CredentialExtractionMiddleware(s.logger))

	// Add a simple v1 API info endpoint
	v1.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version": "v1",
			"status":  "operational",
			"apis":    []string{"agent", "model", "vector", "embeddings", "mcp"},
		})
	})
	
	// Register MCP API routes
	if s.engine != nil && s.engine.GetContextManager() != nil {
		mcpAPI := NewMCPAPI(s.engine.GetContextManager())
		mcpAPI.RegisterRoutes(v1)
		s.logger.Info("MCP API routes registered", nil)
	} else {
		s.logger.Warn("MCP API not available - context manager not initialized", nil)
	}
	
	// Register WebSocket monitoring routes
	if s.config.WebSocket.Enabled && s.wsServer != nil {
		wsMonitoring := websocket.NewMonitoringEndpoints(s.wsServer)
		wsMonitoring.RegisterRoutes(v1)
		s.logger.Info("WebSocket monitoring routes registered", nil)
	}
	
	// Register Embedding Proxy routes
	if s.config.RestAPI.Enabled && s.config.RestAPI.BaseURL != "" {
		embeddingProxy := proxies.NewEmbeddingProxy(s.config.RestAPI.BaseURL, s.logger)
		embeddingProxy.RegisterRoutes(v1)
		s.logger.Info("Embedding proxy routes registered", map[string]any{
			"rest_api_url": s.config.RestAPI.BaseURL,
		})
	} else {
		s.logger.Warn("Embedding proxy not available - REST API not configured", nil)
	}

	// Log API availability via proxies
	if s.config.RestAPI.Enabled {
		if s.agentAPIProxy != nil {
			s.logger.Info("Agent API available via REST API proxy", nil)
		} else {
			s.logger.Warn("Agent API not available - REST API proxy not configured", nil)
		}

		if s.modelAPIProxy != nil {
			s.logger.Info("Model API available via REST API proxy", nil)
		} else {
			s.logger.Warn("Model API not available - REST API proxy not configured", nil)
		}

		if s.vectorAPIProxy != nil {
			s.logger.Info("Vector API available via REST API proxy", nil)
		} else {
			s.logger.Warn("Vector API not available - REST API proxy not configured", nil)
		}
	} else {
		s.logger.Warn("REST API is disabled - API operations will not be available", nil)
	}
}

// GetListenAddress returns the configured listen address
func (s *Server) GetListenAddress() string {
	return s.server.Addr
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

	// Log that we're shutting down the server
	s.logger.Info("Shutting down MCP server", nil)

	return s.server.Shutdown(ctx)
}

// healthHandler returns the health status of all components
func (s *Server) healthHandler(c *gin.Context) {
	health := s.engine.Health()

	// Check if we have a REST API client
	if s.restClientFactory != nil {
		health["rest_api_client"] = "healthy"
	} else {
		health["rest_api_client"] = "unavailable"
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

// metricsHandler returns metrics for Prometheus (commented out - unused)
// func (s *Server) metricsHandler(c *gin.Context) {
// 	// Implementation depends on metrics client
// 	c.String(http.StatusOK, "# metrics data will be here")
// }

// getBaseURL extracts the base URL from the request for HATEOAS links (commented out - unused)
// func (s *Server) getBaseURL(c *gin.Context) string {
// 	scheme := "http"
// 	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
// 		scheme = "https"
// 	}
// 
// 	host := c.Request.Host
// 	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
// 		host = forwardedHost
// 	}
// 
// 	return scheme + "://" + host
// }

// RegisterShutdownHook registers a function to be called during server shutdown
func RegisterShutdownHook(hook func()) {
	shutdownHooks = append(shutdownHooks, hook)
}
