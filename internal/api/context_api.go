package api

import (
	"net/http"
	"time"

	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/gin-gonic/gin"
)

// ContextAPI handles the context management API endpoints
type ContextAPI struct {
	contextManager interfaces.ContextManager
}

// NewContextAPI creates a new context API handler
func NewContextAPI(contextManager interfaces.ContextManager) *ContextAPI {
	return &ContextAPI{
		contextManager: contextManager,
	}
}

// RegisterRoutes registers all context API routes
func (api *ContextAPI) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/contexts", api.createContext)
	router.GET("/contexts/:id", api.getContext)
	router.PUT("/contexts/:id", api.updateContext)
	router.DELETE("/contexts/:id", api.deleteContext)
	router.GET("/contexts", api.listContexts)
	router.POST("/contexts/:id/search", api.searchContext)
	router.GET("/contexts/:id/summary", api.summarizeContext)
}

// createContext creates a new context
func (api *ContextAPI) createContext(c *gin.Context) {
	var request mcp.Context
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := api.contextManager.CreateContext(c.Request.Context(), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// getContext retrieves a context by ID
func (api *ContextAPI) getContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		err := NewBadRequestError("context ID is required", nil)
		c.Error(err)
		return
	}

	result, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		// Check for not found error first
		if strings.Contains(err.Error(), "not found") {
			c.Error(NewContextNotFoundError(contextID, err))
			return
		}
		// Generic server error
		c.Error(NewInternalServerError("Failed to retrieve context", err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// updateContext updates an existing context
func (api *ContextAPI) updateContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.Error(NewBadRequestError("context ID is required", nil))
		return
	}

	var request struct {
		Context mcp.Context              `json:"context"`
		Options mcp.ContextUpdateOptions `json:"options"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(HandleValidationErrors(err))
		return
	}

	// Validate context content if provided
	if len(request.Context.Content) > 0 {
		for i, item := range request.Context.Content {
			if item.Role == "" {
				c.Error(NewValidationError("Invalid context content", 
					map[string]string{
						fmt.Sprintf("content[%d].role", i): "role is required",
					}))
				return
			}
		}
	}

	result, err := api.contextManager.UpdateContext(c.Request.Context(), contextID, &request.Context, &request.Options)
	if err != nil {
		// Check for common errors
		if strings.Contains(err.Error(), "not found") {
			c.Error(NewContextNotFoundError(contextID, err))
			return
		}
		if strings.Contains(err.Error(), "too large") {
			c.Error(NewAPIError(ErrContextTooLarge, "Context exceeds maximum allowed size", http.StatusBadRequest, err))
			return
		}
		// Generic server error
		c.Error(NewInternalServerError("Failed to update context", err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// deleteContext deletes a context
func (api *ContextAPI) deleteContext(c *gin.Context) {
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

// listContexts lists contexts for an agent
func (api *ContextAPI) listContexts(c *gin.Context) {
	agentID := c.Query("agent_id")
	sessionID := c.Query("session_id")
	
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	// Parse additional options
	options := make(map[string]interface{})
	
	// Handle limit
	if limit := c.Query("limit"); limit != "" {
		options["limit"] = limit
	}
	
	// Handle offset
	if offset := c.Query("offset"); offset != "" {
		options["offset"] = offset
	}
	
	// Handle created_after
	if createdAfter := c.Query("created_after"); createdAfter != "" {
		t, err := time.Parse(time.RFC3339, createdAfter)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid created_after timestamp"})
			return
		}
		options["created_after"] = t
	}
	
	// Handle created_before
	if createdBefore := c.Query("created_before"); createdBefore != "" {
		t, err := time.Parse(time.RFC3339, createdBefore)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid created_before timestamp"})
			return
		}
		options["created_before"] = t
	}

	result, err := api.contextManager.ListContexts(c.Request.Context(), agentID, sessionID, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contexts": result})
}

// searchContext searches within a context
func (api *ContextAPI) searchContext(c *gin.Context) {
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

	result, err := api.contextManager.SearchInContext(c.Request.Context(), contextID, request.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": result})
}

// summarizeContext generates a summary of a context
func (api *ContextAPI) summarizeContext(c *gin.Context) {
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
