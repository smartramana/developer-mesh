// Package search provides interfaces and implementations for search operations
package search

import (
	"context"
	"database/sql"
	"fmt"
	
	"github.com/jmoiron/sqlx"
)

// SQLRepository implements the Repository interface using a SQL database
type SQLRepository struct {
	db *sqlx.DB
}

// NewRepository creates a new search repository with the given database
func NewRepository(db *sqlx.DB) Repository {
	return &SQLRepository{db: db}
}

// Create stores a new search result (standardized Repository method)
func (r *SQLRepository) Create(ctx context.Context, result *SearchResult) error {
	if r.db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `INSERT INTO search_results (id, score, distance, content, type, metadata, content_hash) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(ctx, query, 
		result.ID,
		result.Score,
		result.Distance,
		result.Content,
		result.Type,
		result.Metadata,
		result.ContentHash)

	if err != nil {
		return fmt.Errorf("failed to create search result: %w", err)
	}

	return nil
}

// Get retrieves a search result by its ID (standardized Repository method)
func (r *SQLRepository) Get(ctx context.Context, id string) (*SearchResult, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `SELECT id, score, distance, content, type, metadata, content_hash
	          FROM search_results WHERE id = $1`

	var result SearchResult
	err := r.db.GetContext(ctx, &result, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get search result: %w", err)
	}

	return &result, nil
}

// List retrieves search results matching the provided filter (standardized Repository method)
func (r *SQLRepository) List(ctx context.Context, filter Filter) ([]*SearchResult, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `SELECT id, score, distance, content, type, metadata, content_hash FROM search_results`
	
	// Apply filters
	var whereClause string
	var args []interface{}
	argIndex := 1

	if filter != nil {
		for k, v := range filter {
			if whereClause == "" {
				whereClause = " WHERE "
			} else {
				whereClause += " AND "
			}
			whereClause += fmt.Sprintf("%s = $%d", k, argIndex)
			args = append(args, v)
			argIndex++
		}
	}

	query += whereClause + " ORDER BY score DESC"

	var results []*SearchResult
	err := r.db.SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list search results: %w", err)
	}

	return results, nil
}

// Update modifies an existing search result (standardized Repository method)
func (r *SQLRepository) Update(ctx context.Context, result *SearchResult) error {
	if r.db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `UPDATE search_results SET 
	          score = $2, distance = $3, content = $4, type = $5, metadata = $6, content_hash = $7
	          WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, 
		result.ID,
		result.Score,
		result.Distance,
		result.Content,
		result.Type,
		result.Metadata,
		result.ContentHash)

	if err != nil {
		return fmt.Errorf("failed to update search result: %w", err)
	}

	return nil
}

// Delete removes a search result by its ID (standardized Repository method)
func (r *SQLRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM search_results WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete search result: %w", err)
	}

	return nil
}

// SearchByText performs a vector search using text
func (r *SQLRepository) SearchByText(ctx context.Context, query string, options *SearchOptions) (*SearchResults, error) {
	// In a real implementation, this would:
	// 1. Convert query text to vector using an embedding service
	// 2. Call SearchByVector with resulting embedding
	
	// For now, return empty results since this requires integration with vector service
	return &SearchResults{
		Results: []*SearchResult{},
		Total:   0,
		HasMore: false,
	}, fmt.Errorf("SearchByText not yet implemented")
}

// SearchByVector performs a vector search using a pre-computed vector
func (r *SQLRepository) SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	
	if options == nil {
		options = &SearchOptions{
			Limit:         10,
			Offset:        0,
			MinSimilarity: 0.7,
		}
	}
	
	// Build query with placeholder parameters for filters
	// In a real implementation, this would:
	// 1. Use vector similarity search (e.g., pgvector)
	// 2. Apply filters based on options
	// 3. Apply sorting

	// For now, return empty results until vector DB is implemented
	return &SearchResults{
		Results: []*SearchResult{},
		Total:   0,
		HasMore: false,
	}, fmt.Errorf("vector search not yet implemented")
}

// SearchByContentID performs a "more like this" search
func (r *SQLRepository) SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	
	// In a real implementation, this would:
	// 1. Retrieve the vector for the contentID
	// 2. Call SearchByVector with the retrieved vector

	// For now, return empty results until vector DB is implemented
	return &SearchResults{
		Results: []*SearchResult{},
		Total:   0,
		HasMore: false,
	}, fmt.Errorf("more-like-this search not yet implemented")
}

// GetSupportedModels returns a list of models with embeddings
func (r *SQLRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	
	var models []string
	query := `SELECT DISTINCT model_id FROM embeddings WHERE model_id IS NOT NULL`
	
	err := r.db.SelectContext(ctx, &models, query)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("error getting supported models: %w", err)
	}
	
	return models, nil
}

// GetSearchStats retrieves statistics about the search index
func (r *SQLRepository) GetSearchStats(ctx context.Context) (map[string]interface{}, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	
	stats := make(map[string]interface{})
	
	// Get document count
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM embeddings")
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("error getting embedding count: %w", err)
	}
	stats["document_count"] = count
	
	// Get models
	models, err := r.GetSupportedModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting supported models: %w", err)
	}
	stats["models"] = models
	
	return stats, nil
}
