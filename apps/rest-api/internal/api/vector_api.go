package api

import (
	"context"
	"net/http"
	
	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/repository"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
)

// StoreEmbeddingRequest represents a request to store an embedding
type StoreEmbeddingRequest struct {
	ContextID    string    `json:"context_id" binding:"required"`
	ContentIndex int       `json:"content_index" binding:"required"`
	Text         string    `json:"text" binding:"required"`
	Embedding    []float32 `json:"embedding" binding:"required"`
	ModelID      string    `json:"model_id" binding:"required"`
}

// SearchEmbeddingsRequest represents a request to search embeddings
type SearchEmbeddingsRequest struct {
	ContextID           string    `json:"context_id" binding:"required"`
	QueryEmbedding      []float32 `json:"query_embedding" binding:"required"`
	Limit               int       `json:"limit" binding:"required"`
	ModelID             string    `json:"model_id"`
	SimilarityThreshold float64   `json:"similarity_threshold"`
}

// VectorAPI handles the vector operations API endpoints
type VectorAPI struct {
	embedRepo EmbeddingRepositoryInterface
	logger    observability.Logger
}

// NewVectorAPI creates a new vector API handler
func NewVectorAPI(embedRepo EmbeddingRepositoryInterface, logger observability.Logger) *VectorAPI {
	return &VectorAPI{
		embedRepo: embedRepo,
		logger:    logger,
	}
}

// EmbeddingRepositoryInterface defines the interface for embedding repository operations
type EmbeddingRepositoryInterface interface {
	StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error
	SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*repository.Embedding, error)
	SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*repository.Embedding, error)
	GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error)
	DeleteContextEmbeddings(ctx context.Context, contextID string) error
	GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error)
	GetSupportedModels(ctx context.Context) ([]string, error)
	DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error
}

// RegisterRoutes registers the vector API routes with the given router group
func (v *VectorAPI) RegisterRoutes(group *gin.RouterGroup) {
	vectors := group.Group("/vectors")
	vectors.POST("/store", v.storeEmbedding)
	vectors.POST("/search", v.searchEmbeddings)
	vectors.GET("/context/:context_id", v.getContextEmbeddings)
	vectors.DELETE("/context/:context_id", v.deleteContextEmbeddings)
	
	// New multi-model endpoints
	vectors.GET("/models", v.getSupportedModels)
	vectors.GET("/context/:context_id/model/:model_id", v.getModelEmbeddings)
	vectors.DELETE("/context/:context_id/model/:model_id", v.deleteModelEmbeddings)
}

// Handler implementations for the vector endpoints

// storeEmbedding handles storing an embedding
func (v *VectorAPI) storeEmbedding(c *gin.Context) {
	var req StoreEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		v.logger.Error("Failed to bind store embedding request", map[string]interface{}{
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
	err := v.embedRepo.StoreEmbedding(c.Request.Context(), embedding)
	if err != nil {
		v.logger.Error("Failed to store embedding", map[string]interface{}{
			"error":      err.Error(),
			"context_id": req.ContextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, embedding)
}

// searchEmbeddings handles searching for embeddings
func (v *VectorAPI) searchEmbeddings(c *gin.Context) {
	var req SearchEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		v.logger.Error("Failed to bind search embeddings request", map[string]interface{}{
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

	// Check if model ID is provided (new multi-model support)
	if req.ModelID != "" {
		// Default similarity threshold if not provided
		similarityThreshold := 0.5
		if req.SimilarityThreshold > 0 {
			similarityThreshold = req.SimilarityThreshold
		}

		// Use the new multi-model search method
		embeddings, err = v.embedRepo.SearchEmbeddings(
			c.Request.Context(),
			req.QueryEmbedding,
			req.ContextID,
			req.ModelID,
			req.Limit,
			similarityThreshold,
		)
	} else {
		// For backward compatibility, use the legacy method without model filtering
		embeddings, err = v.embedRepo.SearchEmbeddings_Legacy(
			c.Request.Context(),
			req.QueryEmbedding,
			req.ContextID,
			req.Limit,
		)
	}

	if err != nil {
		v.logger.Error("Failed to search embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": req.ContextID,
			"model_id":   req.ModelID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

// getContextEmbeddings handles getting embeddings for a context
func (v *VectorAPI) getContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}

	// Get embeddings using repository
	embeddings, err := v.embedRepo.GetContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		v.logger.Error("Failed to get context embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

// deleteContextEmbeddings handles deleting embeddings for a context
func (v *VectorAPI) deleteContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_id is required"})
		return
	}

	// Delete embeddings using repository
	err := v.embedRepo.DeleteContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		v.logger.Error("Failed to delete context embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// getSupportedModels handles getting a list of all model IDs with embeddings
func (v *VectorAPI) getSupportedModels(c *gin.Context) {
	// Get supported models from repository
	models, err := v.embedRepo.GetSupportedModels(c.Request.Context())
	if err != nil {
		v.logger.Error("Failed to get supported models", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// getModelEmbeddings handles getting embeddings for a specific model in a context
func (v *VectorAPI) getModelEmbeddings(c *gin.Context) {
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
	embeddings, err := v.embedRepo.GetEmbeddingsByModel(c.Request.Context(), contextID, modelID)
	if err != nil {
		v.logger.Error("Failed to get model embeddings", map[string]interface{}{
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
func (v *VectorAPI) deleteModelEmbeddings(c *gin.Context) {
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
	err := v.embedRepo.DeleteModelEmbeddings(c.Request.Context(), contextID, modelID)
	if err != nil {
		v.logger.Error("Failed to delete model embeddings", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
			"model_id":   modelID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
