package proxies

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
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
