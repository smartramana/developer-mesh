// Package adapters provides compatibility adapters for the repository interfaces
package adapters

import (
	"context"
	
	"rest-api/internal/repository"
)

// MockSearchAdapter implements the API's search repository interface
// with a simple implementation for initial testing
type MockSearchAdapter struct {}

// NewMockSearchAdapter creates a new mock search adapter
func NewMockSearchAdapter() repository.SearchRepository {
	return &MockSearchAdapter{}
}

// SearchByContentID searches for embeddings by content ID
func (m *MockSearchAdapter) SearchByContentID(ctx context.Context, contentID string, options *repository.SearchOptions) (*repository.SearchResults, error) {
	// Return empty results for mock implementation
	return &repository.SearchResults{
		Results: []*repository.SearchResult{},
		Total:   0,
		HasMore: false,
	}, nil
}

// SearchByText searches for embeddings by text
func (m *MockSearchAdapter) SearchByText(ctx context.Context, text string, options *repository.SearchOptions) (*repository.SearchResults, error) {
	// Return empty results for mock implementation
	return &repository.SearchResults{
		Results: []*repository.SearchResult{},
		Total:   0,
		HasMore: false,
	}, nil
}

// SearchByEmbedding searches for embeddings by embedding vector
func (m *MockSearchAdapter) SearchByEmbedding(ctx context.Context, embedding []float32, options *repository.SearchOptions) (*repository.SearchResults, error) {
	// Return empty results for mock implementation
	return &repository.SearchResults{
		Results: []*repository.SearchResult{},
		Total:   0,
		HasMore: false,
	}, nil
}
