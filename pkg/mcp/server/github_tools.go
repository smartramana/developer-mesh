package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/S-Corkum/devops-mcp/internal/adapters/github"
	"github.com/S-Corkum/devops-mcp/internal/observability"
	"github.com/S-Corkum/devops-mcp/pkg/mcp/tool"
	githubtools "github.com/S-Corkum/devops-mcp/pkg/mcp/tool/github"
	"github.com/gin-gonic/gin"
)

// GitHubToolsHandler handles GitHub tool endpoints
type GitHubToolsHandler struct {
	registry     *tool.ToolRegistry
	logger       *observability.Logger
	githubConfig *github.Config
}

// NewGitHubToolsHandler creates a new GitHub tools handler
func NewGitHubToolsHandler(
	githubAdapter *github.GitHubAdapter,
	logger *observability.Logger,
) *GitHubToolsHandler {
	registry := tool.NewToolRegistry()
	
	// Create and register GitHub tools
	toolProvider := githubtools.NewGitHubToolProvider(githubAdapter)
	err := toolProvider.RegisterTools(registry)
	if err != nil {
		logger.Error("Failed to register GitHub tools", map[string]interface{}{
			"error": err.Error(),
		})
	}
	
	return &GitHubToolsHandler{
		registry: registry,
		logger:   logger,
	}
}

// RegisterRoutes registers GitHub tool routes
func (h *GitHubToolsHandler) RegisterRoutes(router *gin.RouterGroup) {
	toolsGroup := router.Group("/tools/github")
	
	// List available tools
	toolsGroup.GET("", h.listTools)
	
	// Get tool schema
	toolsGroup.GET("/:tool_name", h.getToolSchema)
	
	// Execute tool
	toolsGroup.POST("/:tool_name", h.executeTool)
}

// ToolResponse represents a standard tool response
type ToolResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// listTools lists all available GitHub tools
func (h *GitHubToolsHandler) listTools(c *gin.Context) {
	tools := h.registry.ListTools()
	
	// Convert to simple list of tool definitions
	toolDefs := make([]tool.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		toolDefs = append(toolDefs, t.Definition)
	}
	
	c.JSON(http.StatusOK, ToolResponse{
		Success: true,
		Data:    toolDefs,
	})
}

// getToolSchema gets the schema for a specific tool
func (h *GitHubToolsHandler) getToolSchema(c *gin.Context) {
	toolName := c.Param("tool_name")
	
	// Get tool from registry
	t, err := h.registry.GetTool(toolName)
	if err != nil {
		c.JSON(http.StatusNotFound, ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Tool not found: %s", toolName),
		})
		return
	}
	
	c.JSON(http.StatusOK, ToolResponse{
		Success: true,
		Data:    t.Definition,
	})
}

// ToolExecuteRequest represents a request to execute a tool
type ToolExecuteRequest struct {
	Parameters map[string]interface{} `json:"parameters"`
}

// executeTool executes a specific tool
func (h *GitHubToolsHandler) executeTool(c *gin.Context) {
	toolName := c.Param("tool_name")
	
	// Get tool from registry
	t, err := h.registry.GetTool(toolName)
	if err != nil {
		c.JSON(http.StatusNotFound, ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Tool not found: %s", toolName),
		})
		return
	}
	
	// Parse request body
	var req ToolExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %s", err.Error()),
		})
		return
	}
	
	// Validate parameters
	if err := t.ValidateParams(req.Parameters); err != nil {
		c.JSON(http.StatusBadRequest, ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid parameters: %s", err.Error()),
		})
		return
	}
	
	// Execute tool
	result, err := t.Handler(req.Parameters)
	if err != nil {
		h.logger.Error("Tool execution failed", map[string]interface{}{
			"tool":  toolName,
			"error": err.Error(),
		})
		
		c.JSON(http.StatusInternalServerError, ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Tool execution failed: %s", err.Error()),
		})
		return
	}
	
	// Return response
	c.JSON(http.StatusOK, ToolResponse{
		Success: true,
		Data:    result,
	})
}

// RegisterGitHubTools registers GitHub tools with an MCP server
func RegisterGitHubTools(
	router *gin.Engine,
	githubAdapter *github.GitHubAdapter,
	logger *observability.Logger,
) {
	handler := NewGitHubToolsHandler(githubAdapter, logger)
	
	// Register with API group
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)
}

// GenerateToolSchemaJSON generates JSON Schema for GitHub tools
func GenerateToolSchemaJSON(registry *tool.ToolRegistry) ([]byte, error) {
	tools := registry.ListTools()
	
	// Convert to array of tool definitions
	toolDefs := make([]tool.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		toolDefs = append(toolDefs, t.Definition)
	}
	
	return json.MarshalIndent(toolDefs, "", "  ")
}
