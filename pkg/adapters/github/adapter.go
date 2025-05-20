package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/events"
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
	eventBus        events.EventBusIface
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
		},
	}

	// Create rate limiter manager
	rateLimiterConfig := resilience.RateLimiterConfig{
		Name:      "github-api",
		Rate:      config.RateLimit,
		Burst:     config.RateLimitBurst,
		WaitLimit: config.RateLimitWait,
	}
	rateLimiterConfigs := map[string]resilience.RateLimiterConfig{
		"github-api": rateLimiterConfig,
	}
	rateLimiterManager := resilience.NewRateLimiterManager(rateLimiterConfigs)

	// Create authentication factory
	authFactory := auth.NewAuthProviderFactory()

	// Setup empty webhook cache
	deliveryCache := make(map[string]time.Time)

	// Create adapter instance
	adapter := &GitHubAdapter{
		config:         config,
		client:         client,
		metricsClient:  metricsClient,
		logger:         logger,
		eventBus:       eventBus,
		authFactory:    authFactory,
		deliveryCache:  deliveryCache,
		rateLimiter:    rateLimiterManager,
		webhookQueue:   make(chan WebhookEvent, config.WebhookQueueSize),
	}

	// Setup authentication provider
	authProvider, err := adapter.setupAuthProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to setup auth provider: %w", err)
	}
	adapter.authProvider = authProvider

	// Create REST and GraphQL clients
	baseURL, err := adapter.getBaseURL()
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	adapter.restClient = api.NewRESTClient(
		baseURL,
		client,
		authProvider,
		adapter.handleRateLimiting,
		logger,
	)

	graphqlURL, err := adapter.getGraphQLURL()
	if err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL URL: %w", err)
	}

	adapter.graphQLClient = api.NewGraphQLClient(
		graphqlURL,
		client,
		authProvider,
		adapter.handleRateLimiting,
		logger,
	)

	// Setup webhook components if enabled
	if config.WebhooksEnabled {
		webhookValidator := wh.NewValidator(config.WebhookSecret)
		adapter.webhookValidator = webhookValidator

		webhookManager := wh.NewManager(logger, metricsClient)
		adapter.webhookManager = webhookManager

		// Create webhook retry manager if configured
		if config.WebhookRetryEnabled {
			retryManager := wh.NewRetryManager(config.WebhookRetryMaxAttempts, config.WebhookRetryDelay)
			adapter.webhookRetryManager = retryManager
		}

		// Start webhook workers
		adapter.startWebhookWorkers(config.WebhookWorkers)
	}

	return adapter, nil
}

// setupAuthProvider creates and initializes the appropriate authentication provider
func (a *GitHubAdapter) setupAuthProvider() (auth.AuthProvider, error) {
	var provider auth.AuthProvider
	var err error

	switch strings.ToLower(a.config.Auth.Type) {
	case "token", "pat", "personal_access_token":
		provider, err = a.authFactory.CreateTokenAuthProvider(a.config.Auth.Token)
	case "app", "github_app":
		provider, err = a.authFactory.CreateAppAuthProvider(
			a.config.Auth.AppID,
			a.config.Auth.InstallationID,
			a.config.Auth.PrivateKey,
		)
	case "oauth":
		provider, err = a.authFactory.CreateOAuthAuthProvider(a.config.Auth.Token)
	case "none", "":
		provider = a.authFactory.CreateAnonymousAuthProvider()
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", a.config.Auth.Type)
	}

	if err != nil {
		return nil, err
	}

	return provider, nil
}

// getBaseURL returns the base URL for GitHub API calls
func (a *GitHubAdapter) getBaseURL() (*url.URL, error) {
	baseURL := a.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return url.Parse(baseURL)
}

// getGraphQLURL returns the GraphQL URL for GitHub API calls
func (a *GitHubAdapter) getGraphQLURL() (*url.URL, error) {
	graphqlURL := a.config.GraphQLURL
	if graphqlURL == "" {
		graphqlURL = "https://api.github.com/graphql"
	}
	return url.Parse(graphqlURL)
}

// handleRateLimiting handles rate limiting for GitHub API calls
func (a *GitHubAdapter) handleRateLimiting(info resilience.GitHubRateLimitInfo) {
	limiter, exists := a.rateLimiter.Get("github-api")
	if !exists {
		return
	}

	// Adjust the rate limiter with the info
	if adjuster, ok := limiter.(*resilience.DefaultRateLimiter); ok {
		adjuster.AdjustRateLimit(info)
	}

	// Log rate limit information if close to the limit
	if float64(info.Remaining)/float64(info.Limit) < 0.1 {
		a.logger.Warnf("GitHub API rate limit is low: %d/%d remaining, reset at %s",
			info.Remaining, info.Limit, info.Reset.Format(time.RFC3339))
	}
}

// startWebhookWorkers starts the given number of webhook worker goroutines
func (a *GitHubAdapter) startWebhookWorkers(workers int) {
	a.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go a.webhookWorker(i)
	}
}

// webhookWorker processes webhook events from the queue
func (a *GitHubAdapter) webhookWorker(id int) {
	defer a.wg.Done()

	a.logger.Infof("Started GitHub webhook worker %d", id)

	for event := range a.webhookQueue {
		a.processWebhookEvent(event)
	}

	a.logger.Infof("Stopped GitHub webhook worker %d", id)
}

// processWebhookEvent processes a webhook event
func (a *GitHubAdapter) processWebhookEvent(event WebhookEvent) {
	logger := a.logger.With(
		"event_type", event.EventType,
		"delivery_id", event.DeliveryID,
		"retry_count", event.RetryCount,
	)

	// Record metrics
	a.metricsClient.IncrementCounter("github_webhook_processed", 1, map[string]string{
		"event_type": event.EventType,
	})

	// Find a handler for this event type
	handler := a.webhookManager.GetHandler(event.EventType)
	if handler == nil {
		logger.Warnf("No handler registered for GitHub webhook event: %s", event.EventType)
		return
	}

	// Process the event with the handler
	err := handler.Handle(context.Background(), event.EventType, event.Payload)
	if err != nil {
		logger.Errorf("Error handling GitHub webhook event: %v", err)
		a.metricsClient.IncrementCounter("github_webhook_error", 1, map[string]string{
			"event_type": event.EventType,
			"error":      err.Error(),
		})

		// Attempt retry if enabled and we haven't exceeded max attempts
		if a.config.WebhookRetryEnabled &&
			event.RetryCount < a.config.WebhookRetryMaxAttempts &&
			a.webhookRetryManager != nil {
			a.retryWebhookEvent(event)
		}
	}
}

// retryWebhookEvent schedules a webhook event for retry
func (a *GitHubAdapter) retryWebhookEvent(event WebhookEvent) {
	// Increment retry count
	event.RetryCount++

	// Calculate delay based on retry count (with exponential backoff)
	delay := a.webhookRetryManager.CalculateDelay(event.RetryCount)

	// Schedule retry
	go func() {
		time.Sleep(delay)
		a.webhookQueue <- event
	}()

	a.logger.Infof("Scheduled retry %d/%d for webhook event %s (delivery: %s) in %s",
		event.RetryCount, a.config.WebhookRetryMaxAttempts, event.EventType, event.DeliveryID, delay)
}

// Close gracefully shuts down the adapter
func (a *GitHubAdapter) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	a.mu.Unlock()

	// Close webhook queue if webhooks are enabled
	if a.config.WebhooksEnabled {
		close(a.webhookQueue)
	}

	// Wait for all webhook workers to complete
	a.wg.Wait()

	return nil
}
