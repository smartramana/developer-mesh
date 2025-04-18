// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	
	"github.com/S-Corkum/mcp-server/internal/adapters/providers/github/testdata"
)

// Test constant values
const (
	standaloneTxTimeout = 5 * time.Second
	standaloneOwner     = "standalone-owner"
	standaloneRepo      = "standalone-repo"
	standaloneToken     = "standalone-token"
)

// MockLogger is a simplified logger for integration testing
type MockLogger struct{
	// Collect logs for verification
	InfoLogs  []string
	ErrorLogs []string
	WarnLogs  []string
	DebugLogs []string
}

func (l *MockLogger) Info(msg string, metadata map[string]interface{})  {
	l.InfoLogs = append(l.InfoLogs, msg)
	fmt.Printf("[INFO] %s %v\n", msg, metadata)
}

func (l *MockLogger) Error(msg string, metadata map[string]interface{}) {
	l.ErrorLogs = append(l.ErrorLogs, msg)
	fmt.Printf("[ERROR] %s %v\n", msg, metadata)
}

func (l *MockLogger) Debug(msg string, metadata map[string]interface{}) {
	l.DebugLogs = append(l.DebugLogs, msg)
	fmt.Printf("[DEBUG] %s %v\n", msg, metadata)
}

func (l *MockLogger) Warn(msg string, metadata map[string]interface{})  {
	l.WarnLogs = append(l.WarnLogs, msg)
	fmt.Printf("[WARN] %s %v\n", msg, metadata)
}

// MockMetricsClient is a simplified metrics client for integration testing
type MockMetricsClient struct {
	// Track recorded operations
	Operations []string
}

func (m *MockMetricsClient) RecordContextOperation(operation, modelID string, durationSeconds float64, tokenCount int) {
	m.Operations = append(m.Operations, fmt.Sprintf("context:%s", operation))
}

func (m *MockMetricsClient) RecordVectorOperation(operation string, durationSeconds float64) {
	m.Operations = append(m.Operations, fmt.Sprintf("vector:%s", operation))
}

func (m *MockMetricsClient) RecordToolOperation(tool, action string, durationSeconds float64, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.Operations = append(m.Operations, fmt.Sprintf("tool:%s:%s:%s", tool, action, status))
}

func (m *MockMetricsClient) RecordAPIRequest(endpoint, method, status string, durationSeconds float64) {
	m.Operations = append(m.Operations, fmt.Sprintf("api:%s:%s:%s", endpoint, method, status))
}

func (m *MockMetricsClient) RecordCacheOperation(operation string, hit bool, durationSeconds float64) {
	hitStatus := "miss"
	if hit {
		hitStatus = "hit"
	}
	m.Operations = append(m.Operations, fmt.Sprintf("cache:%s:%s", operation, hitStatus))
}

// MockEventBus is a simplified event bus for integration testing
type MockEventBus struct {
	// Track emitted events
	Events []interface{}
}

func (e *MockEventBus) Emit(ctx context.Context, event interface{}) error {
	e.Events = append(e.Events, event)
	return nil
}

func (e *MockEventBus) EmitWithCallback(ctx context.Context, event interface{}, callback func(error)) error {
	e.Events = append(e.Events, event)
	if callback != nil {
		callback(nil)
	}
	return nil
}

// standaloneIssuesService mocks the GitHub Issues service for standalone tests
type standaloneIssuesService struct {
	mock.Mock
}

func (m *standaloneIssuesService) ListByRepo(ctx context.Context, owner string, repo string, 
	opt *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, opt)
	
	issues, _ := args.Get(0).([]*github.Issue)
	resp, _ := args.Get(1).(*github.Response)
	
	return issues, resp, args.Error(2)
}

func (m *standaloneIssuesService) Create(ctx context.Context, owner string, repo string, 
	issueRequest *github.IssueRequest) (*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, issueRequest)
	
	issue, _ := args.Get(0).(*github.Issue)
	resp, _ := args.Get(1).(*github.Response)
	
	return issue, resp, args.Error(2)
}

// TestGitHubAdapterStandalone performs standalone tests of the GitHub adapter,
// simulating a more realistic usage pattern.
func TestGitHubAdapterStandalone(t *testing.T) {
	// Skip these tests if running short tests
	if testing.Short() {
		t.Skip("Skipping standalone tests in short mode")
	}
	
	// Test valid configuration
	t.Run("valid configuration", func(t *testing.T) {
		// Setup dependencies
		config := Config{
			Token:        standaloneToken,
			Timeout:      10 * time.Second,
			DefaultOwner: standaloneOwner,
			DefaultRepo:  standaloneRepo,
			EnabledFeatures: []string{
				FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
			},
		}

		logger := &MockLogger{}
		metrics := &MockMetricsClient{}
		eventBus := &MockEventBus{}

		// Create adapter
		adapter, err := NewAdapter(config, eventBus, metrics, logger)
		assert.NoError(t, err, "Adapter creation should succeed")
		require.NotNil(t, adapter, "Adapter should not be nil")
		assert.Equal(t, adapterType, adapter.AdapterType, "Adapter type should be correct")
		
		// Verify log messages
		assert.Contains(t, logger.InfoLogs, "GitHub adapter created", 
			"Creation log message should be recorded")
			
		// Check features are mapped correctly
		for _, feature := range config.EnabledFeatures {
			assert.True(t, adapter.featuresEnabled[feature], 
				"Feature %s should be enabled", feature)
		}
		
		// Clean up
		closeErr := adapter.Close()
		assert.NoError(t, closeErr, "Close should succeed")
	})

	// Test invalid configuration
	t.Run("invalid configuration - missing token", func(t *testing.T) {
		// Setup dependencies with invalid config
		config := Config{
			Timeout:      10 * time.Second,
			DefaultOwner: standaloneOwner,
			DefaultRepo:  standaloneRepo,
			EnabledFeatures: []string{
				FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
			},
		}

		logger := &MockLogger{}
		metrics := &MockMetricsClient{}
		eventBus := &MockEventBus{}

		// Create adapter (should fail)
		adapter, err := NewAdapter(config, eventBus, metrics, logger)
		assert.Error(t, err, "Adapter creation should fail with invalid config")
		assert.Nil(t, adapter, "Adapter should be nil on failure")
		assert.Contains(t, err.Error(), "either token or app authentication is required", 
			"Error should mention missing authentication")
	})
	
	// Test initialization with external GitHub token
	t.Run("initialization with environment token", func(t *testing.T) {
		// Skip if no GitHub token is available
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			t.Skip("Skipping GitHub API test - no GITHUB_TOKEN environment variable set")
		}
		
		// Setup dependencies
		config := Config{
			Token:        token,
			Timeout:      10 * time.Second,
			EnabledFeatures: []string{
				FeatureRepositories,
			},
		}

		logger := &MockLogger{}
		metrics := &MockMetricsClient{}
		eventBus := &MockEventBus{}

		// Create and initialize adapter
		adapter, err := NewAdapter(config, eventBus, metrics, logger)
		require.NoError(t, err, "Adapter creation should succeed")
		
		ctx, cancel := context.WithTimeout(context.Background(), standaloneTxTimeout)
		defer cancel()
		
		// Initialize with the same config
		err = adapter.Initialize(ctx, config)
		assert.NoError(t, err, "Initialization should succeed")
		
		// Verify GitHub client was created
		assert.NotNil(t, adapter.client, "GitHub client should be initialized")
		
		// Clean up
		closeErr := adapter.Close()
		assert.NoError(t, closeErr, "Close should succeed")
	})
}

// TestGitHubDataQueryStandalone tests the GitHubDataQuery utility methods
func TestGitHubDataQueryStandalone(t *testing.T) {
	// Test query creation and chaining
	t.Run("query creation", func(t *testing.T) {
		// Create query using constructor
		query := NewGitHubDataQuery(FeatureIssues)
		assert.Equal(t, FeatureIssues, query.ResourceType, "Resource type should be set")
		assert.NotNil(t, query.Filters, "Filters should be initialized")
		assert.Empty(t, query.Owner, "Owner should be empty")
		assert.Empty(t, query.Repo, "Repo should be empty")
		
		// Chain method calls
		modifiedQuery := query.
			WithOwner(standaloneOwner).
			WithRepo(standaloneOwner, standaloneRepo).
			WithFilter("state", "open").
			WithFilter("sort", "updated")
			
		// Verify values
		assert.Equal(t, FeatureIssues, modifiedQuery.ResourceType, "Resource type should be preserved")
		assert.Equal(t, standaloneOwner, modifiedQuery.Owner, "Owner should be set")
		assert.Equal(t, standaloneRepo, modifiedQuery.Repo, "Repo should be set")
		assert.Equal(t, "open", modifiedQuery.Filters["state"], "State filter should be set")
		assert.Equal(t, "updated", modifiedQuery.Filters["sort"], "Sort filter should be set")
		
		// Original query should be unchanged
		assert.Empty(t, query.Owner, "Original query should be unchanged")
	})
}

// TestGetIssuesStandalone tests the getIssues method with a mock client
func TestGetIssuesStandalone(t *testing.T) {
	// Create a mock configuration
	config := Config{
		Token:        standaloneToken,
		Timeout:      10 * time.Second,
		DefaultOwner: standaloneOwner,
		DefaultRepo:  standaloneRepo,
		EnabledFeatures: []string{
			FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
		},
	}

	// Create dependencies
	logger := &MockLogger{}
	metrics := &MockMetricsClient{}
	eventBus := &MockEventBus{}

	// Create the adapter
	adapter, err := NewAdapter(config, eventBus, metrics, logger)
	require.NoError(t, err, "Adapter creation should succeed")

	// Create mock issues service
	mockIssuesService := new(standaloneIssuesService)
	
	// Replace the GitHub client with mocks
	adapter.client = &github.Client{
		Issues: mockIssuesService,
	}

	// Test successful case
	t.Run("successful issue retrieval", func(t *testing.T) {
		// Set up mock response
		issues := []*github.Issue{
			testdata.CreateMockIssue(1, "Test Issue", "This is a test issue"),
		}
		successResponse := &github.Response{
			Response: &http.Response{
				StatusCode: 200,
			},
		}

		// Set up mock expectations
		mockIssuesService.On("ListByRepo", mock.Anything, standaloneOwner, standaloneRepo, mock.Anything).
			Return(issues, successResponse, nil)

		// Create a query
		query := NewGitHubDataQuery(FeatureIssues).
			WithOwner(standaloneOwner).
			WithRepo(standaloneOwner, standaloneRepo).
			WithFilter("state", "open")

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), standaloneTxTimeout)
		defer cancel()
		
		// Call the method
		result, err := adapter.GetData(ctx, query)

		// Check results
		require.NoError(t, err, "GetData should succeed")
		require.NotNil(t, result, "Result should not be nil")

		// Check that we get the expected issues
		retrievedIssues, ok := result.([]*github.Issue)
		require.True(t, ok, "Result should be an issue slice")
		require.Len(t, retrievedIssues, 1, "Should return one issue")
		assert.Equal(t, 1, *retrievedIssues[0].Number, "Issue number should match")
		assert.Equal(t, "Test Issue", *retrievedIssues[0].Title, "Issue title should match")

		// Assert that expectations were met
		mockIssuesService.AssertExpectations(t)
	})

	// Test error case
	t.Run("error case", func(t *testing.T) {
		// Reset mock
		mockIssuesService = new(standaloneIssuesService)
		adapter.client = &github.Client{
			Issues: mockIssuesService,
		}

		// Set up mock expectations to return an error
		mockIssuesService.On("ListByRepo", mock.Anything, standaloneOwner, standaloneRepo, mock.Anything).
			Return([]*github.Issue{}, &github.Response{
				Response: &http.Response{StatusCode: 500},
			}, fmt.Errorf("API error"))

		// Create a query
		query := GitHubDataQuery{
			ResourceType: FeatureIssues,
			Owner:        standaloneOwner,
			Repo:         standaloneRepo,
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), standaloneTxTimeout)
		defer cancel()
		
		// Call the method
		result, err := adapter.GetData(ctx, query)

		// Check results
		assert.Error(t, err, "Should return error for API failure")
		assert.Nil(t, result, "Result should be nil on error")
		assert.Contains(t, err.Error(), "API error", "Error message should contain API error")

		// Assert that expectations were met
		mockIssuesService.AssertExpectations(t)
	})
	
	// Clean up
	closeErr := adapter.Close()
	assert.NoError(t, closeErr, "Close should succeed")
}

// TestIsSafeOperationStandalone tests the safety checking logic independently
func TestIsSafeOperationStandalone(t *testing.T) {
	// Create a test adapter
	config := Config{
		Token:        standaloneToken,
		Timeout:      10 * time.Second,
		DefaultOwner: standaloneOwner,
		DefaultRepo:  standaloneRepo,
		EnabledFeatures: []string{
			FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
		},
	}

	// Create dependencies
	logger := &MockLogger{}
	metrics := &MockMetricsClient{}
	eventBus := &MockEventBus{}

	// Create the adapter
	adapter, err := NewAdapter(config, eventBus, metrics, logger)
	require.NoError(t, err, "Adapter creation should succeed")

	// Test cases
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
				"owner": standaloneOwner,
				"repo":  standaloneRepo,
				"title": "Test Issue",
			},
			expected: true,
		},
		{
			name:      "unsafe operation - delete repository",
			operation: "delete_repository",
			params: map[string]interface{}{
				"owner": standaloneOwner,
				"repo":  standaloneRepo,
			},
			expected: false,
		},
		{
			name:      "unsafe operation - delete branch",
			operation: "delete_branch",
			params: map[string]interface{}{
				"owner":  standaloneOwner,
				"repo":   standaloneRepo,
				"branch": "main",
			},
			expected: false,
		},
		{
			name:      "conditional unsafe - force merge pull request",
			operation: "merge_pull_request",
			params: map[string]interface{}{
				"owner":       standaloneOwner,
				"repo":        standaloneRepo,
				"pull_number": 1,
				"force":       true,
			},
			expected: false,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call IsSafeOperation
			result, err := adapter.IsSafeOperation(tc.operation, tc.params)

			// Check results
			assert.NoError(t, err, "IsSafeOperation should not error")
			assert.Equal(t, tc.expected, result, 
				"Safety result should match expected value for %s", tc.name)
		})
	}
	
	// Clean up
	closeErr := adapter.Close()
	assert.NoError(t, closeErr, "Close should succeed")
}

// TestConfigStandalone tests the configuration methods independently
func TestConfigStandalone(t *testing.T) {
	// Test IsFeatureEnabled
	t.Run("IsFeatureEnabled", func(t *testing.T) {
		config := DefaultConfig()
		config.EnabledFeatures = []string{FeatureIssues, FeatureRepositories}
		
		// Enabled features
		assert.True(t, config.IsFeatureEnabled(FeatureIssues), 
			"Issues feature should be enabled")
		assert.True(t, config.IsFeatureEnabled(FeatureRepositories), 
			"Repositories feature should be enabled")
		
		// Disabled features
		assert.False(t, config.IsFeatureEnabled(FeaturePullRequests), 
			"Pull requests feature should be disabled")
		assert.False(t, config.IsFeatureEnabled(FeatureComments), 
			"Comments feature should be disabled")
	})
	
	// Test GetTimeout
	t.Run("GetTimeout", func(t *testing.T) {
		// With explicit timeout
		config := DefaultConfig()
		config.Timeout = 15 * time.Second
		assert.Equal(t, 15*time.Second, config.GetTimeout(), 
			"Should return configured timeout")
		
		// With zero timeout
		config = DefaultConfig()
		config.Timeout = 0
		assert.Equal(t, DefaultTimeout, config.GetTimeout(), 
			"Should return default timeout for zero")
		
		// With negative timeout
		config = DefaultConfig()
		config.Timeout = -5 * time.Second
		assert.Equal(t, DefaultTimeout, config.GetTimeout(), 
			"Should return default timeout for negative")
	})
	
	// Test Clone
	t.Run("Clone", func(t *testing.T) {
		original := DefaultConfig()
		original.Token = standaloneToken
		original.DefaultOwner = standaloneOwner
		original.DefaultRepo = standaloneRepo
		original.EnabledFeatures = []string{FeatureIssues, FeatureRepositories}
		
		// Clone the config
		clone := original.Clone()
		
		// Verify they're equal
		assert.Equal(t, original, clone, "Clone should equal original")
		
		// Modify the clone
		clone.Token = "modified-token"
		clone.EnabledFeatures = append(clone.EnabledFeatures, FeatureComments)
		
		// Verify original is unchanged
		assert.Equal(t, standaloneToken, original.Token, "Original should be unchanged")
		assert.Len(t, original.EnabledFeatures, 2, "Original features should be unchanged")
		assert.Len(t, clone.EnabledFeatures, 3, "Clone features should be modified")
	})
}
