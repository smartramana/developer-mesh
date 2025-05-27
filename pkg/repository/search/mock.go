// Package search provides interfaces and implementations for search operations
package search

import (
	"context"
	"fmt"
	"strings"
)

// MockRepository provides an in-memory implementation of the Repository interface for testing
type MockRepository struct {
	documents       map[string]*SearchResult
	supportedModels []string
}

// NewMockRepository creates a new mock repository
func NewMockRepository() Repository {
	return &MockRepository{
		documents:       make(map[string]*SearchResult),
		supportedModels: []string{"text-embedding-3-small", "text-embedding-3-large"},
	}
}

// Create stores a new search result (standardized Repository method)
func (m *MockRepository) Create(ctx context.Context, result *SearchResult) error {
	// Store the result in our mock storage
	m.documents[result.ID] = result
	return nil
}

// Get retrieves a search result by its ID (standardized Repository method)
func (m *MockRepository) Get(ctx context.Context, id string) (*SearchResult, error) {
	result, exists := m.documents[id]
	if !exists {
		return nil, nil // Not found
	}

	// Return a copy to avoid modifying the stored version
	copy := *result
	return &copy, nil
}

// List retrieves search results matching the provided filter (standardized Repository method)
func (m *MockRepository) List(ctx context.Context, filter Filter) ([]*SearchResult, error) {
	var results []*SearchResult

	for _, doc := range m.documents {
		match := true

		if filter != nil {
			for k, v := range filter {
				switch k {
				case "type":
					if doc.Type != v.(string) {
						match = false
					}
				case "content_hash":
					if doc.ContentHash != v.(string) {
						match = false
					}
					// Additional filter fields can be added here
				}
			}
		}

		if match {
			// Clone the doc to avoid modifying the stored version
			copy := *doc
			results = append(results, &copy)
		}
	}

	return results, nil
}

// Update modifies an existing search result (standardized Repository method)
func (m *MockRepository) Update(ctx context.Context, result *SearchResult) error {
	_, exists := m.documents[result.ID]
	if !exists {
		return fmt.Errorf("search result with id %s not found", result.ID)
	}

	// Store the updated result
	m.documents[result.ID] = result
	return nil
}

// Delete removes a search result by its ID (standardized Repository method)
func (m *MockRepository) Delete(ctx context.Context, id string) error {
	_, exists := m.documents[id]
	if !exists {
		return fmt.Errorf("search result with id %s not found", id)
	}

	delete(m.documents, id)
	return nil
}

// AddDocument adds a document to the mock repository
func (m *MockRepository) AddDocument(id string, content string, docType string, metadata map[string]interface{}) {
	m.documents[id] = &SearchResult{
		ID:          id,
		Score:       1.0,
		Distance:    0.0,
		Content:     content,
		Type:        docType,
		Metadata:    metadata,
		ContentHash: fmt.Sprintf("hash-%s", id), // Mock hash
	}
}

// SearchByText performs a vector search using text
func (m *MockRepository) SearchByText(ctx context.Context, query string, options *SearchOptions) (*SearchResults, error) {
	if options == nil {
		options = &SearchOptions{
			Limit:  10,
			Offset: 0,
		}
	}

	var results []*SearchResult
	query = strings.ToLower(query)

	// Simple mock implementation that looks for substrings
	for _, doc := range m.documents {
		if strings.Contains(strings.ToLower(doc.Content), query) {
			// Clone the doc to avoid modifying the stored version
			result := *doc
			result.Score = 0.95 // Mock score
			results = append(results, &result)
		}

		if len(results) >= options.Limit {
			break
		}
	}

	return &SearchResults{
		Results: results,
		Total:   len(results),
		HasMore: false,
	}, nil
}

// SearchByVector performs a vector search using a pre-computed vector
func (m *MockRepository) SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error) {
	// Return all documents sorted by ID since we can't do real vector search in the mock
	if options == nil {
		options = &SearchOptions{
			Limit:  10,
			Offset: 0,
		}
	}

	var results []*SearchResult
	count := 0

	for _, doc := range m.documents {
		if count >= options.Offset && len(results) < options.Limit {
			// Clone the doc to avoid modifying the stored version
			result := *doc
			result.Score = 0.90 - (float32(count) * 0.05) // Mock decreasing scores
			results = append(results, &result)
		}
		count++
	}

	return &SearchResults{
		Results: results,
		Total:   len(m.documents),
		HasMore: len(m.documents) > options.Offset+options.Limit,
	}, nil
}

// SearchByContentID performs a "more like this" search
func (m *MockRepository) SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error) {
	doc, exists := m.documents[contentID]
	if !exists {
		return &SearchResults{
			Results: []*SearchResult{},
			Total:   0,
			HasMore: false,
		}, nil
	}

	// For mock, we'll just search by text using the content
	return m.SearchByText(ctx, doc.Content, options)
}

// GetSupportedModels returns a list of models with embeddings
func (m *MockRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	return m.supportedModels, nil
}

// GetSearchStats retrieves statistics about the search index
func (m *MockRepository) GetSearchStats(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"document_count": len(m.documents),
		"models":         m.supportedModels,
		"is_mock":        true,
	}, nil
}
