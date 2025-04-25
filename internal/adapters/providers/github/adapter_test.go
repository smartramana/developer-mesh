// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/mcp-server/internal/adapters/providers/github/mocks"

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

// Helper functions for test setup and utilities

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
				PrivateKey:   "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAuL5wVvF2Dg4Wn4iTjG7zEwIDAQABAoIBAQDKQvFv0v1jI3eP\n-----END RSA PRIVATE KEY-----",
				Timeout:      10 * time.Second,
				DefaultOwner: testOwner,
				DefaultRepo:  testRepo,
				EnabledFeatures: []string{
					FeatureIssues, FeaturePullRequests, FeatureRepositories, FeatureComments,
				},
			},
			expectError:   true,
			errorContains: "failed to parse private key",
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
			errorContains: "either token or app authentication is required",
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
				Token:           testToken,
				Timeout:         10 * time.Second,
				DefaultOwner:    testOwner,
				DefaultRepo:     testRepo,
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
			eventBus := mocks.NewMockEventBus()
			eventBus.On("Publish", mock.Anything, mock.Anything).Return()
			eventBus.On("Subscribe", mock.Anything, mock.Anything).Return()
			eventBus.On("Close").Return()

			metricsClient := observability.NewMetricsClient()
			logger := observability.NewLogger("test-adapter")

			// Create adapter
			adapter, err := NewAdapter(tc.config, eventBus, metricsClient, logger)

			// Check results
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				assert.Nil(t, adapter, "Adapter should be nil")
			} else {
				assert.NoError(t, err, "Expected no error for %s", tc.name)
				assert.NotNil(t, adapter, "Adapter should not be nil")
				assert.Equal(t, "github", adapter.Type(), "Adapter type should be correct")

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
	eventBus := mocks.NewMockEventBus()
	eventBus.On("Publish", mock.Anything, mock.Anything).Return()
	eventBus.On("Subscribe", mock.Anything, mock.Anything).Return()

	metricsClient := observability.NewMetricsClient()

	// Test with nil logger
	t.Run("nil_logger", func(t *testing.T) {
		adapter, err := NewAdapter(config, eventBus, metricsClient, nil)
		assert.Nil(t, adapter, "Adapter should be nil when logger is nil")
		assert.EqualError(t, err, "logger cannot be nil", "Error should be 'logger cannot be nil'")
	})
}

// TestHandleWebhook tests the webhook handling functionality
func TestHandleWebhook(t *testing.T) {
	// Create test adapter
	config := DefaultConfig()
	config.Token = testToken
	config.DefaultOwner = testOwner
	config.DefaultRepo = testRepo
	config.DisableWebhooks = false // Enable webhook handling for this test

	eventBus := mocks.NewMockEventBus()
	eventBus.On("Publish", mock.Anything, mock.Anything).Return()
	eventBus.On("Subscribe", mock.Anything, mock.Anything).Return()

	metricsClient := observability.NewMetricsClient()
	logger := observability.NewLogger("test-adapter")

	// Create adapter
	adapter, err := NewAdapter(config, eventBus, metricsClient, logger)
	require.NoError(t, err, "Failed to create adapter")

	// Define test cases
	testCases := []struct {
		name        string
		eventType   string
		payload     string
		expectError bool
	}{
		{
			name:        "push event",
			eventType:   "push",
			payload:     `{"ref": "refs/heads/main", "repository": {"full_name": "test-owner/test-repo"}}`,
			expectError: false,
		},
		{
			name:        "issue event",
			eventType:   "issues",
			payload:     `{"action": "opened", "issue": {"number": 1, "title": "Test Issue"}, "repository": {"full_name": "test-owner/test-repo"}}`,
			expectError: false,
		},
		{
			name:        "invalid payload",
			eventType:   "invalid",
			payload:     `not a valid payload`,
			expectError: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call HandleWebhook
			err = adapter.HandleWebhook(context.Background(), tc.eventType, []byte(tc.payload))

			// Check results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// Verify expectations were met
	eventBus.AssertExpectations(t)
}

// TestClose tests the Close method
func TestClose(t *testing.T) {
	// Create test adapter
	config := DefaultConfig()
	config.Token = testToken
	config.DefaultOwner = testOwner
	config.DefaultRepo = testRepo

	eventBus := mocks.NewMockEventBus()
	eventBus.On("Close").Return()

	metricsClient := observability.NewMetricsClient()
	logger := observability.NewLogger("test-adapter")

	// Create adapter
	adapter, err := NewAdapter(config, eventBus, metricsClient, logger)
	require.NoError(t, err, "Failed to create adapter")

	// Call Close
	err = adapter.Close()

	// Check results
	assert.NoError(t, err)

	// Verify the event bus Close method was called
	eventBus.AssertExpectations(t)
}

// TestExecuteAction tests the ExecuteAction method for different GitHub operations
func TestExecuteAction(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle different API endpoints based on request path and method
		if r.URL.Path == "/repos/test-owner/test-repo" && r.Method == "GET" {
			// Return repository info
			repo := map[string]interface{}{
				"id":        12345,
				"name":      "test-repo",
				"full_name": "test-owner/test-repo",
				"owner": map[string]interface{}{
					"login": "test-owner",
				},
			}
			json.NewEncoder(w).Encode(repo)
			return
		}

		// Default 404 response
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer server.Close()

	// Create test dependencies
	config := DefaultConfig()
	config.Token = testToken
	config.DefaultOwner = testOwner
	config.DefaultRepo = testRepo
	config.BaseURL = server.URL + "/"
	config.EnabledFeatures = []string{FeatureRepositories}

	eventBus := mocks.NewMockEventBus()
	eventBus.On("Publish", mock.Anything, mock.Anything).Return()
	eventBus.On("Subscribe", mock.Anything, mock.Anything).Return()

	metricsClient := observability.NewMetricsClient()
	logger := observability.NewLogger("test-adapter")

	// Create adapter
	adapter, err := NewAdapter(config, eventBus, metricsClient, logger)
	require.NoError(t, err, "Failed to create adapter")
	defer adapter.Close()

	// Test getRepository action
	t.Run("getRepository", func(t *testing.T) {
		params := map[string]interface{}{
			"owner": testOwner,
			"repo":  testRepo,
		}

		result, err := adapter.ExecuteAction(context.Background(), testContext, "getRepository", params)
		assert.NoError(t, err, "getRepository should not error")
		assert.NotNil(t, result, "Result should not be nil")

		// Result could be different based on your adapter implementation
		// Here we're just checking that it contains some expected fields
		if m, ok := result.(map[string]interface{}); ok {
			assert.Equal(t, testRepo, m["name"], "Repository name should match")
			assert.Equal(t, testOwner+"/"+testRepo, m["full_name"], "Full name should match")
		} else {
			t.Errorf("Expected map[string]interface{}, got %T", result)
		}
	})
}
