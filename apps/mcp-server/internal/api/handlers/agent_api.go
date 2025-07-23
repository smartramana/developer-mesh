package handlers

import (
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/agent"
	"github.com/developer-mesh/developer-mesh/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentAPI handles agent management endpoints
// Implements tenant-scoped CRUD operations for agents using the repository pattern.
type AgentAPI struct {
	repo agent.Repository
}

// NewAgentAPI creates a new instance of AgentAPI
func NewAgentAPI(repo agent.Repository) *AgentAPI {
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
	// Get tenant ID from context - it's represented as a string in models.Agent
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}
	// Set the tenant ID (uuid) on the agent model
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id format"})
		return
	}
	agent.TenantID = tenantUUID
	if agent.ID == "" {
		agent.ID = util.GenerateUUID() // Assume a UUID generator utility exists
	}
	if err := a.repo.Create(c.Request.Context(), &agent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, agent)
}

// listAgents lists all agents for the current tenant
func (a *AgentAPI) listAgents(c *gin.Context) {
	// Get tenant ID from context as a string
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	// Create a filter based on tenant ID
	filter := agent.FilterFromTenantID(tenantID)

	agents, err := a.repo.List(c.Request.Context(), filter)
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

	if err := a.repo.Update(c.Request.Context(), &agent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agent)
}
