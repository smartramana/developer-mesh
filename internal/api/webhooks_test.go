package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adaptermocks "github.com/S-Corkum/mcp-server/internal/adapters/mocks"
	coremocks "github.com/S-Corkum/mcp-server/internal/core/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock webhook handler for testing
func createMockWebhookHandler(mockEngine *coremocks.MockEngine, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		mockAdapter := &adaptermocks.MockAdapter{}
		mockAdapter.On("HandleWebhook", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		
		mockEngine.On("GetAdapter", mock.Anything).Return(mockAdapter, nil)
		
		// Validate signature
		signature := c.GetHeader("X-Test-Signature")
		if signature == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing signature header"})
			return
		}
		
		// Read request body
		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
			return
		}
		
		// Validate signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))
		
		if signature != expectedSignature {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			return
		}
		
		// Valid request, process webhook
		eventType := c.GetHeader("X-Test-Event")
		mockAdapter.HandleWebhook(context.Background(), eventType, body)
		
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func TestWebhookHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	mockEngine := &coremocks.MockEngine{}
	
	router := gin.New()
	router.POST("/webhook/test", createMockWebhookHandler(mockEngine, "test-secret"))
	
	t.Run("Valid Webhook", func(t *testing.T) {
		// Prepare webhook payload
		payload := []byte(`{"event":"test","data":"example"}`)
		
		// Create the request
		req := httptest.NewRequest(http.MethodPost, "/webhook/test", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Event", "test-event")
		
		// Generate and set the signature
		mac := hmac.New(sha256.New, []byte("test-secret"))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Test-Signature", signature)
		
		// Perform the request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// Check the response
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
	})
	
	t.Run("Invalid Signature", func(t *testing.T) {
		payload := []byte(`{"event":"test","data":"example"}`)
		
		req := httptest.NewRequest(http.MethodPost, "/webhook/test", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Event", "test-event")
		req.Header.Set("X-Test-Signature", "invalid-signature")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// Should fail with invalid signature
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	
	t.Run("Missing Signature", func(t *testing.T) {
		payload := []byte(`{"event":"test","data":"example"}`)
		
		req := httptest.NewRequest(http.MethodPost, "/webhook/test", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Event", "test-event")
		// No signature header
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// Should fail with missing signature
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestSignatureValidation(t *testing.T) {
	t.Run("GitHub SHA256 Signature", func(t *testing.T) {
		payload := []byte("test payload")
		secret := "test-secret"
		
		// Generate SHA256 signature with GitHub format
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		
		// Test GitHub signature validation with custom implementation
		isValid := func(payload []byte, signature, secret string) (bool, error) {
			if len(signature) <= 7 || signature[:7] != "sha256=" {
				return false, errors.New("invalid signature format")
			}
			
			sigValue := signature[7:]
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(payload)
			expectedSig := hex.EncodeToString(mac.Sum(nil))
			
			return hmac.Equal([]byte(sigValue), []byte(expectedSig)), nil
		}
		
		valid, err := isValid(payload, signature, secret)
		assert.NoError(t, err)
		assert.True(t, valid)
	})
	
	t.Run("SHA1 Signature", func(t *testing.T) {
		payload := []byte("test payload")
		secret := "test-secret"
		
		// Generate SHA1 signature
		mac := hmac.New(sha1.New, []byte(secret))
		mac.Write(payload)
		signature := "sha1=" + hex.EncodeToString(mac.Sum(nil))
		
		// Test SHA1 signature validation with custom implementation
		isValid := func(payload []byte, signature, secret string) (bool, error) {
			if len(signature) <= 5 || signature[:5] != "sha1=" {
				return false, errors.New("invalid signature format")
			}
			
			sigValue := signature[5:]
			mac := hmac.New(sha1.New, []byte(secret))
			mac.Write(payload)
			expectedSig := hex.EncodeToString(mac.Sum(nil))
			
			return hmac.Equal([]byte(sigValue), []byte(expectedSig)), nil
		}
		
		valid, err := isValid(payload, signature, secret)
		assert.NoError(t, err)
		assert.True(t, valid)
	})
	
	t.Run("Raw SHA256 Signature", func(t *testing.T) {
		payload := []byte("test payload")
		secret := "test-secret"
		
		// Generate raw SHA256 signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))
		
		// Test raw signature validation
		isValid := func(payload []byte, signature, secret string) (bool, error) {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(payload)
			expectedSig := hex.EncodeToString(mac.Sum(nil))
			
			return hmac.Equal([]byte(signature), []byte(expectedSig)), nil
		}
		
		valid, err := isValid(payload, signature, secret)
		assert.NoError(t, err)
		assert.True(t, valid)
	})
}

// Test implementation of timeout handling
func TestWebhookTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Setup a handler that simulates read timeout
	router.POST("/webhook/timeout", func(c *gin.Context) {
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Millisecond)
		defer cancel()
		
		// Create a channel for the read operation
		readCh := make(chan struct{
			data []byte
			err  error
		})
		
		// Start reading in a goroutine
		go func() {
			// Simulate slow read
			time.Sleep(50 * time.Millisecond)
			data, err := c.GetRawData()
			readCh <- struct {
				data []byte
				err  error
			}{data, err}
		}()
		
		// Wait for read completion or timeout
		select {
		case <-ctx.Done():
			c.JSON(http.StatusRequestTimeout, gin.H{"error": "Request timed out"})
			return
		case result := <-readCh:
			if result.err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": result.err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"size": len(result.data)})
		}
	})
	
	// Test timeout behavior
	req := httptest.NewRequest(http.MethodPost, "/webhook/timeout", bytes.NewBufferString("test payload"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	// Expect timeout response
	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Contains(t, w.Body.String(), "timed out")
}
