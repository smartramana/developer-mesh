package api

import (
	"net/http"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/gin-gonic/gin"
)

// Routes for working with vector embeddings
func (s *Server) setupVectorRoutes(group *gin.RouterGroup) {
	vectorsGroup := group.Group("/vectors")
	vectorsGroup.POST("/store", s.storeEmbedding)
	vectorsGroup.POST("/search", s.searchEmbeddings)
	vectorsGroup.GET("/context/:context_id", s.getContextEmbeddings)
	vectorsGroup.DELETE("/context/:context_id", s.deleteContextEmbeddings)
	
	// New multi-model endpoints
	vectorsGroup.GET("/models", s.getSupportedModels)
	vectorsGroup.GET("/context/:context_id/model/:model_id", s.getModelEmbeddings)
	vectorsGroup.DELETE("/context/:context_id/model/:model_id", s.deleteModelEmbeddings)
}

// storeEmbedding handles storing a vector embedding
func (s *Server) storeEmbedding(c *gin.Context) {
	var req StoreEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Failed to bind store embedding request", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create metadata for content_index and model_id
	metadata := map[string]interface{}{
		"content_index": req.ContentIndex,
		"model_id":      req.ModelID,
	}

	// Create vector object as expected by the repository
	vector := &models.Vector{
		TenantID:  req.ContextID, // Map ContextID to TenantID
		Content:   req.Text,      // Map Text to Content
		Embedding: req.Embedding,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store embedding using adapter
	err := s.embeddingAdapter.StoreEmbedding(c.Request.Context(), vector)
	if err != nil {
		s.logger.Error("Failed to store embedding", map[string]interface{}{
			"error":      err.Error(),
			"context_id": req.ContextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, vector)
}

// searchEmbeddings handles searching for similar embeddings
func (s *Server) searchEmbeddings(c *gin.Context) {
	var req SearchEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Failed to bind search embeddings request", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate the request
	if req.Limit <= 0 {
		req.Limit = 10 // Default limit
	} else if req.Limit > 100 {
		req.Limit = 100 // Maximum limit
	}

	var vectors []*models.Vector
	var err error

	// Check if model ID is provided (new multi-model support)
	if req.ModelID != "" {
		// Default similarity threshold if not provided
		similarityThreshold := 0.5
		if req.SimilarityThreshold > 0 {
			similarityThreshold = req.SimilarityThreshold
		}

		// Use the new multi-model search method
		vectors, err = s.embeddingAdapter.SearchEmbeddings(
			c.Request.Context(),
			req.QueryEmbedding,
			req.ContextID,
			req.ModelID,
			req.Limit,
			similarityThreshold,
		)
	} else {
		// For backward compatibility, use the legacy method without model filtering
		vectors, err = s.embeddingAdapter.SearchEmbeddings_Legacy(
			c.Request.Context(),
			req.QueryEmbedding,
			req.ContextID,
			req.Limit,
		)
	}

	if err != nil {
		s.logger.Error("Failed to search embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": req.ContextID,
			"model_id":   req.ModelID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"embeddings": vectors})
}

// getContextEmbeddings handles retrieving all embeddings for a context
func (s *Server) getContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}

	// Get embeddings using adapter
	vectors, err := s.embeddingAdapter.GetContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		s.logger.Error("Failed to get context embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"embeddings": vectors})
}

// deleteContextEmbeddings handles deleting all embeddings for a context
func (s *Server) deleteContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}

	// Delete embeddings using adapter
	err := s.embeddingAdapter.DeleteContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		s.logger.Error("Failed to delete context embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// getSupportedModels handles getting a list of all model IDs with embeddings
func (s *Server) getSupportedModels(c *gin.Context) {
	// Get supported models from repository
	models, err := s.embeddingRepo.GetSupportedModels(c.Request.Context())
	if err != nil {
		s.logger.Error("Failed to get supported models", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// getModelEmbeddings handles getting embeddings for a specific model in a context
func (s *Server) getModelEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	modelID := c.Param("model_id")
	
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}
	
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model_id is required"})
		return
	}

	// Get embeddings using adapter
	vectors, err := s.embeddingAdapter.GetEmbeddingsByModel(c.Request.Context(), contextID, modelID)
	if err != nil {
		s.logger.Error("Failed to get model embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
			"model_id":   modelID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"embeddings": vectors})
}

// deleteModelEmbeddings handles deleting embeddings for a specific model in a context
func (s *Server) deleteModelEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	modelID := c.Param("model_id")
	
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}
	
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model_id is required"})
		return
	}

	// Delete embeddings using adapter
	err := s.embeddingAdapter.DeleteModelEmbeddings(c.Request.Context(), contextID, modelID)
	if err != nil {
		s.logger.Error("Failed to delete model embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
			"model_id":   modelID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
