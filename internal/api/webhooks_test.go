package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWebhookConfig is a mock implementation of WebhookConfigInterface
type MockWebhookConfig struct {
	mock.Mock
}

func (m *MockWebhookConfig) Enabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockWebhookConfig) GitHubSecret() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockWebhookConfig) GitHubEndpoint() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockWebhookConfig) GitHubIPValidationEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockWebhookConfig) GitHubAllowedEvents() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// computeHMAC computes HMAC-SHA256 for testing
func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	// Test cases
	testCases := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		expected  bool
	}{
		{
			name:      "Valid signature",
			payload:   []byte(`{"action":"opened","issue":{"number":1}}`),
			secret:    "test-secret",
			expected:  true,
		},
		{
			name:      "Invalid signature",
			payload:   []byte(`{"action":"opened","issue":{"number":1}}`),
			signature: "sha256=invalid-signature",
			secret:    "test-secret",
			expected:  false,
		},
		{
			name:      "Invalid signature format",
			payload:   []byte(`{"action":"opened","issue":{"number":1}}`),
			signature: "invalid-format",
			secret:    "test-secret",
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// For valid signature, compute it
			signature := tc.signature
			if tc.expected {
				signature = computeHMAC(tc.payload, tc.secret)
			}
			
			// Verify
			result := verifySignature(tc.payload, signature, tc.secret)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGitHubWebhookHandler(t *testing.T) {
	// Mock config
	mockConfig := new(MockWebhookConfig)
	mockConfig.On("GitHubSecret").Return("test-secret")
	mockConfig.On("GitHubAllowedEvents").Return([]string{"push", "pull_request"})
	
	// Test cases
	testCases := []struct {
		name           string
		method         string
		eventType      string
		payload        map[string]interface{}
		includeSignature bool
		expectedStatus int
	}{
		{
			name:           "Valid push event",
			method:         http.MethodPost,
			eventType:      "push",
			payload:        map[string]interface{}{
				"ref": "refs/heads/main",
				"repository": map[string]interface{}{
					"full_name": "user/repo",
				},
				"sender": map[string]interface{}{
					"login": "username",
				},
			},
			includeSignature: true,
			expectedStatus:  http.StatusOK,
		},
		{
			name:           "Invalid method",
			method:         http.MethodGet,
			payload:        map[string]interface{}{},
			includeSignature: true,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Missing event header",
			method:         http.MethodPost,
			payload:        map[string]interface{}{},
			includeSignature: true,
			expectedStatus:  http.StatusBadRequest,
		},
		{
			name:           "Disallowed event type",
			method:         http.MethodPost,
			eventType:      "issues",
			payload:        map[string]interface{}{},
			includeSignature: true,
			expectedStatus:  http.StatusForbidden,
		},
		{
			name:           "Missing signature",
			method:         http.MethodPost,
			eventType:      "push",
			payload:        map[string]interface{}{},
			includeSignature: false,
			expectedStatus:  http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create handler
			handler := GitHubWebhookHandler(mockConfig)
			
			// Create payload
			var payloadBytes []byte
			var err error
			if tc.payload != nil {
				payloadBytes, err = json.Marshal(tc.payload)
				assert.NoError(t, err)
			} else {
				payloadBytes = []byte{}
			}
			
			// Create request
			req := httptest.NewRequest(tc.method, "/api/webhooks/github", bytes.NewBuffer(payloadBytes))
			
			// Add headers
			if tc.eventType != "" {
				req.Header.Set("X-GitHub-Event", tc.eventType)
			}
			
			if tc.includeSignature {
				signature := computeHMAC(payloadBytes, "test-secret")
				req.Header.Set("X-Hub-Signature-256", signature)
			}
			
			// Create response recorder
			rr := httptest.NewRecorder()
			
			// Call handler
			handler.ServeHTTP(rr, req)
			
			// Check status
			assert.Equal(t, tc.expectedStatus, rr.Code)
		})
	}
}

func TestIPValidationMiddleware(t *testing.T) {
	// Skip real IP validation in tests 
	// This would be a more complex test requiring mocking of the IP ranges API
	// Here we just verify the middleware passes requests through when validation is disabled
	
	// Mock config
	mockConfig := new(MockWebhookConfig)
	mockConfig.On("GitHubIPValidationEnabled").Return(false)
	
	// Create validator
	validator := NewGitHubIPValidator()
	
	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
	
	// Create middleware
	middleware := GitHubIPValidationMiddleware(validator, mockConfig)
	
	// Create request
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", nil)
	req.RemoteAddr = "1.2.3.4:1234" // Non-GitHub IP
	
	// Create response recorder
	rr := httptest.NewRecorder()
	
	// Apply middleware
	middleware(testHandler).ServeHTTP(rr, req)
	
	// Check that middleware passed request through (because validation is disabled)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Body.String())
}