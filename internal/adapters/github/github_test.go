package github

import (
	"context"
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
		
		// Skip validation that would fail without a real token
		t.Skip("Skipping actual GitHub client validation")
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
		
		// Skip validation that would fail without a real token
		t.Skip("Skipping actual GitHub client validation")
	})
}

func TestInitialize(t *testing.T) {
	// Skip actual initialization which would require a real GitHub token
	t.Skip("Skipping test that requires GitHub token")
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
	
	t.Run("Valid Query Type", func(t *testing.T) {
		// Skip test that would require a real GitHub API call
		t.Skip("Skipping test that requires GitHub API")
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
		time.Sleep(100 * time.Millisecond)
		assert.True(t, eventReceived)
	})

	t.Run("Invalid Payload", func(t *testing.T) {
		payload := []byte(`{invalid json`)
		err = adapter.HandleWebhook(context.Background(), "push", payload)
		assert.Error(t, err)
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
		
		assert.True(t, callbackCalled)
	})
}

func TestExecuteAction(t *testing.T) {
	t.Run("Execute Create Issue", func(t *testing.T) {
		// Skip actual API call
		t.Skip("Skipping test that requires GitHub API")
	})
}

func TestIsSafeOperation(t *testing.T) {
	adapter, err := NewAdapter(Config{
		APIToken: "fake-token",
	})
	require.NoError(t, err)
	
	t.Run("Safe Operations", func(t *testing.T) {
		// Test safe operations
		safeOps := []string{
			"create_issue",
			"close_issue",
			"create_pull_request",
			"add_comment",
		}
		
		for _, op := range safeOps {
			isSafe, err := adapter.IsSafeOperation(op, nil)
			assert.NoError(t, err)
			assert.True(t, isSafe, "Operation %s should be safe", op)
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
	})
}

func TestClose(t *testing.T) {
	adapter, err := NewAdapter(Config{APIToken: "fake-token"})
	require.NoError(t, err)

	err = adapter.Close()
	assert.NoError(t, err)
}
