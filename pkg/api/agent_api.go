package api

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gin-gonic/gin"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/pkg/util"
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

// createAgent creates a new agent (tenant-scoped)
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
	c.JSON(http.StatusCreated, agent)
}

// listAgents lists all agents for the current tenant
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

	c.JSON(http.StatusOK, agents)
}

// updateAgent updates an existing agent
func (a *AgentAPI) updateAgent(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing agent id"})
		return
	}

	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	var agent models.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure the ID and tenant ID match what's in the URL and token
	agent.ID = id
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id format"})
		return
	}
	agent.TenantID = tenantUUID

	if err := a.repo.UpdateAgent(c.Request.Context(), &agent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agent)
}
