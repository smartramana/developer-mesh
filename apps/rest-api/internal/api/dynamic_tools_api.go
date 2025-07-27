package api

import (
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/gin-gonic/gin"
)

// DynamicToolsAPI handles dynamic tool management endpoints
type DynamicToolsAPI struct {
	toolService   services.DynamicToolsServiceInterface
	logger        observability.Logger
	metricsClient observability.MetricsClient
	auditLogger   *auth.AuditLogger
}

// NewDynamicToolsAPI creates a new dynamic tools API handler
func NewDynamicToolsAPI(
	toolService services.DynamicToolsServiceInterface,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	auditLogger *auth.AuditLogger,
) *DynamicToolsAPI {
	return &DynamicToolsAPI{
		toolService:   toolService,
		logger:        logger,
		metricsClient: metricsClient,
		auditLogger:   auditLogger,
	}
}

// RegisterRoutes registers all dynamic tool API routes
func (api *DynamicToolsAPI) RegisterRoutes(router *gin.RouterGroup) {
	tools := router.Group("/tools")
	{
		// Tool management
		tools.GET("", api.ListTools)
		tools.POST("", api.CreateTool)
		tools.GET("/:toolId", api.GetTool)
		tools.PUT("/:toolId", api.UpdateTool)
		tools.DELETE("/:toolId", api.DeleteTool)

		// Discovery
		tools.POST("/discover", api.DiscoverTool)
		tools.GET("/discover/:sessionId", api.GetDiscoverySession)
		tools.POST("/discover/:sessionId/confirm", api.ConfirmDiscovery)

		// Health checks
		tools.GET("/:toolId/health", api.CheckHealth)
		tools.POST("/:toolId/health/refresh", api.RefreshHealth)

		// Execution
		tools.POST("/:toolId/execute/:action", api.ExecuteAction)
		tools.GET("/:toolId/actions", api.ListActions)

		// Credentials
		tools.PUT("/:toolId/credentials", api.UpdateCredentials)
	}
}

// ListTools lists all configured tools for the tenant
// @Summary List dynamic tools
// @Description Lists all configured dynamic tools for the authenticated tenant
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param status query string false "Filter by status (active, inactive)"
// @Param include_health query bool false "Include health status information"
// @Success 200 {object} ListToolsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools [get]
func (api *DynamicToolsAPI) ListTools(c *gin.Context) {
	start := time.Now()
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	// Query parameters
	status := c.Query("status")
	includeHealth := c.Query("include_health") == "true"

	tools, err := api.toolService.ListTools(c.Request.Context(), tenantID, status)
	if err != nil {
		api.logger.Error("Failed to list tools", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		// Record failure metric
		if api.metricsClient != nil {
			api.metricsClient.IncrementCounterWithLabels("api.tools.list.error", 1, map[string]string{
				"tenant_id": tenantID,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tools"})
		return
	}

	// Record success metric
	if api.metricsClient != nil {
		api.metricsClient.RecordHistogram("api.tools.list.duration", float64(time.Since(start).Milliseconds()), map[string]string{
			"tenant_id": tenantID,
		})
	}

	// Check health status if requested
	if includeHealth {
		for range tools {
			// This would trigger health checks in parallel
			// For now, we'll use cached health status
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
	})
}

// GetTool gets a specific tool by ID
// @Summary Get a dynamic tool
// @Description Retrieves details of a specific dynamic tool
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Success 200 {object} models.DynamicTool
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId} [get]
func (api *DynamicToolsAPI) GetTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	tool, err := api.toolService.GetTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
			return
		}
		api.logger.Error("Failed to get tool", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tool"})
		return
	}

	c.JSON(http.StatusOK, tool)
}

// CreateTool creates a new dynamic tool
// @Summary Create a dynamic tool
// @Description Creates a new dynamic tool with automatic discovery
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param tool body CreateToolRequest true "Tool configuration"
// @Success 201 {object} models.DynamicTool
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools [post]
func (api *DynamicToolsAPI) CreateTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	_ = c.GetString("user_id")

	var req CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if req.Name == "" || req.BaseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and base_url are required"})
		return
	}

	// Convert to ToolConfig
	config := tools.ToolConfig{
		TenantID:          tenantID,
		Name:              req.Name,
		BaseURL:           req.BaseURL,
		DocumentationURL:  req.DocumentationURL,
		OpenAPIURL:        req.OpenAPIURL,
		Config:            req.Config,
		Credential:        req.Credential,
		Provider:          req.Provider,
		PassthroughConfig: (*tools.PassthroughConfig)(req.PassthroughConfig),
	}

	// Add discovery hints if provided
	if req.DiscoveryHints != nil {
		if config.Config == nil {
			config.Config = make(map[string]interface{})
		}
		config.Config["discovery_hints"] = req.DiscoveryHints
	}

	// Create tool with discovery
	tool, err := api.toolService.CreateTool(c.Request.Context(), tenantID, config)
	if err != nil {
		api.logger.Error("Failed to create tool", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_name": req.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Audit log
	if api.auditLogger != nil {
		api.auditLogger.LogToolRegistration(c.Request.Context(), tenantID, tool.ID, tool.ToolName, true, nil)
	}

	c.JSON(http.StatusCreated, tool)
}

// UpdateTool updates an existing tool
// @Summary Update a dynamic tool
// @Description Updates an existing dynamic tool configuration
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Param tool body UpdateToolRequest true "Updated tool configuration"
// @Success 200 {object} models.DynamicTool
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId} [put]
func (api *DynamicToolsAPI) UpdateTool(c *gin.Context) {
	// Implementation similar to CreateTool
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// DeleteTool deletes a tool
// @Summary Delete a dynamic tool
// @Description Deletes a dynamic tool configuration
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Success 204 "No content"
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId} [delete]
func (api *DynamicToolsAPI) DeleteTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")
	_ = c.GetString("user_id")

	err := api.toolService.DeleteTool(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
			return
		}
		api.logger.Error("Failed to delete tool", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tool"})
		return
	}

	// Audit log
	if api.auditLogger != nil {
		api.auditLogger.LogToolRegistration(c.Request.Context(), tenantID, toolID, "tool.delete", true, nil)
	}

	c.Status(http.StatusNoContent)
}

// DiscoverTool starts a tool discovery session
// @Summary Discover a tool
// @Description Starts a discovery session to find OpenAPI specifications
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param discovery body DiscoveryRequest true "Discovery configuration"
// @Success 200 {object} models.DiscoverySession
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/discover [post]
func (api *DynamicToolsAPI) DiscoverTool(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// GetDiscoverySession gets discovery session status
func (api *DynamicToolsAPI) GetDiscoverySession(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// ConfirmDiscovery confirms a discovery session
func (api *DynamicToolsAPI) ConfirmDiscovery(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// CheckHealth checks tool health
// @Summary Check tool health
// @Description Checks the health status of a dynamic tool
// @Tags Dynamic Tools
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Success 200 {object} models.ToolHealthStatus
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tools/{toolId}/health [get]
func (api *DynamicToolsAPI) CheckHealth(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	health, err := api.toolService.CheckToolHealth(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
			return
		}
		api.logger.Error("Failed to check tool health", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check health"})
		return
	}

	c.JSON(http.StatusOK, health)
}

// RefreshHealth refreshes tool health status
func (api *DynamicToolsAPI) RefreshHealth(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	toolID := c.Param("toolId")

	health, err := api.toolService.RefreshToolHealth(c.Request.Context(), tenantID, toolID)
	if err != nil {
		if err.Error() == "tool not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
			return
		}
		api.logger.Error("Failed to refresh tool health", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh health"})
		return
	}

	c.JSON(http.StatusOK, health)
}

// ListActions lists available actions for a tool
func (api *DynamicToolsAPI) ListActions(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// ExecuteAction executes a tool action
func (api *DynamicToolsAPI) ExecuteAction(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// UpdateCredentials updates tool credentials
func (api *DynamicToolsAPI) UpdateCredentials(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// Request/Response types

type CreateToolRequest struct {
	Name              string                    `json:"name" binding:"required"`
	BaseURL           string                    `json:"base_url" binding:"required"`
	DocumentationURL  string                    `json:"documentation_url,omitempty"`
	OpenAPIURL        string                    `json:"openapi_url,omitempty"`
	Config            map[string]interface{}    `json:"config,omitempty"`
	Credential        *models.TokenCredential   `json:"credential,omitempty"`
	Provider          string                    `json:"provider,omitempty"`
	PassthroughConfig *models.PassthroughConfig `json:"passthrough_config,omitempty"`
	DiscoveryHints    map[string]interface{}    `json:"discovery_hints,omitempty"`
}

type UpdateToolRequest struct {
	DisplayName       string                    `json:"display_name,omitempty"`
	Config            map[string]interface{}    `json:"config,omitempty"`
	PassthroughConfig *models.PassthroughConfig `json:"passthrough_config,omitempty"`
}

type DiscoveryRequest struct {
	BaseURL        string                  `json:"base_url" binding:"required"`
	DiscoveryHints map[string]interface{}  `json:"discovery_hints,omitempty"`
	Credential     *models.TokenCredential `json:"credential,omitempty"`
}

type ListToolsResponse struct {
	Tools []*models.DynamicTool `json:"tools"`
	Count int                   `json:"count"`
}
