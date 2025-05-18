package api

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/common/util"
	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/repository"
)

// getTenantIDFromContext extracts the tenant ID from the Gin context (from AuthMiddleware)
func getTenantIDFromContext(c *gin.Context) string {
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(map[string]interface{}); ok {
			if tid, ok := u["tenant_id"].(string); ok {
				return tid
			}
		}
	}
	return ""
}


// ModelAPI handles model management endpoints
// Implements tenant-scoped CRUD operations for models using the repository pattern.
type ModelAPI struct {
	repo repository.ModelRepository
}

// NewModelAPI creates a new ModelAPI with the provided repository
func NewModelAPI(repo repository.ModelRepository) *ModelAPI {
	return &ModelAPI{repo: repo}
}


// RegisterRoutes registers model endpoints under /models
func (m *ModelAPI) RegisterRoutes(router *gin.RouterGroup) {
	models := router.Group("/models")
	models.POST("", m.createModel)
	models.GET("", m.listModels)
	models.PUT(":id", m.updateModel)
}

// createModel creates a new model (tenant-scoped)
func (m *ModelAPI) createModel(c *gin.Context) {
	var model models.Model
	if err := c.ShouldBindJSON(&model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}
	model.TenantID = tenantID
	if model.ID == "" {
		model.ID = util.GenerateUUID() // Assume a UUID generator utility exists
	}
	if err := m.repo.CreateModel(c.Request.Context(), &model); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": model.ID, "model": model})
}

// listModels lists all models for the authenticated tenant
func (m *ModelAPI) listModels(c *gin.Context) {
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}
	modelsList, err := m.repo.ListModels(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": modelsList})
}

// updateModel updates model metadata (tenant-scoped)
func (m *ModelAPI) updateModel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model id required"})
		return
	}
	var update models.Model
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}
	existing, err := m.repo.GetModelByID(c.Request.Context(), tenantID, id)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}
	if existing.TenantID != tenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}
	update.ID = id
	update.TenantID = tenantID
	if err := m.repo.UpdateModel(c.Request.Context(), &update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "model": update})
}
