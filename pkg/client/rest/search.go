package rest

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// SearchClient provides methods for vector search operations
type SearchClient struct {
	client *RESTClient
	logger observability.Logger
}

// NewSearchClient creates a new SearchClient with the provided client
func NewSearchClient(client *RESTClient, logger observability.Logger) *SearchClient {
	return &SearchClient{
		client: client,
		logger: logger,
	}
}

// SearchByText performs a vector search using text query
func (c *SearchClient) SearchByText(ctx context.Context, query string, options *embedding.SearchOptions) (*embedding.SearchResults, error) {
	c.logger.Debug("Performing text search via REST API", map[string]interface{}{
		"query": query,
	})

	// Prepare the request body
	requestBody := struct {
		Query        string                  `json:"query"`
		ContentTypes []string                `json:"content_types,omitempty"`
		Filters      []embedding.SearchFilter `json:"filters,omitempty"`
		Sorts        []embedding.SearchSort   `json:"sorts,omitempty"`
		Limit        int                     `json:"limit,omitempty"`
		Offset       int                     `json:"offset,omitempty"`
		MinSimilarity float32                `json:"min_similarity,omitempty"`
		WeightFactors map[string]float32     `json:"weight_factors,omitempty"`
	}{
		Query: query,
	}

	// Add options if provided
	if options != nil {
		requestBody.ContentTypes = options.ContentTypes
		requestBody.Filters = options.Filters
		requestBody.Sorts = options.Sorts
		requestBody.Limit = options.Limit
		requestBody.Offset = options.Offset
		requestBody.MinSimilarity = options.MinSimilarity
		requestBody.WeightFactors = options.WeightFactors
	}

	var result struct {
		Results []*embedding.SearchResult `json:"results"`
		Total   int                       `json:"total"`
		HasMore bool                      `json:"has_more"`
	}

	if err := c.client.Post(ctx, "/api/v1/search", requestBody, &result); err != nil {
		return nil, fmt.Errorf("failed to perform text search: %w", err)
	}

	return &embedding.SearchResults{
		Results: result.Results,
		Total:   result.Total,
		HasMore: result.HasMore,
	}, nil
}

// SearchByVector performs a vector search using a pre-computed vector
func (c *SearchClient) SearchByVector(ctx context.Context, vector []float32, options *embedding.SearchOptions) (*embedding.SearchResults, error) {
	c.logger.Debug("Performing vector search via REST API", map[string]interface{}{
		"vector_size": len(vector),
	})

	// Prepare the request body
	requestBody := struct {
		Vector       []float32               `json:"vector"`
		ContentTypes []string                `json:"content_types,omitempty"`
		Filters      []embedding.SearchFilter `json:"filters,omitempty"`
		Sorts        []embedding.SearchSort   `json:"sorts,omitempty"`
		Limit        int                     `json:"limit,omitempty"`
		Offset       int                     `json:"offset,omitempty"`
		MinSimilarity float32                `json:"min_similarity,omitempty"`
		WeightFactors map[string]float32     `json:"weight_factors,omitempty"`
	}{
		Vector: vector,
	}

	// Add options if provided
	if options != nil {
		requestBody.ContentTypes = options.ContentTypes
		requestBody.Filters = options.Filters
		requestBody.Sorts = options.Sorts
		requestBody.Limit = options.Limit
		requestBody.Offset = options.Offset
		requestBody.MinSimilarity = options.MinSimilarity
		requestBody.WeightFactors = options.WeightFactors
	}

	var result struct {
		Results []*embedding.SearchResult `json:"results"`
		Total   int                       `json:"total"`
		HasMore bool                      `json:"has_more"`
	}

	if err := c.client.Post(ctx, "/api/v1/search/vector", requestBody, &result); err != nil {
		return nil, fmt.Errorf("failed to perform vector search: %w", err)
	}

	return &embedding.SearchResults{
		Results: result.Results,
		Total:   result.Total,
		HasMore: result.HasMore,
	}, nil
}

// SearchByContentID performs a "more like this" search using an existing content ID
func (c *SearchClient) SearchByContentID(ctx context.Context, contentID string, options *embedding.SearchOptions) (*embedding.SearchResults, error) {
	c.logger.Debug("Performing content ID search via REST API", map[string]interface{}{
		"content_id": contentID,
	})

	// Prepare the request body
	requestBody := struct {
		ContentID    string                  `json:"content_id"`
		Options      *embedding.SearchOptions `json:"options,omitempty"`
	}{
		ContentID: contentID,
		Options:   options,
	}

	var result struct {
		Results []*embedding.SearchResult `json:"results"`
		Total   int                       `json:"total"`
		HasMore bool                      `json:"has_more"`
	}

	if err := c.client.Post(ctx, "/api/v1/search/similar", requestBody, &result); err != nil {
		return nil, fmt.Errorf("failed to perform content ID search: %w", err)
	}

	return &embedding.SearchResults{
		Results: result.Results,
		Total:   result.Total,
		HasMore: result.HasMore,
	}, nil
}

// GetSupportedModels retrieves a list of all models with embeddings
func (c *SearchClient) GetSupportedModels(ctx context.Context) ([]string, error) {
	c.logger.Debug("Getting supported models via REST API", nil)

	var result struct {
		Models []string `json:"models"`
	}

	if err := c.client.Get(ctx, "/api/v1/search/models", &result); err != nil {
		return nil, fmt.Errorf("failed to get supported models: %w", err)
	}

	return result.Models, nil
}

// GetSearchStats retrieves statistics about the search index
func (c *SearchClient) GetSearchStats(ctx context.Context) (map[string]interface{}, error) {
	c.logger.Debug("Getting search stats via REST API", nil)

	var result map[string]interface{}

	if err := c.client.Get(ctx, "/api/v1/search/stats", &result); err != nil {
		return nil, fmt.Errorf("failed to get search stats: %w", err)
	}

	return result, nil
}
