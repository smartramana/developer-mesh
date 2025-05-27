package client

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/stretchr/testify/assert"
)

// setupMockServer creates a mock HTTP server for testing the client
func setupMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check path and handle accordingly
		if strings.HasPrefix(r.URL.Path, "/api/v1/contexts") {
			handleContextEndpoint(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/webhook/agent") {
			handleWebhookEndpoint(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/v1/devops/adapters") {
			handleToolEndpoint(w, r)
			return
		}

		// Default response for unknown endpoints
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Endpoint not found"}`))
	}))
}

// handleContextEndpoint handles context-related endpoints
func handleContextEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Context creation
	if r.Method == "POST" && r.URL.Path == "/api/v1/contexts" {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
			"id": "test-context-123",
			"agent_id": "test-agent",
			"model_id": "test-model",
			"session_id": "test-session",
			"content": [],
			"metadata": {},
			"current_tokens": 0,
			"max_tokens": 1000,
			"created_at": "2023-01-01T00:00:00Z",
			"updated_at": "2023-01-01T00:00:00Z"
		}`))
		return
	}

	// Context search
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/search") {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"results": [
				{
					"role": "user",
					"content": "This is a test message",
					"timestamp": "2023-01-01T00:00:00Z",
					"tokens": 5
				}
			]
		}`))
		return
	}

	// Context summary
	if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/summary") {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"summary": "This is a test summary"
		}`))
		return
	}

	// List contexts
	if r.Method == "GET" && r.URL.Path == "/api/v1/contexts" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"contexts": [
				{
					"id": "test-context-123",
					"agent_id": "test-agent",
					"model_id": "test-model",
					"session_id": "test-session",
					"content": [],
					"metadata": {},
					"current_tokens": 0,
					"max_tokens": 1000,
					"created_at": "2023-01-01T00:00:00Z",
					"updated_at": "2023-01-01T00:00:00Z"
				}
			]
		}`))
		return
	}

	// Extract context ID from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Invalid context ID"}`))
		return
	}
	contextID := parts[3]

	// Get context
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{
			"id": "%s",
			"agent_id": "test-agent",
			"model_id": "test-model",
			"session_id": "test-session",
			"content": [],
			"metadata": {},
			"current_tokens": 0,
			"max_tokens": 1000,
			"created_at": "2023-01-01T00:00:00Z",
			"updated_at": "2023-01-01T00:00:00Z"
		}`, contextID)))
		return
	}

	// Update context
	if r.Method == "PUT" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{
			"id": "%s",
			"agent_id": "test-agent",
			"model_id": "test-model",
			"session_id": "test-session",
			"content": [
				{
					"role": "user",
					"content": "Updated content",
					"timestamp": "2023-01-01T00:00:00Z",
					"tokens": 2
				}
			],
			"metadata": {},
			"current_tokens": 2,
			"max_tokens": 1000,
			"created_at": "2023-01-01T00:00:00Z",
			"updated_at": "2023-01-01T00:00:00Z"
		}`, contextID)))
		return
	}

	// Delete context
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	// Default response for unsupported methods
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Write([]byte(`{"error":"Method not allowed"}`))
}

// handleWebhookEndpoint handles webhook endpoints
func handleWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check for webhook signature if applicable
	signature := r.Header.Get("X-MCP-Signature")
	if signature != "" {
		// In a real implementation, we would verify the signature here
		// For testing purposes, we'll accept any non-empty signature
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleToolEndpoint handles tool-related endpoints
func handleToolEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// List tools
	if r.Method == "GET" && r.URL.Path == "/api/v1/devops/adapters" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"adapters": ["github", "artifactory"]
		}`))
		return
	}

	// Tool action
	if r.Method == "POST" && strings.Contains(r.URL.Path, "/action") {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"result": "success",
			"data": {
				"issue_number": 123,
				"url": "https://github.com/owner/repo/issues/123"
			}
		}`))
		return
	}

	// Tool query
	if r.Method == "POST" && strings.Contains(r.URL.Path, "/query") {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"result": "success",
			"data": {
				"repositories": ["repo1", "repo2"]
			}
		}`))
		return
	}

	// Default response for unsupported methods
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Write([]byte(`{"error":"Method not allowed"}`))
}

// TestNewClient tests client initialization
func TestNewClient(t *testing.T) {
	t.Run("Default Client", func(t *testing.T) {
		client := NewClient("http://localhost:8080")
		assert.Equal(t, "http://localhost:8080", client.baseURL)
		assert.Empty(t, client.apiKey)
		assert.Empty(t, client.webhookSecret)
		assert.NotNil(t, client.httpClient)
	})

	t.Run("Client With Options", func(t *testing.T) {
		customHTTPClient := &http.Client{Timeout: 60 * time.Second}
		client := NewClient(
			"http://localhost:8080",
			WithAPIKey("test-api-key"),
			WithWebhookSecret("test-webhook-secret"),
			WithHTTPClient(customHTTPClient),
		)

		assert.Equal(t, "http://localhost:8080", client.baseURL)
		assert.Equal(t, "test-api-key", client.apiKey)
		assert.Equal(t, "test-webhook-secret", client.webhookSecret)
		assert.Equal(t, customHTTPClient, client.httpClient)
	})
}

// TestCreateContext tests the CreateContext method
func TestCreateContext(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	contextData := &models.Context{
		AgentID:   "test-agent",
		ModelID:   "test-model",
		SessionID: "test-session",
		MaxTokens: 1000,
		Content:   []models.ContextItem{},
		Metadata:  map[string]interface{}{},
	}

	result, err := client.CreateContext(ctx, contextData)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-context-123", result.ID)
	assert.Equal(t, "test-agent", result.AgentID)
	assert.Equal(t, "test-model", result.ModelID)
	assert.Equal(t, "test-session", result.SessionID)
}

// TestGetContext tests the GetContext method
func TestGetContext(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	result, err := client.GetContext(ctx, "test-context-456")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "test-agent", result.AgentID)
}

// TestUpdateContext tests the UpdateContext method
func TestUpdateContext(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	contextData := &models.Context{
		ID:        "test-context-789",
		AgentID:   "test-agent",
		ModelID:   "test-model",
		SessionID: "test-session",
		MaxTokens: 1000,
		Content: []models.ContextItem{
			{
				Role:    "user",
				Content: "Updated content",
				Tokens:  2,
			},
		},
		Metadata: map[string]interface{}{},
	}

	options := &models.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	}

	result, err := client.UpdateContext(ctx, "test-context-789", contextData, options)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, 1, len(result.Content))
	assert.Equal(t, "Updated content", result.Content[0].Content)
}

// TestDeleteContext tests the DeleteContext method
func TestDeleteContext(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	err := client.DeleteContext(ctx, "test-context-123")
	assert.NoError(t, err)
}

// TestListContexts tests the ListContexts method
func TestListContexts(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	results, err := client.ListContexts(ctx, "test-agent", "test-session", nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "test-context-123", results[0].ID)
}

// TestListContextsWithOptions tests the ListContexts method with additional options
func TestListContextsWithOptions(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	options := map[string]string{
		"limit": "10",
		"sort":  "created_at",
		"order": "desc",
	}
	results, err := client.ListContexts(ctx, "test-agent", "test-session", options)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "test-context-123", results[0].ID)
}

// TestSearchContext tests the SearchContext method
func TestSearchContext(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	results, err := client.SearchContext(ctx, "test-context-123", "test query")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "This is a test message", results[0].Content)
}

// TestSummarizeContext tests the SummarizeContext method
func TestSummarizeContext(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	summary, err := client.SummarizeContext(ctx, "test-context-123")
	assert.NoError(t, err)
	assert.Equal(t, "This is a test summary", summary)
}

// TestSendEvent tests the SendEvent method
func TestSendEvent(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	// Create a client with a webhook secret
	webhookSecret := "test-webhook-secret"
	client := NewClient(server.URL, WithWebhookSecret(webhookSecret))

	ctx := context.Background()
	event := &models.Event{
		Source:    "agent",
		Type:      "task_complete",
		AgentID:   "test-agent",
		SessionID: "test-session",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"task_id": "123",
			"status":  "completed",
		},
	}

	// Convert event to JSON for calculating expected signature
	eventJSON, _ := json.Marshal(event)
	expectedMac := hmac.New(sha256.New, []byte(webhookSecret))
	expectedMac.Write(eventJSON)
	expectedSignature := hex.EncodeToString(expectedMac.Sum(nil))

	// Set up a custom handler to verify the signature
	customServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/webhook/agent", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, expectedSignature, r.Header.Get("X-MCP-Signature"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer customServer.Close()

	// Use the custom server URL for this test
	client.baseURL = customServer.URL

	err := client.SendEvent(ctx, event)
	assert.NoError(t, err)
}

// TestSendEventWithoutWebhookSecret tests the SendEvent method without a webhook secret
func TestSendEventWithoutWebhookSecret(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	// Create a client without a webhook secret
	client := NewClient(server.URL)

	ctx := context.Background()
	event := &models.Event{
		Source:    "agent",
		Type:      "task_complete",
		AgentID:   "test-agent",
		SessionID: "test-session",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"task_id": "123",
			"status":  "completed",
		},
	}

	// Set up a custom handler to verify no signature is sent
	customServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/webhook/agent", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Empty(t, r.Header.Get("X-MCP-Signature"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer customServer.Close()

	// Use the custom server URL for this test
	client.baseURL = customServer.URL

	err := client.SendEvent(ctx, event)
	assert.NoError(t, err)
}

// TestExecuteToolAction tests the ExecuteToolAction method
func TestExecuteToolAction(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	params := map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
		"title": "Test Issue",
		"body":  "This is a test issue",
	}

	result, err := client.ExecuteToolAction(ctx, "test-context-123", "github", "create_issue", params)
	assert.NoError(t, err)
	assert.Equal(t, "success", result["result"])

	// Check that the nested data was parsed correctly
	data, ok := result["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(123), data["issue_number"])
	assert.Equal(t, "https://github.com/owner/repo/issues/123", data["url"])
}

// TestQueryToolData tests the QueryToolData method
func TestQueryToolData(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	query := map[string]interface{}{
		"type": "repositories",
	}

	result, err := client.QueryToolData(ctx, "github", query)
	assert.NoError(t, err)
	assert.Equal(t, "success", result["result"])

	// Check that the nested data was parsed correctly
	data, ok := result["data"].(map[string]interface{})
	assert.True(t, ok)

	repos, ok := data["repositories"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(repos))
	assert.Equal(t, "repo1", repos[0])
	assert.Equal(t, "repo2", repos[1])
}

// TestListTools tests the ListTools method
func TestListTools(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-api-key"))

	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tools))
	assert.Equal(t, "github", tools[0])
	assert.Equal(t, "artifactory", tools[1])
}

// TestErrorHandling tests error responses from the API
func TestErrorHandling(t *testing.T) {
	// Setup a server that always returns errors
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Test error message"}`))
	}))
	defer errorServer.Close()

	client := NewClient(errorServer.URL, WithAPIKey("test-api-key"))
	ctx := context.Background()

	// Test error handling for various methods
	t.Run("CreateContext Error", func(t *testing.T) {
		contextData := &models.Context{
			AgentID:   "test-agent",
			ModelID:   "test-model",
			MaxTokens: 1000,
		}
		_, err := client.CreateContext(ctx, contextData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("GetContext Error", func(t *testing.T) {
		_, err := client.GetContext(ctx, "test-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("UpdateContext Error", func(t *testing.T) {
		contextData := &models.Context{
			ID:       "test-id",
			AgentID:  "test-agent",
			ModelID:  "test-model",
			Content:  []models.ContextItem{},
			Metadata: map[string]interface{}{},
		}
		_, err := client.UpdateContext(ctx, "test-id", contextData, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("DeleteContext Error", func(t *testing.T) {
		err := client.DeleteContext(ctx, "test-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("ListContexts Error", func(t *testing.T) {
		_, err := client.ListContexts(ctx, "test-agent", "", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("SearchContext Error", func(t *testing.T) {
		_, err := client.SearchContext(ctx, "test-id", "query")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("SummarizeContext Error", func(t *testing.T) {
		_, err := client.SummarizeContext(ctx, "test-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("SendEvent Error", func(t *testing.T) {
		event := &models.Event{
			Source:    "agent",
			Type:      "test",
			AgentID:   "test-agent",
			SessionID: "test-session",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{},
		}
		err := client.SendEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("ExecuteToolAction Error", func(t *testing.T) {
		params := map[string]interface{}{}
		_, err := client.ExecuteToolAction(ctx, "context-id", "github", "action", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("QueryToolData Error", func(t *testing.T) {
		query := map[string]interface{}{}
		_, err := client.QueryToolData(ctx, "github", query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})

	t.Run("ListTools Error", func(t *testing.T) {
		_, err := client.ListTools(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Test error message")
	})
}

// TestInvalidJSONResponse tests handling of invalid JSON in responses
func TestInvalidJSONResponse(t *testing.T) {
	// Setup a server that returns invalid JSON
	invalidJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invalid json`)) // Intentionally invalid JSON
	}))
	defer invalidJSONServer.Close()

	client := NewClient(invalidJSONServer.URL)
	ctx := context.Background()

	// Test error handling for invalid JSON responses
	t.Run("CreateContext with Invalid JSON", func(t *testing.T) {
		contextData := &models.Context{
			AgentID:   "test-agent",
			ModelID:   "test-model",
			MaxTokens: 1000,
		}
		_, err := client.CreateContext(ctx, contextData)
		assert.Error(t, err)
		// The error might be dependent on the JSON parser and HTTP client,
		// so we just check that there's an error without validating the specific message
	})
}

// TestNetworkError tests handling of network errors
func TestNetworkError(t *testing.T) {
	// Create a client with an invalid URL to simulate network errors
	client := NewClient("http://invalid-url-that-doesnt-exist.example")
	ctx := context.Background()

	// Test error handling for network errors
	t.Run("Network Error", func(t *testing.T) {
		_, err := client.GetContext(ctx, "test-id")
		assert.Error(t, err)
		// Different systems might return different network error messages,
		// so we just check that there is an error
	})
}
