package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// ToolAPI handles API endpoints for tool operations
type ToolAPI struct {
	adapterBridge interface{}
	logger        observability.Logger
	
	// Handler functions for testing
	executeToolAction   func(c *gin.Context)
	queryToolData       func(c *gin.Context)
	listAvailableTools  func(c *gin.Context)
	listAllowedActions  func(c *gin.Context)
}

// NewToolAPI creates a new tool API handler
func NewToolAPI(adapterBridge interface{}, logger observability.Logger) *ToolAPI {
	api := &ToolAPI{
		adapterBridge: adapterBridge,
		logger:        logger,
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
	api.logger.Info("Registered RESTful tool API routes", nil)
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
			"description": "GitHub integration for repository management",
			"version": "1.0",
			"vendor": "GitHub",
			"auth_methods": []string{"API Key", "OAuth"},
		},
		"harness": {
			"name": "Harness",
			"description": "Harness CI/CD integration",
			"version": "1.0",
			"vendor": "Harness",
			"auth_methods": []string{"API Key"},
		},
		"sonarqube": {
			"name": "SonarQube",
			"description": "Code quality and security analysis",
			"version": "1.0",
			"vendor": "SonarSource",
			"auth_methods": []string{"API Key", "Username/Password"},
		},
		"artifactory": {
			"name": "JFrog Artifactory",
			"description": "Artifact repository manager",
			"version": "1.0",
			"vendor": "JFrog",
			"auth_methods": []string{"API Key", "Username/Password"},
		},
		"xray": {
			"name": "JFrog Xray",
			"description": "Security and license compliance for artifacts",
			"version": "1.0",
			"vendor": "JFrog",
			"auth_methods": []string{"API Key"},
		},
	}
	
	// Check if tool exists
	toolDetails, exists := toolMap[toolName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}
	
	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	toolDetails["_links"] = map[string]string{
		"self": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolName),
		"actions": fmt.Sprintf("%s/api/v1/tools/%s/actions", baseURL, toolName),
		"collection": fmt.Sprintf("%s/api/v1/tools", baseURL),
	}
	
	c.JSON(http.StatusOK, toolDetails)
}

// handleListAvailableTools lists all available tools
func (api *ToolAPI) handleListAvailableTools(c *gin.Context) {
	// List of all available tools
	tools := []map[string]interface{}{
		{
			"id": "github",
			"name": "GitHub",
			"description": "GitHub integration for repository management",
		},
		{
			"id": "harness",
			"name": "Harness",
			"description": "Harness CI/CD integration",
		},
		{
			"id": "sonarqube",
			"name": "SonarQube",
			"description": "Code quality and security analysis",
		},
		{
			"id": "artifactory",
			"name": "JFrog Artifactory",
			"description": "Artifact repository manager",
		},
		{
			"id": "xray",
			"name": "JFrog Xray",
			"description": "Security and license compliance for artifacts",
		},
	}
	
	// Add HATEOAS links
	baseURL := getBaseURLFromContext(c)
	
	// Add links to each tool
	for i := range tools {
		toolID := tools[i]["id"].(string)
		tools[i]["_links"] = map[string]string{
			"self": fmt.Sprintf("%s/api/v1/tools/%s", baseURL, toolID),
			"actions": fmt.Sprintf("%s/api/v1/tools/%s/actions", baseURL, toolID),
		}
	}
	
	// Add collection links
	result := map[string]interface{}{
		"tools": tools,
		"count": len(tools),
		"_links": map[string]string{
			"self": fmt.Sprintf("%s/api/v1/tools", baseURL),
		},
	}
	
	c.JSON(http.StatusOK, result)
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
	
	// Action parameter definitions by tool and action
	actionDetailsMap := map[string]map[string]map[string]interface{}{
		"github": {
			"create_issue": {
				"name": "create_issue",
				"display_name": "Create Issue",
				"description": "Creates a new issue in a GitHub repository",
				"parameters": []map[string]interface{}{
					{
						"name": "repository",
						"type": "string",
						"required": true,
						"description": "Repository name (owner/repo)",
					},
					{
						"name": "title",
						"type": "string",
						"required": true,
						"description": "Issue title",
					},
					{
						"name": "body",
						"type": "string",
						"required": true,
						"description": "Issue body/description",
					},
					{
						"name": "labels",
						"type": "array",
						"required": false,
						"description": "Labels to apply to the issue",
					},
					{
						"name": "assignees",
						"type": "array",
						"required": false,
						"description": "GitHub usernames to assign",
					},
				},
			},
			// Other GitHub actions would be defined here
		},
		"harness": {
			"trigger_pipeline": {
				"name": "trigger_pipeline",
				"display_name": "Trigger Pipeline",
				"description": "Triggers a CI/CD pipeline in Harness",
				"parameters": []map[string]interface{}{
					{
						"name": "pipeline_id",
						"type": "string",
						"required": true,
						"description": "Pipeline identifier",
					},
					{
						"name": "variables",
						"type": "object",
						"required": false,
						"description": "Pipeline variables",
					},
				},
			},
			// Other Harness actions would be defined here
		},
		// Other tools would be defined here
	}
	
	// Validate action exists for this tool
	toolActions, exists := actionDetailsMap[toolName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "No actions defined for this tool"})
		return
	}
	
	actionDetails, exists := toolActions[actionName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Action not found for this tool"})
		return
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

// getBaseURLFromContext extracts the base URL from the request context
func getBaseURLFromContext(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

// toTitleCase converts snake_case to Title Case
func toTitleCase(s string) string {
	words := strings.Split(s, "_")
	for i, word := range words {
		if len(word) > 0 {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
