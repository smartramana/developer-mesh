package api

import (
	"context"
	"net/http"
	"time"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
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

// VectorRepositoryAdapter adapts between repository.VectorAPIRepository and EmbeddingRepositoryInterface
type VectorRepositoryAdapter struct {
	repo repository.VectorAPIRepository
}

// Ensure the adapter implements the interface
var _ EmbeddingRepositoryInterface = (*VectorRepositoryAdapter)(nil)

// StoreEmbedding adapts between models.Vector and repository.Embedding
func (a *VectorRepositoryAdapter) StoreEmbedding(ctx context.Context, vector *models.Vector) error {
	// Get content_index and model_id from metadata if present
	var contentIndex int
	var modelID string
	if vector.Metadata != nil {
		if ci, ok := vector.Metadata["content_index"]; ok {
			if ciInt, ok := ci.(int); ok {
				contentIndex = ciInt
			}
		}
		if mi, ok := vector.Metadata["model_id"]; ok {
			if miStr, ok := mi.(string); ok {
				modelID = miStr
			}
		}
	}

	// Convert from models.Vector to repository.Embedding
	repoEmbedding := &repository.Embedding{
		ID:           vector.ID,
		ContextID:    vector.TenantID, // Map TenantID to ContextID
		ContentIndex: contentIndex,    // From metadata
		Text:         vector.Content,  // Map Content to Text
		Embedding:    vector.Embedding,
		ModelID:      modelID,         // From metadata
	}
	return a.repo.StoreEmbedding(ctx, repoEmbedding)
}

// SearchEmbeddings adapts between the API and repository
func (a *VectorRepositoryAdapter) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*models.Vector, error) {
	results, err := a.repo.SearchEmbeddings(ctx, queryVector, contextID, modelID, limit, similarityThreshold)
	if err != nil {
		return nil, err
	}
	
	// Convert from repository.Embedding to models.Vector
	vectors := make([]*models.Vector, 0, len(results))
	for _, e := range results {
		// Create metadata map for content_index and model_id
		metadata := map[string]interface{}{
			"content_index": e.ContentIndex,
			"model_id":      e.ModelID,
		}

		vectors = append(vectors, &models.Vector{
			ID:        e.ID,
			TenantID:  e.ContextID, // Map ContextID to TenantID
			Content:   e.Text,      // Map Text to Content
			Embedding: e.Embedding,
			Metadata:  metadata,
			CreatedAt: time.Now(), // We don't have this from repository.Embedding
			UpdatedAt: time.Now(), // We don't have this from repository.Embedding
		})
	}
	return vectors, nil
}

// SearchEmbeddings_Legacy adapts for legacy search
func (a *VectorRepositoryAdapter) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*models.Vector, error) {
	results, err := a.repo.SearchEmbeddings_Legacy(ctx, queryVector, contextID, limit)
	if err != nil {
		return nil, err
	}
	
	// Convert from repository.Embedding to models.Vector
	vectors := make([]*models.Vector, 0, len(results))
	for _, e := range results {
		// Create metadata map for content_index and model_id
		metadata := map[string]interface{}{
			"content_index": e.ContentIndex,
			"model_id":      e.ModelID,
		}

		vectors = append(vectors, &models.Vector{
			ID:        e.ID,
			TenantID:  e.ContextID, // Map ContextID to TenantID
			Content:   e.Text,      // Map Text to Content
			Embedding: e.Embedding,
			Metadata:  metadata,
			CreatedAt: time.Now(), // We don't have this from repository.Embedding
			UpdatedAt: time.Now(), // We don't have this from repository.Embedding
		})
	}
	return vectors, nil
}

// GetContextEmbeddings adapts between the API and repository
func (a *VectorRepositoryAdapter) GetContextEmbeddings(ctx context.Context, contextID string) ([]*models.Vector, error) {
	results, err := a.repo.GetContextEmbeddings(ctx, contextID)
	if err != nil {
		return nil, err
	}
	
	// Convert from repository.Embedding to models.Vector
	vectors := make([]*models.Vector, 0, len(results))
	for _, e := range results {
		// Create metadata map for content_index and model_id
		metadata := map[string]interface{}{
			"content_index": e.ContentIndex,
			"model_id":      e.ModelID,
		}

		vectors = append(vectors, &models.Vector{
			ID:        e.ID,
			TenantID:  e.ContextID, // Map ContextID to TenantID
			Content:   e.Text,      // Map Text to Content
			Embedding: e.Embedding,
			Metadata:  metadata,
			CreatedAt: time.Now(), // We don't have this from repository.Embedding
			UpdatedAt: time.Now(), // We don't have this from repository.Embedding
		})
	}
	return vectors, nil
}

// DeleteContextEmbeddings adapts between the API and repository
func (a *VectorRepositoryAdapter) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	return a.repo.DeleteContextEmbeddings(ctx, contextID)
}

// GetEmbeddingsByModel adapts between the API and repository
func (a *VectorRepositoryAdapter) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*models.Vector, error) {
	results, err := a.repo.GetEmbeddingsByModel(ctx, contextID, modelID)
	if err != nil {
		return nil, err
	}
	
	// Convert from repository.Embedding to models.Vector
	vectors := make([]*models.Vector, 0, len(results))
	for _, e := range results {
		// Create metadata map for content_index and model_id
		metadata := map[string]interface{}{
			"content_index": e.ContentIndex,
			"model_id":      e.ModelID,
		}

		vectors = append(vectors, &models.Vector{
			ID:        e.ID,
			TenantID:  e.ContextID, // Map ContextID to TenantID
			Content:   e.Text,      // Map Text to Content
			Embedding: e.Embedding,
			Metadata:  metadata,
			CreatedAt: time.Now(), // We don't have this from repository.Embedding
			UpdatedAt: time.Now(), // We don't have this from repository.Embedding
		})
	}
	return vectors, nil
}

// GetSupportedModels adapts between the API and repository
func (a *VectorRepositoryAdapter) GetSupportedModels(ctx context.Context) ([]string, error) {
	return a.repo.GetSupportedModels(ctx)
}

// DeleteModelEmbeddings adapts between the API and repository
func (a *VectorRepositoryAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	return a.repo.DeleteModelEmbeddings(ctx, contextID, modelID)
}

// NewVectorRepositoryAdapter creates a new adapter
func NewVectorRepositoryAdapter(repo repository.VectorAPIRepository) EmbeddingRepositoryInterface {
	return &VectorRepositoryAdapter{repo: repo}
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
	StoreEmbedding(ctx context.Context, embedding *models.Vector) error
	SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*models.Vector, error)
	SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*models.Vector, error)
	GetContextEmbeddings(ctx context.Context, contextID string) ([]*models.Vector, error)
	DeleteContextEmbeddings(ctx context.Context, contextID string) error
	GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*models.Vector, error)
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

	// Create metadata for content_index and model_id
	metadata := map[string]interface{}{
		"content_index": req.ContentIndex,
		"model_id":      req.ModelID,
	}

	// Create Vector object as expected by the repository interface
	vector := &models.Vector{
		TenantID:  req.ContextID, // Map ContextID to TenantID
		Content:   req.Text,      // Map Text to Content
		Embedding: req.Embedding,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store embedding using repository
	err := v.embedRepo.StoreEmbedding(c.Request.Context(), vector)
	if err != nil {
		v.logger.Error("Failed to store embedding", map[string]interface{}{
			"error":      err.Error(),
			"context_id": req.ContextID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, vector)
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
		vectors, err = v.embedRepo.SearchEmbeddings(
			c.Request.Context(),
			req.QueryEmbedding,
			req.ContextID,
			req.ModelID,
			req.Limit,
			similarityThreshold,
		)
	} else {
		// For backward compatibility, use the legacy method without model filtering
		vectors, err = v.embedRepo.SearchEmbeddings_Legacy(
			c.Request.Context(),
			req.QueryEmbedding,
			req.ContextID,
			req.Limit,
		)
	}

	// Convert vectors to embeddings for API response
	embeddings := make([]*models.Embedding, 0, len(vectors))
	for _, v := range vectors {
		// Extract content_index and model_id from metadata
		contentIndex := 0
		modelID := ""
		if v.Metadata != nil {
			if ci, ok := v.Metadata["content_index"]; ok {
				if ciInt, ok := ci.(int); ok {
					contentIndex = ciInt
				}
			}
			if mi, ok := v.Metadata["model_id"]; ok {
				if miStr, ok := mi.(string); ok {
					modelID = miStr
				}
			}
		}

		// Convert to Embedding for API response
		embeddings = append(embeddings, &models.Embedding{
			ID:           v.ID,
			ContextID:    v.TenantID,  // Map TenantID back to ContextID
			ContentIndex: contentIndex,
			Text:         v.Content,   // Map Content back to Text
			Embedding:    v.Embedding,
			ModelID:      modelID,
		})
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
