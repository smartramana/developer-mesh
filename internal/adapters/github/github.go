package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

// Config holds configuration for the GitHub adapter
type Config struct {
	APIToken        string        `mapstructure:"api_token"`
	WebhookSecret   string        `mapstructure:"webhook_secret"`
	EnterpriseURL   string        `mapstructure:"enterprise_url"`
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	RetryMax        int           `mapstructure:"retry_max"`
	RetryDelay      time.Duration `mapstructure:"retry_delay"`
}

// Adapter implements the adapter interface for GitHub
type Adapter struct {
	adapters.BaseAdapter
	config       Config
	client       *github.Client
	subscribers  map[string][]func(interface{})
	healthStatus string
}

// NewAdapter creates a new GitHub adapter
func NewAdapter(config Config) (*Adapter, error) {
	// Set default values if not provided
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.RetryMax == 0 {
		config.RetryMax = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}

	adapter := &Adapter{
		BaseAdapter: adapters.BaseAdapter{
			RetryMax:   config.RetryMax,
			RetryDelay: config.RetryDelay,
		},
		config: config,
		subscribers: make(map[string][]func(interface{})),
		healthStatus: "initializing",
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
	tc := oauth2.NewClient(ctx, ts)
	tc.Timeout = a.config.RequestTimeout

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

	// Test the connection
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		return err
	}

	a.healthStatus = "healthy"
	return nil
}

// testConnection verifies connectivity to GitHub
func (a *Adapter) testConnection(ctx context.Context) error {
	// Get rate limit to verify connectivity
	_, _, err := a.client.RateLimits(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to GitHub API: %w", err)
	}
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

	// Handle different operations
	switch operation {
	case "get_repositories":
		return a.getRepositories(ctx, queryMap)
	case "get_pull_requests":
		return a.getPullRequests(ctx, queryMap)
	case "get_issues":
		return a.getIssues(ctx, queryMap)
	case "get_repository":
		return a.getRepository(ctx, queryMap)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Handle different actions
	switch action {
	case "create_issue":
		return a.createIssue(ctx, params)
	case "close_issue":
		return a.closeIssue(ctx, params)
	case "create_pull_request":
		return a.createPullRequest(ctx, params)
	case "merge_pull_request":
		return a.mergePullRequest(ctx, params)
	case "add_comment":
		return a.addComment(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
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

	pr, _, err := a.client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return map[string]interface{}{
		"pull_number": pr.GetNumber(),
		"html_url":    pr.GetHTMLURL(),
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

	// Create merge request
	options := &github.PullRequestOptions{
		CommitTitle: commitMessage,
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
	}, nil
}

// getRepositories gets repositories for a user or organization
func (a *Adapter) getRepositories(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	var username string
	if usernameParam, ok := params["username"].(string); ok {
		username = usernameParam
	}

	// Set up list options
	listOptions := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	if typeParam, ok := params["type"].(string); ok {
		listOptions.Type = typeParam
	}

	// Get repositories
	var repositories []*github.Repository
	var err error

	if username == "" {
		// Get authenticated user's repositories
		repositories, _, err = a.client.Repositories.List(ctx, "", listOptions)
	} else {
		// Get repositories for the specified user/org
		repositories, _, err = a.client.Repositories.List(ctx, username, listOptions)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get repositories: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(repositories))
	for _, repo := range repositories {
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
		}
		result = append(result, repoMap)
	}

	return map[string]interface{}{
		"repositories": result,
	}, nil
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
		}
		result = append(result, issueMap)
	}

	return map[string]interface{}{
		"issues": result,
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

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	return a.healthStatus
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}
