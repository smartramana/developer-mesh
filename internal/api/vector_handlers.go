package api

import (
	"fmt"
	"net/http"

	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/gin-gonic/gin"
)

// StoreEmbeddingHandler handles requests to store vector embeddings
func (s *Server) StoreEmbeddingHandler(c *gin.Context) {
	var req StoreEmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Check the embedding dimension - must be 1536 as defined in the database schema
	const expectedDimension = 1536
	if len(req.Embedding) != expectedDimension {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid embedding dimension: expected %d dimensions, got %d. The database schema requires exactly %d dimensions for compatibility with models like Amazon Titan Embeddings.", 
				expectedDimension, len(req.Embedding), expectedDimension),
		})
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

	// Store in repository
	if err := s.embeddingRepo.StoreEmbedding(c.Request.Context(), embedding); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store embedding: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": embedding.ID, "message": "embedding stored"})
}

// SearchEmbeddingsHandler handles semantic search requests
func (s *Server) SearchEmbeddingsHandler(c *gin.Context) {
	var req SearchEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Default limit if not specified
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}

	// Search for similar embeddings
	results, err := s.embeddingRepo.SearchEmbeddings(
		c.Request.Context(), 
		req.QueryEmbedding, 
		req.ContextID, 
		limit,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// GetContextEmbeddingsHandler retrieves all embeddings for a context
func (s *Server) GetContextEmbeddingsHandler(c *gin.Context) {
	contextID := c.Param("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing context ID"})
		return
	}

	embeddings, err := s.embeddingRepo.GetContextEmbeddings(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve embeddings: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, embeddings)
}

// DeleteContextEmbeddingsHandler deletes all embeddings for a context
func (s *Server) DeleteContextEmbeddingsHandler(c *gin.Context) {
	contextID := c.Param("context_id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing context ID"})
		return
	}

	if err := s.embeddingRepo.DeleteContextEmbeddings(c.Request.Context(), contextID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete embeddings: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "embeddings deleted"})
}
