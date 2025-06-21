package embedding

import (
	"context"
)

// SearchFilter defines a filter for metadata fields
type SearchFilter struct {
	// Field is the metadata field to filter on
	Field string `json:"field"`
	// Value is the value to match
	Value interface{} `json:"value"`
	// Operator is the comparison operator (eq, ne, gt, lt, gte, lte, in, contains)
	Operator string `json:"operator"`
}

// SearchSort defines a sort order for results
type SearchSort struct {
	// Field is the field to sort on (can be "similarity" or any metadata field)
	Field string `json:"field"`
	// Direction is the sort direction ("asc" or "desc")
	Direction string `json:"direction"`
}

// SearchOptions contains options for search queries
type SearchOptions struct {
	// ContentTypes filters results to specific content types
	ContentTypes []string `json:"content_types,omitempty"`
	// Filters are metadata filters to apply to the search
	Filters []SearchFilter `json:"filters,omitempty"`
	// Sorts defines the sort order for results
	Sorts []SearchSort `json:"sorts,omitempty"`
	// Limit is the maximum number of results to return
	Limit int `json:"limit"`
	// Offset is the number of results to skip (for pagination)
	Offset int `json:"offset"`
	// MinSimilarity is the minimum similarity score required (0.0 to 1.0)
	MinSimilarity float32 `json:"min_similarity"`
	// WeightFactors defines how to weight different scoring factors
	WeightFactors map[string]float32 `json:"weight_factors,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	// Content is the embedding that matched
	Content *EmbeddingVector `json:"content"`
	// Score is the calculated relevance score (0.0 to 1.0)
	Score float32 `json:"score"`
	// Matches contains information about why this result matched
	Matches map[string]interface{} `json:"matches,omitempty"`
}

// SearchResults represents a collection of search results
type SearchResults struct {
	// Results is the list of search results
	Results []*SearchResult `json:"results"`
	// Total is the total number of results found (for pagination)
	Total int `json:"total"`
	// HasMore indicates if there are more results available
	HasMore bool `json:"has_more"`
}

// SearchService defines the interface for vector search operations
type SearchService interface {
	// Search performs a vector search with the given text
	Search(ctx context.Context, text string, options *SearchOptions) (*SearchResults, error)

	// SearchByVector performs a vector search with a pre-computed vector
	SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error)

	// SearchByContentID performs a "more like this" search based on an existing content ID
	SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error)
}

// AdvancedSearchService extends SearchService with cross-model and hybrid search capabilities
type AdvancedSearchService interface {
	SearchService
	
	// CrossModelSearch performs search across embeddings from different models
	CrossModelSearch(ctx context.Context, req CrossModelSearchRequest) ([]CrossModelSearchResult, error)
	
	// HybridSearch performs hybrid search combining semantic and keyword search
	HybridSearch(ctx context.Context, req HybridSearchRequest) ([]HybridSearchResult, error)
}
