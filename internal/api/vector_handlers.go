package api

import (
	"github.com/gin-gonic/gin"
)

// StoreEmbeddingRequest represents a request to store an embedding
type StoreEmbeddingRequest struct {
	ContextID    string    `json:"context_id" binding:"required"`
	ContentIndex int       `json:"content_index"`
	Text         string    `json:"text" binding:"required"`
	Embedding    []float32 `json:"embedding" binding:"required"`
	ModelID      string    `json:"model_id" binding:"required"`
}

// SearchEmbeddingsRequest represents a request to search embeddings
type SearchEmbeddingsRequest struct {
	ContextID      string    `json:"context_id" binding:"required"`
	QueryEmbedding []float32 `json:"query_embedding" binding:"required"`
	Limit          int       `json:"limit"`
}

// RegisterVectorRoutes registers the vector API routes
func (s *Server) RegisterVectorRoutes(rg *gin.RouterGroup) {
	vectorRoutes := rg.Group("/vectors")
	{
		vectorRoutes.POST("/store", s.storeEmbedding)
		vectorRoutes.POST("/search", s.searchEmbeddings)
		vectorRoutes.GET("/context/:context_id", s.getContextEmbeddings)
		vectorRoutes.DELETE("/context/:context_id", s.deleteContextEmbeddings)
	}
}
