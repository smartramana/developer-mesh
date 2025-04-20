package api

import (
	"net/http"

	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/gin-gonic/gin"
)

// VectorAPI handles vector-related API endpoints
type VectorAPI struct {
	embeddingRepo *repository.EmbeddingRepository
	logger        *observability.Logger
}

// NewVectorAPI creates a new vector API handler
func NewVectorAPI(embeddingRepo *repository.EmbeddingRepository, logger *observability.Logger) *VectorAPI {
	if logger == nil {
		logger = observability.NewLogger("vector_api")
	}
	
	return &VectorAPI{
		embeddingRepo: embeddingRepo,
		logger:        logger,
	}
}

// RegisterRoutes registers vector API routes
func (api *VectorAPI) RegisterRoutes(router *gin.RouterGroup) {
	vectorRoutes := router.Group("/vectors")
	{
		vectorRoutes.POST("/store", api.StoreEmbedding)
		vectorRoutes.POST("/search", api.SearchEmbeddings)
		vectorRoutes.GET("/context/:contextID", api.GetContextEmbeddings)
		vectorRoutes.GET("/context/:contextID/model/:modelID", api.GetModelEmbeddings)
		vectorRoutes.DELETE("/context/:contextID", api.DeleteContextEmbeddings)
		vectorRoutes.DELETE("/context/:contextID/model/:modelID", api.DeleteModelEmbeddings)
		vectorRoutes.GET("/models", api.GetSupportedModels)
	}
}

// StoreEmbedding stores a vector embedding
func (api *VectorAPI) StoreEmbedding(c *gin.Context) {
	var embedding repository.Embedding
	
	if err := c.ShouldBindJSON(&embedding); err != nil {
		api.logger.Warn("Invalid request body for store embedding", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Validate required fields
	if embedding.ContextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}
	
	if embedding.ModelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model_id is required"})
		return
	}
	
	if len(embedding.Embedding) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "embedding vector cannot be empty"})
		return
	}
	
	// Store embedding
	err := api.embeddingRepo.StoreEmbedding(c.Request.Context(), &embedding)
	if err != nil {
		api.logger.Error("Failed to store embedding", map[string]interface{}{
			"error":      err.Error(),
			"context_id": embedding.ContextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Return stored embedding (without the vector to reduce response size)
	embedding.Embedding = nil
	
	c.JSON(http.StatusCreated, embedding)
}

// SearchEmbeddings searches for similar embeddings
func (api *VectorAPI) SearchEmbeddings(c *gin.Context) {
	var request struct {
		ContextID       string    `json:"context_id"`
		QueryEmbedding  []float32 `json:"query_embedding"`
		ModelID         string    `json:"model_id"`
		Limit           int       `json:"limit,omitempty"`
		Threshold       float64   `json:"threshold,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		api.logger.Warn("Invalid request body for search embeddings", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Validate required fields
	if request.ContextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}
	
	if request.ModelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model_id is required"})
		return
	}
	
	if len(request.QueryEmbedding) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query_embedding cannot be empty"})
		return
	}
	
	// Set defaults
	if request.Limit <= 0 {
		request.Limit = 10
	}
	
	if request.Threshold <= 0 {
		request.Threshold = 0.7
	}
	
	// Search embeddings
	results, err := api.embeddingRepo.SearchEmbeddings(
		c.Request.Context(),
		request.QueryEmbedding,
		request.ContextID,
		request.ModelID,
		request.Limit,
		request.Threshold,
	)
	
	if err != nil {
		api.logger.Error("Failed to search embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": request.ContextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Remove embedding vectors from response to reduce size
	for _, result := range results {
		result.Embedding = nil
	}
	
	c.JSON(http.StatusOK, gin.H{
		"context_id": request.ContextID,
		"model_id":   request.ModelID,
		"results":    results,
	})
}

// GetContextEmbeddings gets all embeddings for a context
func (api *VectorAPI) GetContextEmbeddings(c *gin.Context) {
	contextID := c.Param("contextID")
	
	// Get embeddings
	embeddings, err := api.embeddingRepo.GetContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to get context embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Remove embedding vectors from response to reduce size
	for _, embedding := range embeddings {
		embedding.Embedding = nil
	}
	
	c.JSON(http.StatusOK, gin.H{
		"context_id": contextID,
		"embeddings": embeddings,
	})
}

// GetModelEmbeddings gets embeddings for a specific model in a context
func (api *VectorAPI) GetModelEmbeddings(c *gin.Context) {
	contextID := c.Param("contextID")
	modelID := c.Param("modelID")
	
	// Get embeddings
	embeddings, err := api.embeddingRepo.GetEmbeddingsByModel(
		c.Request.Context(),
		contextID,
		modelID,
	)
	
	if err != nil {
		api.logger.Error("Failed to get model embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
			"model_id":   modelID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Remove embedding vectors from response to reduce size
	for _, embedding := range embeddings {
		embedding.Embedding = nil
	}
	
	c.JSON(http.StatusOK, gin.H{
		"context_id": contextID,
		"model_id":   modelID,
		"embeddings": embeddings,
	})
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (api *VectorAPI) DeleteContextEmbeddings(c *gin.Context) {
	contextID := c.Param("contextID")
	
	// Delete embeddings
	err := api.embeddingRepo.DeleteContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		api.logger.Error("Failed to delete context embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "embeddings deleted successfully",
	})
}

// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
func (api *VectorAPI) DeleteModelEmbeddings(c *gin.Context) {
	contextID := c.Param("contextID")
	modelID := c.Param("modelID")
	
	// Delete embeddings
	err := api.embeddingRepo.DeleteModelEmbeddings(
		c.Request.Context(),
		contextID,
		modelID,
	)
	
	if err != nil {
		api.logger.Error("Failed to delete model embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
			"model_id":   modelID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "model embeddings deleted successfully",
	})
}

// GetSupportedModels gets a list of supported embedding models
func (api *VectorAPI) GetSupportedModels(c *gin.Context) {
	// Get models
	models, err := api.embeddingRepo.GetSupportedModels(c.Request.Context())
	if err != nil {
		api.logger.Error("Failed to get supported models", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"models": models,
	})
}
