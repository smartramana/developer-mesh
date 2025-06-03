package embedding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// PgSearchService implements the SearchService interface using PostgreSQL with pgvector
type PgSearchService struct {
	// Database connection
	db *sql.DB
	// Schema name
	schema string
	// Embedding service to generate query vectors
	embeddingService EmbeddingService
	// Default search options
	defaultOptions *SearchOptions
}

// PgSearchConfig contains configuration for the PostgreSQL search service
type PgSearchConfig struct {
	// DB is the database connection
	DB *sql.DB
	// Schema is the PostgreSQL schema name
	Schema string
	// EmbeddingService is used to generate vectors for text queries
	EmbeddingService EmbeddingService
	// DefaultLimit is the default number of results to return
	DefaultLimit int
	// DefaultMinSimilarity is the default minimum similarity threshold
	DefaultMinSimilarity float32
}

// NewPgSearchService creates a new PostgreSQL search service
func NewPgSearchService(config *PgSearchConfig) (*PgSearchService, error) {
	if config.DB == nil {
		return nil, errors.New("database connection is required")
	}

	if config.EmbeddingService == nil {
		return nil, errors.New("embedding service is required")
	}

	if config.Schema == "" {
		config.Schema = "mcp" // Default schema
	}

	if config.DefaultLimit <= 0 {
		config.DefaultLimit = 10 // Default limit
	}

	if config.DefaultMinSimilarity < 0 || config.DefaultMinSimilarity > 1 {
		config.DefaultMinSimilarity = 0.7 // Default minimum similarity
	}

	// Verify pgvector extension is available
	var exists bool
	err := config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check pgvector extension: %w", err)
	}

	if !exists {
		return nil, errors.New("pgvector extension is not installed in the database")
	}

	defaultOptions := &SearchOptions{
		Limit:         config.DefaultLimit,
		MinSimilarity: config.DefaultMinSimilarity,
		WeightFactors: map[string]float32{
			"similarity": 1.0, // Default to full weight on vector similarity
		},
	}

	return &PgSearchService{
		db:               config.DB,
		schema:           config.Schema,
		embeddingService: config.EmbeddingService,
		defaultOptions:   defaultOptions,
	}, nil
}

// Search performs a vector search with the given text
func (s *PgSearchService) Search(ctx context.Context, text string, options *SearchOptions) (*SearchResults, error) {
	if text == "" {
		return nil, errors.New("search text cannot be empty")
	}

	// Generate vector for the search text
	embedding, err := s.embeddingService.GenerateEmbedding(ctx, text, "search_query", "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for search text: %w", err)
	}

	// Search with the generated vector
	return s.SearchByVector(ctx, embedding.Vector, options)
}

// SearchByVector performs a vector search with a pre-computed vector
func (s *PgSearchService) SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error) {
	if len(vector) == 0 {
		return nil, errors.New("search vector cannot be empty")
	}

	// Merge with default options
	opts := s.mergeWithDefaultOptions(options)

	// Build the query
	query, args, err := s.buildSearchQuery(vector, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to build search query: %w", err)
	}

	// Execute the query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// PG search - log but don't fail
			_ = err
		}
	}()

	// Process the results
	return s.processSearchResults(rows, opts)
}

// SearchByContentID performs a "more like this" search based on an existing content ID
func (s *PgSearchService) SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error) {
	if contentID == "" {
		return nil, errors.New("content ID cannot be empty")
	}

	// Get the embedding for the content ID
	query := fmt.Sprintf(`
		SELECT embedding::text 
		FROM %s.embeddings 
		WHERE id = $1
	`, s.schema)

	var embeddingStr string
	err := s.db.QueryRowContext(ctx, query, contentID).Scan(&embeddingStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("content ID not found: %s", contentID)
		}
		return nil, fmt.Errorf("failed to get embedding for content ID: %w", err)
	}

	// Parse the vector
	vector, err := parseVectorFromPg(embeddingStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vector: %w", err)
	}

	// Search with the retrieved vector
	return s.SearchByVector(ctx, vector, options)
}

// Helper methods

// mergeWithDefaultOptions merges provided options with defaults
func (s *PgSearchService) mergeWithDefaultOptions(options *SearchOptions) *SearchOptions {
	if options == nil {
		return s.defaultOptions
	}

	// Create a new options struct
	merged := &SearchOptions{
		ContentTypes:  options.ContentTypes,
		Filters:       options.Filters,
		Sorts:         options.Sorts,
		Offset:        options.Offset,
		WeightFactors: make(map[string]float32),
	}

	// Set limit
	if options.Limit <= 0 {
		merged.Limit = s.defaultOptions.Limit
	} else {
		merged.Limit = options.Limit
	}

	// Set minimum similarity
	if options.MinSimilarity <= 0 || options.MinSimilarity > 1 {
		merged.MinSimilarity = s.defaultOptions.MinSimilarity
	} else {
		merged.MinSimilarity = options.MinSimilarity
	}

	// Merge weight factors
	for k, v := range s.defaultOptions.WeightFactors {
		merged.WeightFactors[k] = v
	}
	for k, v := range options.WeightFactors {
		merged.WeightFactors[k] = v
	}

	return merged
}

// buildSearchQuery builds the SQL query for the search
func (s *PgSearchService) buildSearchQuery(vector []float32, options *SearchOptions) (string, []interface{}, error) {
	// Format vector for pgvector
	vectorStr := formatVectorForPg(vector)

	// Base query with vector similarity calculation
	baseQuery := fmt.Sprintf(`
		SELECT 
			id, 
			context_id, 
			content_index, 
			text,
			embedding::text, 
			vector_dimensions, 
			model_id,
			metadata, 
			content_type,
			1 - (embedding <=> $1::vector) AS similarity
		FROM %s.embeddings
		WHERE 1 = 1
	`, s.schema)

	var conditions []string
	var args []interface{}
	args = append(args, vectorStr) // $1 is the search vector

	// Add minimum similarity condition
	conditions = append(conditions, fmt.Sprintf("(1 - (embedding <=> $1::vector)) >= $%d", len(args)+1))
	args = append(args, options.MinSimilarity)

	// Add content type filter if specified
	if len(options.ContentTypes) > 0 {
		conditions = append(conditions, fmt.Sprintf("content_type = ANY($%d)", len(args)+1))
		args = append(args, pq.Array(options.ContentTypes))
	}

	// Add metadata filters
	if len(options.Filters) > 0 {
		for _, filter := range options.Filters {
			condition, filterArgs, err := s.buildFilterCondition(filter, len(args)+1)
			if err != nil {
				return "", nil, err
			}
			conditions = append(conditions, condition)
			args = append(args, filterArgs...)
		}
	}

	// Build WHERE clause
	whereClause := strings.Join(conditions, " AND ")
	query := baseQuery
	if whereClause != "" {
		query += " AND " + whereClause
	}

	// Add ORDER BY clause
	if len(options.Sorts) > 0 {
		var sortClauses []string
		for _, sort := range options.Sorts {
			direction := "ASC"
			if strings.ToLower(sort.Direction) == "desc" {
				direction = "DESC"
			}

			// Handle special case for similarity (computed column)
			if sort.Field == "similarity" {
				sortClauses = append(sortClauses, fmt.Sprintf("similarity %s", direction))
			} else {
				// For metadata fields, we need to use the -> operator
				if strings.HasPrefix(sort.Field, "metadata.") {
					field := strings.TrimPrefix(sort.Field, "metadata.")
					sortClauses = append(sortClauses, fmt.Sprintf("metadata->>'%s' %s", field, direction))
				} else {
					sortClauses = append(sortClauses, fmt.Sprintf("%s %s", sort.Field, direction))
				}
			}
		}
		query += " ORDER BY " + strings.Join(sortClauses, ", ")
	} else {
		// Default sort by similarity
		query += " ORDER BY similarity DESC"
	}

	// Add LIMIT and OFFSET
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, options.Limit, options.Offset)

	return query, args, nil
}

// buildFilterCondition builds a SQL condition for a metadata filter
func (s *PgSearchService) buildFilterCondition(filter SearchFilter, startArgIndex int) (string, []interface{}, error) {
	var condition string
	var args []interface{}

	// Check if this is a metadata field or a direct column
	if strings.HasPrefix(filter.Field, "metadata.") {
		// Extract the metadata field name
		fieldName := strings.TrimPrefix(filter.Field, "metadata.")

		switch strings.ToLower(filter.Operator) {
		case "eq", "=", "==":
			condition = fmt.Sprintf("metadata->>'%s' = $%d", fieldName, startArgIndex)
			args = append(args, fmt.Sprintf("%v", filter.Value))
		case "ne", "!=", "<>":
			condition = fmt.Sprintf("metadata->>'%s' <> $%d", fieldName, startArgIndex)
			args = append(args, fmt.Sprintf("%v", filter.Value))
		case "gt", ">":
			condition = fmt.Sprintf("(metadata->>'%s')::float > $%d", fieldName, startArgIndex)
			args = append(args, filter.Value)
		case "lt", "<":
			condition = fmt.Sprintf("(metadata->>'%s')::float < $%d", fieldName, startArgIndex)
			args = append(args, filter.Value)
		case "gte", ">=":
			condition = fmt.Sprintf("(metadata->>'%s')::float >= $%d", fieldName, startArgIndex)
			args = append(args, filter.Value)
		case "lte", "<=":
			condition = fmt.Sprintf("(metadata->>'%s')::float <= $%d", fieldName, startArgIndex)
			args = append(args, filter.Value)
		case "in":
			// For IN operator, value must be a slice
			values, ok := filter.Value.([]interface{})
			if !ok {
				return "", nil, fmt.Errorf("value for 'in' operator must be an array")
			}
			valuePlaceholders := make([]string, len(values))
			for i, v := range values {
				valuePlaceholders[i] = fmt.Sprintf("$%d", startArgIndex+i)
				args = append(args, fmt.Sprintf("%v", v))
			}
			condition = fmt.Sprintf("metadata->>'%s' IN (%s)", fieldName, strings.Join(valuePlaceholders, ", "))
		case "contains":
			condition = fmt.Sprintf("metadata->>'%s' LIKE '%%' || $%d || '%%'", fieldName, startArgIndex)
			args = append(args, fmt.Sprintf("%v", filter.Value))
		default:
			return "", nil, fmt.Errorf("unsupported operator: %s", filter.Operator)
		}
	} else {
		// Direct column filtering
		switch strings.ToLower(filter.Operator) {
		case "eq", "=", "==":
			condition = fmt.Sprintf("%s = $%d", filter.Field, startArgIndex)
			args = append(args, filter.Value)
		case "ne", "!=", "<>":
			condition = fmt.Sprintf("%s <> $%d", filter.Field, startArgIndex)
			args = append(args, filter.Value)
		case "in":
			// For IN operator, value must be a slice
			values, ok := filter.Value.([]interface{})
			if !ok {
				return "", nil, fmt.Errorf("value for 'in' operator must be an array")
			}
			valuePlaceholders := make([]string, len(values))
			for i, v := range values {
				valuePlaceholders[i] = fmt.Sprintf("$%d", startArgIndex+i)
				args = append(args, v)
			}
			condition = fmt.Sprintf("%s IN (%s)", filter.Field, strings.Join(valuePlaceholders, ", "))
		default:
			// Other operators may not make sense for non-metadata fields
			return "", nil, fmt.Errorf("unsupported operator for direct column: %s", filter.Operator)
		}
	}

	return condition, args, nil
}

// processSearchResults processes the search results from the database query
func (s *PgSearchService) processSearchResults(rows *sql.Rows, options *SearchOptions) (*SearchResults, error) {
	results := &SearchResults{
		Results: []*SearchResult{},
		Total:   0,
		HasMore: false,
	}

	for rows.Next() {
		var (
			id           string
			contextID    sql.NullString
			contentIndex int
			text         sql.NullString
			embeddingStr string
			dimensions   int
			modelID      string
			metadataJSON sql.NullString
			contentType  string
			similarity   float32
		)

		if err := rows.Scan(
			&id,
			&contextID,
			&contentIndex,
			&text,
			&embeddingStr,
			&dimensions,
			&modelID,
			&metadataJSON,
			&contentType,
			&similarity,
		); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		// Extract content ID from compound ID
		parts := strings.SplitN(id, ":", 2)
		contentID := ""
		if len(parts) > 1 {
			contentID = parts[1]
		} else {
			contentID = id
		}

		// Parse the vector
		vector, err := parseVectorFromPg(embeddingStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse vector: %w", err)
		}

		// Parse metadata JSON
		metadata := make(map[string]interface{})
		if metadataJSON.Valid && metadataJSON.String != "" {
			// Parse the JSON metadata
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
				return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
			}
		}

		// Add similarity score to metadata
		metadata["similarity"] = similarity

		// Create embedding vector
		embedding := &EmbeddingVector{
			Vector:      vector,
			Dimensions:  dimensions,
			ModelID:     modelID,
			ContentType: contentType,
			ContentID:   contentID,
			Metadata:    metadata,
		}

		// Apply scoring formula if weight factors are specified
		score := similarity // Default to raw similarity
		if len(options.WeightFactors) > 0 {
			// Start with the weighted similarity
			score = similarity * options.WeightFactors["similarity"]
			// Add other weighted factors from metadata
			for field, weight := range options.WeightFactors {
				if field == "similarity" {
					continue // Already handled
				}
				if metaValue, ok := metadata[field]; ok {
					// Try to convert to float32
					switch v := metaValue.(type) {
					case float64:
						score += float32(v) * weight
					case float32:
						score += v * weight
					case int:
						score += float32(v) * weight
					case int64:
						score += float32(v) * weight
					}
				}
			}
			// Normalize the score to 0-1 range (simplified)
			if score > 1.0 {
				score = 1.0
			} else if score < 0.0 {
				score = 0.0
			}
		}

		// Create result
		result := &SearchResult{
			Content: embedding,
			Score:   score,
			Matches: map[string]interface{}{
				"similarity": similarity,
			},
		}

		results.Results = append(results.Results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	// Set total and HasMore based on the options limit
	results.Total = len(results.Results)

	// For proper pagination, we would need to do a count query
	// This is a simplified version
	results.HasMore = results.Total >= options.Limit

	return results, nil
}
