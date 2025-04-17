package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/S-Corkum/mcp-server/internal/adapters/harness"
	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// createHMAC creates an HMAC signature for testing
func createHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// setupTestServerWithHarness creates a test server with a mocked Harness adapter
func setupTestServerWithHarness(t *testing.T) (*Server, *httptest.Server, *core.MockEngine) {
	// Create a mock engine and register a Harness adapter
	mockEngine := core.NewMockEngine()
	
	// Create Harness adapter
	harnessAdapter, err := harness.NewAdapter(harness.Config{
		BaseURL:   "https://harness.io",
		AccountID: "test-account",
	})
	if err != nil {
		t.Fatalf("Failed to create Harness adapter: %v", err)
	}
	
	// Register the adapter with the mock engine
	mockEngine.RegisterAdapter("harness", harnessAdapter)
	
	// Set up the server
	gin.SetMode(gin.TestMode)
	config := Config{
		Auth: AuthConfig{
			APIKeys: map[string]string{
				"test-key": "test-role",
			},
		},
		Webhooks: WebhookConfig{
			Harness: WebhookEndpointConfig{
				Enabled:   true,
				Path:      "/harness",
				Secret:    "test-secret",
				BaseURL:   "https://harness.io",
				AccountID: "default-account",
			},
		},
	}
	
	server := NewServer(mockEngine, &repository.MockEmbeddingRepository{}, config)
	
	// Start the test server
	ts := httptest.NewServer(server.router)
	
	return server, ts, mockEngine
}

// TestGetHarnessWebhookURL tests the GetHarnessWebhookURL function
func TestGetHarnessWebhookURL(t *testing.T) {
	server, ts, _ := setupTestServerWithHarness(t)
	defer ts.Close()
	
	// Test with provided accountID and webhookID
	url, err := server.GetHarnessWebhookURL("test-account", "test-webhook")
	assert.NoError(t, err)
	assert.Contains(t, url, "accountIdentifier=test-account")
	assert.Contains(t, url, "webhookIdentifier=test-webhook")
	
	// Test with defaults
	url, err = server.GetHarnessWebhookURL("", "")
	assert.NoError(t, err)
	assert.Contains(t, url, "accountIdentifier=default-account")
	assert.Contains(t, url, "webhookIdentifier=devopsmcp")
}

// TestGetHarnessWebhookURLHandler tests the webhook URL handler
func TestGetHarnessWebhookURLHandler(t *testing.T) {
	_, ts, _ := setupTestServerWithHarness(t)
	defer ts.Close()
	
	// Test with API key authentication
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/webhooks/harness/url?accountIdentifier=test-account&webhookIdentifier=test-webhook", ts.URL), nil)
	req.Header.Set("X-Api-Key", "test-key")
	
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	// Parse response
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	
	assert.NoError(t, err)
	assert.Contains(t, result, "webhook_url")
	assert.Contains(t, result["webhook_url"].(string), "accountIdentifier=test-account")
	assert.Contains(t, result["webhook_url"].(string), "webhookIdentifier=test-webhook")
}

// TestHarnessWebhookWithAccountIdentifier tests handling a webhook with account identifier
func TestHarnessWebhookWithAccountIdentifier(t *testing.T) {
	_, ts, _ := setupTestServerWithHarness(t)
	defer ts.Close()
	
	// Create a test payload
	payload := `{
		"webhookName": "test-webhook",
		"trigger": {
			"type": "generic",
			"status": "success"
		}
	}`
	
	// Create a valid signature
	mac := createHMAC([]byte(payload), "test-secret")
	
	// Send webhook request with account identifier
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/webhook/harness?accountIdentifier=test-account&webhookIdentifier=test-webhook", ts.URL), 
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Harness-Signature", "sha256="+mac)
	
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	// Parse response
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	
	assert.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}
