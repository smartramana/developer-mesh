package webhook_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitHub Webhook Endpoint", func() {
	var (
		serverURL string
		secret    string
	)

	BeforeEach(func() {
		serverURL = os.Getenv("MCP_SERVER_URL")
		if serverURL == "" {
			serverURL = "http://localhost:8080"
		}
		secret = os.Getenv("MCP_GITHUB_WEBHOOK_SECRET")
		if secret == "" {
			secret = "testsecret"
		}
	})

	It("should accept a valid GitHub webhook payload and return 200", func() {
		// Example GitHub event payload
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

		// Compute X-Hub-Signature-256
		h := hmac.New(sha256.New, []byte(secret))
		h.Write(body)
		signature := "sha256=" + hex.EncodeToString(h.Sum(nil))

		// Send POST request with signature header
		req, err := http.NewRequest("POST", serverURL+"/api/webhooks/github", bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", signature)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})
})
