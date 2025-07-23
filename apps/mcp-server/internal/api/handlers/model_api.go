package handlers

import (
	"fmt"
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/model"
	"github.com/developer-mesh/developer-mesh/pkg/util"
	"github.com/gin-gonic/gin"
)

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
		model.ID = util.GenerateUUID()
	}

	// Use the standardized Create method from the repository
	if err := m.repo.Create(c.Request.Context(), &model); err != nil {
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

	// Create a filter for the tenant ID
	// Using model.Filter from the repository/model package
	filter := model.FilterFromTenantID(tenantID)

	// Use the standardized List method
	modelsList, err := m.repo.List(c.Request.Context(), filter)
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

	// Use standardized Repository.Get method with ID parameter
	existing, err := m.repo.Get(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "not found" || err.Error() == fmt.Sprintf("model not found: %s", id) {
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

	// Enforce tenant ownership after retrieval for security
	// This maintains tenant isolation while using the standardized repository pattern
	if existing.TenantID != tenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}
	update.ID = id
	update.TenantID = tenantID

	// Use standardized Update method
	if err := m.repo.Update(c.Request.Context(), &update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "model": update})
}
