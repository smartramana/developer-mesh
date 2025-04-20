package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// Common GraphQL errors
var (
	ErrGraphQLRequestFailed = errors.New("graphql request failed")
	ErrGraphQLNoData        = errors.New("graphql response contained no data")
	ErrGraphQLRateLimited   = errors.New("graphql request rate limited")
	ErrGraphQLUnauthorized  = errors.New("graphql request unauthorized")
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

// QueryBatchItem represents a single query in a batch
type QueryBatchItem struct {
	Name      string
	Query     string
	Variables map[string]interface{}
}

// BatchResult represents the result of a batch query
type BatchResult struct {
	Data   map[string]interface{}
	Errors []GraphQLError
}

// Common operation methods

// GetRepository retrieves a repository by owner and name
func (c *GraphQLClient) GetRepository(ctx context.Context, owner, name string) (map[string]interface{}, error) {
	query := `
	query GetRepository($owner: String!, $name: String!) {
		repository(owner: $owner, name: $name) {
			...RepositoryFields
		}
	}
	` + RepositoryFragment
	
	variables := map[string]interface{}{
		"owner": owner,
		"name":  name,
	}
	
	var result struct {
		Repository map[string]interface{} `json:"repository"`
	}
	
	if err := c.Query(ctx, query, variables, &result); err != nil {
		return nil, err
	}
	
	return result.Repository, nil
}

// ListRepositories lists repositories for the authenticated user
func (c *GraphQLClient) ListRepositories(ctx context.Context, options *PaginationOptions) ([]map[string]interface{}, error) {
	query := `
	query ListRepositories($first: Int!, $after: String) {
		viewer {
			repositories(first: $first, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
				pageInfo {
					hasNextPage
					endCursor
				}
				nodes {
					...RepositoryFields
				}
			}
		}
	}
	` + RepositoryFragment
	
	var repositories []map[string]interface{}
	
	resultHandler := func(page int, data map[string]interface{}) error {
		if viewer, ok := data["viewer"].(map[string]interface{}); ok {
			if repos, ok := viewer["repositories"].(map[string]interface{}); ok {
				if nodes, ok := repos["nodes"].([]interface{}); ok {
					for _, node := range nodes {
						if repo, ok := node.(map[string]interface{}); ok {
							repositories = append(repositories, repo)
						}
					}
				}
			}
		}
		return nil
	}
	
	if options == nil {
		options = DefaultPaginationOptions()
	}
	options.ResultHandler = resultHandler
	
	variables := map[string]interface{}{
		"first": options.PerPage,
	}
	
	if err := c.QueryPaginated(ctx, query, variables, options); err != nil {
		return nil, err
	}
	
	return repositories, nil
}

// GetIssue retrieves an issue by number
func (c *GraphQLClient) GetIssue(ctx context.Context, owner, repo string, number int) (map[string]interface{}, error) {
	query := `
	query GetIssue($owner: String!, $name: String!, $number: Int!) {
		repository(owner: $owner, name: $name) {
			issue(number: $number) {
				...IssueFields
			}
		}
	}
	` + IssueFragment
	
	variables := map[string]interface{}{
		"owner":  owner,
		"name":   repo,
		"number": number,
	}
	
	var result struct {
		Repository struct {
			Issue map[string]interface{} `json:"issue"`
		} `json:"repository"`
	}
	
	if err := c.Query(ctx, query, variables, &result); err != nil {
		return nil, err
	}
	
	return result.Repository.Issue, nil
}

// GetPullRequest retrieves a pull request by number
func (c *GraphQLClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (map[string]interface{}, error) {
	query := `
	query GetPullRequest($owner: String!, $name: String!, $number: Int!) {
		repository(owner: $owner, name: $name) {
			pullRequest(number: $number) {
				...PullRequestFields
			}
		}
	}
	` + PullRequestFragment
	
	variables := map[string]interface{}{
		"owner":  owner,
		"name":   repo,
		"number": number,
	}
	
	var result struct {
		Repository struct {
			PullRequest map[string]interface{} `json:"pullRequest"`
		} `json:"repository"`
	}
	
	if err := c.Query(ctx, query, variables, &result); err != nil {
		return nil, err
	}
	
	return result.Repository.PullRequest, nil
}

// GetCurrentUser gets information about the authenticated user
func (c *GraphQLClient) GetCurrentUser(ctx context.Context) (map[string]interface{}, error) {
	query := `
	query GetCurrentUser {
		viewer {
			...UserFields
		}
	}
	` + UserFragment
	
	var result struct {
		Viewer map[string]interface{} `json:"viewer"`
	}
	
	if err := c.Query(ctx, query, nil, &result); err != nil {
		return nil, err
	}
	
	return result.Viewer, nil
}

// GraphQLClient provides a client for the GitHub GraphQL API
type GraphQLClient struct {
	config        *Config
	client        *http.Client
	rateLimiter   *resilience.RateLimiter
	logger        *observability.Logger
	metricsClient *observability.MetricsClient
	queryCache    map[string]interface{}
	cacheMutex    sync.RWMutex
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
	// Set default URL if not provided
	if config.URL == "" {
		config.URL = "https://api.github.com/graphql"
	}
	
	// Set default request timeout if not provided
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	
	return &GraphQLClient{
		config:        config,
		client:        client,
		rateLimiter:   rateLimiter,
		logger:        logger,
		metricsClient: metricsClient,
		queryCache:    make(map[string]interface{}),
	}
}

// PaginationOptions defines options for paginated GraphQL queries
type PaginationOptions struct {
	// PerPage is the number of items per page
	PerPage int
	// MaxPages is the maximum number of pages to fetch
	MaxPages int
	// PageInfo specifies the GraphQL fields to query for pagination info
	PageInfo string
	// ItemsField is the field in the response containing the paginated items
	ItemsField string
	// ResultHandler is called for each page of results
	ResultHandler func(page int, data map[string]interface{}) error
}

// DefaultPaginationOptions returns default pagination options
func DefaultPaginationOptions() *PaginationOptions {
	return &PaginationOptions{
		PerPage:  100,
		MaxPages: 10,
		PageInfo: `pageInfo {
			hasNextPage
			endCursor
		}`,
		ItemsField: "nodes",
	}
}

// Query executes a GraphQL query
func (c *GraphQLClient) Query(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	// Check rate limits before sending request
	if !c.rateLimiter.Allow() {
		return ErrGraphQLRateLimited
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

	// If the response has no data, that's an error
	if resp.Data == nil || len(resp.Data) == 0 {
		if len(resp.Errors) > 0 {
			return fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
		}
		return ErrGraphQLNoData
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

// QueryPaginated executes a paginated GraphQL query
func (c *GraphQLClient) QueryPaginated(ctx context.Context, query string, variables map[string]interface{}, options *PaginationOptions) error {
	if options == nil {
		options = DefaultPaginationOptions()
	}
	
	// Add pagination variables if not already present
	if variables == nil {
		variables = make(map[string]interface{})
	}
	
	if _, ok := variables["first"]; !ok {
		variables["first"] = options.PerPage
	}
	
	if _, ok := variables["after"]; !ok {
		variables["after"] = nil
	}

	// Process pages
	for page := 1; page <= options.MaxPages; page++ {
		// Execute query
		var resp map[string]interface{}
		if err := c.Query(ctx, query, variables, &resp); err != nil {
			return fmt.Errorf("failed to fetch page %d: %w", page, err)
		}
		
		// Handle page results
		if options.ResultHandler != nil {
			if err := options.ResultHandler(page, resp); err != nil {
				return fmt.Errorf("error handling page %d: %w", page, err)
			}
		}
		
		// Check if there are more pages
		hasNextPage, endCursor := c.extractPageInfo(resp, options.ItemsField)
		if !hasNextPage {
			break
		}
		
		// Update cursor for next page
		variables["after"] = endCursor
		
		// Rate limit between pages
		if page < options.MaxPages {
			time.Sleep(100 * time.Millisecond)
		}
	}
	
	return nil
}

// extractPageInfo extracts pagination info from a GraphQL response
func (c *GraphQLClient) extractPageInfo(data map[string]interface{}, itemsField string) (bool, string) {
	// Navigate into the data structure to find the pageInfo
	for key, value := range data {
		if subObj, ok := value.(map[string]interface{}); ok {
			if pageInfo, ok := subObj["pageInfo"].(map[string]interface{}); ok {
				hasNextPage, _ := pageInfo["hasNextPage"].(bool)
				endCursor, _ := pageInfo["endCursor"].(string)
				return hasNextPage, endCursor
			}
			
			// Recursively search for pageInfo
			hasNextPage, endCursor := c.extractPageInfo(subObj, itemsField)
			if hasNextPage {
				return hasNextPage, endCursor
			}
		}
	}
	
	return false, ""
}

// Common query fragments for reuse
const (
	UserFragment = `
	fragment UserFields on User {
		id
		login
		name
		avatarUrl
		url
		email
		bio
		company
		location
	}
	`
	
	RepositoryFragment = `
	fragment RepositoryFields on Repository {
		id
		name
		nameWithOwner
		description
		url
		sshUrl
		homepageUrl
		isPrivate
		isArchived
		isDisabled
		isFork
		createdAt
		updatedAt
		pushedAt
		defaultBranchRef {
			name
		}
	}
	`
	
	IssueFragment = `
	fragment IssueFields on Issue {
		id
		number
		title
		body
		state
		url
		createdAt
		updatedAt
		closedAt
		author {
			login
		}
		assignees(first: 5) {
			nodes {
				login
			}
		}
		labels(first: 10) {
			nodes {
				name
				color
			}
		}
	}
	`
	
	PullRequestFragment = `
	fragment PullRequestFields on PullRequest {
		id
		number
		title
		body
		state
		url
		isDraft
		mergeable
		createdAt
		updatedAt
		closedAt
		mergedAt
		author {
			login
		}
		assignees(first: 5) {
			nodes {
				login
			}
		}
		labels(first: 10) {
			nodes {
				name
				color
			}
		}
		baseRefName
		headRefName
	}
	`
)

// execute executes a GraphQL request
func (c *GraphQLClient) execute(ctx context.Context, req GraphQLRequest, resp *GraphQLResponse) error {
	// Check rate limits before sending request
	if !c.rateLimiter.Allow() {
		return ErrGraphQLRateLimited
	}
	
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

	// Create HTTP request with timeout
	requestCtx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()
	
	httpReq, err := http.NewRequestWithContext(requestCtx, "POST", c.config.URL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create GraphQL request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.Token != "" {
		httpReq.Header.Set("Authorization", "bearer "+c.config.Token)
	} else if c.config.UseApp && c.config.AppID != "" && c.config.AppPrivateKey != "" {
		// Implement JWT auth for GitHub Apps
		token, err := c.getJWTToken()
		if err != nil {
			return fmt.Errorf("failed to generate GitHub App JWT token: %w", err)
		}
		httpReq.Header.Set("Authorization", "bearer "+token)
	}

	// Execute HTTP request
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		c.metricsClient.IncrementCounter("github.graphql.error", 1)
		return fmt.Errorf("%w: %v", ErrGraphQLRequestFailed, err)
	}
	defer httpResp.Body.Close()

	// Check response status
	if httpResp.StatusCode != http.StatusOK {
		c.metricsClient.IncrementCounter("github.graphql.error", 1)
		
		// Handle specific HTTP status codes
		switch httpResp.StatusCode {
		case http.StatusUnauthorized:
			return ErrGraphQLUnauthorized
		case http.StatusForbidden:
			// Check if this is a rate limit error
			if strings.Contains(httpResp.Header.Get("X-RateLimit-Remaining"), "0") {
				return ErrGraphQLRateLimited
			}
			return fmt.Errorf("forbidden: %w", ErrGraphQLRequestFailed)
		default:
			return fmt.Errorf("HTTP %d: %w", httpResp.StatusCode, ErrGraphQLRequestFailed)
		}
	}

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read GraphQL response: %w", err)
	}

	// Unmarshal response
	if err := json.Unmarshal(body, resp); err != nil {
		return fmt.Errorf("failed to unmarshal GraphQL response: %w", err)
	}

	// Check for GraphQL errors
	if len(resp.Errors) > 0 {
		c.metricsClient.IncrementCounter("github.graphql.error", 1)
		
		// Log errors
		for _, e := range resp.Errors {
			c.logger.Warn("GraphQL error", "message", e.Message, "type", e.Type)
		}
		
		// If response has no data, return error
		if resp.Data == nil || len(resp.Data) == 0 {
			return fmt.Errorf("%w: %s", ErrGraphQLNoData, resp.Errors[0].Message)
		}
		
		// If response has some data, just log the errors and continue
	}

	// Log metrics
	c.metricsClient.IncrementCounter("github.graphql.request", 1)

	return nil
}

// getJWTToken generates a JWT token for GitHub App authentication
// This is a stub - implementation would vary based on how you handle GitHub App authentication
func (c *GraphQLClient) getJWTToken() (string, error) {
	// In a real implementation, this would:
	// 1. Load the private key
	// 2. Generate a JWT token with the required claims
	// 3. Return the signed token
	
	// For now, return an error
	return "", fmt.Errorf("GitHub App JWT authentication not implemented")
}

// BatchQuery executes multiple GraphQL queries in a single request
func (c *GraphQLClient) BatchQuery(ctx context.Context, queries []QueryBatchItem) (map[string]BatchResult, error) {
	// Initialize result
	results := make(map[string]BatchResult)
	
	// Process queries in batches of 10 (GitHub limitation)
	batchSize := 10
	for i := 0; i < len(queries); i += batchSize {
		end := i + batchSize
		if end > len(queries) {
			end = len(queries)
		}
		
		batchQueries := queries[i:end]
		
		// Process batch
		batchResults, err := c.executeBatch(ctx, batchQueries)
		if err != nil {
			return nil, err
		}
		
		// Merge results
		for k, v := range batchResults {
			results[k] = v
		}
	}
	
	return results, nil
}

// executeBatch executes a batch of GraphQL queries
func (c *GraphQLClient) executeBatch(ctx context.Context, queries []QueryBatchItem) (map[string]BatchResult, error) {
	// Create combined query
	combinedQuery := "query {\n"
	variables := make(map[string]interface{})
	
	for _, q := range queries {
		// Extract query body (everything between the outer { })
		queryBody := q.Query
		queryBody = strings.TrimSpace(queryBody)
		
		// If query has the query keyword, extract just the body
		if strings.HasPrefix(queryBody, "query") {
			openBrace := strings.Index(queryBody, "{")
			if openBrace != -1 {
				closeBrace := findMatchingCloseBrace(queryBody, openBrace)
				if closeBrace != -1 {
					queryBody = queryBody[openBrace+1:closeBrace]
				}
			}
		} else if strings.HasPrefix(queryBody, "{") {
			// If query starts with {, extract just the body
			closeBrace := findMatchingCloseBrace(queryBody, 0)
			if closeBrace != -1 {
				queryBody = queryBody[1:closeBrace]
			}
		}
		
		// Add query to combined query
		combinedQuery += fmt.Sprintf("  %s: %s\n", q.Name, queryBody)
		
		// Add variables
		for k, v := range q.Variables {
			variables[fmt.Sprintf("%s_%s", q.Name, k)] = v
		}
	}
	
	combinedQuery += "}"
	
	// Execute combined query
	request := GraphQLRequest{
		Query:     combinedQuery,
		Variables: variables,
	}
	
	var response GraphQLResponse
	if err := c.execute(ctx, request, &response); err != nil {
		return nil, err
	}
	
	// Parse results
	results := make(map[string]BatchResult)
	
	for _, q := range queries {
		result := BatchResult{}
		
		// Extract data for this query
		if response.Data != nil {
			if data, ok := response.Data[q.Name]; ok {
				result.Data = map[string]interface{}{
					"data": data,
				}
			}
		}
		
		// Extract errors for this query
		for _, err := range response.Errors {
			for _, path := range err.Path {
				if pathStr, ok := path.(string); ok && pathStr == q.Name {
					result.Errors = append(result.Errors, err)
					break
				}
			}
		}
		
		results[q.Name] = result
	}
	
	return results, nil
}

// findMatchingCloseBrace finds the matching close brace for an open brace
func findMatchingCloseBrace(s string, openIndex int) int {
	if openIndex >= len(s) || s[openIndex] != '{' {
		return -1
	}
	
	depth := 1
	for i := openIndex + 1; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	
	return -1
}
