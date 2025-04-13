package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/pkg/models"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

// Config holds configuration for the GitHub adapter
type Config struct {
	APIToken         string        `mapstructure:"api_token"`
	WebhookSecret    string        `mapstructure:"webhook_secret"`
	RequestTimeout   time.Duration `mapstructure:"request_timeout"`
	RateLimitPerHour int           `mapstructure:"rate_limit_per_hour"`
	MaxRetries       int           `mapstructure:"max_retries"`
	RetryDelay       time.Duration `mapstructure:"retry_delay"`
	MockResponses    bool          `mapstructure:"mock_responses"`
	MockURL          string        `mapstructure:"mock_url"`
}

// Adapter implements the GitHub integration
type Adapter struct {
	client       *github.Client
	config       Config
	subscribers  map[string][]func(interface{})
	subscriberMu sync.RWMutex
	httpClient   *http.Client
	baseAdapter  adapters.BaseAdapter
}

// NewAdapter creates a new GitHub adapter
func NewAdapter(cfg Config) (*Adapter, error) {
	var client *github.Client
	var httpClient *http.Client
	
	if cfg.MockResponses {
		// Use a standard HTTP client for mock mode
		httpClient = &http.Client{
			Timeout: cfg.RequestTimeout,
		}
		// If mock URL is not specified, use a default localhost URL
		if cfg.MockURL == "" {
			cfg.MockURL = "http://localhost:8081/mock-github"
		}
		// Create a client with a custom base URL for mocking
		client = github.NewClient(httpClient)
		
		// Log that we're using mock mode
		fmt.Println("GitHub adapter running in mock mode with URL:", cfg.MockURL)
	} else {
		// Create OAuth2 client for GitHub authentication
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: cfg.APIToken},
		)
		httpClient = oauth2.NewClient(ctx, ts)

		// Configure timeouts and rate limiting
		httpClient.Timeout = cfg.RequestTimeout

		// Create GitHub client
		client = github.NewClient(httpClient)
	}

	adapter := &Adapter{
		client:      client,
		config:      cfg,
		subscribers: make(map[string][]func(interface{})),
		httpClient:  httpClient,
		baseAdapter: adapters.BaseAdapter{
			RetryMax:   cfg.MaxRetries,
			RetryDelay: cfg.RetryDelay,
		},
	}

	return adapter, nil
}

// Initialize initializes the adapter
func (a *Adapter) Initialize(ctx context.Context, config interface{}) error {
	// Additional initialization if needed
	return nil
}

// GetData retrieves data from GitHub
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	queryParams, ok := query.(models.GitHubQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type, expected GitHubQuery")
	}

	var result interface{}
	var err error

	// If we're in mock mode, return mock data
	if a.config.MockResponses {
		return a.getMockData(ctx, queryParams)
	}

	// Execute the query with retry logic
	err = a.baseAdapter.CallWithRetry(func() error {
		switch queryParams.Type {
		case models.GitHubQueryTypeRepository:
			repo, _, err := a.client.Repositories.Get(ctx, queryParams.Owner, queryParams.Repo)
			if err != nil {
				return err
			}
			result = repo

		case models.GitHubQueryTypePullRequests:
			opts := &github.PullRequestListOptions{
				State: queryParams.State,
				ListOptions: github.ListOptions{
					PerPage: 100,
				},
			}
			prs, _, err := a.client.PullRequests.List(ctx, queryParams.Owner, queryParams.Repo, opts)
			if err != nil {
				return err
			}
			result = prs

		// Add more query types as needed

		default:
			return fmt.Errorf("unsupported query type: %s", queryParams.Type)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// getMockData returns mock data for testing
func (a *Adapter) getMockData(ctx context.Context, queryParams models.GitHubQuery) (interface{}, error) {
	// Create mock data based on the query type
	switch queryParams.Type {
	case models.GitHubQueryTypeRepository:
		// Create a simplified mock repository without using github.Repository
		type MockRepository struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			OwnerLogin    string `json:"owner_login"`
			HTMLURL       string `json:"html_url"`
			Description   string `json:"description"`
			DefaultBranch string `json:"default_branch"`
			CreatedAt     string `json:"created_at"`
			UpdatedAt     string `json:"updated_at"`
		}
		
		mockRepo := &MockRepository{
			ID:            12345,
			Name:          string(queryParams.Repo),
			FullName:      fmt.Sprintf("%s/%s", queryParams.Owner, queryParams.Repo),
			OwnerLogin:    string(queryParams.Owner),
			HTMLURL:       fmt.Sprintf("https://github.com/%s/%s", queryParams.Owner, queryParams.Repo),
			Description:   "This is a mock repository for testing",
			DefaultBranch: "main",
			CreatedAt:     time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			UpdatedAt:     time.Now().Format(time.RFC3339),
		}
		return mockRepo, nil

	case models.GitHubQueryTypePullRequests:
		// Create simplified mock pull requests without using github.PullRequest
		type MockPullRequestBranch struct {
			Ref      string `json:"ref"`
			RepoName string `json:"repo_name"`
		}
		
		type MockPullRequest struct {
			ID        int64  `json:"id"`
			Number    int    `json:"number"`
			Title     string `json:"title"`
			State     string `json:"state"`
			UserLogin string `json:"user_login"`
			Body      string `json:"body"`
			Base      MockPullRequestBranch `json:"base"`
			Head      MockPullRequestBranch `json:"head"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		}
		
		mockPRs := []MockPullRequest{
			{
				ID:        1001,
				Number:    101,
				Title:     "Mock PR 1",
				State:     string(queryParams.State),
				UserLogin: "mock-user",
				Body:      "This is a mock pull request",
				Base: MockPullRequestBranch{
					Ref:      "main",
					RepoName: string(queryParams.Repo),
				},
				Head: MockPullRequestBranch{
					Ref:      "feature-branch",
					RepoName: string(queryParams.Repo),
				},
				CreatedAt: time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
				UpdatedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
			{
				ID:        1002,
				Number:    102,
				Title:     "Mock PR 2",
				State:     string(queryParams.State),
				UserLogin: "another-user",
				Body:      "Another mock pull request",
				Base: MockPullRequestBranch{
					Ref:      "main",
					RepoName: string(queryParams.Repo),
				},
				Head: MockPullRequestBranch{
					Ref:      "bug-fix",
					RepoName: string(queryParams.Repo),
				},
				CreatedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
				UpdatedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			},
		}
		return mockPRs, nil

	default:
		return nil, fmt.Errorf("unsupported query type for mock data: %s", queryParams.Type)
	}
}

// HandleWebhook processes GitHub webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Verify webhook signature if secret is configured
	// This would be done at the API layer

	// Parse the event
	event, err := a.parseWebhookEvent(eventType, payload)
	if err != nil {
		return err
	}

	// Notify subscribers
	a.notifySubscribers(eventType, event)

	return nil
}

// parseWebhookEvent parses a GitHub webhook event
func (a *Adapter) parseWebhookEvent(eventType string, payload []byte) (interface{}, error) {
	switch eventType {
	case "push":
		var event github.PushEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil

	case "pull_request":
		var event github.PullRequestEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil

	// Add more event types as needed

	default:
		// For unknown event types, return raw payload
		var event map[string]interface{}
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil
	}
}

// Subscribe adds a callback for a specific event type
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
	a.subscriberMu.Lock()
	defer a.subscriberMu.Unlock()

	a.subscribers[eventType] = append(a.subscribers[eventType], callback)
	return nil
}

// notifySubscribers notifies subscribers of an event
func (a *Adapter) notifySubscribers(eventType string, event interface{}) {
	a.subscriberMu.RLock()
	defer a.subscriberMu.RUnlock()

	for _, callback := range a.subscribers[eventType] {
		go callback(event)
	}
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	// If we're in mock mode, return healthy
	if a.config.MockResponses {
		return "healthy (mock)"
	}
	
	// Check GitHub API status by making a simple request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Just try to get rate limit info as a simple API test
	_, _, err := a.client.RateLimits(ctx)
	if err != nil {
		return fmt.Sprintf("unhealthy: %v", err)
	}

	return "healthy"
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing to clean up for this adapter
	return nil
}
