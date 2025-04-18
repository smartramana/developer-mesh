package api

import (
	"fmt"
	"net/http"
	"strings"
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
	contexts := router.Group("/contexts")
	
	// Collection endpoints
	contexts.GET("", api.listContexts)      // List all contexts
	contexts.POST("", api.createContext)    // Create a new context
	
	// Individual context endpoints
	contexts.GET("/:id", api.getContext)      // Get a specific context
	contexts.PUT("/:id", api.updateContext)   // Update a context
	contexts.PATCH("/:id", api.patchContext)  // Partially update a context
	contexts.DELETE("/:id", api.deleteContext) // Delete a context
	
	// Sub-resources and actions
	contexts.POST("/:id/search", api.searchContext)    // Search within a context
	contexts.GET("/:id/summary", api.summarizeContext) // Get context summary
	contexts.GET("/:id/items", api.getContextItems)    // Get context items
	contexts.POST("/:id/items", api.addContextItem)    // Add an item to context
}

// @Summary Create a new context
// @Description Create a new conversation context for an AI agent
// @Tags contexts
// @Accept json
// @Produce json
// @Param context body mcp.Context true "Context object to create"
// @Success 201 {object} mcp.Context "Created context with HATEOAS links"
// @Failure 400 {object} ErrorResponse "Invalid request format or missing required fields"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 500 {object} ErrorResponse "Server error while creating context"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts [post]
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
	
	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	
	// Create a Links field if not exists
	if result.Links == nil {
		result.Links = make(map[string]string)
	}
	
	// Add navigation links
	result.Links["self"] = fmt.Sprintf("%s/api/v1/contexts/%s", baseURL, result.ID)
	result.Links["items"] = fmt.Sprintf("%s/api/v1/contexts/%s/items", baseURL, result.ID)
	result.Links["search"] = fmt.Sprintf("%s/api/v1/contexts/%s/search", baseURL, result.ID)
	result.Links["summary"] = fmt.Sprintf("%s/api/v1/contexts/%s/summary", baseURL, result.ID)
	result.Links["collection"] = fmt.Sprintf("%s/api/v1/contexts", baseURL)

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

	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	
	// Create a Links field if not exists
	if result.Links == nil {
		result.Links = make(map[string]string)
	}
	
	// Add navigation links
	result.Links["self"] = fmt.Sprintf("%s/api/v1/contexts/%s", baseURL, contextID)
	result.Links["items"] = fmt.Sprintf("%s/api/v1/contexts/%s/items", baseURL, contextID)
	result.Links["search"] = fmt.Sprintf("%s/api/v1/contexts/%s/search", baseURL, contextID)
	result.Links["summary"] = fmt.Sprintf("%s/api/v1/contexts/%s/summary", baseURL, contextID)
	result.Links["collection"] = fmt.Sprintf("%s/api/v1/contexts", baseURL)

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

	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	responseWithLinks := gin.H{
		"results": result,
		"_links": map[string]string{
			"self": fmt.Sprintf("%s/api/v1/contexts/%s/search", baseURL, contextID),
			"context": fmt.Sprintf("%s/api/v1/contexts/%s", baseURL, contextID),
			"summary": fmt.Sprintf("%s/api/v1/contexts/%s/summary", baseURL, contextID),
		},
	}

	c.JSON(http.StatusOK, responseWithLinks)
}

// patchContext handles partial updates to a context
func (api *ContextAPI) patchContext(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.Error(NewBadRequestError("context ID is required", nil))
		return
	}

	var request struct {
		Metadata map[string]interface{} `json:"metadata,omitempty"`
		Content  []mcp.ContextItem      `json:"content,omitempty"`
		Options  mcp.ContextUpdateOptions `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(HandleValidationErrors(err))
		return
	}

	// Get existing context
	existingContext, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.Error(NewContextNotFoundError(contextID, err))
			return
		}
		c.Error(NewInternalServerError("Failed to retrieve context", err))
		return
	}

	// Apply requested changes
	if request.Metadata != nil {
		// Merge metadata instead of replacing
		if existingContext.Metadata == nil {
			existingContext.Metadata = make(map[string]interface{})
		}
		for k, v := range request.Metadata {
			existingContext.Metadata[k] = v
		}
	}

	if len(request.Content) > 0 {
		// For content, respect the options
		if request.Options.ReplaceContent {
			existingContext.Content = request.Content
		} else {
			// Append by default
			existingContext.Content = append(existingContext.Content, request.Content...)
		}
	}

	// Update the context
	result, err := api.contextManager.UpdateContext(c.Request.Context(), contextID, existingContext, &request.Options)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.Error(NewContextNotFoundError(contextID, err))
			return
		}
		if strings.Contains(err.Error(), "too large") {
			c.Error(NewAPIError(ErrContextTooLarge, "Context exceeds maximum allowed size", http.StatusBadRequest, err))
			return
		}
		c.Error(NewInternalServerError("Failed to update context", err))
		return
	}

	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	result.Links = map[string]string{
		"self": fmt.Sprintf("%s/api/v1/contexts/%s", baseURL, contextID),
		"items": fmt.Sprintf("%s/api/v1/contexts/%s/items", baseURL, contextID),
		"search": fmt.Sprintf("%s/api/v1/contexts/%s/search", baseURL, contextID),
		"summary": fmt.Sprintf("%s/api/v1/contexts/%s/summary", baseURL, contextID),
	}

	c.JSON(http.StatusOK, result)
}

// getContextItems returns all items in a context
func (api *ContextAPI) getContextItems(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	// Get existing context
	context, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.Error(NewContextNotFoundError(contextID, err))
			return
		}
		c.Error(NewInternalServerError("Failed to retrieve context", err))
		return
	}

	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	response := gin.H{
		"items": context.Content,
		"_links": map[string]string{
			"self": fmt.Sprintf("%s/api/v1/contexts/%s/items", baseURL, contextID),
			"context": fmt.Sprintf("%s/api/v1/contexts/%s", baseURL, contextID),
			"add_item": fmt.Sprintf("%s/api/v1/contexts/%s/items", baseURL, contextID),
		},
	}

	c.JSON(http.StatusOK, response)
}

// addContextItem adds a new item to a context
func (api *ContextAPI) addContextItem(c *gin.Context) {
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context ID is required"})
		return
	}

	var item mcp.ContextItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.Error(HandleValidationErrors(err))
		return
	}

	// Validate required fields
	if item.Role == "" {
		c.Error(NewValidationError("Invalid context item", 
			map[string]string{
				"role": "role is required",
			}))
		return
	}

	// Get existing context
	context, err := api.contextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.Error(NewContextNotFoundError(contextID, err))
			return
		}
		c.Error(NewInternalServerError("Failed to retrieve context", err))
		return
	}

	// Add timestamp if not provided
	if item.Timestamp.IsZero() {
		item.Timestamp = time.Now()
	}

	// Append the new item
	context.Content = append(context.Content, item)

	// Update the context
	result, err := api.contextManager.UpdateContext(c.Request.Context(), contextID, context, nil)
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			c.Error(NewAPIError(ErrContextTooLarge, "Context exceeds maximum allowed size", http.StatusBadRequest, err))
			return
		}
		c.Error(NewInternalServerError("Failed to update context", err))
		return
	}

	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	response := gin.H{
		"item_added": item,
		"item_index": len(result.Content) - 1,
		"_links": map[string]string{
			"self": fmt.Sprintf("%s/api/v1/contexts/%s/items", baseURL, contextID),
			"context": fmt.Sprintf("%s/api/v1/contexts/%s", baseURL, contextID),
			"items": fmt.Sprintf("%s/api/v1/contexts/%s/items", baseURL, contextID),
		},
	}

	c.JSON(http.StatusCreated, response)
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
