package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	adapterErrors "github.com/S-Corkum/mcp-server/internal/adapters/errors"
	"github.com/S-Corkum/mcp-server/internal/adapters/providers/github/mocks"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// MockGitHubClient is a mock for the GitHub client
type MockGitHubClient struct {
	mock.Mock
}

// MockIssuesService is a mock for the GitHub Issues service
type MockIssuesService struct {
	mock.Mock
}

// MockRepositoriesService is a mock for the GitHub Repositories service
type MockRepositoriesService struct {
	mock.Mock
}

// MockPullRequestsService is a mock for the GitHub PullRequests service
type MockPullRequestsService struct {
	mock.Mock
}

// Setup mock methods for Issues service
func (m *MockIssuesService) ListByRepo(ctx context.Context, owner string, repo string, opt *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, opt)
	return args.Get(0).([]*github.Issue), args.Get(1).(*github.Response), args.Error(2)
}

func (m *MockIssuesService) Create(ctx context.Context, owner string, repo string, issueRequest *github.IssueRequest) (*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, issueRequest)
	return args.Get(0).(*github.Issue), args.Get(1).(*github.Response), args.Error(2)
}

func (m *MockIssuesService) Edit(ctx context.Context, owner string, repo string, number int, issueRequest *github.IssueRequest) (*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, number, issueRequest)
	return args.Get(0).(*github.Issue), args.Get(1).(*github.Response), args.Error(2)
}

func (m *MockIssuesService) CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	args := m.Called(ctx, owner, repo, number, comment)
	return args.Get(0).(*github.IssueComment), args.Get(1).(*github.Response), args.Error(2)
}

// Setup mock methods for Repositories service
func (m *MockRepositoriesService) List(ctx context.Context, user string, opt *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
	args := m.Called(ctx, user, opt)
	return args.Get(0).([]*github.Repository), args.Get(1).(*github.Response), args.Error(2)
}

// Setup mock methods for PullRequests service
func (m *MockPullRequestsService) List(ctx context.Context, owner string, repo string, opt *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	args := m.Called(ctx, owner, repo, opt)
	return args.Get(0).([]*github.PullRequest), args.Get(1).(*github.Response), args.Error(2)
}

func (m *MockPullRequestsService) Create(ctx context.Context, owner string, repo string, pull *github.NewPullRequest) (*github.PullRequest, *github.Response, error) {
	args := m.Called(ctx, owner, repo, pull)
	return args.Get(0).(*github.PullRequest), args.Get(1).(*github.Response), args.Error(2)
}

func (m *MockPullRequestsService) Merge(ctx context.Context, owner string, repo string, number int, commitMessage string, opt *github.PullRequestOptions) (*github.PullRequestMergeResult, *github.Response, error) {
	args := m.Called(ctx, owner, repo, number, commitMessage, opt)
	return args.Get(0).(*github.PullRequestMergeResult), args.Get(1).(*github.Response), args.Error(2)
}

// createMockAdapter creates a mock GitHub adapter for testing
func createMockAdapter(t *testing.T) (*GitHubAdapter, *MockIssuesService, *MockRepositoriesService, *MockPullRequestsService) {
	// Create mock services
	mockIssuesService := new(MockIssuesService)
	mockRepositoriesService := new(MockRepositoriesService)
	mockPullRequestsService := new(MockPullRequestsService)

	// Create basic adapter for testing
	config := DefaultConfig()
	config.DefaultOwner = "test-owner"
	config.DefaultRepo = "test-repo"
	config.Token = "test-token"

	// Create test event bus and metrics client
	eventBus := events.NewEventBus()
	metricsClient := observability.NewMetricsClient()
	logger := mocks.NewLogger() // Use our mock logger

	// Create adapter
	adapter, err := NewAdapter(config, eventBus, metricsClient, logger)
	assert.NoError(t, err)

	// Replace the GitHub client with mocks
	adapter.client = &github.Client{
		Issues:       mockIssuesService,
		Repositories: mockRepositoriesService,
		PullRequests: mockPullRequestsService,
	}

	return adapter, mockIssuesService, mockRepositoriesService, mockPullRequestsService
}

// createResponseWithStatus creates a mock HTTP response with a specific status
func createResponseWithStatus(statusCode int) *github.Response {
	return &github.Response{
		Response: &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{},
		},
	}
}

// TestNewAdapter tests the adapter creation with various configurations
func TestNewAdapter(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid config with token",
			config: Config{
				Token:        "test-token",
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
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
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
				},
			},
			expectError: false,
		},
		{
			name: "invalid config - missing authentication",
			config: Config{
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
				},
			},
			expectError: true,
		},
		{
			name: "invalid config - missing repo for repo features",
			config: Config{
				Token:   "test-token",
				Timeout: 10 * time.Second,
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
				},
			},
			expectError: true,
		},
		{
			name: "invalid config - negative timeout",
			config: Config{
				Token:        "test-token",
				Timeout:      -1 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
				},
			},
			expectError: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create event bus and metrics client (needed for adapter)
			eventBus := events.NewEventBus()
			metricsClient := observability.NewMetricsClient()
			logger := mocks.NewLogger() // Use our mock logger

			// Create adapter
			adapter, err := NewAdapter(tc.config, eventBus, metricsClient, logger)

			// Check results
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, adapter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, adapter)
				assert.Equal(t, "github", adapter.AdapterType)
				assert.Equal(t, tc.config.DefaultOwner, adapter.defaultOwner)
				assert.Equal(t, tc.config.DefaultRepo, adapter.defaultRepo)
				
				// Verify feature map
				for _, feature := range tc.config.EnabledFeatures {
					assert.True(t, adapter.featuresEnabled[feature])
				}
			}
		})
	}
}

// TestInitialize tests the adapter initialization
func TestInitialize(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name        string
		config      interface{}
		expectError bool
	}{
		{
			name: "valid config",
			config: Config{
				Token:        "test-token",
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
				},
			},
			expectError: false,
		},
		{
			name:        "invalid config type",
			config:      "not a config",
			expectError: true,
		},
		{
			name: "missing authentication",
			config: Config{
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
			},
			expectError: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create basic adapter for testing
			validConfig := DefaultConfig()
			validConfig.Token = "temp-token"
			validConfig.DefaultOwner = "temp-owner"
			validConfig.DefaultRepo = "temp-repo"

			// Create test event bus and metrics client
			eventBus := events.NewEventBus()
			metricsClient := observability.NewMetricsClient()
			logger := mocks.NewLogger() // Use our mock logger

			// Create adapter
			adapter, err := NewAdapter(validConfig, eventBus, metricsClient, logger)
			assert.NoError(t, err)

			// Initialize with the test case config
			err = adapter.Initialize(context.Background(), tc.config)

			// Check results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, adapter.client)

				// Check that config was updated
				if config, ok := tc.config.(Config); ok {
					assert.Equal(t, config.DefaultOwner, adapter.defaultOwner)
					assert.Equal(t, config.DefaultRepo, adapter.defaultRepo)
				}
			}
		})
	}
}

// TestGetData tests the GetData method
func TestGetData(t *testing.T) {
	// Create test adapter with mocks
	adapter, mockIssuesService, mockRepositoriesService, mockPullRequestsService := createMockAdapter(t)

	// Common response objects
	successResponse := createResponseWithStatus(200)

	// Define test cases
	testCases := []struct {
		name        string
		query       interface{}
		setupMocks  func()
		expectError bool
	}{
		{
			name: "valid issues query",
			query: GitHubDataQuery{
				ResourceType: "issues",
				Owner:        "test-owner",
				Repo:         "test-repo",
				Filters: map[string]interface{}{
					"state": "open",
				},
			},
			setupMocks: func() {
				issues := []*github.Issue{
					{Number: github.Int(1), Title: github.String("Test Issue")},
				}
				mockIssuesService.On("ListByRepo", mock.Anything, "test-owner", "test-repo", mock.Anything).Return(issues, successResponse, nil)
			},
			expectError: false,
		},
		{
			name: "valid repositories query",
			query: GitHubDataQuery{
				ResourceType: "repositories",
				Owner:        "test-owner",
			},
			setupMocks: func() {
				repos := []*github.Repository{
					{Name: github.String("test-repo")},
				}
				mockRepositoriesService.On("List", mock.Anything, "test-owner", mock.Anything).Return(repos, successResponse, nil)
			},
			expectError: false,
		},
		{
			name: "valid pull requests query",
			query: GitHubDataQuery{
				ResourceType: "pull_requests",
				Owner:        "test-owner",
				Repo:         "test-repo",
				Filters: map[string]interface{}{
					"state": "open",
				},
			},
			setupMocks: func() {
				prs := []*github.PullRequest{
					{Number: github.Int(1), Title: github.String("Test PR")},
				}
				mockPullRequestsService.On("List", mock.Anything, "test-owner", "test-repo", mock.Anything).Return(prs, successResponse, nil)
			},
			expectError: false,
		},
		{
			name: "invalid query type",
			query: map[string]string{
				"not": "valid",
			},
			setupMocks:  func() {},
			expectError: true,
		},
		{
			name: "unsupported resource type",
			query: GitHubDataQuery{
				ResourceType: "unsupported",
			},
			setupMocks:  func() {},
			expectError: true,
		},
		{
			name: "issues error response",
			query: GitHubDataQuery{
				ResourceType: "issues",
				Owner:        "test-owner",
				Repo:         "test-repo",
			},
			setupMocks: func() {
				mockIssuesService.On("ListByRepo", mock.Anything, "test-owner", "test-repo", mock.Anything).Return(
					[]*github.Issue{}, successResponse, errors.New("API error"),
				)
			},
			expectError: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			tc.setupMocks()

			// Call GetData
			result, err := adapter.GetData(context.Background(), tc.query)

			// Check results
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				// Check type of result based on query
				if query, ok := tc.query.(GitHubDataQuery); ok {
					switch query.ResourceType {
					case "issues":
						_, ok := result.([]*github.Issue)
						assert.True(t, ok)
					case "repositories":
						_, ok := result.([]*github.Repository)
						assert.True(t, ok)
					case "pull_requests":
						_, ok := result.([]*github.PullRequest)
						assert.True(t, ok)
					}
				}
			}

			// Assert that all expectations were met
			mockIssuesService.AssertExpectations(t)
			mockRepositoriesService.AssertExpectations(t)
			mockPullRequestsService.AssertExpectations(t)
		})
	}
}

// TestExecuteAction tests the ExecuteAction method
func TestExecuteAction(t *testing.T) {
	// Create test adapter with mocks
	adapter, mockIssuesService, _, mockPullRequestsService := createMockAdapter(t)

	// Common response objects
	successResponse := createResponseWithStatus(200)

	// Define test cases
	testCases := []struct {
		name        string
		contextID   string
		action      string
		params      map[string]interface{}
		setupMocks  func()
		expectError bool
	}{
		{
			name:      "create issue",
			contextID: "test-context",
			action:    "create_issue",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"title": "Test Issue",
				"body":  "This is a test issue",
				"labels": []string{
					"bug", "feature",
				},
			},
			setupMocks: func() {
				issue := &github.Issue{
					Number: github.Int(1),
					Title:  github.String("Test Issue"),
				}
				mockIssuesService.On("Create", mock.Anything, "test-owner", "test-repo", mock.Anything).Return(issue, successResponse, nil)
			},
			expectError: false,
		},
		{
			name:      "close issue",
			contextID: "test-context",
			action:    "close_issue",
			params: map[string]interface{}{
				"owner":        "test-owner",
				"repo":         "test-repo",
				"issue_number": 1,
			},
			setupMocks: func() {
				issue := &github.Issue{
					Number: github.Int(1),
					State:  github.String("closed"),
				}
				mockIssuesService.On("Edit", mock.Anything, "test-owner", "test-repo", 1, mock.Anything).Return(issue, successResponse, nil)
			},
			expectError: false,
		},
		{
			name:      "add comment",
			contextID: "test-context",
			action:    "add_comment",
			params: map[string]interface{}{
				"owner":        "test-owner",
				"repo":         "test-repo",
				"issue_number": 1,
				"body":         "This is a test comment",
			},
			setupMocks: func() {
				comment := &github.IssueComment{
					ID:   github.Int64(1),
					Body: github.String("This is a test comment"),
				}
				mockIssuesService.On("CreateComment", mock.Anything, "test-owner", "test-repo", 1, mock.Anything).Return(comment, successResponse, nil)
			},
			expectError: false,
		},
		{
			name:      "create pull request",
			contextID: "test-context",
			action:    "create_pull_request",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"title": "Test PR",
				"body":  "This is a test PR",
				"head":  "feature-branch",
				"base":  "main",
			},
			setupMocks: func() {
				pr := &github.PullRequest{
					Number: github.Int(1),
					Title:  github.String("Test PR"),
				}
				mockPullRequestsService.On("Create", mock.Anything, "test-owner", "test-repo", mock.Anything).Return(pr, successResponse, nil)
			},
			expectError: false,
		},
		{
			name:      "merge pull request",
			contextID: "test-context",
			action:    "merge_pull_request",
			params: map[string]interface{}{
				"owner":          "test-owner",
				"repo":           "test-repo",
				"pull_number":    1,
				"commit_message": "Merge PR #1",
			},
			setupMocks: func() {
				result := &github.PullRequestMergeResult{
					Merged: github.Bool(true),
				}
				mockPullRequestsService.On("Merge", mock.Anything, "test-owner", "test-repo", 1, "Merge PR #1", mock.Anything).Return(result, successResponse, nil)
			},
			expectError: false,
		},
		{
			name:      "unsupported action",
			contextID: "test-context",
			action:    "unsupported_action",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			setupMocks:  func() {},
			expectError: true,
		},
		{
			name:      "unsafe operation",
			contextID: "test-context",
			action:    "delete_repository",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			setupMocks:  func() {},
			expectError: true,
		},
		{
			name:      "missing required parameter",
			contextID: "test-context",
			action:    "create_issue",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				// Missing title
			},
			setupMocks:  func() {},
			expectError: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			tc.setupMocks()

			// Call ExecuteAction
			result, err := adapter.ExecuteAction(context.Background(), tc.contextID, tc.action, tc.params)

			// Check results
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				// Check type of result based on action
				switch tc.action {
				case "create_issue", "close_issue":
					_, ok := result.(*github.Issue)
					assert.True(t, ok)
				case "add_comment":
					_, ok := result.(*github.IssueComment)
					assert.True(t, ok)
				case "create_pull_request":
					_, ok := result.(*github.PullRequest)
					assert.True(t, ok)
				case "merge_pull_request":
					_, ok := result.(*github.PullRequestMergeResult)
					assert.True(t, ok)
				}
			}

			// Assert that all expectations were met
			mockIssuesService.AssertExpectations(t)
			mockPullRequestsService.AssertExpectations(t)
		})
	}
}

// TestHandleWebhook tests the webhook handling functionality
func TestHandleWebhook(t *testing.T) {
	// Create test adapter
	adapter, _, _, _ := createMockAdapter(t)

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

// TestIsSafeOperation tests the safety checking logic
func TestIsSafeOperation(t *testing.T) {
	// Create test adapter
	adapter, _, _, _ := createMockAdapter(t)

	// Define test cases
	testCases := []struct {
		name      string
		operation string
		params    map[string]interface{}
		expected  bool
	}{
		{
			name:      "safe operation - create issue",
			operation: "create_issue",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"title": "Test Issue",
			},
			expected: true,
		},
		{
			name:      "unsafe operation - delete repository",
			operation: "delete_repository",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			expected: false,
		},
		{
			name:      "unsafe operation - delete branch",
			operation: "delete_branch",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"branch": "main",
			},
			expected: false,
		},
		{
			name:      "conditional unsafe - force merge pull request",
			operation: "merge_pull_request",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"pull_number": 1,
				"force":  true,
			},
			expected: false,
		},
		{
			name:      "conditional safe - regular merge pull request",
			operation: "merge_pull_request",
			params: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"pull_number": 1,
			},
			expected: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call IsSafeOperation
			result, err := adapter.IsSafeOperation(tc.operation, tc.params)

			// Check results
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMapError tests error mapping functionality
func TestMapError(t *testing.T) {
	// Create test adapter
	adapter, _, _, _ := createMockAdapter(t)

	// Test context
	ctx := map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
	}

	// Define test cases
	testCases := []struct {
		name         string
		operation    string
		err          error
		expectedType error
	}{
		{
			name:         "rate limit error",
			operation:    "list_issues",
			err:          &github.RateLimitError{},
			expectedType: &adapterErrors.RateLimitExceededError{},
		},
		{
			name:         "abuse rate limit error",
			operation:    "list_issues",
			err:          &github.AbuseRateLimitError{},
			expectedType: &adapterErrors.RateLimitExceededError{},
		},
		{
			name:      "unauthorized error",
			operation: "list_issues",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 401,
				},
			},
			expectedType: &adapterErrors.UnauthorizedError{},
		},
		{
			name:      "forbidden error",
			operation: "list_issues",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 403,
				},
			},
			expectedType: &adapterErrors.ForbiddenError{},
		},
		{
			name:      "not found error",
			operation: "list_issues",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 404,
				},
			},
			expectedType: &adapterErrors.ResourceNotFoundError{},
		},
		{
			name:      "invalid request error",
			operation: "list_issues",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 422,
				},
			},
			expectedType: &adapterErrors.InvalidRequestError{},
		},
		{
			name:      "too many requests error",
			operation: "list_issues",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 429,
				},
			},
			expectedType: &adapterErrors.TooManyRequestsError{},
		},
		{
			name:      "service unavailable error",
			operation: "list_issues",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 503,
				},
			},
			expectedType: &adapterErrors.ServiceUnavailableError{},
		},
		{
			name:         "unknown error",
			operation:    "list_issues",
			err:          errors.New("unknown error"),
			expectedType: &adapterErrors.UnknownError{},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call mapError
			result := adapter.mapError(tc.operation, tc.err, ctx)

			// Check results
			assert.NotNil(t, result)
			assert.IsType(t, tc.expectedType, result)

			// Check error details
			switch mappedErr := result.(type) {
			case *adapterErrors.AdapterError:
				assert.Equal(t, "github", mappedErr.Provider)
				assert.Equal(t, tc.operation, mappedErr.Operation)
				assert.NotNil(t, mappedErr.OriginalError)
				assert.Equal(t, ctx, mappedErr.Context)
			}
		})
	}
}

// TestClose tests the Close method
func TestClose(t *testing.T) {
	// Create test adapter
	adapter, _, _, _ := createMockAdapter(t)

	// Call Close
	err := adapter.Close()

	// Check results
	assert.NoError(t, err)
}
