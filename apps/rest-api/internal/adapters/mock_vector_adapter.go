// Package adapters provides compatibility adapters for the repository interfaces
package adapters

import (
	"context"
	"time"

	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/repository"
)

// MockVectorRepository provides an in-memory implementation of the VectorAPIRepository
// interface for testing and development purposes
type MockVectorRepository struct {
	embeddings map[string][]*repository.Embedding
}

// NewMockVectorRepository creates a new mock vector repository
func NewMockVectorRepository() repository.VectorAPIRepository {
	return &MockVectorRepository{
		embeddings: make(map[string][]*repository.Embedding),
	}
}

// StoreEmbedding stores a vector embedding
func (m *MockVectorRepository) StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error {
	// Create a deep copy of the embedding to avoid modification issues
	embCopy := makeEmbeddingCopy(embedding)

	// Default timestamp if none provided
	if embCopy.Metadata == nil {
		embCopy.Metadata = make(map[string]any)
	}
	if _, exists := embCopy.Metadata["created_at"]; !exists {
		embCopy.Metadata["created_at"] = time.Now()
	}

	// Store the embedding by context ID
	m.embeddings[embedding.ContextID] = append(m.embeddings[embedding.ContextID], embCopy)
	return nil
}

// SearchEmbeddings performs a vector search with various filter options
func (m *MockVectorRepository) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*repository.Embedding, error) {
	// Simple mock implementation that returns embeddings that match the context and model
	result := []*repository.Embedding{}

	// Get embeddings for this context
	contextEmbs, ok := m.embeddings[contextID]
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

// SearchEmbeddings_Legacy performs a legacy vector search
func (m *MockVectorRepository) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*repository.Embedding, error) {
	// For legacy search, we'll use an empty modelID
	return m.SearchEmbeddings(ctx, queryVector, contextID, "", limit, 0.7)
}

// GetContextEmbeddings retrieves all embeddings for a context
func (m *MockVectorRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error) {
	result := []*repository.Embedding{}

	contextEmbs, ok := m.embeddings[contextID]
	if !ok {
		return result, nil // No embeddings for this context
	}

	// Deep copy to avoid modification issues
	for _, emb := range contextEmbs {
		result = append(result, makeEmbeddingCopy(emb))
	}

	return result, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (m *MockVectorRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	delete(m.embeddings, contextID)
	return nil
}

// GetEmbeddingsByModel retrieves all embeddings for a context and model
func (m *MockVectorRepository) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error) {
	result := []*repository.Embedding{}

	contextEmbs, ok := m.embeddings[contextID]
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

// GetSupportedModels returns a list of models with embeddings
func (m *MockVectorRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	models := make(map[string]bool)

	for _, embs := range m.embeddings {
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

// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
func (m *MockVectorRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	// Check if we have embeddings for this context
	contextEmbs, ok := m.embeddings[contextID]
	if !ok {
		return nil // No embeddings for this context, nothing to delete
	}

	// Filter out embeddings for the specified model
	filtered := make([]*repository.Embedding, 0, len(contextEmbs))
	for _, emb := range contextEmbs {
		if emb.ModelID != modelID {
			filtered = append(filtered, emb)
		}
	}

	// Update the context embeddings
	m.embeddings[contextID] = filtered
	return nil
}
