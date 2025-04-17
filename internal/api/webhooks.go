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
	"strings"
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

// GetHarnessWebhookURL returns the webhook URL for Harness.io integration
func (s *Server) GetHarnessWebhookURL(accountID, webhookID string) (string, error) {
	// Simplified version for testing
	baseURL := s.config.Webhooks.Harness.BaseURL
	if baseURL == "" {
		baseURL = "https://harness.io"
	}
	
	path := s.config.Webhooks.Harness.WebhookPath
	if path == "" {
		path = "ng/api/webhook"
	}
	
	// Use provided or default account ID
	if accountID == "" {
		accountID = s.config.Webhooks.Harness.AccountID
	}
	
	// Use provided or default webhook ID
	if webhookID == "" {
		webhookID = "devopsmcp"
	}
	
	// Generate URL
	webhookURL := baseURL
	if !strings.HasSuffix(webhookURL, "/") {
		webhookURL += "/"
	}
	
	// Ensure path doesn't start with / if we already added one
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	webhookURL += path
	
	// Add query parameters
	params := make([]string, 0)
	if accountID != "" {
		params = append(params, fmt.Sprintf("accountIdentifier=%s", accountID))
	}
	
	if webhookID != "" {
		params = append(params, fmt.Sprintf("webhookIdentifier=%s", webhookID))
	}
	
	// Append parameters as query string
	if len(params) > 0 {
		webhookURL += "?" + strings.Join(params, "&")
	}
	
	return webhookURL, nil
}

// getHarnessWebhookURLHandler provides the webhook URL for Harness.io integration
func (s *Server) getHarnessWebhookURLHandler(c *gin.Context) {
	accountID := c.Query("accountIdentifier")
	webhookID := c.Query("webhookIdentifier")
	
	webhookURL, err := s.GetHarnessWebhookURL(accountID, webhookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"webhook_url": webhookURL,
		"instructions": "Use this URL in your Harness.io webhook configuration",
	})
}

// harnessWebhookHandler handles webhooks from Harness
func (s *Server) harnessWebhookHandler(c *gin.Context) {
	// Get event type from header or query parameter
	// For generic webhooks, Harness might not send a specific event type header
	eventType := c.GetHeader("X-Harness-Event")
	if eventType == "" {
		eventType = c.Query("eventType")
		if eventType == "" {
			// For generic webhooks, check for common custom headers
			if triggerType := c.GetHeader("X-Harness-Trigger-Type"); triggerType != "" {
				eventType = triggerType
			} else if eventName := c.GetHeader("X-Event-Name"); eventName != "" {
				eventType = eventName
			} else {
				// Use a default event type and let the adapter determine specifics
				eventType = "generic_webhook"
				
				// Check for content type header to help identify the event type
				contentType := c.GetHeader("Content-Type")
				if contentType != "" && contentType != "application/json" {
					log.Printf("Unusual content type for Harness webhook: %s", contentType)
				}
			}
		}
	}
	
	// Extract Harness-specific identifiers from query parameters
	accountID := c.Query("accountIdentifier")
	webhookID := c.Query("webhookIdentifier")
	
	// If we have these identifiers, include them in the logged output for debugging
	if accountID != "" || webhookID != "" {
		log.Printf("Received Harness webhook from account: %s, webhook ID: %s, type: %s", 
			accountID, webhookID, eventType)
	}

	// Extract and map all headers to include in payload processing
	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Read and validate the payload
	// For generic webhooks, Harness supports HMAC authentication
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

	// Log receipt of webhook for debugging
	log.Printf("Received Harness webhook of type '%s'", eventType)

	// For generic webhooks, we need to include headers in the payload
	// Wrap the original payload if it's valid JSON
	var originalPayload map[string]interface{}
	if err := json.Unmarshal(payload, &originalPayload); err == nil {
		// Look for Event Relay format - if this is already wrapped from Harness Event Relay
		isEventRelay := false
		if _, hasHeaders := originalPayload["headers"]; hasHeaders {
			if payloadData, hasPayload := originalPayload["payload"]; hasPayload && payloadData != nil {
				isEventRelay = true
				log.Printf("Detected Event Relay format in webhook payload")
			}
		}
		
		if !isEventRelay {
			// Only wrap if we successfully parsed the JSON and it's not already wrapped
			wrappedPayload := map[string]interface{}{
				"headers": headers,
				"payload": originalPayload,
				"event_type": eventType,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			
			// Convert back to JSON for the adapter
			enhancedPayload, err := json.Marshal(wrappedPayload)
			if err == nil {
				payload = enhancedPayload
			}
		}
	}

	// Try to extract agent ID from the payload or headers for context tracking
	agentID := c.Query("agent_id")
	if agentID == "" {
		// If not in query, try to get from custom header
		agentID = c.GetHeader("X-Agent-ID")
		
		// If still not found, try to extract from payload
		if agentID == "" && originalPayload != nil {
			// Look for common identifiers in the payload
			if app, ok := originalPayload["application"]; ok && app != nil {
				if appName, ok := app.(string); ok {
					agentID = "harness-" + appName
				} else if appMap, ok := app.(map[string]interface{}); ok {
					if name, ok := appMap["name"].(string); ok {
						agentID = "harness-" + name
					}
				}
			} else if pipeline, ok := originalPayload["pipeline"]; ok && pipeline != nil {
				if pipelineID, ok := pipeline.(string); ok {
					agentID = "harness-pipeline-" + pipelineID
				} else if pipelineMap, ok := pipeline.(map[string]interface{}); ok {
					if id, ok := pipelineMap["id"].(string); ok {
						agentID = "harness-pipeline-" + id
					} else if name, ok := pipelineMap["name"].(string); ok {
						agentID = "harness-pipeline-" + name
					}
				}
			} else if execution, ok := originalPayload["execution"]; ok && execution != nil {
				if executionMap, ok := execution.(map[string]interface{}); ok {
					if id, ok := executionMap["id"].(string); ok {
						agentID = "harness-execution-" + id
					}
				}
			}
			
			// Final fallback
			if agentID == "" {
				// Generate a unique ID based on timestamp for default agent ID
				agentID = fmt.Sprintf("harness-webhook-%d", time.Now().UnixNano())
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
		contextHelper := adapters.NewContextAwareAdapter(s.engine.ContextManager, "harness")
		contextID, err := contextHelper.RecordWebhookInContext(c.Request.Context(), agentID, eventType, payload)
		if err != nil {
			// Log the error but don't fail the request
			log.Printf("Warning: Failed to record Harness webhook in context: %v", err)
		} else {
			// Return the context ID in the response
			c.JSON(http.StatusOK, gin.H{"status": "ok", "context_id": contextID, "agent_id": agentID})
			return
		}
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
		// Harness-specific signature validation for both regular and generic webhooks
		
		// For generic webhooks, signature might be in different formats:
		// 1. Plain hexadecimal (most common for generic webhooks)
		// 2. "sha256=<hex>" format (like GitHub)
		// 3. Other hash algorithm prefixes
		
		signatureValue := signature
		
		// If signature starts with "sha256=" or similar, extract the actual value
		if len(signature) > 7 && signature[:7] == "sha256=" {
			signatureValue = signature[7:]
		} else if len(signature) > 5 && signature[:5] == "sha1=" {
			signatureValue = signature[5:]
		}
		
		// Calculate expected signature with SHA-256 (default for Harness)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))
		
		// Use constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(signatureValue), []byte(expectedSignature)) {
			// For generic webhooks, try SHA-1 as a fallback since some integrations may use it
			mac = hmac.New(sha256.New, []byte(secret))
			mac.Write(payload)
			expectedSignatureSha1 := hex.EncodeToString(mac.Sum(nil))
			
			if !hmac.Equal([]byte(signatureValue), []byte(expectedSignatureSha1)) {
				// Log detailed error to help with debugging signature issues
				log.Printf("Signature mismatch for Harness webhook. Got: %s, Expected (SHA-256): %s", 
					signatureValue, expectedSignature)
				return fmt.Errorf("signature mismatch")
			}
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
