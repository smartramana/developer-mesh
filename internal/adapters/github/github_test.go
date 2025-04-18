package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
		assert.Equal(t, 30*time.Second, adapter.config.RequestTimeout)
		assert.Equal(t, 3, adapter.config.RetryMax)
		assert.Equal(t, 1*time.Second, adapter.config.RetryDelay)
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
		
		// Check path
		if r.URL.Path == "/rate_limit" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"resources": {"core": {"limit": 5000, "used": 0, "remaining": 5000, "reset": 1601413472}}}`))
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
		})
		require.NoError(t, err)

		_, err = adapter.ExecuteAction(context.Background(), "context-123", "delete_repository", map[string]interface{}{
			"owner": "testorg",
			"repo":  "testrepo",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
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
		}
		
		for _, op := range allowedOps {
			isSafe, err := IsSafeOperation(op)
			assert.NoError(t, err)
			assert.True(t, isSafe, "Operation %s should be allowed", op)
		}
	})
}

func TestHealth(t *testing.T) {
	t.Run("Health Check", func(t *testing.T) {
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
}

func TestClose(t *testing.T) {
	adapter, err := NewAdapter(Config{APIToken: "fake-token"})
	require.NoError(t, err)

	err = adapter.Close()
	assert.NoError(t, err)
}

func TestParseEvents(t *testing.T) {
	adapter, err := NewAdapter(Config{APIToken: "fake-token"})
	require.NoError(t, err)
	
	t.Run("Parse Pull Request Event", func(t *testing.T) {
		payload := []byte(`{
			"action": "opened",
			"number": 123,
			"pull_request": {
				"url": "https://api.github.com/repos/testuser/testrepo/pulls/123",
				"id": 12345,
				"number": 123,
				"state": "open",
				"title": "Test PR"
			}
		}`)
		
		event, err := adapter.parsePullRequestEvent(payload)
		assert.NoError(t, err)
		assert.NotNil(t, event)
	})
	
	t.Run("Parse Push Event", func(t *testing.T) {
		payload := []byte(`{
			"ref": "refs/heads/main",
			"before": "abc123",
			"after": "def456",
			"repository": {
				"id": 12345,
				"name": "testrepo",
				"full_name": "testuser/testrepo"
			}
		}`)
		
		event, err := adapter.parsePushEvent(payload)
		assert.NoError(t, err)
		assert.NotNil(t, event)
	})
	
	t.Run("Parse Issues Event", func(t *testing.T) {
		payload := []byte(`{
			"action": "opened",
			"issue": {
				"url": "https://api.github.com/repos/testuser/testrepo/issues/123",
				"id": 12345,
				"number": 123,
				"title": "Test Issue",
				"state": "open"
			},
			"repository": {
				"id": 12345,
				"name": "testrepo",
				"full_name": "testuser/testrepo"
			}
		}`)
		
		event, err := adapter.parseIssuesEvent(payload)
		assert.NoError(t, err)
		assert.NotNil(t, event)
	})
}

// Test the additional webhook event parsing functions
func TestParseAdditionalEvents(t *testing.T) {
	adapter, err := NewAdapter(Config{APIToken: "fake-token"})
	require.NoError(t, err)
	
	t.Run("Parse Workflow Run Event", func(t *testing.T) {
		payload := []byte(`{
			"action": "completed",
			"workflow_run": {
				"id": 12345,
				"name": "CI",
				"workflow_id": 54321,
				"status": "completed",
				"conclusion": "success"
			},
			"repository": {
				"id": 12345,
				"name": "testrepo",
				"full_name": "testuser/testrepo"
			}
		}`)
		
		event, err := adapter.parseWorkflowRunEvent(payload)
		assert.NoError(t, err)
		assert.NotNil(t, event)
	})
	
	t.Run("Parse Check Run Event", func(t *testing.T) {
		payload := []byte(`{
			"action": "completed",
			"check_run": {
				"id": 12345,
				"name": "build",
				"status": "completed",
				"conclusion": "success"
			},
			"repository": {
				"id": 12345,
				"name": "testrepo",
				"full_name": "testuser/testrepo"
			}
		}`)
		
		event, err := adapter.parseCheckRunEvent(payload)
		assert.NoError(t, err)
		assert.NotNil(t, event)
	})
}

// Test for headerTransport
func TestHeaderTransport(t *testing.T) {
	t.Run("Add Headers to Request", func(t *testing.T) {
		transport := &headerTransport{
			base:       http.DefaultTransport,
			token:      "test-token",
			apiVersion: "2022-11-28",
		}
		
		req, err := http.NewRequest("GET", "https://example.com", nil)
		require.NoError(t, err)
		
		// Create a mock round tripper to capture the request
		mockRoundTripper := &mockRoundTripper{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
			},
		}
		transport.base = mockRoundTripper
		
		// Perform the request
		_, err = transport.RoundTrip(req)
		require.NoError(t, err)
		
		// Verify headers were set
		assert.Equal(t, "Bearer test-token", mockRoundTripper.request.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github+json", mockRoundTripper.request.Header.Get("Accept"))
		assert.Equal(t, "2022-11-28", mockRoundTripper.request.Header.Get("X-GitHub-Api-Version"))
	})
}

// Mock round tripper for testing
type mockRoundTripper struct {
	request  *http.Request
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.request = req
	return m.response, m.err
}

// Helper function to create a generic repo response for testing
func createMockRepositoryResponse() []byte {
	repo := map[string]interface{}{
		"id":             123,
		"name":           "testrepo",
		"full_name":      "testuser/testrepo",
		"html_url":       "https://github.com/testuser/testrepo",
		"description":    "Test repository",
		"default_branch": "main",
		"created_at":     "2022-01-01T00:00:00Z",
		"updated_at":     "2022-01-02T00:00:00Z",
		"pushed_at":      "2022-01-03T00:00:00Z",
		"language":       "Go",
		"private":        false,
		"fork":           false,
		"forks_count":    5,
		"stargazers_count": 10,
		"watchers_count": 3,
		"open_issues_count": 2,
	}
	data, _ := json.Marshal(repo)
	return data
}
