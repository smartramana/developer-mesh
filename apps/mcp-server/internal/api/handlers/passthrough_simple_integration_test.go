package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
)

// TestPassthroughAuthenticationE2E performs end-to-end testing of pass-through authentication
func TestPassthroughAuthenticationE2E(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Track authentication methods used
	authMethodsUsed := make(map[string]int)
	var authMu sync.Mutex

	// Create logger
	logger := observability.NewLogger("test")

	// Create test router
	router := gin.New()

	// Add credential extraction middleware
	router.Use(auth.CredentialExtractionMiddleware(logger))

	// Add service account configuration middleware
	router.Use(func(c *gin.Context) {
		// Simulate service account availability
		c.Set("service_account_fallback_enabled", true)
		c.Set("available_service_accounts", map[string]bool{
			"github": true,
			"jira":   true,
		})
		c.Next()
	})

	// Add authentication tracking middleware
	router.Use(func(c *gin.Context) {
		// Track which authentication method is used
		creds, hasCreds := auth.GetToolCredentials(c.Request.Context())

		authMu.Lock()
		if hasCreds && creds != nil && creds.GitHub != nil {
			authMethodsUsed["user_credential"]++
			c.Set("auth_method", "user_credential")
			c.Set("auth_token_hint", fmt.Sprintf("%.4s...%.4s",
				creds.GitHub.Token[:4],
				creds.GitHub.Token[len(creds.GitHub.Token)-4:]))
		} else {
			authMethodsUsed["service_account"]++
			c.Set("auth_method", "service_account")
		}
		authMu.Unlock()

		c.Next()
	})

	// Tool action handler
	router.POST("/api/v1/tools/:tool/actions/:action", func(c *gin.Context) {
		tool := c.Param("tool")
		action := c.Param("action")
		authMethod, _ := c.Get("auth_method")
		tokenHint, _ := c.Get("auth_token_hint")

		// Simulate tool execution
		response := gin.H{
			"success": true,
			"tool":    tool,
			"action":  action,
			"auth": gin.H{
				"method": authMethod,
			},
			"result": gin.H{
				"message": fmt.Sprintf("Executed %s:%s successfully", tool, action),
				"time":    time.Now().Format(time.RFC3339),
			},
		}

		if tokenHint != nil {
			response["auth"].(gin.H)["token_hint"] = tokenHint
		}

		c.JSON(http.StatusOK, response)
	})

	// Test scenarios
	tests := []struct {
		name               string
		tool               string
		action             string
		includeCredentials bool
		credentialToken    string
		expectedAuthMethod string
		expectedStatus     int
	}{
		{
			name:               "User credentials passed through",
			tool:               "github",
			action:             "create_issue",
			includeCredentials: true,
			credentialToken:    "ghp_usertoken123456789",
			expectedAuthMethod: "user_credential",
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "Service account fallback",
			tool:               "github",
			action:             "list_repos",
			includeCredentials: false,
			expectedAuthMethod: "service_account",
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "OAuth token pass-through",
			tool:               "github",
			action:             "create_pr",
			includeCredentials: true,
			credentialToken:    "gho_oauthtoken987654321",
			expectedAuthMethod: "user_credential",
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "Multiple tool credentials",
			tool:               "github",
			action:             "sync_with_jira",
			includeCredentials: true,
			credentialToken:    "ghp_multitoken111222333",
			expectedAuthMethod: "user_credential",
			expectedStatus:     http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare request body
			body := map[string]interface{}{
				"parameters": map[string]interface{}{
					"test_param": "test_value",
				},
			}

			if tt.includeCredentials {
				body["credentials"] = map[string]interface{}{
					"github": map[string]interface{}{
						"token": tt.credentialToken,
						"type":  "pat",
					},
				}

				// Add Jira credentials for multi-tool test
				if tt.action == "sync_with_jira" {
					body["credentials"].(map[string]interface{})["jira"] = map[string]interface{}{
						"token":    "jira_token_456",
						"type":     "basic",
						"username": "test@example.com",
					}
				}
			}

			// Create request
			bodyBytes, _ := json.Marshal(body)
			req := httptest.NewRequest("POST",
				fmt.Sprintf("/api/v1/tools/%s/actions/%s", tt.tool, tt.action),
				bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-api-key")

			// Execute request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s",
					tt.expectedStatus, w.Code, w.Body.String())
			}

			// Parse response
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Verify authentication method
			if auth, ok := response["auth"].(map[string]interface{}); ok {
				if method, ok := auth["method"].(string); ok {
					if method != tt.expectedAuthMethod {
						t.Errorf("Expected auth method %s, got %s",
							tt.expectedAuthMethod, method)
					}

					// Verify token hint for user credentials
					if tt.expectedAuthMethod == "user_credential" {
						if tokenHint, ok := auth["token_hint"].(string); ok {
							t.Logf("Token hint: %s", tokenHint)
						}
					}
				}
			}

			// Verify result
			if result, ok := response["result"].(map[string]interface{}); ok {
				if msg, ok := result["message"].(string); ok {
					expectedMsg := fmt.Sprintf("Executed %s:%s successfully", tt.tool, tt.action)
					if msg != expectedMsg {
						t.Errorf("Expected message %s, got %s", expectedMsg, msg)
					}
				}
			}
		})
	}

	// Verify metrics
	t.Run("Verify authentication metrics", func(t *testing.T) {
		authMu.Lock()
		defer authMu.Unlock()

		// Should have both user credentials and service account usage
		if authMethodsUsed["user_credential"] == 0 {
			t.Error("Expected user_credential to be used at least once")
		}
		if authMethodsUsed["service_account"] == 0 {
			t.Error("Expected service_account to be used at least once")
		}

		t.Logf("Authentication methods used: %+v", authMethodsUsed)
	})
}

// TestCredentialPropagationThroughContext tests credential propagation
func TestCredentialPropagationThroughContext(t *testing.T) {
	// Test credential storage and retrieval
	tests := []struct {
		name        string
		credentials *models.ToolCredentials
		tool        string
		shouldExist bool
	}{
		{
			name: "GitHub credentials",
			credentials: &models.ToolCredentials{
				GitHub: &models.TokenCredential{
					Token: "ghp_test123",
					Type:  "pat",
				},
			},
			tool:        "github",
			shouldExist: true,
		},
		{
			name: "Multiple tool credentials",
			credentials: &models.ToolCredentials{
				GitHub: &models.TokenCredential{
					Token: "ghp_github123",
					Type:  "pat",
				},
				Jira: &models.TokenCredential{
					Token:    "jira_token",
					Type:     "basic",
					Username: "user@example.com",
				},
			},
			tool:        "jira",
			shouldExist: true,
		},
		{
			name: "Non-existent tool",
			credentials: &models.ToolCredentials{
				GitHub: &models.TokenCredential{
					Token: "ghp_test",
					Type:  "pat",
				},
			},
			tool:        "gitlab",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with credentials
			ctx := context.Background()
			ctx = auth.WithToolCredentials(ctx, tt.credentials)

			// Test retrieval
			creds, ok := auth.GetToolCredentials(ctx)
			if !ok || creds == nil {
				t.Fatal("Failed to retrieve credentials from context")
			}

			// Test specific tool
			toolCred, exists := auth.GetToolCredential(ctx, tt.tool)
			if exists != tt.shouldExist {
				t.Errorf("Expected tool %s existence to be %v, got %v",
					tt.tool, tt.shouldExist, exists)
			}

			if exists && toolCred == nil {
				t.Error("Tool credential exists but is nil")
			}

			// Test HasToolCredential
			if auth.HasToolCredential(ctx, tt.tool) != tt.shouldExist {
				t.Errorf("HasToolCredential returned incorrect result for %s", tt.tool)
			}
		})
	}
}

// TestConcurrentCredentialAccess tests thread-safe credential access
func TestConcurrentCredentialAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := observability.NewLogger("test")

	// Create router with middleware
	router := gin.New()
	router.Use(auth.CredentialExtractionMiddleware(logger))

	// Track concurrent access
	accessCount := make(map[string]int)
	var mu sync.Mutex

	// Handler that simulates concurrent credential access
	router.POST("/api/v1/tools/github/actions/test", func(c *gin.Context) {
		// Local waitgroup for this request
		var localWg sync.WaitGroup

		// Access credentials multiple times
		for i := 0; i < 10; i++ {
			localWg.Add(1)
			go func(idx int) {
				defer localWg.Done()

				creds, ok := auth.GetToolCredentials(c.Request.Context())
				if ok && creds != nil && creds.GitHub != nil {
					mu.Lock()
					accessCount[creds.GitHub.Token]++
					mu.Unlock()
				}
			}(i)
		}

		localWg.Wait()
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Send multiple concurrent requests
	numRequests := 5
	var requestWg sync.WaitGroup

	for i := 0; i < numRequests; i++ {
		requestWg.Add(1)
		go func(idx int) {
			defer requestWg.Done()

			body := map[string]interface{}{
				"credentials": map[string]interface{}{
					"github": map[string]interface{}{
						"token": fmt.Sprintf("token_%d", idx),
						"type":  "pat",
					},
				},
			}

			bodyBytes, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/api/v1/tools/github/actions/test",
				bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Request %d failed with status %d", idx, w.Code)
			}
		}(i)
	}

	requestWg.Wait()

	// Verify all tokens were accessed correctly
	mu.Lock()
	defer mu.Unlock()

	for token, count := range accessCount {
		if count != 10 { // Each request should access 10 times
			t.Errorf("Token %s was accessed %d times, expected 10", token, count)
		}
	}
}

// TestErrorScenarios tests various error conditions
func TestErrorScenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := observability.NewLogger("test")

	tests := []struct {
		name           string
		setupRouter    func() *gin.Engine
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Invalid JSON in request",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				router.Use(auth.CredentialExtractionMiddleware(logger))
				router.POST("/api/v1/tools/github/actions/test", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"success": true})
				})
				return router
			},
			requestBody:    `{invalid json`,
			expectedStatus: http.StatusOK, // Middleware continues on parse error
		},
		{
			name: "Missing required credentials with validation",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				router.Use(auth.CredentialExtractionMiddleware(logger))
				router.Use(auth.CredentialValidationMiddleware([]string{"github", "jira"}, logger))
				router.POST("/api/v1/tools/multi/actions/sync", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"success": true})
				})
				return router
			},
			requestBody: map[string]interface{}{
				"credentials": map[string]interface{}{
					"github": map[string]interface{}{
						"token": "github-token",
						"type":  "pat",
					},
					// Missing Jira credentials
				},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Missing credentials for required tools",
		},
		{
			name: "Empty credential token",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				router.Use(auth.CredentialExtractionMiddleware(logger))
				router.Use(auth.CredentialValidationMiddleware([]string{"github"}, logger))
				router.POST("/api/v1/tools/github/actions/test", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"success": true})
				})
				return router
			},
			requestBody: map[string]interface{}{
				"credentials": map[string]interface{}{
					"github": map[string]interface{}{
						"token": "", // Empty token
						"type":  "pat",
					},
				},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Missing required credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := tt.setupRouter()

			// Create request
			var bodyReader *bytes.Reader
			if bodyStr, ok := tt.requestBody.(string); ok {
				bodyReader = bytes.NewReader([]byte(bodyStr))
			} else {
				bodyBytes, _ := json.Marshal(tt.requestBody)
				bodyReader = bytes.NewReader(bodyBytes)
			}

			req := httptest.NewRequest("POST", "/api/v1/tools/github/actions/test", bodyReader)
			if tt.name != "Invalid JSON in request" {
				req = httptest.NewRequest("POST", "/api/v1/tools/multi/actions/sync", bodyReader)
			}
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s",
					tt.expectedStatus, w.Code, w.Body.String())
			}

			// Check error message if expected
			if tt.expectedError != "" {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
					if errMsg, ok := response["error"].(string); ok {
						if errMsg != tt.expectedError {
							t.Errorf("Expected error '%s', got '%s'",
								tt.expectedError, errMsg)
						}
					}
				}
			}
		})
	}
}
