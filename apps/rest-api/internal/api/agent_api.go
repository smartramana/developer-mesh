package api

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/storage"
	"github.com/S-Corkum/devops-mcp/pkg/common/util"
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
	agent.TenantID = tenantID
	if agent.ID == "" {
		agent.ID = util.GenerateUUID() // Assume a UUID generator utility exists
	}
	if err := a.repo.CreateAgent(c.Request.Context(), &agent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": agent.ID, "agent": agent})
}

// listAgents lists all agents for the authenticated tenant
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

// updateAgent updates agent metadata (tenant-scoped)
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
	if existing.TenantID != tenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}
	update.ID = id
	update.TenantID = tenantID
	if err := a.repo.UpdateAgent(c.Request.Context(), &update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "agent": update})
}
