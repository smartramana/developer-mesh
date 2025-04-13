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
	router.GET("/tools/:tool/actions", api.listAllowedActions)
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

	c.JSON(http.StatusOK, gin.H{"tools": tools})
}

// listAllowedActions lists all allowed actions for a specific tool
func (api *ToolAPI) listAllowedActions(c *gin.Context) {
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
			"list_pipelines",
			"get_pipeline_logs",
			"rollback_deployment",
			"get_feature_flag",
			"list_feature_flags",
			"toggle_non_prod_feature_flag",
		}
		disallowedActions = []string{
			"delete_pipeline",
			"delete_feature_flag",
			"toggle_prod_feature_flag",
			"delete_service",
		}
		safetyNotes = "Production feature flag deletion and toggling is restricted for safety reasons."
	
	case "artifactory":
		allowedActions = []string{
			"get_artifact",
			"search_artifacts",
			"get_artifact_properties",
			"get_artifact_statistics",
			"get_repository_info",
			"get_builds",
			"get_build_info",
			"get_storage_info",
		}
		disallowedActions = []string{
			"upload_artifact",
			"delete_artifact",
			"move_artifact",
			"copy_artifact",
			"update_artifact",
			"deploy_artifact",
		}
		safetyNotes = "Read-only access for safety reasons. No upload, delete, or modification operations allowed."
	
	case "sonarqube":
		allowedActions = []string{
			"trigger_analysis",
			"get_quality_gate_status",
			"get_issues",
			"get_metrics",
			"get_project_status",
		}
		disallowedActions = []string{
			"delete_project",
			"delete_quality_gate",
		}
		safetyNotes = "Project deletion is restricted for safety reasons."
	
	case "xray":
		allowedActions = []string{
			"scan_artifact",
			"get_vulnerabilities",
			"get_licenses",
			"get_scan_status",
			"get_policy_violations",
		}
		disallowedActions = []string{
			"delete_scan",
			"update_policy",
		}
		safetyNotes = "Policy modifications are restricted for safety reasons."
	
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
