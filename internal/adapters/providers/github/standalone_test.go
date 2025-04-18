package github

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLogger is a simplified logger for testing
type MockLogger struct{}

func (l *MockLogger) Info(msg string, metadata map[string]interface{})  {}
func (l *MockLogger) Error(msg string, metadata map[string]interface{}) {}
func (l *MockLogger) Debug(msg string, metadata map[string]interface{}) {}
func (l *MockLogger) Warn(msg string, metadata map[string]interface{})  {}

// MockMetricsClient is a simplified metrics client for testing
type MockMetricsClient struct{}

func (m *MockMetricsClient) RecordContextOperation(operation, modelID string, durationSeconds float64, tokenCount int) {}
func (m *MockMetricsClient) RecordVectorOperation(operation string, durationSeconds float64)                          {}
func (m *MockMetricsClient) RecordToolOperation(tool, action string, durationSeconds float64, err error)              {}
func (m *MockMetricsClient) RecordAPIRequest(endpoint, method, status string, durationSeconds float64)                {}
func (m *MockMetricsClient) RecordCacheOperation(operation string, hit bool, durationSeconds float64)                 {}

// MockEventBus is a simplified event bus for testing
type MockEventBus struct{}

func (e *MockEventBus) Emit(ctx context.Context, event interface{}) error {
	return nil
}

func (e *MockEventBus) EmitWithCallback(ctx context.Context, event interface{}, callback func(error)) error {
	return nil
}

// MockIssuesService mocks the GitHub Issues service
type MockIssuesService struct {
	mock.Mock
}

func (m *MockIssuesService) ListByRepo(ctx context.Context, owner string, repo string, opt *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, opt)
	return args.Get(0).([]*github.Issue), args.Get(1).(*github.Response), args.Error(2)
}

func (m *MockIssuesService) Create(ctx context.Context, owner string, repo string, issueRequest *github.IssueRequest) (*github.Issue, *github.Response, error) {
	args := m.Called(ctx, owner, repo, issueRequest)
	return args.Get(0).(*github.Issue), args.Get(1).(*github.Response), args.Error(2)
}

// TestGitHubAdapter tests GitHub adapter creation with various configurations
func TestGitHubAdapter(t *testing.T) {
	// Test valid configuration
	t.Run("valid configuration", func(t *testing.T) {
		config := Config{
			Token:        "test-token",
			Timeout:      10 * time.Second,
			DefaultOwner: "test-owner",
			DefaultRepo:  "test-repo",
			EnabledFeatures: []string{
				"issues", "pull_requests", "repositories", "comments",
			},
		}

		logger := &MockLogger{}
		metrics := &MockMetricsClient{}
		eventBus := &MockEventBus{}

		adapter, err := NewAdapter(config, eventBus, metrics, logger)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, "github", adapter.AdapterType)
	})

	// Test invalid configuration
	t.Run("invalid configuration - missing token", func(t *testing.T) {
		config := Config{
			Timeout:      10 * time.Second,
			DefaultOwner: "test-owner",
			DefaultRepo:  "test-repo",
			EnabledFeatures: []string{
				"issues", "pull_requests", "repositories", "comments",
			},
		}

		logger := &MockLogger{}
		metrics := &MockMetricsClient{}
		eventBus := &MockEventBus{}

		adapter, err := NewAdapter(config, eventBus, metrics, logger)
		assert.Error(t, err)
		assert.Nil(t, adapter)
	})
}

// TestGetIssues tests the getIssues method
func TestGetIssues(t *testing.T) {
	// Create a mock configuration
	config := Config{
		Token:        "test-token",
		Timeout:      10 * time.Second,
		DefaultOwner: "test-owner",
		DefaultRepo:  "test-repo",
		EnabledFeatures: []string{
			"issues", "pull_requests", "repositories", "comments",
		},
	}

	// Create mock services
	mockIssuesService := new(MockIssuesService)

	// Create mock dependencies
	logger := &MockLogger{}
	metrics := &MockMetricsClient{}
	eventBus := &MockEventBus{}

	// Create the adapter
	adapter, err := NewAdapter(config, eventBus, metrics, logger)
	assert.NoError(t, err)

	// Replace the GitHub client with mocks
	adapter.client = &github.Client{
		Issues: mockIssuesService,
	}

	// Test successful case
	t.Run("successful issue retrieval", func(t *testing.T) {
		// Set up mock response
		issues := []*github.Issue{
			{Number: github.Int(1), Title: github.String("Test Issue")},
		}
		successResponse := &github.Response{
			Response: &http.Response{
				StatusCode: 200,
			},
		}

		// Set up mock expectations
		mockIssuesService.On("ListByRepo", mock.Anything, "test-owner", "test-repo", mock.Anything).Return(issues, successResponse, nil)

		// Create a query
		query := GitHubDataQuery{
			ResourceType: "issues",
			Owner:        "test-owner",
			Repo:         "test-repo",
			Filters: map[string]interface{}{
				"state": "open",
			},
		}

		// Call the method
		result, err := adapter.GetData(context.Background(), query)

		// Check results
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Check that we get the expected issues
		retrievedIssues, ok := result.([]*github.Issue)
		assert.True(t, ok)
		assert.Equal(t, 1, len(retrievedIssues))
		assert.Equal(t, 1, *retrievedIssues[0].Number)
		assert.Equal(t, "Test Issue", *retrievedIssues[0].Title)

		// Assert that expectations were met
		mockIssuesService.AssertExpectations(t)
	})

	// Test error case
	t.Run("error case", func(t *testing.T) {
		// Reset mock
		mockIssuesService = new(MockIssuesService)
		adapter.client = &github.Client{
			Issues: mockIssuesService,
		}

		// Set up mock expectations to return an error
		mockIssuesService.On("ListByRepo", mock.Anything, "test-owner", "test-repo", mock.Anything).Return(
			[]*github.Issue{}, &github.Response{
				Response: &http.Response{StatusCode: 500},
			}, fmt.Errorf("API error"),
		)

		// Create a query
		query := GitHubDataQuery{
			ResourceType: "issues",
			Owner:        "test-owner",
			Repo:         "test-repo",
		}

		// Call the method
		result, err := adapter.GetData(context.Background(), query)

		// Check results
		assert.Error(t, err)
		assert.Nil(t, result)

		// Assert that expectations were met
		mockIssuesService.AssertExpectations(t)
	})
}

// TestIsSafeOperation tests the safety checking logic
func TestIsSafeOperation(t *testing.T) {
	// Create a test adapter
	config := Config{
		Token:        "test-token",
		Timeout:      10 * time.Second,
		DefaultOwner: "test-owner",
		DefaultRepo:  "test-repo",
	}

	logger := &MockLogger{}
	metrics := &MockMetricsClient{}
	eventBus := &MockEventBus{}

	adapter, err := NewAdapter(config, eventBus, metrics, logger)
	assert.NoError(t, err)

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
			name:      "conditional unsafe - force merge pull request",
			operation: "merge_pull_request",
			params: map[string]interface{}{
				"owner":      "test-owner",
				"repo":       "test-repo",
				"pull_number": 1,
				"force":      true,
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
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestValidateConfig tests the configuration validation logic
func TestValidateConfig(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name        string
		config      Config
		expectValid bool
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
			expectValid: true,
		},
		{
			name: "invalid - missing authentication",
			config: Config{
				Timeout:      10 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
				},
			},
			expectValid: false,
		},
		{
			name: "invalid - negative timeout",
			config: Config{
				Token:        "test-token",
				Timeout:      -1 * time.Second,
				DefaultOwner: "test-owner",
				DefaultRepo:  "test-repo",
				EnabledFeatures: []string{
					"issues", "pull_requests", "repositories", "comments",
				},
			},
			expectValid: false,
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate config
			valid, errors := ValidateConfig(tc.config)
			
			// Check result
			if tc.expectValid {
				assert.True(t, valid, "Expected config to be valid, but got errors: %v", errors)
				assert.Empty(t, errors)
			} else {
				assert.False(t, valid, "Expected config to be invalid, but it passed validation")
				assert.NotEmpty(t, errors)
			}
		})
	}
}
