package api

import (
	"net/http"
	
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

// SearchEmbeddingsRequest represents a request to search for similar embeddings
type SearchEmbeddingsRequest struct {
	ContextID     string    `json:"context_id" binding:"required"`
	QueryEmbedding []float32 `json:"query_embedding" binding:"required"`
	Limit         int       `json:"limit"`
}

// storeEmbedding handles storing a vector embedding
func (s *Server) storeEmbedding(c *gin.Context) {
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
	
	if err := s.embeddingRepo.StoreEmbedding(c.Request.Context(), embedding); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, embedding)
}

// searchEmbeddings handles searching for similar embeddings
func (s *Server) searchEmbeddings(c *gin.Context) {
	var req SearchEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	
	embeddings, err := s.embeddingRepo.SearchEmbeddings(
		c.Request.Context(),
		req.QueryEmbedding,
		req.ContextID,
		limit,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

// getContextEmbeddings handles retrieving all embeddings for a context
func (s *Server) getContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	
	embeddings, err := s.embeddingRepo.GetContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"embeddings": embeddings})
}

// deleteContextEmbeddings handles deleting all embeddings for a context
func (s *Server) deleteContextEmbeddings(c *gin.Context) {
	contextID := c.Param("context_id")
	
	if err := s.embeddingRepo.DeleteContextEmbeddings(c.Request.Context(), contextID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
