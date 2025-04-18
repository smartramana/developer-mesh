package github

import (
	"context"
	"fmt"
	"net/http"
	"time"
	
	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
	
	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	adapterErrors "github.com/S-Corkum/mcp-server/internal/adapters/errors"
	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// Ensure GitHubAdapter implements core.Adapter interface
var _ core.Adapter = (*GitHubAdapter)(nil)

// GitHubAdapter is an adapter for GitHub
type GitHubAdapter struct {
	core.BaseAdapter
	client           *github.Client
	config           Config
	eventBus         *events.EventBus
	metricsClient    *observability.MetricsClient
	logger           *observability.Logger
	
	// Feature flags based on configuration
	featuresEnabled map[string]bool
	
	// Optional default repository settings
	defaultOwner    string
	defaultRepo     string
}

// NewAdapter creates a new GitHub adapter
func NewAdapter(config Config, eventBus *events.EventBus, metricsClient *observability.MetricsClient, logger *observability.Logger) (*GitHubAdapter, error) {
	// Validate configuration
	valid, errors := ValidateConfig(config)
	if !valid {
		return nil, fmt.Errorf("invalid GitHub adapter configuration: %v", errors)
	}
	
	// Create the base adapter
	baseAdapter := core.BaseAdapter{
		AdapterType:    "github",
		AdapterVersion: "1.0.0",
		Features:       make(map[string]bool),
		Timeout:        config.Timeout,
		SafeMode:       true,
	}
	
	// Enable features based on configuration
	featuresEnabled := make(map[string]bool)
	for _, feature := range config.EnabledFeatures {
		featuresEnabled[feature] = true
		baseAdapter.Features[feature] = true
	}
	
	// Create the adapter
	adapter := &GitHubAdapter{
		BaseAdapter:     baseAdapter,
		config:          config,
		eventBus:        eventBus,
		metricsClient:   metricsClient,
		logger:          logger,
		featuresEnabled: featuresEnabled,
		defaultOwner:    config.DefaultOwner,
		defaultRepo:     config.DefaultRepo,
	}
	
	return adapter, nil
}

// Initialize sets up the adapter with configuration
func (a *GitHubAdapter) Initialize(ctx context.Context, rawConfig interface{}) error {
	// Convert raw config to GitHub config if necessary
	var config Config
	var ok bool
	
	if config, ok = rawConfig.(Config); !ok {
		return fmt.Errorf("invalid configuration type for GitHub adapter")
	}
	
	// Create OAuth2 client for authentication
	var tc *http.Client
	
	if config.Token != "" {
		// Use token authentication
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: config.Token},
		)
		tc = oauth2.NewClient(ctx, ts)
	} else if config.AppID != "" && config.InstallID != "" && config.PrivateKey != "" {
		// Use GitHub App authentication
		// This would require a GitHub App authentication implementation
		return fmt.Errorf("GitHub App authentication not implemented yet")
	} else {
		return fmt.Errorf("no valid authentication method provided")
	}
	
	// Create GitHub client
	client := github.NewClient(tc)
	
	// Set custom base URL if provided
	if config.BaseURL != "" {
		var err error
		client, err = github.NewEnterpriseClient(config.BaseURL, config.UploadURL, tc)
		if err != nil {
			return fmt.Errorf("failed to create GitHub enterprise client: %w", err)
		}
	}
	
	// Update adapter with client and config
	a.client = client
	a.config = config
	
	// Initialize resilience patterns
	if config.Resilience.CircuitBreaker.Enabled {
		circuitBreakerConfig := config.Resilience.CircuitBreaker.GetCircuitBreakerConfig("github")
		a.CircuitBreaker = resilience.NewCircuitBreaker(circuitBreakerConfig)
	}
	
	if config.Resilience.RateLimiter.Enabled {
		rateLimiterConfig := config.Resilience.RateLimiter.GetRateLimiterConfig("github")
		a.RateLimiter = resilience.NewRateLimiter(rateLimiterConfig)
	}
	
	// Emit initialization event
	if a.eventBus != nil {
		event := events.NewAdapterEvent("github", events.EventTypeAdapterInitialized, nil)
		a.eventBus.Emit(ctx, event)
	}
	
	return nil
}

// GetData retrieves data from GitHub
func (a *GitHubAdapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	// Convert query to specific data query type
	queryParams, ok := query.(GitHubDataQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type for GitHub adapter")
	}
	
	// Execute query based on the resource type
	switch queryParams.ResourceType {
	case "issues":
		return a.getIssues(ctx, queryParams)
	case "repositories":
		return a.getRepositories(ctx, queryParams)
	case "pull_requests":
		return a.getPullRequests(ctx, queryParams)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", queryParams.ResourceType)
	}
}

// ExecuteAction executes an action on GitHub
func (a *GitHubAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Check if the operation is safe
	safe, err := a.IsSafeOperation(action, params)
	if err != nil {
		return nil, err
	}
	if !safe {
		return nil, fmt.Errorf("operation '%s' is not safe with the provided parameters", action)
	}
	
	// Start tracing and metrics
	startTime := time.Now()
	ctx, span := observability.StartSpan(ctx, fmt.Sprintf("github.%s", action))
	defer span.End()
	
	// Add operation attributes to span
	span.SetAttributes(map[string]interface{}{
		"adapter.name": "github",
		"adapter.operation": action,
		"adapter.contextID": contextID,
	})
	
	// Execute with resilience
	var result interface{}
	var execErr error
	
	// Execute with circuit breaker if configured
	if a.CircuitBreaker != nil {
		result, execErr = a.executeWithCircuitBreaker(ctx, action, params)
	} else {
		result, execErr = a.executeOperation(ctx, action, params)
	}
	
	// Record metrics
	duration := time.Since(startTime)
	if a.metricsClient != nil {
		a.metricsClient.RecordOperation("github", action, duration.Seconds(), execErr != nil)
	}
	
	// Record error if any
	if execErr != nil {
		span.RecordError(execErr)
		a.logger.Error("GitHub operation failed", map[string]interface{}{
			"operation": action,
			"error": execErr.Error(),
			"duration": duration.Seconds(),
			"contextID": contextID,
		})
		
		// Emit event
		if a.eventBus != nil {
			event := events.NewAdapterEvent("github", events.EventTypeOperationFailure, params)
			event.WithMetadata("operation", action)
			event.WithMetadata("error", execErr.Error())
			event.WithMetadata("contextId", contextID)
			event.WithMetadata("duration", duration.Seconds())
			a.eventBus.Emit(ctx, event)
		}
		
		return nil, execErr
	}
	
	// Emit success event
	if a.eventBus != nil {
		event := events.NewAdapterEvent("github", events.EventTypeOperationSuccess, result)
		event.WithMetadata("operation", action)
		event.WithMetadata("contextId", contextID)
		event.WithMetadata("duration", duration.Seconds())
		a.eventBus.Emit(ctx, event)
	}
	
	return result, nil
}

// executeWithCircuitBreaker executes an operation with circuit breaker protection
func (a *GitHubAdapter) executeWithCircuitBreaker(ctx context.Context, action string, params map[string]interface{}) (interface{}, error) {
	var result interface{}
	
	operation := func() (interface{}, error) {
		// Apply rate limiting if configured
		if a.RateLimiter != nil {
			if err := a.RateLimiter.Wait(ctx); err != nil {
				return nil, adapterErrors.NewRateLimitExceededError("github", action, err, map[string]interface{}{
					"params": params,
				})
			}
		}
		
		// Execute the operation
		return a.executeOperation(ctx, action, params)
	}
	
	// Execute with circuit breaker
	cbResult, err := a.CircuitBreaker.Execute(operation)
	if err != nil {
		// Check if circuit is open
		if a.CircuitBreaker.IsOpen() {
			return nil, adapterErrors.NewServiceUnavailableError("github", action, err, map[string]interface{}{
				"params": params,
				"circuit_state": "open",
			})
		}
		return nil, err
	}
	
	result = cbResult
	return result, nil
}

// executeOperation executes an operation based on the action type
func (a *GitHubAdapter) executeOperation(ctx context.Context, action string, params map[string]interface{}) (interface{}, error) {
	switch action {
	case "create_issue":
		return a.createIssue(ctx, params)
	case "close_issue":
		return a.closeIssue(ctx, params)
	case "add_comment":
		return a.addComment(ctx, params)
	case "create_pull_request":
		return a.createPullRequest(ctx, params)
	case "merge_pull_request":
		return a.mergePullRequest(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// HandleWebhook processes webhook events from GitHub
func (a *GitHubAdapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Create a webhook event from the payload
	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		return fmt.Errorf("failed to parse webhook: %w", err)
	}
	
	// Extract basic event info for logging
	eventInfo := map[string]interface{}{
		"eventType": eventType,
	}
	
	// Process based on event type
	switch event := event.(type) {
	case *github.IssuesEvent:
		eventInfo["action"] = event.GetAction()
		eventInfo["repo"] = event.GetRepo().GetFullName()
		eventInfo["issueNumber"] = event.GetIssue().GetNumber()
		
		// Process the issue event
		if err := a.handleIssueEvent(ctx, event); err != nil {
			return err
		}
		
	case *github.PullRequestEvent:
		eventInfo["action"] = event.GetAction()
		eventInfo["repo"] = event.GetRepo().GetFullName()
		eventInfo["prNumber"] = event.GetPullRequest().GetNumber()
		
		// Process the pull request event
		if err := a.handlePullRequestEvent(ctx, event); err != nil {
			return err
		}
		
	case *github.IssueCommentEvent:
		eventInfo["action"] = event.GetAction()
		eventInfo["repo"] = event.GetRepo().GetFullName()
		eventInfo["issueNumber"] = event.GetIssue().GetNumber()
		
		// Process the issue comment event
		if err := a.handleIssueCommentEvent(ctx, event); err != nil {
			return err
		}
		
	default:
		a.logger.Info("Unhandled GitHub webhook event type", map[string]interface{}{
			"eventType": eventType,
		})
	}
	
	// Log webhook event
	a.logger.Info("Received GitHub webhook", eventInfo)
	
	// Emit webhook event
	if a.eventBus != nil {
		webhookEvent := events.NewAdapterEvent("github", events.EventTypeWebhookReceived, event)
		webhookEvent.WithMetadata("eventType", eventType)
		a.eventBus.Emit(ctx, webhookEvent)
	}
	
	return nil
}

// Subscribe registers a callback for a specific event type
func (a *GitHubAdapter) Subscribe(eventType string, callback func(interface{})) error {
	// Not implemented yet
	return fmt.Errorf("subscription not implemented for GitHub adapter")
}

// IsSafeOperation determines if an operation is safe to perform
func (a *GitHubAdapter) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	// Define unsafe operations
	unsafeOperations := map[string]bool{
		"delete_repository": true,
		"delete_branch":     true,
		"delete_issue":      true,
		"delete_pull_request": true,
	}
	
	// Check if operation is in the unsafe list
	if unsafeOperations[operation] {
		return false, nil
	}
	
	// Additional safety checks based on operation
	switch operation {
	case "merge_pull_request":
		// Additional checks for merge_pull_request
		if params["force"] == true {
			return false, nil
		}
	}
	
	return true, nil
}

// Close gracefully shuts down the adapter
func (a *GitHubAdapter) Close() error {
	// Clean up any resources
	
	// Emit close event
	if a.eventBus != nil {
		event := events.NewAdapterEvent("github", events.EventTypeAdapterClosed, nil)
		a.eventBus.Emit(context.Background(), event)
	}
	
	return nil
}

// Operations

// GitHubDataQuery represents a query for GitHub data
type GitHubDataQuery struct {
	ResourceType string                 `json:"resource_type"`
	Owner        string                 `json:"owner"`
	Repo         string                 `json:"repo"`
	Filters      map[string]interface{} `json:"filters"`
}

// getIssues retrieves issues from GitHub
func (a *GitHubAdapter) getIssues(ctx context.Context, query GitHubDataQuery) ([]*github.Issue, error) {
	// Get owner and repo from query or defaults
	owner := query.Owner
	if owner == "" {
		owner = a.defaultOwner
	}
	
	repo := query.Repo
	if repo == "" {
		repo = a.defaultRepo
	}
	
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	
	// Create options from filters
	options := &github.IssueListByRepoOptions{}
	
	// Apply filters
	if query.Filters != nil {
		if state, ok := query.Filters["state"].(string); ok {
			options.State = state
		}
		
		if sort, ok := query.Filters["sort"].(string); ok {
			options.Sort = sort
		}
		
		if direction, ok := query.Filters["direction"].(string); ok {
			options.Direction = direction
		}
		
		if since, ok := query.Filters["since"].(time.Time); ok {
			options.Since = since
		}
		
		if labels, ok := query.Filters["labels"].([]string); ok {
			options.Labels = labels
		}
	}
	
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	
	// List issues
	issues, _, err := a.client.Issues.ListByRepo(timeoutCtx, owner, repo, options)
	if err != nil {
		return nil, a.mapError("list_issues", err, map[string]interface{}{
			"owner": owner,
			"repo": repo,
			"options": options,
		})
	}
	
	return issues, nil
}

// getRepositories retrieves repositories from GitHub
func (a *GitHubAdapter) getRepositories(ctx context.Context, query GitHubDataQuery) ([]*github.Repository, error) {
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	
	// Create options from filters
	options := &github.RepositoryListOptions{}
	
	// Apply filters
	if query.Filters != nil {
		if sort, ok := query.Filters["sort"].(string); ok {
			options.Sort = sort
		}
		
		if direction, ok := query.Filters["direction"].(string); ok {
			options.Direction = direction
		}
		
		if perPage, ok := query.Filters["per_page"].(int); ok {
			options.PerPage = perPage
		}
	}
	
	// List repositories
	var repos []*github.Repository
	var err error
	
	if query.Owner != "" {
		// List user repositories
		repos, _, err = a.client.Repositories.List(timeoutCtx, query.Owner, options)
	} else {
		// List authenticated user repositories
		repos, _, err = a.client.Repositories.List(timeoutCtx, "", options)
	}
	
	if err != nil {
		return nil, a.mapError("list_repositories", err, map[string]interface{}{
			"owner": query.Owner,
			"options": options,
		})
	}
	
	return repos, nil
}

// getPullRequests retrieves pull requests from GitHub
func (a *GitHubAdapter) getPullRequests(ctx context.Context, query GitHubDataQuery) ([]*github.PullRequest, error) {
	// Get owner and repo from query or defaults
	owner := query.Owner
	if owner == "" {
		owner = a.defaultOwner
	}
	
	repo := query.Repo
	if repo == "" {
		repo = a.defaultRepo
	}
	
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	
	// Create options from filters
	options := &github.PullRequestListOptions{}
	
	// Apply filters
	if query.Filters != nil {
		if state, ok := query.Filters["state"].(string); ok {
			options.State = state
		}
		
		if sort, ok := query.Filters["sort"].(string); ok {
			options.Sort = sort
		}
		
		if direction, ok := query.Filters["direction"].(string); ok {
			options.Direction = direction
		}
		
		if base, ok := query.Filters["base"].(string); ok {
			options.Base = base
		}
		
		if head, ok := query.Filters["head"].(string); ok {
			options.Head = head
		}
	}
	
	// List pull requests
	prs, _, err := a.client.PullRequests.List(timeoutCtx, owner, repo, options)
	if err != nil {
		return nil, a.mapError("list_pull_requests", err, map[string]interface{}{
			"owner": owner,
			"repo": repo,
			"options": options,
		})
	}
	
	return prs, nil
}

// createIssue creates a new issue on GitHub
func (a *GitHubAdapter) createIssue(ctx context.Context, params map[string]interface{}) (*github.Issue, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		owner = a.defaultOwner
	}
	
	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		repo = a.defaultRepo
	}
	
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	
	title, ok := params["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}
	
	body, _ := params["body"].(string)
	
	// Extract labels
	var labels []string
	if labelsParam, ok := params["labels"]; ok {
		if labelsArray, ok := labelsParam.([]string); ok {
			labels = labelsArray
		} else if labelsInterface, ok := labelsParam.([]interface{}); ok {
			labels = make([]string, len(labelsInterface))
			for i, label := range labelsInterface {
				if labelStr, ok := label.(string); ok {
					labels[i] = labelStr
				}
			}
		}
	}
	
	// Extract assignees
	var assignees []string
	if assigneesParam, ok := params["assignees"]; ok {
		if assigneesArray, ok := assigneesParam.([]string); ok {
			assignees = assigneesArray
		} else if assigneesInterface, ok := assigneesParam.([]interface{}); ok {
			assignees = make([]string, len(assigneesInterface))
			for i, assignee := range assigneesInterface {
				if assigneeStr, ok := assignee.(string); ok {
					assignees[i] = assigneeStr
				}
			}
		}
	}
	
	// Create issue request
	issueRequest := &github.IssueRequest{
		Title:     github.String(title),
		Body:      github.String(body),
		Labels:    &labels,
		Assignees: &assignees,
	}
	
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	
	// Create issue
	issue, _, err := a.client.Issues.Create(timeoutCtx, owner, repo, issueRequest)
	if err != nil {
		return nil, a.mapError("create_issue", err, map[string]interface{}{
			"owner": owner,
			"repo": repo,
			"title": title,
		})
	}
	
	return issue, nil
}

// closeIssue closes an issue on GitHub
func (a *GitHubAdapter) closeIssue(ctx context.Context, params map[string]interface{}) (*github.Issue, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		owner = a.defaultOwner
	}
	
	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		repo = a.defaultRepo
	}
	
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	
	issueNumberFloat, ok := params["issue_number"].(float64)
	if !ok {
		issueNumberInt, ok := params["issue_number"].(int)
		if !ok {
			return nil, fmt.Errorf("issue_number is required and must be a number")
		}
		issueNumberFloat = float64(issueNumberInt)
	}
	
	issueNumber := int(issueNumberFloat)
	
	// Create issue request to close the issue
	issueRequest := &github.IssueRequest{
		State: github.String("closed"),
	}
	
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	
	// Update issue
	issue, _, err := a.client.Issues.Edit(timeoutCtx, owner, repo, issueNumber, issueRequest)
	if err != nil {
		return nil, a.mapError("close_issue", err, map[string]interface{}{
			"owner": owner,
			"repo": repo,
			"issue_number": issueNumber,
		})
	}
	
	return issue, nil
}

// addComment adds a comment to an issue or pull request
func (a *GitHubAdapter) addComment(ctx context.Context, params map[string]interface{}) (*github.IssueComment, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		owner = a.defaultOwner
	}
	
	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		repo = a.defaultRepo
	}
	
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	
	issueNumberFloat, ok := params["issue_number"].(float64)
	if !ok {
		issueNumberInt, ok := params["issue_number"].(int)
		if !ok {
			return nil, fmt.Errorf("issue_number is required and must be a number")
		}
		issueNumberFloat = float64(issueNumberInt)
	}
	
	issueNumber := int(issueNumberFloat)
	
	body, ok := params["body"].(string)
	if !ok || body == "" {
		return nil, fmt.Errorf("body is required")
	}
	
	// Create comment
	comment := &github.IssueComment{
		Body: github.String(body),
	}
	
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	
	// Add comment
	newComment, _, err := a.client.Issues.CreateComment(timeoutCtx, owner, repo, issueNumber, comment)
	if err != nil {
		return nil, a.mapError("add_comment", err, map[string]interface{}{
			"owner": owner,
			"repo": repo,
			"issue_number": issueNumber,
		})
	}
	
	return newComment, nil
}

// createPullRequest creates a new pull request
func (a *GitHubAdapter) createPullRequest(ctx context.Context, params map[string]interface{}) (*github.PullRequest, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		owner = a.defaultOwner
	}
	
	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		repo = a.defaultRepo
	}
	
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
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
	
	body, _ := params["body"].(string)
	
	// Create pull request
	newPR := &github.NewPullRequest{
		Title: github.String(title),
		Head:  github.String(head),
		Base:  github.String(base),
		Body:  github.String(body),
	}
	
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	
	// Create pull request
	pr, _, err := a.client.PullRequests.Create(timeoutCtx, owner, repo, newPR)
	if err != nil {
		return nil, a.mapError("create_pull_request", err, map[string]interface{}{
			"owner": owner,
			"repo": repo,
			"head": head,
			"base": base,
		})
	}
	
	return pr, nil
}

// mergePullRequest merges a pull request
func (a *GitHubAdapter) mergePullRequest(ctx context.Context, params map[string]interface{}) (*github.PullRequestMergeResult, error) {
	// Extract parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		owner = a.defaultOwner
	}
	
	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		repo = a.defaultRepo
	}
	
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	
	prNumberFloat, ok := params["pull_number"].(float64)
	if !ok {
		prNumberInt, ok := params["pull_number"].(int)
		if !ok {
			return nil, fmt.Errorf("pull_number is required and must be a number")
		}
		prNumberFloat = float64(prNumberInt)
	}
	
	prNumber := int(prNumberFloat)
	
	commitMessage, _ := params["commit_message"].(string)
	
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	
	// Merge pull request
	result, _, err := a.client.PullRequests.Merge(timeoutCtx, owner, repo, prNumber, commitMessage, &github.PullRequestOptions{})
	if err != nil {
		return nil, a.mapError("merge_pull_request", err, map[string]interface{}{
			"owner": owner,
			"repo": repo,
			"pull_number": prNumber,
		})
	}
	
	return result, nil
}

// Webhook handlers

// handleIssueEvent handles GitHub issue events
func (a *GitHubAdapter) handleIssueEvent(ctx context.Context, event *github.IssuesEvent) error {
	// Process based on action
	switch event.GetAction() {
	case "opened", "edited", "closed", "reopened", "assigned", "unassigned", "labeled", "unlabeled":
		// Process event
		a.logger.Info("Processed GitHub issue event", map[string]interface{}{
			"action": event.GetAction(),
			"repo": event.GetRepo().GetFullName(),
			"issueNumber": event.GetIssue().GetNumber(),
		})
	}
	
	return nil
}

// handlePullRequestEvent handles GitHub pull request events
func (a *GitHubAdapter) handlePullRequestEvent(ctx context.Context, event *github.PullRequestEvent) error {
	// Process based on action
	switch event.GetAction() {
	case "opened", "edited", "closed", "reopened", "assigned", "unassigned", "labeled", "unlabeled":
		// Process event
		a.logger.Info("Processed GitHub pull request event", map[string]interface{}{
			"action": event.GetAction(),
			"repo": event.GetRepo().GetFullName(),
			"prNumber": event.GetPullRequest().GetNumber(),
		})
	}
	
	return nil
}

// handleIssueCommentEvent handles GitHub issue comment events
func (a *GitHubAdapter) handleIssueCommentEvent(ctx context.Context, event *github.IssueCommentEvent) error {
	// Process based on action
	switch event.GetAction() {
	case "created", "edited", "deleted":
		// Process event
		a.logger.Info("Processed GitHub issue comment event", map[string]interface{}{
			"action": event.GetAction(),
			"repo": event.GetRepo().GetFullName(),
			"issueNumber": event.GetIssue().GetNumber(),
		})
	}
	
	return nil
}

// Error mapping

// mapError maps GitHub errors to adapter errors
func (a *GitHubAdapter) mapError(operation string, err error, context map[string]interface{}) error {
	// Map GitHub errors to adapter errors
	if err == nil {
		return nil
	}
	
	// Check for specific error types
	if rateLimitErr, ok := err.(*github.RateLimitError); ok {
		return adapterErrors.NewRateLimitExceededError("github", operation, rateLimitErr, context)
	}
	
	if abuseRateLimitErr, ok := err.(*github.AbuseRateLimitError); ok {
		return adapterErrors.NewRateLimitExceededError("github", operation, abuseRateLimitErr, context)
	}
	
	if httpErr, ok := err.(*github.ErrorResponse); ok {
		switch httpErr.Response.StatusCode {
		case 401:
			return adapterErrors.NewUnauthorizedError("github", operation, httpErr, context)
		case 403:
			return adapterErrors.NewForbiddenError("github", operation, httpErr, context)
		case 404:
			return adapterErrors.NewResourceNotFoundError("github", operation, httpErr, context)
		case 422:
			return adapterErrors.NewInvalidRequestError("github", operation, httpErr, context)
		case 429:
			return adapterErrors.NewTooManyRequestsError("github", operation, httpErr, context)
		case 500, 502, 503, 504:
			return adapterErrors.NewServiceUnavailableError("github", operation, httpErr, context)
		default:
			return adapterErrors.NewUnknownError("github", operation, httpErr, context)
		}
	}
	
	// Default error handling
	return adapterErrors.NewUnknownError("github", operation, err, context)
}
