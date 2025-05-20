package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ToolAPI handles API endpoints for tool operations
type ToolAPI struct {
	adapterBridge interface{}
	
	// Handler functions for testing
	executeToolAction   func(c *gin.Context)
	queryToolData       func(c *gin.Context)
	listAvailableTools  func(c *gin.Context)
	listAllowedActions  func(c *gin.Context)
}

// NewToolAPI creates a new tool API handler
func NewToolAPI(adapterBridge interface{}) *ToolAPI {
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
	
	// Log that we're registering routes
	log.Println("Registered RESTful tool API routes")
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

	// Validate action exists for tool
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

	var params map[string]interface{}
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Implementation using adapter bridge
	result := map[string]interface{}{
		"status": "success",
		"message": fmt.Sprintf("Executed %s action on %s tool", actionName, toolName),
		"tool": toolName,
		"action": actionName,
		"params": params,
	}

	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	result["_links"] = map[string]string{
		"self": fmt.Sprintf("%s/api/v1/tools/%s/actions/%s", baseURL, toolName, actionName),
		"tool": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolName),
	}

	c.JSON(http.StatusOK, result)
}

// getToolDetails returns details about a specific tool
func (api *ToolAPI) getToolDetails(c *gin.Context) {
	toolName := c.Param("tool")
	
	// Map of available tools
	toolMap := map[string]map[string]interface{}{
		"github": {
			"name": "GitHub",
			"description": "GitHub API integration for repository management",
			"version": "v3",
			"status": "available",
		},
		"harness": {
			"name": "Harness",
			"description": "Harness CD platform for deployment automation",
			"version": "v1",
			"status": "available",
		},
		"sonarqube": {
			"name": "SonarQube",
			"description": "Code quality and security analysis",
			"version": "v8.9",
			"status": "available",
		},
		"artifactory": {
			"name": "Artifactory",
			"description": "Artifact repository manager",
			"version": "v7.x",
			"status": "available",
		},
		"xray": {
			"name": "JFrog Xray",
			"description": "Security and license compliance scanning",
			"version": "v3.x",
			"status": "available",
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

// handleListAvailableTools lists all available tools
func (api *ToolAPI) handleListAvailableTools(c *gin.Context) {
	baseURL := getBaseURLFromContext(c)
	
	tools := []map[string]interface{}{
		{
			"name": "github",
			"display_name": "GitHub",
			"description": "GitHub API integration",
			"status": "available",
			"_links": map[string]string{
				"self": fmt.Sprintf("%s/api/v1/tools/github", baseURL),
			},
		},
		{
			"name": "harness",
			"display_name": "Harness",
			"description": "Harness CD platform",
			"status": "available",
			"_links": map[string]string{
				"self": fmt.Sprintf("%s/api/v1/tools/harness", baseURL),
			},
		},
		{
			"name": "sonarqube",
			"display_name": "SonarQube",
			"description": "Code quality analysis",
			"status": "available",
			"_links": map[string]string{
				"self": fmt.Sprintf("%s/api/v1/tools/sonarqube", baseURL),
			},
		},
		{
			"name": "artifactory",
			"display_name": "Artifactory",
			"description": "Artifact repository",
			"status": "available",
			"_links": map[string]string{
				"self": fmt.Sprintf("%s/api/v1/tools/artifactory", baseURL),
			},
		},
		{
			"name": "xray",
			"display_name": "JFrog Xray",
			"description": "Security scanning",
			"status": "available",
			"_links": map[string]string{
				"self": fmt.Sprintf("%s/api/v1/tools/xray", baseURL),
			},
		},
	}
	
	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
		"_links": map[string]string{
			"self": fmt.Sprintf("%s/api/v1/tools", baseURL),
		},
	})
}

// getActionDetails gets details about a specific action for a tool
func (api *ToolAPI) getActionDetails(c *gin.Context) {
	toolName := c.Param("tool")
	actionName := c.Param("action")
	
	// Validate tool exists
	toolMap := map[string]bool{
		"github": true,
		"harness": true,
		"sonarqube": true,
		"artifactory": true,
		"xray": true,
	}
	
	if !toolMap[toolName] {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}
	
	// Get allowed actions for this tool
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
	// Add cases for other tools...
	}
	
	// Check if action exists for this tool
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
	
	// Get action details
	var actionDetails map[string]interface{}
	
	// Just a simple example for GitHub create_issue action
	if toolName == "github" && actionName == "create_issue" {
		actionDetails = map[string]interface{}{
			"name": "create_issue",
			"display_name": "Create Issue",
			"description": "Creates a new issue in a GitHub repository",
			"parameters": []map[string]string{
				{
					"name": "repository",
					"type": "string",
					"required": "true",
					"description": "Repository in format owner/repo",
				},
				{
					"name": "title",
					"type": "string",
					"required": "true",
					"description": "Issue title",
				},
				{
					"name": "body",
					"type": "string",
					"required": "false",
					"description": "Issue description",
				},
				{
					"name": "labels",
					"type": "array[string]",
					"required": "false",
					"description": "Issue labels",
				},
			},
		}
	} else {
		// Generic response for other actions
		actionDetails = map[string]interface{}{
			"name": actionName,
			"display_name": toTitleCase(actionName),
			"description": fmt.Sprintf("%s action for %s tool", actionName, toolName),
			"parameters": []string{},
		}
	}
	
	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	actionDetails["_links"] = map[string]string{
		"self": fmt.Sprintf("%s/api/v1/tools/%s/actions/%s", baseURL, toolName, actionName),
		"tool": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolName),
		"execute": fmt.Sprintf("%s/api/v1/tools/%s/actions/%s", baseURL, toolName, actionName),
	}
	
	c.JSON(http.StatusOK, actionDetails)
}

// handleListAllowedActions lists all allowed actions for a tool
func (api *ToolAPI) handleListAllowedActions(c *gin.Context) {
	toolName := c.Param("tool")
	
	// Validate tool exists
	toolMap := map[string]bool{
		"github": true,
		"harness": true,
		"sonarqube": true,
		"artifactory": true,
		"xray": true,
	}
	
	if !toolMap[toolName] {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}
	
	// Get allowed actions for this tool
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
	// Add cases for other tools...
	default:
		allowedActions = []string{}
	}
	
	// Format response with HATEOAS links
	baseURL := getBaseURLFromContext(c)
	
	var formattedActions []map[string]interface{}
	for _, action := range allowedActions {
		formattedActions = append(formattedActions, map[string]interface{}{
			"name": action,
			"display_name": toTitleCase(action),
			"_links": map[string]string{
				"self": fmt.Sprintf("%s/api/v1/tools/%s/actions/%s", baseURL, toolName, action),
			},
		})
	}
	
	c.JSON(http.StatusOK, gin.H{
		"tool": toolName,
		"actions": formattedActions,
		"count": len(formattedActions),
		"_links": map[string]string{
			"self": fmt.Sprintf("%s/api/v1/tools/%s/actions", baseURL, toolName),
			"tool": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolName),
		},
	})
}

// handleQueryToolData handles data queries for a tool
func (api *ToolAPI) handleQueryToolData(c *gin.Context) {
	toolName := c.Param("tool")
	contextID := c.Query("context_id")
	
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}
	
	// Validate tool exists
	toolMap := map[string]bool{
		"github": true,
		"harness": true,
		"sonarqube": true,
		"artifactory": true,
		"xray": true,
	}
	
	if !toolMap[toolName] {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}
	
	var query map[string]interface{}
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Mock implementation
	result := map[string]interface{}{
		"status": "success",
		"message": fmt.Sprintf("Executed query on %s tool", toolName),
		"tool": toolName,
		"query": query,
		"results": []interface{}{},
	}
	
	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	result["_links"] = map[string]string{
		"self": fmt.Sprintf("%s/api/v1/tools/%s/queries", baseURL, toolName),
		"tool": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolName),
	}
	
	c.JSON(http.StatusOK, result)
}

// Helper function to get base URL from context
func getBaseURLFromContext(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

// Helper function to convert snake_case to Title Case
func toTitleCase(s string) string {
	// This is a simplified implementation
	result := ""
	capitalize := true
	
	for _, char := range s {
		if char == '_' {
			capitalize = true
			result += " "
		} else if capitalize {
			result += string(char - 32) // Convert to uppercase ASCII
			capitalize = false
		} else {
			result += string(char)
		}
	}
	
	return result
}
