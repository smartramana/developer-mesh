package repository

import (
	"context"
	"time"
)

// MockEmbeddingRepository is a mock implementation of the EmbeddingRepository for testing
type MockEmbeddingRepository struct {
	// Stores embeddings by contextID
	embeddings map[string][]*Embedding
}

// NewMockEmbeddingRepository creates a mock embedding repository for testing
func NewMockEmbeddingRepository() *MockEmbeddingRepository {
	return &MockEmbeddingRepository{
		embeddings: make(map[string][]*Embedding),
	}
}

// StoreEmbedding stores an embedding
func (m *MockEmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	if m.embeddings == nil {
		m.embeddings = make(map[string][]*Embedding)
	}

	// Generate a mock ID if not present
	if embedding.ID == "" {
		embedding.ID = "mock-embedding-" + time.Now().Format("20060102150405")
	}

	// Set created time if not present
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = time.Now()
	}

	// Store the embedding
	m.embeddings[embedding.ContextID] = append(m.embeddings[embedding.ContextID], embedding)
	return nil
}

// SearchEmbeddings searches for embeddings similar to the query vector
func (m *MockEmbeddingRepository) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error) {
	if m.embeddings == nil || len(m.embeddings[contextID]) == 0 {
		return []*Embedding{}, nil
	}

	// In a real implementation, we would compute vector similarities
	// For the mock, we'll just return the first 'limit' embeddings
	resultCount := limit
	if resultCount > len(m.embeddings[contextID]) {
		resultCount = len(m.embeddings[contextID])
	}

	return m.embeddings[contextID][:resultCount], nil
}

// GetContextEmbeddings gets all embeddings for a context
func (m *MockEmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	if m.embeddings == nil {
		return []*Embedding{}, nil
	}

	// Return embeddings for the given contextID
	return m.embeddings[contextID], nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (m *MockEmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	if m.embeddings != nil {
		delete(m.embeddings, contextID)
	}
	return nil
}
