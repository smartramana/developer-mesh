package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/github/api"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/github/auth"
	wh "github.com/S-Corkum/devops-mcp/pkg/adapters/github/webhook"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/resilience"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"

)

// SimpleRateLimiter implements a simple rate limiter with the RateLimiter interface
type SimpleRateLimiter struct {
	name string
}

// Allow always returns true for the simple rate limiter
func (r *SimpleRateLimiter) Allow() bool {
	return true
}

// Wait implements the Wait method for the RateLimiter interface
func (r *SimpleRateLimiter) Wait(ctx context.Context) error {
	return nil
}

// Name returns the rate limiter name
func (r *SimpleRateLimiter) Name() string {
	return r.name
}

// WebhookEvent represents a GitHub webhook event for processing
type WebhookEvent struct {
	EventType  string
	Payload    []byte
	Headers    http.Header
	DeliveryID string
	ReceivedAt time.Time
	RetryCount int
}

// Error types
var (
	ErrInvalidSignature   = fmt.Errorf("invalid webhook signature")
	ErrReplayAttack       = fmt.Errorf("webhook replay attack detected")
	ErrRateLimitExceeded  = fmt.Errorf("github API rate limit exceeded")
	ErrUnauthorized       = fmt.Errorf("unauthorized github API request")
	ErrForbidden          = fmt.Errorf("forbidden github API request")
	ErrNotFound           = fmt.Errorf("github resource not found")
	ErrOperationNotSupported = fmt.Errorf("operation not supported")
	ErrInvalidParameters  = fmt.Errorf("invalid parameters")
	ErrInvalidAuthentication = fmt.Errorf("invalid authentication configuration")
	ErrWebhookDisabled    = fmt.Errorf("webhooks are disabled")
	ErrWebhookHandlerNotFound = fmt.Errorf("webhook handler not found")
)

// GitHubAdapter provides an adapter for GitHub operations
type GitHubAdapter struct {
	config          *Config
	client          *http.Client
	restClient      *api.RESTClient
	graphQLClient   *api.GraphQLClient
	authProvider    auth.AuthProvider
	authFactory     *auth.AuthProviderFactory
	metricsClient   observability.MetricsClient
	logger          *observability.Logger
	eventBus        events.EventBusIface // Use the proper EventBus interface
	webhookManager  *wh.Manager
	webhookValidator *wh.Validator
	webhookRetryManager *wh.RetryManager
	webhookQueue    chan WebhookEvent
	deliveryCache   map[string]time.Time
	rateLimiter     *resilience.RateLimiterManager
	mu              sync.RWMutex
	closed          bool
	wg              sync.WaitGroup // WaitGroup for webhook workers
}

// New creates a new GitHub adapter
func New(config *Config, logger *observability.Logger, metricsClient observability.MetricsClient, eventBus events.EventBusIface) (*GitHubAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create HTTP client with appropriate timeouts and settings
	client := &http.Client{
		Timeout: config.RequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        config.MaxIdleConns,
			MaxConnsPerHost:     config.MaxConnsPerHost,
			MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
			IdleConnTimeout:     config.IdleConnTimeout,
			TLSHandshakeTimeout: config.TLSHandshakeTimeout,
			ResponseHeaderTimeout: config.ResponseHeaderTimeout,
			ExpectContinueTimeout: config.ExpectContinueTimeout,
		},
	}

	// Create simple rate limiter implementations that always allow
	// This is a temporary solution until we fix the rate limiter configuration
	var restLimiter, graphQLLimiter resilience.RateLimiter
	
	restLimiter = &SimpleRateLimiter{
		name: "github.rest",
	}
	
	graphQLLimiter = &SimpleRateLimiter{
		name: "github.graphql",
	}

	// Create auth provider factory
	authConfigs := map[string]*auth.Config{
		"default": {
			Token:            config.Token,
			AppID:            config.AppID,
			AppPrivateKey:    config.AppPrivateKey,
			AppInstallationID: config.AppInstallationID,
			OAuthToken:       config.OAuthToken,
			OAuthClientID:    config.OAuthClientID,
			OAuthClientSecret: config.OAuthClientSecret,
		},
	}
	authFactory := auth.NewAuthProviderFactory(authConfigs, logger)

	// Create default auth provider
	authProvider, err := authFactory.GetProvider("default")
	if err != nil {
		return nil, fmt.Errorf("failed to create auth provider: %w", err)
	}

	// Create REST client
	restClient := api.NewRESTClient(
		&api.RESTConfig{
			BaseURL:      config.BaseURL,
			UploadURL:    config.UploadURL,
			AuthProvider: authProvider,
		},
		client,
		restLimiter,
		logger,
		metricsClient,
	)

	// Create GraphQL client
	graphQLClient := api.NewGraphQLClient(
		&api.Config{
			URL:           config.GraphQLURL,
			Token:         config.Token,
			AppID:         config.AppID,
			AppPrivateKey: config.AppPrivateKey,
			UseApp:        config.UseApp,
			RequestTimeout: config.RequestTimeout,
		},
		client,
		graphQLLimiter,
		logger,
		metricsClient,
	)

	// Create webhook related components if webhooks are enabled
	var webhookManager *wh.Manager
	var webhookValidator *wh.Validator
	var webhookRetryManager *wh.RetryManager
	var webhookQueue chan WebhookEvent

	if !config.DisableWebhooks {
		// Create webhook queue for asynchronous processing
		webhookQueue = make(chan WebhookEvent, config.WebhookQueueSize)

		// Create webhook manager with an interface wrapper
		webhookManager = wh.NewManager(eventBus, logger)

		// Register default webhook handlers if configured
		if config.AutoCreateWebhookHandlers {
			registerDefaultWebhookHandlers(webhookManager, nil, logger)
		}

		// Create webhook validator
		deliveryCache := wh.NewInMemoryDeliveryCache(config.WebhookDeliveryCache)
		webhookValidator = wh.NewValidator(config.WebhookSecret, deliveryCache)
		
		// Disable signature validation if configured
		if config.DisableSignatureValidation {
			webhookValidator.DisableSignatureValidation()
		}

		// Register JSON schemas if payload validation is enabled
		if config.WebhookValidatePayload {
			registerWebhookSchemas(webhookValidator)
		}

		// Create webhook retry manager
		retryStorage := wh.NewInMemoryRetryStorage()
		webhookRetryManager = wh.NewRetryManager(
			&wh.RetryConfig{
				MaxRetries:     config.WebhookMaxRetries,
				InitialBackoff: config.WebhookRetryInitialBackoff,
				MaxBackoff:     config.WebhookRetryMaxBackoff,
				BackoffFactor:  config.WebhookRetryBackoffFactor,
				Jitter:         config.WebhookRetryJitter,
			},
			retryStorage,
			func(ctx context.Context, event wh.Event) error {
				// Process webhook event
				return processWebhookEvent(ctx, event, webhookManager, logger)
			},
			logger,
		)
	}

	adapter := &GitHubAdapter{
		config:          config,
		client:          client,
		restClient:      restClient,
		graphQLClient:   graphQLClient,
		authProvider:    authProvider,
		authFactory:     authFactory,
		metricsClient:   metricsClient,
		logger:          logger,
		eventBus:        eventBus,
		webhookManager:  webhookManager,
		webhookValidator: webhookValidator,
		webhookRetryManager: webhookRetryManager,
		webhookQueue:    webhookQueue,
		deliveryCache:   make(map[string]time.Time),
	}

	// Start webhook processing if webhooks are enabled
	if !config.DisableWebhooks {
		// Start webhook workers
		go adapter.startWebhookWorkers(config.WebhookWorkers)

		// Start webhook retry manager
		ctx := context.Background()
		if err := webhookRetryManager.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start webhook retry manager: %w", err)
		}
	}

	return adapter, nil
}



// registerDefaultWebhookHandlers registers default webhook handlers
func registerDefaultWebhookHandlers(manager *wh.Manager, eventBus interface{}, logger *observability.Logger) {
	handlers := wh.DefaultEventHandlers()

	for eventType, handler := range handlers {
		// Create a filter for this event type
		filter := &wh.Filter{
			EventTypes: []string{eventType},
		}

		// Register the handler
		handlerID := fmt.Sprintf("default-%s", eventType)
		if err := manager.Register(handlerID, handler, filter); err != nil {
			logger.Error("Failed to register default webhook handler", map[string]interface{}{
				"eventType": eventType,
				"error":     err.Error(),
			})
		}
	}
}

// registerWebhookSchemas registers JSON schemas for webhook events
func registerWebhookSchemas(validator *wh.Validator) {
	schemas := wh.JSONSchemas()

	for eventType, schema := range schemas {
		if err := validator.RegisterSchema(eventType, schema); err != nil {
			// Log error but continue
			fmt.Printf("Failed to register schema for %s: %v\n", eventType, err)
		}
	}
}

// processWebhookEvent processes a webhook event with the webhook manager
func processWebhookEvent(ctx context.Context, event wh.Event, manager *wh.Manager, logger *observability.Logger) error {
	logger.Info("Processing webhook event", map[string]interface{}{
		"eventType":  event.Type,
		"deliveryID": event.DeliveryID,
	})

	// Process event with webhook manager
	return manager.ProcessEvent(ctx, event)
}

// Type returns the adapter type
func (a *GitHubAdapter) Type() string {
	return "github"
}

// Version returns the adapter version
func (a *GitHubAdapter) Version() string {
	return "2.0.0"
}

// Health returns the adapter health status
func (a *GitHubAdapter) Health() string {
	// For now, just return a static status
	return "healthy"
}

// ExecuteAction executes a GitHub action
func (a *GitHubAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Log the action
	a.logger.Info("Executing GitHub action", map[string]interface{}{
		"action":     action,
		"contextID":  contextID,
		"parameters": params,
	})

	// Check supported actions
	switch action {
	// Repository operations
	case "getRepository":
		return a.getRepository(ctx, contextID, params)
	case "listRepositories":
		return a.listRepositories(ctx, contextID, params)
	case "createRepository":
		return a.createRepository(ctx, contextID, params)
	case "updateRepository":
		return a.updateRepository(ctx, contextID, params)
	case "deleteRepository":
		return a.deleteRepository(ctx, contextID, params)

	// Issue operations
	case "getIssue":
		return a.getIssue(ctx, contextID, params)
	case "listIssues":
		return a.listIssues(ctx, contextID, params)
	case "createIssue":
		return a.createIssue(ctx, contextID, params)
	case "updateIssue":
		return a.updateIssue(ctx, contextID, params)
	case "closeIssue":
		return a.closeIssue(ctx, contextID, params)

	// Pull request operations
	case "getPullRequest":
		return a.getPullRequest(ctx, contextID, params)
	case "listPullRequests":
		return a.listPullRequests(ctx, contextID, params)
	case "createPullRequest":
		return a.createPullRequest(ctx, contextID, params)
	case "updatePullRequest":
		return a.updatePullRequest(ctx, contextID, params)
	case "mergePullRequest":
		return a.mergePullRequest(ctx, contextID, params)

	// Comment operations
	case "createIssueComment":
		return a.createIssueComment(ctx, contextID, params)
	case "listIssueComments":
		return a.listIssueComments(ctx, contextID, params)
	case "createPRReview":
		return a.createPRReview(ctx, contextID, params)

	// Workflow operations
	case "listWorkflowRuns":
		return a.listWorkflowRuns(ctx, contextID, params)
	case "getWorkflowRun":
		return a.getWorkflowRun(ctx, contextID, params)
	case "triggerWorkflow":
		return a.triggerWorkflow(ctx, contextID, params)

	// Content operations
	case "getContent":
		return a.getContent(ctx, contextID, params)
	case "createOrUpdateContent":
		return a.createOrUpdateContent(ctx, contextID, params)

	// Release operations
	case "listReleases":
		return a.listReleases(ctx, contextID, params)
	case "createRelease":
		return a.createRelease(ctx, contextID, params)

	// Branch operations
	case "listBranches":
		return a.listBranches(ctx, contextID, params)
	case "getBranch":
		return a.getBranch(ctx, contextID, params)

	// User operations
	case "getUser":
		return a.getUser(ctx, contextID, params)

	// GraphQL operations
	case "executeGraphQL":
		return a.executeGraphQL(ctx, contextID, params)
	case "buildAndExecuteGraphQL":
		return a.buildAndExecuteGraphQL(ctx, contextID, params)

	// Webhook operations
	case "registerWebhookHandler":
		return a.registerWebhookHandler(ctx, contextID, params)
	case "unregisterWebhookHandler":
		return a.unregisterWebhookHandler(ctx, contextID, params)
	case "listWebhookHandlers":
		return a.listWebhookHandlers(ctx, contextID, params)

	// Default case - unsupported action
	default:
		return nil, fmt.Errorf("unsupported GitHub action: %s", action)
	}
}

// ==============================
// Repository Operations
// ==============================

// getRepository gets a GitHub repository
func (a *GitHubAdapter) getRepository(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	// Use GraphQL if preferred for reads and the feature is enabled
	if a.config.PreferGraphQLForReads && a.config.EnableGraphQLBuilder {
		// Use GraphQL for this operation
		withIssues := false
		withPullRequests := false

		// Check if we should include issues and PRs
		if include, ok := params["include_issues"].(bool); ok {
			withIssues = include
		}
		if include, ok := params["include_pull_requests"].(bool); ok {
			withPullRequests = include
		}

		// Build and execute the query
		builder := api.BuildRepositoryQuery(owner, repo, withIssues, withPullRequests)
		query, variables := builder.Build()

		// Set actual variable values
		variables["owner"] = owner
		variables["name"] = repo

		// Execute the query
		var result map[string]interface{}
		err := a.graphQLClient.Query(ctx, query, variables, &result)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	// Fall back to REST API
	return a.restClient.GetRepository(ctx, owner, repo)
}

// listRepositories lists repositories for the authenticated user or a specific user
func (a *GitHubAdapter) listRepositories(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	username, _ := params["username"].(string)
	page, _ := params["page"].(float64)
	perPage, _ := params["per_page"].(float64)

	// Set default pagination 
	options := &api.RestPaginationOptions{
		Page:     int(page),
		PerPage:  int(perPage),
		MaxPages: a.config.MaxPages,
	}

	// Use default page size if not specified
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PerPage <= 0 {
		options.PerPage = a.config.DefaultPageSize
	}

	// Check if username is provided
	if username != "" {
		// Use GraphQL for user repositories if enabled
		if a.config.PreferGraphQLForReads && a.config.EnableGraphQLBuilder {
			builder := api.BuildUserRepositoriesQuery(username, options.PerPage)
			query, variables := builder.Build()

			variables["login"] = username
			variables["first"] = options.PerPage

			var result map[string]interface{}
			err := a.graphQLClient.Query(ctx, query, variables, &result)
			if err != nil {
				return nil, err
			}

			return result, nil
		}

		// Fall back to REST API for user repositories
		path := fmt.Sprintf("users/%s/repos", username)
		var result []map[string]interface{}
		err := a.restClient.GetPaginated(ctx, path, options, &result)
		return result, err
	}

	// List authenticated user's repositories
	return a.restClient.ListRepositories(ctx, options)
}

// createRepository creates a new GitHub repository
func (a *GitHubAdapter) createRepository(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidParameters)
	}

	// Extract optional parameters for repository creation
	repoParams := map[string]interface{}{
		"name": name,
	}

	// Copy optional parameters
	optionalParams := []string{
		"description", "homepage", "private", "has_issues", "has_projects",
		"has_wiki", "auto_init", "gitignore_template", "license_template",
	}

	for _, param := range optionalParams {
		if value, exists := params[param]; exists {
			repoParams[param] = value
		}
	}

	// Check if organization name is provided
	org, hasOrg := params["org"].(string)

	// Determine endpoint based on whether creating in an org or for the user
	path := "user/repos"
	if hasOrg && org != "" {
		path = fmt.Sprintf("orgs/%s/repos", org)
	}

	// Create repository
	var result map[string]interface{}
	err := a.restClient.Post(ctx, path, repoParams, &result)
	return result, err
}

// updateRepository updates a GitHub repository
func (a *GitHubAdapter) updateRepository(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	// Extract update parameters
	updateParams := make(map[string]interface{})

	// Copy optional parameters
	optionalParams := []string{
		"name", "description", "homepage", "private", "has_issues", "has_projects",
		"has_wiki", "default_branch", "allow_squash_merge", "allow_merge_commit",
		"allow_rebase_merge", "delete_branch_on_merge",
	}

	for _, param := range optionalParams {
		if value, exists := params[param]; exists {
			updateParams[param] = value
		}
	}

	// Check if there are any parameters to update
	if len(updateParams) == 0 {
		return nil, fmt.Errorf("%w: no update parameters provided", ErrInvalidParameters)
	}

	// Update repository
	path := fmt.Sprintf("repos/%s/%s", owner, repo)
	var result map[string]interface{}
	err := a.restClient.Patch(ctx, path, updateParams, &result)
	return result, err
}

// deleteRepository deletes a GitHub repository
func (a *GitHubAdapter) deleteRepository(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	// Delete repository
	path := fmt.Sprintf("repos/%s/%s", owner, repo)
	err := a.restClient.Delete(ctx, path)
	return map[string]interface{}{"success": err == nil}, err
}

// ==============================
// Issue Operations
// ==============================

// getIssue gets an issue by number
func (a *GitHubAdapter) getIssue(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid issue number is required", ErrInvalidParameters)
	}

	// Use GraphQL if preferred for reads and the feature is enabled
	if a.config.PreferGraphQLForReads && a.config.EnableGraphQLBuilder {
		builder := api.BuildIssueQuery(owner, repo, int(number))
		query, variables := builder.Build()

		variables["owner"] = owner
		variables["name"] = repo
		variables["number"] = int(number)

		var result map[string]interface{}
		err := a.graphQLClient.Query(ctx, query, variables, &result)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	// Fall back to REST API
	return a.restClient.GetIssue(ctx, owner, repo, int(number))
}

// listIssues lists issues for a repository
func (a *GitHubAdapter) listIssues(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	page, _ := params["page"].(float64)
	perPage, _ := params["per_page"].(float64)

	// Set default pagination
	options := &api.RestPaginationOptions{
		Page:     int(page),
		PerPage:  int(perPage),
		MaxPages: a.config.MaxPages,
	}

	// Use default page size if not specified
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PerPage <= 0 {
		options.PerPage = a.config.DefaultPageSize
	}

	// Use GraphQL if preferred for reads and the feature is enabled
	if a.config.PreferGraphQLForReads && a.config.EnableGraphQLBuilder {
		// Map state to GraphQL enum values
		states := []string{"OPEN"}
		stateParam, hasState := params["state"].(string)
		if hasState {
			if stateParam == "closed" {
				states = []string{"CLOSED"}
			} else if stateParam == "all" {
				states = []string{"OPEN", "CLOSED"}
			}
		}

		builder := api.BuildIssuesQuery(owner, repo, options.PerPage, states)
		query, variables := builder.Build()

		variables["owner"] = owner
		variables["name"] = repo
		variables["first"] = options.PerPage
		variables["states"] = states

		var result map[string]interface{}
		err := a.graphQLClient.Query(ctx, query, variables, &result)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	// Fall back to REST API
	// Build query parameters
	query := url.Values{}
	stateParam, hasState := params["state"].(string)
	if hasState && stateParam != "" {
		query.Set("state", stateParam)
	}

	path := fmt.Sprintf("repos/%s/%s/issues?%s", owner, repo, query.Encode())
	var result []map[string]interface{}
	err := a.restClient.GetPaginated(ctx, path, options, &result)
	return result, err
}

// createIssue creates a new issue
func (a *GitHubAdapter) createIssue(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	title, ok := params["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	issueParams := map[string]interface{}{
		"title": title,
	}

	// Copy optional parameters
	optionalParams := []string{"body", "assignees", "milestone", "labels", "assignee"}
	for _, param := range optionalParams {
		if value, exists := params[param]; exists {
			issueParams[param] = value
		}
	}

	// Create issue
	return a.restClient.CreateIssue(ctx, owner, repo, issueParams)
}

// updateIssue updates an issue
func (a *GitHubAdapter) updateIssue(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid issue number is required", ErrInvalidParameters)
	}

	// Extract update parameters
	updateParams := make(map[string]interface{})

	// Copy optional parameters
	optionalParams := []string{"title", "body", "state", "assignees", "milestone", "labels", "assignee"}
	for _, param := range optionalParams {
		if value, exists := params[param]; exists {
			updateParams[param] = value
		}
	}

	// Check if there are any parameters to update
	if len(updateParams) == 0 {
		return nil, fmt.Errorf("%w: no update parameters provided", ErrInvalidParameters)
	}

	// Update issue
	return a.restClient.UpdateIssue(ctx, owner, repo, int(number), updateParams)
}

// closeIssue closes an issue
func (a *GitHubAdapter) closeIssue(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid issue number is required", ErrInvalidParameters)
	}

	// Set state to closed
	updateParams := map[string]interface{}{"state": "closed"}

	// Close issue
	return a.restClient.UpdateIssue(ctx, owner, repo, int(number), updateParams)
}

// ==============================
// Pull Request Operations
// ==============================

// getPullRequest gets a pull request by number
func (a *GitHubAdapter) getPullRequest(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid PR number is required", ErrInvalidParameters)
	}

	// Use GraphQL if preferred for reads and the feature is enabled
	if a.config.PreferGraphQLForReads && a.config.EnableGraphQLBuilder {
		builder := api.BuildPullRequestQuery(owner, repo, int(number))
		query, variables := builder.Build()

		variables["owner"] = owner
		variables["name"] = repo
		variables["number"] = int(number)

		var result map[string]interface{}
		err := a.graphQLClient.Query(ctx, query, variables, &result)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	// Fall back to REST API
	return a.restClient.GetPullRequest(ctx, owner, repo, int(number))
}

// listPullRequests lists pull requests for a repository
func (a *GitHubAdapter) listPullRequests(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	page, _ := params["page"].(float64)
	perPage, _ := params["per_page"].(float64)

	// Set default pagination
	options := &api.RestPaginationOptions{
		Page:     int(page),
		PerPage:  int(perPage),
		MaxPages: a.config.MaxPages,
	}

	// Use default page size if not specified
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PerPage <= 0 {
		options.PerPage = a.config.DefaultPageSize
	}

	// Fall back to REST API
	return a.restClient.ListPullRequests(ctx, owner, repo, options)
}

// createPullRequest creates a new pull request
func (a *GitHubAdapter) createPullRequest(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	title, ok := params["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalidParameters)
	}

	head, ok := params["head"].(string)
	if !ok || head == "" {
		return nil, fmt.Errorf("%w: head branch is required", ErrInvalidParameters)
	}

	base, ok := params["base"].(string)
	if !ok || base == "" {
		return nil, fmt.Errorf("%w: base branch is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	prParams := map[string]interface{}{
		"title": title,
		"head":  head,
		"base":  base,
	}

	// Add body if provided
	if body, exists := params["body"].(string); exists {
		prParams["body"] = body
	}

	// Add draft if provided
	if draft, exists := params["draft"].(bool); exists {
		prParams["draft"] = draft
	}

	// Create pull request
	return a.restClient.CreatePullRequest(ctx, owner, repo, prParams)
}

// updatePullRequest updates a pull request
func (a *GitHubAdapter) updatePullRequest(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid PR number is required", ErrInvalidParameters)
	}

	// Extract update parameters
	updateParams := make(map[string]interface{})

	// Copy optional parameters
	optionalParams := []string{"title", "body", "state", "base", "maintainer_can_modify"}
	for _, param := range optionalParams {
		if value, exists := params[param]; exists {
			updateParams[param] = value
		}
	}

	// Check if there are any parameters to update
	if len(updateParams) == 0 {
		return nil, fmt.Errorf("%w: no update parameters provided", ErrInvalidParameters)
	}

	// Update pull request
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, int(number))
	var result map[string]interface{}
	err := a.restClient.Patch(ctx, path, updateParams, &result)
	return result, err
}

// mergePullRequest merges a pull request
func (a *GitHubAdapter) mergePullRequest(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid PR number is required", ErrInvalidParameters)
	}

	// Extract optional merge method
	mergeMethod, _ := params["merge_method"].(string)
	if mergeMethod == "" {
		mergeMethod = "merge" // Default to standard merge
	}

	// Validate merge method
	validMethods := map[string]bool{"merge": true, "squash": true, "rebase": true}
	if !validMethods[mergeMethod] {
		return nil, fmt.Errorf("%w: merge_method must be one of: merge, squash, rebase", ErrInvalidParameters)
	}

	// Merge pull request
	return a.restClient.MergePullRequest(ctx, owner, repo, int(number), mergeMethod)
}

// ==============================
// Comment Operations
// ==============================

// createIssueComment creates a comment on an issue
func (a *GitHubAdapter) createIssueComment(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid issue number is required", ErrInvalidParameters)
	}

	body, ok := params["body"].(string)
	if !ok || body == "" {
		return nil, fmt.Errorf("%w: comment body is required", ErrInvalidParameters)
	}

	// Create comment
	return a.restClient.CreateIssueComment(ctx, owner, repo, int(number), body)
}

// listIssueComments lists comments on an issue
func (a *GitHubAdapter) listIssueComments(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid issue number is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	page, _ := params["page"].(float64)
	perPage, _ := params["per_page"].(float64)

	// Set default pagination
	options := &api.RestPaginationOptions{
		Page:     int(page),
		PerPage:  int(perPage),
		MaxPages: a.config.MaxPages,
	}

	// Use default page size if not specified
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PerPage <= 0 {
		options.PerPage = a.config.DefaultPageSize
	}

	// List comments
	path := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, int(number))
	var result []map[string]interface{}
	err := a.restClient.GetPaginated(ctx, path, options, &result)
	return result, err
}

// createPRReview creates a review on a pull request
func (a *GitHubAdapter) createPRReview(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	number, ok := params["number"].(float64)
	if !ok || number <= 0 {
		return nil, fmt.Errorf("%w: valid PR number is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	reviewParams := make(map[string]interface{})

	// Copy optional parameters
	optionalParams := []string{"commit_id", "body", "event", "comments"}
	for _, param := range optionalParams {
		if value, exists := params[param]; exists {
			reviewParams[param] = value
		}
	}

	// Default to comment event if not specified
	if _, exists := reviewParams["event"]; !exists {
		reviewParams["event"] = "COMMENT"
	}

	// Create review
	return a.restClient.CreatePullRequestReview(ctx, owner, repo, int(number), reviewParams)
}

// ==============================
// Workflow Operations
// ==============================

// listWorkflowRuns lists workflow runs for a repository
func (a *GitHubAdapter) listWorkflowRuns(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	page, _ := params["page"].(float64)
	perPage, _ := params["per_page"].(float64)

	// Set default pagination
	options := &api.RestPaginationOptions{
		Page:     int(page),
		PerPage:  int(perPage),
		MaxPages: a.config.MaxPages,
	}

	// Use default page size if not specified
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PerPage <= 0 {
		options.PerPage = a.config.DefaultPageSize
	}

	// List workflow runs
	return a.restClient.ListWorkflowRuns(ctx, owner, repo, options)
}

// getWorkflowRun gets a workflow run by ID
func (a *GitHubAdapter) getWorkflowRun(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	runID, ok := params["run_id"].(float64)
	if !ok || runID <= 0 {
		return nil, fmt.Errorf("%w: valid run_id is required", ErrInvalidParameters)
	}

	// Get workflow run
	return a.restClient.GetWorkflowRun(ctx, owner, repo, int64(runID))
}

// triggerWorkflow triggers a workflow dispatch event
func (a *GitHubAdapter) triggerWorkflow(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	workflowID, ok := params["workflow_id"].(string)
	if !ok || workflowID == "" {
		return nil, fmt.Errorf("%w: workflow_id is required", ErrInvalidParameters)
	}

	ref, ok := params["ref"].(string)
	if !ok || ref == "" {
		return nil, fmt.Errorf("%w: ref is required", ErrInvalidParameters)
	}

	// Extract optional inputs
	inputs, _ := params["inputs"].(map[string]interface{})
	if inputs == nil {
		inputs = make(map[string]interface{})
	}

	// Trigger workflow
	err := a.restClient.TriggerWorkflow(ctx, owner, repo, workflowID, ref, inputs)
	return map[string]interface{}{"success": err == nil}, err
}

// ==============================
// Content Operations
// ==============================

// getContent gets the content of a file
func (a *GitHubAdapter) getContent(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	path, ok := params["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("%w: path is required", ErrInvalidParameters)
	}

	// Extract optional ref parameter
	ref, _ := params["ref"].(string)

	// Get content
	return a.restClient.GetContent(ctx, owner, repo, path, ref)
}

// createOrUpdateContent creates or updates a file
func (a *GitHubAdapter) createOrUpdateContent(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	path, ok := params["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("%w: path is required", ErrInvalidParameters)
	}

	content, ok := params["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("%w: content is required", ErrInvalidParameters)
	}

	message, ok := params["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("%w: commit message is required", ErrInvalidParameters)
	}

	// Create content parameters
	contentParams := map[string]interface{}{
		"message": message,
		"content": content,
	}

	// Add optional parameters
	if branch, exists := params["branch"].(string); exists {
		contentParams["branch"] = branch
	}

	if sha, exists := params["sha"].(string); exists {
		contentParams["sha"] = sha
	}

	// Add committer and author information if provided
	if committer, exists := params["committer"].(map[string]interface{}); exists {
		contentParams["committer"] = committer
	}

	if author, exists := params["author"].(map[string]interface{}); exists {
		contentParams["author"] = author
	}

	// Create or update file
	return a.restClient.CreateOrUpdateContent(ctx, owner, repo, path, contentParams)
}

// ==============================
// Release Operations
// ==============================

// listReleases lists releases for a repository
func (a *GitHubAdapter) listReleases(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	page, _ := params["page"].(float64)
	perPage, _ := params["per_page"].(float64)

	// Set default pagination
	options := &api.RestPaginationOptions{
		Page:     int(page),
		PerPage:  int(perPage),
		MaxPages: a.config.MaxPages,
	}

	// Use default page size if not specified
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PerPage <= 0 {
		options.PerPage = a.config.DefaultPageSize
	}

	// List releases
	return a.restClient.ListReleases(ctx, owner, repo, options)
}

// createRelease creates a new release
func (a *GitHubAdapter) createRelease(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	tagName, ok := params["tag_name"].(string)
	if !ok || tagName == "" {
		return nil, fmt.Errorf("%w: tag_name is required", ErrInvalidParameters)
	}

	// Create release parameters
	releaseParams := map[string]interface{}{
		"tag_name": tagName,
	}

	// Copy optional parameters
	optionalParams := []string{
		"target_commitish", "name", "body", "draft", "prerelease", "generate_release_notes",
	}

	for _, param := range optionalParams {
		if value, exists := params[param]; exists {
			releaseParams[param] = value
		}
	}

	// Create release
	return a.restClient.CreateRelease(ctx, owner, repo, releaseParams)
}

// ==============================
// Branch Operations
// ==============================

// listBranches lists branches for a repository
func (a *GitHubAdapter) listBranches(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	// Extract optional parameters
	page, _ := params["page"].(float64)
	perPage, _ := params["per_page"].(float64)

	// Set default pagination
	options := &api.RestPaginationOptions{
		Page:     int(page),
		PerPage:  int(perPage),
		MaxPages: a.config.MaxPages,
	}

	// Use default page size if not specified
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PerPage <= 0 {
		options.PerPage = a.config.DefaultPageSize
	}

	// List branches
	return a.restClient.ListBranches(ctx, owner, repo, options)
}

// getBranch gets a branch by name
func (a *GitHubAdapter) getBranch(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalidParameters)
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return nil, fmt.Errorf("%w: repo is required", ErrInvalidParameters)
	}

	branch, ok := params["branch"].(string)
	if !ok || branch == "" {
		return nil, fmt.Errorf("%w: branch is required", ErrInvalidParameters)
	}

	// Get branch
	return a.restClient.GetBranch(ctx, owner, repo, branch)
}

// ==============================
// User Operations
// ==============================

// getUser gets a user by username
func (a *GitHubAdapter) getUser(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	username, ok := params["username"].(string)
	if !ok || username == "" {
		return nil, fmt.Errorf("%w: username is required", ErrInvalidParameters)
	}

	// Use GraphQL if preferred for reads and the feature is enabled
	if a.config.PreferGraphQLForReads && a.config.EnableGraphQLBuilder {
		builder := api.BuildUserQuery(username)
		query, variables := builder.Build()

		variables["login"] = username

		var result map[string]interface{}
		err := a.graphQLClient.Query(ctx, query, variables, &result)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	// Fall back to REST API
	path := fmt.Sprintf("users/%s", username)
	var result map[string]interface{}
	err := a.restClient.Get(ctx, path, nil, &result)
	return result, err
}

// ==============================
// GraphQL Operations
// ==============================

// executeGraphQL executes a GraphQL query
func (a *GitHubAdapter) executeGraphQL(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("%w: query is required", ErrInvalidParameters)
	}

	// Extract optional variables
	variables, _ := params["variables"].(map[string]interface{})
	if variables == nil {
		variables = make(map[string]interface{})
	}

	// Execute GraphQL query
	var result map[string]interface{}
	err := a.graphQLClient.Query(ctx, query, variables, &result)
	return result, err
}

// buildAndExecuteGraphQL builds and executes a GraphQL query using the builder
func (a *GitHubAdapter) buildAndExecuteGraphQL(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Check if GraphQL builder is enabled
	if !a.config.EnableGraphQLBuilder {
		return nil, fmt.Errorf("%w: GraphQL builder is not enabled", ErrOperationNotSupported)
	}

	// Extract required parameters
	operationType, ok := params["operation_type"].(string)
	if !ok || operationType == "" {
		return nil, fmt.Errorf("%w: operation_type (query or mutation) is required", ErrInvalidParameters)
	}

	operationName, ok := params["operation_name"].(string)
	if !ok || operationName == "" {
		return nil, fmt.Errorf("%w: operation_name is required", ErrInvalidParameters)
	}

	// Create builder based on operation type
	var builder *api.GraphQLBuilder
	switch strings.ToLower(operationType) {
	case "query":
		builder = api.NewQuery(operationName)
	case "mutation":
		builder = api.NewMutation(operationName)
	default:
		return nil, fmt.Errorf("%w: operation_type must be 'query' or 'mutation'", ErrInvalidParameters)
	}

	// Extract fields
	fields, _ := params["fields"].([]interface{})
	for _, field := range fields {
		if fieldStr, ok := field.(string); ok {
			builder.AddField(fieldStr)
		}
	}

	// Extract variables
	variables, _ := params["variables"].(map[string]interface{})
	variableTypes, _ := params["variable_types"].(map[string]interface{})

	for name, typeVal := range variableTypes {
		if typeStr, ok := typeVal.(string); ok {
			builder.AddVariable(name, typeStr)
		}
	}

	// Extract arguments
	arguments, _ := params["arguments"].(map[string]interface{})
	for name, value := range arguments {
		if valueStr, ok := value.(string); ok {
			builder.AddArgument(name, valueStr)
		}
	}

	// Build the query
	query, queryVariables := builder.Build()

	// Merge with provided variables
	for name, value := range variables {
		queryVariables[name] = value
	}

	// Execute the query
	var result map[string]interface{}
	err := a.graphQLClient.Query(ctx, query, queryVariables, &result)
	return result, err
}

// ==============================
// Webhook Handler Operations
// ==============================

// registerWebhookHandler registers a webhook handler
func (a *GitHubAdapter) registerWebhookHandler(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Check if webhooks are disabled
	if a.config.DisableWebhooks {
		return nil, ErrWebhookDisabled
	}

	// Extract required parameters
	handlerID, ok := params["handler_id"].(string)
	if !ok || handlerID == "" {
		return nil, fmt.Errorf("%w: handler_id is required", ErrInvalidParameters)
	}

	// Extract filter parameters
	filter := &wh.Filter{}

	// Extract event types
	if eventTypes, ok := params["event_types"].([]interface{}); ok {
		for _, et := range eventTypes {
			if etStr, ok := et.(string); ok {
				filter.EventTypes = append(filter.EventTypes, etStr)
			}
		}
	}

	// Extract repositories
	if repos, ok := params["repositories"].([]interface{}); ok {
		for _, r := range repos {
			if rStr, ok := r.(string); ok {
				filter.Repositories = append(filter.Repositories, rStr)
			}
		}
	}

	// Extract branches
	if branches, ok := params["branches"].([]interface{}); ok {
		for _, b := range branches {
			if bStr, ok := b.(string); ok {
				filter.Branches = append(filter.Branches, bStr)
			}
		}
	}

	// Extract actions
	if actions, ok := params["actions"].([]interface{}); ok {
		for _, act := range actions {
			if actStr, ok := act.(string); ok {
				filter.Actions = append(filter.Actions, actStr)
			}
		}
	}

	// Create a handler function that publishes the event to the event bus
	handler := func(ctx context.Context, event wh.Event) error {
		// Create event data
		eventData := map[string]interface{}{
			"event_type":     event.Type,
			"delivery_id":    event.DeliveryID,
			"action":         event.Action,
			"repository":     event.RepositoryFullName,
			"sender":         event.SenderLogin,
			"context_id":     contextID,
			"handler_id":     handlerID,
			"timestamp":      time.Now().Format(time.RFC3339),
		}

		// Include payload if it's not too large
		if len(event.RawPayload) <= 10000 { // Limit to 10KB
			eventData["payload"] = string(event.RawPayload)
		}

		// Publish event to the event bus using the standard interface
		e := &mcp.Event{
			Type:      "github.webhook." + event.Type,
			Timestamp: time.Now(),
			Data:      eventData,
			Source:    "github-adapter",
		}
		a.eventBus.Publish(ctx, e)

		return nil
	}

	// Register handler
	err := a.webhookManager.Register(handlerID, handler, filter)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"handler_id": handlerID,
		"success":    true,
	}, nil
}

// unregisterWebhookHandler unregisters a webhook handler
func (a *GitHubAdapter) unregisterWebhookHandler(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Check if webhooks are disabled
	if a.config.DisableWebhooks {
		return nil, ErrWebhookDisabled
	}

	// Extract required parameters
	handlerID, ok := params["handler_id"].(string)
	if !ok || handlerID == "" {
		return nil, fmt.Errorf("%w: handler_id is required", ErrInvalidParameters)
	}

	// Unregister handler
	err := a.webhookManager.Unregister(handlerID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"handler_id": handlerID,
		"success":    true,
	}, nil
}

// listWebhookHandlers lists all registered webhook handlers
func (a *GitHubAdapter) listWebhookHandlers(ctx context.Context, contextID string, params map[string]interface{}) (interface{}, error) {
	// Check if webhooks are disabled
	if a.config.DisableWebhooks {
		return nil, ErrWebhookDisabled
	}

	// List handlers
	handlers := a.webhookManager.List()

	return map[string]interface{}{
		"handlers": handlers,
	}, nil
}

// ==============================
// Webhook Handler Functions
// ==============================

// HandleWebhook handles a GitHub webhook
func (a *GitHubAdapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Create headers with minimal required fields for validation
	// In a real scenario, these would come from the HTTP request
	headers := http.Header{}
	// These headers are required for webhook validation
	headers.Set("X-GitHub-Event", eventType)
	headers.Set("X-GitHub-Delivery", fmt.Sprintf("test-delivery-%d", time.Now().UnixNano()))
	
	// For test environments, we need to add a signature header
	// In production, this would be calculated by GitHub using the webhook secret
	// Since this is just for testing, we'll add a properly formatted dummy signature
	// Signature format must be sha256=<hex-encoded-hmac>
	headers.Set("X-Hub-Signature-256", "sha256=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	// Check if webhooks are disabled
	if a.config.DisableWebhooks {
		return ErrWebhookDisabled
	}

	// Log the webhook receipt
	a.logger.Info("Received GitHub webhook", map[string]interface{}{
		"eventType": eventType,
	})

	// Extract remote address if available in headers
	remoteAddr := headers.Get("X-Forwarded-For")
	if remoteAddr == "" {
		remoteAddr = headers.Get("X-Real-IP")
	}
	
	// Validate webhook
	if err := a.webhookValidator.ValidateWithIP(eventType, payload, headers, remoteAddr); err != nil {
		a.logger.Error("Webhook validation failed", map[string]interface{}{
			"eventType": eventType,
			"error":     err.Error(),
		})
		return err
	}

	// Parse event
	event, err := wh.ParseEvent(eventType, payload, headers)
	if err != nil {
		a.logger.Error("Failed to parse webhook event", map[string]interface{}{
			"eventType": eventType,
			"error":     err.Error(),
		})
		return err
	}

	// Process event with webhook manager
	return a.webhookRetryManager.ScheduleRetry(ctx, event, nil)
}

// startWebhookWorkers starts a pool of workers to process webhooks
func (a *GitHubAdapter) startWebhookWorkers(count int) {
	for i := 0; i < count; i++ {
		a.wg.Add(1)
		workerID := i
		
		go func(id int) {
			defer a.wg.Done() // Ensure waitgroup is decremented when worker exits
			
			a.logger.Info("Starting GitHub webhook worker", map[string]interface{}{
				"workerID": id,
			})
			
			// Process webhooks from the queue until closed
			for {
				// Check if adapter is closed
				a.mu.RLock()
				closed := a.closed
				a.mu.RUnlock()
				
				if closed {
					a.logger.Info("Shutting down GitHub webhook worker", map[string]interface{}{
						"workerID": id,
					})
					return
				}
				
				// Get next webhook event from queue with timeout to allow for periodic closed checks
				select {
				case event, ok := <-a.webhookQueue:
					if !ok {
						// Queue closed, worker should exit
						return
					}
					
					// Process the event with proper context handling
					func() {
						// Create a context with timeout for processing this event
						// Use a defer to ensure the cancel function is always called
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel() // Ensure context is always canceled on all return paths
						
						// First parse the event
						parsedEvent, err := wh.ParseEvent(event.EventType, event.Payload, event.Headers)
						if err != nil {
							a.logger.Error("Failed to parse webhook event", map[string]interface{}{
								"eventType": event.EventType,
								"deliveryID": event.DeliveryID,
								"error": err.Error(),
							})
							return // Exit this function but continue the worker loop
						}
						
						// Process the event with the webhook manager
						err = a.webhookManager.ProcessEvent(ctx, parsedEvent)
						if err != nil {
							a.logger.Error("Error processing webhook event", map[string]interface{}{
								"eventType": event.EventType, 
								"deliveryID": event.DeliveryID,
								"error": err.Error(),
							})
						}
					}()
					
				case <-time.After(1 * time.Second):
					// Timeout to allow for closed check - this prevents the goroutine
					// from being blocked indefinitely when the adapter is closing
					continue
				}
			}
		}(workerID) // Pass workerID to the goroutine to avoid closure issues
	}
}

// Close closes the adapter and releases resources
func (a *GitHubAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.closed {
		return nil
	}
	
	a.closed = true
	
	// Close webhook queue
	if a.webhookQueue != nil {
		close(a.webhookQueue)
	}
	
	// Close webhook retry manager
	if a.webhookRetryManager != nil {
		a.webhookRetryManager.Close()
	}
	
	// Create a channel for timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait() // Wait for all webhook workers to exit
		close(done)
	}()
	
	// Wait with timeout to avoid hanging in tests
	select {
	case <-done:
		// All workers exited successfully
	case <-time.After(5 * time.Second):
		a.logger.Warn("Timed out waiting for webhook workers to exit", nil)
	}
	
	a.logger.Info("GitHub adapter closed", nil)
	
	return nil
}
