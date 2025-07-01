package api

import (
	"net/http"

	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/repository"

	"github.com/S-Corkum/devops-mcp/pkg/common/util"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentAPI handles agent management endpoints
// Implements tenant-scoped CRUD operations for agents using the repository pattern.
type AgentAPI struct {
	repo repository.AgentRepository
}

func NewAgentAPI(repo repository.AgentRepository) *AgentAPI {
	return &AgentAPI{repo: repo}
}

// RegisterRoutes registers agent endpoints under /agents
func (a *AgentAPI) RegisterRoutes(router *gin.RouterGroup) {
	agents := router.Group("/agents")
	agents.POST("", a.createAgent)
	agents.GET("", a.listAgents)
	agents.PUT(":id", a.updateAgent)
}

// createAgent godoc
// @Summary Create a new AI agent
// @Description Register a new AI agent with the orchestration platform
// @Tags agents
// @Accept json
// @Produce json
// @Param agent body models.Agent true "Agent configuration with name, type, capabilities"
// @Success 201 {object} map[string]interface{} "Created agent with ID"
// @Failure 400 {object} map[string]interface{} "Invalid request body or tenant ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /agents [post]
func (a *AgentAPI) createAgent(c *gin.Context) {
	var agent models.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id format"})
		return
	}
	agent.TenantID = tenantUUID
	if agent.ID == "" {
		agent.ID = util.GenerateUUID() // Assume a UUID generator utility exists
	}
	if err := a.repo.CreateAgent(c.Request.Context(), &agent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": agent.ID, "agent": agent})
}

// listAgents godoc
// @Summary List all agents
// @Description List all AI agents for the authenticated tenant
// @Tags agents
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "List of agents"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /agents [get]
func (a *AgentAPI) listAgents(c *gin.Context) {
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}
	agents, err := a.repo.ListAgents(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// updateAgent godoc
// @Summary Update an agent
// @Description Update agent metadata and configuration
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param agent body models.Agent true "Updated agent configuration"
// @Success 200 {object} map[string]interface{} "Updated agent"
// @Failure 400 {object} map[string]interface{} "Invalid request body or missing agent ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 403 {object} map[string]interface{} "Forbidden - agent belongs to different tenant"
// @Failure 404 {object} map[string]interface{} "Agent not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /agents/{id} [put]
func (a *AgentAPI) updateAgent(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent id required"})
		return
	}
	var update models.Agent
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}
	existing, err := a.repo.GetAgentByID(c.Request.Context(), tenantID, id)
	if err != nil {
		// If it's a not found error, return 404
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id format"})
		return
	}
	if existing.TenantID != tenantUUID {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}
	update.ID = id
	update.TenantID = tenantUUID
	if err := a.repo.UpdateAgent(c.Request.Context(), &update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "agent": update})
}
