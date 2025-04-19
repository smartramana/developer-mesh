package repository

import (
	"context"
)

// MockEmbeddingRepository is a mock implementation of the EmbeddingRepository
type MockEmbeddingRepository struct {
	// Using empty maps for basic implementation
	embeddings map[string][]interface{}
}

// NewMockEmbeddingRepository creates a mock embedding repository for testing
func NewMockEmbeddingRepository() *MockEmbeddingRepository {
	return &MockEmbeddingRepository{
		embeddings: make(map[string][]interface{}),
	}
}

// StoreEmbedding stores an embedding
func (m *MockEmbeddingRepository) StoreEmbedding(ctx context.Context, embedding interface{}) error {
	// Extract contextID from the embedding if needed
	// For now, just using a default contextID for demonstration
	contextID := "default"
	
	if m.embeddings == nil {
		m.embeddings = make(map[string][]interface{})
	}
	
	m.embeddings[contextID] = append(m.embeddings[contextID], embedding)
	return nil
}

// SearchEmbeddings searches for embeddings
func (m *MockEmbeddingRepository) SearchEmbeddings(ctx context.Context, queryEmbedding []float32, contextID string, limit int) ([]interface{}, error) {
	return nil, nil
}

// GetContextEmbeddings gets all embeddings for a context
func (m *MockEmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]interface{}, error) {
	return m.embeddings[contextID], nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (m *MockEmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	delete(m.embeddings, contextID)
	return nil
}
