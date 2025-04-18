package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

// Constants for API and rate limiting
const (
	defaultAPIVersion      = "2022-11-28"
	minRateLimitRemaining  = 10
	defaultRetryMax        = 3
	defaultRetryDelay      = 1 * time.Second
	defaultRequestTimeout  = 30 * time.Second
	defaultConcurrency     = 5
	defaultDefaultPerPage  = 100
)

// Config holds configuration for the GitHub adapter
type Config struct {
	APIToken                string        `mapstructure:"api_token"`
	WebhookSecret           string        `mapstructure:"webhook_secret"`
	EnterpriseURL           string        `mapstructure:"enterprise_url"`
	APIVersion              string        `mapstructure:"api_version"`
	RequestTimeout          time.Duration `mapstructure:"request_timeout"`
	RetryMax                int           `mapstructure:"retry_max"`
	RetryDelay              time.Duration `mapstructure:"retry_delay"`
	MockResponses           bool          `mapstructure:"mock_responses"`
	MockURL                 string        `mapstructure:"mock_url"`
	RateLimitThreshold      int64         `mapstructure:"rate_limit_threshold"`
	DefaultPerPage          int           `mapstructure:"default_per_page"`
	EnableRetryOnRateLimit  bool          `mapstructure:"enable_retry_on_rate_limit"`
	Concurrency             int           `mapstructure:"concurrency"`
	LogRequests             bool          `mapstructure:"log_requests"`
	SafeMode                bool          `mapstructure:"safe_mode"`
}

// Adapter implements the adapter interface for GitHub
type Adapter struct {
	adapters.BaseAdapter
	config        Config
	client        *github.Client
	httpClient    *http.Client
	subscribers   map[string][]func(interface{})
	healthStatus  string
	rateLimits    *github.RateLimits
	lastRateCheck time.Time
	stats         *AdapterStats
}

// AdapterStats tracks usage statistics of the GitHub adapter
type AdapterStats struct {
	RequestsTotal          int64
	RequestsSuccess        int64
	RequestsFailed         int64
	RequestsRetried        int64
	RateLimitHits          int64
	LastError              string
	LastErrorTime          time.Time
	LastSuccessfulRequest  time.Time
	AverageResponseTime    time.Duration
	TotalResponseTime      time.Duration
	ResponseTimeCount      int64
	SuccessfulOperations   map[string]int64
	FailedOperations       map[string]int64
}

// NewAdapter creates a new GitHub adapter
func NewAdapter(config Config) (*Adapter, error) {
	// Set default values if not provided
	if config.RequestTimeout == 0 {
		config.RequestTimeout = defaultRequestTimeout
	}
	if config.RetryMax == 0 {
		config.RetryMax = defaultRetryMax
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = defaultRetryDelay
	}
	if config.APIVersion == "" {
		config.APIVersion = defaultAPIVersion
	}
	if config.RateLimitThreshold == 0 {
		config.RateLimitThreshold = minRateLimitRemaining
	}
	if config.DefaultPerPage == 0 {
		config.DefaultPerPage = defaultDefaultPerPage
	}
	if config.Concurrency == 0 {
		config.Concurrency = defaultConcurrency
	}

	adapter := &Adapter{
		BaseAdapter: adapters.BaseAdapter{
			RetryMax:   config.RetryMax,
			RetryDelay: config.RetryDelay,
			SafeMode:   config.SafeMode,
		},
		config:        config,
		subscribers:   make(map[string][]func(interface{})),
		healthStatus:  "initializing",
		stats: &AdapterStats{
			SuccessfulOperations: make(map[string]int64),
			FailedOperations:     make(map[string]int64),
		},
		lastRateCheck: time.Time{},
	}

	return adapter, nil
}

// Initialize sets up the adapter with the GitHub client
func (a *Adapter) Initialize(ctx context.Context, cfg interface{}) error {
	// Parse config if provided
	if cfg != nil {
		config, ok := cfg.(Config)
		if !ok {
			return fmt.Errorf("invalid config type: %T", cfg)
		}
		a.config = config
	}

	// Validate configuration
	if a.config.APIToken == "" {
		return fmt.Errorf("GitHub API token is required")
	}

	// Create OAuth2 client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: a.config.APIToken},
	)
	a.httpClient = &http.Client{Timeout: a.config.RequestTimeout}
	tc := oauth2.NewClient(ctx, ts)
	tc.Timeout = a.config.RequestTimeout

	// Create a logging transport if configured
	var baseTransport http.RoundTripper = http.DefaultTransport
	if a.config.LogRequests {
		baseTransport = &loggingTransport{
			base: baseTransport,
		}
	}

	// Wrap the transport to add API version header
	if transport, ok := tc.Transport.(*oauth2.Transport); ok {
		transport.Base = &headerTransport{
			base:       baseTransport,
			token:      a.config.APIToken,
			apiVersion: a.config.APIVersion,
		}
		tc.Transport = transport
	}

	// Create GitHub client
	if a.config.EnterpriseURL != "" {
		client, err := github.NewEnterpriseClient(a.config.EnterpriseURL, a.config.EnterpriseURL, tc)
		if err != nil {
			a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
			return err
		}
		a.client = client
	} else {
		a.client = github.NewClient(tc)
	}
	
	// Test the connection and check rate limits
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		return err
	}

	// Check rate limits
	if err := a.updateRateLimits(ctx); err != nil {
		log.Printf("Warning: Failed to fetch rate limits: %v", err)
	}

	a.healthStatus = "healthy"
	return nil
}

// updateRateLimits fetches the current rate limit status from GitHub
func (a *Adapter) updateRateLimits(ctx context.Context) error {
	// Skip if we've checked recently (within the last minute)
	if !a.lastRateCheck.IsZero() && time.Since(a.lastRateCheck) < time.Minute {
		return nil
	}

	rateLimits, _, err := a.client.RateLimits(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch rate limits: %w", err)
	}

	a.rateLimits = rateLimits
	a.lastRateCheck = time.Now()

	// Log rate limit status
	core := rateLimits.GetCore()
	log.Printf("GitHub API Rate Limits: %d/%d remaining. Resets at %s", 
		core.Remaining, core.Limit, core.Reset.Format(time.RFC3339))

	// Check if we're approaching the rate limit
	if int64(core.Remaining) < a.config.RateLimitThreshold {
		log.Printf("Warning: GitHub API rate limit threshold reached (%d/%d remaining)",
			core.Remaining, core.Limit)
		resetTime := time.Until(core.Reset.Time)
		log.Printf("Rate limit will reset in %s", resetTime.String())
		a.stats.RateLimitHits++
	}

	return nil
}

// loggingTransport is a custom transport that logs HTTP requests
type loggingTransport struct {
	base http.RoundTripper
}

// RoundTrip logs the request and the response
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	log.Printf("GitHub API Request: %s %s", req.Method, req.URL.String())
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	duration := time.Since(start)
	
	if err != nil {
		log.Printf("GitHub API Error: %v (took %s)", err, duration)
		return resp, err
	}
	
	log.Printf("GitHub API Response: %d %s (took %s)", 
		resp.StatusCode, resp.Status, duration)
	return resp, nil
}

// headerTransport is a custom transport that adds headers to all requests
type headerTransport struct {
	base       http.RoundTripper
	token      string
	apiVersion string
}

// RoundTrip adds the required GitHub API headers to the request
func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", t.apiVersion)
	
	// Add a user agent if not present
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "MCP-GitHub-Adapter")
	}
	
	// Set content type for POST/PUT/PATCH requests if not already set
	if (req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH") && 
	   req.Header.Get("Content-Type") == "" && req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return t.base.RoundTrip(req)
}

// testConnection verifies connectivity to GitHub
func (a *Adapter) testConnection(ctx context.Context) error {
	// If mock_responses is enabled, try connecting to the mock server instead
	if a.config.MockResponses && a.config.MockURL != "" {
		// Create a custom HTTP client for testing the mock connection
		httpClient := &http.Client{Timeout: a.config.RequestTimeout}
		
		// First try health endpoint which should be more reliable
		healthURL := a.config.MockURL + "/health"
		req, err := http.NewRequest("GET", healthURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		// Add required headers
		req.Header.Set("Authorization", "Bearer "+a.config.APIToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", a.config.APIVersion)
		req.Header.Set("User-Agent", "MCP-GitHub-Adapter")
		
		resp, err := httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			log.Println("Successfully connected to mock GitHub API health endpoint")
			// Set health status to healthy
			a.healthStatus = "healthy"
			return nil
		}
		
		// Fall back to rate_limit endpoint
		req, err = http.NewRequest("GET", a.config.MockURL+"/rate_limit", nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		// Add required headers
		req.Header.Set("Authorization", "Bearer "+a.config.APIToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", a.config.APIVersion)
		req.Header.Set("User-Agent", "MCP-GitHub-Adapter")
		
		resp, err = httpClient.Do(req)
		if err != nil {
			a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to mock GitHub API: %v", err)
			// Don't return error, just make the adapter usable in degraded mode
			log.Printf("Warning: Failed to connect to mock GitHub API: %v", err)
			a.stats.LastError = err.Error()
			a.stats.LastErrorTime = time.Now()
			a.stats.RequestsFailed++
			return nil
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			a.healthStatus = fmt.Sprintf("unhealthy: mock GitHub API returned status code: %d", resp.StatusCode)
			// Don't return error, just make the adapter usable in degraded mode
			log.Printf("Warning: Mock GitHub API returned unexpected status code: %d", resp.StatusCode)
			a.stats.LastError = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
			a.stats.LastErrorTime = time.Now()
			a.stats.RequestsFailed++
			return nil
		}
		
		// Successfully connected to mock server
		log.Println("Successfully connected to mock GitHub API")
		a.healthStatus = "healthy"
		a.stats.RequestsSuccess++
		a.stats.LastSuccessfulRequest = time.Now()
		return nil
	}
	
	// Use the actual GitHub API for verification if not in mock mode
	// First check if we're approaching rate limit and if we should wait
	if a.rateLimits != nil && a.rateLimits.Core != nil {
		core := a.rateLimits.GetCore()
		if int64(core.Remaining) < a.config.RateLimitThreshold {
			resetTime := time.Until(core.Reset.Time)
			if resetTime > 0 && a.config.EnableRetryOnRateLimit {
				log.Printf("Rate limit threshold reached. Waiting %s for reset...", resetTime)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(resetTime + time.Second): // Add a second buffer
					// Continue after waiting
				}
			}
		}
	}
	
	// Update rate limits
	rateLimits, resp, err := a.client.RateLimits(ctx)
	if err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		// Log the error details
		a.stats.LastError = err.Error()
		a.stats.LastErrorTime = time.Now()
		a.stats.RequestsFailed++
		
		// Check if this is a rate limit error
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			// Try to parse the response body for rate limit information
			body, readErr := io.ReadAll(resp.Body)
			if readErr == nil {
				if strings.Contains(string(body), "rate limit exceeded") {
					log.Printf("Rate limit exceeded. Check your limits and retry after the reset time.")
					a.stats.RateLimitHits++
				}
			}
		}
		
		// Don't return error, just make the adapter usable in degraded mode
		log.Printf("Warning: Failed to connect to GitHub API: %v", err)
		return nil
	}
	
	// Store the rate limits for future reference
	a.rateLimits = rateLimits
	a.lastRateCheck = time.Now()
	core := rateLimits.GetCore()
	log.Printf("GitHub API rate limits: %d/%d remaining. Resets at %s", 
		core.Remaining, core.Limit, core.Reset.Format(time.RFC3339))
	
	a.healthStatus = "healthy"
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()
	return nil
}

// GetData retrieves data from GitHub
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	// Parse the query
	queryMap, ok := query.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid query type: %T", query)
	}

	// Check the operation type
	operation, ok := queryMap["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("missing operation in query")
	}

	// Track total request count
	a.stats.RequestsTotal++
	
	// Check rate limits before proceeding
	if err := a.updateRateLimits(ctx); err == nil && a.rateLimits != nil {
		// If rate limits are critically low and we're not configured to retry,
		// return an error instead of making the API call
		if a.rateLimits.Core != nil && 
		   int64(a.rateLimits.Core.Remaining) < a.config.RateLimitThreshold && 
		   !a.config.EnableRetryOnRateLimit {
			resetTime := time.Until(a.rateLimits.Core.Reset.Time)
			return nil, fmt.Errorf("GitHub API rate limit threshold reached (%d/%d remaining). Resets in %s",
				a.rateLimits.Core.Remaining, a.rateLimits.Core.Limit, resetTime)
		}
	}

	startTime := time.Now()
	var result interface{}
	var err error

	// Handle different operations
	switch operation {
	case "get_repositories":
		result, err = a.getRepositories(ctx, queryMap)
	case "get_pull_requests":
		result, err = a.getPullRequests(ctx, queryMap)
	case "get_issues":
		result, err = a.getIssues(ctx, queryMap)
	case "get_repository":
		result, err = a.getRepository(ctx, queryMap)
	case "get_branches":
		result, err = a.getBranches(ctx, queryMap)
	case "get_teams":
		result, err = a.getTeams(ctx, queryMap)
	case "get_workflow_runs":
		result, err = a.getWorkflowRuns(ctx, queryMap)
	case "get_commits":
		result, err = a.getCommits(ctx, queryMap)
	case "search_code":
		result, err = a.searchCode(ctx, queryMap)
	case "get_users":
		result, err = a.getUsers(ctx, queryMap)
	case "get_webhooks":
		result, err = a.getWebhooks(ctx, queryMap)
	case "get_adapter_stats":
		result, err = a.getAdapterStats(ctx, queryMap)
	case "search_repositories":
		result, err = a.searchRepositories(ctx, queryMap)
	case "get_workflows":
		result, err = a.getWorkflows(ctx, queryMap)
	case "get_required_workflow_approvals":
		result, err = a.getRequiredWorkflowApprovals(ctx, queryMap)
	default:
		err = fmt.Errorf("unsupported operation: %s", operation)
	}

	duration := time.Since(startTime)
	
	// Update stats
	if err != nil {
		a.stats.RequestsFailed++
		a.stats.FailedOperations[operation]++
		a.stats.LastError = err.Error()
		a.stats.LastErrorTime = time.Now()
	} else {
		a.stats.RequestsSuccess++
		a.stats.SuccessfulOperations[operation]++
		a.stats.LastSuccessfulRequest = time.Now()
		
		// Update response time stats
		a.stats.TotalResponseTime += duration
		a.stats.ResponseTimeCount++
		a.stats.AverageResponseTime = time.Duration(int64(a.stats.TotalResponseTime) / a.stats.ResponseTimeCount)
	}

	return result, err
}

// getRepositories retrieves repositories based on the provided parameters
func (a *Adapter) getRepositories(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	var owner string
	if v, ok := params["owner"].(string); ok && v != "" {
		owner = v
	}

	var visibility string
	if v, ok := params["visibility"].(string); ok && v != "" {
		visibility = v
	}

	var sort string
	if v, ok := params["sort"].(string); ok && v != "" {
		sort = v
	}

	var direction string
	if v, ok := params["direction"].(string); ok && v != "" {
		direction = v
	}

	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.RepositoryListOptions{
		Visibility:  visibility,
		Sort:        sort,
		Direction:   direction,
		ListOptions: github.ListOptions{Page: page, PerPage: pageSize},
	}

	// Fetch repositories
	var repositories []*github.Repository
	var err error
	var resp *github.Response

	if owner == "" {
		// Get repositories for the authenticated user
		repositories, resp, err = a.client.Repositories.List(ctx, "", listOptions)
	} else {
		// Get repositories for the specified organization
		// Convert to RepositoryListByOrgOptions
		orgOptions := &github.RepositoryListByOrgOptions{
			Type:        visibility,
			Sort:        sort,
			Direction:   direction,
			ListOptions: github.ListOptions{Page: page, PerPage: pageSize},
		}
		repositories, resp, err = a.client.Repositories.ListByOrg(ctx, owner, orgOptions)
	}

	// Handle errors
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch repositories")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(repositories))
	for _, repo := range repositories {
		repoMap := map[string]interface{}{
			"id":             repo.ID,
			"name":           repo.Name,
			"full_name":      repo.FullName,
			"description":    repo.Description,
			"private":        repo.Private,
			"language":       repo.Language,
			"default_branch": repo.DefaultBranch,
			"html_url":       repo.HTMLURL,
			"created_at":     repo.CreatedAt,
			"updated_at":     repo.UpdatedAt,
			"pushed_at":      repo.PushedAt,
			"size":           repo.Size,
			"visibility":     repo.Visibility,
		}
		result = append(result, repoMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"repositories": result,
		"page":         page,
		"per_page":     pageSize,
		"has_next":     resp.NextPage != 0,
		"next_page":    resp.NextPage,
		"total_count":  len(repositories), // GitHub doesn't provide total count, this is just current page
	}, nil
}

// searchRepositories allows searching for repositories using the GitHub search API
func (a *Adapter) searchRepositories(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract query parameter (required)
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	// Extract optional parameters with defaults
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	var sort string
	if v, ok := params["sort"].(string); ok && v != "" {
		sort = v
	}

	var order string
	if v, ok := params["order"].(string); ok && v != "" {
		order = v
	}

	// Create search options
	searchOptions := &github.SearchOptions{
		Sort:        sort,
		Order:       order,
		ListOptions: github.ListOptions{Page: page, PerPage: pageSize},
	}

	// Perform the search
	searchResult, resp, err := a.client.Search.Repositories(ctx, query, searchOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to search repositories")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(searchResult.Repositories))
	for _, repo := range searchResult.Repositories {
		repoMap := map[string]interface{}{
			"id":             repo.ID,
			"name":           repo.Name,
			"full_name":      repo.FullName,
			"description":    repo.Description,
			"private":        repo.Private,
			"language":       repo.Language,
			"default_branch": repo.DefaultBranch,
			"html_url":       repo.HTMLURL,
			"created_at":     repo.CreatedAt,
			"updated_at":     repo.UpdatedAt,
			"pushed_at":      repo.PushedAt,
			"size":           repo.Size,
			"visibility":     repo.Visibility,
		}
		result = append(result, repoMap)
	}

	// Build result with pagination info
	return map[string]interface{}{
		"items":       result,
		"total_count": searchResult.GetTotal(),
		"page":        page,
		"per_page":    pageSize,
		"has_next":    resp.NextPage != 0,
		"next_page":   resp.NextPage,
		"query":       query,
	}, nil
}

// getPullRequests retrieves pull requests for a repository
func (a *Adapter) getPullRequests(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional parameters
	var state string
	if v, ok := params["state"].(string); ok && v != "" {
		state = v
	} else {
		state = "open" // Default to open PRs
	}

	var sort string
	if v, ok := params["sort"].(string); ok && v != "" {
		sort = v
	} else {
		sort = "created" // Default sort by created date
	}

	var direction string
	if v, ok := params["direction"].(string); ok && v != "" {
		direction = v
	} else {
		direction = "desc" // Default to newest first
	}

	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.PullRequestListOptions{
		State:       state,
		Sort:        sort,
		Direction:   direction,
		ListOptions: github.ListOptions{Page: page, PerPage: pageSize},
	}

	// Fetch pull requests
	prs, resp, err := a.client.PullRequests.List(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch pull requests")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(prs))
	for _, pr := range prs {
		prMap := map[string]interface{}{
			"id":             pr.ID,
			"number":         pr.Number,
			"title":          pr.Title,
			"state":          pr.State,
			"html_url":       pr.HTMLURL,
			"created_at":     pr.CreatedAt,
			"updated_at":     pr.UpdatedAt,
			"closed_at":      pr.ClosedAt,
			"merged_at":      pr.MergedAt,
			"base_ref":       pr.Base.Ref,
			"head_ref":       pr.Head.Ref,
			"user_login":     pr.User.Login,
			"merged":         pr.Merged,
			"mergeable":      pr.Mergeable,
			"draft":          pr.Draft,
			"body":           pr.Body,
			"comments_count": pr.Comments,
			"commits_count":  pr.Commits,
			"labels":         pr.Labels,
		}
		result = append(result, prMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"pull_requests": result,
		"page":          page,
		"per_page":      pageSize,
		"has_next":      resp.NextPage != 0,
		"next_page":     resp.NextPage,
		"repository":    owner + "/" + repo,
	}, nil
}

// getIssues retrieves issues for a repository
func (a *Adapter) getIssues(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional parameters
	var state string
	if v, ok := params["state"].(string); ok && v != "" {
		state = v
	} else {
		state = "open" // Default to open issues
	}

	var sort string
	if v, ok := params["sort"].(string); ok && v != "" {
		sort = v
	} else {
		sort = "created" // Default sort by created date
	}

	var direction string
	if v, ok := params["direction"].(string); ok && v != "" {
		direction = v
	} else {
		direction = "desc" // Default to newest first
	}

	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.IssueListByRepoOptions{
		State:       state,
		Sort:        sort,
		Direction:   direction,
		ListOptions: github.ListOptions{Page: page, PerPage: pageSize},
	}

	// Fetch issues
	issues, resp, err := a.client.Issues.ListByRepo(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch issues")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(issues))
	for _, issue := range issues {
		// Skip pull requests (GitHub returns them as issues, but we want to handle them separately)
		if issue.IsPullRequest() {
			continue
		}

		issueMap := map[string]interface{}{
			"id":             issue.ID,
			"number":         issue.Number,
			"title":          issue.Title,
			"state":          issue.State,
			"html_url":       issue.HTMLURL,
			"created_at":     issue.CreatedAt,
			"updated_at":     issue.UpdatedAt,
			"closed_at":      issue.ClosedAt,
			"user_login":     issue.User.Login,
			"body":           issue.Body,
			"comments_count": issue.Comments,
			"labels":         issue.Labels,
			"assignees":      issue.Assignees,
			"milestone":      issue.Milestone,
		}
		result = append(result, issueMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"issues":     result,
		"page":       page,
		"per_page":   pageSize,
		"has_next":   resp.NextPage != 0,
		"next_page":  resp.NextPage,
		"repository": owner + "/" + repo,
	}, nil
}

// getRepository retrieves detailed information about a single repository
func (a *Adapter) getRepository(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Fetch repository
	repository, resp, err := a.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch repository")
	}

	// Convert to a map
	result := map[string]interface{}{
		"id":                 repository.ID,
		"name":               repository.Name,
		"full_name":          repository.FullName,
		"description":        repository.Description,
		"private":            repository.Private,
		"fork":               repository.Fork,
		"language":           repository.Language,
		"size":               repository.Size,
		"default_branch":     repository.DefaultBranch,
		"html_url":           repository.HTMLURL,
		"clone_url":          repository.CloneURL,
		"ssh_url":            repository.SSHURL,
		"git_url":            repository.GitURL,
		"created_at":         repository.CreatedAt,
		"updated_at":         repository.UpdatedAt,
		"pushed_at":          repository.PushedAt,
		"open_issues_count":  repository.OpenIssuesCount,
		"forks_count":        repository.ForksCount,
		"stargazers_count":   repository.StargazersCount,
		"watchers_count":     repository.WatchersCount,
		"subscribers_count":  repository.SubscribersCount,
		"network_count":      repository.NetworkCount,
		"has_wiki":           repository.HasWiki,
		"has_pages":          repository.HasPages,
		"has_issues":         repository.HasIssues,
		"has_projects":       repository.HasProjects,
		"has_downloads":      repository.HasDownloads,
		"archived":           repository.Archived,
		"disabled":           repository.Disabled,
		"visibility":         repository.Visibility,
		"license":            repository.License,
		"topics":             repository.Topics,
		"owner": map[string]interface{}{
			"id":        repository.Owner.ID,
			"login":     repository.Owner.Login,
			"type":      repository.Owner.Type,
			"avatar_url": repository.Owner.AvatarURL,
			"html_url":  repository.Owner.HTMLURL,
		},
	}

	return result, nil
}

// getBranches retrieves branches for a repository
func (a *Adapter) getBranches(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional parameters
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.BranchListOptions{
		ListOptions: github.ListOptions{Page: page, PerPage: pageSize},
	}

	// If protected parameter is provided, add it to options
	if protected, ok := params["protected"].(bool); ok {
		listOptions.Protected = &protected
	}

	// Fetch branches
	branches, resp, err := a.client.Repositories.ListBranches(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch branches")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(branches))
	for _, branch := range branches {
		branchMap := map[string]interface{}{
			"name":      branch.Name,
			"protected": branch.Protected,
		}

		// Include commit information if available
		if branch.Commit != nil {
			branchMap["commit"] = map[string]interface{}{
				"sha":     branch.Commit.SHA,
				"url":     branch.Commit.URL,
				"html_url": branch.Commit.HTMLURL,
			}
		}

		result = append(result, branchMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"branches":   result,
		"page":       page,
		"per_page":   pageSize,
		"has_next":   resp.NextPage != 0,
		"next_page":  resp.NextPage,
		"repository": owner + "/" + repo,
	}, nil
}

// getTeams retrieves teams for an organization
func (a *Adapter) getTeams(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	org, ok := params["org"].(string)
	if !ok || org == "" {
		return nil, fmt.Errorf("org is required")
	}

	// Extract optional parameters
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.ListOptions{
		Page:    page,
		PerPage: pageSize,
	}

	// Fetch teams
	teams, resp, err := a.client.Teams.ListTeams(ctx, org, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch teams")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(teams))
	for _, team := range teams {
		teamMap := map[string]interface{}{
			"id":           team.ID,
			"name":         team.Name,
			"slug":         team.Slug,
			"description":  team.Description,
			"privacy":      team.Privacy,
			"permission":   team.Permission,
			"members_count": team.MembersCount,
			"repos_count":  team.ReposCount,
			"html_url":     team.HTMLURL,
		}
		result = append(result, teamMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"teams":         result,
		"page":          page,
		"per_page":      pageSize,
		"has_next":      resp.NextPage != 0,
		"next_page":     resp.NextPage,
		"organization":  org,
	}, nil
}

// getWorkflows retrieves workflows for a repository
func (a *Adapter) getWorkflows(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional parameters
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.ListOptions{
		Page:    page,
		PerPage: pageSize,
	}

	// Fetch workflows
	workflows, resp, err := a.client.Actions.ListWorkflows(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch workflows")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(workflows.Workflows))
	for _, workflow := range workflows.Workflows {
		workflowMap := map[string]interface{}{
			"id":         workflow.ID,
			"name":       workflow.Name,
			"path":       workflow.Path,
			"state":      workflow.State,
			"created_at": workflow.CreatedAt,
			"updated_at": workflow.UpdatedAt,
			"html_url":   workflow.HTMLURL,
			"badge_url":  workflow.BadgeURL,
		}
		result = append(result, workflowMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"workflows":    result,
		"total_count":  workflows.TotalCount,
		"page":         page,
		"per_page":     pageSize,
		"has_next":     resp.NextPage != 0,
		"next_page":    resp.NextPage,
		"repository":   owner + "/" + repo,
	}, nil
}

// getWorkflowRuns retrieves workflow runs for a repository
func (a *Adapter) getWorkflowRuns(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional parameters
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: pageSize,
		},
	}

	// Optional filter by workflow ID
	if workflowID, ok := params["workflow_id"].(float64); ok && workflowID > 0 {
		id := int64(workflowID)
		// Fetch runs for a specific workflow
		runs, resp, err := a.client.Actions.ListWorkflowRunsByID(ctx, owner, repo, id, listOptions)
		if err != nil {
			return nil, a.handleError(err, resp, "failed to fetch workflow runs")
		}

		// Convert to a more generic format
		result := make([]map[string]interface{}, 0, len(runs.WorkflowRuns))
		for _, run := range runs.WorkflowRuns {
			runMap := a.workflowRunToMap(run)
			result = append(result, runMap)
		}

		// Build pagination info
		return map[string]interface{}{
			"workflow_runs": result,
			"total_count":   runs.TotalCount,
			"page":          page,
			"per_page":      pageSize,
			"has_next":      resp.NextPage != 0,
			"next_page":     resp.NextPage,
			"repository":    owner + "/" + repo,
			"workflow_id":   id,
		}, nil
	}

	// Fetch runs for all workflows
	runs, resp, err := a.client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch workflow runs")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(runs.WorkflowRuns))
	for _, run := range runs.WorkflowRuns {
		runMap := a.workflowRunToMap(run)
		result = append(result, runMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"workflow_runs": result,
		"total_count":   runs.TotalCount,
		"page":          page,
		"per_page":      pageSize,
		"has_next":      resp.NextPage != 0,
		"next_page":     resp.NextPage,
		"repository":    owner + "/" + repo,
	}, nil
}

// getRequiredWorkflowApprovals retrieves approval status for a workflow run
func (a *Adapter) getRequiredWorkflowApprovals(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	runID, ok := params["run_id"].(float64)
	if !ok || runID <= 0 {
		return nil, fmt.Errorf("run_id is required and must be a positive number")
	}

	// Get the workflow run
	workflowRun, resp, err := a.client.Actions.GetWorkflowRunByID(ctx, owner, repo, int64(runID))
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch workflow run")
	}
	
	// In the latest GitHub API, we need to use deployment protection rules instead
	// Create a simulated approval structure based on workflow run data
	
	// Create empty result for approvals
	result := make([]map[string]interface{}, 0)
	
	// Add a simulated approval if the workflow has an environment
	if workflowRun.HeadRepository != nil && workflowRun.Status != nil {
		approvalMap := map[string]interface{}{
			"environment": "production", // Default environment
			"state":       *workflowRun.Status,
			"comment":     "Workflow status information",
			"created_at":  workflowRun.CreatedAt,
		}
		
		if workflowRun.Actor != nil {
			approvalMap["user"] = map[string]interface{}{
				"id":        workflowRun.Actor.ID,
				"login":     workflowRun.Actor.Login,
				"type":      workflowRun.Actor.Type,
				"avatar_url": workflowRun.Actor.AvatarURL,
			}
		}
		
		result = append(result, approvalMap)
	}

	// Build result
	return map[string]interface{}{
		"approvals":   result,
		"run_id":      runID,
		"repository":  owner + "/" + repo,
	}, nil
}

// workflowRunToMap converts a GitHub workflow run to a map
func (a *Adapter) workflowRunToMap(run *github.WorkflowRun) map[string]interface{} {
	runMap := map[string]interface{}{
		"id":          run.ID,
		"name":        run.Name,
		"workflow_id": run.WorkflowID,
		"status":      run.Status,
		"conclusion":  run.Conclusion,
		"created_at":  run.CreatedAt,
		"updated_at":  run.UpdatedAt,
		"html_url":    run.HTMLURL,
		"event":       run.Event,
		"run_number":  run.RunNumber,
	}

	if run.HeadBranch != nil {
		runMap["head_branch"] = run.HeadBranch
	}

	if run.HeadSHA != nil {
		runMap["head_sha"] = run.HeadSHA
	}

	return runMap
}

// enableWorkflow enables a workflow
func (a *Adapter) enableWorkflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	workflowID, ok := params["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	// Enable the workflow
	resp, err := a.client.Actions.EnableWorkflowByID(ctx, owner, repo, int64(parseIDString(workflowID)))
	if err != nil {
		return nil, a.handleError(err, resp, "failed to enable workflow")
	}

	// Build result
	return map[string]interface{}{
		"success":     true,
		"workflow_id": workflowID,
		"repository":  owner + "/" + repo,
	}, nil
}

// disableWorkflow disables a workflow
func (a *Adapter) disableWorkflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	workflowID, ok := params["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	// Disable the workflow
	resp, err := a.client.Actions.DisableWorkflowByID(ctx, owner, repo, int64(parseIDString(workflowID)))
	if err != nil {
		return nil, a.handleError(err, resp, "failed to disable workflow")
	}

	// Build result
	return map[string]interface{}{
		"success":     true,
		"workflow_id": workflowID,
		"repository":  owner + "/" + repo,
	}, nil
}

// approveWorkflowRun approves a pending workflow run
func (a *Adapter) approveWorkflowRun(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	runID, ok := params["run_id"].(float64)
	if !ok || runID <= 0 {
		return nil, fmt.Errorf("run_id is required and must be a positive number")
	}

	environment, ok := params["environment"].(string)
	if !ok || environment == "" {
		return nil, fmt.Errorf("environment is required")
	}

	// Extract optional parameters
	var comment string
	if c, ok := params["comment"].(string); ok {
		comment = c
	}

	// Approve the workflow deployment (using deployment API instead of now-removed ApproveWorkflowRun)
	_, resp, err := a.client.Repositories.CreateDeploymentStatus(ctx, owner, repo, int64(runID), &github.DeploymentStatusRequest{
		State:       github.String("success"),
		Description: github.String(comment),
		Environment: github.String(environment),
	})
	if err != nil {
		return nil, a.handleError(err, resp, "failed to approve workflow run")
	}

	// Build result
	return map[string]interface{}{
		"success":     true,
		"run_id":      int64(runID),
		"environment": environment,
		"repository":  owner + "/" + repo,
	}, nil
}

// rejectWorkflowRun rejects a pending workflow run
func (a *Adapter) rejectWorkflowRun(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	runID, ok := params["run_id"].(float64)
	if !ok || runID <= 0 {
		return nil, fmt.Errorf("run_id is required and must be a positive number")
	}

	environment, ok := params["environment"].(string)
	if !ok || environment == "" {
		return nil, fmt.Errorf("environment is required")
	}

	// Extract optional parameters
	var comment string
	if c, ok := params["comment"].(string); ok {
		comment = c
	}

	// Reject the workflow deployment (using deployment API instead of now-removed RejectWorkflowRun)
	_, resp, err := a.client.Repositories.CreateDeploymentStatus(ctx, owner, repo, int64(runID), &github.DeploymentStatusRequest{
		State:       github.String("failure"),
		Description: github.String(comment),
		Environment: github.String(environment),
	})
	if err != nil {
		return nil, a.handleError(err, resp, "failed to reject workflow run")
	}

	// Build result
	return map[string]interface{}{
		"success":     true,
		"run_id":      int64(runID),
		"environment": environment,
		"repository":  owner + "/" + repo,
	}, nil
}

// getCommits retrieves commits for a repository
func (a *Adapter) getCommits(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional parameters
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.CommitsListOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: pageSize,
		},
	}

	// Handle optional filters
	if sha, ok := params["sha"].(string); ok && sha != "" {
		listOptions.SHA = sha
	}

	if path, ok := params["path"].(string); ok && path != "" {
		listOptions.Path = path
	}

	if author, ok := params["author"].(string); ok && author != "" {
		listOptions.Author = author
	}

	if since, ok := params["since"].(string); ok && since != "" {
		sinceTime, err := time.Parse(time.RFC3339, since)
		if err == nil {
			listOptions.Since = sinceTime
		}
	}

	if until, ok := params["until"].(string); ok && until != "" {
		untilTime, err := time.Parse(time.RFC3339, until)
		if err == nil {
			listOptions.Until = untilTime
		}
	}

	// Fetch commits
	commits, resp, err := a.client.Repositories.ListCommits(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch commits")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(commits))
	for _, commit := range commits {
		commitMap := map[string]interface{}{
			"sha":        commit.SHA,
			"html_url":   commit.HTMLURL,
			"commit": map[string]interface{}{
				"message":   commit.Commit.Message,
				"author":    commit.Commit.Author,
				"committer": commit.Commit.Committer,
			},
		}

		if commit.Author != nil {
			commitMap["author"] = map[string]interface{}{
				"login":     commit.Author.Login,
				"id":        commit.Author.ID,
				"avatar_url": commit.Author.AvatarURL,
				"html_url":  commit.Author.HTMLURL,
			}
		}

		if commit.Committer != nil {
			commitMap["committer"] = map[string]interface{}{
				"login":     commit.Committer.Login,
				"id":        commit.Committer.ID,
				"avatar_url": commit.Committer.AvatarURL,
				"html_url":  commit.Committer.HTMLURL,
			}
		}

		if commit.Stats != nil {
			commitMap["stats"] = map[string]interface{}{
				"additions": commit.Stats.Additions,
				"deletions": commit.Stats.Deletions,
				"total":     commit.Stats.Total,
			}
		}

		result = append(result, commitMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"commits":    result,
		"page":       page,
		"per_page":   pageSize,
		"has_next":   resp.NextPage != 0,
		"next_page":  resp.NextPage,
		"repository": owner + "/" + repo,
	}, nil
}

// searchCode searches for code in repositories
func (a *Adapter) searchCode(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract query parameter (required)
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	// Extract optional parameters
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create search options
	searchOptions := &github.SearchOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: pageSize,
		},
	}

	// Perform the search
	searchResult, resp, err := a.client.Search.Code(ctx, query, searchOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to search code")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(searchResult.CodeResults))
	for _, code := range searchResult.CodeResults {
		codeMap := map[string]interface{}{
			"name":        code.Name,
			"path":        code.Path,
			"sha":         code.SHA,
			"html_url":    code.HTMLURL,
			"git_url":     code.HTMLURL, // Use HTMLURL instead of GitURL which is no longer available
			"repository": map[string]interface{}{
				"id":        code.Repository.ID,
				"name":      code.Repository.Name,
				"full_name": code.Repository.FullName,
				"html_url":  code.Repository.HTMLURL,
			},
		}
		result = append(result, codeMap)
	}

	// Build result with pagination info
	return map[string]interface{}{
		"items":           result,
		"total_count":     searchResult.GetTotal(),
		"incomplete_results": searchResult.GetIncompleteResults(),
		"page":            page,
		"per_page":        pageSize,
		"has_next":        resp.NextPage != 0,
		"next_page":       resp.NextPage,
		"query":           query,
	}, nil
}

// getUsers retrieves users from GitHub
func (a *Adapter) getUsers(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if we're looking up a specific user
	if username, ok := params["username"].(string); ok && username != "" {
		// Fetch a single user
		user, resp, err := a.client.Users.Get(ctx, username)
		if err != nil {
			return nil, a.handleError(err, resp, "failed to fetch user")
		}

		// Convert to a map
		result := map[string]interface{}{
			"id":           user.ID,
			"login":        user.Login,
			"type":         user.Type,
			"name":         user.Name,
			"company":      user.Company,
			"blog":         user.Blog,
			"location":     user.Location,
			"email":        user.Email,
			"bio":          user.Bio,
			"public_repos": user.PublicRepos,
			"public_gists": user.PublicGists,
			"followers":    user.Followers,
			"following":    user.Following,
			"created_at":   user.CreatedAt,
			"updated_at":   user.UpdatedAt,
			"html_url":     user.HTMLURL,
			"avatar_url":   user.AvatarURL,
		}

		return result, nil
	}

	// If we're looking for org members
	if org, ok := params["org"].(string); ok && org != "" {
		// Extract optional parameters
		var pageSize int
		if v, ok := params["per_page"].(float64); ok && v > 0 {
			pageSize = int(v)
		} else {
			pageSize = a.config.DefaultPerPage
		}

		var page int
		if v, ok := params["page"].(float64); ok && v > 0 {
			page = int(v)
		} else {
			page = 1
		}

		// Create list options
		listOptions := &github.ListMembersOptions{
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: pageSize,
			},
		}

		// Fetch organization members
		members, resp, err := a.client.Organizations.ListMembers(ctx, org, listOptions)
		if err != nil {
			return nil, a.handleError(err, resp, "failed to fetch organization members")
		}

		// Convert to a more generic format
		result := make([]map[string]interface{}, 0, len(members))
		for _, member := range members {
			memberMap := map[string]interface{}{
				"id":         member.ID,
				"login":      member.Login,
				"type":       member.Type,
				"html_url":   member.HTMLURL,
				"avatar_url": member.AvatarURL,
			}
			result = append(result, memberMap)
		}

		// Build pagination info
		return map[string]interface{}{
			"members":      result,
			"page":         page,
			"per_page":     pageSize,
			"has_next":     resp.NextPage != 0,
			"next_page":    resp.NextPage,
			"organization": org,
		}, nil
	}

	// If we're looking for team members
	if teamID, ok := params["team_id"].(float64); ok && teamID > 0 {
		org, ok := params["org"].(string)
		if !ok || org == "" {
			return nil, fmt.Errorf("org is required when fetching team members")
		}

		// Extract optional parameters
		var pageSize int
		if v, ok := params["per_page"].(float64); ok && v > 0 {
			pageSize = int(v)
		} else {
			pageSize = a.config.DefaultPerPage
		}

		var page int
		if v, ok := params["page"].(float64); ok && v > 0 {
			page = int(v)
		} else {
			page = 1
		}

		// Create list options
		listOptions := &github.TeamListTeamMembersOptions{
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: pageSize,
			},
		}

		// Convert team ID to string for slug
		teamSlug := fmt.Sprintf("%d", int64(teamID))
		
		// Fetch team members
		members, resp, err := a.client.Teams.ListTeamMembersBySlug(ctx, org, teamSlug, listOptions)
		if err != nil {
			return nil, a.handleError(err, resp, "failed to fetch team members")
		}

		// Convert to a more generic format
		result := make([]map[string]interface{}, 0, len(members))
		for _, member := range members {
			memberMap := map[string]interface{}{
				"id":         member.ID,
				"login":      member.Login,
				"type":       member.Type,
				"html_url":   member.HTMLURL,
				"avatar_url": member.AvatarURL,
			}
			result = append(result, memberMap)
		}

		// Build pagination info
		return map[string]interface{}{
			"members":      result,
			"page":         page,
			"per_page":     pageSize,
			"has_next":     resp.NextPage != 0,
			"next_page":    resp.NextPage,
			"team_id":      int64(teamID),
			"organization": org,
		}, nil
	}

	// By default, search for users
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required when searching for users")
	}

	// Extract optional parameters
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create search options
	searchOptions := &github.SearchOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: pageSize,
		},
	}

	// Perform the search
	searchResult, resp, err := a.client.Search.Users(ctx, query, searchOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to search users")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(searchResult.Users))
	for _, user := range searchResult.Users {
		userMap := map[string]interface{}{
			"id":         user.ID,
			"login":      user.Login,
			"type":       user.Type,
			"html_url":   user.HTMLURL,
			"avatar_url": user.AvatarURL,
			"score":      1.0, // Default score since Score field is no longer available
		}
		result = append(result, userMap)
	}

	// Build result with pagination info
	return map[string]interface{}{
		"users":       result,
		"total_count": searchResult.GetTotal(),
		"page":        page,
		"per_page":    pageSize,
		"has_next":    resp.NextPage != 0,
		"next_page":   resp.NextPage,
		"query":       query,
	}, nil
}

// getWebhooks retrieves webhooks for a repository
func (a *Adapter) getWebhooks(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional parameters
	var pageSize int
	if v, ok := params["per_page"].(float64); ok && v > 0 {
		pageSize = int(v)
	} else {
		pageSize = a.config.DefaultPerPage
	}

	var page int
	if v, ok := params["page"].(float64); ok && v > 0 {
		page = int(v)
	} else {
		page = 1
	}

	// Create list options
	listOptions := &github.ListOptions{
		Page:    page,
		PerPage: pageSize,
	}

	// Fetch webhooks
	hooks, resp, err := a.client.Repositories.ListHooks(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to fetch webhooks")
	}

	// Convert to a more generic format
	result := make([]map[string]interface{}, 0, len(hooks))
	for _, hook := range hooks {
		hookMap := map[string]interface{}{
			"id":          hook.ID,
			"type":        hook.Type,
			"name":        hook.Name,
			"active":      hook.Active,
			"events":      hook.Events,
			"config":      hook.Config,
			"created_at":  hook.CreatedAt,
			"updated_at":  hook.UpdatedAt,
			"url":         hook.URL,
			"ping_url":    hook.PingURL,
			"test_url":    hook.TestURL,
		}
		result = append(result, hookMap)
	}

	// Build pagination info
	return map[string]interface{}{
		"webhooks":    result,
		"page":        page,
		"per_page":    pageSize,
		"has_next":    resp.NextPage != 0,
		"next_page":   resp.NextPage,
		"repository":  owner + "/" + repo,
	}, nil
}

// getAdapterStats returns the current adapter statistics
func (a *Adapter) getAdapterStats(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	result := map[string]interface{}{
		"requests_total":    a.stats.RequestsTotal,
		"requests_success":  a.stats.RequestsSuccess,
		"requests_failed":   a.stats.RequestsFailed,
		"requests_retried":  a.stats.RequestsRetried,
		"rate_limit_hits":   a.stats.RateLimitHits,
		"health_status":     a.healthStatus,
		"average_response_time_ms": a.stats.AverageResponseTime.Milliseconds(),
	}
	
	// Add rate limit info if available
	if a.rateLimits != nil && a.rateLimits.Core != nil {
		result["rate_limit"] = map[string]interface{}{
			"limit":     a.rateLimits.Core.Limit,
			"remaining": a.rateLimits.Core.Remaining,
			"reset":     a.rateLimits.Core.Reset.Format(time.RFC3339),
			"used":      (a.rateLimits.Core.Limit - a.rateLimits.Core.Remaining),
		}
		
		result["rate_limit_checked_at"] = a.lastRateCheck.Format(time.RFC3339)
	}
	
	// Add error information if available
	if a.stats.LastError != "" {
		result["last_error"] = a.stats.LastError
		result["last_error_time"] = a.stats.LastErrorTime.Format(time.RFC3339)
	}
	
	// Add success information if available
	if !a.stats.LastSuccessfulRequest.IsZero() {
		result["last_successful_request"] = a.stats.LastSuccessfulRequest.Format(time.RFC3339)
	}
	
	// Add operation stats
	result["successful_operations"] = a.stats.SuccessfulOperations
	result["failed_operations"] = a.stats.FailedOperations
	
	return result, nil
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// First check if this operation is safe
	if safe, err := a.IsSafeOperation(action, params); !safe {
		a.stats.RequestsFailed++
		a.stats.LastError = fmt.Sprintf("operation %s is not allowed: %v", action, err)
		a.stats.LastErrorTime = time.Now()
		return nil, fmt.Errorf("operation %s is not allowed: %w", action, err)
	}

	// Track total request count
	a.stats.RequestsTotal++
	
	// Check rate limits before proceeding
	if err := a.updateRateLimits(ctx); err == nil && a.rateLimits != nil {
		// If rate limits are critically low and we're not configured to retry,
		// return an error instead of making the API call
		if a.rateLimits.Core != nil && 
		   a.rateLimits.Core.Remaining < int(a.config.RateLimitThreshold) && 
		   !a.config.EnableRetryOnRateLimit {
			resetTime := time.Until(a.rateLimits.Core.Reset.Time)
			a.stats.RequestsFailed++
			a.stats.RateLimitHits++
			a.stats.LastError = fmt.Sprintf("GitHub API rate limit threshold reached (%d/%d remaining)", 
				a.rateLimits.Core.Remaining, a.rateLimits.Core.Limit)
			a.stats.LastErrorTime = time.Now()
			return nil, fmt.Errorf("GitHub API rate limit threshold reached (%d/%d remaining). Resets in %s",
				a.rateLimits.Core.Remaining, a.rateLimits.Core.Limit, resetTime)
		}
	}

	// Store context ID in params for traceability
	if contextID != "" {
		params["context_id"] = contextID
	}

	startTime := time.Now()
	var result interface{}
	var err error
	
	// Handle different actions
	switch action {
	case "create_issue":
		result, err = a.createIssue(ctx, params)
	case "close_issue":
		result, err = a.closeIssue(ctx, params)
	case "create_pull_request":
		result, err = a.createPullRequest(ctx, params)
	case "merge_pull_request":
		result, err = a.mergePullRequest(ctx, params)
	case "add_comment":
		result, err = a.addComment(ctx, params)
	case "create_branch":
		result, err = a.createBranch(ctx, params)
	case "create_webhook":
		result, err = a.createWebhook(ctx, params)
	case "delete_webhook":
		result, err = a.deleteWebhook(ctx, params)
	case "trigger_workflow":
		result, err = a.triggerWorkflow(ctx, params)
	case "enable_workflow":
		result, err = a.enableWorkflow(ctx, params)
	case "disable_workflow":
		result, err = a.disableWorkflow(ctx, params)
	case "approve_workflow_run":
		result, err = a.approveWorkflowRun(ctx, params)
	case "reject_workflow_run":
		result, err = a.rejectWorkflowRun(ctx, params)
	default:
		err = fmt.Errorf("unsupported action: %s", action)
	}

	duration := time.Since(startTime)
	
	// Update stats
	if err != nil {
		a.stats.RequestsFailed++
		a.stats.FailedOperations[action]++
		a.stats.LastError = err.Error()
		a.stats.LastErrorTime = time.Now()
	} else {
		a.stats.RequestsSuccess++
		a.stats.SuccessfulOperations[action]++
		a.stats.LastSuccessfulRequest = time.Now()
		
		// Update response time stats
		a.stats.TotalResponseTime += duration
		a.stats.ResponseTimeCount++
		a.stats.AverageResponseTime = time.Duration(int64(a.stats.TotalResponseTime) / a.stats.ResponseTimeCount)
	}

	return result, err
}

// createIssue creates a new issue in a repository
func (a *Adapter) createIssue(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	title, ok := params["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// Extract optional parameters
	var body string
	if v, ok := params["body"].(string); ok {
		body = v
	}

	// Create issue request
	issueRequest := &github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	// Extract optional assignees
	if assignees, ok := params["assignees"].([]interface{}); ok {
		issueAssignees := make([]string, 0, len(assignees))
		for _, assignee := range assignees {
			if a, ok := assignee.(string); ok && a != "" {
				issueAssignees = append(issueAssignees, a)
			}
		}
		issueRequest.Assignees = &issueAssignees
	}

	// Extract optional labels
	if labels, ok := params["labels"].([]interface{}); ok {
		issueLabels := make([]string, 0, len(labels))
		for _, label := range labels {
			if l, ok := label.(string); ok && l != "" {
				issueLabels = append(issueLabels, l)
			}
		}
		issueRequest.Labels = &issueLabels
	}

	// Create the issue
	issue, resp, err := a.client.Issues.Create(ctx, owner, repo, issueRequest)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to create issue")
	}

	// Build result
	result := map[string]interface{}{
		"id":         issue.ID,
		"number":     issue.Number,
		"title":      issue.Title,
		"state":      issue.State,
		"html_url":   issue.HTMLURL,
		"created_at": issue.CreatedAt,
		"repository": owner + "/" + repo,
	}

	return result, nil
}

// closeIssue closes an issue in a repository
func (a *Adapter) closeIssue(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Ensure number is of the right type
	var issueNumber int
	if num, ok := params["number"].(float64); ok {
		issueNumber = int(num)
	} else if num, ok := params["issue_number"].(float64); ok {
		issueNumber = int(num)
	} else {
		return nil, fmt.Errorf("issue number is required")
	}

	// Create state update request with closed state
	closedState := "closed"
	issueRequest := &github.IssueRequest{
		State: &closedState,
	}

	// Update the issue to close it
	issue, resp, err := a.client.Issues.Edit(ctx, owner, repo, issueNumber, issueRequest)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to close issue")
	}

	// Build result
	result := map[string]interface{}{
		"id":         issue.ID,
		"number":     issue.Number,
		"title":      issue.Title,
		"state":      issue.State,
		"html_url":   issue.HTMLURL,
		"closed_at":  issue.ClosedAt,
		"repository": owner + "/" + repo,
	}

	return result, nil
}

// createPullRequest creates a new pull request in a repository
func (a *Adapter) createPullRequest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	title, ok := params["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	head, ok := params["head"].(string)
	if !ok || head == "" {
		return nil, fmt.Errorf("head branch is required")
	}

	base, ok := params["base"].(string)
	if !ok || base == "" {
		return nil, fmt.Errorf("base branch is required")
	}

	// Extract optional parameters
	var body string
	if v, ok := params["body"].(string); ok {
		body = v
	}

	// Create pull request
	newPR := &github.NewPullRequest{
		Title: &title,
		Head:  &head,
		Base:  &base,
		Body:  &body,
	}

	// Set draft status if provided
	if draft, ok := params["draft"].(bool); ok {
		newPR.Draft = &draft
	}

	// Set maintainer can modify if provided
	if maintainerCanModify, ok := params["maintainer_can_modify"].(bool); ok {
		newPR.MaintainerCanModify = &maintainerCanModify
	}

	// Create the pull request
	pr, resp, err := a.client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to create pull request")
	}

	// Build result
	result := map[string]interface{}{
		"id":            pr.ID,
		"number":        pr.Number,
		"title":         pr.Title,
		"state":         pr.State,
		"html_url":      pr.HTMLURL,
		"created_at":    pr.CreatedAt,
		"head_ref":      pr.Head.Ref,
		"base_ref":      pr.Base.Ref,
		"draft":         pr.Draft,
		"repository":    owner + "/" + repo,
	}

	return result, nil
}

// mergePullRequest merges a pull request
func (a *Adapter) mergePullRequest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Handle different parameter names for PR number
	var prNumber int
	if num, ok := params["number"].(float64); ok {
		prNumber = int(num)
	} else if num, ok := params["pull_number"].(float64); ok {
		prNumber = int(num)
	} else {
		return nil, fmt.Errorf("pull request number is required")
	}

	// Extract optional merge parameters
	mergeOptions := &github.PullRequestOptions{}

	if commitTitle, ok := params["commit_title"].(string); ok && commitTitle != "" {
		mergeOptions.CommitTitle = commitTitle
	}

	// Get commit message from params - will be used directly in the Merge call
	var commitMessage string
	if cm, ok := params["commit_message"].(string); ok && cm != "" {
		commitMessage = cm
	}

	if mergeMethod, ok := params["merge_method"].(string); ok && mergeMethod != "" {
		mergeOptions.MergeMethod = mergeMethod
	}

	// Perform the merge
	result, resp, err := a.client.PullRequests.Merge(ctx, owner, repo, prNumber, commitMessage, mergeOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to merge pull request")
	}

	return map[string]interface{}{
		"merged":      result.Merged,
		"message":     result.Message,
		"sha":         result.SHA,
		"pull_number": prNumber,
		"repository":  owner + "/" + repo,
	}, nil
}

// addComment adds a comment to an issue or pull request
func (a *Adapter) addComment(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Handle different parameter names for issue/PR number
	var number int
	if num, ok := params["number"].(float64); ok {
		number = int(num)
	} else if num, ok := params["issue_number"].(float64); ok {
		number = int(num)
	} else if num, ok := params["pull_number"].(float64); ok {
		number = int(num)
	} else {
		return nil, fmt.Errorf("issue or pull request number is required")
	}

	body, ok := params["body"].(string)
	if !ok || body == "" {
		return nil, fmt.Errorf("comment body is required")
	}

	// Create comment
	commentRequest := &github.IssueComment{
		Body: &body,
	}

	// Add the comment
	comment, resp, err := a.client.Issues.CreateComment(ctx, owner, repo, number, commentRequest)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to add comment")
	}

	// Build result
	result := map[string]interface{}{
		"id":         comment.ID,
		"body":       comment.Body,
		"user_login": comment.User.Login,
		"created_at": comment.CreatedAt,
		"html_url":   comment.HTMLURL,
		"repository": owner + "/" + repo,
		"number":     number,
	}

	return result, nil
}

// createBranch creates a new branch in a repository
func (a *Adapter) createBranch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	branchName, ok := params["branch"].(string)
	if !ok || branchName == "" {
		return nil, fmt.Errorf("branch name is required")
	}

	sha, ok := params["sha"].(string)
	if !ok || sha == "" {
		return nil, fmt.Errorf("SHA (commit or branch to base the new branch on) is required")
	}

	// Check if the branch operation is safe
	if safe, err := IsSafeBranchOperation("create_branch", branchName); !safe {
		return nil, err
	}

	// Get the reference to create the branch from
	// First, check if the SHA is a branch name
	baseRef, _, err := a.client.Git.GetRef(ctx, owner, repo, "heads/"+sha)
	if err != nil {
		// If not a branch, try as a commit SHA
		baseRef, _, err = a.client.Git.GetRef(ctx, owner, repo, "heads/"+params["base_branch"].(string))
		if err != nil {
			return nil, a.handleError(err, nil, "failed to find base reference")
		}
	}

	// Create a new reference (branch)
	newRef := &github.Reference{
		Ref: github.String("refs/heads/" + branchName),
		Object: &github.GitObject{
			SHA: baseRef.Object.SHA,
		},
	}

	ref, resp, err := a.client.Git.CreateRef(ctx, owner, repo, newRef)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to create branch")
	}

	// Build result
	result := map[string]interface{}{
		"ref":        ref.Ref,
		"url":        ref.URL,
		"object_sha": ref.Object.SHA,
		"repository": owner + "/" + repo,
	}

	return result, nil
}

// createWebhook creates a new webhook for a repository
func (a *Adapter) createWebhook(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	url, ok := params["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	// Check if the webhook URL is safe
	if safe, err := IsSafeWebhookURL(url); !safe {
		return nil, err
	}

	// Extract optional parameters
	var secret string
	if s, ok := params["secret"].(string); ok {
		secret = s
	}

	var contentType string
	if ct, ok := params["content_type"].(string); ok && (ct == "json" || ct == "form") {
		contentType = ct
	} else {
		contentType = "json" // Default to JSON
	}

	var active bool = true
	if a, ok := params["active"].(bool); ok {
		active = a
	}

	// Extract events
	var events []string
	if e, ok := params["events"].([]interface{}); ok && len(e) > 0 {
		for _, event := range e {
			if eventStr, ok := event.(string); ok && eventStr != "" {
				events = append(events, eventStr)
			}
		}
	} else {
		// Default to just push events
		events = []string{"push"}
	}

	// Create webhook configuration
	config := map[string]interface{}{
		"url":          url,
		"content_type": contentType,
	}

	if secret != "" {
		config["secret"] = secret
	}

	hook := &github.Hook{
		Name:   github.String("web"),
		Events: events,
		Active: github.Bool(active),
		Config: config,
	}

	// Create the webhook
	newHook, resp, err := a.client.Repositories.CreateHook(ctx, owner, repo, hook)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to create webhook")
	}

	// Build result
	result := map[string]interface{}{
		"id":         newHook.ID,
		"type":       newHook.Type,
		"name":       newHook.Name,
		"events":     newHook.Events,
		"active":     newHook.Active,
		"created_at": newHook.CreatedAt,
		"updated_at": newHook.UpdatedAt,
		"url":        newHook.URL,
		"test_url":   newHook.TestURL,
		"ping_url":   newHook.PingURL,
		"repository": owner + "/" + repo,
	}

	return result, nil
}

// deleteWebhook deletes a webhook from a repository
func (a *Adapter) deleteWebhook(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract hook ID
	var hookID int64
	if id, ok := params["hook_id"].(float64); ok {
		hookID = int64(id)
	} else {
		return nil, fmt.Errorf("hook_id is required")
	}

	// Delete the webhook
	resp, err := a.client.Repositories.DeleteHook(ctx, owner, repo, hookID)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to delete webhook")
	}

	// Build result
	result := map[string]interface{}{
		"success":    true,
		"hook_id":    hookID,
		"repository": owner + "/" + repo,
	}

	return result, nil
}

// triggerWorkflow triggers a workflow in a repository
func (a *Adapter) triggerWorkflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("owner is required")
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	workflowID, ok := params["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	// Extract optional ref parameter (default to main/master)
	var ref string
	if r, ok := params["ref"].(string); ok && r != "" {
		ref = r
	} else {
		// Try to get the default branch from the repository
		repository, _, err := a.client.Repositories.Get(ctx, owner, repo)
		if err == nil && repository.DefaultBranch != nil {
			ref = *repository.DefaultBranch
		} else {
			// Fall back to main as the default
			ref = "main"
		}
	}

	// Extract optional inputs
	var inputs map[string]interface{}
	if i, ok := params["inputs"].(map[string]interface{}); ok {
		inputs = i
	}

	// Create workflow dispatch event
	event := github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs,
	}

	// Trigger the workflow
	resp, err := a.client.Actions.CreateWorkflowDispatchEventByID(ctx, owner, repo, int64(parseIDString(workflowID)), event)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to trigger workflow")
	}

	// Build result
	result := map[string]interface{}{
		"success":     true,
		"workflow_id": workflowID,
		"ref":         ref,
		"repository":  owner + "/" + repo,
	}

	if inputs != nil {
		result["inputs"] = inputs
	}

	return result, nil
}

// HandleWebhook processes a webhook event
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Parse the payload based on event type
	var event interface{}
	var err error

	switch eventType {
	case "pull_request":
		event, err = a.parsePullRequestEvent(payload)
	case "push":
		event, err = a.parsePushEvent(payload)
	case "issues":
		event, err = a.parseIssuesEvent(payload)
	case "issue_comment":
		event, err = a.parseIssueCommentEvent(payload)
	case "workflow_run":
		event, err = a.parseWorkflowRunEvent(payload)
	case "check_run":
		event, err = a.parseCheckRunEvent(payload)
	case "check_suite":
		event, err = a.parseCheckSuiteEvent(payload)
	case "create":
		event, err = a.parseCreateEvent(payload)
	case "delete":
		event, err = a.parseDeleteEvent(payload)
	case "ping":
		event, err = a.parsePingEvent(payload)
	default:
		// For other event types, use a generic map
		var genericEvent map[string]interface{}
		err = json.Unmarshal(payload, &genericEvent)
		event = genericEvent
	}

	if err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Notify subscribers
	if callbacks, ok := a.subscribers[eventType]; ok {
		for _, callback := range callbacks {
			callback(event)
		}
	}

	// Also notify subscribers of "all" events
	if callbacks, ok := a.subscribers["all"]; ok {
		for _, callback := range callbacks {
			callback(event)
		}
	}

	return nil
}

// parsePullRequestEvent parses a pull request event
func (a *Adapter) parsePullRequestEvent(payload []byte) (interface{}, error) {
	var event github.PullRequestEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parsePushEvent parses a push event
func (a *Adapter) parsePushEvent(payload []byte) (interface{}, error) {
	var event github.PushEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parseIssuesEvent parses an issues event
func (a *Adapter) parseIssuesEvent(payload []byte) (interface{}, error) {
	var event github.IssuesEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parseIssueCommentEvent parses an issue comment event
func (a *Adapter) parseIssueCommentEvent(payload []byte) (interface{}, error) {
	var event github.IssueCommentEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parseWorkflowRunEvent parses a workflow run event
func (a *Adapter) parseWorkflowRunEvent(payload []byte) (interface{}, error) {
	var event github.WorkflowRunEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parseCheckRunEvent parses a check run event
func (a *Adapter) parseCheckRunEvent(payload []byte) (interface{}, error) {
	var event github.CheckRunEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parseCheckSuiteEvent parses a check suite event
func (a *Adapter) parseCheckSuiteEvent(payload []byte) (interface{}, error) {
	var event github.CheckSuiteEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parseCreateEvent parses a create event
func (a *Adapter) parseCreateEvent(payload []byte) (interface{}, error) {
	var event github.CreateEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parseDeleteEvent parses a delete event
func (a *Adapter) parseDeleteEvent(payload []byte) (interface{}, error) {
	var event github.DeleteEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parsePingEvent parses a ping event
func (a *Adapter) parsePingEvent(payload []byte) (interface{}, error) {
	var event github.PingEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// Subscribe registers a callback for a specific event type
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
	a.subscribers[eventType] = append(a.subscribers[eventType], callback)
	return nil
}

// handleError processes GitHub API errors and updates stats
func (a *Adapter) handleError(err error, resp *github.Response, context string) error {
	// Increment error counter
	a.stats.RequestsFailed++
	a.stats.LastError = err.Error()
	a.stats.LastErrorTime = time.Now()
	
	// Handle specific error types
	var rateLimitErr *github.RateLimitError
	var abuseRateLimitErr *github.AbuseRateLimitError
	
	if errors.As(err, &rateLimitErr) {
		a.stats.RateLimitHits++
		
		// Update rate limits with the information from the error
		if a.rateLimits == nil {
			a.rateLimits = &github.RateLimits{}
		}
		
		if a.rateLimits.Core == nil {
			a.rateLimits.Core = &github.Rate{}
		}
		
		// Copy rate info if it exists in the error
		if rateLimitErr.Rate.Limit > 0 {
			a.rateLimits.Core.Limit = rateLimitErr.Rate.Limit
			a.rateLimits.Core.Remaining = rateLimitErr.Rate.Remaining
			a.rateLimits.Core.Reset = rateLimitErr.Rate.Reset
			
			resetTime := time.Until(rateLimitErr.Rate.Reset.Time)
			
			log.Printf("Rate limit exceeded. Remaining: %d/%d. Resets in: %s", 
				rateLimitErr.Rate.Remaining, rateLimitErr.Rate.Limit, resetTime)
			
			// If configured, wait for reset time
			if a.config.EnableRetryOnRateLimit && resetTime > 0 {
				log.Printf("Waiting for rate limit reset...")
				time.Sleep(resetTime + time.Second) // Add a second buffer
				return fmt.Errorf("%s (rate limit exceeded, retrying after waiting): %w", context, err)
			}
		}
		
		return fmt.Errorf("%s (rate limit exceeded): %w", context, err)
	} else if errors.As(err, &abuseRateLimitErr) {
		a.stats.RateLimitHits++
		
		// Get retry-after duration if available
		var retryAfter time.Duration
		if abuseRateLimitErr.RetryAfter != nil {
			retryAfter = *abuseRateLimitErr.RetryAfter
		} else {
			retryAfter = 30 * time.Second // Default backoff
		}
		
		log.Printf("Abuse detection mechanism triggered. Retrying after: %s", retryAfter)
		
		// If configured, wait for retry-after time
		if a.config.EnableRetryOnRateLimit {
			time.Sleep(retryAfter)
			return fmt.Errorf("%s (abuse detection, retrying after waiting): %w", context, err)
		}
		
		return fmt.Errorf("%s (abuse detection): %w", context, err)
	}
	
	// Handle HTTP status errors
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("%s (resource not found): %w", context, err)
		case http.StatusUnauthorized:
			return fmt.Errorf("%s (unauthorized): %w", context, err)
		case http.StatusForbidden:
			return fmt.Errorf("%s (forbidden): %w", context, err)
		case http.StatusBadRequest:
			return fmt.Errorf("%s (bad request): %w", context, err)
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
			// These are potentially retryable
			a.stats.RequestsRetried++
			return fmt.Errorf("%s (server error): %w", context, err)
		}
	}
	
	// Default error handling
	return fmt.Errorf("%s: %w", context, err)
}

// IsSafeOperation determines if an operation is safe to perform
func (a *Adapter) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	// If safe mode is disabled in the config, all operations are considered safe
	if !a.config.SafeMode {
		return true, nil
	}
	
	// Use the IsSafeOperation function from safety.go
	return IsSafeOperation(operation)
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	// Check if rate limits are critically low
	if a.rateLimits != nil && a.rateLimits.Core != nil {
		core := a.rateLimits.Core
		if core.Remaining < int(a.config.RateLimitThreshold) {
			resetTime := time.Until(core.Reset.Time)
			if resetTime > 0 {
				a.healthStatus = fmt.Sprintf("degraded: rate limit critical (%d/%d) - resets in %s",
					core.Remaining, core.Limit, resetTime.Round(time.Second))
			}
		}
	}

	// Check if too many requests are failing
	if a.stats.RequestsTotal > 0 {
		failureRate := float64(a.stats.RequestsFailed) / float64(a.stats.RequestsTotal)
		if failureRate > 0.25 { // If more than 25% of requests are failing
			a.healthStatus = fmt.Sprintf("degraded: high failure rate (%.1f%%) - last error: %s",
				failureRate*100, a.stats.LastError)
		}
	}

	return a.healthStatus
}

// GetHealthDetails returns detailed health information
func (a *Adapter) GetHealthDetails() map[string]interface{} {
	details := map[string]interface{}{
		"status": a.healthStatus,
		"stats": map[string]interface{}{
			"requests_total":          a.stats.RequestsTotal,
			"requests_success":        a.stats.RequestsSuccess,
			"requests_failed":         a.stats.RequestsFailed,
			"requests_retried":        a.stats.RequestsRetried,
			"rate_limit_hits":         a.stats.RateLimitHits,
			"average_response_time_ms": a.stats.AverageResponseTime.Milliseconds(),
			"successful_operations":    a.stats.SuccessfulOperations,
			"failed_operations":        a.stats.FailedOperations,
		},
	}

	// Add rate limit info if available
	if a.rateLimits != nil && a.rateLimits.Core != nil {
		resetTime := time.Until(a.rateLimits.Core.Reset.Time)
		details["rate_limit"] = map[string]interface{}{
			"limit":     a.rateLimits.Core.Limit,
			"remaining": a.rateLimits.Core.Remaining,
			"used":      (a.rateLimits.Core.Limit - a.rateLimits.Core.Remaining),
			"resets_in":  resetTime.String(),
			"checked_at": a.lastRateCheck.Format(time.RFC3339),
		}
	}

	// Add error information if available
	if a.stats.LastError != "" {
		details["last_error"] = map[string]interface{}{
			"message": a.stats.LastError,
			"time":    a.stats.LastErrorTime.Format(time.RFC3339),
		}
	}

	// Add success information if available
	if !a.stats.LastSuccessfulRequest.IsZero() {
		details["last_successful_request"] = a.stats.LastSuccessfulRequest.Format(time.RFC3339)
	}

	return details
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Log final stats
	log.Printf("GitHub adapter closing - Stats: %d requests (%d success, %d failed, %d retried)",
		a.stats.RequestsTotal, a.stats.RequestsSuccess, a.stats.RequestsFailed, a.stats.RequestsRetried)
	
	// Nothing to clean up for HTTP client
	return nil
}

// Helper function to parse ID string to int64
func parseIDString(idStr string) int64 {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		// If it's not a valid ID, just return 0
		return 0
	}
	return id
}
