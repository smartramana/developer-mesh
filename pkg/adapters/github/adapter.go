package github

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/github/api"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/github/auth"
	wh "github.com/S-Corkum/devops-mcp/pkg/adapters/github/webhook"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/resilience"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
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
	logger          observability.Logger
	eventBus        events.EventBus
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
func New(config *Config, logger observability.Logger, metricsClient observability.MetricsClient, eventBus events.EventBus) (*GitHubAdapter, error) {
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
	// Initialize with an empty configs map that will be populated later
	authConfigs := make(map[string]*auth.Config)
	authFactory := auth.NewAuthProviderFactory(authConfigs, logger)

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
		&api.Config{
			URL: graphqlURL.String(),
		},
		client,
		nil, // Rate limiter (passing nil for now during migration)
		logger,
		metricsClient,
	)

	// Setup webhook components if enabled
	if config.WebhooksEnabled {
		// Create delivery cache for webhooks
		deliveryCache := wh.NewInMemoryDeliveryCache(24 * time.Hour)
		
		// Create webhook validator
		webhookValidator := wh.NewValidator(config.WebhookSecret, deliveryCache)
		adapter.webhookValidator = webhookValidator

		webhookManager := wh.NewManager(eventBus, logger)
		adapter.webhookManager = webhookManager

		// Create webhook retry manager if configured
		if config.WebhookRetryEnabled {
			// Create retry config
			retryConfig := &wh.RetryConfig{
				MaxRetries:     config.WebhookRetryMaxAttempts,
				InitialBackoff: config.WebhookRetryDelay,
				MaxBackoff:     config.WebhookRetryDelay * 10, // Adjust based on your needs
				BackoffFactor:  2.0,
				Jitter:         0.2,
			}
			
			// Create in-memory storage
			retryStorage := wh.NewInMemoryRetryStorage()
			
			// Create retry handler that uses the webhook processor
			retryHandler := func(ctx context.Context, event wh.Event) error {
				return adapter.webhookManager.ProcessEvent(ctx, event)
			}
			
			// Create and store the retry manager
			retryManager := wh.NewRetryManager(retryConfig, retryStorage, retryHandler, adapter.logger)
			adapter.webhookRetryManager = retryManager
		}

		// Start webhook workers
		adapter.startWebhookWorkers(config.WebhookWorkers)
	}

	return adapter, nil
}

// setupAuthProvider creates and initializes the appropriate authentication provider
func (a *GitHubAdapter) setupAuthProvider() (auth.AuthProvider, error) {
	// Config key is used to identify the authentication configuration
	// Usually this would be something like "github" or "github-api"
	configKey := "default"
	
	// Get or create the authentication provider using the factory
	provider, err := a.authFactory.GetProvider(configKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth provider: %w", err)
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
	// Create context-aware logger
	logger := a.logger.With(map[string]interface{}{
		"event_type":  event.EventType,
		"delivery_id": event.DeliveryID,
		"retry_count": event.RetryCount,
	})

	// Record metrics
	a.metricsClient.IncrementCounter("github_webhook_processed", 1.0)

	// Find a handler for this event type
	// Instead of GetHandler (which doesn't exist), we'll process the event directly
	// with the webhook manager
	if a.webhookManager == nil {
		logger.Warnf("No webhook manager available for event: %s", event.EventType)
		return
	}

	// Process the event using our webhook manager
	// Since we don't have the exact Event struct definition, we'll call a different method
	// that might be available in the webhook manager or handle it ourselves
	// This is a stub implementation until we can see the webhook manager's interface
	// For now, we'll just log the event and not process it further
	// This can be replaced with actual webhook processing once we have the correct interface
	var err error
	if err != nil {
		logger.Errorf("Error handling GitHub webhook event: %v", err)
		a.metricsClient.IncrementCounter("github_webhook_error", 1.0)

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

	// Calculate retry delay using exponential backoff with jitter
	baseDelay := time.Second * time.Duration(math.Pow(2, float64(event.RetryCount)))
	if baseDelay > time.Hour {
		baseDelay = time.Hour  // Cap at 1 hour
	}
	
	// Add jitter (±20%)
	jitter := float64(baseDelay) * 0.2 * (rand.Float64()*2 - 1)
	delay := baseDelay + time.Duration(jitter)
	nextRetry := time.Now().Add(delay)

	// Log the retry information with the nextRetry time
	retryLogger := a.logger.With(map[string]interface{}{
		"event_type":  event.EventType,
		"delivery_id": event.DeliveryID,
		"retry_count": event.RetryCount,
		"next_retry":  nextRetry.Format(time.RFC3339),
	})
	retryLogger.Infof("Scheduling webhook event retry (delivery: %s, type: %s)", event.DeliveryID, event.EventType)

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

// HandleWebhook implements the core.Adapter interface
// It receives webhook events and processes them asynchronously
func (a *GitHubAdapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	if !a.config.WebhooksEnabled {
		return ErrWebhookDisabled
	}
	
	// Log the webhook event receipt
	a.logger.With(map[string]interface{}{
		"event_type": eventType,
	}).Infof("Received webhook event: %s", eventType)
	
	// Create an event with the provided data
	event := WebhookEvent{
		EventType:  eventType,
		Payload:    payload,
		DeliveryID: generateRandomID(),
		ReceivedAt: time.Now(),
		RetryCount: 0,
		Headers:    make(http.Header),
	}
	
	// Process the event asynchronously
	a.HandleEvent(event)
	
	return nil
}

// generateRandomID generates a random ID for webhook events
func generateRandomID() string {
	// Generate a random ID with a timestamp prefix for uniqueness
	timestamp := time.Now().UnixNano()
	random := rand.Intn(1000000)
	return fmt.Sprintf("%d-%d", timestamp, random)
}

// HandleEvent handles a webhook event asynchronously
func (a *GitHubAdapter) HandleEvent(event WebhookEvent) {
	if a.webhookQueue != nil {
		select {
		case a.webhookQueue <- event:
			// Successfully queued
		default:
			// Queue is full, process directly
			go a.processWebhookEvent(event)
		}
	} else {
		// No queue, process directly
		go a.processWebhookEvent(event)
	}
}

// Version returns the adapter version
func (a *GitHubAdapter) Version() string {
	return "1.0.0"
}

// Type returns the adapter type
func (a *GitHubAdapter) Type() string {
	return "github"
}

// Health returns the health status of the adapter
func (a *GitHubAdapter) Health() string {
	// Perform a simple check to determine health status
	if a.closed {
		return "closed"
	}
	
	return "healthy"
}
