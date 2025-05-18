package proxies

import (
	"context"
	"fmt"
	"reflect"

	"github.com/S-Corkum/devops-mcp/pkg/client/rest"
	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// SearchAPIProxy implements the SearchRepository interface by delegating to a REST API client
type SearchAPIProxy struct {
	client *rest.SearchClient
	logger observability.Logger
}

// Helper function to safely access result fields using reflection
// This handles the case where the embedding.SearchResult structure may have changed
func getResultField(result interface{}, fieldName string) interface{} {
	// Use reflection to safely access fields
	val := reflect.ValueOf(result)
	
	// Handle nil or non-struct types
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	
	// If not a struct, we can't access fields
	if val.Kind() != reflect.Struct {
		return nil
	}
	
	// Try to get the field
	field := val.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	
	// Return the interface{} value of the field
	return field.Interface()
}

// NewSearchAPIProxy creates a new SearchAPIProxy
func NewSearchAPIProxy(client *rest.SearchClient, logger observability.Logger) repository.SearchRepository {
	if logger == nil {
		logger = observability.NewLogger("search-api-proxy")
	}

	return &SearchAPIProxy{
		client: client,
		logger: logger,
	}
}

// SearchByText performs a vector search using a text query
func (p *SearchAPIProxy) SearchByText(ctx context.Context, query string, options *repository.SearchOptions) (*repository.SearchResults, error) {
	p.logger.Debug("Performing text search via API proxy", map[string]interface{}{
		"query": query,
	})

	// Convert repository.SearchOptions to embedding.SearchOptions
	embeddingOptions := &embedding.SearchOptions{
		Limit:         options.Limit,
		Offset:        options.Offset,
		MinSimilarity: options.MinSimilarity,
	}

	// Convert filters if present
	if len(options.Filters) > 0 {
		embeddingOptions.Filters = make([]embedding.SearchFilter, len(options.Filters))
		for i, filter := range options.Filters {
			embeddingOptions.Filters[i] = embedding.SearchFilter{
				Field:    filter.Field,
				Operator: filter.Operator,
				Value:    filter.Value,
			}
		}
	}

	// Convert sorts if present
	if len(options.Sorts) > 0 {
		embeddingOptions.Sorts = make([]embedding.SearchSort, len(options.Sorts))
		for i, sort := range options.Sorts {
			embeddingOptions.Sorts[i] = embedding.SearchSort{
				Field:     sort.Field,
				Direction: sort.Direction,
			}
		}
	}

	// Copy content types and weight factors
	embeddingOptions.ContentTypes = options.ContentTypes
	embeddingOptions.WeightFactors = options.WeightFactors

	results, err := p.client.SearchByText(ctx, query, embeddingOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to perform text search via REST API: %w", err)
	}

	return p.convertSearchResults(results)
}

// SearchByVector performs a vector search using a pre-computed vector
func (p *SearchAPIProxy) SearchByVector(ctx context.Context, vector []float32, options *repository.SearchOptions) (*repository.SearchResults, error) {
	p.logger.Debug("Performing vector search via API proxy", map[string]interface{}{
		"vector_size": len(vector),
	})

	// Convert repository.SearchOptions to embedding.SearchOptions (same as in SearchByText)
	embeddingOptions := &embedding.SearchOptions{
		Limit:         options.Limit,
		Offset:        options.Offset,
		MinSimilarity: options.MinSimilarity,
		ContentTypes:  options.ContentTypes,
		WeightFactors: options.WeightFactors,
	}

	// Convert filters if present
	if len(options.Filters) > 0 {
		embeddingOptions.Filters = make([]embedding.SearchFilter, len(options.Filters))
		for i, filter := range options.Filters {
			embeddingOptions.Filters[i] = embedding.SearchFilter{
				Field:    filter.Field,
				Operator: filter.Operator,
				Value:    filter.Value,
			}
		}
	}

	// Convert sorts if present
	if len(options.Sorts) > 0 {
		embeddingOptions.Sorts = make([]embedding.SearchSort, len(options.Sorts))
		for i, sort := range options.Sorts {
			embeddingOptions.Sorts[i] = embedding.SearchSort{
				Field:     sort.Field,
				Direction: sort.Direction,
			}
		}
	}

	results, err := p.client.SearchByVector(ctx, vector, embeddingOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to perform vector search via REST API: %w", err)
	}

	return p.convertSearchResults(results)
}

// SearchByContentID performs a "more like this" search using an existing content ID
func (p *SearchAPIProxy) SearchByContentID(ctx context.Context, contentID string, options *repository.SearchOptions) (*repository.SearchResults, error) {
	p.logger.Debug("Performing content ID search via API proxy", map[string]interface{}{
		"content_id": contentID,
	})

	// Convert repository.SearchOptions to embedding.SearchOptions (same as in SearchByText)
	embeddingOptions := &embedding.SearchOptions{
		Limit:         options.Limit,
		Offset:        options.Offset,
		MinSimilarity: options.MinSimilarity,
		ContentTypes:  options.ContentTypes,
		WeightFactors: options.WeightFactors,
	}

	// Convert filters if present
	if len(options.Filters) > 0 {
		embeddingOptions.Filters = make([]embedding.SearchFilter, len(options.Filters))
		for i, filter := range options.Filters {
			embeddingOptions.Filters[i] = embedding.SearchFilter{
				Field:    filter.Field,
				Operator: filter.Operator,
				Value:    filter.Value,
			}
		}
	}

	// Convert sorts if present
	if len(options.Sorts) > 0 {
		embeddingOptions.Sorts = make([]embedding.SearchSort, len(options.Sorts))
		for i, sort := range options.Sorts {
			embeddingOptions.Sorts[i] = embedding.SearchSort{
				Field:     sort.Field,
				Direction: sort.Direction,
			}
		}
	}

	results, err := p.client.SearchByContentID(ctx, contentID, embeddingOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to perform content ID search via REST API: %w", err)
	}

	return p.convertSearchResults(results)
}

// convertSearchResults is a helper method to convert embedding.SearchResults to repository.SearchResults
// using reflection to safely handle field differences between versions
func (p *SearchAPIProxy) convertSearchResults(results *embedding.SearchResults) (*repository.SearchResults, error) {
	// Convert embedding.SearchResults to repository.SearchResults
	repoResults := &repository.SearchResults{
		Total:   results.Total,
		HasMore: results.HasMore,
	}

	// Convert individual search results
	repoResults.Results = make([]*repository.SearchResult, len(results.Results))
	for i, result := range results.Results {
		// Extract required fields from embedding structure
		// Adapt between the differing field names based on current embedding package
		// Set safe default values when fields might be missing
		var id string
		var score float64
		var distance float32
		var content string
		var contentType string = "text"
		var metadata map[string]interface{}
		var contentHash string
		
		// Map from embedding result fields to repository fields
		// Each field access is protected with reflection
		
		// Handle the ID field - the embedding package might use a different field name
		if idVal := getResultField(result, "ID"); idVal != nil {
			if str, ok := idVal.(string); ok {
				id = str
			}
		}
		if id == "" {
			id = fmt.Sprintf("%v", i) // Default to index if not available
		}
		
		// Handle score
		if scoreVal := getResultField(result, "Score"); scoreVal != nil {
			if s, ok := scoreVal.(float64); ok {
				score = s
			}
		}
		
		// Handle distance
		if distVal := getResultField(result, "Distance"); distVal != nil {
			if d, ok := distVal.(float32); ok {
				distance = d
			}
		}
		
		// Handle content - might be in an EmbeddingVector field
		if contentVal := getResultField(result, "Content"); contentVal != nil {
			if c, ok := contentVal.(string); ok {
				content = c
			}
		} else if vectorVal := getResultField(result, "Vector"); vectorVal != nil {
			// Try to extract content from the vector if it exists
			if vect, ok := vectorVal.(interface{ GetText() string }); ok {
				content = vect.GetText()
			}
		}
		
		// Handle type field
		if typeVal := getResultField(result, "Type"); typeVal != nil {
			if t, ok := typeVal.(string); ok {
				contentType = t
			}
		}
		
		// Handle metadata
		if metaVal := getResultField(result, "Metadata"); metaVal != nil {
			if m, ok := metaVal.(map[string]interface{}); ok {
				metadata = m
			}
		}
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		
		// Handle content hash
		if hashVal := getResultField(result, "ContentHash"); hashVal != nil {
			if h, ok := hashVal.(string); ok {
				contentHash = h
			}
		}
		
		// Use extracted values to create the repository search result
		repoResults.Results[i] = &repository.SearchResult{
			ID:          id,
			Score:       float32(score), // Convert float64 to float32
			Distance:    distance,
			Content:     content,
			Type:        contentType,
			Metadata:    metadata,
			ContentHash: contentHash,
		}
	}

	return repoResults, nil
}

// GetSupportedModels retrieves a list of all models with embeddings
func (p *SearchAPIProxy) GetSupportedModels(ctx context.Context) ([]string, error) {
	p.logger.Debug("Getting supported models via API proxy", nil)

	models, err := p.client.GetSupportedModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get supported models via REST API: %w", err)
	}

	return models, nil
}

// GetSearchStats retrieves statistics about the search index
func (p *SearchAPIProxy) GetSearchStats(ctx context.Context) (map[string]interface{}, error) {
	p.logger.Debug("Getting search stats via API proxy", nil)

	stats, err := p.client.GetSearchStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get search stats via REST API: %w", err)
	}

	return stats, nil
}
