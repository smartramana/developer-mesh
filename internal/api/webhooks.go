package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
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

	// Extract agent ID from the payload, headers, or query parameters
	agentID := c.Query("agent_id")
	if agentID == "" {
		// If not in query, try to get from custom header
		agentID = c.GetHeader("X-Agent-ID")
		
		// If still not found, use a default or extract from payload if possible
		if agentID == "" {
			// Try to extract repository name as a fallback
			var payloadMap map[string]interface{}
			if err := json.Unmarshal(payload, &payloadMap); err == nil {
				if repo, ok := payloadMap["repository"].(map[string]interface{}); ok {
					if fullName, ok := repo["full_name"].(string); ok {
						agentID = "github-" + fullName
					}
				}
			}
			
			// Final fallback
			if agentID == "" {
				agentID = "github-default"
			}
		}
	}

	// Forward to adapter for processing
	if err := adapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
		return
	}

	// Record in context if agent ID is provided
	if agentID != "" {
		contextHelper := adapters.NewContextAwareAdapter(s.engine.ContextManager, "github")
		contextID, err := contextHelper.RecordWebhookInContext(c.Request.Context(), agentID, eventType, payload)
		if err != nil {
			// Log the error but don't fail the request
			log.Printf("Warning: Failed to record GitHub webhook in context: %v", err)
		} else {
			// Return the context ID in the response
			c.JSON(http.StatusOK, gin.H{"status": "ok", "context_id": contextID})
			return
		}
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
	if err := adapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
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
	if err := adapter.HandleWebhook(c.Request.Context(), "", payload); err != nil {
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
	if err := adapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
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
	if err := adapter.HandleWebhook(c.Request.Context(), eventType, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readAndValidateWebhookPayload reads the request body and validates the webhook signature
func (s *Server) readAndValidateWebhookPayload(c *gin.Context, signatureHeader, secret string) ([]byte, error) {
	// Check for proper content type
	contentType := c.GetHeader("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		return nil, fmt.Errorf("invalid Content-Type: expected application/json, got %s", contentType)
	}

	// Limit the size of the request body
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, webhookPayloadSizeLimit)

	// Read the request body with a timeout
	bodyReader := io.LimitReader(c.Request.Body, webhookPayloadSizeLimit)
	
	// Create a context with timeout for reading
	readCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	
	// Use a channel to handle the read operation with timeout
	readCh := make(chan struct {
		payload []byte
		err     error
	})
	
	go func() {
		payload, err := io.ReadAll(bodyReader)
		readCh <- struct {
			payload []byte
			err     error
		}{payload, err}
	}()
	
	// Wait for read completion or timeout
	var payload []byte
	var err error
	select {
	case <-readCtx.Done():
		return nil, fmt.Errorf("request body read timed out")
	case result := <-readCh:
		payload = result.payload
		err = result.err
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Reset the request body for potential future middleware
	c.Request.Body = io.NopCloser(bytes.NewBuffer(payload))

	// Require signature validation for all webhook endpoints
	if secret == "" {
		return nil, fmt.Errorf("webhook secret not configured")
	}

	// Get the signature from the header
	signature := c.GetHeader(signatureHeader)
	if signature == "" {
		return nil, fmt.Errorf("missing %s header", signatureHeader)
	}

	// Validate the signature
	if err := validateSignature(payload, signature, secret, signatureHeader); err != nil {
		// Delay response on invalid signature to prevent timing attacks
		time.Sleep(time.Duration(50+rand.Intn(150)) * time.Millisecond)
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	return payload, nil
}

// validateSignature validates the webhook signature
func validateSignature(payload []byte, signature, secret, signatureHeader string) error {
	if secret == "" {
		return fmt.Errorf("webhook secret is not configured")
	}

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

		// Use constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			return fmt.Errorf("signature mismatch")
		}

	case "X-Harness-Signature":
		// Harness-specific signature validation
		// Calculate expected signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Use constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			return fmt.Errorf("signature mismatch")
		}

	case "X-SonarQube-Signature":
		// SonarQube-specific signature validation 
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Use constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			return fmt.Errorf("signature mismatch")
		}

	case "X-JFrog-Signature":
		// JFrog-specific signature validation
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Use constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			return fmt.Errorf("signature mismatch")
		}

	default:
		return fmt.Errorf("unsupported signature header: %s", signatureHeader)
	}

	return nil
}
