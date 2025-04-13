package api

import (
	"net/http"

	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/gin-gonic/gin"
)

// ToolAPI handles API endpoints for tool operations
type ToolAPI struct {
	adapterBridge *core.AdapterContextBridge
}

// NewToolAPI creates a new tool API handler
func NewToolAPI(adapterBridge *core.AdapterContextBridge) *ToolAPI {
	return &ToolAPI{
		adapterBridge: adapterBridge,
	}
}

// RegisterRoutes registers all tool API routes
func (api *ToolAPI) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/tools/:tool/actions/:action", api.executeToolAction)
	router.POST("/tools/:tool/query", api.queryToolData)
	router.GET("/tools", api.listAvailableTools)
}

// executeToolAction executes an action on a tool
func (api *ToolAPI) executeToolAction(c *gin.Context) {
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

// queryToolData retrieves data from a tool
func (api *ToolAPI) queryToolData(c *gin.Context) {
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

// listAvailableTools lists all available tools
func (api *ToolAPI) listAvailableTools(c *gin.Context) {
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
			},
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
			"description": "JFrog Artifactory integration for artifact management",
			"actions": []string{
				"upload_artifact",
				"download_artifact",
				"get_artifact_info",
				"delete_artifact",
			},
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

	c.JSON(http.StatusOK, gin.H{"tools": tools})
}
