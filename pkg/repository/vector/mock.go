// Package vector provides repositories for vector operations
package vector

import (
	"context"
	"time"
)

// MockRepository provides a simple in-memory implementation of Repository for testing
type MockRepository struct {
	embeddings map[string]*Embedding
	models     map[string]bool
}

// NewMockRepository creates a new mock repository
func NewMockRepository() Repository {
	return &MockRepository{
		embeddings: make(map[string]*Embedding),
		models:     make(map[string]bool),
	}
}

// Create implements the standardized Repository method
func (m *MockRepository) Create(ctx context.Context, embedding *Embedding) error {
	return m.StoreEmbedding(ctx, embedding)
}

// Get implements the standardized Repository method
func (m *MockRepository) Get(ctx context.Context, id string) (*Embedding, error) {
	// Return the embedding directly from the map
	embedding, exists := m.embeddings[id]
	if !exists {
		return nil, nil
	}
	return embedding, nil
}

// List implements the standardized Repository method
func (m *MockRepository) List(ctx context.Context, filter Filter) ([]*Embedding, error) {
	var results []*Embedding

	for _, e := range m.embeddings {
		match := true

		for k, v := range filter {
			switch k {
			case "context_id":
				if e.ContextID != v.(string) {
					match = false
				}
			case "model_id":
				if e.ModelID != v.(string) {
					match = false
				}
			}
		}

		if match {
			results = append(results, e)
		}
	}

	return results, nil
}

// Update implements the standardized Repository method
func (m *MockRepository) Update(ctx context.Context, embedding *Embedding) error {
	return m.StoreEmbedding(ctx, embedding)
}

// Delete implements the standardized Repository method
func (m *MockRepository) Delete(ctx context.Context, id string) error {
	delete(m.embeddings, id)
	return nil
}

// StoreEmbedding implements Repository.StoreEmbedding
func (m *MockRepository) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	// Ensure embedding has a creation time
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = time.Now()
	}

	// Store the embedding by ID
	m.embeddings[embedding.ID] = embedding

	// Track the model ID
	if embedding.ModelID != "" {
		m.models[embedding.ModelID] = true
	}

	return nil
}

// SearchEmbeddings implements Repository.SearchEmbeddings
func (m *MockRepository) SearchEmbeddings(
	ctx context.Context,
	queryVector []float32,
	contextID string,
	modelID string,
	limit int,
	similarityThreshold float64,
) ([]*Embedding, error) {
	// Simple implementation that just returns embeddings matching contextID and modelID
	var results []*Embedding

	for _, e := range m.embeddings {
		if e.ContextID == contextID && (modelID == "" || e.ModelID == modelID) {
			results = append(results, e)
		}

		if len(results) >= limit && limit > 0 {
			break
		}
	}

	return results, nil
}

// SearchEmbeddings_Legacy implements Repository.SearchEmbeddings_Legacy
func (m *MockRepository) SearchEmbeddings_Legacy(
	ctx context.Context,
	queryVector []float32,
	contextID string,
	limit int,
) ([]*Embedding, error) {
	// Delegate to SearchEmbeddings with empty modelID
	return m.SearchEmbeddings(ctx, queryVector, contextID, "", limit, 0.0)
}

// GetContextEmbeddings implements Repository.GetContextEmbeddings
func (m *MockRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	var results []*Embedding

	for _, e := range m.embeddings {
		if e.ContextID == contextID {
			results = append(results, e)
		}
	}

	return results, nil
}

// DeleteContextEmbeddings implements Repository.DeleteContextEmbeddings
func (m *MockRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	// Find and remove all embeddings with the given contextID
	for id, e := range m.embeddings {
		if e.ContextID == contextID {
			delete(m.embeddings, id)
		}
	}

	return nil
}

// GetEmbeddingsByModel implements Repository.GetEmbeddingsByModel
func (m *MockRepository) GetEmbeddingsByModel(
	ctx context.Context,
	contextID string,
	modelID string,
) ([]*Embedding, error) {
	var results []*Embedding

	for _, e := range m.embeddings {
		if e.ContextID == contextID && e.ModelID == modelID {
			results = append(results, e)
		}
	}

	return results, nil
}

// GetSupportedModels implements Repository.GetSupportedModels
func (m *MockRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	models := make([]string, 0, len(m.models))

	for model := range m.models {
		models = append(models, model)
	}

	return models, nil
}

// DeleteModelEmbeddings implements Repository.DeleteModelEmbeddings
func (m *MockRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	// Find and remove all embeddings with the given contextID and modelID
	for id, e := range m.embeddings {
		if e.ContextID == contextID && e.ModelID == modelID {
			delete(m.embeddings, id)
		}
	}

	return nil
}
