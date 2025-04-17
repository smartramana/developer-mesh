package api

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/S-Corkum/mcp-server/internal/repository"
)

// VectorAPI handles vector embedding operations
type VectorAPI struct {
	repo *repository.EmbeddingRepository
}

// NewVectorAPI creates a new VectorAPI instance
func NewVectorAPI(repo *repository.EmbeddingRepository) *VectorAPI {
	return &VectorAPI{
		repo: repo,
	}
}

// RegisterRoutes registers all vector-related routes
func (api *VectorAPI) RegisterRoutes(router *gin.RouterGroup) {
	vectorRoutes := router.Group("/vectors")
	{
		vectorRoutes.POST("/store", api.storeEmbedding)
		vectorRoutes.POST("/search", api.searchEmbeddings)
		vectorRoutes.GET("/context/:context_id", api.getContextEmbeddings)
		vectorRoutes.DELETE("/context/:context_id", api.deleteContextEmbeddings)
	}
}

// StoreEmbeddingRequest defines the request format for storing embeddings
type StoreEmbeddingRequest struct {
	ContextID    string    `json:"context_id" binding:"required"`
	ContentIndex int       `json:"content_index" binding:"required"`
	Text         string    `json:"text" binding:"required"`
	Embedding    []float32 `json:"embedding" binding:"required"`
	ModelID      string    `json:"model_id" binding:"required"`
}

// SearchEmbeddingsRequest defines the request format for searching embeddings
type SearchEmbeddingsRequest struct {
	ContextID      string    `json:"context_id" binding:"required"`
	QueryEmbedding []float32 `json:"query_embedding" binding:"required"`
	Limit          int       `json:"limit"`
}

// storeEmbedding handles the POST /vectors/store endpoint
func (api *VectorAPI) storeEmbedding(c *gin.Context) {
	var req StoreEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	embedding := &repository.Embedding{
		ContextID:    req.ContextID,
		ContentIndex: req.ContentIndex,
		Text:         req.Text,
		Embedding:    req.Embedding,
		ModelID:      req.ModelID,
	}
	
	err := api.repo.StoreEmbedding(c.Request.Context(), embedding)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, embedding)
}

// searchEmbeddings handles the POST /vectors/search endpoint
func (api *VectorAPI) searchEmbeddings(c *gin.Context) {
	var req SearchEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if req.Limit <= 0 {
		req.Limit = 10 // Default limit
	}
	
	embeddings, err := api.repo.SearchEmbeddings(c.Request.Context(), req.QueryEmbedding, req.ContextID, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

// getContextEmbeddings handles the GET /vectors/context/:context_id endpoint
func (api *VectorAPI) getContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	embeddings, err := api.repo.GetContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

// deleteContextEmbeddings handles the DELETE /vectors/context/:context_id endpoint
func (api *VectorAPI) deleteContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	err := api.repo.DeleteContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
