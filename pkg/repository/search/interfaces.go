// Package search provides interfaces and implementations for search operations
package search

import (
	"context"
)

// SearchOptions defines options for search operations
type SearchOptions struct {
	Limit         int
	Offset        int
	MinSimilarity float32
	Filters       []SearchFilter
	Sorts         []SearchSort
	ContentTypes  []string
	WeightFactors map[string]float32
}

// SearchFilter defines a filter for search operations
type SearchFilter struct {
	Field    string
	Operator string
	Value    interface{}
}

// SearchSort defines a sort order for search operations
type SearchSort struct {
	Field     string
	Direction string
}

// SearchResults contains results from a search operation
type SearchResults struct {
	Results []*SearchResult
	Total   int
	HasMore bool
}

// SearchResult represents a single result item from a search
type SearchResult struct {
	ID          string
	Score       float32
	Distance    float32
	Content     string
	Type        string
	Metadata    map[string]interface{}
	ContentHash string
}

// Repository defines the interface for search operations
type Repository interface {
	// SearchByText performs a vector search using text
	SearchByText(ctx context.Context, query string, options *SearchOptions) (*SearchResults, error)
	
	// SearchByVector performs a vector search using a pre-computed vector
	SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error)
	
	// SearchByContentID performs a "more like this" search
	SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error)
	
	// GetSupportedModels returns a list of models with embeddings
	GetSupportedModels(ctx context.Context) ([]string, error)
	
	// GetSearchStats retrieves statistics about the search index
	GetSearchStats(ctx context.Context) (map[string]interface{}, error)
}
