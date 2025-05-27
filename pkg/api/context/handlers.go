package context

import (
	"net/http"
	"strconv"

	contextManager "github.com/S-Corkum/devops-mcp/pkg/core/context"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
)

// ContextResponse wraps a Context with HATEOAS links
type ContextResponse struct {
	*models.Context
	Links map[string]string `json:"_links,omitempty"`
}

// API handles context-related API endpoints
type API struct {
	contextManager *contextManager.Manager
	logger         observability.Logger // Changed from pointer to interface type
	metricsClient  observability.MetricsClient
}

// NewAPI creates a new context API handler
func NewAPI(
	contextManager *contextManager.Manager,
	logger observability.Logger, // Changed from pointer to interface type
	metricsClient observability.MetricsClient,
) *API {
	if logger == nil {
		logger = observability.NewLogger("context_api")
	}

	return &API{
		contextManager: contextManager,
		logger:         logger,
		metricsClient:  metricsClient,
	}
}

// RegisterRoutes registers context API routes
func (api *API) RegisterRoutes(router *gin.RouterGroup) {
	contextRoutes := router.Group("/contexts")
	{
		contextRoutes.POST("", api.CreateContext)
		contextRoutes.GET("/:contextID", api.GetContext)
		contextRoutes.PUT("/:contextID", api.UpdateContext)
		contextRoutes.DELETE("/:contextID", api.DeleteContext)
		contextRoutes.GET("", api.ListContexts)
		contextRoutes.GET("/:contextID/summary", api.SummarizeContext)
		contextRoutes.POST("/:contextID/search", api.SearchInContext)
	}
}

// CreateContext creates a new context
func (api *API) CreateContext(c *gin.Context) {
	var contextData models.Context

	if err := c.ShouldBindJSON(&contextData); err != nil {
		api.logger.Warn("Invalid request body for create context", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := api.contextManager.CreateContext(c.Request.Context(), &contextData)
	if err != nil {
		api.logger.Error("Failed to create context", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "create_context",
			"status":    "success",
		})
	}

	// Create response with HATEOAS links
	response := &ContextResponse{
		Context: result,
		Links: map[string]string{
			"self":    "/api/v1/contexts/" + result.ID,
			"summary": "/api/v1/contexts/" + result.ID + "/summary",
			"search":  "/api/v1/contexts/" + result.ID + "/search",
		},
	}

	c.JSON(http.StatusCreated, response)
}

// GetContext retrieves a context by ID
func (api *API) GetContext(c *gin.Context) {
	contextID := c.Param("contextID")

	// Read query parameters for content options
	includeContent := true
	if includeContentParam := c.Query("include_content"); includeContentParam != "" {
		var err error
		includeContent, err = strconv.ParseBool(includeContentParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid include_content parameter"})
			return
		}
	}

	result, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Warn("Failed to get context", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Optionally remove content for lighter responses
	if !includeContent {
		result.Content = []models.ContextItem{}
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "get_context",
			"status":    "success",
		})
	}

	// Create response with HATEOAS links
	response := &ContextResponse{
		Context: result,
		Links: map[string]string{
			"self":    "/api/v1/contexts/" + result.ID,
			"summary": "/api/v1/contexts/" + result.ID + "/summary",
			"search":  "/api/v1/contexts/" + result.ID + "/search",
		},
	}

	c.JSON(http.StatusOK, response)
}

// UpdateContext updates an existing context
func (api *API) UpdateContext(c *gin.Context) {
	contextID := c.Param("contextID")

	var request struct {
		Context *models.Context              `json:"context"`
		Options *models.ContextUpdateOptions `json:"options"`
	}

	// Bind the request body once into the typed struct
	if err := c.ShouldBindJSON(&request); err != nil {
		api.logger.Warn("Invalid request body for update context", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// If context is present and metadata is nil, set to empty map to ensure valid JSON object downstream
	if request.Context != nil && request.Context.Metadata == nil {
		request.Context.Metadata = map[string]interface{}{}
	}

	if request.Context == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context is required"})
		return
	}

	result, err := api.contextManager.UpdateContext(c.Request.Context(), contextID, request.Context, request.Options)
	if err != nil {
		api.logger.Error("Failed to update context", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "update_context",
			"status":    "success",
		})
	}

	// Create response with HATEOAS links
	response := &ContextResponse{
		Context: result,
		Links: map[string]string{
			"self":    "/api/v1/contexts/" + result.ID,
			"summary": "/api/v1/contexts/" + result.ID + "/summary",
			"search":  "/api/v1/contexts/" + result.ID + "/search",
		},
	}

	c.JSON(http.StatusOK, response)
}

// DeleteContext deletes a context
func (api *API) DeleteContext(c *gin.Context) {
	contextID := c.Param("contextID")

	err := api.contextManager.DeleteContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to delete context", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "delete_context",
			"status":    "success",
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "context deleted"})
}

// ListContexts lists contexts for an agent
func (api *API) ListContexts(c *gin.Context) {
	agentID := c.Query("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	sessionID := c.Query("session_id")

	// Parse limit from query
	limit := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
			return
		}
	}

	options := map[string]interface{}{}
	if limit > 0 {
		options["limit"] = limit
	}

	result, err := api.contextManager.ListContexts(c.Request.Context(), agentID, sessionID, options)
	if err != nil {
		api.logger.Error("Failed to list contexts", map[string]interface{}{
			"error":    err.Error(),
			"agent_id": agentID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "list_contexts",
			"status":    "success",
		})
	}

	// Note: The models.Context no longer has Links field in the new structure
	// This section is preserved in comments for reference but is no longer applicable
	// for _, ctx := range result {
	// 	if ctx.Links == nil {
	// 		ctx.Links = make(map[string]string)
	// 	}
	// 	ctx.Links["self"] = "/api/v1/contexts/" + ctx.ID
	// 	ctx.Links["summary"] = "/api/v1/contexts/" + ctx.ID + "/summary"
	// 	ctx.Links["search"] = "/api/v1/contexts/" + ctx.ID + "/search"
	// }

	c.JSON(http.StatusOK, result)
}

// SummarizeContext generates a summary of a context
func (api *API) SummarizeContext(c *gin.Context) {
	contextID := c.Param("contextID")

	result, err := api.contextManager.SummarizeContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to summarize context", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "summarize_context",
			"status":    "success",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"context_id": contextID,
		"summary":    result,
		"_links": map[string]string{
			"self":    "/api/v1/contexts/" + contextID,
			"context": "/api/v1/contexts/" + contextID,
		},
	})
}

// SearchInContext searches for text within a context
func (api *API) SearchInContext(c *gin.Context) {
	contextID := c.Param("contextID")

	var request struct {
		Query string `json:"query"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		api.logger.Warn("Invalid request body for search in context", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := api.contextManager.SearchInContext(c.Request.Context(), contextID, request.Query)
	if err != nil {
		api.logger.Error("Failed to search in context", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record metric
	if api.metricsClient != nil {
		api.metricsClient.RecordCounter("context_api_requests", 1, map[string]string{
			"operation": "search_in_context",
			"status":    "success",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"context_id": contextID,
		"query":      request.Query,
		"results":    result,
		"_links": map[string]string{
			"self":    "/api/v1/contexts/" + contextID + "/search",
			"context": "/api/v1/contexts/" + contextID,
		},
	})
}
