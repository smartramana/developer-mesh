package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEdgeAuthenticator_NoAPIURL(t *testing.T) {
	// When no REST API URL is configured, should deny access (fail closed)
	auth := NewEdgeAuthenticator("", "test-edge-123")

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "some-key")

	result := auth.AuthenticateRequest(req)
	assert.False(t, result, "Should deny request when no REST API URL is configured")
}

func TestEdgeAuthenticator_ValidAPIKey(t *testing.T) {
	// Create mock REST API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/v1/auth/edge-mcp", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var req EdgeMCPAuthRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Respond based on API key
		if req.APIKey == "valid-key-123" {
			json.NewEncoder(w).Encode(EdgeMCPAuthResponse{
				Success:  true,
				Token:    "jwt-token-xyz",
				TenantID: "tenant-abc",
			})
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(EdgeMCPAuthResponse{
				Success: false,
				Message: "Invalid API key",
			})
		}
	}))
	defer server.Close()

	auth := NewEdgeAuthenticator(server.URL, "edge-test-123")

	tests := []struct {
		name       string
		apiKey     string
		header     string
		shouldPass bool
	}{
		{
			name:       "Valid API Key with Bearer",
			apiKey:     "valid-key-123",
			header:     "Authorization",
			shouldPass: true,
		},
		{
			name:       "Valid API Key with X-API-Key",
			apiKey:     "valid-key-123",
			header:     "X-API-Key",
			shouldPass: true,
		},
		{
			name:       "Invalid API Key",
			apiKey:     "wrong-key",
			header:     "X-API-Key",
			shouldPass: false,
		},
		{
			name:       "Empty API Key",
			apiKey:     "",
			header:     "X-API-Key",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			if tt.apiKey != "" {
				if tt.header == "Authorization" {
					req.Header.Set("Authorization", "Bearer "+tt.apiKey)
				} else {
					req.Header.Set(tt.header, tt.apiKey)
				}
			}

			result := auth.AuthenticateRequest(req)
			assert.Equal(t, tt.shouldPass, result)
		})
	}
}

func TestEdgeAuthenticator_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(EdgeMCPAuthResponse{
			Success:  true,
			Token:    "jwt-token",
			TenantID: "tenant-123",
		})
	}))
	defer server.Close()

	auth := NewEdgeAuthenticator(server.URL, "edge-test")

	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-API-Key", "cached-key")

	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-API-Key", "cached-key")

	// First request should call API
	result1 := auth.AuthenticateRequest(req1)
	assert.True(t, result1)
	assert.Equal(t, 1, callCount)

	// Second request should use cache
	result2 := auth.AuthenticateRequest(req2)
	assert.True(t, result2)
	assert.Equal(t, 1, callCount, "Should use cached result, not call API again")
}

func TestEdgeAuthenticator_NoCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not call API when no credentials provided")
	}))
	defer server.Close()

	auth := NewEdgeAuthenticator(server.URL, "edge-test")

	req, _ := http.NewRequest("GET", "/test", nil)
	// No headers set

	result := auth.AuthenticateRequest(req)
	assert.False(t, result, "Should reject request with no credentials")
}
