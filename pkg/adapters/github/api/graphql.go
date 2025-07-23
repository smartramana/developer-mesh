package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/adapters/resilience"
	"github.com/developer-mesh/developer-mesh/pkg/common/errors"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/golang-jwt/jwt/v4"
)

// GraphQLRequest represents a GitHub GraphQL API request
type GraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// GraphQLResponse represents a GitHub GraphQL API response
type GraphQLResponse struct {
	Data   map[string]any `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents an error in a GraphQL response
type GraphQLError struct {
	Message   string                 `json:"message"`
	Locations []GraphQLErrorLocation `json:"locations,omitempty"`
	Path      []any                  `json:"path,omitempty"`
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
	Variables map[string]any
}

// BatchResult represents the result of a batch query
type BatchResult struct {
	Data   map[string]any
	Errors []GraphQLError
}

// Common operation methods

// GetRepository retrieves a repository by owner and name
func (c *GraphQLClient) GetRepository(ctx context.Context, owner, name string) (map[string]any, error) {
	query := `
	query GetRepository($owner: String!, $name: String!) {
		repository(owner: $owner, name: $name) {
			...RepositoryFields
		}
	}
	` + RepositoryFragment

	variables := map[string]any{
		"owner": owner,
		"name":  name,
	}

	var result struct {
		Repository map[string]any `json:"repository"`
	}

	if err := c.Query(ctx, query, variables, &result); err != nil {
		return nil, err
	}

	return result.Repository, nil
}

// ListRepositories lists repositories for the authenticated user
func (c *GraphQLClient) ListRepositories(ctx context.Context, options *GraphQLPaginationOptions) ([]map[string]any, error) {
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

	var repositories []map[string]any

	resultHandler := func(page int, data map[string]any) error {
		if viewer, ok := data["viewer"].(map[string]any); ok {
			if repos, ok := viewer["repositories"].(map[string]any); ok {
				if nodes, ok := repos["nodes"].([]any); ok {
					for _, node := range nodes {
						if repo, ok := node.(map[string]any); ok {
							repositories = append(repositories, repo)
						}
					}
				}
			}
		}
		return nil
	}

	if options == nil {
		options = DefaultGraphQLPaginationOptions()
	}
	options.ResultHandler = resultHandler

	variables := map[string]any{
		"first": options.PerPage,
	}

	if err := c.QueryPaginated(ctx, query, variables, options); err != nil {
		return nil, err
	}

	return repositories, nil
}

// GetIssue retrieves an issue by number
func (c *GraphQLClient) GetIssue(ctx context.Context, owner, repo string, number int) (map[string]any, error) {
	query := `
	query GetIssue($owner: String!, $name: String!, $number: Int!) {
		repository(owner: $owner, name: $name) {
			issue(number: $number) {
				...IssueFields
			}
		}
	}
	` + IssueFragment

	variables := map[string]any{
		"owner":  owner,
		"name":   repo,
		"number": number,
	}

	var result struct {
		Repository struct {
			Issue map[string]any `json:"issue"`
		} `json:"repository"`
	}

	if err := c.Query(ctx, query, variables, &result); err != nil {
		return nil, err
	}

	return result.Repository.Issue, nil
}

// GetPullRequest retrieves a pull request by number
func (c *GraphQLClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (map[string]any, error) {
	query := `
	query GetPullRequest($owner: String!, $name: String!, $number: Int!) {
		repository(owner: $owner, name: $name) {
			pullRequest(number: $number) {
				...PullRequestFields
			}
		}
	}
	` + PullRequestFragment

	variables := map[string]any{
		"owner":  owner,
		"name":   repo,
		"number": number,
	}

	var result struct {
		Repository struct {
			PullRequest map[string]any `json:"pullRequest"`
		} `json:"repository"`
	}

	if err := c.Query(ctx, query, variables, &result); err != nil {
		return nil, err
	}

	return result.Repository.PullRequest, nil
}

// GetCurrentUser gets information about the authenticated user
func (c *GraphQLClient) GetCurrentUser(ctx context.Context) (map[string]any, error) {
	query := `
	query GetCurrentUser {
		viewer {
			...UserFields
		}
	}
	` + UserFragment

	var result struct {
		Viewer map[string]any `json:"viewer"`
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
	rateLimiter   resilience.RateLimiter
	logger        observability.Logger
	metricsClient observability.MetricsClient
	queryCache    map[string]map[string]any
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
func NewGraphQLClient(config *Config, client *http.Client, rateLimiter resilience.RateLimiter, logger observability.Logger, metricsClient observability.MetricsClient) *GraphQLClient {
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
		queryCache:    make(map[string]map[string]any),
	}
}

// GraphQLPaginationOptions defines options for paginated GraphQL queries
type GraphQLPaginationOptions struct {
	// PerPage is the number of items per page
	PerPage int
	// MaxPages is the maximum number of pages to fetch
	MaxPages int
	// PageInfo specifies the GraphQL fields to query for pagination info
	PageInfo string
	// ItemsField is the field in the response containing the paginated items
	ItemsField string
	// ResultHandler is called for each page of results
	ResultHandler func(page int, data map[string]any) error
}

// DefaultGraphQLPaginationOptions returns default pagination options
func DefaultGraphQLPaginationOptions() *GraphQLPaginationOptions {
	return &GraphQLPaginationOptions{
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
func (c *GraphQLClient) Query(ctx context.Context, query string, variables map[string]any, result any) error {
	// Check rate limits before sending request
	if c.rateLimiter != nil && !c.rateLimiter.Allow() {
		return errors.NewGitHubError(
			errors.ErrRateLimitExceeded,
			0,
			"rate limit exceeded for GitHub GraphQL API",
		)
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
	if len(resp.Data) == 0 {
		if len(resp.Errors) > 0 {
			return errors.NewGitHubError(
				errors.ErrGraphQLResponse,
				0,
				resp.Errors[0].Message,
			).WithResource("graphql", "")
		}
		return errors.NewGitHubError(
			errors.ErrGraphQLResponse,
			0,
			"GraphQL response contained no data",
		).WithResource("graphql", "")
	}

	// Decode response data into result
	// First check if result is nil
	if result == nil {
		return nil
	}

	// Handle different result types
	switch v := result.(type) {
	case *map[string]any:
		// Direct assignment for map
		*v = resp.Data
	case *any:
		// Direct assignment for interface
		*v = resp.Data
	default:
		// For other types, marshal and unmarshal
		data, err := json.Marshal(resp.Data)
		if err != nil {
			return errors.NewGitHubError(
				errors.ErrGraphQLResponse,
				0,
				"failed to marshal GraphQL response data",
			).WithContext("error", err.Error())
		}

		if err := json.Unmarshal(data, result); err != nil {
			return errors.NewGitHubError(
				errors.ErrGraphQLResponse,
				0,
				"failed to unmarshal GraphQL response data",
			).WithContext("error", err.Error()).
				WithContext("result_type", fmt.Sprintf("%T", result))
		}
	}

	return nil
}

// QueryPaginated executes a paginated GraphQL query
func (c *GraphQLClient) QueryPaginated(ctx context.Context, query string, variables map[string]any, options *GraphQLPaginationOptions) error {
	if options == nil {
		options = DefaultGraphQLPaginationOptions()
	}

	// Add pagination variables if not already present
	if variables == nil {
		variables = make(map[string]any)
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
		var resp map[string]any
		if err := c.Query(ctx, query, variables, &resp); err != nil {
			return errors.NewGitHubError(
				errors.ErrGraphQLResponse,
				0,
				fmt.Sprintf("failed to fetch page %d", page),
			).WithContext("error", err.Error())
		}

		// Handle page results
		if options.ResultHandler != nil {
			if err := options.ResultHandler(page, resp); err != nil {
				return errors.NewGitHubError(
					errors.ErrGraphQLResponse,
					0,
					fmt.Sprintf("error handling page %d", page),
				).WithContext("error", err.Error())
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
func (c *GraphQLClient) extractPageInfo(data map[string]any, itemsField string) (bool, string) {
	// First try looking for the specified itemsField directly
	if itemsField != "" {
		// Try to find the field that contains the connection (e.g., "repositories", "issues", etc.)
		for _, value := range data {
			// Look for the connection in the top level
			connectionData, ok := c.findConnection(value, itemsField)
			if ok {
				return c.extractPageInfoFromConnection(connectionData)
			}

			// If not at top level, try one level down
			if subObj, ok := value.(map[string]any); ok {
				for _, subValue := range subObj {
					connectionData, ok := c.findConnection(subValue, itemsField)
					if ok {
						return c.extractPageInfoFromConnection(connectionData)
					}
				}
			}
		}
	}

	// Fallback to searching the entire structure recursively
	return c.recursiveExtractPageInfo(data)
}

// findConnection tries to find the connection object containing pageInfo
func (c *GraphQLClient) findConnection(value any, itemsField string) (map[string]any, bool) {
	// Check if the value is a map
	connectionData, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}

	// Check if it has pageInfo
	if _, hasPageInfo := connectionData["pageInfo"]; hasPageInfo {
		// This is likely the connection we're looking for
		return connectionData, true
	}

	// Check if it has the items field we're looking for
	if _, hasItems := connectionData[itemsField]; hasItems {
		return connectionData, true
	}

	return nil, false
}

// extractPageInfoFromConnection extracts pagination info from a connection object
func (c *GraphQLClient) extractPageInfoFromConnection(connection map[string]any) (bool, string) {
	// Look for pageInfo in the connection
	pageInfoRaw, exists := connection["pageInfo"]
	if !exists {
		return false, ""
	}

	// Type assertion with validation
	pageInfo, ok := pageInfoRaw.(map[string]any)
	if !ok && c.logger != nil {
		c.logger.Warn("Invalid pageInfo type", map[string]any{
			"expected": "map[string]any",
			"actual":   fmt.Sprintf("%T", pageInfoRaw),
		})
		return false, ""
	}

	// Extract hasNextPage with validation
	hasNextPageVal, exists := pageInfo["hasNextPage"]
	if !exists {
		return false, ""
	}

	// Handle different types for hasNextPage (some APIs might return string "true"/"false")
	var hasNextPage bool

	switch v := hasNextPageVal.(type) {
	case bool:
		hasNextPage = v
	case string:
		hasNextPage = (v == "true")
	case int:
		hasNextPage = (v != 0)
	case float64:
		hasNextPage = (v != 0)
	default:
		c.logger.Warn("Invalid hasNextPage type", map[string]any{
			"expected": "bool/string/number",
			"actual":   fmt.Sprintf("%T", hasNextPageVal),
			"value":    fmt.Sprintf("%v", hasNextPageVal),
		})
		return false, ""
	}

	// If there's no next page, no need to extract the cursor
	if !hasNextPage {
		return false, ""
	}

	// Extract endCursor with validation
	endCursorVal, exists := pageInfo["endCursor"]
	if !exists {
		// If no cursor but hasNextPage is true, log a warning
		c.logger.Warn("No endCursor found but hasNextPage is true", nil)
		return false, ""
	}

	// Handle null cursor (end of pagination)
	if endCursorVal == nil {
		return false, ""
	}

	// Handle different types for endCursor
	var endCursor string

	switch v := endCursorVal.(type) {
	case string:
		endCursor = v
	case int, int64, float64:
		endCursor = fmt.Sprintf("%v", v)
	default:
		c.logger.Warn("Invalid endCursor type", map[string]any{
			"expected": "string",
			"actual":   fmt.Sprintf("%T", endCursorVal),
		})
		return false, ""
	}

	return hasNextPage, endCursor
}

// recursiveExtractPageInfo recursively searches for pagination info
func (c *GraphQLClient) recursiveExtractPageInfo(data map[string]any) (bool, string) {
	// Navigate into the data structure to find the pageInfo
	for _, value := range data {
		if subObj, ok := value.(map[string]any); ok {
			// Check if this object has pageInfo
			if _, exists := subObj["pageInfo"]; exists {
				// Get pagination info from this connection
				connection, _ := c.findConnection(subObj, "")
				return c.extractPageInfoFromConnection(connection)
			}

			// Recursively search for pageInfo
			hasNextPage, endCursor := c.recursiveExtractPageInfo(subObj)
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
	if c.rateLimiter != nil && !c.rateLimiter.Allow() {
		return errors.NewGitHubError(
			errors.ErrRateLimitExceeded,
			0,
			"rate limit exceeded for GitHub GraphQL API",
		)
	}

	// Start metrics timing
	start := time.Now()
	defer func() {
		if c.metricsClient != nil {
			c.metricsClient.RecordHistogram("github.graphql.request_duration", time.Since(start).Seconds(), map[string]string{})
		}
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
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	// Set authentication headers
	if c.config.Token != "" {
		// Simple token authentication
		httpReq.Header.Set("Authorization", "bearer "+c.config.Token)
	} else if c.config.UseApp && c.config.AppID != "" && c.config.AppPrivateKey != "" {
		// GitHub App authentication using JWT
		token, err := c.getJWTToken()
		if err != nil {
			return errors.NewGitHubError(
				errors.ErrGraphQLRequest,
				0,
				"failed to generate GitHub App JWT token",
			).WithContext("error", err.Error())
		}
		httpReq.Header.Set("Authorization", "bearer "+token)
	}

	// Execute HTTP request
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		if c.metricsClient != nil {
			c.metricsClient.RecordCounter("github.graphql.error", 1, map[string]string{"type": "request_error"})
		}

		// Create a detailed error with context
		githubErr := errors.NewGitHubError(
			errors.ErrGraphQLRequest,
			0,
			"failed to execute GraphQL request",
		)

		// Add context information
		githubErr = githubErr.WithContext("error", err.Error())
		githubErr = githubErr.WithResource("graphql", c.config.URL)
		githubErr = githubErr.WithOperation("POST", c.config.URL)

		// Add query preview
		if len(req.Query) > 20 {
			githubErr = githubErr.WithContext("query_preview", req.Query[:20]+"...")
		} else {
			githubErr = githubErr.WithContext("query_preview", req.Query)
		}

		// Log the error
		c.logger.Error("GraphQL request failed", map[string]any{
			"error": err.Error(),
			"url":   c.config.URL,
		})

		return githubErr
	}
	defer func() {
		if err := httpResp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]any{"error": err})
		}
	}()

	// Check response status
	if httpResp.StatusCode != http.StatusOK {
		if c.metricsClient != nil {
			c.metricsClient.RecordCounter("github.graphql.error", 1, map[string]string{"status": fmt.Sprintf("%d", httpResp.StatusCode)})
		}

		// Read error body if available
		errorBody, _ := io.ReadAll(httpResp.Body)

		// Try to parse error body as JSON
		var errorResponse struct {
			Message       string `json:"message"`
			Documentation string `json:"documentation_url"`
		}
		// Ignore error, we'll use raw body if this fails
		_ = json.Unmarshal(errorBody, &errorResponse)

		// Create appropriate error
		var message string
		if errorResponse.Message != "" {
			message = errorResponse.Message
		} else {
			message = string(errorBody)
		}

		// Create structured error based on status code
		githubErr := errors.FromHTTPError(
			httpResp.StatusCode,
			message,
			errorResponse.Documentation,
		)

		// Add GraphQL context
		githubErr = githubErr.WithResource("graphql", "")
		githubErr = githubErr.WithOperation("POST", c.config.URL)

		// Add rate limit info if available
		if rateLimit := httpResp.Header.Get("X-RateLimit-Limit"); rateLimit != "" {
			githubErr = githubErr.WithContext("rate_limit", rateLimit)
			githubErr = githubErr.WithContext("rate_limit_remaining", httpResp.Header.Get("X-RateLimit-Remaining"))
			githubErr = githubErr.WithContext("rate_limit_reset", httpResp.Header.Get("X-RateLimit-Reset"))
		}

		// Log appropriate error level
		if c.logger != nil {
			if httpResp.StatusCode >= 500 {
				c.logger.Error("GitHub GraphQL server error", map[string]any{
					"status":  httpResp.StatusCode,
					"message": message,
				})
			} else {
				c.logger.Warn("GitHub GraphQL client error", map[string]any{
					"status":  httpResp.StatusCode,
					"message": message,
				})
			}
		}

		return githubErr
	}

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return errors.NewGitHubError(
			errors.ErrGraphQLResponse,
			httpResp.StatusCode,
			"failed to read GraphQL response",
		).WithContext("error", err.Error())
	}

	// Unmarshal response
	if err := json.Unmarshal(body, resp); err != nil {
		return errors.NewGitHubError(
			errors.ErrGraphQLResponse,
			httpResp.StatusCode,
			"failed to unmarshal GraphQL response",
		).WithContext("error", err.Error())
	}

	// Check for GraphQL errors
	if len(resp.Errors) > 0 {
		if c.metricsClient != nil {
			c.metricsClient.RecordCounter("github.graphql.error", 1, map[string]string{"type": "graphql_error"})
		}

		// Log errors
		for _, e := range resp.Errors {
			if c.logger != nil {
				c.logger.Warn("GraphQL error", map[string]any{
					"message": e.Message,
					"type":    e.Type,
					"query":   strings.Split(req.Query, "\n")[0] + "...", // Log first line of query
				})
			}
		}

		// If response has no data, return error
		if len(resp.Data) == 0 {
			// Create structured error
			githubErr := errors.NewGitHubError(
				errors.ErrGraphQLResponse,
				0,
				resp.Errors[0].Message,
			)

			// Add GraphQL context
			githubErr = githubErr.WithResource("graphql", "")
			githubErr = githubErr.WithOperation("POST", c.config.URL)

			// Add error details
			if resp.Errors[0].Type != "" {
				githubErr = githubErr.WithContext("error_type", resp.Errors[0].Type)
			}

			// Add location if available
			if len(resp.Errors[0].Locations) > 0 {
				loc := resp.Errors[0].Locations[0]
				githubErr = githubErr.WithContext("error_location", fmt.Sprintf("line %d, column %d", loc.Line, loc.Column))
			}

			// Add query info (first 100 chars only)
			if len(req.Query) > 0 {
				queryPreview := req.Query
				if len(queryPreview) > 100 {
					queryPreview = queryPreview[:97] + "..."
				}
				githubErr = githubErr.WithContext("query_preview", queryPreview)
			}

			return githubErr
		}

		// If response has some data, just log the errors and continue
		c.logger.Info("GraphQL query returned partial data with errors", map[string]any{
			"error_count": len(resp.Errors),
			"data_fields": len(resp.Data),
		})
	}

	// Log metrics
	if c.metricsClient != nil {
		c.metricsClient.RecordCounter("github.graphql.request", 1, map[string]string{"status": "success"})
	}

	return nil
}

// getJWTToken generates a JWT token for GitHub App authentication
func (c *GraphQLClient) getJWTToken() (string, error) {
	// Check if App ID and private key are provided
	if c.config.AppID == "" {
		return "", errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"GitHub App ID is required for JWT generation",
		)
	}

	if c.config.AppPrivateKey == "" {
		return "", errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"GitHub App private key is required for JWT generation",
		)
	}

	// Parse the private key
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(c.config.AppPrivateKey))
	if err != nil {
		return "", errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to parse private key for JWT generation",
		).WithContext("error", err.Error())
	}

	// Create the token with required claims
	now := time.Now()
	expirationTime := now.Add(10 * time.Minute) // GitHub tokens are valid for 10 minutes

	claims := &jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		Issuer:    c.config.AppID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Sign the token with the private key
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to sign JWT token",
		).WithContext("error", err.Error())
	}

	if c.logger != nil {
		c.logger.Debug("Generated GitHub App JWT token", map[string]any{
			"app_id":     c.config.AppID,
			"expires_at": expirationTime.Format(time.RFC3339),
			"token_type": "jwt",
		})
	}

	return tokenString, nil
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
	variables := make(map[string]any)

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
					queryBody = queryBody[openBrace+1 : closeBrace]
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
				result.Data = map[string]any{
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
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}
