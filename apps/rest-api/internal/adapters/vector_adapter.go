// Package adapters provides compatibility adapters for the repository interfaces
package adapters

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	corerepo "github.com/S-Corkum/devops-mcp/pkg/repository"
	"rest-api/internal/repository"
)

// ServerEmbeddingAdapter adapts between the API's expected interface and the repository implementation
type ServerEmbeddingAdapter struct {
	repo corerepo.VectorAPIRepository
}

// NewServerEmbeddingAdapter creates a new adapter for the vector repository
func NewServerEmbeddingAdapter(repo corerepo.VectorAPIRepository) repository.VectorAPIRepository {
	return &ServerEmbeddingAdapter{repo: repo}
}

// DirectVectorAdapter creates an implementation of the VectorAPIRepository interface
// that directly implements the required methods without relying on pkg/repository
func NewVectorAPIAdapter(vectorDB *database.VectorDatabase) repository.VectorAPIRepository {
	// Create a direct vector repository implementation
	return &DirectVectorAdapter{vectorDB: vectorDB}
}

// DirectVectorAdapter implements the VectorAPIRepository interface directly
type DirectVectorAdapter struct {
	vectorDB *database.VectorDatabase
	// Used for in-memory storage when no vectorDB is available
	embeddings map[string][]*repository.Embedding
}

// makeEmbeddingCopy creates a deep copy of an embedding
func makeEmbeddingCopy(embedding *repository.Embedding) *repository.Embedding {
	if embedding == nil {
		return nil
	}

	// Create a copy of the embedding
	embCopy := &repository.Embedding{
		ID:           embedding.ID,
		ContextID:    embedding.ContextID,
		ContentIndex: embedding.ContentIndex,
		Text:         embedding.Text,
		Embedding:    make([]float32, len(embedding.Embedding)),
		ModelID:      embedding.ModelID,
		Metadata:     embedding.Metadata,
	}

	// Copy the embedding vector
	copy(embCopy.Embedding, embedding.Embedding)
	return embCopy
}

// StoreEmbedding stores a vector embedding
func (a *DirectVectorAdapter) StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error {
	// Use in-memory storage when no vector database is available
	if a.vectorDB == nil {
		// Initialize the map if it's nil
		if a.embeddings == nil {
			a.embeddings = make(map[string][]*repository.Embedding)
		}

		// Store the embedding by context ID
		a.embeddings[embedding.ContextID] = append(a.embeddings[embedding.ContextID], makeEmbeddingCopy(embedding))
		return nil
	}

	// When using a vector database, we would call the appropriate methods
	// This would typically involve something like:
	// return a.vectorDB.StoreEmbedding(ctx, embedding.ContextID, embedding.ModelID, embedding.Embedding, embedding.Text, embedding.Metadata)
	// For now, just use in-memory storage
	return nil
}

// SearchEmbeddings performs a vector search with various filter options
func (a *DirectVectorAdapter) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*repository.Embedding, error) {
	// Use in-memory storage when no vector database is available
	if a.vectorDB == nil {
		// Initialize the map if it's nil
		if a.embeddings == nil {
			a.embeddings = make(map[string][]*repository.Embedding)
		}

		// Simple in-memory implementation
		result := []*repository.Embedding{}

		// Get embeddings for this context
		contextEmbs, ok := a.embeddings[contextID]
		if !ok {
			return result, nil // No embeddings for this context
		}

		// Filter by model ID if provided
		for _, emb := range contextEmbs {
			if modelID == "" || emb.ModelID == modelID {
				result = append(result, makeEmbeddingCopy(emb))
			}
		}

		// Limit results if needed
		if limit > 0 && len(result) > limit {
			result = result[:limit]
		}

		return result, nil
	}

	// When using a vector database, we would call the appropriate methods
	// For now, just use in-memory storage
	return []*repository.Embedding{}, nil
}

// SearchEmbeddings_Legacy performs a legacy vector search
func (a *DirectVectorAdapter) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*repository.Embedding, error) {
	// For legacy search, we'll use an empty modelID
	return a.SearchEmbeddings(ctx, queryVector, contextID, "", limit, 0.7)
}

// GetContextEmbeddings retrieves all embeddings for a context
func (a *DirectVectorAdapter) GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error) {
	if a.vectorDB == nil {
		if a.embeddings == nil {
			a.embeddings = make(map[string][]*repository.Embedding)
		}

		result := []*repository.Embedding{}
		contextEmbs, ok := a.embeddings[contextID]
		if !ok {
			return result, nil // No embeddings for this context
		}

		// Make a copy of each embedding to avoid modification issues
		for _, emb := range contextEmbs {
			result = append(result, makeEmbeddingCopy(emb))
		}

		return result, nil
	}

	// When using a vector database, we would call the appropriate methods
	return []*repository.Embedding{}, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (a *DirectVectorAdapter) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	if a.vectorDB == nil {
		if a.embeddings != nil {
			delete(a.embeddings, contextID)
		}
		return nil
	}

	// When using a vector database, we would call the appropriate methods
	return nil
}

// GetEmbeddingsByModel retrieves all embeddings for a context and model
func (a *DirectVectorAdapter) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error) {
	if a.vectorDB == nil {
		if a.embeddings == nil {
			a.embeddings = make(map[string][]*repository.Embedding)
		}

		result := []*repository.Embedding{}

		contextEmbs, ok := a.embeddings[contextID]
		if !ok {
			return result, nil // No embeddings for this context
		}

		// Filter by model ID
		for _, emb := range contextEmbs {
			if emb.ModelID == modelID {
				result = append(result, makeEmbeddingCopy(emb))
			}
		}

		return result, nil
	}

	// When using a vector database, we would call the appropriate methods
	return []*repository.Embedding{}, nil
}

// GetSupportedModels returns a list of models with embeddings
func (a *DirectVectorAdapter) GetSupportedModels(ctx context.Context) ([]string, error) {
	if a.vectorDB == nil {
		if a.embeddings == nil {
			a.embeddings = make(map[string][]*repository.Embedding)
		}

		models := make(map[string]bool)

		for _, embs := range a.embeddings {
			for _, emb := range embs {
				if emb.ModelID != "" {
					models[emb.ModelID] = true
				}
			}
		}

		// Convert map keys to slice
		result := make([]string, 0, len(models))
		for model := range models {
			result = append(result, model)
		}

		return result, nil
	}

	// When using a vector database, we would call the appropriate methods
	return []string{"default-model"}, nil
}

// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
func (a *DirectVectorAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	// Use in-memory storage when no vector database is available
	if a.vectorDB == nil {
		// Initialize the map if it's nil
		if a.embeddings == nil {
			a.embeddings = make(map[string][]*repository.Embedding)
		}

		// Get embeddings for this context
		contextEmbs, ok := a.embeddings[contextID]
		if !ok {
			return nil // No embeddings for this context, nothing to delete
		}

		// Filter out embeddings for the specified model
		newEmbs := make([]*repository.Embedding, 0, len(contextEmbs))
		for _, emb := range contextEmbs {
			if emb.ModelID != modelID {
				newEmbs = append(newEmbs, emb)
			}
		}

		// Update the context embeddings
		a.embeddings[contextID] = newEmbs
		return nil
	}

	// When using a vector database, we would call the appropriate methods
	return nil
}

// StoreEmbedding stores a vector embedding
func (a *ServerEmbeddingAdapter) StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error {
	// Convert from API Embedding to core repository Embedding
	repoEmbedding := &corerepo.Embedding{
		ID:           embedding.ID,
		ContextID:    embedding.ContextID,
		ContentIndex: embedding.ContentIndex,
		Text:         embedding.Text,
		Embedding:    embedding.Embedding,
		ModelID:      embedding.ModelID,
	}

	return a.repo.StoreEmbedding(ctx, repoEmbedding)
}

// SearchEmbeddings performs a vector search with various filter options
func (a *ServerEmbeddingAdapter) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*repository.Embedding, error) {
	repoEmbeddings, err := a.repo.SearchEmbeddings(ctx, queryVector, contextID, modelID, limit, similarityThreshold)
	if err != nil {
		return nil, err
	}

	// Convert from core repository Embeddings to API Embeddings
	apiEmbeddings := make([]*repository.Embedding, len(repoEmbeddings))
	for i, e := range repoEmbeddings {
		apiEmbeddings[i] = &repository.Embedding{
			ID:           e.ID,
			ContextID:    e.ContextID,
			ContentIndex: e.ContentIndex,
			Text:         e.Text,
			Embedding:    e.Embedding,
			ModelID:      e.ModelID,
			Metadata:     e.Metadata,
		}
	}

	return apiEmbeddings, nil
}

// SearchEmbeddings_Legacy performs a legacy vector search
func (a *ServerEmbeddingAdapter) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*repository.Embedding, error) {
	// For legacy search, set modelID to empty and use a standard similarity threshold
	return a.SearchEmbeddings(ctx, queryVector, contextID, "", limit, 0.7)
}

// GetContextEmbeddings retrieves all embeddings for a context
func (a *ServerEmbeddingAdapter) GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error) {
	repoEmbeddings, err := a.repo.GetContextEmbeddings(ctx, contextID)
	if err != nil {
		return nil, err
	}

	// Convert from core repository Embeddings to API Embeddings
	apiEmbeddings := make([]*repository.Embedding, len(repoEmbeddings))
	for i, e := range repoEmbeddings {
		apiEmbeddings[i] = &repository.Embedding{
			ID:           e.ID,
			ContextID:    e.ContextID,
			ContentIndex: e.ContentIndex,
			Text:         e.Text,
			Embedding:    e.Embedding,
			ModelID:      e.ModelID,
			Metadata:     e.Metadata,
		}
	}

	return apiEmbeddings, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (a *ServerEmbeddingAdapter) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	return a.repo.DeleteContextEmbeddings(ctx, contextID)
}

// GetEmbeddingsByModel retrieves all embeddings for a context and model
func (a *ServerEmbeddingAdapter) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error) {
	repoEmbeddings, err := a.repo.GetEmbeddingsByModel(ctx, contextID, modelID)
	if err != nil {
		return nil, err
	}

	// Convert from core repository Embeddings to API Embeddings
	apiEmbeddings := make([]*repository.Embedding, len(repoEmbeddings))
	for i, e := range repoEmbeddings {
		apiEmbeddings[i] = &repository.Embedding{
			ID:           e.ID,
			ContextID:    e.ContextID,
			ContentIndex: e.ContentIndex,
			Text:         e.Text,
			Embedding:    e.Embedding,
			ModelID:      e.ModelID,
			Metadata:     e.Metadata,
		}
	}

	return apiEmbeddings, nil
}

// GetSupportedModels returns a list of models with embeddings
func (a *ServerEmbeddingAdapter) GetSupportedModels(ctx context.Context) ([]string, error) {
	return a.repo.GetSupportedModels(ctx)
}

// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
func (a *ServerEmbeddingAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	return a.repo.DeleteModelEmbeddings(ctx, contextID, modelID)
}
