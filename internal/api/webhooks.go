package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/S-Corkum/mcp-server/internal/adapters/artifactory"
	"github.com/S-Corkum/mcp-server/internal/adapters/github"
	"github.com/S-Corkum/mcp-server/internal/adapters/harness"
	"github.com/S-Corkum/mcp-server/internal/adapters/sonarqube"
	"github.com/S-Corkum/mcp-server/internal/adapters/xray"
	"github.com/gin-gonic/gin"
)

// webhookPayloadSizeLimit limits webhook payload size to prevent DOS attacks
const webhookPayloadSizeLimit = 10 * 1024 * 1024 // 10MB

// githubWebhookHandler handles webhooks from GitHub
func (s *Server) githubWebhookHandler(c *gin.Context) {
	// Get event type from header
	eventType := c.GetHeader("X-GitHub-Event")
	if eventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-GitHub-Event header"})
		return
	}

	// Read and validate the payload
	payload, err := s.readAndValidateWebhookPayload(c, "X-Hub-Signature-256", s.config.Webhooks.GitHub.Secret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get GitHub adapter
	adapter, err := s.engine.GetAdapter("github")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GitHub adapter not configured"})
		return
	}

	// Forward to adapter for processing
	githubAdapter, ok := adapter.(*github.Adapter)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid GitHub adapter type"})
		return
	}

	if err := githubAdapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// harnessWebhookHandler handles webhooks from Harness
func (s *Server) harnessWebhookHandler(c *gin.Context) {
	// Get event type from header or query parameter
	eventType := c.GetHeader("X-Harness-Event")
	if eventType == "" {
		eventType = c.Query("eventType")
		if eventType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing event type"})
			return
		}
	}

	// Read and validate the payload
	payload, err := s.readAndValidateWebhookPayload(c, "X-Harness-Signature", s.config.Webhooks.Harness.Secret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get Harness adapter
	adapter, err := s.engine.GetAdapter("harness")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Harness adapter not configured"})
		return
	}

	// Forward to adapter for processing
	harnessAdapter, ok := adapter.(*harness.Adapter)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Harness adapter type"})
		return
	}

	if err := harnessAdapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// sonarqubeWebhookHandler handles webhooks from SonarQube
func (s *Server) sonarqubeWebhookHandler(c *gin.Context) {
	// SonarQube doesn't have explicit event types in headers
	// The event type will be determined from the payload

	// Read and validate the payload
	payload, err := s.readAndValidateWebhookPayload(c, "X-SonarQube-Signature", s.config.Webhooks.SonarQube.Secret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get SonarQube adapter
	adapter, err := s.engine.GetAdapter("sonarqube")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "SonarQube adapter not configured"})
		return
	}

	// Forward to adapter for processing
	sonarqubeAdapter, ok := adapter.(*sonarqube.Adapter)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid SonarQube adapter type"})
		return
	}

	if err := sonarqubeAdapter.HandleWebhook(c.Request.Context(), "", payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// artifactoryWebhookHandler handles webhooks from JFrog Artifactory
func (s *Server) artifactoryWebhookHandler(c *gin.Context) {
	// Get event type from header or query parameter
	eventType := c.GetHeader("X-JFrog-Event-Type")
	if eventType == "" {
		eventType = "unknown"
	}

	// Read and validate the payload
	payload, err := s.readAndValidateWebhookPayload(c, "X-JFrog-Signature", s.config.Webhooks.Artifactory.Secret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get Artifactory adapter
	adapter, err := s.engine.GetAdapter("artifactory")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Artifactory adapter not configured"})
		return
	}

	// Forward to adapter for processing
	artifactoryAdapter, ok := adapter.(*artifactory.Adapter)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Artifactory adapter type"})
		return
	}

	if err := artifactoryAdapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// xrayWebhookHandler handles webhooks from JFrog Xray
func (s *Server) xrayWebhookHandler(c *gin.Context) {
	// Get event type from header or query parameter
	eventType := c.GetHeader("X-JFrog-Event-Type")
	if eventType == "" {
		eventType = "unknown"
	}

	// Read and validate the payload
	payload, err := s.readAndValidateWebhookPayload(c, "X-JFrog-Signature", s.config.Webhooks.Xray.Secret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get Xray adapter
	adapter, err := s.engine.GetAdapter("xray")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xray adapter not configured"})
		return
	}

	// Forward to adapter for processing
	xrayAdapter, ok := adapter.(*xray.Adapter)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Xray adapter type"})
		return
	}

	if err := xrayAdapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readAndValidateWebhookPayload reads the request body and validates the webhook signature
func (s *Server) readAndValidateWebhookPayload(c *gin.Context, signatureHeader, secret string) ([]byte, error) {
	// Limit the size of the request body
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, webhookPayloadSizeLimit)

	// Read the request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Reset the request body for potential future middleware
	c.Request.Body = io.NopCloser(bytes.NewBuffer(payload))

	// Skip signature validation if no secret is configured
	if secret == "" {
		return payload, nil
	}

	// Get the signature from the header
	signature := c.GetHeader(signatureHeader)
	if signature == "" {
		return nil, fmt.Errorf("missing %s header", signatureHeader)
	}

	// Validate the signature
	if err := validateSignature(payload, signature, secret, signatureHeader); err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	return payload, nil
}

// validateSignature validates the webhook signature
func validateSignature(payload []byte, signature, secret, signatureHeader string) error {
	// Different signature formats based on the service
	switch signatureHeader {
	case "X-Hub-Signature-256":
		// GitHub uses "sha256=<hex>"
		if len(signature) <= 7 || signature[:7] != "sha256=" {
			return fmt.Errorf("invalid signature format")
		}
		signature = signature[7:]

		// Calculate expected signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Compare signatures
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			return fmt.Errorf("signature mismatch")
		}

	case "X-Harness-Signature", "X-SonarQube-Signature", "X-JFrog-Signature":
		// Assume they use similar format to GitHub
		// Calculate expected signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Compare signatures
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			return fmt.Errorf("signature mismatch")
		}

	default:
		return fmt.Errorf("unsupported signature header: %s", signatureHeader)
	}

	return nil
}
