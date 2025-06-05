# MCP Multi-Agent Embedding Integration Plan V2 (Clean Implementation)

This plan removes all legacy implementations and focuses on the ideal architecture. No backward compatibility - just the clean, modern multi-agent system.

## Overview
Replace all existing embedding/vector functionality with the multi-agent embedding system (ServiceV2). Remove mock implementations, old endpoints, and legacy code.

## Pre-Implementation Checklist
- [ ] Backup current code: `git checkout -b backup/pre-multi-agent`
- [ ] Ensure database is running: `docker-compose -f docker-compose.local.yml up -d database redis`
- [ ] Have environment variables set (OPENAI_API_KEY required)

## Phase 1: Remove Legacy Code (Priority: Critical)

### Step 1.1: Remove Old Vector Endpoints from REST API
**File**: `apps/rest-api/internal/api/server.go`

Remove these route definitions (around line 400):
```go
// DELETE THESE ROUTES:
vectors := v1.Group("/vectors")
{
    vectors.POST("/store", s.vectorHandlers.Store)
    vectors.POST("/search", s.vectorHandlers.Search)
    vectors.GET("/context/:context_id", s.vectorHandlers.GetByContext)
    // ... any other old vector routes
}
```

### Step 1.2: Delete Old Vector Handlers
**Files to DELETE**:
- `apps/rest-api/internal/api/vector_handlers.go`
- `apps/rest-api/internal/api/vector_handlers_test.go`
- `apps/rest-api/internal/api/server_vector.go`

### Step 1.3: Remove Mock Embedding Service
**Files to DELETE**:
- `pkg/embedding/mock_service.go`
- `pkg/embedding/service.go` (old version, keeping service_v2.go)

### Step 1.4: Clean Up Database Package
**File**: `pkg/database/vector.go`

Remove these methods if they exist:
- `InsertEmbedding` (old version without agent support)
- `SearchEmbeddings` (old version without cross-model support)
- Any methods that don't support agent_id or normalized_embedding

### Step 1.5: Remove Old MCP Vector Endpoints
**File**: `apps/mcp-server/internal/api/server_vector.go`

Delete the entire file - we'll create a new clean version.

**Validation**:
```bash
# Ensure the code still compiles after deletions
cd apps/rest-api && go build ./cmd/api
cd ../mcp-server && go build ./cmd/server
```

## Phase 2: Create Clean REST API Implementation (Priority: High)

### Step 2.1: Create Embedding Service Factory
**File**: `apps/rest-api/internal/adapters/embedding_factory.go`

```go
package adapters

import (
    "context"
    "fmt"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/agents"
    "github.com/S-Corkum/devops-mcp/pkg/cache"
    "github.com/S-Corkum/devops-mcp/pkg/config"
    "github.com/S-Corkum/devops-mcp/pkg/database"
    "github.com/S-Corkum/devops-mcp/pkg/embedding"
    "github.com/S-Corkum/devops-mcp/pkg/embedding/providers"
)

// CreateEmbeddingService creates the multi-agent embedding service
func CreateEmbeddingService(cfg *config.Config, db database.Database, cache cache.Cache) (*embedding.ServiceV2, error) {
    // Initialize providers map
    providerMap := make(map[string]providers.Provider)
    
    // Configure OpenAI if enabled
    if cfg.Embedding.Providers.OpenAI.Enabled && cfg.Embedding.Providers.OpenAI.APIKey != "" {
        openaiCfg := providers.ProviderConfig{
            APIKey:   cfg.Embedding.Providers.OpenAI.APIKey,
            Endpoint: "https://api.openai.com/v1",
            MaxRetries: 3,
            RetryDelayBase: 100 * time.Millisecond,
        }
        
        openaiProvider, err := providers.NewOpenAIProvider(openaiCfg)
        if err != nil {
            return nil, fmt.Errorf("failed to create OpenAI provider: %w", err)
        }
        providerMap["openai"] = openaiProvider
    }
    
    // Configure AWS Bedrock if enabled
    if cfg.Embedding.Providers.Bedrock.Enabled {
        bedrockCfg := providers.ProviderConfig{
            Region:   cfg.Embedding.Providers.Bedrock.Region,
            Endpoint: cfg.Embedding.Providers.Bedrock.Endpoint,
        }
        
        bedrockProvider, err := providers.NewBedrockProvider(bedrockCfg)
        if err != nil {
            return nil, fmt.Errorf("failed to create Bedrock provider: %w", err)
        }
        providerMap["bedrock"] = bedrockProvider
    }
    
    // Configure Google if enabled
    if cfg.Embedding.Providers.Google.Enabled && cfg.Embedding.Providers.Google.APIKey != "" {
        googleCfg := providers.ProviderConfig{
            APIKey:   cfg.Embedding.Providers.Google.APIKey,
            Endpoint: cfg.Embedding.Providers.Google.Endpoint,
        }
        
        googleProvider, err := providers.NewGoogleProvider(googleCfg)
        if err != nil {
            return nil, fmt.Errorf("failed to create Google provider: %w", err)
        }
        providerMap["google"] = googleProvider
    }
    
    // Require at least one provider
    if len(providerMap) == 0 {
        return nil, fmt.Errorf("at least one embedding provider must be configured (OpenAI, Bedrock, or Google)")
    }
    
    // Initialize repositories
    agentRepo := agents.NewRepository(db)
    agentService := agents.NewService(agentRepo)
    embeddingRepo := embedding.NewRepository(db)
    metricsRepo := embedding.NewMetricsRepository(db)
    
    // Create embedding cache adapter
    embeddingCache := NewEmbeddingCacheAdapter(cache)
    
    // Create ServiceV2 - this is our ONLY embedding service
    return embedding.NewServiceV2(embedding.ServiceV2Config{
        Providers:    providerMap,
        AgentService: agentService,
        Repository:   embeddingRepo,
        MetricsRepo:  metricsRepo,
        Cache:        embeddingCache,
    })
}

// EmbeddingCacheAdapter adapts cache.Cache to embedding.EmbeddingCache
type EmbeddingCacheAdapter struct {
    cache cache.Cache
}

func NewEmbeddingCacheAdapter(cache cache.Cache) embedding.EmbeddingCache {
    return &EmbeddingCacheAdapter{cache: cache}
}

func (e *EmbeddingCacheAdapter) Get(ctx context.Context, key string) (*embedding.CachedEmbedding, error) {
    var cached embedding.CachedEmbedding
    err := e.cache.Get(ctx, key, &cached)
    if err != nil {
        return nil, err
    }
    return &cached, nil
}

func (e *EmbeddingCacheAdapter) Set(ctx context.Context, key string, embedding *embedding.CachedEmbedding, ttl time.Duration) error {
    return e.cache.Set(ctx, key, embedding, ttl)
}

func (e *EmbeddingCacheAdapter) Delete(ctx context.Context, key string) error {
    return e.cache.Delete(ctx, key)
}
```

### Step 2.2: Create Clean Embedding API Handlers
**File**: `apps/rest-api/internal/api/embedding_api.go` (replace entire file)

```go
package api

import (
    "net/http"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/agents"
    "github.com/S-Corkum/devops-mcp/pkg/embedding"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

// EmbeddingAPI handles all embedding-related endpoints
type EmbeddingAPI struct {
    embeddingService *embedding.ServiceV2
    agentService     agents.Service
    logger           Logger
}

// NewEmbeddingAPI creates a new embedding API handler
func NewEmbeddingAPI(embeddingService *embedding.ServiceV2, agentService agents.Service, logger Logger) *EmbeddingAPI {
    return &EmbeddingAPI{
        embeddingService: embeddingService,
        agentService:     agentService,
        logger:           logger,
    }
}

// RegisterRoutes registers all embedding routes
func (api *EmbeddingAPI) RegisterRoutes(router *gin.RouterGroup) {
    embeddings := router.Group("/embeddings")
    {
        // Core embedding operations
        embeddings.POST("", api.generateEmbedding)
        embeddings.POST("/batch", api.batchGenerateEmbeddings)
        embeddings.POST("/search", api.searchEmbeddings)
        embeddings.POST("/search/cross-model", api.crossModelSearch)
        
        // Provider management
        embeddings.GET("/providers/health", api.getProviderHealth)
        
        // Agent configuration
        agents := embeddings.Group("/agents")
        {
            agents.POST("", api.createAgentConfig)
            agents.GET("/:agentId", api.getAgentConfig)
            agents.PUT("/:agentId", api.updateAgentConfig)
            agents.GET("/:agentId/models", api.getAgentModels)
            agents.GET("/:agentId/costs", api.getAgentCosts)
        }
    }
}

// generateEmbedding handles POST /api/embeddings
func (api *EmbeddingAPI) generateEmbedding(c *gin.Context) {
    var req embedding.GenerateEmbeddingRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "Invalid request: " + err.Error(),
        })
        return
    }
    
    // Inject tenant ID from auth context
    if tenantID, exists := c.Get("tenant_id"); exists {
        req.TenantID = tenantID.(uuid.UUID)
    }
    
    // Default task type if not specified
    if req.TaskType == "" {
        req.TaskType = agents.TaskTypeGeneralQA
    }
    
    resp, err := api.embeddingService.GenerateEmbedding(c.Request.Context(), req)
    if err != nil {
        api.logger.Error("Failed to generate embedding", 
            "error", err, 
            "agent_id", req.AgentID,
            "text_length", len(req.Text))
        
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "Failed to generate embedding",
        })
        return
    }
    
    c.JSON(http.StatusOK, resp)
}

// batchGenerateEmbeddings handles POST /api/embeddings/batch
func (api *EmbeddingAPI) batchGenerateEmbeddings(c *gin.Context) {
    var reqs []embedding.GenerateEmbeddingRequest
    if err := c.ShouldBindJSON(&reqs); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "Invalid request: " + err.Error(),
        })
        return
    }
    
    // Inject tenant ID for all requests
    if tenantID, exists := c.Get("tenant_id"); exists {
        tid := tenantID.(uuid.UUID)
        for i := range reqs {
            reqs[i].TenantID = tid
            if reqs[i].TaskType == "" {
                reqs[i].TaskType = agents.TaskTypeGeneralQA
            }
        }
    }
    
    resps, err := api.embeddingService.BatchGenerateEmbeddings(c.Request.Context(), reqs)
    if err != nil {
        api.logger.Error("Failed to batch generate embeddings", "error", err, "count", len(reqs))
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "Failed to generate embeddings",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "embeddings": resps,
        "count":      len(resps),
    })
}

// searchEmbeddings handles POST /api/embeddings/search
func (api *EmbeddingAPI) searchEmbeddings(c *gin.Context) {
    var req embedding.SearchRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "Invalid request: " + err.Error(),
        })
        return
    }
    
    // Create search service
    searchService := embedding.NewSearchServiceV2(api.embeddingService, api.embeddingService.Repository)
    
    // Inject tenant ID
    ctx := c.Request.Context()
    if tenantID, exists := c.Get("tenant_id"); exists {
        ctx = context.WithValue(ctx, "tenant_id", tenantID.(uuid.UUID))
    }
    
    results, err := searchService.Search(ctx, req)
    if err != nil {
        api.logger.Error("Search failed", "error", err, "agent_id", req.AgentID)
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "Search failed",
        })
        return
    }
    
    c.JSON(http.StatusOK, results)
}

// crossModelSearch handles POST /api/embeddings/search/cross-model
func (api *EmbeddingAPI) crossModelSearch(c *gin.Context) {
    var req embedding.CrossModelSearchRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "Invalid request: " + err.Error(),
        })
        return
    }
    
    searchService := embedding.NewSearchServiceV2(api.embeddingService, api.embeddingService.Repository)
    
    // Inject tenant ID
    ctx := c.Request.Context()
    if tenantID, exists := c.Get("tenant_id"); exists {
        ctx = context.WithValue(ctx, "tenant_id", tenantID.(uuid.UUID))
    }
    
    results, err := searchService.CrossModelSearch(ctx, req)
    if err != nil {
        api.logger.Error("Cross-model search failed", "error", err)
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "Search failed",
        })
        return
    }
    
    c.JSON(http.StatusOK, results)
}

// getProviderHealth handles GET /api/embeddings/providers/health
func (api *EmbeddingAPI) getProviderHealth(c *gin.Context) {
    health := api.embeddingService.GetProviderHealth(c.Request.Context())
    
    c.JSON(http.StatusOK, gin.H{
        "providers": health,
        "timestamp": time.Now().UTC(),
    })
}

// Agent configuration endpoints

// createAgentConfig handles POST /api/embeddings/agents
func (api *EmbeddingAPI) createAgentConfig(c *gin.Context) {
    var config agents.AgentConfig
    if err := c.ShouldBindJSON(&config); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "Invalid request: " + err.Error(),
        })
        return
    }
    
    // Set defaults
    config.ID = uuid.New()
    config.IsActive = true
    config.CreatedAt = time.Now()
    config.UpdatedAt = time.Now()
    
    if err := api.agentService.CreateConfig(c.Request.Context(), &config); err != nil {
        api.logger.Error("Failed to create agent config", "error", err, "agent_id", config.AgentID)
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "Failed to create agent configuration",
        })
        return
    }
    
    c.JSON(http.StatusCreated, config)
}

// getAgentConfig handles GET /api/embeddings/agents/:agentId
func (api *EmbeddingAPI) getAgentConfig(c *gin.Context) {
    agentID := c.Param("agentId")
    
    config, err := api.agentService.GetConfig(c.Request.Context(), agentID)
    if err != nil {
        c.JSON(http.StatusNotFound, ErrorResponse{
            Error: "Agent configuration not found",
        })
        return
    }
    
    c.JSON(http.StatusOK, config)
}

// updateAgentConfig handles PUT /api/embeddings/agents/:agentId
func (api *EmbeddingAPI) updateAgentConfig(c *gin.Context) {
    agentID := c.Param("agentId")
    
    var update agents.ConfigUpdateRequest
    if err := c.ShouldBindJSON(&update); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "Invalid request: " + err.Error(),
        })
        return
    }
    
    config, err := api.agentService.UpdateConfig(c.Request.Context(), agentID, &update)
    if err != nil {
        api.logger.Error("Failed to update agent config", "error", err, "agent_id", agentID)
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "Failed to update agent configuration",
        })
        return
    }
    
    c.JSON(http.StatusOK, config)
}

// getAgentModels handles GET /api/embeddings/agents/:agentId/models
func (api *EmbeddingAPI) getAgentModels(c *gin.Context) {
    agentID := c.Param("agentId")
    taskTypeStr := c.DefaultQuery("task_type", string(agents.TaskTypeGeneralQA))
    taskType := agents.TaskType(taskTypeStr)
    
    primary, fallback, err := api.agentService.GetModelsForAgent(c.Request.Context(), agentID, taskType)
    if err != nil {
        c.JSON(http.StatusNotFound, ErrorResponse{
            Error: "Agent configuration not found",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "agent_id":        agentID,
        "task_type":       taskType,
        "primary_models":  primary,
        "fallback_models": fallback,
    })
}

// getAgentCosts handles GET /api/embeddings/agents/:agentId/costs
func (api *EmbeddingAPI) getAgentCosts(c *gin.Context) {
    agentID := c.Param("agentId")
    periodDays := c.DefaultQuery("period_days", "30")
    
    // Convert to duration
    days := 30
    if d, err := strconv.Atoi(periodDays); err == nil && d > 0 && d <= 365 {
        days = d
    }
    period := time.Duration(days) * 24 * time.Hour
    
    if api.embeddingService.MetricsRepository == nil {
        c.JSON(http.StatusServiceUnavailable, ErrorResponse{
            Error: "Metrics not available",
        })
        return
    }
    
    costs, err := api.embeddingService.MetricsRepository.GetAgentCosts(c.Request.Context(), agentID, period)
    if err != nil {
        api.logger.Error("Failed to get agent costs", "error", err, "agent_id", agentID)
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "Failed to retrieve cost data",
        })
        return
    }
    
    c.JSON(http.StatusOK, costs)
}
```

### Step 2.3: Update REST API Server
**File**: `apps/rest-api/internal/api/server.go`

Replace the server struct and initialization:
```go
type Server struct {
    router          *gin.Engine
    logger          Logger
    cfg             *config.Config
    db              database.Database
    cache           cache.Cache
    embeddingAPI    *EmbeddingAPI  // Only embedding API, no legacy handlers
    contextHandlers *context.ContextHandlers
    authMiddleware  gin.HandlerFunc
    webhookHandlers map[string]webhooks.WebhookHandler
}

// In NewServer function, replace embedding initialization:
// Remove all old vector/embedding initialization
// Add this instead:
embeddingService, err := adapters.CreateEmbeddingService(cfg, db, cache)
if err != nil {
    return nil, fmt.Errorf("failed to create embedding service: %w", err)
}

agentService := agents.NewService(agents.NewRepository(db))
embeddingAPI := NewEmbeddingAPI(embeddingService, agentService, logger)

s := &Server{
    router:       gin.New(),
    logger:       logger,
    cfg:          cfg,
    db:           db,
    cache:        cache,
    embeddingAPI: embeddingAPI,
    // ... other fields
}

// In setupRoutes, replace all vector routes with:
api := s.router.Group("/api")
api.Use(s.authMiddleware)
{
    // Only the modern embedding API
    s.embeddingAPI.RegisterRoutes(api)
    
    // Other APIs (contexts, etc.)
    s.contextHandlers.RegisterRoutes(api)
}
```

## Phase 3: Clean MCP Server Implementation (Priority: High)

### Step 3.1: Create Clean MCP Embedding Proxy
**File**: `apps/mcp-server/internal/api/proxies/embedding_proxy.go` (new file)

```go
package proxies

import (
    "context"
    "fmt"
    
    "github.com/S-Corkum/devops-mcp/pkg/agents"
    "github.com/S-Corkum/devops-mcp/pkg/client/rest"
    "github.com/S-Corkum/devops-mcp/pkg/embedding"
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/google/uuid"
)

// EmbeddingProxy handles all embedding operations through the REST API
type EmbeddingProxy struct {
    client rest.Client
    logger Logger
}

// NewEmbeddingProxy creates a new embedding proxy
func NewEmbeddingProxy(client rest.Client, logger Logger) *EmbeddingProxy {
    return &EmbeddingProxy{
        client: client,
        logger: logger,
    }
}

// GenerateEmbedding generates an embedding for the given text
func (p *EmbeddingProxy) GenerateEmbedding(ctx context.Context, agentID, text string, contextID *uuid.UUID) (*embedding.GenerateEmbeddingResponse, error) {
    req := embedding.GenerateEmbeddingRequest{
        AgentID:  agentID,
        Text:     text,
        TaskType: agents.TaskTypeGeneralQA, // Default, can be enhanced
    }
    
    if contextID != nil {
        req.ContextID = *contextID
    }
    
    // Extract tenant ID from context
    if tenantID, ok := ctx.Value("tenant_id").(uuid.UUID); ok {
        req.TenantID = tenantID
    }
    
    var resp embedding.GenerateEmbeddingResponse
    if err := p.client.Post(ctx, "/api/embeddings", req, &resp); err != nil {
        return nil, fmt.Errorf("failed to generate embedding: %w", err)
    }
    
    return &resp, nil
}

// Search performs semantic search
func (p *EmbeddingProxy) Search(ctx context.Context, agentID, query string, limit int) ([]*models.SearchResult, error) {
    req := embedding.SearchRequest{
        AgentID: agentID,
        Query:   query,
        Limit:   limit,
    }
    
    if limit == 0 {
        req.Limit = 10
    }
    
    var resp embedding.SearchResponse
    if err := p.client.Post(ctx, "/api/embeddings/search", req, &resp); err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }
    
    // Convert to models.SearchResult
    results := make([]*models.SearchResult, len(resp.Results))
    for i, r := range resp.Results {
        results[i] = &models.SearchResult{
            ID:         r.ID,
            Content:    r.Content,
            Similarity: r.Similarity,
            Metadata:   r.Metadata,
        }
    }
    
    return results, nil
}

// CrossModelSearch performs search across different embedding models
func (p *EmbeddingProxy) CrossModelSearch(ctx context.Context, query string, opts embedding.CrossModelSearchOptions) (*embedding.CrossModelSearchResponse, error) {
    req := embedding.CrossModelSearchRequest{
        Query:          query,
        SearchModel:    opts.SearchModel,
        IncludeModels:  opts.IncludeModels,
        ExcludeModels:  opts.ExcludeModels,
        Limit:          opts.Limit,
        MinSimilarity:  opts.MinSimilarity,
        MetadataFilter: opts.MetadataFilter,
    }
    
    var resp embedding.CrossModelSearchResponse
    if err := p.client.Post(ctx, "/api/embeddings/search/cross-model", req, &resp); err != nil {
        return nil, fmt.Errorf("cross-model search failed: %w", err)
    }
    
    return &resp, nil
}

// GetProviderHealth returns health status of embedding providers
func (p *EmbeddingProxy) GetProviderHealth(ctx context.Context) (map[string]embedding.ProviderHealth, error) {
    var resp struct {
        Providers map[string]embedding.ProviderHealth `json:"providers"`
    }
    
    if err := p.client.Get(ctx, "/api/embeddings/providers/health", &resp); err != nil {
        return nil, fmt.Errorf("failed to get provider health: %w", err)
    }
    
    return resp.Providers, nil
}
```

### Step 3.2: Create Clean MCP Embedding Handlers
**File**: `apps/mcp-server/internal/api/mcp_embedding_handlers.go` (new file)

```go
package api

import (
    "context"
    "encoding/json"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/google/uuid"
)

// MCPEmbeddingHandlers handles MCP protocol embedding requests
type MCPEmbeddingHandlers struct {
    embeddingProxy *proxies.EmbeddingProxy
    logger         Logger
}

// NewMCPEmbeddingHandlers creates new MCP embedding handlers
func NewMCPEmbeddingHandlers(proxy *proxies.EmbeddingProxy, logger Logger) *MCPEmbeddingHandlers {
    return &MCPEmbeddingHandlers{
        embeddingProxy: proxy,
        logger:         logger,
    }
}

// HandleEmbeddingRequest processes MCP embedding requests
func (h *MCPEmbeddingHandlers) HandleEmbeddingRequest(ctx context.Context, req json.RawMessage) (interface{}, error) {
    var baseReq struct {
        Action string `json:"action"`
    }
    
    if err := json.Unmarshal(req, &baseReq); err != nil {
        return nil, fmt.Errorf("invalid request format: %w", err)
    }
    
    switch baseReq.Action {
    case "generate_embedding":
        return h.handleGenerateEmbedding(ctx, req)
    case "search":
        return h.handleSearch(ctx, req)
    case "cross_model_search":
        return h.handleCrossModelSearch(ctx, req)
    case "provider_health":
        return h.handleProviderHealth(ctx, req)
    default:
        return nil, fmt.Errorf("unknown embedding action: %s", baseReq.Action)
    }
}

func (h *MCPEmbeddingHandlers) handleGenerateEmbedding(ctx context.Context, req json.RawMessage) (interface{}, error) {
    var request struct {
        Action    string    `json:"action"`
        AgentID   string    `json:"agent_id"`
        Text      string    `json:"text"`
        ContextID *uuid.UUID `json:"context_id,omitempty"`
    }
    
    if err := json.Unmarshal(req, &request); err != nil {
        return nil, fmt.Errorf("invalid generate embedding request: %w", err)
    }
    
    if request.AgentID == "" {
        return nil, fmt.Errorf("agent_id is required")
    }
    
    if request.Text == "" {
        return nil, fmt.Errorf("text is required")
    }
    
    resp, err := h.embeddingProxy.GenerateEmbedding(ctx, request.AgentID, request.Text, request.ContextID)
    if err != nil {
        h.logger.Error("Failed to generate embedding", "error", err, "agent_id", request.AgentID)
        return nil, err
    }
    
    return map[string]interface{}{
        "embedding_id": resp.EmbeddingID,
        "model_used":   resp.ModelUsed,
        "provider":     resp.Provider,
        "dimensions":   resp.Dimensions,
        "cached":       resp.Cached,
        "cost_usd":     resp.CostUSD,
    }, nil
}

func (h *MCPEmbeddingHandlers) handleSearch(ctx context.Context, req json.RawMessage) (interface{}, error) {
    var request struct {
        Action  string `json:"action"`
        AgentID string `json:"agent_id"`
        Query   string `json:"query"`
        Limit   int    `json:"limit,omitempty"`
    }
    
    if err := json.Unmarshal(req, &request); err != nil {
        return nil, fmt.Errorf("invalid search request: %w", err)
    }
    
    if request.AgentID == "" {
        return nil, fmt.Errorf("agent_id is required")
    }
    
    if request.Query == "" {
        return nil, fmt.Errorf("query is required")
    }
    
    results, err := h.embeddingProxy.Search(ctx, request.AgentID, request.Query, request.Limit)
    if err != nil {
        h.logger.Error("Search failed", "error", err, "agent_id", request.AgentID)
        return nil, err
    }
    
    return map[string]interface{}{
        "results": results,
        "count":   len(results),
    }, nil
}

func (h *MCPEmbeddingHandlers) handleCrossModelSearch(ctx context.Context, req json.RawMessage) (interface{}, error) {
    var request struct {
        Action         string                 `json:"action"`
        Query          string                 `json:"query"`
        SearchModel    string                 `json:"search_model,omitempty"`
        IncludeModels  []string               `json:"include_models,omitempty"`
        ExcludeModels  []string               `json:"exclude_models,omitempty"`
        Limit          int                    `json:"limit,omitempty"`
        MinSimilarity  float64                `json:"min_similarity,omitempty"`
        MetadataFilter map[string]interface{} `json:"metadata_filter,omitempty"`
    }
    
    if err := json.Unmarshal(req, &request); err != nil {
        return nil, fmt.Errorf("invalid cross-model search request: %w", err)
    }
    
    if request.Query == "" {
        return nil, fmt.Errorf("query is required")
    }
    
    opts := embedding.CrossModelSearchOptions{
        SearchModel:    request.SearchModel,
        IncludeModels:  request.IncludeModels,
        ExcludeModels:  request.ExcludeModels,
        Limit:          request.Limit,
        MinSimilarity:  request.MinSimilarity,
        MetadataFilter: request.MetadataFilter,
    }
    
    resp, err := h.embeddingProxy.CrossModelSearch(ctx, request.Query, opts)
    if err != nil {
        h.logger.Error("Cross-model search failed", "error", err)
        return nil, err
    }
    
    return resp, nil
}

func (h *MCPEmbeddingHandlers) handleProviderHealth(ctx context.Context, req json.RawMessage) (interface{}, error) {
    health, err := h.embeddingProxy.GetProviderHealth(ctx)
    if err != nil {
        return nil, err
    }
    
    return map[string]interface{}{
        "providers": health,
        "timestamp": time.Now().UTC(),
    }, nil
}
```

### Step 3.3: Update MCP Server Router
**File**: `apps/mcp-server/internal/api/mcp_api.go`

Update the MCP request router to use the new embedding handlers:
```go
// In the MCP API handler initialization
embeddingProxy := proxies.NewEmbeddingProxy(restClient, logger)
embeddingHandlers := NewMCPEmbeddingHandlers(embeddingProxy, logger)

// In the request router
switch requestType {
case "embedding", "embeddings":
    return embeddingHandlers.HandleEmbeddingRequest(ctx, request)
// ... other cases
}
```

## Phase 4: Configuration Updates (Priority: High)

### Step 4.1: Update Configuration Structure
**File**: `pkg/config/config.go`

Add embedding configuration:
```go
type Config struct {
    // ... existing fields ...
    Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`
}

type EmbeddingConfig struct {
    Providers struct {
        OpenAI struct {
            Enabled bool   `yaml:"enabled" json:"enabled"`
            APIKey  string `yaml:"api_key" json:"api_key" env:"OPENAI_API_KEY"`
        } `yaml:"openai" json:"openai"`
        Bedrock struct {
            Enabled  bool   `yaml:"enabled" json:"enabled"`
            Region   string `yaml:"region" json:"region" env:"AWS_REGION"`
            Endpoint string `yaml:"endpoint" json:"endpoint" env:"AWS_ENDPOINT_URL"`
        } `yaml:"bedrock" json:"bedrock"`
        Google struct {
            Enabled bool   `yaml:"enabled" json:"enabled"`
            APIKey  string `yaml:"api_key" json:"api_key" env:"GOOGLE_API_KEY"`
        } `yaml:"google" json:"google"`
    } `yaml:"providers" json:"providers"`
}
```

### Step 4.2: Update Base Configuration
**File**: `configs/config.base.yaml`

```yaml
# Embedding Configuration
embedding:
  providers:
    openai:
      enabled: true
      api_key: ${OPENAI_API_KEY}
    bedrock:
      enabled: false
      region: ${AWS_REGION:-us-east-1}
    google:
      enabled: false
      api_key: ${GOOGLE_API_KEY}
```

### Step 4.3: Update Environment File
**File**: `.env.example`

```bash
# Embedding Providers (at least one required)
# Option 1: OpenAI
OPENAI_API_KEY=your-openai-api-key-here

# Option 2: AWS Bedrock
# AWS_REGION=us-east-1
# AWS_ACCESS_KEY_ID=your-access-key
# AWS_SECRET_ACCESS_KEY=your-secret-key

# Option 3: Google AI
# GOOGLE_API_KEY=your-google-api-key
```

## Phase 5: Documentation & API Specification Updates (Priority: High)

### Step 5.1: Remove Legacy Swagger Definitions
**Files to UPDATE**:

**File**: `docs/swagger/openapi.yaml`
- Remove all `/api/v1/vectors/*` path definitions
- Remove old vector-related schemas (VectorStoreRequest, VectorSearchRequest, etc.)
- Add reference to new embeddings API: `$ref: './core/embeddings_v2.yaml'`

**File**: `docs/swagger/core/vectors.yaml`
- DELETE this entire file (replaced by embeddings_v2.yaml)

### Step 5.2: Update Main API Documentation
**File**: `docs/api-reference/rest-api-reference.md`

Replace entire "Vector Operations" section with:

```markdown
## Embedding Operations

The DevOps MCP uses a multi-agent embedding system that provides intelligent routing, cross-model compatibility, and cost optimization.

### Key Concepts

- **Agent Configuration**: Each AI agent has its own embedding preferences and constraints
- **Smart Routing**: Automatic selection of best provider based on agent strategy
- **Cross-Model Search**: Search across embeddings from different models
- **Dimension Normalization**: All embeddings normalized to 1536 dimensions for compatibility

### Endpoints

#### Generate Embedding
`POST /api/embeddings`

Generate an embedding for the specified agent and text.

**Request Body:**
```json
{
  "agent_id": "claude-assistant",
  "text": "Content to embed",
  "task_type": "general_qa",
  "context_id": "ctx_123"
}
```

**Response:**
```json
{
  "embedding_id": "emb_789",
  "model_used": "text-embedding-3-small",
  "provider": "openai",
  "dimensions": 1536,
  "cached": false,
  "cost_usd": 0.00002
}
```

#### Cross-Model Search
`POST /api/embeddings/search/cross-model`

Search across embeddings created by different models.

**Request Body:**
```json
{
  "query": "search query",
  "search_model": "text-embedding-3-small",
  "include_models": ["voyage-2", "text-embedding-ada-002"],
  "limit": 10
}
```

See the [Embedding API Reference](./embedding-api-reference.md) for complete details.
```

### Step 5.3: Create New Embedding API Reference
**File**: `docs/api-reference/embedding-api-reference.md` (new file)

```markdown
# Embedding API Reference

The multi-agent embedding system provides sophisticated embedding generation and search capabilities with provider failover, cost optimization, and cross-model compatibility.

## Overview

Each AI agent can have customized embedding configurations including:
- Preferred models for different task types
- Embedding strategy (quality, speed, cost, balanced)
- Cost constraints and rate limits
- Fallback behavior

## API Endpoints

### Generate Embedding
`POST /api/embeddings`

Generate an embedding using agent-specific configuration.

**Headers:**
- `X-API-Key`: Your API key (required)
- `X-Tenant-ID`: Tenant UUID (required)

**Request Body:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| agent_id | string | Yes | The agent identifier |
| text | string | Yes | Text to generate embedding for (max 50,000 chars) |
| task_type | string | No | Task type: general_qa, code_analysis, multilingual, research, structured_data |
| context_id | uuid | No | Associate embedding with a context |
| metadata | object | No | Additional metadata to store |

**Response:**
```json
{
  "embedding_id": "550e8400-e29b-41d4-a716-446655440000",
  "request_id": "req_123",
  "model_used": "text-embedding-3-small",
  "provider": "openai",
  "dimensions": 1536,
  "normalized_dimensions": 1536,
  "cost_usd": 0.00002,
  "tokens_used": 127,
  "generation_time_ms": 145,
  "cached": false,
  "metadata": {}
}
```

### Batch Generate Embeddings
`POST /api/embeddings/batch`

Generate multiple embeddings in a single request.

**Request Body:**
Array of embedding requests (same structure as single request)

**Response:**
```json
{
  "embeddings": [...],
  "count": 10
}
```

### Search Embeddings
`POST /api/embeddings/search`

Search for similar content using agent-specific models.

**Request Body:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| agent_id | string | Yes | The agent identifier |
| query | string | Yes | Search query text |
| limit | integer | No | Max results (1-100, default: 10) |
| threshold | float | No | Minimum similarity (0-1, default: 0.7) |
| metadata_filter | object | No | Filter by metadata fields |

### Cross-Model Search
`POST /api/embeddings/search/cross-model`

Search across embeddings from different models using dimension normalization.

**Request Body:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| query | string | Yes | Search query text |
| search_model | string | No | Model to use for query embedding |
| include_models | array | No | Models to include in search |
| exclude_models | array | No | Models to exclude from search |
| limit | integer | No | Max results (1-100, default: 10) |
| min_similarity | float | No | Minimum similarity (0-1, default: 0.7) |

### Provider Health
`GET /api/embeddings/providers/health`

Get health status of all configured embedding providers.

**Response:**
```json
{
  "providers": {
    "openai": {
      "name": "openai",
      "status": "healthy",
      "circuit_breaker_state": "closed",
      "failure_count": 0
    },
    "bedrock": {
      "name": "bedrock", 
      "status": "unhealthy",
      "error": "connection timeout",
      "circuit_breaker_state": "open",
      "failure_count": 5
    }
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Agent Configuration

### Create Agent Configuration
`POST /api/embeddings/agents`

Create embedding configuration for a new agent.

**Request Body:**
```json
{
  "agent_id": "my-agent",
  "embedding_strategy": "balanced",
  "model_preferences": [
    {
      "task_type": "general_qa",
      "primary_models": ["text-embedding-3-small"],
      "fallback_models": ["text-embedding-ada-002"]
    }
  ],
  "constraints": {
    "max_cost_per_month_usd": 100.0,
    "rate_limits": {
      "requests_per_minute": 100
    }
  }
}
```

### Get Agent Configuration
`GET /api/embeddings/agents/{agentId}`

### Update Agent Configuration
`PUT /api/embeddings/agents/{agentId}`

### Get Agent Models
`GET /api/embeddings/agents/{agentId}/models?task_type=general_qa`

### Get Agent Costs
`GET /api/embeddings/agents/{agentId}/costs?period_days=30`

## Supported Models

### OpenAI
- `text-embedding-3-small` (1536 dims, $0.02/1M tokens)
- `text-embedding-3-large` (3072 dims, $0.13/1M tokens)
- `text-embedding-ada-002` (1536 dims, $0.10/1M tokens)

### AWS Bedrock
- `amazon.titan-embed-text-v1` (1536 dims)
- `amazon.titan-embed-text-v2:0` (1024 dims)

### Google AI
- `text-embedding-004` (768 dims)
- `text-multilingual-embedding-002` (768 dims)

## Error Responses

All errors follow this format:
```json
{
  "error": "Error message",
  "details": "Additional context"
}
```

Common HTTP status codes:
- 400: Bad Request (invalid input)
- 401: Unauthorized (missing/invalid API key)
- 404: Not Found (agent not configured)
- 429: Too Many Requests (rate limit exceeded)
- 500: Internal Server Error
- 503: Service Unavailable (no providers available)
```

### Step 5.4: Update MCP Server Documentation
**File**: `docs/api-reference/mcp-server-reference.md`

Update the "Embedding Operations" section:

```markdown
## Embedding Operations

The MCP Server provides embedding operations through the multi-agent embedding system. All requests are routed to the REST API's v2 embedding endpoints.

### Generate Embedding

**Request:**
```json
{
  "action": "generate_embedding",
  "agent_id": "claude-assistant",
  "text": "Content to embed",
  "context_id": "ctx_123"
}
```

**Response:**
```json
{
  "embedding_id": "550e8400-e29b-41d4-a716-446655440000",
  "model_used": "text-embedding-3-large",
  "provider": "openai",
  "dimensions": 3072,
  "cached": false,
  "cost_usd": 0.00013
}
```

### Search

**Request:**
```json
{
  "action": "search",
  "agent_id": "claude-assistant",
  "query": "kubernetes deployment",
  "limit": 10
}
```

### Cross-Model Search

**Request:**
```json
{
  "action": "cross_model_search",
  "query": "deployment strategies",
  "include_models": ["text-embedding-3-small", "voyage-2"],
  "limit": 20
}
```

### Provider Health Check

**Request:**
```json
{
  "action": "provider_health"
}
```

All embedding operations require an `agent_id` to determine which models and strategies to use.
```

### Step 5.5: Clean Up Legacy Documentation
**Files to DELETE or update**:
- `docs/examples/vector-search-implementation.md` - Replace with multi-agent examples
- Any references to `/api/v1/vectors` in documentation
- Any examples using mock embeddings

### Step 5.6: Update README Files
**File**: `README.md` (main project README)

Update the features section:
```markdown
## Key Features

- **Multi-Agent Embedding System**: Each AI agent can have customized embedding models and strategies
- **Intelligent Provider Routing**: Automatic failover between OpenAI, AWS Bedrock, and Google AI
- **Cross-Model Search**: Search across embeddings created by different models
- **Cost Optimization**: Track and optimize embedding costs per agent
```

**File**: `apps/rest-api/README.md`

Add section:
```markdown
## Embedding System

The REST API provides a sophisticated multi-agent embedding system:

- Agent-specific model configuration
- Smart routing between providers (OpenAI, Bedrock, Google)
- Cross-model search with dimension normalization
- Cost tracking and optimization
- Circuit breaker pattern for resilience

Configuration requires at least one embedding provider. See `.env.example` for setup.
```

### Step 5.7: Update Configuration Documentation
**File**: `docs/operations/configuration-guide.md`

Add new section:
```markdown
## Embedding Configuration

The embedding system requires at least one provider to be configured:

### OpenAI Configuration
```yaml
embedding:
  providers:
    openai:
      enabled: true
      api_key: ${OPENAI_API_KEY}
```

### AWS Bedrock Configuration
```yaml
embedding:
  providers:
    bedrock:
      enabled: true
      region: us-east-1
      # Uses standard AWS credential chain
```

### Google AI Configuration
```yaml
embedding:
  providers:
    google:
      enabled: true
      api_key: ${GOOGLE_API_KEY}
```

### Agent Configuration Example
```yaml
# Create via API
POST /api/embeddings/agents
{
  "agent_id": "production-claude",
  "embedding_strategy": "quality",
  "model_preferences": [
    {
      "task_type": "general_qa",
      "primary_models": ["text-embedding-3-large"],
      "fallback_models": ["text-embedding-3-small"]
    },
    {
      "task_type": "code_analysis",
      "primary_models": ["voyage-code-2"],
      "fallback_models": ["text-embedding-3-large"]
    }
  ],
  "constraints": {
    "max_cost_per_month_usd": 500.0
  }
}
```

## Phase 6: Testing & Validation (Priority: Critical)

### Step 5.1: Create Integration Test
**File**: `test/integration/embedding_integration_test.go`

```go
package integration

import (
    "context"
    "testing"
    
    "github.com/S-Corkum/devops-mcp/pkg/agents"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEmbeddingIntegration(t *testing.T) {
    ctx := context.Background()
    
    // Step 1: Create agent configuration
    agentConfig := &agents.AgentConfig{
        AgentID:           "test-agent-001",
        EmbeddingStrategy: agents.StrategyQuality,
        ModelPreferences: []agents.ModelPreference{
            {
                TaskType:      agents.TaskTypeGeneralQA,
                PrimaryModels: []string{"text-embedding-3-small"},
            },
        },
    }
    
    err := restClient.CreateAgentConfig(ctx, agentConfig)
    require.NoError(t, err)
    
    // Step 2: Generate embedding through REST API
    embResp, err := restClient.GenerateEmbedding(ctx, embedding.GenerateEmbeddingRequest{
        AgentID: "test-agent-001",
        Text:    "This is a test embedding",
    })
    require.NoError(t, err)
    assert.NotEmpty(t, embResp.EmbeddingID)
    assert.Equal(t, "text-embedding-3-small", embResp.ModelUsed)
    
    // Step 3: Test MCP integration
    mcpResp, err := mcpClient.Request(ctx, map[string]interface{}{
        "action":   "generate_embedding",
        "agent_id": "test-agent-001",
        "text":     "MCP test embedding",
    })
    require.NoError(t, err)
    assert.NotEmpty(t, mcpResp["embedding_id"])
    
    // Step 4: Test search functionality
    searchResp, err := restClient.SearchEmbeddings(ctx, embedding.SearchRequest{
        AgentID: "test-agent-001",
        Query:   "test",
        Limit:   10,
    })
    require.NoError(t, err)
    assert.NotEmpty(t, searchResp.Results)
}
```

### Step 5.2: Manual Test Script
**File**: `scripts/test-embedding-system.sh`

```bash
#!/bin/bash
set -e

API_URL="http://localhost:8081/api"
MCP_URL="http://localhost:8080/api/v1"
API_KEY="dev-api-key"

echo "=== Testing Multi-Agent Embedding System ==="

# 1. Health check
echo "1. Checking provider health..."
curl -s -X GET "$API_URL/embeddings/providers/health" \
  -H "X-API-Key: $API_KEY" | jq .

# 2. Create agent config
echo "2. Creating agent configuration..."
curl -s -X POST "$API_URL/embeddings/agents" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "agent_id": "claude-assistant",
    "embedding_strategy": "quality",
    "model_preferences": [{
      "task_type": "general_qa",
      "primary_models": ["text-embedding-3-large"],
      "fallback_models": ["text-embedding-3-small"]
    }]
  }' | jq .

# 3. Generate embedding via REST
echo "3. Generating embedding via REST API..."
RESP=$(curl -s -X POST "$API_URL/embeddings" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "agent_id": "claude-assistant",
    "text": "What is the capital of France?"
  }')
echo "$RESP" | jq .
EMBEDDING_ID=$(echo "$RESP" | jq -r .embedding_id)

# 4. Test MCP integration
echo "4. Testing MCP embedding generation..."
curl -s -X POST "$MCP_URL/request" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "action": "generate_embedding",
    "agent_id": "claude-assistant",
    "text": "Paris is the capital of France"
  }' | jq .

# 5. Search test
echo "5. Testing search..."
curl -s -X POST "$API_URL/embeddings/search" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "agent_id": "claude-assistant",
    "query": "capital city",
    "limit": 5
  }' | jq .

echo "=== All tests completed successfully! ==="
```

## Cleanup Checklist

After implementation, verify these files/features have been removed or updated:

### Code Files Deleted:
- [ ] ❌ `apps/rest-api/internal/api/vector_handlers.go` - DELETED
- [ ] ❌ `apps/rest-api/internal/api/server_vector.go` - DELETED  
- [ ] ❌ `apps/mcp-server/internal/api/server_vector.go` - DELETED
- [ ] ❌ `pkg/embedding/mock_service.go` - DELETED
- [ ] ❌ `pkg/embedding/service.go` (old version) - DELETED

### API Changes:
- [ ] ❌ Old `/api/v1/vectors/*` endpoints - REMOVED
- [ ] ❌ Mock embedding generation code - REMOVED
- [ ] ❌ Deterministic vector generation - REMOVED

### Documentation Cleanup:
- [ ] ❌ `docs/swagger/core/vectors.yaml` - DELETED
- [ ] ❌ Old vector references in `docs/swagger/openapi.yaml` - REMOVED
- [ ] ❌ `docs/examples/vector-search-implementation.md` - DELETED or UPDATED
- [ ] ✅ `docs/api-reference/rest-api-reference.md` - UPDATED with new embedding API
- [ ] ✅ `docs/api-reference/mcp-server-reference.md` - UPDATED with new embedding operations
- [ ] ✅ `docs/api-reference/embedding-api-reference.md` - CREATED
- [ ] ✅ Main `README.md` - UPDATED with multi-agent features
- [ ] ✅ `apps/rest-api/README.md` - UPDATED with embedding system info
- [ ] ✅ `docs/operations/configuration-guide.md` - UPDATED with embedding config

## Success Criteria

1. ✅ Only ServiceV2 is used for all embedding operations
2. ✅ All embedding requests require agent_id
3. ✅ Real OpenAI embeddings are generated (not mocks)
4. ✅ MCP Server routes all requests through the new system
5. ✅ Cross-model search works properly
6. ✅ Provider health monitoring is functional
7. ✅ No legacy vector code remains

## Benefits of Clean Implementation

1. **Simpler Architecture**: One embedding system, not multiple
2. **Consistent Behavior**: All embeddings go through agent configuration
3. **Better Performance**: Proper caching and routing from day one
4. **Easier Maintenance**: No legacy code to maintain
5. **Clear Mental Model**: Developers understand there's only one way to do embeddings