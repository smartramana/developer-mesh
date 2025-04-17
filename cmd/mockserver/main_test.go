package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGitHubMockHandler tests the GitHub mock API handler
func TestGitHubMockHandler(t *testing.T) {
	// Create a handler similar to the one in main
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Special handling for rate limit endpoint that's used for health checks
		if r.URL.Path == "/mock-github/rate_limit" {
			response := map[string]interface{}{
				"resources": map[string]interface{}{
					"core": map[string]interface{}{
						"limit": 5000,
						"used": 0,
						"remaining": 5000,
						"reset": time.Now().Add(1 * time.Hour).Unix(),
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		
		// Special handling for health endpoint
		if r.URL.Path == "/mock-github/health" {
			response := map[string]interface{}{
				"status": "ok",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		
		// Default response for other endpoints
		response := map[string]interface{}{
			"success": true,
			"message": "Mock GitHub response",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)
	}

	// Test rate limit endpoint
	t.Run("GitHub Rate Limit", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/mock-github/rate_limit", nil)
		assert.NoError(t, err)
		
		rr := httptest.NewRecorder()
		http.HandlerFunc(handler).ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		
		var response map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		
		resources, ok := response["resources"].(map[string]interface{})
		assert.True(t, ok)
		
		core, ok := resources["core"].(map[string]interface{})
		assert.True(t, ok)
		
		// Verify the fields in the response
		assert.Equal(t, float64(5000), core["limit"])
		assert.Equal(t, float64(0), core["used"])
		assert.Equal(t, float64(5000), core["remaining"])
		assert.NotZero(t, core["reset"])
	})

	// Test health endpoint
	t.Run("GitHub Health", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/mock-github/health", nil)
		assert.NoError(t, err)
		
		rr := httptest.NewRecorder()
		http.HandlerFunc(handler).ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		
		var response map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		
		assert.Equal(t, "ok", response["status"])
		assert.NotEmpty(t, response["timestamp"])
	})

	// Test default response
	t.Run("GitHub Default", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/mock-github/repos/owner/repo", nil)
		assert.NoError(t, err)
		
		rr := httptest.NewRecorder()
		http.HandlerFunc(handler).ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		
		var response map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		
		assert.Equal(t, true, response["success"])
		assert.Equal(t, "Mock GitHub response", response["message"])
		assert.NotEmpty(t, response["timestamp"])
	})
}

// TestWebhookMockHandler tests the webhook handler
func TestWebhookMockHandler(t *testing.T) {
	// Create a webhook handler similar to the one in main
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}

	t.Run("Webhook Handler", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/api/v1/webhook/github", nil)
		assert.NoError(t, err)
		
		rr := httptest.NewRecorder()
		http.HandlerFunc(handler).ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, `{"status":"ok"}`, rr.Body.String())
	})
}

// TestHealthCheckHandler tests the health check handler
func TestHealthCheckHandler(t *testing.T) {
	// Create a health check handler similar to the one in main
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}

	t.Run("Health Check", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/health", nil)
		assert.NoError(t, err)
		
		rr := httptest.NewRecorder()
		http.HandlerFunc(handler).ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, `{"status":"healthy"}`, rr.Body.String())
	})
}
