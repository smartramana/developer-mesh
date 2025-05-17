package api

import (
	"context"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// ServerEmbeddingAdapter adapts between server and repository interface expectations
type ServerEmbeddingAdapter struct {
	repo repository.VectorAPIRepository
}

// NewServerEmbeddingAdapter creates a new adapter for the server
func NewServerEmbeddingAdapter(repo repository.VectorAPIRepository) *ServerEmbeddingAdapter {
	return &ServerEmbeddingAdapter{
		repo: repo,
	}
}

// StoreEmbedding adapts the server's vector to the repository's embedding
func (a *ServerEmbeddingAdapter) StoreEmbedding(ctx context.Context, vector *models.Vector) error {
	// Convert from models.Vector to repository.Embedding
	metadata := vector.Metadata
	
	var contentIndex int
	var modelID string
	
	if metadata != nil {
		if ci, ok := metadata["content_index"]; ok {
			if ciInt, ok := ci.(int); ok {
				contentIndex = ciInt
			}
		}
		if mi, ok := metadata["model_id"]; ok {
			if miStr, ok := mi.(string); ok {
				modelID = miStr
			}
		}
	}
	
	repoEmbedding := &repository.Embedding{
		ID:           vector.ID,
		ContextID:    vector.TenantID,  // Map TenantID to ContextID
		ContentIndex: contentIndex,     // From metadata
		Text:         vector.Content,   // Map Content to Text
		Embedding:    vector.Embedding,
		ModelID:      modelID,          // From metadata
	}
	
	return a.repo.StoreEmbedding(ctx, repoEmbedding)
}

// SearchEmbeddings adapts the search method for the server
func (a *ServerEmbeddingAdapter) SearchEmbeddings(ctx context.Context, queryEmbedding []float32, contextID string, modelID string, limit int, threshold float64) ([]*models.Vector, error) {
	// Call repository method
	repoEmbeddings, err := a.repo.SearchEmbeddings(ctx, queryEmbedding, contextID, modelID, limit, threshold)
	if err != nil {
		return nil, err
	}
	
	// Convert repository embeddings to models.Vector for the API
	return a.convertEmbeddingsToVectors(repoEmbeddings), nil
}

// SearchEmbeddings_Legacy adapts the legacy search method for the server
func (a *ServerEmbeddingAdapter) SearchEmbeddings_Legacy(ctx context.Context, queryEmbedding []float32, contextID string, limit int) ([]*models.Vector, error) {
	// Call repository method
	repoEmbeddings, err := a.repo.SearchEmbeddings_Legacy(ctx, queryEmbedding, contextID, limit)
	if err != nil {
		return nil, err
	}
	
	// Convert repository embeddings to models.Vector for the API
	return a.convertEmbeddingsToVectors(repoEmbeddings), nil
}

// GetContextEmbeddings adapts the context embeddings retrieval method for the server
func (a *ServerEmbeddingAdapter) GetContextEmbeddings(ctx context.Context, contextID string) ([]*models.Vector, error) {
	// Call repository method
	repoEmbeddings, err := a.repo.GetContextEmbeddings(ctx, contextID)
	if err != nil {
		return nil, err
	}
	
	// Convert repository embeddings to models.Vector for the API
	return a.convertEmbeddingsToVectors(repoEmbeddings), nil
}

// DeleteContextEmbeddings adapts the context embeddings deletion method for the server
func (a *ServerEmbeddingAdapter) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	return a.repo.DeleteContextEmbeddings(ctx, contextID)
}

// GetEmbeddingsByModel adapts the model embeddings retrieval method for the server
func (a *ServerEmbeddingAdapter) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*models.Vector, error) {
	// Call repository method
	repoEmbeddings, err := a.repo.GetEmbeddingsByModel(ctx, contextID, modelID)
	if err != nil {
		return nil, err
	}
	
	// Convert repository embeddings to models.Vector for the API
	return a.convertEmbeddingsToVectors(repoEmbeddings), nil
}

// GetSupportedModels adapts the supported models retrieval method for the server
func (a *ServerEmbeddingAdapter) GetSupportedModels(ctx context.Context) ([]string, error) {
	return a.repo.GetSupportedModels(ctx)
}

// DeleteModelEmbeddings adapts the model embeddings deletion method for the server
func (a *ServerEmbeddingAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	return a.repo.DeleteModelEmbeddings(ctx, contextID, modelID)
}

// Helper method to convert repository embeddings to models.Vector
func (a *ServerEmbeddingAdapter) convertEmbeddingsToVectors(embeddings []*repository.Embedding) []*models.Vector {
	vectors := make([]*models.Vector, 0, len(embeddings))
	
	for _, e := range embeddings {
		// Create metadata map for content_index and model_id
		metadata := map[string]interface{}{
			"content_index": e.ContentIndex,
			"model_id":      e.ModelID,
		}
		
		// Create vector from embedding
		vector := &models.Vector{
			ID:        e.ID,
			TenantID:  e.ContextID,    // Map ContextID to TenantID
			Content:   e.Text,         // Map Text to Content
			Embedding: e.Embedding,
			Metadata:  metadata,
			CreatedAt: time.Now(),     // We don't have this from repository.Embedding
			UpdatedAt: time.Now(),     // We don't have this from repository.Embedding
		}
		
		vectors = append(vectors, vector)
	}
	
	return vectors
}
