package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/agents"
	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EmbeddingAPI handles all embedding-related endpoints
type EmbeddingAPI struct {
	embeddingService *embedding.ServiceV2
	agentService     *agents.Service
	logger           observability.Logger
}

// NewEmbeddingAPI creates a new embedding API handler
func NewEmbeddingAPI(embeddingService *embedding.ServiceV2, agentService *agents.Service, logger observability.Logger) *EmbeddingAPI {
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
		agentsGroup := embeddings.Group("/agents")
		{
			agentsGroup.POST("", api.createAgentConfig)
			agentsGroup.GET("/:agentId", api.getAgentConfig)
			agentsGroup.PUT("/:agentId", api.updateAgentConfig)
			agentsGroup.GET("/:agentId/models", api.getAgentModels)
			agentsGroup.GET("/:agentId/costs", api.getAgentCosts)
		}
	}
}

// generateEmbedding handles POST /api/embeddings
func (api *EmbeddingAPI) generateEmbedding(c *gin.Context) {
	var req embedding.GenerateEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
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
		api.logger.Error("Failed to generate embedding", map[string]any{
			"error":       err.Error(),
			"agent_id":    sanitizeLogValue(req.AgentID),
			"text_length": len(req.Text),
		})

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to generate embedding",
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
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
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
		api.logger.Error("Failed to batch generate embeddings", map[string]any{
			"error": err.Error(),
			"count": len(reqs),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to generate embeddings",
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
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Create search service adapter
	searchService := NewSearchServiceAdapter(api.embeddingService, api.logger)
	
	// Inject tenant ID from auth context if not provided
	if req.TenantID == uuid.Nil {
		if tenantID, exists := c.Get("tenant_id"); exists {
			req.TenantID = tenantID.(uuid.UUID)
		}
	}
	
	// Perform search
	results, err := searchService.Search(c.Request.Context(), req)
	if err != nil {
		api.logger.Error("Search failed", map[string]any{
			"error": err.Error(),
			"tenant_id": req.TenantID,
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Search operation failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

// crossModelSearch handles POST /api/embeddings/search/cross-model
func (api *EmbeddingAPI) crossModelSearch(c *gin.Context) {
	var req embedding.CrossModelSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Create search service adapter
	searchService := NewSearchServiceAdapter(api.embeddingService, api.logger)
	
	// Convert to standard search request for adapter
	searchReq := embedding.SearchRequest{
		QueryEmbedding: req.QueryEmbedding,
		ModelName:      req.SearchModel,
		TenantID:       req.TenantID,
		ContextID:      req.ContextID,
		Limit:          req.Limit,
		Threshold:      req.MinSimilarity,
	}
	
	// Add metadata filter if provided
	if len(req.MetadataFilter) > 0 {
		filterJSON, err := json.Marshal(req.MetadataFilter)
		if err == nil {
			searchReq.MetadataFilter = filterJSON
		}
	}
	
	// Perform search
	results, err := searchService.Search(c.Request.Context(), searchReq)
	if err != nil {
		api.logger.Error("Cross-model search failed", map[string]any{
			"error": err.Error(),
			"tenant_id": req.TenantID,
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Cross-model search operation failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
		"search_model": req.SearchModel,
	})
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
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Set defaults
	config.ID = uuid.New()
	config.IsActive = true
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	if err := api.agentService.CreateConfig(c.Request.Context(), &config); err != nil {
		api.logger.Error("Failed to create agent config", map[string]any{
			"error":    err.Error(),
			"agent_id": sanitizeLogValue(config.AgentID),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to create agent configuration",
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
			Code:    ErrNotFound,
			Message: "Agent configuration not found",
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
			Code:    ErrBadRequest,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	config, err := api.agentService.UpdateConfig(c.Request.Context(), agentID, &update)
	if err != nil {
		api.logger.Error("Failed to update agent config", map[string]any{
			"error":    err.Error(),
			"agent_id": sanitizeLogValue(agentID),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to update agent configuration",
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
			Code:    ErrNotFound,
			Message: "Agent configuration not found",
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

	// Create metrics adapter
	metricsAdapter := NewMetricsRepositoryAdapter(api.embeddingService, api.logger)
	
	// Get cost summary
	costs, err := metricsAdapter.GetAgentCosts(c.Request.Context(), agentID, period)
	if err != nil {
		api.logger.Error("Failed to get agent costs", map[string]any{
			"error": err.Error(),
			"agent_id": sanitizeLogValue(agentID),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: "Failed to retrieve cost metrics",
		})
		return
	}

	c.JSON(http.StatusOK, costs)
}

// Use the common ErrorResponse from errors.go

// sanitizeLogValue removes newlines and carriage returns from user input to prevent log injection
func sanitizeLogValue(input string) string {
	// Remove newlines, carriage returns, and other control characters
	sanitized := strings.ReplaceAll(input, "\n", "\\n")
	sanitized = strings.ReplaceAll(sanitized, "\r", "\\r")
	sanitized = strings.ReplaceAll(sanitized, "\t", "\\t")
	// Limit length to prevent excessive log sizes
	if len(sanitized) > 100 {
		sanitized = sanitized[:100] + "..."
	}
	return sanitized
}
