package adapters

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/internal/resilience"
	"github.com/google/go-github/v53/github"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"
)

// GitHubConfig holds configuration for the GitHub adapter
type GitHubConfig struct {
	// Authentication
	Token string `mapstructure:"token"`
	
	// API settings
	BaseURL      string        `mapstructure:"base_url"`
	UploadURL    string        `mapstructure:"upload_url"`
	Timeout      time.Duration `mapstructure:"timeout"`
	
	// Rate limiting
	EnableRateLimiting bool    `mapstructure:"enable_rate_limiting"`
	RequestsPerHour    float64 `mapstructure:"requests_per_hour"`
	MaxBurst           int     `mapstructure:"max_burst"`
	
	// Circuit breaker
	EnableCircuitBreaker bool          `mapstructure:"enable_circuit_breaker"`
	MaxRequests          uint32        `mapstructure:"max_requests"`
	Interval             time.Duration `mapstructure:"interval"`
	Timeout              time.Duration `mapstructure:"timeout"`
	FailureRatio         float64       `mapstructure:"failure_ratio"`
}

// GitHubAdapter is an adapter for GitHub
type GitHubAdapter struct {
	client           *github.Client
	config           GitHubConfig
	circuitBreaker   *resilience.CircuitBreakerManager
	rateLimiter      *resilience.RateLimiterManager
	metricsClient    *observability.MetricsClient
}

// NewGitHubAdapter creates a new GitHub adapter
func NewGitHubAdapter(config GitHubConfig) (*GitHubAdapter, error) {
	// Create OAuth2 client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Token},
	)
	tc := oauth2.NewClient(ctx, ts)
	
	// Create GitHub client
	client := github.NewClient(tc)
	
	// Set custom base URL if provided
	if config.BaseURL != "" {
		var err error
		client, err = github.NewEnterpriseClient(config.BaseURL, config.UploadURL, tc)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub enterprise client: %w", err)
		}
	}
	
	// Apply default timeout if not set
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	
	// Create circuit breaker
	cbConfigs := make(map[string]resilience.CircuitBreakerConfig)
	if config.EnableCircuitBreaker {
		cbConfigs[resilience.GitHubCircuitBreaker] = resilience.CircuitBreakerConfig{
			Name:         resilience.GitHubCircuitBreaker,
			MaxRequests:  config.MaxRequests,
			Interval:     config.Interval,
			Timeout:      config.Timeout,
			FailureRatio: config.FailureRatio,
		}
	}
	circuitBreaker := resilience.NewCircuitBreakerManager(cbConfigs)
	
	// Create rate limiter
	rlConfigs := make(map[string]resilience.RateLimiterConfig)
	if config.EnableRateLimiting {
		rlConfigs[resilience.GitHubRateLimiter] = resilience.RateLimiterConfig{
			Name:      resilience.GitHubRateLimiter,
			Rate:      config.RequestsPerHour / 3600, // Convert to requests per second
			Burst:     config.MaxBurst,
			WaitLimit: 5 * time.Second,
		}
	}
	rateLimiter := resilience.NewRateLimiterManager(rlConfigs)
	
	return &GitHubAdapter{
		client:         client,
		config:         config,
		circuitBreaker: circuitBreaker,
		rateLimiter:    rateLimiter,
		metricsClient:  observability.NewMetricsClient(),
	}, nil
}

// executeWithResilience executes a function with circuit breaker and rate limiter
func (a *GitHubAdapter) executeWithResilience(ctx context.Context, operation string, fn func() (interface{}, error)) (interface{}, error) {
	// Record metrics
	startTime := time.Now()
	var err error
	
	// Start tracing
	ctx, span := observability.TraceTool(ctx, "github", operation)
	defer span.End()
	
	// Add operation as attribute
	span.SetAttributes(attribute.String("github.operation", operation))
	
	// Define the execution function
	execute := func() (interface{}, error) {
		// Apply rate limiting if enabled
		if a.config.EnableRateLimiting {
			wrapper := func() (interface{}, error) {
				return fn()
			}
			
			return a.rateLimiter.Execute(ctx, resilience.GitHubRateLimiter, wrapper)
		}
		
		// Execute the function directly
		return fn()
	}
	
	// Apply circuit breaker if enabled
	var result interface{}
	if a.config.EnableCircuitBreaker {
		result, err = a.circuitBreaker.Execute(ctx, resilience.GitHubCircuitBreaker, execute)
	} else {
		result, err = execute()
	}
	
	// Record metrics
	duration := time.Since(startTime)
	a.metricsClient.RecordToolOperation("github", operation, duration.Seconds(), err)
	
	// Record error if any
	if err != nil {
		span.RecordError(err)
		log.Printf("GitHub operation %s failed: %v", operation, err)
	}
	
	return result, err
}

// CreateIssue creates a new issue in a repository
func (a *GitHubAdapter) CreateIssue(ctx context.Context, owner, repo, title, body string, labels []string) (*github.Issue, error) {
	var issue *github.Issue
	
	// Create issue request
	issueRequest := &github.IssueRequest{
		Title:  github.String(title),
		Body:   github.String(body),
		Labels: &labels,
	}
	
	// Execute with resilience
	result, err := a.executeWithResilience(ctx, "create_issue", func() (interface{}, error) {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
		defer cancel()
		
		// Create issue
		i, _, err := a.client.Issues.Create(timeoutCtx, owner, repo, issueRequest)
		return i, err
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}
	
	// Convert result to issue
	var ok bool
	if issue, ok = result.(*github.Issue); !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	
	return issue, nil
}

// CloseIssue closes an issue in a repository
func (a *GitHubAdapter) CloseIssue(ctx context.Context, owner, repo string, number int) (*github.Issue, error) {
	var issue *github.Issue
	
	// Create issue request
	issueRequest := &github.IssueRequest{
		State: github.String("closed"),
	}
	
	// Execute with resilience
	result, err := a.executeWithResilience(ctx, "close_issue", func() (interface{}, error) {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
		defer cancel()
		
		// Update issue
		i, _, err := a.client.Issues.Edit(timeoutCtx, owner, repo, number, issueRequest)
		return i, err
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to close issue: %w", err)
	}
	
	// Convert result to issue
	var ok bool
	if issue, ok = result.(*github.Issue); !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	
	return issue, nil
}

// GetIssue gets an issue from a repository
func (a *GitHubAdapter) GetIssue(ctx context.Context, owner, repo string, number int) (*github.Issue, error) {
	var issue *github.Issue
	
	// Execute with resilience
	result, err := a.executeWithResilience(ctx, "get_issue", func() (interface{}, error) {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
		defer cancel()
		
		// Get issue
		i, _, err := a.client.Issues.Get(timeoutCtx, owner, repo, number)
		return i, err
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}
	
	// Convert result to issue
	var ok bool
	if issue, ok = result.(*github.Issue); !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	
	return issue, nil
}

// ListIssues lists issues in a repository
func (a *GitHubAdapter) ListIssues(ctx context.Context, owner, repo string, options *github.IssueListByRepoOptions) ([]*github.Issue, error) {
	var issues []*github.Issue
	
	// Execute with resilience
	result, err := a.executeWithResilience(ctx, "list_issues", func() (interface{}, error) {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
		defer cancel()
		
		// List issues
		i, _, err := a.client.Issues.ListByRepo(timeoutCtx, owner, repo, options)
		return i, err
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}
	
	// Convert result to issues
	var ok bool
	if issues, ok = result.([]*github.Issue); !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	
	return issues, nil
}

// CreatePullRequest creates a new pull request in a repository
func (a *GitHubAdapter) CreatePullRequest(ctx context.Context, owner, repo, title, body, head, base string) (*github.PullRequest, error) {
	var pr *github.PullRequest
	
	// Create pull request
	newPR := &github.NewPullRequest{
		Title: github.String(title),
		Body:  github.String(body),
		Head:  github.String(head),
		Base:  github.String(base),
	}
	
	// Execute with resilience
	result, err := a.executeWithResilience(ctx, "create_pull_request", func() (interface{}, error) {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
		defer cancel()
		
		// Create pull request
		p, _, err := a.client.PullRequests.Create(timeoutCtx, owner, repo, newPR)
		return p, err
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}
	
	// Convert result to pull request
	var ok bool
	if pr, ok = result.(*github.PullRequest); !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	
	return pr, nil
}

// MergePullRequest merges a pull request
func (a *GitHubAdapter) MergePullRequest(ctx context.Context, owner, repo string, number int, commitMessage string) (*github.PullRequestMergeResult, error) {
	var result *github.PullRequestMergeResult
	
	// Execute with resilience
	res, err := a.executeWithResilience(ctx, "merge_pull_request", func() (interface{}, error) {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
		defer cancel()
		
		// Merge pull request
		r, _, err := a.client.PullRequests.Merge(timeoutCtx, owner, repo, number, commitMessage, &github.PullRequestOptions{})
		return r, err
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to merge pull request: %w", err)
	}
	
	// Convert result to merge result
	var ok bool
	if result, ok = res.(*github.PullRequestMergeResult); !ok {
		return nil, fmt.Errorf("unexpected result type: %T", res)
	}
	
	return result, nil
}

// AddComment adds a comment to an issue or pull request
func (a *GitHubAdapter) AddComment(ctx context.Context, owner, repo string, number int, body string) (*github.IssueComment, error) {
	var comment *github.IssueComment
	
	// Create comment
	newComment := &github.IssueComment{
		Body: github.String(body),
	}
	
	// Execute with resilience
	result, err := a.executeWithResilience(ctx, "add_comment", func() (interface{}, error) {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
		defer cancel()
		
		// Create comment
		c, _, err := a.client.Issues.CreateComment(timeoutCtx, owner, repo, number, newComment)
		return c, err
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to add comment: %w", err)
	}
	
	// Convert result to comment
	var ok bool
	if comment, ok = result.(*github.IssueComment); !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	
	return comment, nil
}
