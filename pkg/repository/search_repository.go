package repository

import (
	"context"
)

// SearchFilter represents a filter for search queries
type SearchFilter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// SearchSort represents a sort order for search results
type SearchSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// SearchOptions represents options for search queries
type SearchOptions struct {
	ContentTypes  []string             `json:"content_types,omitempty"`
	Filters       []SearchFilter       `json:"filters,omitempty"`
	Sorts         []SearchSort         `json:"sorts,omitempty"`
	Limit         int                  `json:"limit,omitempty"`
	Offset        int                  `json:"offset,omitempty"`
	MinSimilarity float32              `json:"min_similarity,omitempty"`
	WeightFactors map[string]float32   `json:"weight_factors,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID          string                 `json:"id"`
	Score       float32                `json:"score"`
	Distance    float32                `json:"distance"`
	Content     string                 `json:"content"`
	Type        string                 `json:"type"`
	Metadata    map[string]interface{} `json:"metadata"`
	ContentHash string                 `json:"content_hash"`
}

// SearchResults represents the results of a search query
type SearchResults struct {
	Results []*SearchResult `json:"results"`
	Total   int             `json:"total"`
	HasMore bool            `json:"has_more"`
}

// SearchRepository defines methods for performing search operations
type SearchRepository interface {
	// SearchByText performs a vector search using a text query
	SearchByText(ctx context.Context, query string, options *SearchOptions) (*SearchResults, error)
	
	// SearchByVector performs a vector search using a pre-computed vector
	SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error)
	
	// SearchByContentID performs a "more like this" search using an existing content ID
	SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error)
	
	// GetSupportedModels retrieves a list of all models with embeddings
	GetSupportedModels(ctx context.Context) ([]string, error)
	
	// GetSearchStats retrieves statistics about the search index
	GetSearchStats(ctx context.Context) (map[string]interface{}, error)
}
