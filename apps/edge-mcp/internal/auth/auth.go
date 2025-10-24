package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Authenticator handles authentication
type Authenticator interface {
	AuthenticateRequest(r *http.Request) bool
	GetTenantID(apiKey string) (string, error)
}

// EdgeAuthenticator implements REST API-based authentication for Edge MCP
type EdgeAuthenticator struct {
	restAPIURL string
	edgeMCPID  string
	httpClient *http.Client

	// Cache for validated API keys
	cacheMu   sync.RWMutex
	authCache map[string]*CachedAuth
}

// CachedAuth holds cached authentication results
type CachedAuth struct {
	Valid     bool
	TenantID  string
	Token     string
	ExpiresAt time.Time
}

// EdgeMCPAuthRequest matches the REST API request structure
type EdgeMCPAuthRequest struct {
	EdgeMCPID string `json:"edge_mcp_id"`
	APIKey    string `json:"api_key"`
}

// EdgeMCPAuthResponse matches the REST API response structure
type EdgeMCPAuthResponse struct {
	Success  bool   `json:"success"`
	Token    string `json:"token,omitempty"`
	Message  string `json:"message,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
}

// NewEdgeAuthenticator creates a new Edge authenticator
func NewEdgeAuthenticator(restAPIURL, edgeMCPID string) Authenticator {
	return &EdgeAuthenticator{
		restAPIURL: restAPIURL,
		edgeMCPID:  edgeMCPID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		authCache: make(map[string]*CachedAuth),
	}
}

// AuthenticateRequest authenticates an HTTP request by calling the REST API
func (a *EdgeAuthenticator) AuthenticateRequest(r *http.Request) bool {
	// Extract API key from request
	apiKey := a.extractAPIKey(r)
	if apiKey == "" {
		return false
	}

	// Check cache first
	if cached := a.getCached(apiKey); cached != nil && cached.Valid && time.Now().Before(cached.ExpiresAt) {
		return true
	}

	// Validate against REST API
	valid, tenantID, token := a.validateWithAPI(r.Context(), apiKey)

	// Cache the result (valid for 5 minutes)
	a.cacheMu.Lock()
	a.authCache[apiKey] = &CachedAuth{
		Valid:     valid,
		TenantID:  tenantID,
		Token:     token,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	a.cacheMu.Unlock()

	return valid
}

// GetTenantID returns the tenant ID for a validated API key
func (a *EdgeAuthenticator) GetTenantID(apiKey string) (string, error) {
	if cached := a.getCached(apiKey); cached != nil && cached.Valid {
		return cached.TenantID, nil
	}
	return "", fmt.Errorf("API key not authenticated")
}

// extractAPIKey extracts the API key from the request
func (a *EdgeAuthenticator) extractAPIKey(r *http.Request) string {
	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Check X-API-Key header
		authHeader = r.Header.Get("X-API-Key")
	}

	// If no header found, check query parameter (for WebSocket connections from browsers)
	if authHeader == "" {
		authHeader = r.URL.Query().Get("token")
	}

	if authHeader == "" {
		return ""
	}

	// Handle Bearer token format
	return strings.TrimPrefix(authHeader, "Bearer ")
}

// getCached retrieves cached authentication result
func (a *EdgeAuthenticator) getCached(apiKey string) *CachedAuth {
	a.cacheMu.RLock()
	defer a.cacheMu.RUnlock()
	return a.authCache[apiKey]
}

// validateWithAPI validates an API key with the REST API
func (a *EdgeAuthenticator) validateWithAPI(ctx context.Context, apiKey string) (bool, string, string) {
	// If no REST API URL configured, fail closed (deny access)
	if a.restAPIURL == "" {
		return false, "", ""
	}

	// Prepare request
	reqBody := EdgeMCPAuthRequest{
		EdgeMCPID: a.edgeMCPID,
		APIKey:    apiKey,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return false, "", ""
	}

	// Call REST API
	url := fmt.Sprintf("%s/api/v1/auth/edge-mcp", strings.TrimSuffix(a.restAPIURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, "", ""
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false, "", ""
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse response
	var authResp EdgeMCPAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return false, "", ""
	}

	return authResp.Success, authResp.TenantID, authResp.Token
}
