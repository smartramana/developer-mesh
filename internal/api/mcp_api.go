package api

import (
	"net/http"
	"context"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/gin-gonic/gin"
)

// ContextManagerInterface defines the interface for context management
type ContextManagerInterface interface {
	CreateContext(ctx context.Context, context *mcp.Context) (*mcp.Context, error)
	GetContext(ctx context.Context, contextID string) (*mcp.Context, error)
	UpdateContext(ctx context.Context, contextID string, context *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error)
	DeleteContext(ctx context.Context, contextID string) error
	ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error)
	SearchInContext(ctx context.Context, contextID, query string) ([]mcp.ContextItem, error)
	SummarizeContext(ctx context.Context, contextID string) (string, error)
}

// MCPAPI handles the MCP-specific API endpoints
type MCPAPI struct {
	contextManager ContextManagerInterface
}

// NewMCPAPI creates a new MCP API handler
func NewMCPAPI(contextManager interface{}) *MCPAPI {
	return &MCPAPI{
		contextManager: contextManager.(ContextManagerInterface),
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
}

// createContext creates a new context
func (api *MCPAPI) createContext(c *gin.Context) {
	var request mcp.Context
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := api.contextManager.CreateContext(c.Request.Context(), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "context created", "id": request.ID})
}

// getContext retrieves a context by ID
func (api *MCPAPI) getContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	_, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": contextID, "message": "context retrieved"})
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
		Content []mcp.ContextItem          `json:"content"`
		Options *mcp.ContextUpdateOptions  `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Get the current context
	currentContext, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "context not found: " + err.Error()})
		return
	}

	// Ensure we have options
	if updateRequest.Options == nil {
		updateRequest.Options = &mcp.ContextUpdateOptions{}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"summary": summary})
}