package api

import (
	"fmt"
	"log"
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
	tools := router.Group("/tools")
	
	// Collection endpoints
	tools.GET("", api.listAvailableTools)
	
	// Single tool endpoints
	tools.GET("/:tool", api.getToolDetails)
	
	// Tool actions as sub-resources
	actions := tools.Group("/:tool/actions")
	actions.GET("", api.listAllowedActions)
	actions.GET("/:action", api.getActionDetails)
	actions.POST("/:action", api.executeToolAction)
	
	// Tool data queries
	tools.POST("/:tool/queries", api.queryToolData)
	
	// For backward compatibility, maintain old endpoints
	router.POST("/tools/:tool/actions/:action", api.executeToolAction)
	router.POST("/tools/:tool/query", api.queryToolData)
	
	// Log that we're registering routes
	log.Println("Registered RESTful tool API routes")
}

// @Summary Execute tool action
// @Description Execute an action using a specific DevOps tool
// @Tags tools
// @Accept json
// @Produce json
// @Param tool path string true "Tool name"
// @Param action path string true "Action name"
// @Param context_id query string true "Context ID for tracking the operation"
// @Param params body object true "Action parameters"
// @Success 200 {object} object "Action result with HATEOAS links"
// @Failure 400 {object} ErrorResponse "Invalid request or missing parameters"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 404 {object} ErrorResponse "Tool or action not found"
// @Failure 500 {object} ErrorResponse "Error executing the action"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /tools/{tool}/actions/{action} [post]
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

	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	result["_links"] = map[string]string{
		"self": fmt.Sprintf("%s/api/v1/tools/%s/actions/%s", baseURL, toolName, actionName),
		"tool": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolName),
	}

	c.JSON(http.StatusOK, result)
}

// @Summary Get tool details
// @Description Get detailed information about a specific DevOps tool
// @Tags tools
// @Accept json
// @Produce json
// @Param tool path string true "Tool name"
// @Success 200 {object} object "Tool details with available actions and HATEOAS links"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 404 {object} ErrorResponse "Tool not found"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /tools/{tool} [get]
// getToolDetails returns details about a specific tool
func (api *ToolAPI) getToolDetails(c *gin.Context) {
	toolName := c.Param("tool")
	
	// Map of available tools
	toolMap := map[string]map[string]interface{}{
		"github": {
			"name": "github",
			"description": "GitHub integration for repository, pull request, and code management",
			"actions": []string{
				"create_issue",
				"close_issue",
				"create_pull_request",
				"merge_pull_request",
				"add_comment",
				"archive_repository",
			},
			"safety_notes": "Cannot delete repositories for safety reasons",
		},
		"harness": {
			"name": "harness",
			"description": "Harness CI/CD integration for builds and deployments",
			"actions": []string{
				"trigger_pipeline",
				"get_pipeline_status",
				"stop_pipeline",
				"rollback_deployment",
			},
			"safety_notes": "Cannot delete production feature flags for safety reasons",
		},
		"sonarqube": {
			"name": "sonarqube",
			"description": "SonarQube integration for code quality analysis",
			"actions": []string{
				"trigger_analysis",
				"get_quality_gate_status",
				"get_issues",
			},
		},
		"artifactory": {
			"name": "artifactory",
			"description": "JFrog Artifactory integration for artifact management (read-only)",
			"actions": []string{
				"download_artifact",
				"get_artifact_info",
				"search_artifacts",
			},
			"safety_notes": "Read-only access for safety reasons (no upload or delete capabilities)",
		},
		"xray": {
			"name": "xray",
			"description": "JFrog Xray integration for security scanning",
			"actions": []string{
				"scan_artifact",
				"get_vulnerabilities",
				"get_licenses",
			},
		},
	}
	
	toolInfo, exists := toolMap[toolName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}
	
	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	toolInfo["_links"] = map[string]string{
		"self": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolName),
		"actions": fmt.Sprintf("%s/api/v1/tools/%s/actions", baseURL, toolName),
		"queries": fmt.Sprintf("%s/api/v1/tools/%s/queries", baseURL, toolName),
	}
	
	c.JSON(http.StatusOK, toolInfo)
}

// @Summary Get action details
// @Description Get detailed information about a specific action for a DevOps tool
// @Tags tools
// @Accept json
// @Produce json
// @Param tool path string true "Tool name"
// @Param action path string true "Action name"
// @Success 200 {object} object "Action details with parameters and example usage"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 404 {object} ErrorResponse "Tool or action not found"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /tools/{tool}/actions/{action} [get]
// getActionDetails returns details about a specific action
func (api *ToolAPI) getActionDetails(c *gin.Context) {
	toolName := c.Param("tool")
	actionName := c.Param("action")
	
	// Get allowed actions for the tool
	var allowedActions []string
	
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
			"archive_repository",
		}
	case "harness":
		allowedActions = []string{
			"trigger_pipeline",
			"get_pipeline_status",
			"stop_pipeline",
			"rollback_deployment",
			"get_pipelines",
		}
	case "sonarqube":
		allowedActions = []string{
			"trigger_analysis",
			"get_quality_gate_status",
			"get_issues",
			"get_metrics",
			"get_projects",
		}
	case "artifactory":
		allowedActions = []string{
			"download_artifact",
			"get_artifact_info",
			"search_artifacts",
			"get_build_info",
			"get_repositories",
		}
	case "xray":
		allowedActions = []string{
			"scan_artifact",
			"get_vulnerabilities",
			"get_licenses",
			"get_component_summary",
		}
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}
	
	// Check if the action exists for this tool
	actionExists := false
	for _, action := range allowedActions {
		if action == actionName {
			actionExists = true
			break
		}
	}
	
	if !actionExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Action not found for this tool"})
		return
	}
	
	// Get action description and parameters
	actionDesc := getActionDescription(toolName, actionName)
	
	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	actionDesc["_links"] = map[string]string{
		"self": fmt.Sprintf("%s/api/v1/tools/%s/actions/%s", baseURL, toolName, actionName),
		"tool": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolName),
		"actions": fmt.Sprintf("%s/api/v1/tools/%s/actions", baseURL, toolName),
	}
	
	c.JSON(http.StatusOK, actionDesc)
}

// getActionDescription returns detailed information about an action
func getActionDescription(toolName, actionName string) map[string]interface{} {
	// This would typically come from a database or configuration
	// Here's a simplified example for a few actions
	
	actionDescriptions := map[string]map[string]map[string]interface{}{
		"github": {
			"create_issue": {
				"name": "create_issue",
				"description": "Creates a new issue in a GitHub repository",
				"parameters": map[string]interface{}{
					"owner": "Repository owner (organization or user)",
					"repo": "Repository name",
					"title": "Issue title",
					"body": "Issue description",
					"labels": "Array of label names",
					"assignees": "Array of usernames to assign",
				},
				"required_parameters": []string{"owner", "repo", "title"},
				"example": map[string]interface{}{
					"owner": "octocat",
					"repo": "hello-world",
					"title": "Bug in login form",
					"body": "The login form doesn't submit when using Safari",
					"labels": []string{"bug", "frontend"},
				},
			},
		},
	}
	
	// Check if we have a detailed description for this action
	if toolActions, ok := actionDescriptions[toolName]; ok {
		if actionDesc, ok := toolActions[actionName]; ok {
			return actionDesc
		}
	}
	
	// Return minimal information if no detailed description is available
	return map[string]interface{}{
		"name": actionName,
		"description": fmt.Sprintf("%s action for %s", actionName, toolName),
		"parameters": map[string]interface{}{},
	}
}

// Helper function to get base URL from gin context
func getBaseURLFromContext(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	
	host := c.Request.Host
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}
	
	return scheme + "://" + host
}

// @Summary Query tool data
// @Description Retrieve data from a DevOps tool using a custom query
// @Tags tools
// @Accept json
// @Produce json
// @Param tool path string true "Tool name"
// @Param context_id query string true "Context ID for tracking the operation"
// @Param query body object true "Query parameters"
// @Success 200 {object} object "Query results"
// @Failure 400 {object} ErrorResponse "Invalid request or missing parameters"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 404 {object} ErrorResponse "Tool not found"
// @Failure 500 {object} ErrorResponse "Error executing the query"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /tools/{tool}/queries [post]
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

// @Summary List all available tools
// @Description Returns a list of all available DevOps tools with their descriptions and actions
// @Tags tools
// @Accept json
// @Produce json
// @Success 200 {object} object "Tool list with HATEOAS links"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /tools [get]
// handleListAvailableTools lists all available tools
func (api *ToolAPI) handleListAvailableTools(c *gin.Context) {
	// Get base URL for HATEOAS links
	baseURL := getBaseURLFromContext(c)
	
	// Create tool collection
	toolsList := []map[string]interface{}{
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
		{
			"name": "harness",
			"description": "Harness CI/CD integration for builds and deployments",
			"actions": []string{
				"trigger_pipeline",
				"get_pipeline_status",
				"stop_pipeline",
				"rollback_deployment",
			},
			"safety_notes": "Cannot delete production feature flags for safety reasons",
		},
		{
			"name": "sonarqube",
			"description": "SonarQube integration for code quality analysis",
			"actions": []string{
				"trigger_analysis",
				"get_quality_gate_status",
				"get_issues",
			},
		},
		{
			"name": "artifactory",
			"description": "JFrog Artifactory integration for artifact management (read-only)",
			"actions": []string{
				"download_artifact",
				"get_artifact_info",
				"search_artifacts",
			},
			"safety_notes": "Read-only access for safety reasons (no upload or delete capabilities)",
		},
		{
			"name": "xray",
			"description": "JFrog Xray integration for security scanning",
			"actions": []string{
				"scan_artifact",
				"get_vulnerabilities",
				"get_licenses",
			},
		},
	}
	
	// Add HATEOAS links for the tools collection
	responseWithLinks := gin.H{
		"tools": toolsList,
		"_links": map[string]string{
			"self": fmt.Sprintf("%s/api/v1/tools", baseURL),
		},
	}
	
	c.JSON(http.StatusOK, responseWithLinks)
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
		
	case "harness":
		allowedActions = []string{
			"trigger_pipeline",
			"get_pipeline_status",
			"stop_pipeline",
			"rollback_deployment",
			"get_pipelines",
		}
		disallowedActions = []string{
			"delete_pipeline",
			"delete_application",
			"delete_environment",
			"delete_service",
		}
		safetyNotes = "Pipeline and resource deletion operations are restricted for safety reasons."
		
	case "sonarqube":
		allowedActions = []string{
			"trigger_analysis",
			"get_quality_gate_status",
			"get_issues",
			"get_metrics",
			"get_projects",
		}
		disallowedActions = []string{
			"delete_project",
			"reset_quality_gate",
		}
		safetyNotes = "Project deletion and quality gate reset operations are restricted."
		
	case "artifactory":
		allowedActions = []string{
			"download_artifact",
			"get_artifact_info",
			"search_artifacts",
			"get_build_info",
			"get_repositories",
		}
		disallowedActions = []string{
			"upload_artifact",
			"delete_artifact",
			"move_artifact",
			"delete_repository",
		}
		safetyNotes = "Read-only access for safety reasons (no upload, delete, or modify operations allowed)."
		
	case "xray":
		allowedActions = []string{
			"scan_artifact",
			"get_vulnerabilities",
			"get_licenses",
			"get_component_summary",
		}
		disallowedActions = []string{
			"ignore_vulnerability",
			"delete_scan",
		}
		safetyNotes = "Only scanning and information retrieval operations are supported."
		
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
