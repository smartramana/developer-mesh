package proxies

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/search"
)

// MockSearchRepository provides a temporary implementation of the SearchRepository
// interface while the embedding package is being integrated
type MockSearchRepository struct {
	logger observability.Logger
}

// NewMockSearchRepository creates a new mock search repository
func NewMockSearchRepository(logger observability.Logger) repository.SearchRepository {
	if logger == nil {
		logger = observability.NewLogger("mock-search-repository")
	}

	return &MockSearchRepository{
		logger: logger,
	}
}

// SearchByText performs a vector search using a text query
func (m *MockSearchRepository) SearchByText(ctx context.Context, query string, options *repository.SearchOptions) (*repository.SearchResults, error) {
	m.logger.Debug("Mock text search", map[string]interface{}{
		"query": query,
	})

	// Return empty results for now
	return &repository.SearchResults{
		Results: []*repository.SearchResult{},
		Total:   0,
		HasMore: false,
	}, nil
}

// SearchByVector performs a vector search using a pre-computed vector
func (m *MockSearchRepository) SearchByVector(ctx context.Context, vector []float32, options *repository.SearchOptions) (*repository.SearchResults, error) {
	m.logger.Debug("Mock vector search", map[string]interface{}{
		"vector_size": len(vector),
	})

	// Return empty results for now
	return &repository.SearchResults{
		Results: []*repository.SearchResult{},
		Total:   0,
		HasMore: false,
	}, nil
}

// SearchByContentID performs a "more like this" search using an existing content ID
func (m *MockSearchRepository) SearchByContentID(ctx context.Context, contentID string, options *repository.SearchOptions) (*repository.SearchResults, error) {
	m.logger.Debug("Mock content ID search", map[string]interface{}{
		"content_id": contentID,
	})

	// Return empty results for now
	return &repository.SearchResults{
		Results: []*repository.SearchResult{},
		Total:   0,
		HasMore: false,
	}, nil
}

// GetSupportedModels retrieves a list of all models with embeddings
func (m *MockSearchRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	m.logger.Debug("Mock get supported models", nil)

	// Return a list of mock models
	return []string{"mock-model-1", "mock-model-2"}, nil
}

// GetSearchStats retrieves statistics about the search index
func (m *MockSearchRepository) GetSearchStats(ctx context.Context) (map[string]interface{}, error) {
	m.logger.Debug("Mock get search stats", nil)

	// Return mock stats
	return map[string]interface{}{
		"total_embeddings": 0,
		"total_models":     0,
		"status":           "transitioning to REST API",
	}, nil
}

// The following methods implement the standard Repository[SearchResult] interface

// Create stores a new search result (standardized Repository method)
func (m *MockSearchRepository) Create(ctx context.Context, result *repository.SearchResult) error {
	m.logger.Debug("Mock create search result", map[string]interface{}{
		"id": result.ID,
	})
	return nil
}

// Get retrieves a search result by its ID (standardized Repository method)
func (m *MockSearchRepository) Get(ctx context.Context, id string) (*repository.SearchResult, error) {
	m.logger.Debug("Mock get search result", map[string]interface{}{
		"id": id,
	})

	// Return a mock result
	return &repository.SearchResult{
		ID:      id,
		Score:   1.0,
		Content: "Mock content for " + id,
	}, nil
}

// List retrieves search results matching the provided filter (standardized Repository method)
func (m *MockSearchRepository) List(ctx context.Context, filter search.Filter) ([]*repository.SearchResult, error) {
	m.logger.Debug("Mock list search results", map[string]interface{}{
		"filter": filter,
	})

	// Return empty list for now
	return []*repository.SearchResult{}, nil
}

// Update modifies an existing search result (standardized Repository method)
func (m *MockSearchRepository) Update(ctx context.Context, result *repository.SearchResult) error {
	m.logger.Debug("Mock update search result", map[string]interface{}{
		"id": result.ID,
	})
	return nil
}

// Delete removes a search result by its ID (standardized Repository method)
func (m *MockSearchRepository) Delete(ctx context.Context, id string) error {
	m.logger.Debug("Mock delete search result", map[string]interface{}{
		"id": id,
	})
	return nil
}
