// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	adapterErrors "github.com/S-Corkum/mcp-server/internal/adapters/errors"
	"github.com/S-Corkum/mcp-server/internal/adapters/providers/github/mocks"
	"github.com/S-Corkum/mcp-server/internal/adapters/providers/github/testdata"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// Test constant values
const (
	testTimeout = 5 * time.Second
	testOwner   = "test-owner"
	testRepo    = "test-repo"
	testToken   = "test-token"
	testContext = "test-context"
)

// Mock definitions for GitHub client and services
// These mocks implement the same interfaces as the GitHub API client

// MockGitHubClient is a mock for the GitHub client.
// It's used in tests to avoid making real API calls.
type MockGitHubClient struct {
	mock.Mock
}

// MockIssuesService is a mock for the GitHub Issues service.
// It implements the same methods as the GitHub Issues service.
type MockIssuesService struct {
	mock.Mock
}

// MockRepositoriesService is a mock for the GitHub Repositories service.
// It implements the same methods as the GitHub Repositories service.
type MockRepositoriesService struct {
	mock.Mock
}

// MockPullRequestsService is a mock for the GitHub PullRequests service.
// It implements the same methods as the GitHub PullRequests service.
type MockPullRequestsService struct {
	mock.Mock
}

// Setup mock methods for Issues service
// These methods match the signatures of the GitHub API client

// ListByRepo mocks the GitHub Issues.ListByRepo method
func (m *MockIssuesService) ListByRepo(ctx context.Context, owner string, repo string, 
	opt *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, opt)
	
	issues, ok := args.Get(0).([]*github.Issue)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("invalid type for issues: %T", args.Get(0)))
	}
	
	resp, ok := args.Get(1).(*github.Response)
	if !ok && args.Get(1) != nil {
		panic(fmt.Sprintf("invalid type for response: %T", args.Get(1)))
	}
	
	return issues, resp, args.Error(2)
}

// Create mocks the GitHub Issues.Create method
func (m *MockIssuesService) Create(ctx context.Context, owner string, repo string, 
	issueRequest *github.IssueRequest) (*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, issueRequest)
	
	issue, ok := args.Get(0).(*github.Issue)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("invalid type for issue: %T", args.Get(0)))
	}
	
	resp, ok := args.Get(1).(*github.Response)
	if !ok && args.Get(1) != nil {
		panic(fmt.Sprintf("invalid type for response: %T", args.Get(1)))
	}
	
	return issue, resp, args.Error(2)
}

// Edit mocks the GitHub Issues.Edit method
func (m *MockIssuesService) Edit(ctx context.Context, owner string, repo string, number int, 
	issueRequest *github.IssueRequest) (*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, number, issueRequest)
	
	issue, ok := args.Get(0).(*github.Issue)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("invalid type for issue: %T", args.Get(0)))
	}
	
	resp, ok := args.Get(1).(*github.Response)
	if !ok && args.Get(1) != nil {
		panic(fmt.Sprintf("invalid type for response: %T", args.Get(1)))
	}
	
	return issue, resp, args.Error(2)
}

// CreateComment mocks the GitHub Issues.CreateComment method
func (m *MockIssuesService) CreateComment(ctx context.Context, owner string, repo string, number int, 
	comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	args := m.Called(ctx, owner, repo, number, comment)
	
	issueComment, ok := args.Get(0).(*github.IssueComment)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("invalid type for comment: %T", args.Get(0)))
	}
	
	resp, ok := args.Get(1).(*github.Response)
	if !ok && args.Get(1) != nil {
		panic(fmt.Sprintf("invalid type for response: %T", args.Get(1)))
	}
	
	return issueComment, resp, args.Error(2)
}

// Setup mock methods for Repositories service

// List mocks the GitHub Repositories.List method
func (m *MockRepositoriesService) List(ctx context.Context, user string, 
	opt *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
	args := m.Called(ctx, user, opt)
	
	repos, ok := args.Get(0).([]*github.Repository)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("invalid type for repositories: %T", args.Get(0)))
	}
	
	resp, ok := args.Get(1).(*github.Response)
	if !ok && args.Get(1) != nil {
		panic(fmt.Sprintf("invalid type for response: %T", args.Get(1)))
	}
	
	return repos, resp, args.Error(2)
}

// Setup mock methods for PullRequests service

// List mocks the GitHub PullRequests.List method
func (m *MockPullRequestsService) List(ctx context.Context, owner string, repo string, 
	opt *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	args := m.Called(ctx, owner, repo, opt)
	
	prs, ok := args.Get(0).([]*github.PullRequest)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("invalid type for pull requests: %T", args.Get(0)))
	}
	
	resp, ok := args.Get(1).(*github.Response)
	if !ok && args.Get(1) != nil {
		panic(fmt.Sprintf("invalid type for response: %T", args.Get(1)))
	}
	
	return prs, resp, args.Error(2)
}

// Create mocks the GitHub PullRequests.Create method
func (m *MockPullRequestsService) Create(ctx context.Context, owner string, repo string, 
	pull *github.NewPullRequest) (*github.PullRequest, *github.Response, error) {
	args := m.Called(ctx, owner, repo, pull)
	
	pr, ok := args.Get(0).(*github.PullRequest)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("invalid type for pull request: %T", args.Get(0)))
	}
	
	resp, ok := args.Get(1).(*github.Response)
	if !ok && args.Get(1) != nil {
		panic(fmt.Sprintf("invalid type for response: %T", args.Get(1)))
	}
	
	return pr, resp, args.Error(2)
}

// Merge mocks the GitHub PullRequests.Merge method
func (m *MockPullRequestsService) Merge(ctx context.Context, owner string, repo string, number int, 
	commitMessage string, opt *github.PullRequestOptions) (*github.PullRequestMergeResult, *github.Response, error) {
	args := m.Called(ctx, owner, repo, number, commitMessage, opt)
	
	result, ok := args.Get(0).(*github.PullRequestMergeResult)
	if !ok && args.Get(0) != nil {
		panic(fmt.Sprintf("invalid type for merge result: %T", args.Get(0)))
	}
	
	resp, ok := args.Get(1).(*github.Response)
	if !ok && args.Get(1) != nil {
		panic(fmt.Sprintf("invalid type for response: %T", args.Get(1)))
	}
	
	return result, resp, args.Error(2)
}

// Helper functions for test setup and utilities

// createTestContext creates a context with timeout for testing
func createTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), testTimeout)
}

// createMockAdapter creates a mock GitHub adapter for testing.
// It initializes the adapter with mock services for issues, repositories, and pull requests.
//
// Parameters:
//   - t: The testing instance
//   - customConfig: Optional custom configuration (use nil for default)
//
// Returns:
//   - Initialized GitHubAdapter with mock services
//   - MockIssuesService instance
//   - MockRepositoriesService instance
//   - MockPullRequestsService instance
func createMockAdapter(t *testing.T, customConfig *Config) (*GitHubAdapter, *MockIssuesService, *MockRepositoriesService, *MockPullRequestsService) {
	// Create mock services
	mockIssuesService := new(MockIssuesService)
	mockRepositoriesService := new(MockRepositoriesService)
	mockPullRequestsService := new(MockPullRequestsService)

	// Create basic adapter for testing
	config := DefaultConfig()
	if customConfig != nil {
		config = *customConfig
	} else {
		config.DefaultOwner = testOwner
		config.DefaultRepo = testRepo
		config.Token = testToken
	}

	// Create test event bus and metrics client
	eventBus := events.NewEventBus(observability.NewLogger("test-event-bus"))
	metricsClient := observability.NewMetricsClient()
	logger := observability.NewLogger("test-adapter")

	// Create adapter
	adapter, err := NewAdapter(config, eventBus, metricsClient, logger)
	require.NoError(t, err, "Failed to create adapter")

	// Replace the GitHub client with mocks
	adapter.client = &github.Client{
		Issues:       mockIssuesService,
		Repositories: mockRepositoriesService,
		PullRequests: mockPullRequestsService,
	}

	return adapter, mockIssuesService, mockRepositoriesService, mockPullRequestsService
}

// createResponseWithStatus creates a mock HTTP response with a specific status code.
// This is useful for testing error handling with different HTTP status codes.
//
// Parameters:
//   - statusCode: The HTTP status code to include in the response
//
// Returns:
//   - github.Response with the specified status code
func createResponseWithStatus(statusCode int) *github.Response {
	return &github.Response{
		Response: &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{},
		},
	}
}

// setupMockExpectations sets up common mock expectations for testing.
// This centralizes the setup of mock expectations for reuse across tests.
//
// Parameters:
//   - mocks: A map of mock services to set up
//   - expectations: A map of expectation names to setup functions
func setupMockExpectations(mocks map[string]interface{}, expectations map[string]func()) {
	for name, setupFunc := range expectations {
		if setupFunc != nil {
			setupFunc()
		}
	}
}

// TestNewAdapter tests the adapter creation with various configurations.
// It verifies that the adapter is properly initialized with different configurations
// and that errors are returned for invalid configurations.
func TestNewAdapter(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name          string
		config        Config
		expectError   bool
		errorContains string
	}{
		{
			name: "valid config with token",
			config: Config{
				Token:        testToken,
				Timeout:      10 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
			},
			expectError: false,
		},
		{
			name: "valid config with GitHub App",
			config: Config{
				AppID:        "12345",
				InstallID:    "67890",
				PrivateKey:   "test-key",
				Timeout:      10 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
			},
			expectError: false,
		},
		{
			name: "invalid config - missing authentication",
			config: Config{
				Timeout:      10 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
			},
			expectError:   true,
			errorContains: "authentication is required",
		},
		{
			name: "invalid config - missing repo for repo features",
			config: Config{
				Token:   testToken,
				Timeout: 10 * time.Second,
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
			},
			expectError:   true,
			errorContains: "default owner and repository are required",
		},
		{
			name: "invalid config - negative timeout",
			config: Config{
				Token:        testToken,
				Timeout:      -1 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
			},
			expectError:   true,
			errorContains: "timeout must be positive",
		},
		{
			name: "invalid config - empty features",
			config: Config{
				Token:        testToken,
				Timeout:      10 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{},
			},
			expectError:   true, 
			errorContains: "at least one feature must be enabled",
		},
		{
			name: "invalid config - unknown feature",
			config: Config{
				Token:        testToken,
				Timeout:      10 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{
					FeatureIssues, "unknown-feature",
				},
			},
			expectError:   true,
			errorContains: "unknown feature",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create dependencies for adapter
			eventBus := events.NewEventBus(observability.NewLogger("test-event-bus"))
			metricsClient := observability.NewMetricsClient()
			logger := observability.NewLogger("test-adapter")

			// Create adapter
			adapter, err := NewAdapter(tc.config, eventBus, metricsClient, logger)

			// Check results
			if tc.expectError {
				assert.Error(t, err, "Expected error for %s", tc.name)
				assert.Nil(t, adapter, "Adapter should be nil when error occurs")
				
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains, 
						"Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Expected no error for %s", tc.name)
				require.NotNil(t, adapter, "Adapter should not be nil")
				assert.Equal(t, "github", adapter.Type(), "Adapter type should be correct")
				assert.Equal(t, tc.config.DefaultOwner, adapter.defaultOwner, "Default owner should match")
				assert.Equal(t, tc.config.DefaultRepo, adapter.defaultRepo, "Default repo should match")
				
				// Verify feature map
				for _, feature := range tc.config.EnabledFeatures {
					assert.True(t, adapter.featuresEnabled[feature], 
						"Feature %s should be enabled", feature)
				}
				
				// Verify adapter cleanup
				closeErr := adapter.Close()
				assert.NoError(t, closeErr, "Close should not return error")
			}
		})
	}
}

// TestNewAdapterWithNilDependencies tests adapter creation with nil dependencies
func TestNewAdapterWithNilDependencies(t *testing.T) {
	// Create valid config
	config := DefaultConfig()
	config.Token = testToken
	config.DefaultOwner = testOwner
	config.DefaultRepo = testRepo
	
	// Create valid dependencies
	eventBus := events.NewEventBus(observability.NewLogger("test-event-bus"))
	metricsClient := observability.NewMetricsClient()
	
	// Test with nil logger
	t.Run("nil logger", func(t *testing.T) {
		adapter, err := NewAdapter(config, eventBus, metricsClient, nil)
		assert.Error(t, err, "Should error with nil logger")
		assert.Nil(t, adapter, "Adapter should be nil")
		assert.Contains(t, err.Error(), "logger cannot be nil", "Error should mention nil logger")
	})
}

// TestHandleWebhook tests the webhook handling functionality
func TestHandleWebhook(t *testing.T) {
	// Create test adapter
	adapter, _, _, _ := createMockAdapter(t, nil)

	// Create test server to handle webhook events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Define test cases
	testCases := []struct {
		name        string
		eventType   string
		payload     interface{}
		expectError bool
	}{
		{
			name:      "issue event",
			eventType: "issues",
			payload: github.IssuesEvent{
				Action: github.String("opened"),
				Repo: &github.Repository{
					FullName: github.String("test-owner/test-repo"),
				},
				Issue: &github.Issue{
					Number: github.Int(1),
					Title:  github.String("Test Issue"),
				},
			},
			expectError: false,
		},
		{
			name:      "pull request event",
			eventType: "pull_request",
			payload: github.PullRequestEvent{
				Action: github.String("opened"),
				Repo: &github.Repository{
					FullName: github.String("test-owner/test-repo"),
				},
				PullRequest: &github.PullRequest{
					Number: github.Int(1),
					Title:  github.String("Test PR"),
				},
			},
			expectError: false,
		},
		{
			name:      "issue comment event",
			eventType: "issue_comment",
			payload: github.IssueCommentEvent{
				Action: github.String("created"),
				Repo: &github.Repository{
					FullName: github.String("test-owner/test-repo"),
				},
				Issue: &github.Issue{
					Number: github.Int(1),
				},
				Comment: &github.IssueComment{
					ID:   github.Int64(1),
					Body: github.String("Test Comment"),
				},
			},
			expectError: false,
		},
		{
			name:        "invalid payload",
			eventType:   "invalid",
			payload:     "not a valid payload",
			expectError: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert payload to JSON
			payloadBytes, err := json.Marshal(tc.payload)
			assert.NoError(t, err)

			// Call HandleWebhook
			err = adapter.HandleWebhook(context.Background(), tc.eventType, payloadBytes)

			// Check results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestClose tests the Close method
func TestClose(t *testing.T) {
	// Create test adapter
	adapter, _, _, _ := createMockAdapter(t, nil)

	// Call Close
	err := adapter.Close()

	// Check results
	assert.NoError(t, err)
}
