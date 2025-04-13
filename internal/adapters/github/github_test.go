package github

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdapter(t *testing.T) {
	// Test with mock responses enabled
	t.Run("With Mock Responses", func(t *testing.T) {
		cfg := Config{
			MockResponses:    true,
			MockURL:          "http://localhost:8081",
			RequestTimeout:   5 * time.Second,
			MaxRetries:       3,
			RetryDelay:       1 * time.Second,
			RateLimitPerHour: 5000,
		}

		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		assert.NotNil(t, adapter.client)
		assert.NotNil(t, adapter.httpClient)
		assert.NotNil(t, adapter.subscribers)
		assert.Equal(t, cfg, adapter.config)
	})

	// Test with regular API access (token-based)
	t.Run("With API Token", func(t *testing.T) {
		cfg := Config{
			APIToken:         "fake-token",
			RequestTimeout:   5 * time.Second,
			MaxRetries:       3,
			RetryDelay:       1 * time.Second,
			RateLimitPerHour: 5000,
		}

		adapter, err := NewAdapter(cfg)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		assert.NotNil(t, adapter.client)
		assert.NotNil(t, adapter.httpClient)
		assert.NotNil(t, adapter.subscribers)
		assert.Equal(t, cfg, adapter.config)
	})
}

func TestInitialize(t *testing.T) {
	adapter, err := NewAdapter(Config{MockResponses: true})
	require.NoError(t, err)

	err = adapter.Initialize(context.Background(), nil)
	assert.NoError(t, err)
}

func TestGetMockData(t *testing.T) {
	adapter, err := NewAdapter(Config{
		MockResponses: true,
		MockURL:       "http://localhost:8081",
	})
	require.NoError(t, err)

	t.Run("Repository Query", func(t *testing.T) {
		query := models.GitHubQuery{
			Type:  models.GitHubQueryTypeRepository,
			Owner: "testuser",
			Repo:  "testrepo",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Convert the result to a generic map to check fields
		var repoMap map[string]interface{}
		repoData, err := json.Marshal(result)
		require.NoError(t, err)
		err = json.Unmarshal(repoData, &repoMap)
		require.NoError(t, err)
		
		// Check expected fields
		assert.Equal(t, "testrepo", repoMap["name"])
		assert.Equal(t, "testuser", repoMap["owner_login"])
		assert.Equal(t, "testuser/testrepo", repoMap["full_name"])
	})

	t.Run("Pull Requests Query", func(t *testing.T) {
		query := models.GitHubQuery{
			Type:  models.GitHubQueryTypePullRequests,
			Owner: "testuser",
			Repo:  "testrepo",
			State: "open",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Convert the result to a slice of maps to check fields
		var prsSlice []map[string]interface{}
		prsData, err := json.Marshal(result)
		require.NoError(t, err)
		err = json.Unmarshal(prsData, &prsSlice)
		require.NoError(t, err)
		
		// Verify it's a slice with 2 items
		assert.Equal(t, 2, len(prsSlice))
		
		// Check the first PR
		pr1 := prsSlice[0]
		assert.Equal(t, "open", pr1["state"])
		
		// Check base repo name
		base1, ok := pr1["base"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "testrepo", base1["repo_name"])
	})

	t.Run("Invalid Query Type", func(t *testing.T) {
		query := models.GitHubQuery{
			Type:  "invalid-type",
			Owner: "testuser",
			Repo:  "testrepo",
		}

		result, err := adapter.GetData(context.Background(), query)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unsupported query type")
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
}

func TestHandleWebhook(t *testing.T) {
	adapter, err := NewAdapter(Config{MockResponses: true})
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

	t.Run("Pull Request Event", func(t *testing.T) {
		payload := []byte(`{
			"action": "opened",
			"number": 101,
			"pull_request": {
				"id": 1001,
				"number": 101,
				"title": "Test PR",
				"state": "open",
				"user": {
					"login": "testuser"
				},
				"body": "This is a test PR",
				"base": {
					"ref": "main",
					"repo": {
						"name": "testrepo"
					}
				},
				"head": {
					"ref": "feature-branch",
					"repo": {
						"name": "testrepo"
					}
				}
			},
			"repository": {
				"id": 12345,
				"name": "testrepo",
				"full_name": "testuser/testrepo"
			}
		}`)

		// Register a test subscriber
		eventReceived := false
		err = adapter.Subscribe("pull_request", func(event interface{}) {
			eventReceived = true
		})
		require.NoError(t, err)

		// Handle the webhook
		err = adapter.HandleWebhook(context.Background(), "pull_request", payload)
		assert.NoError(t, err)

		// Verify subscriber was notified
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
	adapter, err := NewAdapter(Config{MockResponses: true})
	require.NoError(t, err)

	t.Run("Subscribe to Event", func(t *testing.T) {
		callbackCalled := false
		callback := func(event interface{}) {
			callbackCalled = true
		}

		err = adapter.Subscribe("test-event", callback)
		assert.NoError(t, err)

		// Verify the callback was registered
		adapter.subscriberMu.RLock()
		subscribers := adapter.subscribers["test-event"]
		adapter.subscriberMu.RUnlock()
		
		assert.Len(t, subscribers, 1)

		// Manually trigger notification
		adapter.notifySubscribers("test-event", struct{}{})
		time.Sleep(100 * time.Millisecond)
		assert.True(t, callbackCalled)
	})

	t.Run("Multiple Subscribers", func(t *testing.T) {
		callCount := 0
		callback1 := func(event interface{}) {
			callCount++
		}
		callback2 := func(event interface{}) {
			callCount++
		}

		err = adapter.Subscribe("multi-event", callback1)
		assert.NoError(t, err)
		err = adapter.Subscribe("multi-event", callback2)
		assert.NoError(t, err)

		// Verify the callbacks were registered
		adapter.subscriberMu.RLock()
		subscribers := adapter.subscribers["multi-event"]
		adapter.subscriberMu.RUnlock()
		
		assert.Len(t, subscribers, 2)

		// Manually trigger notification
		adapter.notifySubscribers("multi-event", struct{}{})
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, 2, callCount)
	})
}

func TestHealth(t *testing.T) {
	t.Run("Mock Mode Health", func(t *testing.T) {
		adapter, err := NewAdapter(Config{MockResponses: true})
		require.NoError(t, err)

		health := adapter.Health()
		assert.Equal(t, "healthy (mock)", health)
	})
}

func TestClose(t *testing.T) {
	adapter, err := NewAdapter(Config{MockResponses: true})
	require.NoError(t, err)

	err = adapter.Close()
	assert.NoError(t, err)
}
