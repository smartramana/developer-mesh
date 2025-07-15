package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Config ---
type mockConfig struct {
	mock.Mock
}

func (m *mockConfig) Enabled() bool {
	args := m.Called()
	return args.Bool(0)
}
func (m *mockConfig) GitHubAllowedEvents() []string {
	args := m.Called()
	return args.Get(0).([]string)
}
func (m *mockConfig) GitHubSecret() string {
	args := m.Called()
	return args.String(0)
}
func (m *mockConfig) GitHubEndpoint() string {
	args := m.Called()
	return args.String(0)
}
func (m *mockConfig) GitHubIPValidationEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestGitHubWebhookHandler_MissingEventHeader(t *testing.T) {
	config := new(mockConfig)
	// Set up all the mock expectations that might be called
	config.On("GitHubSecret").Return("testsecret")
	config.On("GitHubAllowedEvents").Return([]string{"push"})
	config.On("GitHubIPValidationEnabled").Return(false)
	logger := observability.NewLogger("test-webhooks")
	handler := GitHubWebhookHandler(config, logger)

	req := httptest.NewRequest("POST", "/", nil)
	w := httptest.NewRecorder()

	handler(w, req)
	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGitHubWebhookHandler_DisallowedEvent(t *testing.T) {
	config := new(mockConfig)
	config.On("GitHubAllowedEvents").Return([]string{"push"})
	config.On("GitHubSecret").Return("testsecret")
	config.On("GitHubIPValidationEnabled").Return(false)
	logger := observability.NewLogger("test-webhooks")
	handler := GitHubWebhookHandler(config, logger)

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-GitHub-Event", "pull_request")
	w := httptest.NewRecorder()

	handler(w, req)
	resp := w.Result()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestGitHubWebhookHandler_MissingSignature(t *testing.T) {
	config := new(mockConfig)
	config.On("GitHubAllowedEvents").Return([]string{"push"})
	config.On("GitHubSecret").Return("testsecret")
	config.On("GitHubIPValidationEnabled").Return(false)
	logger := observability.NewLogger("test-webhooks")
	handler := GitHubWebhookHandler(config, logger)

	// Create a proper GitHub push event payload
	payload := map[string]interface{}{
		"ref":        "refs/heads/main",
		"repository": map[string]interface{}{"full_name": "repo"},
		"sender":     map[string]interface{}{"login": "user"},
		"head_commit": map[string]interface{}{
			"id":      "abc123",
			"message": "Test commit",
			"author": map[string]interface{}{
				"name":  "Test Author",
				"email": "test@example.com",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/", bytes.NewBuffer(body))
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()

	handler(w, req)
	resp := w.Result()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGitHubWebhookHandler_InvalidSignature(t *testing.T) {
	// Ensure test mode is disabled for this test
	oldTestMode := os.Getenv("MCP_TEST_MODE")
	if err := os.Setenv("MCP_TEST_MODE", "false"); err != nil {
		t.Fatalf("Failed to set MCP_TEST_MODE: %v", err)
	}
	defer func() {
		if err := os.Setenv("MCP_TEST_MODE", oldTestMode); err != nil {
			t.Logf("Failed to restore MCP_TEST_MODE: %v", err)
		}
	}()

	config := new(mockConfig)
	config.On("GitHubAllowedEvents").Return([]string{"push"})
	config.On("GitHubSecret").Return("testsecret")
	config.On("GitHubIPValidationEnabled").Return(false)
	logger := observability.NewLogger("test-webhooks")
	handler := GitHubWebhookHandler(config, logger)

	// Create a proper GitHub push event payload
	payload := map[string]interface{}{
		"ref":        "refs/heads/main",
		"repository": map[string]interface{}{"full_name": "repo"},
		"sender":     map[string]interface{}{"login": "user"},
		"head_commit": map[string]interface{}{
			"id":      "abc123",
			"message": "Test commit",
			"author": map[string]interface{}{
				"name":  "Test Author",
				"email": "test@example.com",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/", bytes.NewBuffer(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsig")
	w := httptest.NewRecorder()

	handler(w, req)
	resp := w.Result()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGitHubWebhookHandler_ValidEventAndSignature(t *testing.T) {
	config := new(mockConfig)
	config.On("GitHubAllowedEvents").Return([]string{"push"})
	config.On("GitHubSecret").Return("testsecret")
	config.On("GitHubIPValidationEnabled").Return(false)
	config.On("GitHubEndpoint").Return("/api/webhook/github")
	logger := observability.NewLogger("test-webhooks")
	handler := GitHubWebhookHandler(config, logger)

	payload := map[string]interface{}{
		"ref":        "refs/heads/main",
		"repository": map[string]interface{}{"full_name": "repo"},
		"sender":     map[string]interface{}{"login": "user"},
		"head_commit": map[string]interface{}{
			"id":      "abc123",
			"message": "Test commit",
			"author": map[string]interface{}{
				"name":  "Test Author",
				"email": "test@example.com",
			},
		},
	}
	body, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, []byte("testsecret"))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/", bytes.NewBuffer(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()

	handler(w, req)
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
