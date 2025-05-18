// Package types provides shared types for the REST API internal packages
package types

// Embedding represents a vector embedding in the REST API
type Embedding struct {
	ID           string
	ContextID    string
	ContentIndex int
	Text         string
	Embedding    []float32
	ModelID      string
	Metadata     map[string]interface{}
}

// SearchOptions defines options for search operations
type SearchOptions struct {
	Limit               int
	Offset              int
	SimilarityThreshold float64
	Filters             map[string]interface{}
	Sort                string
	SortDirection       string
}

// SearchResult represents a single search result
type SearchResult struct {
	ID         string
	Content    string
	Similarity float64
	Metadata   map[string]interface{}
}

// SearchResults represents a collection of search results
type SearchResults struct {
	Results []*SearchResult
	Total   int
	HasMore bool
}
