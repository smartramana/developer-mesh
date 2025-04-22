package api

import (
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/internal/repository"
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
	ContextID      string    `json:"context_id" binding:"required"`
	QueryEmbedding []float32 `json:"query_embedding" binding:"required"`
	Limit          int       `json:"limit" binding:"required"`
}

// VectorAPI handles the vector operations API endpoints
type VectorAPI struct {
	embedRepo *repository.EmbeddingRepository
	logger    *observability.Logger
}

// NewVectorAPI creates a new vector API handler
func NewVectorAPI(embedRepo *repository.EmbeddingRepository, logger *observability.Logger) *VectorAPI {
	return &VectorAPI{
		embedRepo: embedRepo,
		logger:    logger,
	}
}

// RegisterRoutes registers the vector API routes with the given router group
func (v *VectorAPI) RegisterRoutes(group *gin.RouterGroup) {
	vectors := group.Group("/vectors")
	vectors.POST("/store", v.storeEmbedding)
	vectors.POST("/search", v.searchEmbeddings)
	vectors.GET("/context/:context_id", v.getContextEmbeddings)
	vectors.DELETE("/context/:context_id", v.deleteContextEmbeddings)
}

// Handler implementations for the vector endpoints

// storeEmbedding handles storing an embedding
func (v *VectorAPI) storeEmbedding(c *gin.Context) {
	var req StoreEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
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
	err := v.embedRepo.StoreEmbedding(c, embedding)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, embedding)
}

// searchEmbeddings handles searching for embeddings
func (v *VectorAPI) searchEmbeddings(c *gin.Context) {
	var req SearchEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Search embeddings using repository
	embeddings, err := v.embedRepo.SearchEmbeddings(c, req.QueryEmbedding, req.ContextID, req.Limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"embeddings": embeddings})
}

// getContextEmbeddings handles getting embeddings for a context
func (v *VectorAPI) getContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")

	// Get embeddings using repository
	embeddings, err := v.embedRepo.GetContextEmbeddings(c, contextID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"embeddings": embeddings})
}

// deleteContextEmbeddings handles deleting embeddings for a context
func (v *VectorAPI) deleteContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")

	// Delete embeddings using repository
	err := v.embedRepo.DeleteContextEmbeddings(c, contextID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "deleted"})
}
