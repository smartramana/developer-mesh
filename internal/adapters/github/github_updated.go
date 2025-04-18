package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	MockResponses   bool          `mapstructure:"mock_responses"`
	MockURL         string        `mapstructure:"mock_url"`
}

// Adapter implements the adapter interface for GitHub
type Adapter struct {
	adapters.BaseAdapter
	config       Config
	client       *github.Client
	httpClient   *http.Client
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
	a.httpClient = &http.Client{Timeout: a.config.RequestTimeout}
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

	// Add API version header to all requests
	a.client.WithAuthToken(a.config.APIToken)
	
	// Test the connection
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		return err
	}

	a.healthStatus = "healthy"
	return nil
}

// WithAuthToken sets the authorization token for all requests
func (c *github.Client) WithAuthToken(token string) {
	c.WithAuthTokenAndApiVersion(token, "2022-11-28")
}

// WithAuthTokenAndApiVersion sets the authorization token and API version for all requests
func (c *github.Client) WithAuthTokenAndApiVersion(token, apiVersion string) {
	c.BaseURL = c.BaseURL // This is just to access the Client internals
	
	// Add the transport to automatically include headers
	if c.Client.Transport != nil {
		c.Client.Transport = &headerTransport{
			base:       c.Client.Transport,
			token:      token,
			apiVersion: apiVersion,
		}
	} else {
		c.Client.Transport = &headerTransport{
			base:       http.DefaultTransport,
			token:      token,
			apiVersion: apiVersion,
		}
	}
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
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		
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
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		
		resp, err = httpClient.Do(req)
		if err != nil {
			a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to mock GitHub API: %v", err)
			// Don't return error, just make the adapter usable in degraded mode
			log.Printf("Warning: Failed to connect to mock GitHub API: %v", err)
			return nil
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			a.healthStatus = fmt.Sprintf("unhealthy: mock GitHub API returned status code: %d", resp.StatusCode)
			// Don't return error, just make the adapter usable in degraded mode
			log.Printf("Warning: Mock GitHub API returned unexpected status code: %d", resp.StatusCode)
			return nil
		}
		
		// Successfully connected to mock server
		log.Println("Successfully connected to mock GitHub API")
		a.healthStatus = "healthy"
		return nil
	}
	
	// Use the actual GitHub API for verification if not in mock mode
	_, _, err := a.client.RateLimits(ctx)
	if err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		// Don't return error, just make the adapter usable in degraded mode
		log.Printf("Warning: Failed to connect to GitHub API: %v", err)
		return nil
	}
	
	a.healthStatus = "healthy"
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
	case "get_branches":
		return a.getBranches(ctx, queryMap)
	case "get_teams":
		return a.getTeams(ctx, queryMap)
	case "get_workflow_runs":
		return a.getWorkflowRuns(ctx, queryMap)
	case "get_commits":
		return a.getCommits(ctx, queryMap)
	case "search_code":
		return a.searchCode(ctx, queryMap)
	case "get_users":
		return a.getUsers(ctx, queryMap)
	case "get_webhooks":
		return a.getWebhooks(ctx, queryMap)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// First check if this operation is safe
	if safe, err := IsSafeOperation(action); !safe {
		return nil, fmt.Errorf("operation %s is not allowed: %w", action, err)
	}

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
	case "create_branch":
		return a.createBranch(ctx, params)
	case "create_webhook":
		return a.createWebhook(ctx, params)
	case "delete_webhook":
		return a.deleteWebhook(ctx, params)
	case "check_workflow_run":
		return a.checkWorkflowRun(ctx, params)
	case "trigger_workflow":
		return a.triggerWorkflow(ctx, params)
	case "list_team_members":
		return a.listTeamMembers(ctx, params)
	case "add_team_member":
		return a.addTeamMember(ctx, params)
	case "remove_team_member":
		return a.removeTeamMember(ctx, params)
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

	workflowID, ok := params["workflow_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing workflow_id parameter")
	}

	ref, ok := params["ref"].(string)
	if !ok {
		// If no ref provided, use the default branch
		repoInfo, _, err := a.client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository: %w", err)
		}
		
		ref = repoInfo.GetDefaultBranch()
	}

	// Get inputs if provided
	inputs := make(map[string]interface{})
	if inputsParam, ok := params["inputs"].(map[string]interface{}); ok {
		inputs = inputsParam
	}

	// Create workflow dispatch event
	event := github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs,
	}

	// Trigger the workflow
	_, err := a.client.Actions.CreateWorkflowDispatchEventByID(ctx, owner, repo, int64(workflowID), event)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger workflow: %w", err)
	}

	return map[string]interface{}{
		"success":     true,
		"workflow_id": workflowID,
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
			"forks_count":   repo.GetForksCount(),
			"stars_count":   repo.GetStargazersCount(),
			"watchers_count": repo.GetWatchersCount(),
			"open_issues_count": repo.GetOpenIssuesCount(),
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
		"forks_count":    repository.GetForksCount(),
		"stars_count":    repository.GetStargazersCount(),
		"watchers_count": repository.GetWatchersCount(),
		"open_issues_count": repository.GetOpenIssuesCount(),
		"has_issues":     repository.GetHasIssues(),
		"has_wiki":       repository.GetHasWiki(),
		"has_projects":   repository.GetHasProjects(),
		"archived":       repository.GetArchived(),
		"disabled":       repository.GetDisabled(),
		"license":        repository.GetLicense().GetSPDXID(),
		"topics":         repository.Topics,
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
			"created_at":  team.GetCreatedAt(),
			"updated_at":  team.GetUpdatedAt(),
		}
		result = append(result, teamMap)
	}

	return map[string]interface{}{
		"teams": result,
	}, nil
}

// getWorkflowRuns gets workflow runs for a repository
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
			PerPage: 100,
		},
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
	
	if headSHAParam, ok := params["head_sha"].(string); ok {
		listOptions.HeadSHA = headSHAParam
	}

	// Get workflow runs based on workflow ID if provided
	var runs *github.WorkflowRuns
	var err error
	
	if workflowIDParam, ok := params["workflow_id"].(string); ok {
		runs, _, err = a.client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowIDParam, listOptions)
	} else {
		// Get all workflow runs
		runs, _, err = a.client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, listOptions)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow runs: %w", err)
	}

	// Convert to map for easier serialization
	result := make([]map[string]interface{}, 0, len(runs.WorkflowRuns))
	for _, run := range runs.WorkflowRuns {
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
			"display_title": run.GetDisplayTitle(),
		}
		result = append(result, runMap)
	}

	return map[string]interface{}{
		"workflow_runs": result,
		"total_count":   runs.GetTotalCount(),
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
					"url": commit.Commit.GetTree().GetURL(),
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

// searchCode searches code in a repository
func (a *Adapter) searchCode(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("missing query parameter")
	}

	// Set up search options
	searchOptions := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// Add filter parameters if provided
	if ownerParam, ok := params["owner"].(string); ok {
		// Add "repo:" to the query
		if repoParam, ok := params["repo"].(string); ok {
			query = fmt.Sprintf("%s repo:%s/%s", query, ownerParam, repoParam)
		} else {
			query = fmt.Sprintf("%s user:%s", query, ownerParam)
		}
	}

	// Add language filter if provided
	if langParam, ok := params["language"].(string); ok {
		query = fmt.Sprintf("%s language:%s", query, langParam)
	}

	// Add path filter if provided
	if pathParam, ok := params["path"].(string); ok {
		query = fmt.Sprintf("%s path:%s", query, pathParam)
	}

	// Add filename filter if provided
	if filenameParam, ok := params["filename"].(string); ok {
		query = fmt.Sprintf("%s filename:%s", query, filenameParam)
	}

	// Add extension filter if provided
	if extensionParam, ok := params["extension"].(string); ok {
		query = fmt.Sprintf("%s extension:%s", query, extensionParam)
	}

	// Search code
	results, _, err := a.client.Search.Code(ctx, query, searchOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to search code: %w", err)
	}

	// Convert to map for easier serialization
	codeResults := make([]map[string]interface{}, 0, len(results.CodeResults))
	for _, result := range results.CodeResults {
		resultMap := map[string]interface{}{
			"name":        result.GetName(),
			"path":        result.GetPath(),
			"sha":         result.GetSHA(),
			"html_url":    result.GetHTMLURL(),
			"repository": map[string]interface{}{
				"name":     result.Repository.GetName(),
				"full_name": result.Repository.GetFullName(),
				"html_url":  result.Repository.GetHTMLURL(),
			},
		}
		codeResults = append(codeResults, resultMap)
	}

	return map[string]interface{}{
		"total_count": results.GetTotal(),
		"items":       codeResults,
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
	return a.healthStatus
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}
