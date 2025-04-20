package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// GraphQLRequest represents a GitHub GraphQL API request
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GitHub GraphQL API response
type GraphQLResponse struct {
	Data   map[string]interface{} `json:"data,omitempty"`
	Errors []GraphQLError         `json:"errors,omitempty"`
}

// GraphQLError represents an error in a GraphQL response
type GraphQLError struct {
	Message   string                 `json:"message"`
	Locations []GraphQLErrorLocation `json:"locations,omitempty"`
	Path      []interface{}          `json:"path,omitempty"`
	Type      string                 `json:"type,omitempty"`
}

// GraphQLErrorLocation represents the location of a GraphQL error
type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// GraphQLClient provides a client for the GitHub GraphQL API
type GraphQLClient struct {
	config        *Config
	client        *http.Client
	rateLimiter   *resilience.RateLimiter
	logger        *observability.Logger
	metricsClient *observability.MetricsClient
}

// Config holds configuration for the GraphQL client
type Config struct {
	URL            string
	Token          string
	AppID          string
	AppPrivateKey  string
	UseApp         bool
	RequestTimeout time.Duration
}

// NewGraphQLClient creates a new GitHub GraphQL client
func NewGraphQLClient(config *Config, client *http.Client, rateLimiter *resilience.RateLimiter, logger *observability.Logger, metricsClient *observability.MetricsClient) *GraphQLClient {
	return &GraphQLClient{
		config:        config,
		client:        client,
		rateLimiter:   rateLimiter,
		logger:        logger,
		metricsClient: metricsClient,
	}
}

// Query executes a GraphQL query
func (c *GraphQLClient) Query(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	// Check rate limits before sending request
	if !c.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for GitHub GraphQL API")
	}

	// Create request
	req := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	// Execute request
	var resp GraphQLResponse
	if err := c.execute(ctx, req, &resp); err != nil {
		return err
	}

	// Check for GraphQL errors
	if len(resp.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	// Decode response data into result
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal GraphQL response data: %w", err)
	}

	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("failed to unmarshal GraphQL response data: %w", err)
	}

	return nil
}

// execute executes a GraphQL request
func (c *GraphQLClient) execute(ctx context.Context, req GraphQLRequest, resp *GraphQLResponse) error {
	// Start metrics timing
	start := time.Now()
	defer func() {
		c.metricsClient.RecordDuration("github.graphql.request", time.Since(start))
	}()

	// Marshal request
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.config.URL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create GraphQL request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.Token != "" {
		httpReq.Header.Set("Authorization", "bearer "+c.config.Token)
	}
	// TODO: Implement JWT auth for GitHub Apps

	// Execute HTTP request
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		c.metricsClient.IncrementCounter("github.graphql.error", 1)
		return fmt.Errorf("failed to execute GraphQL request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read GraphQL response: %w", err)
	}

	// Unmarshal response
	if err := json.Unmarshal(body, resp); err != nil {
		return fmt.Errorf("failed to unmarshal GraphQL response: %w", err)
	}

	// Log metrics
	c.metricsClient.IncrementCounter("github.graphql.request", 1)
	if len(resp.Errors) > 0 {
		c.metricsClient.IncrementCounter("github.graphql.error", 1)
	}

	return nil
}
