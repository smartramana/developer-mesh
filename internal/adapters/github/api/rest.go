package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/errors"
	"github.com/S-Corkum/mcp-server/internal/adapters/github/auth"
	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// RESTClient provides a client for the GitHub REST API
type RESTClient struct {
	baseURL       string
	uploadURL     string
	client        *http.Client
	rateLimiter   resilience.RateLimiter
	authProvider  auth.AuthProvider
	logger        *observability.Logger
	metricsClient *observability.MetricsClient
	etagCache     map[string]string
	responseCache map[string]interface{}
	cacheMutex    sync.RWMutex
}

// PaginationOptions provides options for paginated requests
type PaginationOptions struct {
	Page     int
	PerPage  int
	MaxPages int
}

// DefaultPaginationOptions returns default pagination options
func DefaultPaginationOptions() *PaginationOptions {
	return &PaginationOptions{
		Page:     1,
		PerPage:  100,
		MaxPages: 10,
	}
}

// RESTConfig holds configuration for the REST client
type RESTConfig struct {
	BaseURL      string
	UploadURL    string
	AuthProvider auth.AuthProvider
}

// NewRESTClient creates a new GitHub REST client
func NewRESTClient(config *RESTConfig, client *http.Client, rateLimiter resilience.RateLimiter, logger *observability.Logger, metricsClient *observability.MetricsClient) *RESTClient {
	// Set default base URL if not provided
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	
	// Set default upload URL if not provided
	uploadURL := config.UploadURL
	if uploadURL == "" {
		uploadURL = "https://uploads.github.com"
	}
	
	// Ensure URLs end with a slash
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	if !strings.HasSuffix(uploadURL, "/") {
		uploadURL += "/"
	}
	
	return &RESTClient{
		baseURL:       baseURL,
		uploadURL:     uploadURL,
		client:        client,
		rateLimiter:   rateLimiter,
		authProvider:  config.AuthProvider,
		logger:        logger,
		metricsClient: metricsClient,
		etagCache:     make(map[string]string),
		responseCache: make(map[string]interface{}),
	}
}

// requestOptions holds options for making an API request
type requestOptions struct {
	Method   string
	Path     string
	Body     interface{}
	Query    url.Values
	Headers  map[string]string
	IsUpload bool
}

// RateLimitInfo stores GitHub rate limit information
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
	Used      int
}

// doRequest executes an API request with the given options
func (c *RESTClient) doRequest(ctx context.Context, opts requestOptions, result interface{}) error {
	// Check rate limits before sending request
	if !c.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for GitHub REST API")
	}
	
	// Start metrics timing
	start := time.Now()
	defer func() {
		c.metricsClient.RecordDuration("github.rest.request", time.Since(start))
	}()
	
	// Determine base URL
	baseURL := c.baseURL
	if opts.IsUpload {
		baseURL = c.uploadURL
	}
	
	// Build request path
	reqPath := opts.Path
	if opts.Query != nil && len(opts.Query) > 0 {
		reqPath += "?" + opts.Query.Encode()
	}
	
	// Create request URL
	reqURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	
	reqURL, err = reqURL.Parse(reqPath)
	if err != nil {
		return fmt.Errorf("invalid request path: %w", err)
	}
	
	// Create request body if needed
	var bodyReader io.Reader
	if opts.Body != nil {
		// Marshal request body
		bodyData, err := json.Marshal(opts.Body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(bodyData)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, opts.Method, reqURL.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// Set content type and accept headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	// Set authentication headers
	if c.authProvider != nil {
		if err := c.authProvider.SetAuthHeaders(req); err != nil {
			return fmt.Errorf("failed to set auth headers: %w", err)
		}
	}
	
	// Set custom headers
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}
	
	// Execute HTTP request
	resp, err := c.client.Do(req)
	if err != nil {
		c.metricsClient.IncrementCounter("github.rest.error", 1)
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()
	
	// Log metrics
	c.metricsClient.IncrementCounter("github.rest.request", 1)
	
	// Extract and store rate limit information
	rateLimitInfo := c.extractRateLimitInfo(resp.Header)
	if rateLimitInfo != nil {
		// Update rate limiter with the latest information
		if c.rateLimiter != nil {
			if adjuster, ok := c.rateLimiter.(interface{ AdjustRateLimit(RateLimitInfo) }); ok {
				adjuster.AdjustRateLimit(*rateLimitInfo)
			}
		}
		
		// Log rate limit information
		c.logger.Info("GitHub API rate limit",
			"remaining", rateLimitInfo.Remaining,
			"limit", rateLimitInfo.Limit,
			"reset", rateLimitInfo.Reset,
			"used", rateLimitInfo.Used)
		
		// Record rate limit metrics
		c.metricsClient.RecordGauge("github.rest.rate_limit.remaining", float64(rateLimitInfo.Remaining))
		c.metricsClient.RecordGauge("github.rest.rate_limit.limit", float64(rateLimitInfo.Limit))
		c.metricsClient.RecordGauge("github.rest.rate_limit.used", float64(rateLimitInfo.Used))
	}
	
	// Extract ETag for future conditional requests
	etag := resp.Header.Get("ETag")
	if etag != "" && opts.Method == "GET" {
		// Store ETag for the requested path
		c.storeETag(opts.Path, etag)
	}
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Handle 304 Not Modified response
	if resp.StatusCode == http.StatusNotModified {
		// Try to load cached response from previous request
		cachedResp := c.loadCachedResponse(opts.Path)
		if cachedResp != nil && result != nil {
			// Copy cached response to result
			*result.(*interface{}) = *cachedResp
			return nil
		}
		// If no cached response is found, return empty result
		return nil
	}
	
	// Check for error response
	if resp.StatusCode >= 400 {
		var errorResp struct {
			Message          string `json:"message"`
			DocumentationURL string `json:"documentation_url"`
			Errors           []struct {
				Resource string `json:"resource"`
				Field    string `json:"field"`
				Code     string `json:"code"`
			} `json:"errors"`
		}
		
		if err := json.Unmarshal(body, &errorResp); err != nil {
			// If we can't parse the error response, create a generic error
			return errors.NewGitHubError(
				errors.ErrRESTResponse,
				resp.StatusCode,
				string(body),
			).WithOperation(opts.Method, opts.Path)
		}
		
		// Create structured error
		githubErr := errors.FromHTTPError(
			resp.StatusCode,
			errorResp.Message,
			errorResp.DocumentationURL,
		)
		
		// Add operation info
		githubErr.WithOperation(opts.Method, opts.Path)
		
		// Add resource info if available
		if len(errorResp.Errors) > 0 {
			githubErr.WithResource(errorResp.Errors[0].Resource, "")
			
			// Add first error details to context
			githubErr.WithContext("error_code", errorResp.Errors[0].Code)
			if errorResp.Errors[0].Field != "" {
				githubErr.WithContext("error_field", errorResp.Errors[0].Field)
			}
		}
		
		// Extract resource ID from path if possible
		pathParts := strings.Split(opts.Path, "/")
		if len(pathParts) >= 3 && len(pathParts) % 2 == 1 {
			// Typical REST path: repos/owner/repo or repos/owner/repo/issues/123
			if githubErr.Resource == "" && pathParts[0] != "" {
				githubErr.WithResource(pathParts[0], strings.Join(pathParts[1:], "/"))
			}
		}
		
		// Add rate limit info if available
		if rateLimit := resp.Header.Get("X-RateLimit-Limit"); rateLimit != "" {
			githubErr.WithContext("rate_limit", rateLimit)
			githubErr.WithContext("rate_limit_remaining", resp.Header.Get("X-RateLimit-Remaining"))
			githubErr.WithContext("rate_limit_reset", resp.Header.Get("X-RateLimit-Reset"))
		}
		
		// Log the error with appropriate level
		if resp.StatusCode >= 500 {
			c.logger.Error("GitHub API server error", 
				"status", resp.StatusCode,
				"message", errorResp.Message,
				"path", opts.Path,
				"method", opts.Method)
		} else {
			c.logger.Warn("GitHub API client error", 
				"status", resp.StatusCode,
				"message", errorResp.Message,
				"path", opts.Path,
				"method", opts.Method)
		}
		
		return githubErr
	}
	
	// Handle empty response
	if len(body) == 0 || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	
	// Unmarshal response
	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}
	
	return nil
}

// GetPaginated makes a paginated GET request
func (c *RESTClient) GetPaginated(ctx context.Context, path string, options *PaginationOptions, result interface{}) error {
	// Set default options if not provided
	if options == nil {
		options = DefaultPaginationOptions()
	}
	
	// Create query parameters
	query := url.Values{}
	query.Set("page", strconv.Itoa(options.Page))
	query.Set("per_page", strconv.Itoa(options.PerPage))
	
	// Check result type
	switch typedResult := result.(type) {
	case *[]map[string]interface{}:
		// Handle map slice result type (most common)
		return c.getPaginatedMapSlice(ctx, path, query, options, typedResult)
	case *[]interface{}:
		// Handle generic interface slice
		return c.getPaginatedInterfaceSlice(ctx, path, query, options, typedResult)
	case *interface{}:
		// Single request, store directly in result
		return c.doRequest(ctx, requestOptions{
			Method: "GET",
			Path:   path,
			Query:  query,
		}, result)
	default:
		// For other types, just make a single request
		c.logger.Warn("Unsupported result type for pagination", map[string]interface{}{
			"type": fmt.Sprintf("%T", result),
			"path": path,
		})
		return c.doRequest(ctx, requestOptions{
			Method: "GET",
			Path:   path,
			Query:  query,
		}, result)
	}
}

// getPaginatedMapSlice handles pagination for []map[string]interface{} result type
func (c *RESTClient) getPaginatedMapSlice(ctx context.Context, path string, query url.Values, options *PaginationOptions, result *[]map[string]interface{}) error {
	var allItems []map[string]interface{}
	
	// Track current page
	currentPage := options.Page
	maxPages := options.MaxPages
	
	for currentPage <= maxPages {
		// Update page parameter
		query.Set("page", strconv.Itoa(currentPage))
		
		// Make request
		var items []map[string]interface{}
		err := c.doRequest(ctx, requestOptions{
			Method: "GET",
			Path:   path,
			Query:  query,
		}, &items)
		
		if err != nil {
			return err
		}
		
		// Break if no items returned
		if len(items) == 0 {
			break
		}
		
		// Append items to result
		allItems = append(allItems, items...)
		
		// Check if we've reached the last page
		if len(items) < options.PerPage {
			break
		}
		
		// Increment page
		currentPage++
		
		// Add slight delay between requests to avoid rate limiting
		if currentPage <= maxPages {
			time.Sleep(100 * time.Millisecond)
		}
	}
	
	// Set result
	*result = allItems
	
	return nil
}

// getPaginatedInterfaceSlice handles pagination for []interface{} result type
func (c *RESTClient) getPaginatedInterfaceSlice(ctx context.Context, path string, query url.Values, options *PaginationOptions, result *[]interface{}) error {
	var allItems []interface{}
	
	// Track current page
	currentPage := options.Page
	maxPages := options.MaxPages
	
	for currentPage <= maxPages {
		// Update page parameter
		query.Set("page", strconv.Itoa(currentPage))
		
		// Make request
		var items []interface{}
		err := c.doRequest(ctx, requestOptions{
			Method: "GET",
			Path:   path,
			Query:  query,
		}, &items)
		
		if err != nil {
			return err
		}
		
		// Break if no items returned
		if len(items) == 0 {
			break
		}
		
		// Append items to result
		allItems = append(allItems, items...)
		
		// Check if we've reached the last page
		if len(items) < options.PerPage {
			break
		}
		
		// Increment page
		currentPage++
		
		// Add slight delay between requests to avoid rate limiting
		if currentPage <= maxPages {
			time.Sleep(100 * time.Millisecond)
		}
	}
	
	// Set result
	*result = allItems
	
	return nil
}

// Get makes a GET request
func (c *RESTClient) Get(ctx context.Context, path string, query url.Values, result interface{}) error {
	// Set up headers
	headers := make(map[string]string)
	
	// Add ETag header for conditional request if we have a cached ETag
	if etag := c.getETag(path); etag != "" {
		headers["If-None-Match"] = etag
	}
	
	return c.doRequest(ctx, requestOptions{
		Method:  "GET",
		Path:    path,
		Query:   query,
		Headers: headers,
	}, result)
}

// Post makes a POST request
func (c *RESTClient) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, requestOptions{
		Method: "POST",
		Path:   path,
		Body:   body,
	}, result)
}

// Patch makes a PATCH request
func (c *RESTClient) Patch(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, requestOptions{
		Method: "PATCH",
		Path:   path,
		Body:   body,
	}, result)
}

// Put makes a PUT request
func (c *RESTClient) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, requestOptions{
		Method: "PUT",
		Path:   path,
		Body:   body,
	}, result)
}

// Delete makes a DELETE request
func (c *RESTClient) Delete(ctx context.Context, path string) error {
	return c.doRequest(ctx, requestOptions{
		Method: "DELETE",
		Path:   path,
	}, nil)
}

// Upload uploads a file
func (c *RESTClient) Upload(ctx context.Context, path string, filename string, contentType string, data []byte, result interface{}) error {
	// Create multipart request
	body := &bytes.Buffer{}
	body.Write(data)
	
	return c.doRequest(ctx, requestOptions{
		Method: "POST",
		Path:   path,
		Body:   body,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		IsUpload: true,
	}, result)
}

// Common Repository Operations

// GetRepository gets a repository by owner and name
func (c *RESTClient) GetRepository(ctx context.Context, owner, repo string) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s", owner, repo)
	var result map[string]interface{}
	err := c.Get(ctx, path, nil, &result)
	return result, err
}

// ListRepositories lists repositories for the authenticated user
func (c *RESTClient) ListRepositories(ctx context.Context, options *PaginationOptions) ([]map[string]interface{}, error) {
	path := "user/repos"
	var result []map[string]interface{}
	err := c.GetPaginated(ctx, path, options, &result)
	return result, err
}

// Common Issue Operations

// GetIssue gets an issue by number
func (c *RESTClient) GetIssue(ctx context.Context, owner, repo string, number int) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, number)
	var result map[string]interface{}
	err := c.Get(ctx, path, nil, &result)
	return result, err
}

// ListIssues lists issues for a repository
func (c *RESTClient) ListIssues(ctx context.Context, owner, repo string, options *PaginationOptions) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/issues", owner, repo)
	var result []map[string]interface{}
	err := c.GetPaginated(ctx, path, options, &result)
	return result, err
}

// CreateIssue creates a new issue
func (c *RESTClient) CreateIssue(ctx context.Context, owner, repo string, issue map[string]interface{}) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/issues", owner, repo)
	var result map[string]interface{}
	err := c.Post(ctx, path, issue, &result)
	return result, err
}

// UpdateIssue updates an issue
func (c *RESTClient) UpdateIssue(ctx context.Context, owner, repo string, number int, issue map[string]interface{}) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, number)
	var result map[string]interface{}
	err := c.Patch(ctx, path, issue, &result)
	return result, err
}

// Common Pull Request Operations

// GetPullRequest gets a pull request by number
func (c *RESTClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number)
	var result map[string]interface{}
	err := c.Get(ctx, path, nil, &result)
	return result, err
}

// ListPullRequests lists pull requests for a repository
func (c *RESTClient) ListPullRequests(ctx context.Context, owner, repo string, options *PaginationOptions) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls", owner, repo)
	var result []map[string]interface{}
	err := c.GetPaginated(ctx, path, options, &result)
	return result, err
}

// CreatePullRequest creates a new pull request
func (c *RESTClient) CreatePullRequest(ctx context.Context, owner, repo string, pr map[string]interface{}) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls", owner, repo)
	var result map[string]interface{}
	err := c.Post(ctx, path, pr, &result)
	return result, err
}

// MergePullRequest merges a pull request
func (c *RESTClient) MergePullRequest(ctx context.Context, owner, repo string, number int, mergeMethod string) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/merge", owner, repo, number)
	body := map[string]interface{}{
		"merge_method": mergeMethod,
	}
	var result map[string]interface{}
	err := c.Put(ctx, path, body, &result)
	return result, err
}

// Common Workflow Operations

// ListWorkflowRuns lists workflow runs for a repository
func (c *RESTClient) ListWorkflowRuns(ctx context.Context, owner, repo string, options *PaginationOptions) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs", owner, repo)
	var result map[string]interface{}
	var runs []map[string]interface{}
	
	err := c.GetPaginated(ctx, path, options, &result)
	if err != nil {
		return nil, err
	}
	
	// Extract workflow runs from response
	if workflow_runs, ok := result["workflow_runs"].([]interface{}); ok {
		for _, run := range workflow_runs {
			if runMap, ok := run.(map[string]interface{}); ok {
				runs = append(runs, runMap)
			}
		}
	}
	
	return runs, nil
}

// GetWorkflowRun gets a workflow run by ID
func (c *RESTClient) GetWorkflowRun(ctx context.Context, owner, repo string, runID int64) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs/%d", owner, repo, runID)
	var result map[string]interface{}
	err := c.Get(ctx, path, nil, &result)
	return result, err
}

// TriggerWorkflow triggers a workflow dispatch event
func (c *RESTClient) TriggerWorkflow(ctx context.Context, owner, repo, workflowID, ref string, inputs map[string]interface{}) error {
	path := fmt.Sprintf("repos/%s/%s/actions/workflows/%s/dispatches", owner, repo, workflowID)
	body := map[string]interface{}{
		"ref":    ref,
		"inputs": inputs,
	}
	return c.Post(ctx, path, body, nil)
}

// Common Comment Operations

// CreateIssueComment creates a comment on an issue
func (c *RESTClient) CreateIssueComment(ctx context.Context, owner, repo string, number int, comment string) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, number)
	body := map[string]interface{}{
		"body": comment,
	}
	var result map[string]interface{}
	err := c.Post(ctx, path, body, &result)
	return result, err
}

// CreatePullRequestReview creates a review on a pull request
func (c *RESTClient) CreatePullRequestReview(ctx context.Context, owner, repo string, number int, review map[string]interface{}) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, number)
	var result map[string]interface{}
	err := c.Post(ctx, path, review, &result)
	return result, err
}

// Common Release Operations

// ListReleases lists releases for a repository
func (c *RESTClient) ListReleases(ctx context.Context, owner, repo string, options *PaginationOptions) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/releases", owner, repo)
	var result []map[string]interface{}
	err := c.GetPaginated(ctx, path, options, &result)
	return result, err
}

// CreateRelease creates a new release
func (c *RESTClient) CreateRelease(ctx context.Context, owner, repo string, release map[string]interface{}) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/releases", owner, repo)
	var result map[string]interface{}
	err := c.Post(ctx, path, release, &result)
	return result, err
}

// UploadReleaseAsset uploads an asset to a release
func (c *RESTClient) UploadReleaseAsset(ctx context.Context, owner, repo string, releaseID int64, filename, contentType string, data []byte) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/releases/%d/assets?name=%s", owner, repo, releaseID, url.QueryEscape(filename))
	var result map[string]interface{}
	err := c.Upload(ctx, path, filename, contentType, data, &result)
	return result, err
}

// Common Branch Operations

// ListBranches lists branches for a repository
func (c *RESTClient) ListBranches(ctx context.Context, owner, repo string, options *PaginationOptions) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/branches", owner, repo)
	var result []map[string]interface{}
	err := c.GetPaginated(ctx, path, options, &result)
	return result, err
}

// GetBranch gets a branch by name
func (c *RESTClient) GetBranch(ctx context.Context, owner, repo, branch string) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/branches/%s", owner, repo, branch)
	var result map[string]interface{}
	err := c.Get(ctx, path, nil, &result)
	return result, err
}

// Helper methods for ETag and rate limit handling

// extractRateLimitInfo extracts rate limit information from response headers
func (c *RESTClient) extractRateLimitInfo(headers http.Header) *RateLimitInfo {
	limit, _ := strconv.Atoi(headers.Get("X-RateLimit-Limit"))
	remaining, _ := strconv.Atoi(headers.Get("X-RateLimit-Remaining"))
	resetStr := headers.Get("X-RateLimit-Reset")
	used, _ := strconv.Atoi(headers.Get("X-RateLimit-Used"))
	
	// If any of the required headers are missing, return nil
	if limit == 0 && remaining == 0 && resetStr == "" {
		return nil
	}
	
	// Parse reset timestamp
	resetTimestamp, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		c.logger.Warn("Failed to parse rate limit reset timestamp", "error", err)
		return nil
	}
	
	reset := time.Unix(resetTimestamp, 0)
	
	return &RateLimitInfo{
		Limit:     limit,
		Remaining: remaining,
		Reset:     reset,
		Used:      used,
	}
}

// storeETag stores an ETag for a specific path
func (c *RESTClient) storeETag(path string, etag string) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.etagCache[path] = etag
}

// getETag retrieves an ETag for a specific path
func (c *RESTClient) getETag(path string) string {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()
	return c.etagCache[path]
}

// storeCachedResponse stores a response for a specific path
func (c *RESTClient) storeCachedResponse(path string, response interface{}) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.responseCache[path] = response
}

// loadCachedResponse loads a cached response for a specific path
func (c *RESTClient) loadCachedResponse(path string) interface{} {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()
	return c.responseCache[path]
}

// clearCachedResponses clears all cached responses
func (c *RESTClient) clearCachedResponses() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.responseCache = make(map[string]interface{})
	c.etagCache = make(map[string]string)
}

// Common Commit Operations

// GetCommit gets a commit by SHA
func (c *RESTClient) GetCommit(ctx context.Context, owner, repo, sha string) (map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/commits/%s", owner, repo, sha)
	var result map[string]interface{}
	err := c.Get(ctx, path, nil, &result)
	return result, err
}

// ListCommits lists commits for a repository
func (c *RESTClient) ListCommits(ctx context.Context, owner, repo string, options *PaginationOptions) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("repos/%s/%s/commits", owner, repo)
	var result []map[string]interface{}
	err := c.GetPaginated(ctx, path, options, &result)
	return result, err
}

// Common Content Operations

// GetContent gets the content of a file
func (c *RESTClient) GetContent(ctx context.Context, owner, repo, path, ref string) (map[string]interface{}, error) {
	apiPath := fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, path)
	
	// Add ref parameter if provided
	query := url.Values{}
	if ref != "" {
		query.Set("ref", ref)
	}
	
	var result map[string]interface{}
	err := c.Get(ctx, apiPath, query, &result)
	return result, err
}

// CreateOrUpdateContent creates or updates a file
func (c *RESTClient) CreateOrUpdateContent(ctx context.Context, owner, repo, path string, content map[string]interface{}) (map[string]interface{}, error) {
	apiPath := fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, path)
	var result map[string]interface{}
	err := c.Put(ctx, apiPath, content, &result)
	return result, err
}
