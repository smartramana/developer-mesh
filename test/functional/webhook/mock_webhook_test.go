package webhook_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// MockWebhookConfig implements a simple webhook configuration for testing
type MockWebhookConfig struct {
	EnabledFlag        bool
	GitHubEndpointVal  string
	GitHubSecretVal    string
	AllowedEventsVal   []string
	IPValidationVal    bool
	ProcessedEvents    map[string]bool
	ErrorEvents        map[string]bool // Tracks events that should be considered errors
	ProcessedEventsMux sync.Mutex
}

func (m *MockWebhookConfig) Enabled() bool {
	return m.EnabledFlag
}

func (m *MockWebhookConfig) GitHubEndpoint() string {
	return m.GitHubEndpointVal
}

func (m *MockWebhookConfig) GitHubSecret() string {
	return m.GitHubSecretVal
}

func (m *MockWebhookConfig) GitHubIPValidationEnabled() bool {
	return m.IPValidationVal
}

func (m *MockWebhookConfig) GitHubAllowedEvents() []string {
	return m.AllowedEventsVal
}

// MockWebhookHandler is a simplified version of the real webhook handler for testing
func MockWebhookHandler(config *MockWebhookConfig) http.HandlerFunc {
	logger := observability.NewLogger("mock-webhooks")
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Check method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			logger.Warn("Mock webhook received non-POST request", nil)
			return
		}

		// 2. Validate event type
		eventType := r.Header.Get("X-GitHub-Event")
		if eventType == "" {
			http.Error(w, "Missing X-GitHub-Event header", http.StatusBadRequest)
			logger.Warn("Mock webhook missing event header", nil)
			return
		}

		// 3. Check if event is allowed
		isAllowed := false
		for _, allowed := range config.AllowedEventsVal {
			if eventType == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			http.Error(w, "Event type not allowed", http.StatusForbidden)
			logger.Warn("Mock webhook event type not allowed", map[string]interface{}{
				"eventType": eventType,
			})
			return
		}

		// 4. Read request body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			logger.Error("Mock webhook failed to read request body", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore for later use

		// 5. Verify signature
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			http.Error(w, "Missing X-Hub-Signature-256 header", http.StatusUnauthorized)
			logger.Warn("Mock webhook missing signature header", nil)
			return
		}

		// Verify the signature
		valid := verifySignature(bodyBytes, signature, config.GitHubSecretVal)
		if !valid {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			logger.Warn("Mock webhook invalid signature", nil)
			return
		}

		// 6. Check idempotency via GitHub Delivery ID
		deliveryID := r.Header.Get("X-GitHub-Delivery")
		if deliveryID == "" {
			http.Error(w, "Missing X-GitHub-Delivery header", http.StatusBadRequest)
			logger.Warn("Mock webhook missing delivery ID", nil)
			return
		}

		// Special handling for error events (like certain push events)
		// Don't set idempotency keys for events that are considered errors
		isErrorEvent := false

		// Check if this is a push event with error characteristics
		if eventType == "push" {
			// Parse the body to determine if it's an error event
			var payload map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &payload); err == nil {
				// If the payload contains event_type=push, consider it an error event
				// (This is a simplification - in real code you'd have more specific criteria)
				if evtType, ok := payload["event_type"].(string); ok && evtType == "push" {
					isErrorEvent = true
					config.ProcessedEventsMux.Lock()
					config.ErrorEvents[deliveryID] = true
					config.ProcessedEventsMux.Unlock()
				}
			}
		}

		// Only set idempotency for non-error events
		if !isErrorEvent {
			// Check if this event was already processed
			config.ProcessedEventsMux.Lock()
			if processed, exists := config.ProcessedEvents[deliveryID]; exists && processed {
				// This is a duplicate event, but we'll return 200 OK for idempotency
				config.ProcessedEventsMux.Unlock()
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"success","message":"Webhook already processed (idempotent)"}`))
				return
			}
			// Mark as processed
			config.ProcessedEvents[deliveryID] = true
			config.ProcessedEventsMux.Unlock()
		}

		// Return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","message":"Webhook processed successfully"}`))
	}
}

// Helper function to verify webhook signatures
func verifySignature(payload []byte, signature, secret string) bool {
	if !bytes.HasPrefix([]byte(signature), []byte("sha256=")) {
		return false
	}

	signatureHash := signature[7:] // Remove "sha256=" prefix
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signatureHash), []byte(expectedHash))
}

// Helper to set up a test server
func createMockWebhookServer() (*httptest.Server, *MockWebhookConfig) {
	config := &MockWebhookConfig{
		EnabledFlag:       true,
		GitHubEndpointVal: "/api/webhooks/github",
		GitHubSecretVal:   "test-github-webhook-secret",
		AllowedEventsVal:  []string{"issues", "push", "pull_request"},
		IPValidationVal:   false,
		ProcessedEvents:   make(map[string]bool),
		ErrorEvents:       make(map[string]bool),
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/webhooks/github", MockWebhookHandler(config)).Methods("POST")

	server := httptest.NewServer(router)
	return server, config
}

var _ = Describe("Mock GitHub Webhook Tests", func() {
	// Add a test case to specifically test error event handling for push events
	var (
		mockServer *httptest.Server
		mockConfig *MockWebhookConfig
	)

	BeforeEach(func() {
		mockServer, mockConfig = createMockWebhookServer()
	})

	AfterEach(func() {
		mockServer.Close()
	})

	It("should validate webhook signatures correctly", func() {
		// Create a test payload
		payload := map[string]interface{}{
			"action": "opened",
			"issue": map[string]interface{}{
				"number": 1,
				"title":  "Test Issue",
			},
			"repository": map[string]interface{}{
				"full_name": "S-Corkum/devops-mcp",
			},
			"sender": map[string]interface{}{
				"login": "testuser",
			},
		}
		body, err := json.Marshal(payload)
		Expect(err).NotTo(HaveOccurred())

		// Compute valid signature
		mac := hmac.New(sha256.New, []byte(mockConfig.GitHubSecretVal))
		mac.Write(body)
		validSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		// Test with valid signature
		req, err := http.NewRequest("POST", mockServer.URL+"/api/webhooks/github", bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", validSignature)
		req.Header.Set("X-GitHub-Event", "issues")
		req.Header.Set("X-GitHub-Delivery", "test-valid-"+time.Now().Format("20060102150405"))

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		// Test with invalid signature
		invalidSignature := "sha256=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		req, err = http.NewRequest("POST", mockServer.URL+"/api/webhooks/github", bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", invalidSignature)
		req.Header.Set("X-GitHub-Event", "issues")
		req.Header.Set("X-GitHub-Delivery", "test-invalid-"+time.Now().Format("20060102150405"))

		resp, err = client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
	})

	It("should ensure idempotency with the same delivery ID", func() {
		// Create a test payload
		payload := map[string]interface{}{
			"action": "opened",
			"issue": map[string]interface{}{
				"number": 2,
				"title":  "Idempotency Test",
			},
			"repository": map[string]interface{}{
				"full_name": "S-Corkum/devops-mcp",
			},
			"sender": map[string]interface{}{
				"login": "testuser",
			},
		}
		body, err := json.Marshal(payload)
		Expect(err).NotTo(HaveOccurred())

		// Compute signature
		mac := hmac.New(sha256.New, []byte(mockConfig.GitHubSecretVal))
		mac.Write(body)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		// Use the same delivery ID for both requests
		deliveryID := "test-idempotent-" + time.Now().Format("20060102150405")

		// First request should succeed
		req, err := http.NewRequest("POST", mockServer.URL+"/api/webhooks/github", bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", signature)
		req.Header.Set("X-GitHub-Event", "issues")
		req.Header.Set("X-GitHub-Delivery", deliveryID)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		// Second request with same ID should still return 200 (idempotent)
		req, err = http.NewRequest("POST", mockServer.URL+"/api/webhooks/github", bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", signature)
		req.Header.Set("X-GitHub-Event", "issues")
		req.Header.Set("X-GitHub-Delivery", deliveryID)

		resp, err = client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		// Verify it was marked as processed exactly once
		Expect(mockConfig.ProcessedEvents).To(HaveKey(deliveryID))
		Expect(mockConfig.ProcessedEvents[deliveryID]).To(BeTrue())
	})

	It("should reject invalid event types", func() {
		// Create payload with unsupported event
		payload := map[string]interface{}{
			"action": "opened",
			"repository": map[string]interface{}{
				"full_name": "S-Corkum/devops-mcp",
			},
		}
		body, err := json.Marshal(payload)
		Expect(err).NotTo(HaveOccurred())

		// Compute signature
		mac := hmac.New(sha256.New, []byte(mockConfig.GitHubSecretVal))
		mac.Write(body)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		// Send request with unsupported event type
		req, err := http.NewRequest("POST", mockServer.URL+"/api/webhooks/github", bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", signature)
		req.Header.Set("X-GitHub-Event", "unsupported_event") // Not in allowed events
		req.Header.Set("X-GitHub-Delivery", "test-invalid-event-"+time.Now().Format("20060102150405"))

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
	})

	It("should not set idempotency key for error events (push event)", func() {
		// Create a payload that simulates a push event that would be considered an error
		payload := map[string]interface{}{
			"action":     "opened",
			"event_type": "push", // This is the marker that makes it an error event
			"issue": map[string]interface{}{
				"number": 3,
				"title":  "Error Event Test",
			},
			"repository": map[string]interface{}{
				"full_name": "S-Corkum/devops-mcp",
			},
			"sender": map[string]interface{}{
				"login": "testuser",
			},
		}
		body, err := json.Marshal(payload)
		Expect(err).NotTo(HaveOccurred())

		// Compute signature
		mac := hmac.New(sha256.New, []byte(mockConfig.GitHubSecretVal))
		mac.Write(body)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		// Create unique delivery ID for this test
		deliveryID := "test-error-event-" + time.Now().Format("20060102150405")

		// Send request with push event
		req, err := http.NewRequest("POST", mockServer.URL+"/api/webhooks/github", bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", signature)
		req.Header.Set("X-GitHub-Event", "push") // Use push event type
		req.Header.Set("X-GitHub-Delivery", deliveryID)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		// The request should succeed even though it's an error event
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		// Verify the event was marked as an error event
		mockConfig.ProcessedEventsMux.Lock()
		defer mockConfig.ProcessedEventsMux.Unlock()

		// Check it was marked as an error event
		Expect(mockConfig.ErrorEvents).To(HaveKey(deliveryID))

		// Verify it was NOT stored in the processed events map (no idempotency key set)
		_, exists := mockConfig.ProcessedEvents[deliveryID]
		Expect(exists).To(BeFalse(), "Idempotency key should not be set for error events")
	})
})
