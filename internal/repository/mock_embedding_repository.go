package repository

import (
	"context"
)

// NewMockEmbeddingRepository creates a mock embedding repository for testing
func NewMockEmbeddingRepository() *EmbeddingRepository {
	return &EmbeddingRepository{}
}

// MockEmbeddingRepository is a mock implementation of the EmbeddingRepository
type MockEmbeddingRepository struct {
	embeddings map[string][]*Embedding
}

// StoreEmbedding stores an embedding
func (m *MockEmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	if m.embeddings == nil {
		m.embeddings = make(map[string][]*Embedding)
	}
	
	m.embeddings[embedding.ContextID] = append(m.embeddings[embedding.ContextID], embedding)
	return nil
}

// SearchEmbeddings searches for embeddings
func (m *MockEmbeddingRepository) SearchEmbeddings(ctx context.Context, queryEmbedding []float32, contextID string, limit int) ([]*EmbeddingSearchResult, error) {
	return nil, nil
}

// GetContextEmbeddings gets all embeddings for a context
func (m *MockEmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	return m.embeddings[contextID], nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (m *MockEmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	delete(m.embeddings, contextID)
	return nil
}
