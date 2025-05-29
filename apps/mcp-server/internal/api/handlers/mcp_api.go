package handlers

import (
	"context"
	"net/http"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
)

// ContextManagerInterface defines the interface for context management
type ContextManagerInterface interface {
	CreateContext(ctx context.Context, context *models.Context) (*models.Context, error)
	GetContext(ctx context.Context, contextID string) (*models.Context, error)
	UpdateContext(ctx context.Context, contextID string, context *models.Context, options *models.ContextUpdateOptions) (*models.Context, error)
	DeleteContext(ctx context.Context, contextID string) error
	ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*models.Context, error)
	SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error)
	SummarizeContext(ctx context.Context, contextID string) (string, error)
}

// MCPAPI handles the MCP-specific API endpoints
type MCPAPI struct {
	contextManager ContextManagerInterface
	logger         observability.Logger
}

// NewMCPAPI creates a new MCP API handler
func NewMCPAPI(contextManager interface{}, logger observability.Logger) *MCPAPI {
	return &MCPAPI{
		contextManager: contextManager.(ContextManagerInterface),
		logger:         logger,
	}
}

// RegisterRoutes registers all MCP API routes
func (api *MCPAPI) RegisterRoutes(router *gin.RouterGroup) {
	mcpRoutes := router.Group("/mcp")
	{
		mcpRoutes.POST("/context", api.createContext)
		mcpRoutes.GET("/context/:id", api.getContext)
		mcpRoutes.PUT("/context/:id", api.updateContext)
		mcpRoutes.DELETE("/context/:id", api.deleteContext)
		mcpRoutes.GET("/contexts", api.listContexts)
		mcpRoutes.POST("/context/:id/search", api.searchContext)
		mcpRoutes.GET("/context/:id/summary", api.summarizeContext)
	}
	
	api.logger.Info("Registered MCP API routes", nil)
}

// createContext creates a new context
// @Summary Create a new context
// @Description Creates a new conversation context for an AI agent
// @Tags Contexts
// @Accept json
// @Produce json
// @Param request body models.Context true "Context creation request"
// @Success 201 {object} map[string]interface{} "Context created successfully"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /mcp/context [post]
func (api *MCPAPI) createContext(c *gin.Context) {
	var request models.Context
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdContext, err := api.contextManager.CreateContext(c.Request.Context(), &request)
	if err != nil {
		api.logger.Error("Failed to create context", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "context created", 
		"id": createdContext.ID,
		"context": createdContext,
	})
}

// getContext retrieves a context by ID
// @Summary Get context by ID
// @Description Retrieves a specific context including all conversation history and metadata
// @Tags Contexts
// @Accept json
// @Produce json
// @Param id path string true "Context ID"
// @Success 200 {object} models.Context "Context details"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 401 {object} map[string]string "Unauthorized" 
// @Failure 404 {object} map[string]string "Context not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /mcp/context/{id} [get]
func (api *MCPAPI) getContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	contextObj, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to get context", map[string]interface{}{
			"context_id": contextID,
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, contextObj)
}

// updateContext updates an existing context
func (api *MCPAPI) updateContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	// Parse request body
	var updateRequest struct {
		Content []models.ContextItem          `json:"content"`
		Options *models.ContextUpdateOptions  `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Get the current context
	currentContext, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Context not found during update", map[string]interface{}{
			"context_id": contextID,
			"error": err.Error(),
		})
		c.JSON(http.StatusNotFound, gin.H{"error": "context not found: " + err.Error()})
		return
	}

	// Ensure we have options
	if updateRequest.Options == nil {
		updateRequest.Options = &models.ContextUpdateOptions{}
	}
	
	// When using MCPAPI, we want to replace content by default
	updateRequest.Options.ReplaceContent = true

	// Update content if provided
	if updateRequest.Content != nil {
		currentContext.Content = updateRequest.Content
	}

	// Update the context in the database
	updatedContext, err := api.contextManager.UpdateContext(
		c.Request.Context(),
		contextID,
		currentContext,
		updateRequest.Options,
	)

	if err != nil {
		api.logger.Error("Failed to update context", map[string]interface{}{
			"context_id": contextID,
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update context: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedContext)
}

// deleteContext deletes a context
func (api *MCPAPI) deleteContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	err := api.contextManager.DeleteContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to delete context", map[string]interface{}{
			"context_id": contextID,
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// listContexts returns a list of contexts for an agent
func (api *MCPAPI) listContexts(c *gin.Context) {
	agentID := c.Query("agent_id")
	sessionID := c.Query("session_id")
	
	contexts, err := api.contextManager.ListContexts(c.Request.Context(), agentID, sessionID, nil)
	if err != nil {
		api.logger.Error("Failed to list contexts", map[string]interface{}{
			"agent_id": agentID,
			"session_id": sessionID,
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"contexts": contexts})
}

// searchContext searches for text within a context
func (api *MCPAPI) searchContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}
	
	var request struct {
		Query string `json:"query" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	results, err := api.contextManager.SearchInContext(c.Request.Context(), contextID, request.Query)
	if err != nil {
		api.logger.Error("Failed to search in context", map[string]interface{}{
			"context_id": contextID,
			"query": request.Query,
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// summarizeContext generates a summary of a context
func (api *MCPAPI) summarizeContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}
	
	summary, err := api.contextManager.SummarizeContext(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to summarize context", map[string]interface{}{
			"context_id": contextID,
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"summary": summary})
}
