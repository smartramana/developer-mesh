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
	eventBus := events.NewEventBus()
	metricsClient := observability.NewMetricsClient()
	logger := mocks.NewLogger() // Use our mock logger

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
			eventBus := events.NewEventBus()
			metricsClient := observability.NewMetricsClient()
			logger := mocks.NewLogger() // Use our mock logger

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
				assert.Equal(t, adapterType, adapter.AdapterType, "Adapter type should be correct")
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
	eventBus := events.NewEventBus()
	metricsClient := observability.NewMetricsClient()
	
	// Test with nil logger
	t.Run("nil logger", func(t *testing.T) {
		adapter, err := NewAdapter(config, eventBus, metricsClient, nil)
		assert.Error(t, err, "Should error with nil logger")
		assert.Nil(t, adapter, "Adapter should be nil")
		assert.Contains(t, err.Error(), "logger cannot be nil", "Error should mention nil logger")
	})
}

// TestInitialize tests the adapter initialization process.
// It verifies that the adapter is properly initialized with different configurations
// and that errors are returned for invalid initialization parameters.
func TestInitialize(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name          string
		config        interface{}
		expectError   bool
		errorContains string
	}{
		{
			name: "valid config",
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
			name:          "invalid config type",
			config:        "not a config",
			expectError:   true,
			errorContains: "invalid configuration type",
		},
		{
			name: "missing authentication",
			config: Config{
				Timeout:      10 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
			},
			expectError:   true,
			errorContains: "no valid authentication method provided",
		},
		{
			name: "GitHub App authentication not implemented",
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
			expectError:   true,
			errorContains: "GitHub App authentication not implemented",
		},
		{
			name: "invalid base URL",
			config: Config{
				Token:        testToken,
				BaseURL:      "http://invalid-url%$&",
				UploadURL:    "http://invalid-url%$&",
				Timeout:      10 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
			},
			expectError:   true,
			errorContains: "failed to create GitHub enterprise client",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create basic adapter for testing with valid initial config
			validConfig := DefaultConfig()
			validConfig.Token = "temp-token"
			validConfig.DefaultOwner = "temp-owner"
			validConfig.DefaultRepo = "temp-repo"
			validConfig.EnabledFeatures = []string{FeatureIssues, FeatureRepositories}

			// Create test dependencies
			eventBus := events.NewEventBus()
			metricsClient := observability.NewMetricsClient()
			logger := mocks.NewLogger() // Use our mock logger

			// Create adapter
			adapter, err := NewAdapter(validConfig, eventBus, metricsClient, logger)
			require.NoError(t, err, "Adapter creation should succeed")

			// Create context with timeout
			ctx, cancel := createTestContext()
			defer cancel()

			// Initialize with the test case config
			err = adapter.Initialize(ctx, tc.config)

			// Check results
			if tc.expectError {
				assert.Error(t, err, "Should return error for %s", tc.name)
				
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains, 
						"Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Should not return error for %s", tc.name)
				assert.NotNil(t, adapter.client, "Client should be initialized")

				// Check that config was updated
				if config, ok := tc.config.(Config); ok {
					assert.Equal(t, config.DefaultOwner, adapter.defaultOwner, 
						"Default owner should be updated")
					assert.Equal(t, config.DefaultRepo, adapter.defaultRepo, 
						"Default repo should be updated")
					
					// Check that features were updated
					for _, feature := range config.EnabledFeatures {
						assert.True(t, adapter.featuresEnabled[feature], 
							"Feature %s should be enabled", feature)
					}
				}
			}
			
			// Clean up
			closeErr := adapter.Close()
			assert.NoError(t, closeErr, "Close should not return error")
		})
	}
}

// TestInitializeWithNilContext tests initialization with nil context
func TestInitializeWithNilContext(t *testing.T) {
	// Create adapter
	adapter, _, _, _ := createMockAdapter(t, nil)
	
	// Create valid config
	config := DefaultConfig()
	config.Token = testToken
	config.DefaultOwner = testOwner
	config.DefaultRepo = testRepo
	
	// Initialize with nil context (should use background context)
	err := adapter.Initialize(nil, config)
	
	// Verify results
	assert.NoError(t, err, "Initialize should succeed with nil context")
	assert.NotNil(t, adapter.client, "Client should be initialized")
	
	// Clean up
	closeErr := adapter.Close()
	assert.NoError(t, closeErr, "Close should not return error")
}

// TestGetData tests the GetData method for retrieving data from GitHub.
// It verifies that different types of queries return the expected results
// and that errors are properly handled and returned.
func TestGetData(t *testing.T) {
	// Create test adapter with mocks
	adapter, mockIssuesService, mockRepositoriesService, mockPullRequestsService := createMockAdapter(t, nil)

	// Common response objects
	successResponse := createResponseWithStatus(200)
	errorResponse := createResponseWithStatus(500)

	// Define test cases
	testCases := []struct {
		name          string
		query         interface{}
		setupMocks    func()
		expectError   bool
		errorContains string
		checkResult   func(t *testing.T, result interface{})
	}{
		{
			name: "valid issues query",
			query: GitHubDataQuery{
				ResourceType: FeatureIssues,
				Owner:        testOwner,
				Repo:         testRepo,
				Filters: map[string]interface{}{
					"state": "open",
				},
			},
			setupMocks: func() {
				issues := []*github.Issue{
					testdata.CreateMockIssue(1, "Test Issue", "This is a test issue"),
				}
				mockIssuesService.On("ListByRepo", mock.Anything, testOwner, testRepo, mock.Anything).
					Return(issues, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				issues, ok := result.([]*github.Issue)
				require.True(t, ok, "Result should be an issue slice")
				require.Len(t, issues, 1, "Should return one issue")
				assert.Equal(t, 1, *issues[0].Number, "Issue number should match")
				assert.Equal(t, "Test Issue", *issues[0].Title, "Issue title should match")
			},
		},
		{
			name: "valid repositories query",
			query: GitHubDataQuery{
				ResourceType: FeatureRepositories,
				Owner:        testOwner,
			},
			setupMocks: func() {
				repos := []*github.Repository{
					testdata.CreateMockRepository(1, testRepo, testOwner),
				}
				mockRepositoriesService.On("List", mock.Anything, testOwner, mock.Anything).
					Return(repos, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				repos, ok := result.([]*github.Repository)
				require.True(t, ok, "Result should be a repository slice")
				require.Len(t, repos, 1, "Should return one repository")
				assert.Equal(t, testRepo, *repos[0].Name, "Repository name should match")
				assert.Equal(t, testOwner, *repos[0].Owner.Login, "Repository owner should match")
			},
		},
		{
			name: "valid pull requests query",
			query: GitHubDataQuery{
				ResourceType: FeaturePullRequests,
				Owner:        testOwner,
				Repo:         testRepo,
				Filters: map[string]interface{}{
					"state": "open",
				},
			},
			setupMocks: func() {
				prs := []*github.PullRequest{
					testdata.CreateMockPullRequest(1, "Test PR", "This is a test PR", "feature", "main"),
				}
				mockPullRequestsService.On("List", mock.Anything, testOwner, testRepo, mock.Anything).
					Return(prs, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				prs, ok := result.([]*github.PullRequest)
				require.True(t, ok, "Result should be a pull request slice")
				require.Len(t, prs, 1, "Should return one pull request")
				assert.Equal(t, 1, *prs[0].Number, "Pull request number should match")
				assert.Equal(t, "Test PR", *prs[0].Title, "Pull request title should match")
			},
		},
		{
			name: "valid query with helper methods",
			query: NewGitHubDataQuery(FeatureRepositories).
				WithOwner(testOwner).
				WithFilter("sort", "updated"),
			setupMocks: func() {
				repos := []*github.Repository{
					testdata.CreateMockRepository(1, testRepo, testOwner),
				}
				mockRepositoriesService.On("List", mock.Anything, testOwner, mock.Anything).
					Return(repos, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				repos, ok := result.([]*github.Repository)
				require.True(t, ok, "Result should be a repository slice")
				require.Len(t, repos, 1, "Should return one repository")
			},
		},
		{
			name:          "invalid query type",
			query:         map[string]string{"not": "valid"},
			setupMocks:    func() {},
			expectError:   true,
			errorContains: "invalid query type for GitHub adapter",
		},
		{
			name: "nil context handled",
			query: GitHubDataQuery{
				ResourceType: FeatureRepositories,
				Owner:        testOwner,
			},
			setupMocks: func() {
				repos := []*github.Repository{
					testdata.CreateMockRepository(1, testRepo, testOwner),
				}
				// Using nil context should be handled properly
				mockRepositoriesService.On("List", mock.Anything, testOwner, mock.Anything).
					Return(repos, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				repos, ok := result.([]*github.Repository)
				require.True(t, ok, "Result should be a repository slice")
				require.Len(t, repos, 1, "Should return one repository")
			},
		},
		{
			name: "unsupported resource type",
			query: GitHubDataQuery{
				ResourceType: "unsupported",
			},
			setupMocks:    func() {},
			expectError:   true,
			errorContains: "unsupported resource type",
		},
		{
			name: "API error",
			query: GitHubDataQuery{
				ResourceType: FeatureIssues,
				Owner:        testOwner,
				Repo:         testRepo,
			},
			setupMocks: func() {
				mockIssuesService.On("ListByRepo", mock.Anything, testOwner, testRepo, mock.Anything).
					Return([]*github.Issue{}, errorResponse, errors.New("API error"))
			},
			expectError:   true,
			errorContains: "API error",
		},
		{
			name: "feature not enabled",
			query: GitHubDataQuery{
				ResourceType: FeatureIssues,
				Owner:        testOwner, 
				Repo:         testRepo,
			},
			setupMocks: func() {
				// Create a new adapter with specific features
				customConfig := DefaultConfig()
				customConfig.Token = testToken
				customConfig.DefaultOwner = testOwner
				customConfig.DefaultRepo = testRepo
				// Only enable repositories (not issues)
				customConfig.EnabledFeatures = []string{FeatureRepositories}
				
				// Replace the adapter with a new one
				newAdapter, _, _, _ := createMockAdapter(t, &customConfig)
				*adapter = *newAdapter
			},
			expectError:   true,
			errorContains: "issues feature is not enabled",
		},
		{
			name: "missing required parameters",
			query: GitHubDataQuery{
				ResourceType: FeatureIssues,
				// Missing owner and repo
			},
			setupMocks: func() {
				// Create a new adapter without default owner/repo
				customConfig := DefaultConfig()
				customConfig.Token = testToken
				// No default owner/repo
				customConfig.EnabledFeatures = []string{FeatureIssues}
				
				// Replace the adapter with a new one
				newAdapter, _, _, _ := createMockAdapter(t, &customConfig)
				*adapter = *newAdapter
			},
			expectError:   true,
			errorContains: "owner and repo are required",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks
			tc.setupMocks()

			// Create context with timeout
			ctx, cancel := createTestContext()
			defer cancel()

			// Call GetData
			result, err := adapter.GetData(ctx, tc.query)

			// Check results
			if tc.expectError {
				assert.Error(t, err, "Should return error for %s", tc.name)
				assert.Nil(t, result, "Result should be nil when error occurs")
				
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains, 
						"Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Should not return error for %s", tc.name)
				assert.NotNil(t, result, "Result should not be nil")

				// Check result type and contents
				if tc.checkResult != nil {
					tc.checkResult(t, result)
				}
			}

			// Assert that all expectations were met
			mockIssuesService.AssertExpectations(t)
			mockRepositoriesService.AssertExpectations(t)
			mockPullRequestsService.AssertExpectations(t)
		})
	}
}

// TestExecuteAction tests the ExecuteAction method for performing operations on GitHub.
// It verifies that different types of actions are executed correctly
// and that errors are properly handled and returned.
func TestExecuteAction(t *testing.T) {
	// Create test adapter with mocks
	adapter, mockIssuesService, _, mockPullRequestsService := createMockAdapter(t, nil)

	// Common response objects
	successResponse := createResponseWithStatus(200)
	errorResponse := createResponseWithStatus(500)

	// Define test cases
	testCases := []struct {
		name          string
		contextID     string
		action        string
		params        map[string]interface{}
		setupMocks    func()
		expectError   bool
		errorContains string
		checkResult   func(t *testing.T, result interface{})
	}{
		{
			name:      "create issue",
			contextID: testContext,
			action:    "create_issue",
			params: map[string]interface{}{
				"owner": testOwner,
				"repo":  testRepo,
				"title": "Test Issue",
				"body":  "This is a test issue",
				"labels": []string{
					"bug", "feature",
				},
			},
			setupMocks: func() {
				issue := testdata.CreateMockIssue(1, "Test Issue", "This is a test issue")
				mockIssuesService.On("Create", mock.Anything, testOwner, testRepo, mock.Anything).
					Return(issue, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				issue, ok := result.(*github.Issue)
				require.True(t, ok, "Result should be an issue")
				assert.Equal(t, 1, *issue.Number, "Issue number should match")
				assert.Equal(t, "Test Issue", *issue.Title, "Issue title should match")
			},
		},
		{
			name:      "close issue",
			contextID: testContext,
			action:    "close_issue",
			params: map[string]interface{}{
				"owner":        testOwner,
				"repo":         testRepo,
				"issue_number": 1,
			},
			setupMocks: func() {
				issue := testdata.CreateMockIssue(1, "Test Issue", "This is a test issue")
				*issue.State = "closed"
				mockIssuesService.On("Edit", mock.Anything, testOwner, testRepo, 1, mock.Anything).
					Return(issue, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				issue, ok := result.(*github.Issue)
				require.True(t, ok, "Result should be an issue")
				assert.Equal(t, "closed", *issue.State, "Issue state should be closed")
			},
		},
		{
			name:      "add comment",
			contextID: testContext,
			action:    "add_comment",
			params: map[string]interface{}{
				"owner":        testOwner,
				"repo":         testRepo,
				"issue_number": 1,
				"body":         "This is a test comment",
			},
			setupMocks: func() {
				comment := testdata.CreateMockIssueComment(1, "This is a test comment")
				mockIssuesService.On("CreateComment", mock.Anything, testOwner, testRepo, 1, mock.Anything).
					Return(comment, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				comment, ok := result.(*github.IssueComment)
				require.True(t, ok, "Result should be a comment")
				assert.Equal(t, "This is a test comment", *comment.Body, "Comment body should match")
			},
		},
		{
			name:      "create pull request",
			contextID: testContext,
			action:    "create_pull_request",
			params: map[string]interface{}{
				"owner": testOwner,
				"repo":  testRepo,
				"title": "Test PR",
				"body":  "This is a test PR",
				"head":  "feature-branch",
				"base":  "main",
			},
			setupMocks: func() {
				pr := testdata.CreateMockPullRequest(1, "Test PR", "This is a test PR", "feature-branch", "main")
				mockPullRequestsService.On("Create", mock.Anything, testOwner, testRepo, mock.Anything).
					Return(pr, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				pr, ok := result.(*github.PullRequest)
				require.True(t, ok, "Result should be a pull request")
				assert.Equal(t, "Test PR", *pr.Title, "PR title should match")
				assert.Equal(t, "feature-branch", *pr.Head.Ref, "PR head should match")
				assert.Equal(t, "main", *pr.Base.Ref, "PR base should match")
			},
		},
		{
			name:      "merge pull request",
			contextID: testContext,
			action:    "merge_pull_request",
			params: map[string]interface{}{
				"owner":          testOwner,
				"repo":           testRepo,
				"pull_number":    1,
				"commit_message": "Merge PR #1",
			},
			setupMocks: func() {
				result := testdata.CreateMockPullRequestMergeResult(true, "Successfully merged")
				mockPullRequestsService.On("Merge", mock.Anything, testOwner, testRepo, 1, "Merge PR #1", mock.Anything).
					Return(result, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				mergeResult, ok := result.(*github.PullRequestMergeResult)
				require.True(t, ok, "Result should be a merge result")
				assert.True(t, *mergeResult.Merged, "PR should be merged")
			},
		},
		{
			name:          "unsupported action",
			contextID:     testContext,
			action:        "unsupported_action",
			params:        map[string]interface{}{"owner": testOwner, "repo": testRepo},
			setupMocks:    func() {},
			expectError:   true,
			errorContains: "unsupported action",
		},
		{
			name:          "unsafe operation",
			contextID:     testContext,
			action:        "delete_repository",
			params:        map[string]interface{}{"owner": testOwner, "repo": testRepo},
			setupMocks:    func() {},
			expectError:   true,
			errorContains: "not safe",
		},
		{
			name:          "missing required parameter",
			contextID:     testContext,
			action:        "create_issue",
			params:        map[string]interface{}{"owner": testOwner, "repo": testRepo}, // Missing title
			setupMocks:    func() {},
			expectError:   true,
			errorContains: "title is required",
		},
		{
			name:      "API error",
			contextID: testContext,
			action:    "create_issue",
			params: map[string]interface{}{
				"owner": testOwner,
				"repo":  testRepo,
				"title": "Test Issue",
			},
			setupMocks: func() {
				mockIssuesService.On("Create", mock.Anything, testOwner, testRepo, mock.Anything).
					Return((*github.Issue)(nil), errorResponse, errors.New("API error"))
			},
			expectError:   true,
			errorContains: "API error",
		},
		{
			name:      "with circuit breaker - success",
			contextID: testContext,
			action:    "create_issue",
			params: map[string]interface{}{
				"owner": testOwner,
				"repo":  testRepo,
				"title": "Test Issue",
			},
			setupMocks: func() {
				// Enable circuit breaker
				circuitBreaker := resilience.NewCircuitBreaker(
					resilience.CircuitBreakerConfig{
						Name:             "test",
						FailureThreshold: 0.5,
						ResetTimeout:     time.Second * 5,
					},
				)
				adapter.CircuitBreaker = circuitBreaker
				
				// Setup mock response
				issue := testdata.CreateMockIssue(1, "Test Issue", "")
				mockIssuesService.On("Create", mock.Anything, testOwner, testRepo, mock.Anything).
					Return(issue, successResponse, nil)
			},
			expectError: false,
			checkResult: func(t *testing.T, result interface{}) {
				issue, ok := result.(*github.Issue)
				require.True(t, ok, "Result should be an issue")
			},
		},
		{
			name:      "with rate limiter - success",
			contextID: testContext,
			action:    "create_issue",
			params: map[string]interface{}{
				"owner": testOwner,
				"repo":  testRepo,
				"title": "Test Issue",
			},
			setupMocks: func() {
				// Remove circuit breaker and add rate limiter
				adapter.CircuitBreaker = nil
				adapter.RateLimiter = resilience.NewRateLimiter(
					resilience.RateLimiterConfig{
						Name:              "test",
						RequestsPerSecond: 10,
					},
				)
				
				// Setup mock response
				issue := testdata.CreateMockIssue(1, "Test Issue", "")
				mockIssuesService.On("Create", mock.Anything, testOwner, testRepo, mock.Anything).
					Return(issue, successResponse, nil)
			},
			expectError: false,
		},
		{
			name:      "float issue number",
			contextID: testContext,
			action:    "close_issue",
			params: map[string]interface{}{
				"owner":        testOwner,
				"repo":         testRepo,
				"issue_number": float64(1),
			},
			setupMocks: func() {
				issue := testdata.CreateMockIssue(1, "Test Issue", "This is a test issue")
				*issue.State = "closed"
				mockIssuesService.On("Edit", mock.Anything, testOwner, testRepo, 1, mock.Anything).
					Return(issue, successResponse, nil)
			},
			expectError: false,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks and dependencies
			tc.setupMocks()

			// Create context with timeout
			ctx, cancel := createTestContext()
			defer cancel()

			// Call ExecuteAction
			result, err := adapter.ExecuteAction(ctx, tc.contextID, tc.action, tc.params)

			// Check results
			if tc.expectError {
				assert.Error(t, err, "Should return error for %s", tc.name)
				assert.Nil(t, result, "Result should be nil when error occurs")
				
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains, 
						"Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Should not return error for %s", tc.name)
				assert.NotNil(t, result, "Result should not be nil")

				// Check result type and contents
				if tc.checkResult != nil {
					tc.checkResult(t, result)
				}
			}

			// Assert that all expectations were met
			mockIssuesService.AssertExpectations(t)
			mockPullRequestsService.AssertExpectations(t)
			
			// Clean up resources
			if adapter.CircuitBreaker != nil {
				adapter.CircuitBreaker.Close()
				adapter.CircuitBreaker = nil
			}
			
			if adapter.RateLimiter != nil {
				adapter.RateLimiter.Close()
				adapter.RateLimiter = nil
			}
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
