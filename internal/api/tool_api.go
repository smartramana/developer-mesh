package api

import (
	"net/http"

	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/gin-gonic/gin"
)

// ToolAPI handles API endpoints for tool operations
type ToolAPI struct {
	adapterBridge *core.AdapterContextBridge
	
	// Handler functions for testing
	executeToolAction   func(c *gin.Context)
	queryToolData       func(c *gin.Context)
	listAvailableTools  func(c *gin.Context)
	listAllowedActions  func(c *gin.Context)
}

// NewToolAPI creates a new tool API handler
func NewToolAPI(adapterBridge *core.AdapterContextBridge) *ToolAPI {
	api := &ToolAPI{
		adapterBridge: adapterBridge,
	}
	
	// Initialize handler functions
	api.executeToolAction = api.handleExecuteToolAction
	api.queryToolData = api.handleQueryToolData
	api.listAvailableTools = api.handleListAvailableTools
	api.listAllowedActions = api.handleListAllowedActions
	
	return api
}

// RegisterRoutes registers all tool API routes
func (api *ToolAPI) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/tools/:tool/actions/:action", api.executeToolAction)
	router.POST("/tools/:tool/query", api.queryToolData)
	router.GET("/tools", api.listAvailableTools)
	router.GET("/tools/:tool/actions", api.listAllowedActions)
}

// handleExecuteToolAction executes an action on a tool
func (api *ToolAPI) handleExecuteToolAction(c *gin.Context) {
	toolName := c.Param("tool")
	actionName := c.Param("action")
	contextID := c.Query("context_id")

	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}

	var params map[string]interface{}
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := api.adapterBridge.ExecuteToolAction(c.Request.Context(), contextID, toolName, actionName, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleQueryToolData retrieves data from a tool
func (api *ToolAPI) handleQueryToolData(c *gin.Context) {
	toolName := c.Param("tool")
	contextID := c.Query("context_id")

	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}

	var query map[string]interface{}
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := api.adapterBridge.GetToolData(c.Request.Context(), contextID, toolName, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleListAvailableTools lists all available tools
func (api *ToolAPI) handleListAvailableTools(c *gin.Context) {
	// In a real implementation, this would retrieve the list of available tools
	// and their capabilities from the engine
	// For now, we'll return a simple list based on the adapter types
	tools := []map[string]interface{}{
		{
			"name": "github",
			"description": "GitHub integration for repository, pull request, and code management",
			"actions": []string{
				"create_issue",
				"close_issue",
				"create_pull_request",
				"merge_pull_request",
				"add_comment",
				"archive_repository", // Note: can archive but not delete
			},
			"safety_notes": "Cannot delete repositories for safety reasons",
		},
	}

	c.JSON(http.StatusOK, gin.H{"tools": tools})
}

// handleListAllowedActions lists all allowed actions for a specific tool
func (api *ToolAPI) handleListAllowedActions(c *gin.Context) {
	toolName := c.Param("tool")

	// In a real implementation, this would retrieve the list of allowed actions
	// from the adapter based on safety restrictions
	var allowedActions []string
	var disallowedActions []string
	var safetyNotes string

	switch toolName {
	case "github":
		allowedActions = []string{
			"create_issue",
			"close_issue",
			"create_pull_request",
			"merge_pull_request",
			"add_comment",
			"get_repository",
			"list_repositories",
			"get_pull_request",
			"list_pull_requests",
			"get_issue",
			"list_issues",
			"archive_repository", // Can archive but not delete
		}
		disallowedActions = []string{
			"delete_repository",
			"delete_branch",
			"delete_organization",
		}
		safetyNotes = "Repository deletion is restricted for safety reasons, but archiving is allowed."
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tool": toolName,
		"allowed_actions": allowedActions,
		"disallowed_actions": disallowedActions,
		"safety_notes": safetyNotes,
	})
}
