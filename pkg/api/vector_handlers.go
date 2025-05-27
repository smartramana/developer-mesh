package api

import (
	"net/http"

	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/gin-gonic/gin"
)

// StoreEmbeddingRequest represents a request to store an embedding
type StoreEmbeddingRequest struct {
	ContextID    string    `json:"context_id" binding:"required"`
	ContentIndex int       `json:"content_index" binding:"required"`
	Text         string    `json:"text" binding:"required"`
	Embedding    []float32 `json:"embedding" binding:"required"`
	ModelID      string    `json:"model_id"`
}

// SearchEmbeddingsRequest represents a request to search for similar embeddings
type SearchEmbeddingsRequest struct {
	ContextID           string    `json:"context_id" binding:"required"`
	QueryEmbedding      []float32 `json:"query_embedding" binding:"required"`
	Limit               int       `json:"limit" binding:"required"`
	ModelID             string    `json:"model_id"`
	SimilarityThreshold float64   `json:"similarity_threshold"`
}

// Routes for working with vector embeddings
func (s *Server) setupVectorRoutes(group *gin.RouterGroup) {
	vectorsGroup := group.Group("/vectors")
	vectorsGroup.POST("/store", s.storeEmbedding)
	vectorsGroup.POST("/search", s.searchEmbeddings)
	vectorsGroup.GET("/context/:context_id", s.getContextEmbeddings)
	vectorsGroup.DELETE("/context/:context_id", s.deleteContextEmbeddings)

	// Multi-model endpoints
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

	// Create embedding object
	embedding := &repository.Embedding{
		ContextID:    req.ContextID,
		ContentIndex: req.ContentIndex,
		Text:         req.Text,
		Embedding:    req.Embedding,
		ModelID:      req.ModelID,
	}

	// Store embedding using repository
	err := s.embeddingRepo.StoreEmbedding(c.Request.Context(), embedding)
	if err != nil {
		s.logger.Error("Failed to store embedding", map[string]interface{}{
			"error":      err.Error(),
			"context_id": req.ContextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, embedding)
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

	var embeddings []*repository.Embedding
	var err error

	// Check if model ID is provided (multi-model support)
	if req.ModelID != "" {
		// Default similarity threshold if not provided
		similarityThreshold := 0.5
		if req.SimilarityThreshold > 0 {
			similarityThreshold = req.SimilarityThreshold
		}

		// Use the multi-model search method
		embeddings, err = s.embeddingRepo.SearchEmbeddings(
			c.Request.Context(),
			req.QueryEmbedding,
			req.ContextID,
			req.ModelID,
			req.Limit,
			similarityThreshold,
		)
	} else {
		// For backward compatibility, use the legacy method without model filtering
		embeddings, err = s.embeddingRepo.SearchEmbeddings_Legacy(
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

	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

// getContextEmbeddings handles retrieving all embeddings for a context
func (s *Server) getContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}

	// Get embeddings using repository
	embeddings, err := s.embeddingRepo.GetContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		s.logger.Error("Failed to get context embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

// deleteContextEmbeddings handles deleting all embeddings for a context
func (s *Server) deleteContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}

	// Delete embeddings using repository
	err := s.embeddingRepo.DeleteContextEmbeddings(c.Request.Context(), contextID)
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

	// Get embeddings using repository
	embeddings, err := s.embeddingRepo.GetEmbeddingsByModel(c.Request.Context(), contextID, modelID)
	if err != nil {
		s.logger.Error("Failed to get model embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
			"model_id":   modelID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
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

	// Delete embeddings using repository
	err := s.embeddingRepo.DeleteModelEmbeddings(c.Request.Context(), contextID, modelID)
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
