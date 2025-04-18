package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdapter(t *testing.T) {
	// Test with simple API token config (skipping validation that would require a real token)
	t.Run("With API Token", func(t *testing.T) {
		cfg := Config{
			APIToken:       "fake-token",
			RequestTimeout: 5 * time.Second,
			RetryMax:       3,
			RetryDelay:     1 * time.Second,
		}

		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		
		// Verify config was properly set
		assert.Equal(t, cfg.APIToken, adapter.config.APIToken)
		assert.Equal(t, cfg.RequestTimeout, adapter.config.RequestTimeout)
		assert.Equal(t, cfg.RetryMax, adapter.config.RetryMax)
		assert.Equal(t, cfg.RetryDelay, adapter.config.RetryDelay)
	})
	
	// Test with enterprise URL
	t.Run("With Enterprise URL", func(t *testing.T) {
		cfg := Config{
			APIToken:       "fake-token",
			EnterpriseURL:  "https://github.enterprise.com",
			RequestTimeout: 5 * time.Second,
			RetryMax:       3,
			RetryDelay:     1 * time.Second,
		}

		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		
		// Verify enterprise URL was properly set
		assert.Equal(t, cfg.EnterpriseURL, adapter.config.EnterpriseURL)
	})
	
	// Test defaults
	t.Run("With Default Values", func(t *testing.T) {
		cfg := Config{
			APIToken: "fake-token",
		}

		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		
		// Verify defaults were properly set
		assert.Equal(t, defaultRequestTimeout, adapter.config.RequestTimeout)
		assert.Equal(t, defaultRetryMax, adapter.config.RetryMax)
		assert.Equal(t, defaultRetryDelay, adapter.config.RetryDelay)
		assert.Equal(t, defaultAPIVersion, adapter.config.APIVersion)
		assert.Equal(t, minRateLimitRemaining, adapter.config.RateLimitThreshold)
		assert.Equal(t, defaultDefaultPerPage, adapter.config.DefaultPerPage)
		assert.Equal(t, defaultConcurrency, adapter.config.Concurrency)
	})
	
	// Test additional config options
	t.Run("With Additional Config Options", func(t *testing.T) {
		cfg := Config{
			APIToken:             "fake-token",
			APIVersion:           "2022-11-28",
			RateLimitThreshold:   50,
			DefaultPerPage:       50,
			EnableRetryOnRateLimit: true,
			Concurrency:          10,
			LogRequests:          true,
			SafeMode:             true,
		}

		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		
		// Verify additional config options were properly set
		assert.Equal(t, "2022-11-28", adapter.config.APIVersion)
		assert.Equal(t, 50, adapter.config.RateLimitThreshold)
		assert.Equal(t, 50, adapter.config.DefaultPerPage)
		assert.True(t, adapter.config.EnableRetryOnRateLimit)
		assert.Equal(t, 10, adapter.config.Concurrency)
		assert.True(t, adapter.config.LogRequests)
		assert.True(t, adapter.config.SafeMode)
		
		// Verify stats is initialized
		assert.NotNil(t, adapter.stats)
		assert.NotNil(t, adapter.stats.SuccessfulOperations)
		assert.NotNil(t, adapter.stats.FailedOperations)
	})
}

func TestInitialize(t *testing.T) {
	// Set up mock server for testing
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API version header is set
		apiVersion := r.Header.Get("X-GitHub-Api-Version")
		if apiVersion != "2022-11-28" {
			t.Errorf("Expected API Version header 2022-11-28, got %s", apiVersion)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// Check auth header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer fake-token" {
			t.Errorf("Expected Authorization header with token, got %s", authHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		
		// Check Accept header
		acceptHeader := r.Header.Get("Accept")
		if acceptHeader != "application/vnd.github+json" {
			t.Errorf("Expected Accept header application/vnd.github+json, got %s", acceptHeader)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// Check path
		if r.URL.Path == "/rate_limit" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"resources": {"core": {"limit": 5000, "used": 0, "remaining": 5000, "reset": 1727395200}}}`))
			return
		}
		
		// Check health endpoint
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
			return
		}
		
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	t.Run("Initialize with Mock Server", func(t *testing.T) {
		cfg := Config{
			APIToken:       "fake-token",
			RequestTimeout: 5 * time.Second,
			RetryMax:       3,
			RetryDelay:     1 * time.Second,
			MockResponses:  true,
			MockURL:        mockServer.URL,
		}

		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		
		err = adapter.Initialize(context.Background(), nil)
		require.NoError(t, err)
		
		// Check that health status was updated
		assert.Equal(t, "healthy", adapter.Health())
		
		// Check that stats were updated
		assert.Equal(t, int64(1), adapter.stats.RequestsSuccess)
		assert.NotEqual(t, time.Time{}, adapter.stats.LastSuccessfulRequest)
	})
	
	t.Run("Initialize with Enterprise URL", func(t *testing.T) {
		// This test can't actually connect to an enterprise server, but we can test the code path
		cfg := Config{
			APIToken:       "fake-token",
			EnterpriseURL:  "https://github.enterprise.example.com",
			RequestTimeout: 5 * time.Second,
			RetryMax:       3,
			RetryDelay:     1 * time.Second,
		}

		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		
		// We won't call Initialize as it would try to connect to the fake URL
		assert.Equal(t, "https://github.enterprise.example.com", adapter.config.EnterpriseURL)
	})
	
	t.Run("Initialize with Custom Config", func(t *testing.T) {
		// Start with a basic adapter
		adapter, err := NewAdapter(Config{APIToken: "fake-token"})
		require.NoError(t, err)
		
		// Create a new config to pass to Initialize
		newConfig := Config{
			APIToken:                "fake-token",
			RequestTimeout:          10 * time.Second,
			RetryMax:                5,
			RetryDelay:              2 * time.Second,
			MockResponses:           true,
			MockURL:                 mockServer.URL,
			RateLimitThreshold:      20,
			DefaultPerPage:          50,
			EnableRetryOnRateLimit:  true,
			Concurrency:             10,
			LogRequests:             true,
			SafeMode:                true,
		}
		
		// Initialize with the new config
		err = adapter.Initialize(context.Background(), newConfig)
		require.NoError(t, err)
		
		// Verify the config was updated
		assert.Equal(t, "fake-token", adapter.config.APIToken)
		assert.Equal(t, 10*time.Second, adapter.config.RequestTimeout)
		assert.Equal(t, 5, adapter.config.RetryMax)
		assert.Equal(t, 2*time.Second, adapter.config.RetryDelay)
		assert.Equal(t, 20, adapter.config.RateLimitThreshold)
		assert.Equal(t, 50, adapter.config.DefaultPerPage)
		assert.True(t, adapter.config.EnableRetryOnRateLimit)
		assert.Equal(t, 10, adapter.config.Concurrency)
		assert.True(t, adapter.config.LogRequests)
		assert.True(t, adapter.config.SafeMode)
	})
	
	t.Run("Initialize with Invalid Config", func(t *testing.T) {
		adapter, err := NewAdapter(Config{APIToken: "fake-token"})
		require.NoError(t, err)
		
		// Try to initialize with an invalid config type
		err = adapter.Initialize(context.Background(), "not-a-config")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid config type")
	})
	
	t.Run("Initialize with Missing API Token", func(t *testing.T) {
		adapter, err := NewAdapter(Config{APIToken: "fake-token"})
		require.NoError(t, err)
		
		// Try to initialize with a config missing the API token
		err = adapter.Initialize(context.Background(), Config{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GitHub API token is required")
	})
}

func TestUpdateRateLimits(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rate_limit" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			
			// Set the reset time to 1 hour from now
			resetTime := time.Now().Add(1 * time.Hour).Unix()
			
			response := fmt.Sprintf(`{
				"resources": {
					"core": {
						"limit": 5000,
						"used": 1000,
						"remaining": 4000,
						"reset": %d
					},
					"search": {
						"limit": 30,
						"used": 10,
						"remaining": 20,
						"reset": %d
					}
				}
			}`, resetTime, resetTime)
			
			w.Write([]byte(response))
			return
		}
		
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()
	
	t.Run("Update Rate Limits", func(t *testing.T) {
		cfg := Config{
			APIToken:       "fake-token",
			MockResponses:  true,
			MockURL:        mockServer.URL,
		}
		
		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// Update rate limits
		err = adapter.updateRateLimits(context.Background())
		require.NoError(t, err)
		
		// Verify rate limits were updated
		assert.NotNil(t, adapter.rateLimits)
		assert.Equal(t, int64(5000), adapter.rateLimits.Core.Limit)
		assert.Equal(t, int64(4000), adapter.rateLimits.Core.Remaining)
		assert.Equal(t, int64(1000), adapter.rateLimits.Core.Used)
		
		// Test the time check - should not update if called again within a minute
		lastCheck := adapter.lastRateCheck
		err = adapter.updateRateLimits(context.Background())
		require.NoError(t, err)
		assert.Equal(t, lastCheck, adapter.lastRateCheck, "Rate limits should not update if checked within a minute")
		
		// Force update by setting lastRateCheck to zero time
		adapter.lastRateCheck = time.Time{}
		err = adapter.updateRateLimits(context.Background())
		require.NoError(t, err)
		assert.NotEqual(t, time.Time{}, adapter.lastRateCheck)
	})
	
	t.Run("Rate Limit Threshold Warning", func(t *testing.T) {
		// Set up a server that returns a low rate limit
		lowRateLimitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rate_limit" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				
				// Set a very low remaining rate limit
				resetTime := time.Now().Add(1 * time.Hour).Unix()
				
				response := fmt.Sprintf(`{
					"resources": {
						"core": {
							"limit": 5000,
							"used": 4995,
							"remaining": 5,
							"reset": %d
						}
					}
				}`, resetTime)
				
				w.Write([]byte(response))
				return
			}
			
			w.WriteHeader(http.StatusNotFound)
		}))
		defer lowRateLimitServer.Close()
		
		cfg := Config{
			APIToken:           "fake-token",
			MockResponses:      true,
			MockURL:            lowRateLimitServer.URL,
			RateLimitThreshold: 10,
		}
		
		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(lowRateLimitServer.URL + "/")
		
		// Update rate limits
		err = adapter.updateRateLimits(context.Background())
		require.NoError(t, err)
		
		// Verify rate limits were updated
		assert.NotNil(t, adapter.rateLimits)
		assert.Equal(t, int64(5), adapter.rateLimits.Core.Remaining)
		
		// Verify rate limit hit was recorded
		assert.Equal(t, int64(1), adapter.stats.RateLimitHits)
	})
	
	t.Run("Error Handling", func(t *testing.T) {
		// Set up a server that returns an error
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer errorServer.Close()
		
		cfg := Config{
			APIToken:      "fake-token",
			MockResponses: true,
			MockURL:       errorServer.URL,
		}
		
		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(errorServer.URL + "/")
		
		// Update rate limits - should return an error
		err = adapter.updateRateLimits(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch rate limits")
	})
}

func TestGetData(t *testing.T) {
	t.Run("Invalid Query Type", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)

		_, err = adapter.GetData(context.Background(), "not-a-valid-query")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid query type")
	})
	
	t.Run("Missing Operation", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)

		_, err = adapter.GetData(context.Background(), map[string]interface{}{
			"owner": "testorg",
			"repo":  "testrepo",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing operation")
	})
	
	t.Run("Unsupported Operation", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)

		_, err = adapter.GetData(context.Background(), map[string]interface{}{
			"operation": "unknown_operation",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported operation")
	})
	
	t.Run("Rate Limit Check", func(t *testing.T) {
		// Create a mock adapter with a very low rate limit
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		// Manually set rate limits to simulate low remaining
		adapter.rateLimits = &github.RateLimits{
			Core: &github.Rate{
				Limit:     5000,
				Remaining: 5,
				Reset:     github.Timestamp{Time: time.Now().Add(1 * time.Hour)},
			},
		}
		adapter.lastRateCheck = time.Now()
		adapter.config.RateLimitThreshold = 10
		adapter.config.EnableRetryOnRateLimit = false
		
		// Try to get data
		_, err = adapter.GetData(context.Background(), map[string]interface{}{
			"operation": "get_repositories",
		})
		
		// Should fail due to low rate limit
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GitHub API rate limit threshold reached")
	})
	
	t.Run("Stats Tracking", func(t *testing.T) {
		// Set up a mock server that returns a successful response
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rate_limit" {
				// Return a rate limit response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"resources": {"core": {"limit": 5000, "used": 0, "remaining": 5000, "reset": 1727395200}}}`))
				return
			}
			
			if r.URL.Path == "/user/repos" {
				// Return a repositories response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[
					{
						"id": 123,
						"name": "test-repo",
						"full_name": "testuser/test-repo",
						"private": false,
						"html_url": "https://github.com/testuser/test-repo",
						"description": "Test Repository",
						"default_branch": "main"
					}
				]`))
				return
			}
			
			w.WriteHeader(http.StatusNotFound)
		}))
		defer mockServer.Close()
		
		adapter, err := NewAdapter(Config{
			APIToken:      "fake-token",
			MockResponses: true,
			MockURL:       mockServer.URL,
		})
		require.NoError(t, err)
		
		// Create a mock client
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// Call GetData with a supported operation
		_, err = adapter.GetData(context.Background(), map[string]interface{}{
			"operation": "get_repositories",
		})
		
		// No error should occur
		assert.NoError(t, err)
		
		// Check that stats were updated
		assert.Equal(t, int64(1), adapter.stats.RequestsTotal)
		assert.Equal(t, int64(1), adapter.stats.RequestsSuccess)
		assert.Equal(t, int64(1), adapter.stats.SuccessfulOperations["get_repositories"])
		assert.NotEqual(t, time.Time{}, adapter.stats.LastSuccessfulRequest)
	})
}

func TestExecuteAction(t *testing.T) {
	t.Run("Invalid Action", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)

		_, err = adapter.ExecuteAction(context.Background(), "context-123", "unknown_action", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported action")
	})
	
	t.Run("Restricted Operation", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
			SafeMode: true,
		})
		require.NoError(t, err)

		_, err = adapter.ExecuteAction(context.Background(), "context-123", "delete_repository", map[string]interface{}{
			"owner": "testorg",
			"repo":  "testrepo",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})
	
	t.Run("Safety Check Disabled", func(t *testing.T) {
		// Create a mock server that accepts any action
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rate_limit" {
				// Return a rate limit response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"resources": {"core": {"limit": 5000, "used": 0, "remaining": 5000, "reset": 1727395200}}}`))
				return
			}
			
			// Return success for any other request
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer mockServer.Close()
		
		adapter, err := NewAdapter(Config{
			APIToken:      "fake-token",
			MockResponses: true,
			MockURL:       mockServer.URL,
			SafeMode:      false, // Disable safety checks
		})
		require.NoError(t, err)
		
		// Create a mock client
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// With safety disabled, even dangerous operations should be allowed
		_, err = adapter.ExecuteAction(context.Background(), "context-123", "delete_repository", map[string]interface{}{
			"owner": "testorg",
			"repo":  "testrepo",
		})
		
		// No error should occur since safety is disabled
		assert.Error(t, err) // Will still error because the operation is not implemented
		assert.Contains(t, err.Error(), "unsupported action")
		assert.NotContains(t, err.Error(), "not allowed")
	})
	
	t.Run("Rate Limit Check", func(t *testing.T) {
		// Create a mock adapter with a very low rate limit
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		// Manually set rate limits to simulate low remaining
		adapter.rateLimits = &github.RateLimits{
			Core: &github.Rate{
				Limit:     5000,
				Remaining: 5,
				Reset:     github.Timestamp{Time: time.Now().Add(1 * time.Hour)},
			},
		}
		adapter.lastRateCheck = time.Now()
		adapter.config.RateLimitThreshold = 10
		adapter.config.EnableRetryOnRateLimit = false
		
		// Try to execute action
		_, err = adapter.ExecuteAction(context.Background(), "context-123", "create_issue", map[string]interface{}{
			"owner": "testorg",
			"repo":  "testrepo",
			"title": "Test Issue",
		})
		
		// Should fail due to low rate limit
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GitHub API rate limit threshold reached")
	})
	
	t.Run("Context ID in Params", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		// Mock the createIssue method
		originalCreateIssue := adapter.createIssue
		defer func() {
			// Restore the original method
			adapter.createIssue = originalCreateIssue
		}()
		
		called := false
		adapter.createIssue = func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			called = true
			// Check if contextID was added to params
			contextID, ok := params["context_id"].(string)
			assert.True(t, ok, "context_id should be added to params")
			assert.Equal(t, "test-context", contextID)
			return nil, nil
		}
		
		// Execute action with context ID
		_, _ = adapter.ExecuteAction(context.Background(), "test-context", "create_issue", map[string]interface{}{
			"owner": "testorg",
			"repo":  "testrepo",
			"title": "Test Issue",
		})
		
		assert.True(t, called, "createIssue should have been called")
	})
	
	t.Run("Stats Tracking", func(t *testing.T) {
		// Set up a mock server that returns a successful response
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rate_limit" {
				// Return a rate limit response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"resources": {"core": {"limit": 5000, "used": 0, "remaining": 5000, "reset": 1727395200}}}`))
				return
			}
			
			// Return a successful response for other requests
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer mockServer.Close()
		
		adapter, err := NewAdapter(Config{
			APIToken:      "fake-token",
			MockResponses: true,
			MockURL:       mockServer.URL,
		})
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// Mock the createIssue method to return a successful result
		originalCreateIssue := adapter.createIssue
		defer func() {
			adapter.createIssue = originalCreateIssue
		}()
		
		adapter.createIssue = func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"id":     123,
				"number": 42,
				"title":  "Test Issue",
			}, nil
		}
		
		// Execute action
		_, err = adapter.ExecuteAction(context.Background(), "context-123", "create_issue", map[string]interface{}{
			"owner": "testorg",
			"repo":  "testrepo",
			"title": "Test Issue",
		})
		
		// No error should occur
		assert.NoError(t, err)
		
		// Check that stats were updated
		assert.Equal(t, int64(1), adapter.stats.RequestsTotal)
		assert.Equal(t, int64(1), adapter.stats.RequestsSuccess)
		assert.Equal(t, int64(1), adapter.stats.SuccessfulOperations["create_issue"])
		assert.NotZero(t, adapter.stats.AverageResponseTime)
		assert.NotEqual(t, time.Time{}, adapter.stats.LastSuccessfulRequest)
	})
}

func TestHandleWebhook(t *testing.T) {
	adapter, err := NewAdapter(Config{
		APIToken: "fake-token",
	})
	require.NoError(t, err)

	t.Run("Push Event", func(t *testing.T) {
		payload := []byte(`{
			"ref": "refs/heads/main",
			"repository": {
				"id": 12345,
				"name": "testrepo",
				"full_name": "testuser/testrepo",
				"owner": {
					"login": "testuser"
				}
			},
			"sender": {
				"login": "testuser"
			}
		}`)

		// Register a test subscriber
		eventReceived := false
		err = adapter.Subscribe("push", func(event interface{}) {
			eventReceived = true
		})
		require.NoError(t, err)

		// Handle the webhook
		err = adapter.HandleWebhook(context.Background(), "push", payload)
		assert.NoError(t, err)

		// Verify subscriber was notified (giving time for goroutine to execute)
		assert.True(t, eventReceived, "Event handler was not called")
	})

	t.Run("Invalid Payload", func(t *testing.T) {
		payload := []byte(`{invalid json`)
		err = adapter.HandleWebhook(context.Background(), "push", payload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse webhook payload")
	})
	
	t.Run("All Events Subscriber", func(t *testing.T) {
		payload := []byte(`{
			"action": "opened",
			"number": 123,
			"pull_request": {
				"url": "https://api.github.com/repos/testuser/testrepo/pulls/123",
				"id": 12345,
				"number": 123,
				"state": "open",
				"title": "Test PR",
				"user": {
					"login": "testuser"
				}
			},
			"repository": {
				"id": 12345,
				"name": "testrepo",
				"full_name": "testuser/testrepo"
			}
		}`)

		// Register a test subscriber for all events
		allEventsReceived := false
		err = adapter.Subscribe("all", func(event interface{}) {
			allEventsReceived = true
		})
		require.NoError(t, err)

		// Handle the webhook
		err = adapter.HandleWebhook(context.Background(), "pull_request", payload)
		assert.NoError(t, err)

		// Verify all-events subscriber was notified
		assert.True(t, allEventsReceived, "All events handler was not called")
	})
	
	t.Run("Unknown Event Type", func(t *testing.T) {
		payload := []byte(`{
			"key": "value",
			"number": 123
		}`)
		
		eventReceived := false
		err = adapter.Subscribe("custom_event", func(event interface{}) {
			eventReceived = true
		})
		require.NoError(t, err)
		
		// Handle the webhook
		err = adapter.HandleWebhook(context.Background(), "custom_event", payload)
		assert.NoError(t, err)
		
		// Verify subscriber was notified
		assert.True(t, eventReceived, "Custom event handler was not called")
	})
}

func TestSubscribe(t *testing.T) {
	adapter, err := NewAdapter(Config{
		APIToken: "fake-token",
	})
	require.NoError(t, err)

	t.Run("Subscribe to Event", func(t *testing.T) {
		callbackCalled := false
		callback := func(event interface{}) {
			callbackCalled = true
		}

		err = adapter.Subscribe("test-event", callback)
		assert.NoError(t, err)

		// Manually trigger notification by simulating an event
		for _, cb := range adapter.subscribers["test-event"] {
			cb(struct{}{})
		}
		
		assert.True(t, callbackCalled, "Callback was not called")
	})
	
	t.Run("Multiple Subscribers", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		callback1Called := false
		callback2Called := false
		
		callback1 := func(event interface{}) {
			callback1Called = true
		}
		
		callback2 := func(event interface{}) {
			callback2Called = true
		}
		
		err = adapter.Subscribe("multi-event", callback1)
		assert.NoError(t, err)
		
		err = adapter.Subscribe("multi-event", callback2)
		assert.NoError(t, err)
		
		// Verify we have 2 subscribers
		assert.Equal(t, 2, len(adapter.subscribers["multi-event"]))
		
		// Manually trigger notification
		for _, cb := range adapter.subscribers["multi-event"] {
			cb(struct{}{})
		}
		
		assert.True(t, callback1Called, "First callback was not called")
		assert.True(t, callback2Called, "Second callback was not called")
	})
}

func TestIsSafeOperation(t *testing.T) {
	t.Run("Safe Operations", func(t *testing.T) {
		// Test safe operations
		safeOps := []string{
			"create_issue",
			"close_issue",
			"create_pull_request",
			"add_comment",
			"merge_pull_request",
			"get_repository",
			"create_branch",
			"create_webhook",
			"check_workflow_run",
			"trigger_workflow",
			"list_team_members",
			"add_team_member",
		}
		
		for _, op := range safeOps {
			isSafe, err := IsSafeOperation(op)
			assert.NoError(t, err)
			assert.True(t, isSafe, "Operation %s should be safe", op)
		}
	})
	
	t.Run("Unsafe Operations", func(t *testing.T) {
		// Test unsafe operations
		unsafeOps := []string{
			"delete_repository",
			"delete_team",
			"delete_organization",
			"force_push",
			"transfer_repository",
			"delete_branch_protection",
			"update_repository_visibility",
			"set_team_permissions",
			"add_collaborator_admin",
			"set_admin_permissions",
			"modify_security_settings",
			"disable_branch_protection",
			"modify_default_branch",
			"modify_access_token",
			"update_security_policy",
		}
		
		for _, op := range unsafeOps {
			isSafe, err := IsSafeOperation(op)
			assert.Error(t, err)
			assert.False(t, isSafe, "Operation %s should be unsafe", op)
		}
	})
	
	t.Run("Allowed Dangerous Operations", func(t *testing.T) {
		// Test operations that would normally be unsafe but are explicitly allowed
		allowedOps := []string{
			"close_pull_request",
			"delete_webhook",
			"archive_repository",
			"lock_issue",
			"close_issue",
			"remove_team_member",
			"merge_pull_request",
		}
		
		for _, op := range allowedOps {
			isSafe, err := IsSafeOperation(op)
			assert.NoError(t, err)
			assert.True(t, isSafe, "Operation %s should be allowed", op)
		}
	})
	
	t.Run("Allowed Delete Operations", func(t *testing.T) {
		// Test delete operations that are explicitly allowed
		allowedDeleteOps := []string{
			"delete_webhook",
			"delete_comment",
			"delete_label",
			"delete_milestone",
			"delete_project_column",
			"delete_project_card",
		}
		
		for _, op := range allowedDeleteOps {
			isSafe, err := IsSafeOperation(op)
			assert.NoError(t, err)
			assert.True(t, isSafe, "Operation %s should be allowed", op)
		}
	})
	
	t.Run("Operations with Dangerous Prefixes", func(t *testing.T) {
		// Test operations with dangerous prefixes that aren't explicitly allowed
		dangerousOps := []string{
			"delete_unknown_resource",
			"remove_repository_protection",
			"force_delete_branch",
			"update_security_token",
			"modify_access_policy",
			"set_admin_privilege",
			"transfer_ownership",
		}
		
		for _, op := range dangerousOps {
			isSafe, err := IsSafeOperation(op)
			assert.Error(t, err)
			assert.False(t, isSafe, "Operation %s with dangerous prefix should be unsafe", op)
		}
	})
	
	t.Run("Adapter IsSafeOperation", func(t *testing.T) {
		// Test with safe mode enabled
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
			SafeMode: true,
		})
		require.NoError(t, err)
		
		// Safe operation should be allowed
		isSafe, err := adapter.IsSafeOperation("create_issue", nil)
		assert.NoError(t, err)
		assert.True(t, isSafe)
		
		// Unsafe operation should be rejected
		isSafe, err = adapter.IsSafeOperation("delete_repository", nil)
		assert.Error(t, err)
		assert.False(t, isSafe)
		
		// Test with safe mode disabled
		adapter, err = NewAdapter(Config{
			APIToken: "fake-token",
			SafeMode: false,
		})
		require.NoError(t, err)
		
		// All operations should be allowed when safe mode is disabled
		isSafe, err = adapter.IsSafeOperation("delete_repository", nil)
		assert.NoError(t, err)
		assert.True(t, isSafe)
	})
}

func TestHealth(t *testing.T) {
	t.Run("Health Check - Basic", func(t *testing.T) {
		adapter, err := NewAdapter(Config{APIToken: "fake-token"})
		require.NoError(t, err)
		
		// The adapter is initialized with healthStatus = "initializing"
		health := adapter.Health()
		assert.Equal(t, "initializing", health)
		
		// Change the status
		adapter.healthStatus = "healthy"
		health = adapter.Health()
		assert.Equal(t, "healthy", health)
	})
	
	t.Run("Health Check - Rate Limit Critical", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
			RateLimitThreshold: 10,
		})
		require.NoError(t, err)
		
		// Initialize with a healthy status
		adapter.healthStatus = "healthy"
		
		// Set a critically low rate limit
		resetTime := time.Now().Add(1 * time.Hour)
		adapter.rateLimits = &github.RateLimits{
			Core: &github.Rate{
				Limit:     5000,
				Remaining: 5,
				Reset:     github.Timestamp{Time: resetTime},
			},
		}
		
		// Check health
		health := adapter.Health()
		assert.Contains(t, health, "degraded: rate limit critical")
	})
	
	t.Run("Health Check - High Failure Rate", func(t *testing.T) {
		adapter, err := NewAdapter(Config{APIToken: "fake-token"})
		require.NoError(t, err)
		
		// Initialize with a healthy status
		adapter.healthStatus = "healthy"
		
		// Set high failure rate
		adapter.stats.RequestsTotal = 100
		adapter.stats.RequestsFailed = 30  // 30% failure rate
		adapter.stats.LastError = "some error"
		
		// Check health
		health := adapter.Health()
		assert.Contains(t, health, "degraded: high failure rate")
	})
}

func TestGetHealthDetails(t *testing.T) {
	t.Run("Health Details", func(t *testing.T) {
		adapter, err := NewAdapter(Config{APIToken: "fake-token"})
		require.NoError(t, err)
		
		adapter.healthStatus = "healthy"
		adapter.stats.RequestsTotal = 100
		adapter.stats.RequestsSuccess = 90
		adapter.stats.RequestsFailed = 10
		adapter.stats.RequestsRetried = 5
		adapter.stats.RateLimitHits = 2
		adapter.stats.LastError = "test error"
		adapter.stats.LastErrorTime = time.Now()
		adapter.stats.AverageResponseTime = 150 * time.Millisecond
		adapter.stats.SuccessfulOperations["get_repositories"] = 20
		adapter.stats.FailedOperations["get_issues"] = 5
		
		// Set rate limits
		resetTime := time.Now().Add(1 * time.Hour)
		adapter.rateLimits = &github.RateLimits{
			Core: &github.Rate{
				Limit:     5000,
				Remaining: 4000,
				Used:      1000,
				Reset:     github.Timestamp{Time: resetTime},
			},
		}
		adapter.lastRateCheck = time.Now()
		
		// Get health details
		details := adapter.GetHealthDetails()
		
		// Verify details
		assert.Equal(t, "healthy", details["status"])
		
		stats, ok := details["stats"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, int64(100), stats["requests_total"])
		assert.Equal(t, int64(90), stats["requests_success"])
		assert.Equal(t, int64(10), stats["requests_failed"])
		assert.Equal(t, int64(5), stats["requests_retried"])
		assert.Equal(t, int64(2), stats["rate_limit_hits"])
		assert.Equal(t, int64(150), stats["average_response_time_ms"])
		
		successfulOps, ok := stats["successful_operations"].(map[string]int64)
		assert.True(t, ok)
		assert.Equal(t, int64(20), successfulOps["get_repositories"])
		
		failedOps, ok := stats["failed_operations"].(map[string]int64)
		assert.True(t, ok)
		assert.Equal(t, int64(5), failedOps["get_issues"])
		
		rateLimit, ok := details["rate_limit"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, int64(5000), rateLimit["limit"])
		assert.Equal(t, int64(4000), rateLimit["remaining"])
		assert.Equal(t, int64(1000), rateLimit["used"])
		assert.Contains(t, rateLimit["resets_in"], "h0m")
		
		lastError, ok := details["last_error"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "test error", lastError["message"])
	})
}

func TestClose(t *testing.T) {
	adapter, err := NewAdapter(Config{APIToken: "fake-token"})
	require.NoError(t, err)
	
	// Set some stats
	adapter.stats.RequestsTotal = 100
	adapter.stats.RequestsSuccess = 90
	adapter.stats.RequestsFailed = 10
	adapter.stats.RequestsRetried = 5

	err = adapter.Close()
	assert.NoError(t, err)
}

func TestRepositoryOperations(t *testing.T) {
	// Set up a mock server that returns repository data
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check path for different repository operations
		if r.URL.Path == "/user/repos" {
			// Return user repositories
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{
					"id": 123,
					"name": "test-repo-1",
					"full_name": "testuser/test-repo-1",
					"private": false,
					"html_url": "https://github.com/testuser/test-repo-1",
					"description": "Test Repository 1",
					"default_branch": "main"
				},
				{
					"id": 456,
					"name": "test-repo-2",
					"full_name": "testuser/test-repo-2",
					"private": true,
					"html_url": "https://github.com/testuser/test-repo-2",
					"description": "Test Repository 2",
					"default_branch": "main"
				}
			]`))
			return
		}
		
		if r.URL.Path == "/orgs/testorg/repos" {
			// Return organization repositories
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{
					"id": 789,
					"name": "org-repo",
					"full_name": "testorg/org-repo",
					"private": false,
					"html_url": "https://github.com/testorg/org-repo",
					"description": "Organization Repository",
					"default_branch": "main"
				}
			]`))
			return
		}
		
		if r.URL.Path == "/repos/testuser/test-repo" {
			// Return single repository
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"id": 123,
				"name": "test-repo",
				"full_name": "testuser/test-repo",
				"private": false,
				"html_url": "https://github.com/testuser/test-repo",
				"description": "Test Repository",
				"default_branch": "main",
				"forks_count": 10,
				"stargazers_count": 20,
				"watchers_count": 20,
				"open_issues_count": 5,
				"license": {
					"key": "mit",
					"name": "MIT License",
					"url": "https://api.github.com/licenses/mit"
				},
				"owner": {
					"login": "testuser",
					"id": 1234,
					"avatar_url": "https://avatars.githubusercontent.com/u/1234?v=4",
					"html_url": "https://github.com/testuser"
				}
			}`))
			return
		}
		
		if r.URL.Path == "/search/repositories" {
			// Return search results
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"total_count": 2,
				"incomplete_results": false,
				"items": [
					{
						"id": 123,
						"name": "test-repo",
						"full_name": "testuser/test-repo",
						"private": false,
						"html_url": "https://github.com/testuser/test-repo",
						"description": "Test Repository",
						"default_branch": "main"
					},
					{
						"id": 456,
						"name": "another-repo",
						"full_name": "testuser/another-repo",
						"private": false,
						"html_url": "https://github.com/testuser/another-repo",
						"description": "Another Test Repository",
						"default_branch": "main"
					}
				]
			}`))
			return
		}
		
		// Return rate limit for other requests
		if r.URL.Path == "/rate_limit" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"resources": {"core": {"limit": 5000, "used": 0, "remaining": 5000, "reset": 1727395200}}}`))
			return
		}
		
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()
	
	t.Run("Get User Repositories", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// Get repositories for authenticated user
		result, err := adapter.getRepositories(context.Background(), map[string]interface{}{})
		
		// Verify result
		require.NoError(t, err)
		require.NotNil(t, result)
		
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		
		repositories, ok := resultMap["repositories"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 2, len(repositories))
		
		// Check first repository
		assert.Equal(t, "test-repo-1", repositories[0]["name"])
		assert.Equal(t, "testuser/test-repo-1", repositories[0]["full_name"])
		assert.Equal(t, false, repositories[0]["private"])
	})
	
	t.Run("Get Organization Repositories", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// Get repositories for organization
		result, err := adapter.getRepositories(context.Background(), map[string]interface{}{
			"owner": "testorg",
		})
		
		// Verify result
		require.NoError(t, err)
		require.NotNil(t, result)
		
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		
		repositories, ok := resultMap["repositories"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 1, len(repositories))
		
		// Check repository
		assert.Equal(t, "org-repo", repositories[0]["name"])
		assert.Equal(t, "testorg/org-repo", repositories[0]["full_name"])
	})
	
	t.Run("Get Single Repository", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// Get single repository
		result, err := adapter.getRepository(context.Background(), map[string]interface{}{
			"owner": "testuser",
			"repo":  "test-repo",
		})
		
		// Verify result
		require.NoError(t, err)
		require.NotNil(t, result)
		
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		
		// Check repository details
		assert.Equal(t, "test-repo", resultMap["name"])
		assert.Equal(t, "testuser/test-repo", resultMap["full_name"])
		assert.Equal(t, false, resultMap["private"])
		assert.Equal(t, "Test Repository", resultMap["description"])
		assert.Equal(t, "main", resultMap["default_branch"])
		assert.Equal(t, float64(10), resultMap["forks_count"])
		assert.Equal(t, float64(20), resultMap["stargazers_count"])
		
		// Check owner details
		owner, ok := resultMap["owner"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "testuser", owner["login"])
	})
	
	t.Run("Search Repositories", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// Search repositories
		result, err := adapter.searchRepositories(context.Background(), map[string]interface{}{
			"query": "test-repo",
		})
		
		// Verify result
		require.NoError(t, err)
		require.NotNil(t, result)
		
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		
		// Check search results
		assert.Equal(t, float64(2), resultMap["total_count"])
		assert.Equal(t, "test-repo", resultMap["query"])
		
		items, ok := resultMap["items"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 2, len(items))
		
		// Check first repository
		assert.Equal(t, "test-repo", items[0]["name"])
		assert.Equal(t, "testuser/test-repo", items[0]["full_name"])
	})
	
	t.Run("Repository Pagination", func(t *testing.T) {
		adapter, err := NewAdapter(Config{
			APIToken: "fake-token",
		})
		require.NoError(t, err)
		
		// Mock the client creation
		adapter.client = github.NewClient(nil)
		adapter.client.BaseURL, _ = url.Parse(mockServer.URL + "/")
		
		// Mock response to include pagination headers
		origClient := adapter.client
		adapter.client = &github.Client{
			BaseURL: origClient.BaseURL,
			Repositories: &mockRepositoriesService{
				client: origClient,
				withPagination: true,
			},
			Search: origClient.Search,
		}
		
		// Get repositories with pagination
		result, err := adapter.getRepositories(context.Background(), map[string]interface{}{
			"page":     float64(1),
			"per_page": float64(10),
		})
		
		// Verify result
		require.NoError(t, err)
		require.NotNil(t, result)
		
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		
		// Check pagination info
		assert.Equal(t, 1, resultMap["page"])
		assert.Equal(t, 10, resultMap["per_page"])
		assert.Equal(t, true, resultMap["has_next"])
		assert.Equal(t, 2, resultMap["next_page"])
	})
}

// Mock implementation of RepositoriesService for testing pagination
type mockRepositoriesService struct {
	client        *github.Client
	withPagination bool
}

func (m *mockRepositoriesService) List(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
	// Call the original client to get the response
	repos, resp, err := m.client.Repositories.List(ctx, user, opts)
	
	// Add pagination info if needed
	if m.withPagination {
		resp.NextPage = 2
		resp.PrevPage = 0
		resp.FirstPage = 1
		resp.LastPage = 3
	}
	
	return repos, resp, err
}

func (m *mockRepositoriesService) ListByOrg(ctx context.Context, org string, opts *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error) {
	// Call the original client to get the response
	repos, resp, err := m.client.Repositories.ListByOrg(ctx, org, opts)
	
	// Add pagination info if needed
	if m.withPagination {
		resp.NextPage = 2
		resp.PrevPage = 0
		resp.FirstPage = 1
		resp.LastPage = 3
	}
	
	return repos, resp, err
}

// Mock other methods of RepositoriesService that might be called
func (m *mockRepositoriesService) Get(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
	return m.client.Repositories.Get(ctx, owner, repo)
}

// Helper function to create a mock response for testing
func createMockResponse(status int, headers map[string]string, body string) *http.Response {
	header := http.Header{}
	for k, v := range headers {
		header.Set(k, v)
	}
	
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// Helper function to create a general response with pagination
func createPaginatedResponse(t *testing.T, page int, perPage int, items []interface{}, totalCount int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check query parameters for pagination
		queryPage := r.URL.Query().Get("page")
		queryPerPage := r.URL.Query().Get("per_page")
		
		// If pagination params provided, make sure they match expected values
		if queryPage != "" {
			pageNum, err := strconv.Atoi(queryPage)
			require.NoError(t, err)
			assert.Equal(t, page, pageNum)
		}
		
		if queryPerPage != "" {
			perPageNum, err := strconv.Atoi(queryPerPage)
			require.NoError(t, err)
			assert.Equal(t, perPage, perPageNum)
		}
		
		// Create response
		response := map[string]interface{}{
			"total_count": totalCount,
			"items":       items,
		}
		
		// Convert to JSON
		data, err := json.Marshal(response)
		require.NoError(t, err)
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

// Helper function to create a check run event
func (a *Adapter) parseCheckRunEvent(payload []byte) (interface{}, error) {
	var event github.CheckRunEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

// Helper function to create a check suite event
func (a *Adapter) parseCheckSuiteEvent(payload []byte) (interface{}, error) {
	var event github.CheckSuiteEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}
