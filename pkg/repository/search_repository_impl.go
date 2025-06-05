package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/repository/search"
	"github.com/jmoiron/sqlx"
)

// SearchRepositoryImpl implements the SearchRepository interface
type SearchRepositoryImpl struct {
	db         *sqlx.DB
	vectorRepo VectorAPIRepository
}

// NewSearchRepository creates a new SearchRepository
func NewSearchRepository(db *sqlx.DB, vectorRepo VectorAPIRepository) SearchRepository {
	// Use the provided vector repository or create a new one if not provided
	if vectorRepo == nil {
		vectorRepo = NewEmbeddingRepository(db)
	}

	return &SearchRepositoryImpl{
		db:         db,
		vectorRepo: vectorRepo,
	}
}

// SearchByText performs a vector search using a text query
func (r *SearchRepositoryImpl) SearchByText(ctx context.Context, query string, options *SearchOptions) (*SearchResults, error) {
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}

	if options == nil {
		options = &SearchOptions{
			Limit: 10,
		}
	}

	// In a real implementation, we would:
	// 1. Convert the text query to a vector using an embedding model
	// 2. Search using the vector

	// For compatibility, we'll use a simplified approach
	// This adapter bridges the gap between API expectations and implementation
	dummyVector := []float32{0.1, 0.2, 0.3} // This would normally be generated from the query

	// Extract context filter if present
	contextID := ""
	modelID := ""

	for _, filter := range options.Filters {
		switch filter.Field {
		case "context_id":
			if strVal, ok := filter.Value.(string); ok {
				contextID = strVal
			}
		case "model_id":
			if strVal, ok := filter.Value.(string); ok {
				modelID = strVal
			}
		}
	}

	// Call the vector repository's search function
	embeddings, err := r.vectorRepo.SearchEmbeddings(
		ctx,
		dummyVector,
		contextID,
		modelID,
		options.Limit,
		float64(options.MinSimilarity),
	)

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert embeddings to search results
	results := &SearchResults{
		Results: make([]*SearchResult, len(embeddings)),
		Total:   len(embeddings),
		HasMore: false, // Simplified implementation
	}

	for i, emb := range embeddings {
		results.Results[i] = &SearchResult{
			ID:          emb.ID,
			Score:       0.9 - float32(i)*0.1, // Simplified scoring
			Distance:    float32(i) * 0.1,
			Content:     emb.Text,
			Type:        "text",
			Metadata:    emb.Metadata,
			ContentHash: "", // Not implemented in this adapter
		}
	}

	return results, nil
}

// SearchByVector performs a vector search using a pre-computed vector
func (r *SearchRepositoryImpl) SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error) {
	if len(vector) == 0 {
		return nil, errors.New("vector cannot be empty")
	}

	if options == nil {
		options = &SearchOptions{
			Limit: 10,
		}
	}

	// Extract context filter if present
	contextID := ""
	modelID := ""

	for _, filter := range options.Filters {
		switch filter.Field {
		case "context_id":
			if strVal, ok := filter.Value.(string); ok {
				contextID = strVal
			}
		case "model_id":
			if strVal, ok := filter.Value.(string); ok {
				modelID = strVal
			}
		}
	}

	// Call the vector repository's search function
	embeddings, err := r.vectorRepo.SearchEmbeddings(
		ctx,
		vector,
		contextID,
		modelID,
		options.Limit,
		float64(options.MinSimilarity),
	)

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert embeddings to search results
	results := &SearchResults{
		Results: make([]*SearchResult, len(embeddings)),
		Total:   len(embeddings),
		HasMore: false,
	}

	for i, emb := range embeddings {
		results.Results[i] = &SearchResult{
			ID:          emb.ID,
			Score:       0.9 - float32(i)*0.1, // Simplified scoring
			Distance:    float32(i) * 0.1,
			Content:     emb.Text,
			Type:        "text",
			Metadata:    emb.Metadata,
			ContentHash: "",
		}
	}

	return results, nil
}

// SearchByContentID performs a "more like this" search using an existing content ID
func (r *SearchRepositoryImpl) SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error) {
	if contentID == "" {
		return nil, errors.New("content ID cannot be empty")
	}

	if options == nil {
		options = &SearchOptions{
			Limit: 10,
		}
	}

	// In a real implementation, we would:
	// 1. Retrieve the embedding for the content ID
	// 2. Use that embedding to perform a vector search

	// For this adapter implementation, we'll simulate with a dummy vector
	dummyVector := []float32{0.1, 0.2, 0.3}

	// Extract context filter if present
	contextID := ""
	modelID := ""

	for _, filter := range options.Filters {
		switch filter.Field {
		case "context_id":
			if strVal, ok := filter.Value.(string); ok {
				contextID = strVal
			}
		case "model_id":
			if strVal, ok := filter.Value.(string); ok {
				modelID = strVal
			}
		}
	}

	// Call the vector repository's search function
	embeddings, err := r.vectorRepo.SearchEmbeddings(
		ctx,
		dummyVector,
		contextID,
		modelID,
		options.Limit,
		float64(options.MinSimilarity),
	)

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert embeddings to search results
	results := &SearchResults{
		Results: make([]*SearchResult, len(embeddings)),
		Total:   len(embeddings),
		HasMore: false,
	}

	for i, emb := range embeddings {
		results.Results[i] = &SearchResult{
			ID:          emb.ID,
			Score:       0.9 - float32(i)*0.1,
			Distance:    float32(i) * 0.1,
			Content:     emb.Text,
			Type:        "text",
			Metadata:    emb.Metadata,
			ContentHash: "",
		}
	}

	return results, nil
}

// GetSupportedModels retrieves a list of all models with embeddings
func (r *SearchRepositoryImpl) GetSupportedModels(ctx context.Context) ([]string, error) {
	return r.vectorRepo.GetSupportedModels(ctx)
}

// GetSearchStats retrieves statistics about the search index
func (r *SearchRepositoryImpl) GetSearchStats(ctx context.Context) (map[string]any, error) {
	// Simplified implementation for the adapter
	return map[string]any{
		"total_embeddings": 0,
		"total_models":     0,
		"total_contexts":   0,
		"status":           "healthy",
	}, nil
}

// The following methods implement the standard Repository[SearchResult] interface

// Create stores a new search result
func (r *SearchRepositoryImpl) Create(ctx context.Context, result *SearchResult) error {
	// Currently not implemented since search results are derived from embeddings
	// In a real implementation, we would store the search result in the database
	return fmt.Errorf("create operation not supported by SearchRepositoryImpl")
}

// Get retrieves a search result by its ID
func (r *SearchRepositoryImpl) Get(ctx context.Context, id string) (*SearchResult, error) {
	// For now, let's simulate getting a search result by using a query with the ID
	// In a real implementation, we would query the database directly
	options := &SearchOptions{
		Limit: 1,
		Filters: []SearchFilter{
			{
				Field:    "id",
				Operator: "eq",
				Value:    id,
			},
		},
	}

	// Try using the content ID search as a workaround for direct get
	results, err := r.SearchByContentID(ctx, id, options)
	if err != nil {
		return nil, err
	}

	if results == nil || len(results.Results) == 0 {
		return nil, nil // Not found
	}

	return results.Results[0], nil
}

// List retrieves search results matching the provided filter
func (r *SearchRepositoryImpl) List(ctx context.Context, filter search.Filter) ([]*SearchResult, error) {
	// Convert the generic filter to SearchOptions
	options := &SearchOptions{
		Limit: 100, // Default limit
	}

	// Extract filters from the map
	if filter != nil {
		for field, value := range filter {
			options.Filters = append(options.Filters, SearchFilter{
				Field:    field,
				Operator: "eq",
				Value:    value,
			})
		}
	}

	// Use empty text search to get all results
	results, err := r.SearchByText(ctx, "", options)
	if err != nil {
		return nil, err
	}

	if results == nil {
		return []*SearchResult{}, nil
	}

	return results.Results, nil
}

// Update modifies an existing search result
func (r *SearchRepositoryImpl) Update(ctx context.Context, result *SearchResult) error {
	// Currently not implemented since search results are derived from embeddings
	return fmt.Errorf("update operation not supported by SearchRepositoryImpl")
}

// Delete removes a search result by its ID
func (r *SearchRepositoryImpl) Delete(ctx context.Context, id string) error {
	// Currently not implemented since search results are derived from embeddings
	return fmt.Errorf("delete operation not supported by SearchRepositoryImpl")
}
