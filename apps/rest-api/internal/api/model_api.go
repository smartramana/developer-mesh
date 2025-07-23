package api

import (
	"fmt"
	"strconv"

	"net/http"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/repository"

	"github.com/developer-mesh/developer-mesh/pkg/common/util"
	"github.com/developer-mesh/developer-mesh/pkg/models"
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
	models.POST("/search", m.searchModels)
	models.GET("/test", m.testModel) // Test endpoint - must be before /:id
	models.GET("/:id", m.getModel)
	models.PUT("/:id", m.updateModel)
	models.DELETE("/:id", m.deleteModel)
}

// createModel godoc
// @Summary Create a new AI model
// @Description Register a new AI model configuration for tenant
// @Tags models
// @Accept json
// @Produce json
// @Param model body models.Model true "Model configuration with name, provider, type"
// @Success 201 {object} models.Model "Created model"
// @Failure 400 {object} map[string]interface{} "Invalid request body"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /models [post]
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
	c.JSON(http.StatusCreated, model)
}

// testModel is a test endpoint to debug serialization
func (m *ModelAPI) testModel(c *gin.Context) {
	testModel := models.Model{
		ID:       "test-123",
		TenantID: "test-tenant",
		Name:     "Test Model",
	}
	c.JSON(http.StatusOK, testModel)
}

// listModels godoc
// @Summary List all models
// @Description List all AI models for the authenticated tenant with pagination
// @Tags models
// @Accept json
// @Produce json
// @Param limit query integer false "Results per page (max: 100, default: 20)"
// @Param offset query integer false "Pagination offset (default: 0)"
// @Success 200 {object} map[string]interface{} "List of models with pagination info"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /models [get]
func (m *ModelAPI) listModels(c *gin.Context) {
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	// Parse query parameters for pagination
	limit := 20 // default
	offset := 0 // default

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = min(parsedLimit, 100) // max limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Get all models for the tenant
	modelsList, err := m.repo.ListModels(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Apply pagination
	total := len(modelsList)
	start := min(offset, total)
	end := min(start+limit, total)

	paginatedModels := modelsList[start:end]

	// Build response with pagination info
	response := gin.H{
		"models": paginatedModels,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	// Add pagination links
	baseURL := fmt.Sprintf("%s%s", c.Request.Host, c.Request.URL.Path)
	links := gin.H{}

	// Add next link if there are more results
	if end < total {
		nextOffset := offset + limit
		links["next"] = fmt.Sprintf("%s?limit=%d&offset=%d", baseURL, limit, nextOffset)
	}

	// Add previous link if not at the beginning
	if offset > 0 {
		prevOffset := max(offset-limit, 0)
		links["prev"] = fmt.Sprintf("%s?limit=%d&offset=%d", baseURL, limit, prevOffset)
	}

	if len(links) > 0 {
		response["_links"] = links
	}

	c.JSON(http.StatusOK, response)
}

// updateModel godoc
// @Summary Update a model
// @Description Update model configuration and metadata
// @Tags models
// @Accept json
// @Produce json
// @Param id path string true "Model ID"
// @Param model body models.Model true "Updated model configuration"
// @Success 200 {object} models.Model "Updated model"
// @Failure 400 {object} map[string]interface{} "Invalid request body or missing model ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 403 {object} map[string]interface{} "Forbidden - model belongs to different tenant"
// @Failure 404 {object} map[string]interface{} "Model not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /models/{id} [put]
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
	// Return the updated model
	c.JSON(http.StatusOK, update)
}

// getModel godoc
// @Summary Get a model by ID
// @Description Retrieve a specific AI model configuration by ID
// @Tags models
// @Accept json
// @Produce json
// @Param id path string true "Model ID"
// @Success 200 {object} models.Model "Model details"
// @Failure 400 {object} map[string]interface{} "Missing model ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 403 {object} map[string]interface{} "Forbidden - model belongs to different tenant"
// @Failure 404 {object} map[string]interface{} "Model not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /models/{id} [get]
func (m *ModelAPI) getModel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model id required"})
		return
	}

	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	model, err := m.repo.GetModelByID(c.Request.Context(), tenantID, id)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}

	// Verify tenant access
	if model.TenantID != tenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, model)
}

// deleteModel godoc
// @Summary Delete a model
// @Description Delete an AI model configuration
// @Tags models
// @Accept json
// @Produce json
// @Param id path string true "Model ID"
// @Success 200 {object} map[string]interface{} "Deletion confirmation"
// @Failure 400 {object} map[string]interface{} "Missing model ID"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 403 {object} map[string]interface{} "Forbidden - model belongs to different tenant"
// @Failure 404 {object} map[string]interface{} "Model not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /models/{id} [delete]
func (m *ModelAPI) deleteModel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model id required"})
		return
	}

	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	// First check if the model exists and belongs to the tenant
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

	// Delete the model
	if err := m.repo.DeleteModel(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "model deleted successfully"})
}

// searchModels godoc
// @Summary Search models
// @Description Search for AI models based on query parameters
// @Tags models
// @Accept json
// @Produce json
// @Param request body object true "Search request with query, limit, and offset"
// @Success 200 {object} map[string]interface{} "Search results with matching models"
// @Failure 400 {object} map[string]interface{} "Invalid request body"
// @Failure 401 {object} map[string]interface{} "Unauthorized or missing tenant ID"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /models/search [post]
func (m *ModelAPI) searchModels(c *gin.Context) {
	tenantID := util.GetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
		return
	}

	var searchReq struct {
		Query  string `json:"query"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}

	if err := c.ShouldBindJSON(&searchReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if searchReq.Limit == 0 {
		searchReq.Limit = 20
	}
	if searchReq.Limit > 100 {
		searchReq.Limit = 100
	}

	// Search models
	models, err := m.repo.SearchModels(c.Request.Context(), tenantID, searchReq.Query, searchReq.Limit, searchReq.Offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": models,
		"query":   searchReq.Query,
		"limit":   searchReq.Limit,
		"offset":  searchReq.Offset,
	})
}
