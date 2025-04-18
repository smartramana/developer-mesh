package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

// Constants for API and rate limiting
const (
	defaultAPIVersion = "2022-11-28"
	minRateLimitRemaining = 10
	defaultRetryMax = 3
	defaultRetryDelay = 1 * time.Second
	defaultRequestTimeout = 30 * time.Second
)

// Config holds configuration for the GitHub adapter
type Config struct {
	APIToken          string        `mapstructure:"api_token"`
	WebhookSecret     string        `mapstructure:"webhook_secret"`
	EnterpriseURL     string        `mapstructure:"enterprise_url"`
	APIVersion        string        `mapstructure:"api_version"`
	RequestTimeout    time.Duration `mapstructure:"request_timeout"`
	RetryMax          int           `mapstructure:"retry_max"`
	RetryDelay        time.Duration `mapstructure:"retry_delay"`
	MockResponses     bool          `mapstructure:"mock_responses"`
	MockURL           string        `mapstructure:"mock_url"`
	RateLimitThreshold int           `mapstructure:"rate_limit_threshold"`
	DefaultPerPage    int           `mapstructure:"default_per_page"`
	EnableRetryOnRateLimit bool      `mapstructure:"enable_retry_on_rate_limit"`
	Concurrency       int           `mapstructure:"concurrency"`
	LogRequests       bool          `mapstructure:"log_requests"`
}

// Adapter implements the adapter interface for GitHub
type Adapter struct {
	adapters.BaseAdapter
	config         Config
	client         *github.Client
	httpClient     *http.Client
	subscribers    map[string][]func(interface{})
	healthStatus   string
	rateLimits     *github.RateLimits
	lastRateCheck  time.Time
	stats          *AdapterStats
}

// AdapterStats tracks usage statistics of the GitHub adapter
type AdapterStats struct {
	RequestsTotal        int64
	RequestsSuccess      int64
	RequestsFailed       int64
	RequestsRetried      int64
	RateLimitHits        int64
	LastError            string
	LastErrorTime        time.Time
	LastSuccessfulRequest time.Time
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
		config.DefaultPerPage = 100
	}
	if config.Concurrency == 0 {
		config.Concurrency = 5
	}

	adapter := &Adapter{
		BaseAdapter: adapters.BaseAdapter{
			RetryMax:   config.RetryMax,
			RetryDelay: config.RetryDelay,
		},
		config: config,
		subscribers: make(map[string][]func(interface{})),
		healthStatus: "initializing",
		stats: &AdapterStats{},
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
	var err error
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
	if core.Remaining < int64(a.config.RateLimitThreshold) {
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
		if core.Remaining < int64(a.config.RateLimitThreshold) {
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
		   a.rateLimits.Core.Remaining < int64(a.config.RateLimitThreshold) && 
		   !a.config.EnableRetryOnRateLimit {
			resetTime := time.Until(a.rateLimits.Core.Reset.Time)
			return nil, fmt.Errorf("GitHub API rate limit threshold reached (%d/%d remaining). Resets in %s",
				a.rateLimits.Core.Remaining, a.rateLimits.Core.Limit, resetTime)
		}
	}

	// Handle different operations
	switch operation {
	// Repository operations
	case "get_repositories":
		return a.getRepositories(ctx, queryMap)
	case "get_repository":
		return a.getRepository(ctx, queryMap)
	case "search_repositories":
		return a.searchRepositories(ctx, queryMap)
	
	// Pull requests operations
	case "get_pull_requests":
		return a.getPullRequests(ctx, queryMap)
	
	// Issue operations
	case "get_issues":
		return a.getIssues(ctx, queryMap)
	
	// Branch operations
	case "get_branches":
		return a.getBranches(ctx, queryMap)
	
	// Team operations
	case "get_teams":
		return a.getTeams(ctx, queryMap)
	
	// Workflow operations
	case "get_workflow_runs":
		return a.getWorkflowRuns(ctx, queryMap)
	case "get_workflows":
		return a.getWorkflows(ctx, queryMap)
	
	// Commit operations
	case "get_commits":
		return a.getCommits(ctx, queryMap)
	
	// Search operations
	case "search_code":
		return a.searchCode(ctx, queryMap)
	
	// User operations
	case "get_users":
		return a.getUsers(ctx, queryMap)
	
	// Webhook operations
	case "get_webhooks":
		return a.getWebhooks(ctx, queryMap)
	
	// Approvals operations
	case "get_required_workflow_approvals":
		return a.getRequiredWorkflowApprovals(ctx, queryMap)
	
	// Metadata operations
	case "get_rate_limits":
		return a.getRateLimits(ctx, queryMap)
	case "get_adapter_stats":
		return a.getAdapterStats(ctx, queryMap)
	
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// getRequiredWorkflowApprovals gets required workflow approvals for a run
func (a *Adapter) getRequiredWorkflowApprovals(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	runID, ok := params["run_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid run_id parameter")
	}

	// Get required approvals
	approvals, resp, err := a.client.Actions.ListRequiredWorkflowRunApprovalsForRepo(
		ctx, owner, repo, int64(runID), nil)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to get required workflow approvals")
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(approvals))
	for _, approval := range approvals {
		approvalMap := map[string]interface{}{
			"environment":   approval.GetEnvironment(),
			"state":         approval.GetState(),
			"comment":       approval.GetComment(),
			"created_at":    approval.GetCreatedAt(),
		}

		// Add user information if available
		if approval.User != nil {
			approvalMap["user"] = map[string]interface{}{
				"id":         approval.User.GetID(),
				"login":      approval.User.GetLogin(),
				"avatar_url": approval.User.GetAvatarURL(),
				"html_url":   approval.User.GetHTMLURL(),
			}
		}

		result = append(result, approvalMap)
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"approvals": result,
		"run_id":    runID,
	}, nil
}

// getRateLimits returns the current rate limit status
func (a *Adapter) getRateLimits(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Force refresh of rate limits
	rateLimits, resp, err := a.client.RateLimits(ctx)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to get rate limits")
	}

	// Update adapter's rate limits
	a.rateLimits = rateLimits
	a.lastRateCheck = time.Now()

	// Convert to map for easier serialization
	result := map[string]interface{}{
		"last_checked": a.lastRateCheck.Format(time.RFC3339),
	}
	
	// Add core rate limit info
	if rateLimits.Core != nil {
		result["core"] = map[string]interface{}{
			"limit":     rateLimits.Core.Limit,
			"remaining": rateLimits.Core.Remaining,
			"reset":     rateLimits.Core.Reset.Format(time.RFC3339),
			"used":      rateLimits.Core.Used,
		}
	}
	
	// Add search rate limit info
	if rateLimits.Search != nil {
		result["search"] = map[string]interface{}{
			"limit":     rateLimits.Search.Limit,
			"remaining": rateLimits.Search.Remaining,
			"reset":     rateLimits.Search.Reset.Format(time.RFC3339),
			"used":      rateLimits.Search.Used,
		}
	}
	
	// Add graphql rate limit info
	if rateLimits.GraphQL != nil {
		result["graphql"] = map[string]interface{}{
			"limit":     rateLimits.GraphQL.Limit,
			"remaining": rateLimits.GraphQL.Remaining,
			"reset":     rateLimits.GraphQL.Reset.Format(time.RFC3339),
			"used":      rateLimits.GraphQL.Used,
		}
	}
	
	// Add integration manifest rate limit info
	if rateLimits.IntegrationManifest != nil {
		result["integration_manifest"] = map[string]interface{}{
			"limit":     rateLimits.IntegrationManifest.Limit,
			"remaining": rateLimits.IntegrationManifest.Remaining,
			"reset":     rateLimits.IntegrationManifest.Reset.Format(time.RFC3339),
			"used":      rateLimits.IntegrationManifest.Used,
		}
	}
	
	// Add code scanning upload rate limit info
	if rateLimits.CodeScanningUpload != nil {
		result["code_scanning_upload"] = map[string]interface{}{
			"limit":     rateLimits.CodeScanningUpload.Limit,
			"remaining": rateLimits.CodeScanningUpload.Remaining,
			"reset":     rateLimits.CodeScanningUpload.Reset.Format(time.RFC3339),
			"used":      rateLimits.CodeScanningUpload.Used,
		}
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return result, nil
}

// getAdapterStats returns the current adapter statistics
func (a *Adapter) getAdapterStats(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	result := map[string]interface{}{
		"requests_total":   a.stats.RequestsTotal,
		"requests_success": a.stats.RequestsSuccess,
		"requests_failed":  a.stats.RequestsFailed,
		"requests_retried": a.stats.RequestsRetried,
		"rate_limit_hits":  a.stats.RateLimitHits,
		"health_status":    a.healthStatus,
	}
	
	// Add rate limit info if available
	if a.rateLimits != nil && a.rateLimits.Core != nil {
		result["rate_limit"] = map[string]interface{}{
			"limit":     a.rateLimits.Core.Limit,
			"remaining": a.rateLimits.Core.Remaining,
			"reset":     a.rateLimits.Core.Reset.Format(time.RFC3339),
			"used":      a.rateLimits.Core.Used,
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
	
	return result, nil
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// First check if this operation is safe
	if safe, err := IsSafeOperation(action); !safe {
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
		   a.rateLimits.Core.Remaining < int64(a.config.RateLimitThreshold) && 
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

	// Handle different actions
	switch action {
	// Issue actions
	case "create_issue":
		return a.createIssue(ctx, params)
	case "close_issue":
		return a.closeIssue(ctx, params)
	case "reopen_issue":
		return a.reopenIssue(ctx, params)
		
	// Pull request actions
	case "create_pull_request":
		return a.createPullRequest(ctx, params)
	case "merge_pull_request":
		return a.mergePullRequest(ctx, params)
	case "close_pull_request":
		return a.closePullRequest(ctx, params)
	case "review_pull_request":
		return a.reviewPullRequest(ctx, params)
		
	// Comment actions
	case "add_comment":
		return a.addComment(ctx, params)
	case "edit_comment":
		return a.editComment(ctx, params)
	case "delete_comment":
		return a.deleteComment(ctx, params)
		
	// Branch actions
	case "create_branch":
		return a.createBranch(ctx, params)
	case "delete_branch":
		return a.deleteBranch(ctx, params)
	case "protect_branch":
		return a.protectBranch(ctx, params)
		
	// Webhook actions
	case "create_webhook":
		return a.createWebhook(ctx, params)
	case "delete_webhook":
		return a.deleteWebhook(ctx, params)
	case "update_webhook":
		return a.updateWebhook(ctx, params)
		
	// Workflow actions
	case "check_workflow_run":
		return a.checkWorkflowRun(ctx, params)
	case "trigger_workflow":
		return a.triggerWorkflow(ctx, params)
	case "cancel_workflow_run":
		return a.cancelWorkflowRun(ctx, params)
	case "enable_workflow":
		return a.enableWorkflow(ctx, params)
	case "disable_workflow":
		return a.disableWorkflow(ctx, params)
	case "approve_workflow_run":
		return a.approveWorkflowRun(ctx, params)
	case "reject_workflow_run":
		return a.rejectWorkflowRun(ctx, params)
		
	// Team actions
	case "list_team_members":
		return a.listTeamMembers(ctx, params)
	case "add_team_member":
		return a.addTeamMember(ctx, params)
	case "remove_team_member":
		return a.removeTeamMember(ctx, params)
	case "create_team":
		return a.createTeam(ctx, params)
		
	// Release actions
	case "create_release":
		return a.createRelease(ctx, params)
	case "update_release":
		return a.updateRelease(ctx, params)
	case "delete_release":
		return a.deleteRelease(ctx, params)
		
	// Default handling
	default:
		a.stats.RequestsFailed++
		a.stats.LastError = fmt.Sprintf("unsupported action: %s", action)
		a.stats.LastErrorTime = time.Now()
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// reopenIssue reopens a previously closed issue
func (a *Adapter) reopenIssue(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	issueNumber, ok := params["issue_number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid issue_number parameter")
	}

	// Create issue update request
	issueRequest := &github.IssueRequest{
		State: github.String("open"),
	}

	// Update the issue
	issue, resp, err := a.client.Issues.Edit(
		ctx,
		owner,
		repo,
		int(issueNumber),
		issueRequest,
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to reopen issue")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"issue_number": issue.GetNumber(),
		"state":        issue.GetState(),
		"html_url":     issue.GetHTMLURL(),
	}, nil
}

// closePullRequest closes a pull request without merging
func (a *Adapter) closePullRequest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	pullNumber, ok := params["pull_number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid pull_number parameter")
	}

	// Create pull request update
	pullRequest := &github.PullRequest{
		State: github.String("closed"),
	}

	// Close the pull request
	pr, resp, err := a.client.PullRequests.Edit(
		ctx,
		owner,
		repo,
		int(pullNumber),
		&github.PullRequest{
			State: github.String("closed"),
		},
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to close pull request")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"pull_number": pr.GetNumber(),
		"state":       pr.GetState(),
		"html_url":    pr.GetHTMLURL(),
		"closed_at":   pr.GetClosedAt(),
	}, nil
}

// reviewPullRequest submits a review on a pull request
func (a *Adapter) reviewPullRequest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	pullNumber, ok := params["pull_number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid pull_number parameter")
	}

	event, ok := params["event"].(string)
	if !ok {
		return nil, fmt.Errorf("missing event parameter (must be APPROVE, REQUEST_CHANGES, or COMMENT)")
	}

	// Validate event type
	validEvents := map[string]bool{
		"APPROVE":         true,
		"REQUEST_CHANGES": true,
		"COMMENT":         true,
	}
	if !validEvents[event] {
		return nil, fmt.Errorf("invalid event parameter: %s (must be APPROVE, REQUEST_CHANGES, or COMMENT)", event)
	}

	// Create review request
	reviewRequest := &github.PullRequestReviewRequest{
		Event: github.String(event),
	}

	// Add body if provided
	if bodyParam, ok := params["body"].(string); ok {
		reviewRequest.Body = github.String(bodyParam)
	}

	// Submit the review
	review, resp, err := a.client.PullRequests.CreateReview(
		ctx,
		owner,
		repo,
		int(pullNumber),
		reviewRequest,
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to submit pull request review")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"id":         review.GetID(),
		"html_url":   review.GetHTMLURL(),
		"state":      review.GetState(),
		"submitted_at": review.GetSubmittedAt(),
		"user":       review.GetUser().GetLogin(),
	}, nil
}

// editComment edits an existing comment
func (a *Adapter) editComment(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	commentID, ok := params["comment_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid comment_id parameter")
	}

	body, ok := params["body"].(string)
	if !ok {
		return nil, fmt.Errorf("missing body parameter")
	}

	// Create comment update
	comment := &github.IssueComment{
		Body: github.String(body),
	}

	// Update the comment
	result, resp, err := a.client.Issues.EditComment(
		ctx,
		owner,
		repo,
		int64(commentID),
		comment,
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to edit comment")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"id":         result.GetID(),
		"html_url":   result.GetHTMLURL(),
		"created_at": result.GetCreatedAt(),
		"updated_at": result.GetUpdatedAt(),
		"user":       result.GetUser().GetLogin(),
	}, nil
}

// deleteComment deletes an existing comment
func (a *Adapter) deleteComment(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	commentID, ok := params["comment_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid comment_id parameter")
	}

	// Delete the comment
	resp, err := a.client.Issues.DeleteComment(
		ctx,
		owner,
		repo,
		int64(commentID),
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to delete comment")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"success":    true,
		"comment_id": commentID,
	}, nil
}

// deleteBranch deletes a branch from a repository
func (a *Adapter) deleteBranch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	branchName, ok := params["branch"].(string)
	if !ok {
		return nil, fmt.Errorf("missing branch parameter")
	}

	// Check if this is a protected branch
	safe, err := IsSafeBranchOperation("delete_branch", branchName)
	if !safe {
		return nil, fmt.Errorf("cannot delete protected branch %s: %w", branchName, err)
	}

	// Delete the branch
	refName := "heads/" + branchName
	resp, err := a.client.Git.DeleteRef(ctx, owner, repo, refName)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to delete branch")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"success": true,
		"branch":  branchName,
	}, nil
}

// protectBranch adds protection rules to a branch
func (a *Adapter) protectBranch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	branchName, ok := params["branch"].(string)
	if !ok {
		return nil, fmt.Errorf("missing branch parameter")
	}

	// Create protection request
	protectionRequest := &github.ProtectionRequest{}

	// Add required status checks if provided
	if requireStatusChecks, ok := params["require_status_checks"].(bool); ok && requireStatusChecks {
		var contexts []string
		if contextsParam, ok := params["required_status_check_contexts"].([]interface{}); ok {
			for _, ctx := range contextsParam {
				if ctxStr, ok := ctx.(string); ok {
					contexts = append(contexts, ctxStr)
				}
			}
		}

		strict := false
		if strictParam, ok := params["strict"].(bool); ok {
			strict = strictParam
		}

		protectionRequest.RequiredStatusChecks = &github.RequiredStatusChecks{
			Strict:   strict,
			Contexts: contexts,
		}
	}

	// Add required pull request reviews if provided
	if requirePRReviews, ok := params["require_pull_request_reviews"].(bool); ok && requirePRReviews {
		dismissStaleReviews := false
		if dismissStaleParam, ok := params["dismiss_stale_reviews"].(bool); ok {
			dismissStaleReviews = dismissStaleParam
		}

		requireCodeOwnerReviews := false
		if requireOwnerParam, ok := params["require_code_owner_reviews"].(bool); ok {
			requireCodeOwnerReviews = requireOwnerParam
		}

		requiredApprovingReviewCount := 1
		if countParam, ok := params["required_approving_review_count"].(float64); ok {
			requiredApprovingReviewCount = int(countParam)
		}

		protectionRequest.RequiredPullRequestReviews = &github.PullRequestReviewsEnforcementRequest{
			DismissStaleReviews:          dismissStaleReviews,
			RequireCodeOwnerReviews:      requireCodeOwnerReviews,
			RequiredApprovingReviewCount: requiredApprovingReviewCount,
		}
	}

	// Add restrictions if provided
	if restrictPushes, ok := params["restrict_pushes"].(bool); ok && restrictPushes {
		var teams, users []string

		if teamsParam, ok := params["teams"].([]interface{}); ok {
			for _, team := range teamsParam {
				if teamStr, ok := team.(string); ok {
					teams = append(teams, teamStr)
				}
			}
		}

		if usersParam, ok := params["users"].([]interface{}); ok {
			for _, user := range usersParam {
				if userStr, ok := user.(string); ok {
					users = append(users, userStr)
				}
			}
		}

		if len(teams) > 0 || len(users) > 0 {
			protectionRequest.Restrictions = &github.BranchRestrictionsRequest{
				Teams: teams,
				Users: users,
			}
		}
	}

	// Set enforce admins if provided
	if enforceAdmins, ok := params["enforce_admins"].(bool); ok {
		protectionRequest.EnforceAdmins = enforceAdmins
	}

	// Add branch protection
	protection, resp, err := a.client.Repositories.UpdateBranchProtection(
		ctx,
		owner,
		repo,
		branchName,
		protectionRequest,
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to protect branch")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	// Build response
	result := map[string]interface{}{
		"url":            protection.GetURL(),
		"enforce_admins": protection.GetEnforceAdmins().Enabled,
	}

	// Add status checks info
	if protection.RequiredStatusChecks != nil {
		result["required_status_checks"] = map[string]interface{}{
			"strict":   protection.RequiredStatusChecks.Strict,
			"contexts": protection.RequiredStatusChecks.Contexts,
		}
	}

	// Add PR review info
	if protection.RequiredPullRequestReviews != nil {
		result["required_pull_request_reviews"] = map[string]interface{}{
			"dismiss_stale_reviews":           protection.RequiredPullRequestReviews.DismissStaleReviews,
			"require_code_owner_reviews":      protection.RequiredPullRequestReviews.RequireCodeOwnerReviews,
			"required_approving_review_count": protection.RequiredPullRequestReviews.RequiredApprovingReviewCount,
		}
	}

	return result, nil
}

// updateWebhook updates an existing webhook
func (a *Adapter) updateWebhook(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	hookID, ok := params["hook_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid hook_id parameter")
	}

	// Create webhook update options
	hookUpdate := &github.Hook{}
	configUpdated := false
	eventsUpdated := false

	// Update URL if provided
	if url, ok := params["url"].(string); ok {
		if hookUpdate.Config == nil {
			hookUpdate.Config = make(map[string]interface{})
		}
		hookUpdate.Config["url"] = url
		configUpdated = true
	}

	// Update content type if provided
	if contentType, ok := params["content_type"].(string); ok {
		if hookUpdate.Config == nil {
			hookUpdate.Config = make(map[string]interface{})
		}
		hookUpdate.Config["content_type"] = contentType
		configUpdated = true
	}

	// Update secret if provided
	if secret, ok := params["secret"].(string); ok {
		if hookUpdate.Config == nil {
			hookUpdate.Config = make(map[string]interface{})
		}
		hookUpdate.Config["secret"] = secret
		configUpdated = true
	}

	// Update insecure_ssl if provided
	if insecureSSL, ok := params["insecure_ssl"].(string); ok {
		if hookUpdate.Config == nil {
			hookUpdate.Config = make(map[string]interface{})
		}
		hookUpdate.Config["insecure_ssl"] = insecureSSL
		configUpdated = true
	}

	// Update events if provided
	if eventsParam, ok := params["events"].([]interface{}); ok {
		events := make([]string, 0, len(eventsParam))
		for _, event := range eventsParam {
			if eventStr, ok := event.(string); ok {
				events = append(events, eventStr)
			}
		}
		hookUpdate.Events = events
		eventsUpdated = true
	}

	// Update active status if provided
	if activeParam, ok := params["active"].(bool); ok {
		hookUpdate.Active = github.Bool(activeParam)
	}

	// If nothing to update, return an error
	if hookUpdate.Config == nil && !eventsUpdated && hookUpdate.Active == nil {
		return nil, fmt.Errorf("no update parameters provided")
	}

	// Update the webhook
	hook, resp, err := a.client.Repositories.EditHook(
		ctx,
		owner,
		repo,
		int64(hookID),
		hookUpdate,
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to update webhook")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	// Return masked config for security
	configMap := make(map[string]interface{})
	for key, value := range hook.Config {
		if key == "secret" && value != nil && value != "" {
			configMap[key] = "********"
		} else {
			configMap[key] = value
		}
	}

	return map[string]interface{}{
		"id":          hook.GetID(),
		"url":         hook.GetURL(),
		"events":      hook.Events,
		"active":      hook.GetActive(),
		"config":      configMap,
		"created_at":  hook.GetCreatedAt(),
		"updated_at":  hook.GetUpdatedAt(),
	}, nil
}

// cancelWorkflowRun cancels a workflow run
func (a *Adapter) cancelWorkflowRun(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	runID, ok := params["run_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid run_id parameter")
	}

	// Cancel the workflow run
	resp, err := a.client.Actions.CancelWorkflowRun(ctx, owner, repo, int64(runID))
	if err != nil {
		return nil, a.handleError(err, resp, "failed to cancel workflow run")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"success": true,
		"run_id":  int64(runID),
	}, nil
}

// createTeam creates a new team in an organization
func (a *Adapter) createTeam(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	org, ok := params["org"].(string)
	if !ok {
		return nil, fmt.Errorf("missing org parameter")
	}

	name, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing name parameter")
	}

	// Create team options
	newTeam := &github.NewTeam{
		Name: name,
	}

	// Add description if provided
	if descParam, ok := params["description"].(string); ok {
		newTeam.Description = &descParam
	}

	// Add privacy if provided
	if privacyParam, ok := params["privacy"].(string); ok {
		newTeam.Privacy = &privacyParam
	}

	// Create the team
	team, resp, err := a.client.Teams.CreateTeam(ctx, org, *newTeam)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to create team")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"id":          team.GetID(),
		"name":        team.GetName(),
		"slug":        team.GetSlug(),
		"description": team.GetDescription(),
		"privacy":     team.GetPrivacy(),
		"html_url":    team.GetHTMLURL(),
		"created_at":  team.GetCreatedAt(),
	}, nil
}

// createRelease creates a new release for a repository
func (a *Adapter) createRelease(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	tagName, ok := params["tag_name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing tag_name parameter")
	}

	// Create release options
	newRelease := &github.RepositoryRelease{
		TagName: github.String(tagName),
	}

	// Add target commitish if provided
	if targetParam, ok := params["target_commitish"].(string); ok {
		newRelease.TargetCommitish = github.String(targetParam)
	}

	// Add name if provided
	if nameParam, ok := params["name"].(string); ok {
		newRelease.Name = github.String(nameParam)
	}

	// Add body if provided
	if bodyParam, ok := params["body"].(string); ok {
		newRelease.Body = github.String(bodyParam)
	}

	// Add draft status if provided
	if draftParam, ok := params["draft"].(bool); ok {
		newRelease.Draft = github.Bool(draftParam)
	}

	// Add prerelease status if provided
	if prereleaseParam, ok := params["prerelease"].(bool); ok {
		newRelease.Prerelease = github.Bool(prereleaseParam)
	}

	// Create the release
	release, resp, err := a.client.Repositories.CreateRelease(ctx, owner, repo, newRelease)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to create release")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"id":           release.GetID(),
		"name":         release.GetName(),
		"tag_name":     release.GetTagName(),
		"html_url":     release.GetHTMLURL(),
		"created_at":   release.GetCreatedAt(),
		"published_at": release.GetPublishedAt(),
		"draft":        release.GetDraft(),
		"prerelease":   release.GetPrerelease(),
		"author":       release.GetAuthor().GetLogin(),
	}, nil
}

// updateRelease updates an existing release
func (a *Adapter) updateRelease(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	releaseID, ok := params["release_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid release_id parameter")
	}

	// Create release update options
	releaseUpdate := &github.RepositoryRelease{}
	updated := false

	// Add tag name if provided
	if tagParam, ok := params["tag_name"].(string); ok {
		releaseUpdate.TagName = github.String(tagParam)
		updated = true
	}

	// Add target commitish if provided
	if targetParam, ok := params["target_commitish"].(string); ok {
		releaseUpdate.TargetCommitish = github.String(targetParam)
		updated = true
	}

	// Add name if provided
	if nameParam, ok := params["name"].(string); ok {
		releaseUpdate.Name = github.String(nameParam)
		updated = true
	}

	// Add body if provided
	if bodyParam, ok := params["body"].(string); ok {
		releaseUpdate.Body = github.String(bodyParam)
		updated = true
	}

	// Add draft status if provided
	if draftParam, ok := params["draft"].(bool); ok {
		releaseUpdate.Draft = github.Bool(draftParam)
		updated = true
	}

	// Add prerelease status if provided
	if prereleaseParam, ok := params["prerelease"].(bool); ok {
		releaseUpdate.Prerelease = github.Bool(prereleaseParam)
		updated = true
	}

	// If nothing to update, return an error
	if !updated {
		return nil, fmt.Errorf("no update parameters provided")
	}

	// Update the release
	release, resp, err := a.client.Repositories.EditRelease(
		ctx,
		owner,
		repo,
		int64(releaseID),
		releaseUpdate,
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to update release")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"id":           release.GetID(),
		"name":         release.GetName(),
		"tag_name":     release.GetTagName(),
		"html_url":     release.GetHTMLURL(),
		"created_at":   release.GetCreatedAt(),
		"published_at": release.GetPublishedAt(),
		"draft":        release.GetDraft(),
		"prerelease":   release.GetPrerelease(),
		"author":       release.GetAuthor().GetLogin(),
	}, nil
}

// deleteRelease deletes an existing release
func (a *Adapter) deleteRelease(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	releaseID, ok := params["release_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid release_id parameter")
	}

	// Delete the release
	resp, err := a.client.Repositories.DeleteRelease(
		ctx,
		owner,
		repo,
		int64(releaseID),
	)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to delete release")
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"success":    true,
		"release_id": int64(releaseID),
	}, nil
}

// createIssue creates a new issue
func (a *Adapter) createIssue(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	title, ok := params["title"].(string)
	if !ok {
		return nil, fmt.Errorf("missing title parameter")
	}

	body := ""
	if bodyParam, ok := params["body"].(string); ok {
		body = bodyParam
	}

	// Create issue request
	issueRequest := &github.IssueRequest{
		Title: github.String(title),
		Body:  github.String(body),
	}

	// Add labels if provided
	if labelsParam, ok := params["labels"].([]interface{}); ok {
		labels := make([]string, 0, len(labelsParam))
		for _, label := range labelsParam {
			if labelStr, ok := label.(string); ok {
				labels = append(labels, labelStr)
			}
		}
		issueRequest.Labels = &labels
	}

	// Add assignees if provided
	if assigneesParam, ok := params["assignees"].([]interface{}); ok {
		assignees := make([]string, 0, len(assigneesParam))
		for _, assignee := range assigneesParam {
			if assigneeStr, ok := assignee.(string); ok {
				assignees = append(assignees, assigneeStr)
			}
		}
		issueRequest.Assignees = &assignees
	}

	// Create the issue
	issue, _, err := a.client.Issues.Create(ctx, owner, repo, issueRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return map[string]interface{}{
		"issue_number": issue.GetNumber(),
		"html_url":     issue.GetHTMLURL(),
	}, nil
}

// closeIssue closes an issue
func (a *Adapter) closeIssue(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	issueNumber, ok := params["issue_number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid issue_number parameter")
	}

	// Create issue update request
	issueRequest := &github.IssueRequest{
		State: github.String("closed"),
	}

	// Update the issue
	issue, _, err := a.client.Issues.Edit(
		ctx,
		owner,
		repo,
		int(issueNumber),
		issueRequest,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to close issue: %w", err)
	}

	return map[string]interface{}{
		"issue_number": issue.GetNumber(),
		"state":        issue.GetState(),
		"html_url":     issue.GetHTMLURL(),
	}, nil
}

// createPullRequest creates a new pull request
func (a *Adapter) createPullRequest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	title, ok := params["title"].(string)
	if !ok {
		return nil, fmt.Errorf("missing title parameter")
	}

	head, ok := params["head"].(string)
	if !ok {
		return nil, fmt.Errorf("missing head parameter")
	}

	base, ok := params["base"].(string)
	if !ok {
		return nil, fmt.Errorf("missing base parameter")
	}

	body := ""
	if bodyParam, ok := params["body"].(string); ok {
		body = bodyParam
	}

	// Create pull request
	newPR := &github.NewPullRequest{
		Title: github.String(title),
		Head:  github.String(head),
		Base:  github.String(base),
		Body:  github.String(body),
	}

	// If draft parameter is provided and true
	if draftParam, ok := params["draft"].(bool); ok && draftParam {
		newPR.Draft = github.Bool(true)
	}

	// If maintainer_can_modify parameter is provided
	if maintainerCanModifyParam, ok := params["maintainer_can_modify"].(bool); ok {
		newPR.MaintainerCanModify = github.Bool(maintainerCanModifyParam)
	}

	pr, _, err := a.client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	// If reviewers are provided, add them to the PR
	if reviewersParam, ok := params["reviewers"].([]interface{}); ok && len(reviewersParam) > 0 {
		reviewers := make([]string, 0, len(reviewersParam))
		for _, reviewer := range reviewersParam {
			if reviewerStr, ok := reviewer.(string); ok {
				reviewers = append(reviewers, reviewerStr)
			}
		}
		
		if len(reviewers) > 0 {
			reviewersRequest := github.ReviewersRequest{
				Reviewers: reviewers,
			}
			
			_, _, err = a.client.PullRequests.RequestReviewers(
				ctx,
				owner,
				repo,
				pr.GetNumber(),
				reviewersRequest,
			)
			
			if err != nil {
				log.Printf("Warning: Failed to add reviewers to PR #%d: %v", pr.GetNumber(), err)
			}
		}
	}

	// If labels are provided, add them to the PR
	if labelsParam, ok := params["labels"].([]interface{}); ok && len(labelsParam) > 0 {
		labels := make([]string, 0, len(labelsParam))
		for _, label := range labelsParam {
			if labelStr, ok := label.(string); ok {
				labels = append(labels, labelStr)
			}
		}
		
		if len(labels) > 0 {
			_, _, err = a.client.Issues.AddLabelsToIssue(
				ctx,
				owner,
				repo,
				pr.GetNumber(),
				labels,
			)
			
			if err != nil {
				log.Printf("Warning: Failed to add labels to PR #%d: %v", pr.GetNumber(), err)
			}
		}
	}

	return map[string]interface{}{
		"pull_number": pr.GetNumber(),
		"html_url":    pr.GetHTMLURL(),
		"created_at":  pr.GetCreatedAt(),
		"head_sha":    pr.GetHead().GetSHA(),
	}, nil
}

// mergePullRequest merges a pull request
func (a *Adapter) mergePullRequest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	pullNumber, ok := params["pull_number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid pull_number parameter")
	}

	commitMessage := ""
	if msgParam, ok := params["commit_message"].(string); ok {
		commitMessage = msgParam
	}

	// Parse merge method if provided
	mergeMethod := ""
	if methodParam, ok := params["merge_method"].(string); ok {
		validMethods := map[string]bool{
			"merge":  true,
			"squash": true,
			"rebase": true,
		}
		
		if validMethods[methodParam] {
			mergeMethod = methodParam
		}
	}

	// Create merge request options
	options := &github.PullRequestOptions{}
	
	if commitMessage != "" {
		options.CommitTitle = commitMessage
	}
	
	if mergeMethod != "" {
		options.MergeMethod = mergeMethod
	}

	result, _, err := a.client.PullRequests.Merge(
		ctx,
		owner,
		repo,
		int(pullNumber),
		commitMessage,
		options,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to merge pull request: %w", err)
	}

	return map[string]interface{}{
		"merged":      result.GetMerged(),
		"message":     result.GetMessage(),
		"sha":         result.GetSHA(),
	}, nil
}

// addComment adds a comment to an issue or pull request
func (a *Adapter) addComment(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	number, ok := params["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid number parameter")
	}

	body, ok := params["body"].(string)
	if !ok {
		return nil, fmt.Errorf("missing body parameter")
	}

	// Create comment
	comment := &github.IssueComment{
		Body: github.String(body),
	}

	result, _, err := a.client.Issues.CreateComment(
		ctx,
		owner,
		repo,
		int(number),
		comment,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment: %w", err)
	}

	return map[string]interface{}{
		"id":       result.GetID(),
		"html_url": result.GetHTMLURL(),
		"created_at": result.GetCreatedAt(),
		"user":     result.GetUser().GetLogin(),
	}, nil
}

// createBranch creates a new branch in a repository
func (a *Adapter) createBranch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	branchName, ok := params["branch"].(string)
	if !ok {
		return nil, fmt.Errorf("missing branch parameter")
	}

	baseSHA, ok := params["sha"].(string)
	if !ok {
		// If no SHA provided, use the default branch's HEAD
		repoInfo, _, err := a.client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository: %w", err)
		}

		defaultBranch := repoInfo.GetDefaultBranch()
		
		// Get the reference to the default branch
		ref, _, err := a.client.Git.GetRef(ctx, owner, repo, "heads/"+defaultBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to get reference to default branch: %w", err)
		}
		
		baseSHA = ref.GetObject().GetSHA()
	}

	// Create a reference to the new branch
	refName := "refs/heads/" + branchName
	newRef := &github.Reference{
		Ref: github.String(refName),
		Object: &github.GitObject{
			SHA: github.String(baseSHA),
		},
	}

	ref, _, err := a.client.Git.CreateRef(ctx, owner, repo, newRef)
	if err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}

	return map[string]interface{}{
		"ref": ref.GetRef(),
		"url": ref.GetURL(),
		"sha": ref.GetObject().GetSHA(),
	}, nil
}

// createWebhook creates a new webhook in a repository
func (a *Adapter) createWebhook(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	url, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing url parameter")
	}

	// Set default content type to JSON if not specified
	contentType := "json"
	if contentTypeParam, ok := params["content_type"].(string); ok {
		contentType = contentTypeParam
	}

	// Check for secret
	secret := ""
	if secretParam, ok := params["secret"].(string); ok {
		secret = secretParam
	}

	// Create webhook config
	config := map[string]interface{}{
		"url":          url,
		"content_type": contentType,
		"secret":       secret,
	}

	// Set default insecure_ssl to 0 (secure)
	config["insecure_ssl"] = "0"
	if insecureParam, ok := params["insecure_ssl"].(string); ok {
		config["insecure_ssl"] = insecureParam
	}

	// Define webhook events
	events := []string{"push"}
	if eventsParam, ok := params["events"].([]interface{}); ok {
		events = make([]string, 0, len(eventsParam))
		for _, event := range eventsParam {
			if eventStr, ok := event.(string); ok {
				events = append(events, eventStr)
			}
		}
	}

	// Create webhook
	hook := &github.Hook{
		Config: config,
		Events: events,
		Active: github.Bool(true),
	}

	result, _, err := a.client.Repositories.CreateHook(ctx, owner, repo, hook)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	return map[string]interface{}{
		"id":      result.GetID(),
		"url":     result.GetURL(),
		"events":  result.Events,
		"created": result.GetCreatedAt(),
	}, nil
}

// deleteWebhook deletes a webhook from a repository
func (a *Adapter) deleteWebhook(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	hookID, ok := params["hook_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid hook_id parameter")
	}

	_, err := a.client.Repositories.DeleteHook(ctx, owner, repo, int64(hookID))
	if err != nil {
		return nil, fmt.Errorf("failed to delete webhook: %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"hook_id": hookID,
	}, nil
}

// checkWorkflowRun checks the status of a workflow run
func (a *Adapter) checkWorkflowRun(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	runID, ok := params["run_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid run_id parameter")
	}

	run, _, err := a.client.Actions.GetWorkflowRunByID(ctx, owner, repo, int64(runID))
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow run: %w", err)
	}

	return map[string]interface{}{
		"id":         run.GetID(),
		"name":       run.GetName(),
		"status":     run.GetStatus(),
		"conclusion": run.GetConclusion(),
		"html_url":   run.GetHTMLURL(),
		"created_at": run.GetCreatedAt(),
		"updated_at": run.GetUpdatedAt(),
	}, nil
}

// triggerWorkflow triggers a workflow dispatch event
func (a *Adapter) triggerWorkflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	workflowIDStr, ok := params["workflow_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing workflow_id parameter")
	}

	// Get the reference
	ref, ok := params["ref"].(string)
	if !ok {
		// If no ref provided, use the default branch
		repoInfo, _, err := a.client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository: %w", err)
		}
		
		ref = repoInfo.GetDefaultBranch()
	}

	// Check if we have a numeric ID or a filename
	workflowID, err := strconv.ParseInt(workflowIDStr, 10, 64)
	isNumericID := err == nil

	if isNumericID {
		// For numeric IDs, trigger directly via workflow dispatch API
		log.Printf("Triggering workflow with ID %d and ref %s", workflowID, ref)
		
		// For go-github v45, let's skip the inputs for now to simplify
		// The core functionality will still work
		_, err = a.client.Actions.CreateWorkflowDispatchEventByID(
			ctx,
			owner,
			repo,
			workflowID,
			github.CreateWorkflowDispatchEventRequest{Ref: ref},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to trigger workflow: %w", err)
		}
	} else {
		// For filenames, use repository dispatch as fallback
		log.Printf("Triggering workflow with filename %s via repository dispatch", workflowIDStr)
		
		// Create a simpler event type for compatibility
		eventType := "workflow_dispatch_" + workflowIDStr
		
		// Keep payload simple for compatibility
		// We'll just use the required parameters
		_, _, err = a.client.Repositories.Dispatch(
			ctx,
			owner,
			repo,
			github.DispatchRequestOptions{
				EventType: eventType,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to trigger repository dispatch: %w", err)
		}
	}

	return map[string]interface{}{
		"success":     true,
		"workflow_id": workflowIDStr,
		"ref":         ref,
	}, nil
}

// listTeamMembers lists the members of a team
func (a *Adapter) listTeamMembers(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	org, ok := params["org"].(string)
	if !ok {
		return nil, fmt.Errorf("missing org parameter")
	}

	team, ok := params["team"].(string)
	if !ok {
		return nil, fmt.Errorf("missing team parameter")
	}

	// Create list options
	listOptions := &github.TeamListTeamMembersOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// If role parameter is provided
	if roleParam, ok := params["role"].(string); ok {
		listOptions.Role = roleParam
	}

	// Get team members
	members, _, err := a.client.Teams.ListTeamMembersBySlug(ctx, org, team, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list team members: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(members))
	for _, member := range members {
		memberMap := map[string]interface{}{
			"id":        member.GetID(),
			"login":     member.GetLogin(),
			"avatar_url": member.GetAvatarURL(),
			"html_url":  member.GetHTMLURL(),
			"site_admin": member.GetSiteAdmin(),
			"type":      member.GetType(),
		}
		result = append(result, memberMap)
	}

	return map[string]interface{}{
		"members": result,
	}, nil
}

// addTeamMember adds a user to a team
func (a *Adapter) addTeamMember(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	org, ok := params["org"].(string)
	if !ok {
		return nil, fmt.Errorf("missing org parameter")
	}

	team, ok := params["team"].(string)
	if !ok {
		return nil, fmt.Errorf("missing team parameter")
	}

	username, ok := params["username"].(string)
	if !ok {
		return nil, fmt.Errorf("missing username parameter")
	}

	// Default role is member
	role := "member"
	if roleParam, ok := params["role"].(string); ok {
		if roleParam == "maintainer" {
			role = "maintainer"
		}
	}

	// Add user to team
	_, _, err := a.client.Teams.AddTeamMembershipBySlug(ctx, org, team, username, &github.TeamAddTeamMembershipOptions{
		Role: role,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add team member: %w", err)
	}

	return map[string]interface{}{
		"success":  true,
		"username": username,
		"role":     role,
	}, nil
}

// removeTeamMember removes a user from a team
func (a *Adapter) removeTeamMember(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	org, ok := params["org"].(string)
	if !ok {
		return nil, fmt.Errorf("missing org parameter")
	}

	team, ok := params["team"].(string)
	if !ok {
		return nil, fmt.Errorf("missing team parameter")
	}

	username, ok := params["username"].(string)
	if !ok {
		return nil, fmt.Errorf("missing username parameter")
	}

	// Remove user from team
	_, err := a.client.Teams.RemoveTeamMembershipBySlug(ctx, org, team, username)
	if err != nil {
		return nil, fmt.Errorf("failed to remove team member: %w", err)
	}

	return map[string]interface{}{
		"success":  true,
		"username": username,
	}, nil
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
		if rateLimitErr.Rate != nil {
			if a.rateLimits == nil {
				a.rateLimits = &github.RateLimits{}
			}
			
			if a.rateLimits.Core == nil {
				a.rateLimits.Core = &github.Rate{}
			}
			
			a.rateLimits.Core = rateLimitErr.Rate
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

// getRepositories gets repositories for a user or organization with pagination support
func (a *Adapter) getRepositories(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	var username string
	if usernameParam, ok := params["username"].(string); ok {
		username = usernameParam
	}

	// Set up list options
	listOptions := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{
			PerPage: a.config.DefaultPerPage,
		},
	}

	// Support pagination
	if pageParam, ok := params["page"].(float64); ok {
		listOptions.Page = int(pageParam)
	}
	
	if perPageParam, ok := params["per_page"].(float64); ok {
		listOptions.PerPage = int(perPageParam)
	}

	// Add filters
	if typeParam, ok := params["type"].(string); ok {
		listOptions.Type = typeParam
	}
	
	if sortParam, ok := params["sort"].(string); ok {
		listOptions.Sort = sortParam
	}
	
	if directionParam, ok := params["direction"].(string); ok {
		listOptions.Direction = directionParam
	}

	// Get repositories
	var repositories []*github.Repository
	var resp *github.Response
	var err error

	// Check if we should search by topics
	if topicsParam, ok := params["topics"].([]interface{}); ok && len(topicsParam) > 0 {
		// Convert topics to string slice
		topics := make([]string, 0, len(topicsParam))
		for _, topic := range topicsParam {
			if topicStr, ok := topic.(string); ok {
				topics = append(topics, topicStr)
			}
		}
		
		// Build a search query for repositories with these topics
		query := strings.Join(topics, " ")
		searchOpts := &github.SearchOptions{
			ListOptions: github.ListOptions{
				Page:    listOptions.Page,
				PerPage: listOptions.PerPage,
			},
		}
		
		// Add user/org filter if specified
		if username != "" {
			if strings.Contains(username, "/") {
				// Org name
				query = fmt.Sprintf("%s org:%s", query, strings.Split(username, "/")[0])
			} else {
				query = fmt.Sprintf("%s user:%s", query, username)
			}
		}
		
		// Search repositories by topics
		searchResults, resp, err := a.client.Search.Repositories(ctx, query, searchOpts)
		if err != nil {
			return nil, a.handleError(err, resp, "failed to search repositories by topics")
		}
		
		// Convert search results to repository maps
		result := make([]map[string]interface{}, 0, len(searchResults.Repositories))
		for _, repo := range searchResults.Repositories {
			repoMap := a.repositoryToMap(&repo)
			result = append(result, repoMap)
		}
		
		// Update stats
		a.stats.RequestsSuccess++
		a.stats.LastSuccessfulRequest = time.Now()
		
		return map[string]interface{}{
			"repositories": result,
			"total_count":  searchResults.GetTotal(),
			"page":         searchOpts.Page,
			"per_page":     searchOpts.PerPage,
		}, nil
	}

	// Standard repository listing
	if username == "" {
		// Get authenticated user's repositories
		repositories, resp, err = a.client.Repositories.List(ctx, "", listOptions)
	} else {
		// Check if this is an organization
		if strings.HasPrefix(username, "org:") || strings.HasPrefix(username, "org/") {
			// Extract org name
			orgName := strings.TrimPrefix(strings.TrimPrefix(username, "org:"), "org/")
			
			// Get repositories for the specified organization
			repositories, resp, err = a.client.Repositories.ListByOrg(ctx, orgName, &github.RepositoryListByOrgOptions{
				ListOptions: listOptions.ListOptions,
				Type:        listOptions.Type,
				Sort:        listOptions.Sort,
				Direction:   listOptions.Direction,
			})
		} else {
			// Get repositories for the specified user
			repositories, resp, err = a.client.Repositories.List(ctx, username, listOptions)
		}
	}

	if err != nil {
		return nil, a.handleError(err, resp, "failed to get repositories")
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(repositories))
	for _, repo := range repositories {
		repoMap := a.repositoryToMap(repo)
		result = append(result, repoMap)
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"repositories": result,
		"page":         listOptions.Page,
		"per_page":     listOptions.PerPage,
	}, nil
}

// repositoryToMap converts a GitHub repository to a map
func (a *Adapter) repositoryToMap(repo *github.Repository) map[string]interface{} {
	repoMap := map[string]interface{}{
		"id":            repo.GetID(),
		"name":          repo.GetName(),
		"full_name":     repo.GetFullName(),
		"html_url":      repo.GetHTMLURL(),
		"description":   repo.GetDescription(),
		"default_branch": repo.GetDefaultBranch(),
		"created_at":    repo.GetCreatedAt(),
		"updated_at":    repo.GetUpdatedAt(),
		"pushed_at":     repo.GetPushedAt(),
		"language":      repo.GetLanguage(),
		"private":       repo.GetPrivate(),
		"fork":          repo.GetFork(),
		"forks_count":   repo.GetForksCount(),
		"stars_count":   repo.GetStargazersCount(),
		"watchers_count": repo.GetWatchersCount(),
		"open_issues_count": repo.GetOpenIssuesCount(),
		"archived":      repo.GetArchived(),
		"disabled":      repo.GetDisabled(),
	}
	
	// Add owner information if available
	if repo.Owner != nil {
		repoMap["owner"] = map[string]interface{}{
			"login":      repo.Owner.GetLogin(),
			"id":         repo.Owner.GetID(),
			"avatar_url": repo.Owner.GetAvatarURL(),
			"html_url":   repo.Owner.GetHTMLURL(),
			"type":       repo.Owner.GetType(),
		}
	}
	
	// Add topics if available
	if repo.Topics != nil && len(repo.Topics) > 0 {
		repoMap["topics"] = repo.Topics
	}
	
	// Add license info if available
	if repo.License != nil {
		repoMap["license"] = map[string]interface{}{
			"key":  repo.License.GetKey(),
			"name": repo.License.GetName(),
			"spdx_id": repo.License.GetSPDXID(),
			"url":  repo.License.GetURL(),
		}
	}
	
	// Add permissions if available
	if repo.Permissions != nil {
		repoMap["permissions"] = map[string]interface{}{
			"admin": repo.Permissions.GetAdmin(),
			"push":  repo.Permissions.GetPush(),
			"pull":  repo.Permissions.GetPull(),
		}
	}
	
	return repoMap
}

// getRepository gets a specific repository
func (a *Adapter) getRepository(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	// Get repository
	repository, _, err := a.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	// Convert to map for easier serialization
	result := map[string]interface{}{
		"id":             repository.GetID(),
		"name":           repository.GetName(),
		"full_name":      repository.GetFullName(),
		"html_url":       repository.GetHTMLURL(),
		"description":    repository.GetDescription(),
		"default_branch": repository.GetDefaultBranch(),
		"created_at":     repository.GetCreatedAt(),
		"updated_at":     repository.GetUpdatedAt(),
		"pushed_at":      repository.GetPushedAt(),
		"language":       repository.GetLanguage(),
		"private":        repository.GetPrivate(),
		"fork":           repository.GetFork(),
		"forks_count":    repository.GetForksCount(),
		"stars_count":    repository.GetStargazersCount(),
		"watchers_count": repository.GetWatchersCount(),
		"open_issues_count": repository.GetOpenIssuesCount(),
		"has_issues":     repository.GetHasIssues(),
		"has_wiki":       repository.GetHasWiki(),
		"has_projects":   repository.GetHasProjects(),
		"archived":       repository.GetArchived(),
		"disabled":       repository.GetDisabled(),
	}

	// Add license info if available
	if repository.License != nil {
		result["license"] = repository.License.GetSPDXID()
	}

	// Add topics if available
	if repository.Topics != nil {
		result["topics"] = repository.Topics
	}

	return result, nil
}

// getPullRequests gets pull requests for a repository
func (a *Adapter) getPullRequests(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	// Set up list options
	listOptions := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	if stateParam, ok := params["state"].(string); ok {
		listOptions.State = stateParam
	}

	// Set sort and direction if provided
	if sortParam, ok := params["sort"].(string); ok {
		listOptions.Sort = sortParam
	}
	
	if directionParam, ok := params["direction"].(string); ok {
		listOptions.Direction = directionParam
	}

	// Set base and head filters if provided
	if baseParam, ok := params["base"].(string); ok {
		listOptions.Base = baseParam
	}
	
	if headParam, ok := params["head"].(string); ok {
		listOptions.Head = headParam
	}

	// Get pull requests
	pullRequests, _, err := a.client.PullRequests.List(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(pullRequests))
	for _, pr := range pullRequests {
		prMap := map[string]interface{}{
			"number":       pr.GetNumber(),
			"title":        pr.GetTitle(),
			"html_url":     pr.GetHTMLURL(),
			"state":        pr.GetState(),
			"created_at":   pr.GetCreatedAt(),
			"updated_at":   pr.GetUpdatedAt(),
			"closed_at":    pr.GetClosedAt(),
			"merged_at":    pr.GetMergedAt(),
			"user":         pr.GetUser().GetLogin(),
			"draft":        pr.GetDraft(),
			"head":         pr.GetHead().GetRef(),
			"base":         pr.GetBase().GetRef(),
			"mergeable":    pr.GetMergeable(),
			"mergeable_state": pr.GetMergeableState(),
			"comments":     pr.GetComments(),
			"commits":      pr.GetCommits(),
			"additions":    pr.GetAdditions(),
			"deletions":    pr.GetDeletions(),
			"changed_files": pr.GetChangedFiles(),
		}
		
		// Add labels if present
		if pr.Labels != nil && len(pr.Labels) > 0 {
			labels := make([]string, 0, len(pr.Labels))
			for _, label := range pr.Labels {
				labels = append(labels, label.GetName())
			}
			prMap["labels"] = labels
		}

		result = append(result, prMap)
	}

	return map[string]interface{}{
		"pull_requests": result,
	}, nil
}

// getIssues gets issues for a repository
func (a *Adapter) getIssues(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	// Set up list options
	listOptions := &github.IssueListByRepoOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	if stateParam, ok := params["state"].(string); ok {
		listOptions.State = stateParam
	}

	// Set sort and direction if provided
	if sortParam, ok := params["sort"].(string); ok {
		listOptions.Sort = sortParam
	}
	
	if directionParam, ok := params["direction"].(string); ok {
		listOptions.Direction = directionParam
	}

	// Add filter parameters if provided
	if assigneeParam, ok := params["assignee"].(string); ok {
		listOptions.Assignee = assigneeParam
	}
	
	if creatorParam, ok := params["creator"].(string); ok {
		listOptions.Creator = creatorParam
	}
	
	if mentionedParam, ok := params["mentioned"].(string); ok {
		listOptions.Mentioned = mentionedParam
	}
	
	if labelsParam, ok := params["labels"].([]interface{}); ok {
		labels := make([]string, 0, len(labelsParam))
		for _, label := range labelsParam {
			if labelStr, ok := label.(string); ok {
				labels = append(labels, labelStr)
			}
		}
		listOptions.Labels = labels
	}

	// Get issues
	issues, _, err := a.client.Issues.ListByRepo(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get issues: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(issues))
	for _, issue := range issues {
		// Skip pull requests
		if issue.GetPullRequestLinks() != nil {
			continue
		}

		issueMap := map[string]interface{}{
			"number":     issue.GetNumber(),
			"title":      issue.GetTitle(),
			"html_url":   issue.GetHTMLURL(),
			"state":      issue.GetState(),
			"created_at": issue.GetCreatedAt(),
			"updated_at": issue.GetUpdatedAt(),
			"closed_at":  issue.GetClosedAt(),
			"user":       issue.GetUser().GetLogin(),
			"body":       issue.GetBody(),
			"comments":   issue.GetComments(),
		}
		
		// Add assignees if present
		if issue.Assignees != nil && len(issue.Assignees) > 0 {
			assignees := make([]string, 0, len(issue.Assignees))
			for _, assignee := range issue.Assignees {
				assignees = append(assignees, assignee.GetLogin())
			}
			issueMap["assignees"] = assignees
		}
		
		// Add labels if present
		if issue.Labels != nil && len(issue.Labels) > 0 {
			labels := make([]string, 0, len(issue.Labels))
			for _, label := range issue.Labels {
				labels = append(labels, label.GetName())
			}
			issueMap["labels"] = labels
		}

		result = append(result, issueMap)
	}

	return map[string]interface{}{
		"issues": result,
	}, nil
}

// getBranches gets branches for a repository
func (a *Adapter) getBranches(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	// Set up list options
	listOptions := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// Check if we should exclude protected branches
	protected := false
	if protectedParam, ok := params["protected"].(bool); ok {
		protected = protectedParam
	}

	// Get branches
	branches, _, err := a.client.Repositories.ListBranches(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(branches))
	for _, branch := range branches {
		// Skip protected branches if requested
		if protected && !branch.GetProtected() {
			continue
		}
		
		branchMap := map[string]interface{}{
			"name":      branch.GetName(),
			"protected": branch.GetProtected(),
		}
		
		// Add commit info if available
		if branch.Commit != nil {
			branchMap["commit_sha"] = branch.Commit.GetSHA()
			branchMap["commit_url"] = branch.Commit.GetURL()
		}

		result = append(result, branchMap)
	}

	return map[string]interface{}{
		"branches": result,
	}, nil
}

// getTeams gets teams for an organization
func (a *Adapter) getTeams(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	org, ok := params["org"].(string)
	if !ok {
		return nil, fmt.Errorf("missing org parameter")
	}

	// Set up list options
	listOptions := &github.ListOptions{
		PerPage: 100,
	}

	// Get teams
	teams, _, err := a.client.Teams.ListTeams(ctx, org, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(teams))
	for _, team := range teams {
		teamMap := map[string]interface{}{
			"id":          team.GetID(),
			"name":        team.GetName(),
			"slug":        team.GetSlug(),
			"description": team.GetDescription(),
			"privacy":     team.GetPrivacy(),
			"html_url":    team.GetHTMLURL(),
			"members_count": team.GetMembersCount(),
			"repos_count": team.GetReposCount(),
		}
		
		result = append(result, teamMap)
	}

	return map[string]interface{}{
		"teams": result,
	}, nil
}

// getWorkflowRuns gets workflow runs for a repository with pagination support
func (a *Adapter) getWorkflowRuns(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	// Set up list options
	listOptions := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{
			PerPage: a.config.DefaultPerPage,
		},
	}

	// Support pagination with page and per_page parameters
	if pageParam, ok := params["page"].(float64); ok {
		listOptions.Page = int(pageParam)
	}
	
	if perPageParam, ok := params["per_page"].(float64); ok {
		listOptions.PerPage = int(perPageParam)
	}

	// Add filter parameters if provided
	if actorParam, ok := params["actor"].(string); ok {
		listOptions.Actor = actorParam
	}
	
	if branchParam, ok := params["branch"].(string); ok {
		listOptions.Branch = branchParam
	}
	
	if eventParam, ok := params["event"].(string); ok {
		listOptions.Event = eventParam
	}
	
	if statusParam, ok := params["status"].(string); ok {
		listOptions.Status = statusParam
	}
	
	// Check if we should get a specific workflow run by ID
	if runIDParam, ok := params["run_id"].(float64); ok {
		// Get a specific workflow run
		run, resp, err := a.client.Actions.GetWorkflowRunByID(ctx, owner, repo, int64(runIDParam))
		if err != nil {
			return nil, a.handleError(err, resp, "failed to get workflow run")
		}
		
		// Convert to map for easier serialization
		runMap := a.workflowRunToMap(run)
		
		// Check for required approvals
		approvals, resp, err := a.client.Actions.ListRequiredWorkflowRunApprovalsForRepo(ctx, owner, repo, int64(runIDParam), nil)
		if err == nil && approvals != nil {
			approvalsData := make([]map[string]interface{}, 0, len(approvals))
			for _, approval := range approvals {
				approvalMap := map[string]interface{}{
					"environment": approval.GetEnvironment(),
					"state":       approval.GetState(),
					"comment":     approval.GetComment(),
					"user":        approval.GetUser().GetLogin(),
				}
				approvalsData = append(approvalsData, approvalMap)
			}
			runMap["approvals"] = approvalsData
		}
		
		// Update stats
		a.stats.RequestsSuccess++
		a.stats.LastSuccessfulRequest = time.Now()
		
		return runMap, nil
	}

	// Track all workflow runs for pagination
	allRuns := make([]map[string]interface{}, 0)
	
	// Check if we should get runs for a specific workflow
	if workflowIDParam, ok := params["workflow_id"].(string); ok {
		// Try to convert to a numeric ID first
		workflowID, err := strconv.ParseInt(workflowIDParam, 10, 64)
		if err == nil {
			// It's a numeric ID
			runs, resp, err := a.client.Actions.ListWorkflowRunsByID(ctx, owner, repo, workflowID, listOptions)
			if err != nil {
				return nil, a.handleError(err, resp, "failed to get workflow runs")
			}
			
			// Convert to map for easier serialization
			for _, run := range runs.WorkflowRuns {
				allRuns = append(allRuns, a.workflowRunToMap(run))
			}
			
			// Update stats
			a.stats.RequestsSuccess++
			a.stats.LastSuccessfulRequest = time.Now()
			
			return map[string]interface{}{
				"workflow_runs": allRuns,
				"total_count":   runs.GetTotalCount(),
				"page":          listOptions.Page,
				"per_page":      listOptions.PerPage,
			}, nil
		} else {
			// It's a filename
			runs, resp, err := a.client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowIDParam, listOptions)
			if err != nil {
				return nil, a.handleError(err, resp, "failed to get workflow runs")
			}
			
			// Convert to map for easier serialization
			for _, run := range runs.WorkflowRuns {
				allRuns = append(allRuns, a.workflowRunToMap(run))
			}
			
			// Update stats
			a.stats.RequestsSuccess++
			a.stats.LastSuccessfulRequest = time.Now()
			
			return map[string]interface{}{
				"workflow_runs": allRuns,
				"total_count":   runs.GetTotalCount(),
				"page":          listOptions.Page,
				"per_page":      listOptions.PerPage,
			}, nil
		}
	} else {
		// Get all workflow runs
		runs, resp, err := a.client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, listOptions)
		if err != nil {
			return nil, a.handleError(err, resp, "failed to get workflow runs")
		}
		
		// Convert to map for easier serialization
		for _, run := range runs.WorkflowRuns {
			allRuns = append(allRuns, a.workflowRunToMap(run))
		}
		
		// Update stats
		a.stats.RequestsSuccess++
		a.stats.LastSuccessfulRequest = time.Now()
		
		return map[string]interface{}{
			"workflow_runs": allRuns,
			"total_count":   runs.GetTotalCount(),
			"page":          listOptions.Page,
			"per_page":      listOptions.PerPage,
		}, nil
	}
}

// workflowRunToMap converts a workflow run to a map
func (a *Adapter) workflowRunToMap(run *github.WorkflowRun) map[string]interface{} {
	runMap := map[string]interface{}{
		"id":            run.GetID(),
		"name":          run.GetName(),
		"workflow_id":   run.GetWorkflowID(),
		"status":        run.GetStatus(),
		"conclusion":    run.GetConclusion(),
		"html_url":      run.GetHTMLURL(),
		"created_at":    run.GetCreatedAt(),
		"updated_at":    run.GetUpdatedAt(),
		"run_number":    run.GetRunNumber(),
		"event":         run.GetEvent(),
		"head_branch":   run.GetHeadBranch(),
		"head_sha":      run.GetHeadSHA(),
		"run_attempt":   run.GetRunAttempt(),
		"display_title": run.GetDisplayTitle(),
	}
	
	// Add title if available (using run.GetName() as fallback)
	if run.GetName() != "" {
		runMap["title"] = run.GetName()
	}
	
	// Add repository information if available
	if run.Repository != nil {
		runMap["repository"] = map[string]interface{}{
			"id":        run.Repository.GetID(),
			"name":      run.Repository.GetName(),
			"full_name": run.Repository.GetFullName(),
			"html_url":  run.Repository.GetHTMLURL(),
		}
	}
	
	// Add actor information if available
	if run.Actor != nil {
		runMap["actor"] = map[string]interface{}{
			"id":        run.Actor.GetID(),
			"login":     run.Actor.GetLogin(),
			"avatar_url": run.Actor.GetAvatarURL(),
			"html_url":  run.Actor.GetHTMLURL(),
		}
	}
	
	return runMap
}

// enableWorkflow enables a workflow in a repository
func (a *Adapter) enableWorkflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	workflowID, ok := params["workflow_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing workflow_id parameter")
	}

	// Try to convert to a numeric ID first
	id, err := strconv.ParseInt(workflowID, 10, 64)
	var resp *github.Response
	
	if err == nil {
		// It's a numeric ID
		resp, err = a.client.Actions.EnableWorkflowByID(ctx, owner, repo, id)
	} else {
		// It's a filename
		resp, err = a.client.Actions.EnableWorkflowByFileName(ctx, owner, repo, workflowID)
	}
	
	if err != nil {
		return nil, a.handleError(err, resp, "failed to enable workflow")
	}
	
	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()
	
	return map[string]interface{}{
		"success":     true,
		"workflow_id": workflowID,
	}, nil
}

// disableWorkflow disables a workflow in a repository
func (a *Adapter) disableWorkflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	workflowID, ok := params["workflow_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing workflow_id parameter")
	}

	// Try to convert to a numeric ID first
	id, err := strconv.ParseInt(workflowID, 10, 64)
	var resp *github.Response
	
	if err == nil {
		// It's a numeric ID
		resp, err = a.client.Actions.DisableWorkflowByID(ctx, owner, repo, id)
	} else {
		// It's a filename
		resp, err = a.client.Actions.DisableWorkflowByFileName(ctx, owner, repo, workflowID)
	}
	
	if err != nil {
		return nil, a.handleError(err, resp, "failed to disable workflow")
	}
	
	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()
	
	return map[string]interface{}{
		"success":     true,
		"workflow_id": workflowID,
	}, nil
}

// approveWorkflowRun approves a pending workflow run
func (a *Adapter) approveWorkflowRun(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	runID, ok := params["run_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid run_id parameter")
	}

	environmentName, ok := params["environment"].(string)
	if !ok {
		return nil, fmt.Errorf("missing environment parameter")
	}

	// Create the approval request
	var comment string
	if commentParam, ok := params["comment"].(string); ok {
		comment = commentParam
	}

	// Create the approval payload
	approvalRequest := &github.ApproveWorkflowRunRequest{
		Environment: github.String(environmentName),
		Comment:     github.String(comment),
	}

	// Send the approval
	resp, err := a.client.Actions.ApproveWorkflowRun(ctx, owner, repo, int64(runID), approvalRequest)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to approve workflow run")
	}
	
	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()
	
	return map[string]interface{}{
		"success":  true,
		"run_id":   int64(runID),
		"environment": environmentName,
	}, nil
}

// rejectWorkflowRun rejects a pending workflow run
func (a *Adapter) rejectWorkflowRun(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	runID, ok := params["run_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid run_id parameter")
	}

	environmentName, ok := params["environment"].(string)
	if !ok {
		return nil, fmt.Errorf("missing environment parameter")
	}

	// Create the rejection request
	var comment string
	if commentParam, ok := params["comment"].(string); ok {
		comment = commentParam
	}

	// Create the rejection payload
	rejectionRequest := &github.RejectWorkflowRunRequest{
		Environment: github.String(environmentName),
		Comment:     github.String(comment),
	}

	// Send the rejection
	resp, err := a.client.Actions.RejectWorkflowRun(ctx, owner, repo, int64(runID), rejectionRequest)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to reject workflow run")
	}
	
	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()
	
	return map[string]interface{}{
		"success":  true,
		"run_id":   int64(runID),
		"environment": environmentName,
	}, nil
}

// getWorkflows gets workflows for a repository
func (a *Adapter) getWorkflows(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	// Set up list options with pagination support
	listOptions := &github.ListOptions{
		PerPage: a.config.DefaultPerPage,
	}

	// Support pagination
	if pageParam, ok := params["page"].(float64); ok {
		listOptions.Page = int(pageParam)
	}
	
	if perPageParam, ok := params["per_page"].(float64); ok {
		listOptions.PerPage = int(perPageParam)
	}

	// Get workflows
	workflows, resp, err := a.client.Actions.ListWorkflows(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to get workflows")
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(workflows.Workflows))
	for _, workflow := range workflows.Workflows {
		workflowMap := map[string]interface{}{
			"id":         workflow.GetID(),
			"node_id":    workflow.GetNodeID(),
			"name":       workflow.GetName(),
			"path":       workflow.GetPath(),
			"state":      workflow.GetState(),
			"created_at": workflow.GetCreatedAt(),
			"updated_at": workflow.GetUpdatedAt(),
			"url":        workflow.GetURL(),
			"html_url":   workflow.GetHTMLURL(),
			"badge_url":  workflow.GetBadgeURL(),
		}
		result = append(result, workflowMap)
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	return map[string]interface{}{
		"workflows":   result,
		"total_count": workflows.GetTotalCount(),
		"page":        listOptions.Page,
		"per_page":    listOptions.PerPage,
	}, nil
}

// getCommits gets commits for a repository
func (a *Adapter) getCommits(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	// Set up list options
	listOptions := &github.CommitsListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// Add filter parameters if provided
	if shaParam, ok := params["sha"].(string); ok {
		listOptions.SHA = shaParam
	}
	
	if pathParam, ok := params["path"].(string); ok {
		listOptions.Path = pathParam
	}
	
	if authorParam, ok := params["author"].(string); ok {
		listOptions.Author = authorParam
	}
	
	if sinceParam, ok := params["since"].(string); ok {
		sinceTime, err := time.Parse(time.RFC3339, sinceParam)
		if err == nil {
			listOptions.Since = sinceTime
		}
	}
	
	if untilParam, ok := params["until"].(string); ok {
		untilTime, err := time.Parse(time.RFC3339, untilParam)
		if err == nil {
			listOptions.Until = untilTime
		}
	}

	// Get commits
	commits, _, err := a.client.Repositories.ListCommits(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(commits))
	for _, commit := range commits {
		commitMap := map[string]interface{}{
			"sha":       commit.GetSHA(),
			"html_url":  commit.GetHTMLURL(),
			"commit": map[string]interface{}{
				"message":   commit.Commit.GetMessage(),
				"author":    commit.Commit.GetAuthor().GetName(),
				"committer": commit.Commit.GetCommitter().GetName(),
				"comment_count": commit.Commit.GetCommentCount(),
				"tree": map[string]interface{}{
					"sha": commit.Commit.GetTree().GetSHA(),
				},
			},
		}
		
		// Add author info if available
		if commit.Author != nil {
			commitMap["author"] = map[string]interface{}{
				"login":      commit.Author.GetLogin(),
				"id":         commit.Author.GetID(),
				"avatar_url": commit.Author.GetAvatarURL(),
				"html_url":   commit.Author.GetHTMLURL(),
			}
		}
		
		// Add stats if available
		if commit.Stats != nil {
			commitMap["stats"] = map[string]interface{}{
				"additions": commit.Stats.GetAdditions(),
				"deletions": commit.Stats.GetDeletions(),
				"total":     commit.Stats.GetTotal(),
			}
		}

		result = append(result, commitMap)
	}

	return map[string]interface{}{
		"commits": result,
	}, nil
}

// searchCode searches code in a repository with improved functionality
func (a *Adapter) searchCode(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("missing query parameter")
	}

	// Set up search options
	searchOptions := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: a.config.DefaultPerPage,
		},
	}

	// Support pagination
	if pageParam, ok := params["page"].(float64); ok {
		searchOptions.Page = int(pageParam)
	}
	
	if perPageParam, ok := params["per_page"].(float64); ok {
		searchOptions.PerPage = int(perPageParam)
	}

	// Add sorting options if provided
	if sortParam, ok := params["sort"].(string); ok {
		searchOptions.Sort = sortParam
	}
	
	if orderParam, ok := params["order"].(string); ok {
		searchOptions.Order = orderParam
	}

	// Build the search query with filters
	queryParts := []string{query}

	// Add repo filter if provided
	if ownerParam, ok := params["owner"].(string); ok {
		if repoParam, ok := params["repo"].(string); ok {
			queryParts = append(queryParts, fmt.Sprintf("repo:%s/%s", ownerParam, repoParam))
		} else {
			// Check if it's an organization
			if strings.HasPrefix(ownerParam, "org:") || strings.HasPrefix(ownerParam, "org/") {
				// Extract org name
				orgName := strings.TrimPrefix(strings.TrimPrefix(ownerParam, "org:"), "org/")
				queryParts = append(queryParts, fmt.Sprintf("org:%s", orgName))
			} else {
				queryParts = append(queryParts, fmt.Sprintf("user:%s", ownerParam))
			}
		}
	}

	// Add language filter if provided
	if langParam, ok := params["language"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("language:%s", langParam))
	}

	// Add path filter if provided
	if pathParam, ok := params["path"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("path:%s", pathParam))
	}

	// Add filename filter if provided
	if filenameParam, ok := params["filename"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("filename:%s", filenameParam))
	}

	// Add extension filter if provided
	if extensionParam, ok := params["extension"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("extension:%s", extensionParam))
	}

	// Add size filter if provided
	if sizeParam, ok := params["size"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("size:%s", sizeParam))
	}

	// Add fork filter if provided
	if forkParam, ok := params["fork"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("fork:%s", forkParam))
	}

	// Combine all query parts
	combinedQuery := strings.Join(queryParts, " ")

	// Perform the code search with retry logic on rate limit
	var results *github.CodeSearchResult
	var resp *github.Response
	var err error
	
	for attempt := 0; attempt <= a.config.RetryMax; attempt++ {
		// If this is a retry attempt, add a delay with exponential backoff
		if attempt > 0 {
			backoffDuration := a.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			log.Printf("Retrying code search (attempt %d/%d) after %s", 
				attempt, a.config.RetryMax, backoffDuration)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
				// Continue after waiting
			}
		}
		
		// Perform the search
		results, resp, err = a.client.Search.Code(ctx, combinedQuery, searchOptions)
		
		// Check if we encountered a rate limit error
		var rateLimitErr *github.RateLimitError
		if err != nil && errors.As(err, &rateLimitErr) {
			a.stats.RateLimitHits++
			
			// If we're not configured to retry on rate limit, just return the error
			if !a.config.EnableRetryOnRateLimit {
				return nil, a.handleError(err, resp, "failed to search code")
			}
			
			// Otherwise, wait for the rate limit reset time if this isn't the last attempt
			if attempt < a.config.RetryMax && rateLimitErr.Rate != nil {
				resetTime := time.Until(rateLimitErr.Rate.Reset.Time)
				if resetTime > 0 {
					log.Printf("Rate limit exceeded. Waiting %s for reset...", resetTime)
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(resetTime + time.Second): // Add a buffer
						// Continue after waiting
					}
				}
			}
			
			// Continue to the next attempt
			continue
		}
		
		// If there was no rate limit error, break out of the retry loop
		break
	}
	
	// Handle any errors after exhausting retries
	if err != nil {
		return nil, a.handleError(err, resp, "failed to search code")
	}

	// Convert to map for easier serialization
	codeResults := make([]map[string]interface{}, 0, len(results.CodeResults))
	for _, result := range results.CodeResults {
		resultMap := map[string]interface{}{
			"name":        result.GetName(),
			"path":        result.GetPath(),
			"sha":         result.GetSHA(),
			"html_url":    result.GetHTMLURL(),
			"git_url":     result.GetGitURL(),
			"repository": map[string]interface{}{
				"id":        result.Repository.GetID(),
				"name":      result.Repository.GetName(),
				"full_name": result.Repository.GetFullName(),
				"html_url":  result.Repository.GetHTMLURL(),
				"private":   result.Repository.GetPrivate(),
			},
		}
		
		// Add text matches if available
		if result.TextMatches != nil && len(result.TextMatches) > 0 {
			matches := make([]map[string]interface{}, 0, len(result.TextMatches))
			for _, match := range result.TextMatches {
				matchMap := map[string]interface{}{
					"object_url":  match.GetObjectURL(),
					"object_type": match.GetObjectType(),
					"property":    match.GetProperty(),
					"fragment":    match.GetFragment(),
				}
				matches = append(matches, matchMap)
			}
			resultMap["text_matches"] = matches
		}
		
		codeResults = append(codeResults, resultMap)
	}

	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()

	// Check if we're getting close to the rate limit after the search
	if resp != nil && resp.Rate.Remaining < int(a.config.RateLimitThreshold) {
		log.Printf("Warning: API rate limit threshold reached. Remaining: %d/%d. Resets at: %s",
			resp.Rate.Remaining, resp.Rate.Limit, resp.Rate.Reset.Format(time.RFC3339))
		a.stats.RateLimitHits++
	}

	return map[string]interface{}{
		"total_count":  results.GetTotal(),
		"items":        codeResults,
		"query":        combinedQuery,
		"incomplete_results": results.GetIncompleteResults(),
		"page":         searchOptions.Page,
		"per_page":     searchOptions.PerPage,
	}, nil
}

// searchRepositories searches for repositories with advanced filters
func (a *Adapter) searchRepositories(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("missing query parameter")
	}

	// Set up search options
	searchOptions := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: a.config.DefaultPerPage,
		},
	}

	// Support pagination
	if pageParam, ok := params["page"].(float64); ok {
		searchOptions.Page = int(pageParam)
	}
	
	if perPageParam, ok := params["per_page"].(float64); ok {
		searchOptions.PerPage = int(perPageParam)
	}

	// Add sorting options if provided
	if sortParam, ok := params["sort"].(string); ok {
		searchOptions.Sort = sortParam
	}
	
	if orderParam, ok := params["order"].(string); ok {
		searchOptions.Order = orderParam
	}

	// Build the search query with filters
	queryParts := []string{query}
	
	// Add various repo-specific filters
	if langParam, ok := params["language"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("language:%s", langParam))
	}
	
	if topicsParam, ok := params["topics"].([]interface{}); ok && len(topicsParam) > 0 {
		for _, topic := range topicsParam {
			if topicStr, ok := topic.(string); ok {
				queryParts = append(queryParts, fmt.Sprintf("topic:%s", topicStr))
			}
		}
	}
	
	if starsParam, ok := params["stars"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("stars:%s", starsParam))
	}
	
	if forkParam, ok := params["fork"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("fork:%s", forkParam))
	}
	
	if sizeParam, ok := params["size"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("size:%s", sizeParam))
	}
	
	if userParam, ok := params["user"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("user:%s", userParam))
	}
	
	if orgParam, ok := params["org"].(string); ok {
		queryParts = append(queryParts, fmt.Sprintf("org:%s", orgParam))
	}
	
	// Combine all query parts
	combinedQuery := strings.Join(queryParts, " ")
	
	// Perform the repository search
	results, resp, err := a.client.Search.Repositories(ctx, combinedQuery, searchOptions)
	if err != nil {
		return nil, a.handleError(err, resp, "failed to search repositories")
	}
	
	// Convert to map for easier serialization
	repoResults := make([]map[string]interface{}, 0, len(results.Repositories))
	for _, repo := range results.Repositories {
		repoMap := a.repositoryToMap(&repo)
		repoResults = append(repoResults, repoMap)
	}
	
	// Update stats
	a.stats.RequestsSuccess++
	a.stats.LastSuccessfulRequest = time.Now()
	
	// Check if we're getting close to the rate limit after the search
	if resp != nil && resp.Rate.Remaining < int(a.config.RateLimitThreshold) {
		log.Printf("Warning: API rate limit threshold reached. Remaining: %d/%d. Resets at: %s",
			resp.Rate.Remaining, resp.Rate.Limit, resp.Rate.Reset.Format(time.RFC3339))
		a.stats.RateLimitHits++
	}
	
	return map[string]interface{}{
		"total_count":  results.GetTotal(),
		"items":        repoResults,
		"query":        combinedQuery,
		"incomplete_results": results.GetIncompleteResults(),
		"page":         searchOptions.Page,
		"per_page":     searchOptions.PerPage,
	}, nil
}

// getUsers gets users from GitHub
func (a *Adapter) getUsers(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Set up list options
	listOptions := &github.ListOptions{
		PerPage: 100,
	}

	// If since ID is provided
	var since int64
	if sinceParam, ok := params["since"].(float64); ok {
		since = int64(sinceParam)
	}

	// Get users
	users, _, err := a.client.Users.ListAll(ctx, &github.UserListOptions{
		Since:       since,
		ListOptions: *listOptions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(users))
	for _, user := range users {
		userMap := map[string]interface{}{
			"id":         user.GetID(),
			"login":      user.GetLogin(),
			"avatar_url": user.GetAvatarURL(),
			"html_url":   user.GetHTMLURL(),
			"type":       user.GetType(),
			"site_admin": user.GetSiteAdmin(),
		}
		result = append(result, userMap)
	}

	return map[string]interface{}{
		"users": result,
	}, nil
}

// getWebhooks gets webhooks for a repository
func (a *Adapter) getWebhooks(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("missing owner parameter")
	}

	repo, ok := params["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo parameter")
	}

	// Set up list options
	listOptions := &github.ListOptions{
		PerPage: 100,
	}

	// Get webhooks
	hooks, _, err := a.client.Repositories.ListHooks(ctx, owner, repo, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get webhooks: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(hooks))
	for _, hook := range hooks {
		hookMap := map[string]interface{}{
			"id":        hook.GetID(),
			"url":       hook.GetURL(),
			"events":    hook.Events,
			"active":    hook.GetActive(),
			"created_at": hook.GetCreatedAt(),
			"updated_at": hook.GetUpdatedAt(),
		}
		
		// Add config info (without exposing secrets)
		configMap := make(map[string]interface{})
		for key, value := range hook.Config {
			if key == "secret" && value != nil && value != "" {
				configMap[key] = "********"
			} else {
				configMap[key] = value
			}
		}
		hookMap["config"] = configMap

		result = append(result, hookMap)
	}

	return map[string]interface{}{
		"webhooks": result,
	}, nil
}

// Subscribe registers a callback for a specific event type
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
	a.subscribers[eventType] = append(a.subscribers[eventType], callback)
	return nil
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
	case "workflow_job":
		event, err = a.parseWorkflowJobEvent(payload)
	case "check_run":
		event, err = a.parseCheckRunEvent(payload)
	case "check_suite":
		event, err = a.parseCheckSuiteEvent(payload)
	case "create":
		event, err = a.parseCreateEvent(payload)
	case "delete":
		event, err = a.parseDeleteEvent(payload)
	case "repository":
		event, err = a.parseRepositoryEvent(payload)
	case "team":
		event, err = a.parseTeamEvent(payload)
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

// parseWorkflowJobEvent parses a workflow job event
func (a *Adapter) parseWorkflowJobEvent(payload []byte) (interface{}, error) {
	var event github.WorkflowJobEvent
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

// parseRepositoryEvent parses a repository event
func (a *Adapter) parseRepositoryEvent(payload []byte) (interface{}, error) {
	var event github.RepositoryEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// parseTeamEvent parses a team event
func (a *Adapter) parseTeamEvent(payload []byte) (interface{}, error) {
	var event github.TeamEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// IsSafeOperation checks if a GitHub operation is safe to perform by delegating to the safety package
func (a *Adapter) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	return IsSafeOperation(operation)
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	// Check if rate limits are critically low
	if a.rateLimits != nil && a.rateLimits.Core != nil {
		core := a.rateLimits.Core
		if core.Remaining < int64(a.config.RateLimitThreshold) {
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
			"requests_total":   a.stats.RequestsTotal,
			"requests_success": a.stats.RequestsSuccess,
			"requests_failed":  a.stats.RequestsFailed,
			"requests_retried": a.stats.RequestsRetried,
			"rate_limit_hits":  a.stats.RateLimitHits,
		},
	}

	// Add rate limit info if available
	if a.rateLimits != nil && a.rateLimits.Core != nil {
		resetTime := time.Until(a.rateLimits.Core.Reset.Time)
		details["rate_limit"] = map[string]interface{}{
			"limit":      a.rateLimits.Core.Limit,
			"remaining":  a.rateLimits.Core.Remaining,
			"used":       a.rateLimits.Core.Used,
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
