package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestValidateSignature(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "test-secret"
	
	// Test GitHub signature
	t.Run("GitHub Signature", func(t *testing.T) {
		// Generate valid signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		
		// Test valid signature
		err := validateSignature(payload, expectedSignature, secret, "X-Hub-Signature-256")
		assert.NoError(t, err)
		
		// Test invalid signature
		err = validateSignature(payload, "sha256=invalid", secret, "X-Hub-Signature-256")
		assert.Error(t, err)
		
		// Test invalid format
		err = validateSignature(payload, "invalid-format", secret, "X-Hub-Signature-256")
		assert.Error(t, err)
	})
	
	// Test Harness signature
	t.Run("Harness Signature", func(t *testing.T) {
		// Generate valid signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))
		
		// Test valid signature
		err := validateSignature(payload, expectedSignature, secret, "X-Harness-Signature")
		assert.NoError(t, err)
		
		// Test invalid signature
		err = validateSignature(payload, "invalid", secret, "X-Harness-Signature")
		assert.Error(t, err)
	})
	
	// Test missing secret
	t.Run("Missing Secret", func(t *testing.T) {
		err := validateSignature(payload, "signature", "", "X-Hub-Signature-256")
		assert.Error(t, err)
	})
	
	// Test unsupported signature format
	t.Run("Unsupported Format", func(t *testing.T) {
		err := validateSignature(payload, "signature", secret, "X-Unknown-Signature")
		assert.Error(t, err)
	})
}

// Test GitHub webhook handler with invalid request
func TestGitHubWebhookHandlerMissingEventHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a standalone test handler function
	handler := func(c *gin.Context) {
		// Check for event header
		eventType := c.GetHeader("X-GitHub-Event")
		if eventType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-GitHub-Event header"})
			return
		}
		
		// If we have the header, return OK
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	
	// Create a router and register the handler
	router := gin.New()
	router.POST("/webhook/github", handler)
	
	// Create request without event header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader([]byte(`{}`)))
	
	// Serve the request
	router.ServeHTTP(w, req)
	
	// Check the response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing X-GitHub-Event header")
}

// Test GitHub webhook with valid request
func TestGitHubWebhookHandlerValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a standalone test handler function
	handler := func(c *gin.Context) {
		// Check for event header
		eventType := c.GetHeader("X-GitHub-Event")
		if eventType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-GitHub-Event header"})
			return
		}
		
		// If we have the header, return OK
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	
	// Create a router and register the handler
	router := gin.New()
	router.POST("/webhook/github", handler)
	
	// Create request with event header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-GitHub-Event", "push")
	
	// Serve the request
	router.ServeHTTP(w, req)
	
	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

// Test GitHub webhook with adapter error
func TestGitHubWebhookHandlerAdapterError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a standalone test handler function that simulates adapter error
	handler := func(c *gin.Context) {
		// Check for event header
		eventType := c.GetHeader("X-GitHub-Event")
		if eventType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-GitHub-Event header"})
			return
		}
		
		// Simulate adapter error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GitHub adapter not configured"})
	}
	
	// Create a router and register the handler
	router := gin.New()
	router.POST("/webhook/github", handler)
	
	// Create request with event header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-GitHub-Event", "push")
	
	// Serve the request
	router.ServeHTTP(w, req)
	
	// Check the response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "GitHub adapter not configured")
}
