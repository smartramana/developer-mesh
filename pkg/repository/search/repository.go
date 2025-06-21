// Package search provides interfaces and implementations for search operations
package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// EmbeddingService defines the interface for generating embeddings
type EmbeddingService interface {
	// GenerateEmbedding generates an embedding for the given text
	GenerateEmbedding(ctx context.Context, text string, model string) ([]float32, error)
	// GenerateBatch generates embeddings for multiple texts
	GenerateBatch(ctx context.Context, texts []string, model string) ([][]float32, error)
}

// SQLRepository implements the Repository interface using a SQL database
type SQLRepository struct {
	db               *sqlx.DB
	embeddingService EmbeddingService
	defaultModel     string
}

// NewRepository creates a new search repository with the given database
func NewRepository(db *sqlx.DB) Repository {
	return &SQLRepository{
		db:           db,
		defaultModel: "amazon.titan-embed-text-v1", // Default Bedrock model
	}
}

// NewRepositoryWithEmbedding creates a new search repository with embedding service
func NewRepositoryWithEmbedding(db *sqlx.DB, embeddingService EmbeddingService, defaultModel string) Repository {
	return &SQLRepository{
		db:               db,
		embeddingService: embeddingService,
		defaultModel:     defaultModel,
	}
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
	var args []any
	argIndex := 1

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
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	if r.embeddingService == nil {
		return nil, fmt.Errorf("embedding service not configured: use NewRepositoryWithEmbedding to enable text search")
	}

	// Generate embedding for the query text
	model := r.defaultModel
	if options != nil && len(options.ContentTypes) > 0 {
		// ContentTypes can specify preferred models
		for _, ct := range options.ContentTypes {
			if strings.HasPrefix(ct, "model:") {
				model = strings.TrimPrefix(ct, "model:")
				break
			}
		}
	}

	// Generate embedding with timeout
	embedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	vector, err := r.embeddingService.GenerateEmbedding(embedCtx, query, model)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for query: %w", err)
	}

	// Use the vector search method
	return r.SearchByVector(ctx, vector, options)
}

// SearchByVector performs a vector search using a pre-computed vector
func (r *SQLRepository) SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Default options
	if options == nil {
		options = &SearchOptions{
			MaxResults:          10,
			SimilarityThreshold: 0.7,
			RankingAlgorithm:    "cosine",
		}
	}

	// Apply defaults for missing values
	if options.MaxResults == 0 && options.Limit == 0 {
		options.MaxResults = 10
	}
	// Use MaxResults if Limit not set (backward compatibility)
	if options.Limit == 0 {
		options.Limit = options.MaxResults
	}
	// Use SimilarityThreshold if MinSimilarity not set (backward compatibility)
	if options.MinSimilarity == 0 && options.SimilarityThreshold > 0 {
		options.MinSimilarity = options.SimilarityThreshold
	}
	if options.MinSimilarity == 0 {
		options.MinSimilarity = 0.7
	}
	if options.RankingAlgorithm == "" {
		options.RankingAlgorithm = "cosine"
	}

	// Select the appropriate distance operator based on ranking algorithm
	var distanceOp string
	switch options.RankingAlgorithm {
	case "euclidean":
		distanceOp = "<->" // L2 distance
	case "dot_product":
		distanceOp = "<#>" // Negative inner product
	case "cosine":
		fallthrough
	default:
		distanceOp = "<=>" // Cosine distance
	}

	// Build the base query
	query := fmt.Sprintf(`
		SELECT 
			id, 
			content_index,
			text as content,
			metadata,
			model_id as type,
			1 - (embedding %s $1::vector) as similarity
		FROM mcp.embeddings
		WHERE 1 - (embedding %s $1::vector) > $2`, distanceOp, distanceOp)

	args := []interface{}{vector, options.MinSimilarity}
	argIndex := 3

	// Add metadata filters if specified
	if len(options.MetadataFilters) > 0 {
		// Handle special exclude_id filter
		if excludeID, ok := options.MetadataFilters["exclude_id"]; ok {
			query += fmt.Sprintf(" AND id != $%d", argIndex)
			args = append(args, excludeID)
			argIndex++

			// Remove exclude_id from metadata filters
			filteredMeta := make(map[string]interface{})
			for k, v := range options.MetadataFilters {
				if k != "exclude_id" {
					filteredMeta[k] = v
				}
			}

			// Add remaining metadata filters if any
			if len(filteredMeta) > 0 {
				query += fmt.Sprintf(" AND metadata @> $%d::jsonb", argIndex)
				args = append(args, filteredMeta)
				argIndex++
			}
		} else {
			query += fmt.Sprintf(" AND metadata @> $%d::jsonb", argIndex)
			args = append(args, options.MetadataFilters)
			argIndex++
		}
	}

	// Add content type filters if specified
	if len(options.ContentTypes) > 0 {
		query += fmt.Sprintf(" AND model_id = ANY($%d::text[])", argIndex)
		args = append(args, options.ContentTypes)
		argIndex++
	}

	// Add hybrid search if requested (combine with full-text search)
	if options.HybridSearch && len(options.Filters) > 0 {
		for _, filter := range options.Filters {
			if filter.Field == "text" && filter.Operator == "contains" {
				query += fmt.Sprintf(" AND text ILIKE $%d", argIndex)
				args = append(args, fmt.Sprintf("%%%v%%", filter.Value))
				argIndex++
			}
		}
	}

	// Add ordering
	query += fmt.Sprintf(" ORDER BY embedding %s $1::vector", distanceOp)

	// Add limit and offset
	query += fmt.Sprintf(" LIMIT %d", options.Limit)
	if options.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", options.Offset)
	}

	// Execute query
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search query failed: %w", err)
	}
	defer rows.Close()

	// Process results
	results := []*SearchResult{}
	for rows.Next() {
		var result SearchResult
		var metadata sql.NullString

		err := rows.Scan(
			&result.ID,
			&result.Score, // Using content_index as score temporarily
			&result.Content,
			&metadata,
			&result.Type,
			&result.Distance, // This is actually similarity (1 - distance)
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		// Parse metadata if present
		if metadata.Valid && metadata.String != "" {
			result.Metadata = make(map[string]any)
			if err := json.Unmarshal([]byte(metadata.String), &result.Metadata); err != nil {
				// Log error but don't fail the entire search
				result.Metadata = map[string]any{"error": "failed to parse metadata"}
			}
		}

		// Convert similarity to score (higher is better)
		result.Score = result.Distance
		// Store actual distance
		result.Distance = 1 - result.Distance

		results = append(results, &result)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	// Check if there are more results
	hasMore := false
	if len(results) == options.Limit {
		// Quick check for one more result
		checkQuery := fmt.Sprintf(`
			SELECT 1 FROM mcp.embeddings 
			WHERE 1 - (embedding %s $1::vector) > $2 
			LIMIT 1 OFFSET %d`, distanceOp, options.Offset+options.Limit)

		var exists int
		err = r.db.QueryRowContext(ctx, checkQuery, vector, options.MinSimilarity).Scan(&exists)
		hasMore = err == nil
	}

	return &SearchResults{
		Results: results,
		Total:   len(results),
		HasMore: hasMore,
	}, nil
}

// SearchByContentID performs a "more like this" search
func (r *SQLRepository) SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Retrieve the embedding vector for the given content ID
	var vector []float32
	var model string

	query := `SELECT embedding, model_id FROM mcp.embeddings WHERE id = $1`

	row := r.db.QueryRowContext(ctx, query, contentID)
	err := row.Scan(&vector, &model)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("content not found: %s", contentID)
		}
		return nil, fmt.Errorf("failed to retrieve content embedding: %w", err)
	}

	// Ensure we exclude the source content from results
	if options == nil {
		options = &SearchOptions{}
	}

	// Add a filter to exclude the source content
	if options.MetadataFilters == nil {
		options.MetadataFilters = make(map[string]interface{})
	}
	options.MetadataFilters["exclude_id"] = contentID

	// Perform vector search with the retrieved embedding
	results, err := r.SearchByVector(ctx, vector, options)
	if err != nil {
		return nil, fmt.Errorf("failed to perform similarity search: %w", err)
	}

	// Filter out the source content if it somehow appears in results
	filteredResults := make([]*SearchResult, 0, len(results.Results))
	for _, result := range results.Results {
		if result.ID != contentID {
			filteredResults = append(filteredResults, result)
		}
	}
	results.Results = filteredResults
	results.Total = len(filteredResults)

	return results, nil
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
func (r *SQLRepository) GetSearchStats(ctx context.Context) (map[string]any, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	stats := make(map[string]any)

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
