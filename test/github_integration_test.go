package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/adapters/github"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubAdapter_ExecuteAction(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle different API endpoints
		switch r.URL.Path {
		case "/repos/octocat/hello-world":
			// Return repository data
			repo := map[string]interface{}{
				"id":       123456,
				"name":     "hello-world",
				"full_name": "octocat/hello-world",
				"owner": map[string]interface{}{
					"login": "octocat",
					"id":    1,
				},
				"html_url": "https://github.com/octocat/hello-world",
				"private": false,
			}
			json.NewEncoder(w).Encode(repo)
			
		case "/repos/octocat/hello-world/issues":
			if r.Method == "GET" {
				// Return issues list
				issues := []map[string]interface{}{
					{
						"id":     1,
						"number": 1,
						"title":  "Test issue 1",
						"state":  "open",
						"user": map[string]interface{}{
							"login": "octocat",
							"id":    1,
						},
					},
					{
						"id":     2,
						"number": 2,
						"title":  "Test issue 2",
						"state":  "closed",
						"user": map[string]interface{}{
							"login": "octocat",
							"id":    1,
						},
					},
				}
				json.NewEncoder(w).Encode(issues)
			} else if r.Method == "POST" {
				// Handle issue creation
				var issue map[string]interface{}
				json.NewDecoder(r.Body).Decode(&issue)
				
				response := map[string]interface{}{
					"id":     3,
					"number": 3,
					"title":  issue["title"],
					"body":   issue["body"],
					"state":  "open",
					"user": map[string]interface{}{
						"login": "octocat",
						"id":    1,
					},
				}
				json.NewEncoder(w).Encode(response)
			}
			
		case "/repos/octocat/hello-world/pulls":
			// Return pull requests list
			prs := []map[string]interface{}{
				{
					"id":     1,
					"number": 1,
					"title":  "Test PR 1",
					"state":  "open",
					"user": map[string]interface{}{
						"login": "octocat",
						"id":    1,
					},
				},
				{
					"id":     2,
					"number": 2,
					"title":  "Test PR 2",
					"state":  "closed",
					"user": map[string]interface{}{
						"login": "octocat",
						"id":    1,
					},
				},
			}
			json.NewEncoder(w).Encode(prs)
			
		default:
			// Return 404 for unknown endpoints
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	
	// Create test logger and metrics client
	logger := observability.NewLogger("test", "debug")
	metricsClient := observability.NewMetricsClient("test")
	
	// Create event bus
	eventBus := events.NewEventBus()
	
	// Create GitHub adapter config
	config := github.DefaultConfig()
	config.BaseURL = server.URL + "/"
	config.UploadURL = server.URL + "/"
	config.GraphQLURL = server.URL + "/graphql"
	config.Token = "test-token"
	
	// Create adapter
	adapter, err := github.New(config, logger, metricsClient, eventBus)
	require.NoError(t, err)
	defer adapter.Close()
	
	// Test getRepository
	t.Run("GetRepository", func(t *testing.T) {
		params := map[string]interface{}{
			"owner": "octocat",
			"repo":  "hello-world",
		}
		
		result, err := adapter.ExecuteAction(context.Background(), "test-context", "getRepository", params)
		require.NoError(t, err)
		
		repo := result.(map[string]interface{})
		assert.Equal(t, "hello-world", repo["name"])
		assert.Equal(t, "octocat/hello-world", repo["full_name"])
	})
	
	// Test listIssues
	t.Run("ListIssues", func(t *testing.T) {
		params := map[string]interface{}{
			"owner": "octocat",
			"repo":  "hello-world",
		}
		
		result, err := adapter.ExecuteAction(context.Background(), "test-context", "listIssues", params)
		require.NoError(t, err)
		
		issues := result.([]map[string]interface{})
		assert.Len(t, issues, 2)
		assert.Equal(t, "Test issue 1", issues[0]["title"])
		assert.Equal(t, "open", issues[0]["state"])
		assert.Equal(t, "Test issue 2", issues[1]["title"])
		assert.Equal(t, "closed", issues[1]["state"])
	})
	
	// Test createIssue
	t.Run("CreateIssue", func(t *testing.T) {
		params := map[string]interface{}{
			"owner": "octocat",
			"repo":  "hello-world",
			"title": "New test issue",
			"body":  "This is a test issue created by the integration tests.",
		}
		
		result, err := adapter.ExecuteAction(context.Background(), "test-context", "createIssue", params)
		require.NoError(t, err)
		
		issue := result.(map[string]interface{})
		assert.Equal(t, "New test issue", issue["title"])
		assert.Equal(t, "This is a test issue created by the integration tests.", issue["body"])
		assert.Equal(t, "open", issue["state"])
	})
	
	// Test listPullRequests
	t.Run("ListPullRequests", func(t *testing.T) {
		params := map[string]interface{}{
			"owner": "octocat",
			"repo":  "hello-world",
		}
		
		result, err := adapter.ExecuteAction(context.Background(), "test-context", "listPullRequests", params)
		require.NoError(t, err)
		
		prs := result.([]map[string]interface{})
		assert.Len(t, prs, 2)
		assert.Equal(t, "Test PR 1", prs[0]["title"])
		assert.Equal(t, "open", prs[0]["state"])
		assert.Equal(t, "Test PR 2", prs[1]["title"])
		assert.Equal(t, "closed", prs[1]["state"])
	})
}

func TestGitHubAdapter_WebhookHandling(t *testing.T) {
	// Create test logger and metrics client
	logger := observability.NewLogger("test", "debug")
	metricsClient := observability.NewMetricsClient("test")
	
	// Create event bus
	eventBus := events.NewEventBus()
	
	// Create channel to receive events
	eventChan := make(chan events.Event, 10)
	
	// Subscribe to webhook events
	eventBus.Subscribe("github.webhook.push", func(event events.Event) {
		eventChan <- event
	})
	
	// Create GitHub adapter config
	config := github.DefaultConfig()
	config.WebhookSecret = "test-webhook-secret"
	config.DisableWebhooks = false
	
	// Create adapter
	adapter, err := github.New(config, logger, metricsClient, eventBus)
	require.NoError(t, err)
	defer adapter.Close()
	
	// Test webhook handler registration
	t.Run("RegisterWebhookHandler", func(t *testing.T) {
		params := map[string]interface{}{
			"handler_id":   "test-push-handler",
			"event_types":  []interface{}{"push"},
			"repositories": []interface{}{"octocat/hello-world"},
			"branches":     []interface{}{"main"},
		}
		
		result, err := adapter.ExecuteAction(context.Background(), "test-context", "registerWebhookHandler", params)
		require.NoError(t, err)
		
		response := result.(map[string]interface{})
		assert.Equal(t, "test-push-handler", response["handler_id"])
		assert.Equal(t, true, response["success"])
	})
	
	// Test listing webhook handlers
	t.Run("ListWebhookHandlers", func(t *testing.T) {
		result, err := adapter.ExecuteAction(context.Background(), "test-context", "listWebhookHandlers", nil)
		require.NoError(t, err)
		
		response := result.(map[string]interface{})
		handlers := response["handlers"].([]string)
		
		// Should include our registered handler and default handlers
		assert.Contains(t, handlers, "test-push-handler")
		assert.Contains(t, handlers, "default-push")
	})
	
	// Create valid webhook signature
	createSignature := func(payload []byte) string {
		mac := hmac.New(sha256.New, []byte("test-webhook-secret"))
		mac.Write(payload)
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	
	// Test handling a push webhook
	t.Run("HandlePushWebhook", func(t *testing.T) {
		// Create webhook payload
		payload := []byte(`{
			"ref": "refs/heads/main",
			"repository": {
				"id": 123456,
				"name": "hello-world",
				"full_name": "octocat/hello-world",
				"owner": {
					"login": "octocat",
					"id": 1
				}
			},
			"pusher": {
				"name": "octocat"
			},
			"sender": {
				"login": "octocat",
				"id": 1
			}
		}`)
		
		// Create headers
		headers := http.Header{}
		headers.Set("X-GitHub-Event", "push")
		headers.Set("X-GitHub-Delivery", "test-delivery-id")
		headers.Set("X-Hub-Signature-256", createSignature(payload))
		
		// Handle webhook
		err := adapter.HandleWebhook(context.Background(), "push", payload, headers)
		require.NoError(t, err)
		
		// Wait for event to be processed
		var event events.Event
		select {
		case event = <-eventChan:
			// Event received
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for webhook event")
		}
		
		// Verify event data
		assert.Equal(t, "github.webhook.push", event.Type)
		
		data := event.Data.(map[string]interface{})
		assert.Equal(t, "push", data["event_type"])
		assert.Equal(t, "test-delivery-id", data["delivery_id"])
		assert.Equal(t, "octocat/hello-world", data["repository"])
		assert.Equal(t, "octocat", data["sender"])
	})
	
	// Test unregistering a webhook handler
	t.Run("UnregisterWebhookHandler", func(t *testing.T) {
		params := map[string]interface{}{
			"handler_id": "test-push-handler",
		}
		
		result, err := adapter.ExecuteAction(context.Background(), "test-context", "unregisterWebhookHandler", params)
		require.NoError(t, err)
		
		response := result.(map[string]interface{})
		assert.Equal(t, "test-push-handler", response["handler_id"])
		assert.Equal(t, true, response["success"])
		
		// Verify handler was removed
		listResult, err := adapter.ExecuteAction(context.Background(), "test-context", "listWebhookHandlers", nil)
		require.NoError(t, err)
		
		listResponse := listResult.(map[string]interface{})
		handlers := listResponse["handlers"].([]string)
		assert.NotContains(t, handlers, "test-push-handler")
	})
}
