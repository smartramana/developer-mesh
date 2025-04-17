package api

import (
	"context"
	"net/http"

	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/gin-gonic/gin"
)

// Server represents the API server
type Server struct {
	router               *gin.Engine
	server               *http.Server
	engine               *core.Engine
	config               Config
	embeddingRepo        *repository.EmbeddingRepository
	
	// Handler functions (for testing overrides)
	storeEmbedding       func(c *gin.Context)
	searchEmbeddings     func(c *gin.Context)
	getContextEmbeddings func(c *gin.Context)
	deleteContextEmbeddings func(c *gin.Context)
}

// NewServer creates a new API server
func NewServer(engine *core.Engine, embeddingRepo *repository.EmbeddingRepository, cfg Config) *Server {
	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(RequestLogger())
	router.Use(MetricsMiddleware())

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

	server := &Server{
		router:       router,
		engine:       engine,
		embeddingRepo: embeddingRepo,
		config:       cfg,
		server:       &http.Server{
			Addr:         cfg.ListenAddress,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}
	
	// Initialize default handler functions
	server.storeEmbedding = server.handleStoreEmbedding
	server.searchEmbeddings = server.handleSearchEmbeddings
	server.getContextEmbeddings = server.handleGetContextEmbeddings
	server.deleteContextEmbeddings = server.handleDeleteContextEmbeddings

	// Initialize routes
	server.setupRoutes()

	return server
}

// setupRoutes initializes all API routes
func (s *Server) setupRoutes() {
	// Public endpoints
	s.router.GET("/health", s.healthHandler)
	
	// Metrics endpoints - add authentication
	s.router.GET("/metrics", AuthMiddleware("api_key"), s.metricsHandler)

	// API v1 routes - require authentication
	v1 := s.router.Group("/api/v1")
	v1.Use(AuthMiddleware("jwt")) // Require JWT auth for all API endpoints
	
	// Context management API
	contextAPI := NewContextAPI(s.engine.ContextManager)
	contextAPI.RegisterRoutes(v1)
	
	// Tool integration API
	toolAPI := NewToolAPI(s.engine.AdapterBridge)
	toolAPI.RegisterRoutes(v1)
	
	// Vector API endpoints
	vectorRoutes := v1.Group("/vectors")
	{
		vectorRoutes.POST("/store", s.storeEmbedding)
		vectorRoutes.POST("/search", s.searchEmbeddings)
		vectorRoutes.GET("/context/:context_id", s.getContextEmbeddings)
		vectorRoutes.DELETE("/context/:context_id", s.deleteContextEmbeddings)
	}
	
	// Webhook endpoints - each has its own authentication via secret validation
	webhook := s.router.Group("/webhook")
	{
		// AI agent event webhook
		webhook.POST("/agent", s.agentWebhookHandler)
		
		// DevOps tool webhooks - only GitHub is supported
		if s.config.Webhooks.GitHub.Enabled {
			path := "/github"
			if s.config.Webhooks.GitHub.Path != "" {
				path = s.config.Webhooks.GitHub.Path
			}
			webhook.POST(path, s.githubWebhookHandler)
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

// Vector operations handler functions
// These are the actual implementations that will be used by default

func (s *Server) handleStoreEmbedding(c *gin.Context) {
	var req StoreEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	embedding := &repository.Embedding{
		ContextID:    req.ContextID,
		ContentIndex: req.ContentIndex,
		Text:         req.Text,
		Embedding:    req.Embedding,
		ModelID:      req.ModelID,
	}
	
	err := s.embeddingRepo.StoreEmbedding(c.Request.Context(), embedding)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, embedding)
}

func (s *Server) handleSearchEmbeddings(c *gin.Context) {
	var req SearchEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if req.Limit <= 0 {
		req.Limit = 10 // Default limit
	}
	
	embeddings, err := s.embeddingRepo.SearchEmbeddings(c.Request.Context(), req.QueryEmbedding, req.ContextID, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

func (s *Server) handleGetContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	embeddings, err := s.embeddingRepo.GetContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

func (s *Server) handleDeleteContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	err := s.embeddingRepo.DeleteContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
