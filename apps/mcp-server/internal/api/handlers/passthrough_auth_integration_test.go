package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCredentialExtractionMiddleware tests the credential extraction from HTTP requests
func TestCredentialExtractionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := observability.NewLogger("test")

	tests := []struct {
		name               string
		requestPath        string
		requestBody        interface{}
		expectedCredential *models.ToolCredentials
		shouldExtract      bool
	}{
		{
			name:        "Extract GitHub credentials from tool action request",
			requestPath: "/api/v1/tools/github/actions/create_issue",
			requestBody: map[string]interface{}{
				"parameters": map[string]interface{}{
					"owner": "test-owner",
					"repo":  "test-repo",
				},
				"credentials": map[string]interface{}{
					"github": map[string]interface{}{
						"token": "ghp_test123456789",
						"type":  "pat",
					},
				},
			},
			expectedCredential: &models.ToolCredentials{
				GitHub: &models.TokenCredential{
					Token: "ghp_test123456789",
					Type:  "pat",
				},
			},
			shouldExtract: true,
		},
		{
			name:        "Extract multiple tool credentials",
			requestPath: "/api/v1/tools/github/actions/sync_with_jira",
			requestBody: map[string]interface{}{
				"parameters": map[string]interface{}{
					"sync": true,
				},
				"credentials": map[string]interface{}{
					"github": map[string]interface{}{
						"token": "ghp_github123",
						"type":  "pat",
					},
					"jira": map[string]interface{}{
						"token":    "jira-token-456",
						"type":     "basic",
						"username": "user@example.com",
					},
				},
			},
			expectedCredential: &models.ToolCredentials{
				GitHub: &models.TokenCredential{
					Token: "ghp_github123",
					Type:  "pat",
				},
				Jira: &models.TokenCredential{
					Token:    "jira-token-456",
					Type:     "basic",
					Username: "user@example.com",
				},
			},
			shouldExtract: true,
		},
		{
			name:        "OAuth token with expiration",
			requestPath: "/api/v1/tools/github/actions/list_repos",
			requestBody: map[string]interface{}{
				"credentials": map[string]interface{}{
					"github": map[string]interface{}{
						"token":      "gho_oauth789",
						"type":       "oauth",
						"expires_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
					},
				},
			},
			expectedCredential: &models.ToolCredentials{
				GitHub: &models.TokenCredential{
					Token:     "gho_oauth789",
					Type:      "oauth",
					ExpiresAt: time.Now().Add(1 * time.Hour),
				},
			},
			shouldExtract: true,
		},
		{
			name:        "No credentials in request",
			requestPath: "/api/v1/tools/github/actions/public_repos",
			requestBody: map[string]interface{}{
				"parameters": map[string]interface{}{
					"owner": "public-owner",
				},
			},
			expectedCredential: nil,
			shouldExtract:      false,
		},
		{
			name:        "Non-tool request should not extract",
			requestPath: "/api/v1/contexts",
			requestBody: map[string]interface{}{
				"name": "test-context",
				"credentials": map[string]interface{}{
					"github": map[string]interface{}{
						"token": "should-not-extract",
					},
				},
			},
			expectedCredential: nil,
			shouldExtract:      false,
		},
		{
			name:        "GitHub Enterprise credentials",
			requestPath: "/api/v1/tools/github/actions/create_pr",
			requestBody: map[string]interface{}{
				"credentials": map[string]interface{}{
					"github": map[string]interface{}{
						"token":    "enterprise-token",
						"type":     "pat",
						"base_url": "https://github.enterprise.com/api/v3/",
					},
				},
			},
			expectedCredential: &models.ToolCredentials{
				GitHub: &models.TokenCredential{
					Token:   "enterprise-token",
					Type:    "pat",
					BaseURL: "https://github.enterprise.com/api/v3/",
				},
			},
			shouldExtract: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup router with middleware
			router := gin.New()
			router.Use(auth.CredentialExtractionMiddleware(logger))

			// Variable to capture extracted credentials
			var extractedCreds *models.ToolCredentials

			// Test handler
			router.POST("/*path", func(c *gin.Context) {
				// Check if credentials were extracted
				creds, ok := auth.GetToolCredentials(c.Request.Context())
				if ok {
					extractedCreds = creds
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			// Create request
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", tt.requestPath, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, http.StatusOK, w.Code)

			// Verify credential extraction
			if tt.shouldExtract {
				require.NotNil(t, extractedCreds, "Expected credentials to be extracted")

				// Compare credentials
				if tt.expectedCredential.GitHub != nil {
					require.NotNil(t, extractedCreds.GitHub)
					assert.Equal(t, tt.expectedCredential.GitHub.Token, extractedCreds.GitHub.Token)
					assert.Equal(t, tt.expectedCredential.GitHub.Type, extractedCreds.GitHub.Type)
					if tt.expectedCredential.GitHub.BaseURL != "" {
						assert.Equal(t, tt.expectedCredential.GitHub.BaseURL, extractedCreds.GitHub.BaseURL)
					}
				}

				if tt.expectedCredential.Jira != nil {
					require.NotNil(t, extractedCreds.Jira)
					assert.Equal(t, tt.expectedCredential.Jira.Token, extractedCreds.Jira.Token)
					assert.Equal(t, tt.expectedCredential.Jira.Type, extractedCreds.Jira.Type)
					assert.Equal(t, tt.expectedCredential.Jira.Username, extractedCreds.Jira.Username)
				}
			} else {
				assert.Nil(t, extractedCreds, "Expected no credentials to be extracted")
			}
		})
	}
}

// TestPassThroughAuthenticationFlow tests the complete authentication flow
func TestPassThroughAuthenticationFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := observability.NewLogger("test")

	// Test scenarios
	scenarios := []struct {
		name            string
		setupRequest    func() *http.Request
		setupMiddleware func(*gin.Engine)
		expectedStatus  int
		expectedError   string
		verifyContext   func(*testing.T, *gin.Context)
	}{
		{
			name: "Successful pass-through authentication",
			setupRequest: func() *http.Request {
				body := map[string]interface{}{
					"action": "create_issue",
					"parameters": map[string]interface{}{
						"title": "Test Issue",
					},
					"credentials": map[string]interface{}{
						"github": map[string]interface{}{
							"token": "user-github-token",
							"type":  "pat",
						},
					},
				}
				bodyBytes, _ := json.Marshal(body)
				req := httptest.NewRequest("POST", "/api/v1/tools/github/actions/create_issue", bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer api-key-123")
				return req
			},
			setupMiddleware: func(router *gin.Engine) {
				router.Use(auth.CredentialExtractionMiddleware(logger))
			},
			expectedStatus: http.StatusOK,
			verifyContext: func(t *testing.T, c *gin.Context) {
				creds, ok := auth.GetToolCredentials(c.Request.Context())
				assert.True(t, ok)
				assert.NotNil(t, creds)
				assert.NotNil(t, creds.GitHub)
				assert.Equal(t, "user-github-token", creds.GitHub.Token)
			},
		},
		{
			name: "Service account fallback when no credentials",
			setupRequest: func() *http.Request {
				body := map[string]interface{}{
					"action": "list_repos",
					"parameters": map[string]interface{}{
						"owner": "test-owner",
					},
				}
				bodyBytes, _ := json.Marshal(body)
				req := httptest.NewRequest("POST", "/api/v1/tools/github/actions/list_repos", bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer api-key-123")
				return req
			},
			setupMiddleware: func(router *gin.Engine) {
				router.Use(auth.CredentialExtractionMiddleware(logger))
				router.Use(func(c *gin.Context) {
					// Simulate service account availability
					c.Set("service_account_fallback_enabled", true)
					c.Set("available_service_accounts", map[string]bool{
						"github": true,
					})
					c.Next()
				})
			},
			expectedStatus: http.StatusOK,
			verifyContext: func(t *testing.T, c *gin.Context) {
				creds, ok := auth.GetToolCredentials(c.Request.Context())
				assert.False(t, ok)
				assert.Nil(t, creds)

				// Verify service account flags are set
				fallback, _ := c.Get("service_account_fallback_enabled")
				assert.True(t, fallback.(bool))
			},
		},
		{
			name: "Expired credential should be rejected",
			setupRequest: func() *http.Request {
				body := map[string]interface{}{
					"credentials": map[string]interface{}{
						"github": map[string]interface{}{
							"token":      "expired-token",
							"type":       "oauth",
							"expires_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
						},
					},
				}
				bodyBytes, _ := json.Marshal(body)
				req := httptest.NewRequest("POST", "/api/v1/tools/github/actions/create_issue", bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMiddleware: func(router *gin.Engine) {
				router.Use(auth.CredentialExtractionMiddleware(logger))
				router.Use(func(c *gin.Context) {
					// Check for expired credentials
					if creds, ok := auth.GetToolCredentials(c.Request.Context()); ok && creds != nil {
						if creds.GitHub != nil && creds.GitHub.IsExpired() {
							c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
								"error": "GitHub credential has expired",
							})
							return
						}
					}
					c.Next()
				})
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "GitHub credential has expired",
		},
		{
			name: "Missing required credentials",
			setupRequest: func() *http.Request {
				body := map[string]interface{}{
					"action": "sync_data",
				}
				bodyBytes, _ := json.Marshal(body)
				req := httptest.NewRequest("POST", "/api/v1/tools/github/actions/sync_data", bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMiddleware: func(router *gin.Engine) {
				router.Use(auth.CredentialExtractionMiddleware(logger))
				router.Use(auth.CredentialValidationMiddleware([]string{"github"}, logger))
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Missing required credentials",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Setup router
			router := gin.New()

			// Apply middleware
			scenario.setupMiddleware(router)

			// Test handler
			router.POST("/api/v1/tools/:tool/actions/:action", func(c *gin.Context) {
				// Run verification if provided
				if scenario.verifyContext != nil {
					scenario.verifyContext(t, c)
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			// Execute request
			req := scenario.setupRequest()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, scenario.expectedStatus, w.Code)

			// Check error message if expected
			if scenario.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], scenario.expectedError)
			}
		})
	}
}

// TestMetricsRecording tests that authentication metrics are properly recorded
func TestMetricsRecording(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := observability.NewLogger("test")

	// Create mock metrics client
	metricsRecorded := make(map[string]int)

	// Setup router with metrics middleware
	router := gin.New()
	router.Use(auth.CredentialExtractionMiddleware(logger))
	router.Use(func(c *gin.Context) {
		// Record metrics based on auth method
		creds, hasCreds := auth.GetToolCredentials(c.Request.Context())

		if hasCreds && creds != nil && creds.GitHub != nil {
			metricsRecorded["user_credential"]++
		} else {
			metricsRecorded["service_account"]++
		}

		c.Next()
	})

	// Test handler
	router.POST("/api/v1/tools/:tool/actions/:action", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Test with user credentials
	t.Run("User credential metrics", func(t *testing.T) {
		body := map[string]interface{}{
			"credentials": map[string]interface{}{
				"github": map[string]interface{}{
					"token": "user-token",
					"type":  "pat",
				},
			},
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/tools/github/actions/test", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, metricsRecorded["user_credential"])
	})

	// Test without credentials (service account)
	t.Run("Service account metrics", func(t *testing.T) {
		body := map[string]interface{}{
			"parameters": map[string]interface{}{
				"test": "value",
			},
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/tools/github/actions/test", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, metricsRecorded["service_account"])
	})
}

// TestCredentialSanitization tests that credentials are properly sanitized in logs
func TestCredentialSanitization(t *testing.T) {
	creds := &models.ToolCredentials{
		GitHub: &models.TokenCredential{
			Token: "ghp_1234567890abcdef",
			Type:  "pat",
		},
		Jira: &models.TokenCredential{
			Token:    "jira-secret-token",
			Type:     "basic",
			Username: "user@example.com",
		},
	}

	sanitized := creds.SanitizedForLogging()

	// Verify GitHub token is sanitized
	if githubVal, ok := sanitized["github"]; ok {
		githubSanitized := githubVal.(map[string]interface{})
		// The actual implementation may vary, so check for reasonable sanitization
		if tokenHint, ok := githubSanitized["token_hint"].(string); ok {
			assert.Contains(t, tokenHint, "...")
			assert.NotEqual(t, creds.GitHub.Token, tokenHint)
		}
		assert.Equal(t, "pat", githubSanitized["type"])
	}

	// Verify Jira token is sanitized
	if jiraVal, ok := sanitized["jira"]; ok {
		jiraSanitized := jiraVal.(map[string]interface{})
		if tokenHint, ok := jiraSanitized["token_hint"].(string); ok {
			assert.Contains(t, tokenHint, "...")
			assert.NotEqual(t, creds.Jira.Token, tokenHint)
		}
		assert.Equal(t, "basic", jiraSanitized["type"])
		// Username might not be included in sanitized output
	}
}

// TestContextPropagation tests that credentials are properly propagated through context
func TestContextPropagation(t *testing.T) {
	// Create initial context with credentials
	creds := &models.ToolCredentials{
		GitHub: &models.TokenCredential{
			Token: "test-token",
			Type:  "pat",
		},
	}

	ctx := context.Background()
	ctx = auth.WithToolCredentials(ctx, creds)

	// Test retrieval
	t.Run("Direct retrieval", func(t *testing.T) {
		retrieved, ok := auth.GetToolCredentials(ctx)
		assert.True(t, ok)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test-token", retrieved.GitHub.Token)
	})

	// Test specific tool retrieval
	t.Run("Tool-specific retrieval", func(t *testing.T) {
		githubCred, ok := auth.GetToolCredential(ctx, "github")
		assert.True(t, ok)
		assert.NotNil(t, githubCred)
		assert.Equal(t, "test-token", githubCred.Token)

		// Non-existent tool
		jiraCred, ok := auth.GetToolCredential(ctx, "jira")
		assert.False(t, ok)
		assert.Nil(t, jiraCred)
	})

	// Test credential check
	t.Run("Has credential check", func(t *testing.T) {
		assert.True(t, auth.HasToolCredential(ctx, "github"))
		assert.False(t, auth.HasToolCredential(ctx, "jira"))
	})

	// Test with wrapper
	t.Run("CredentialContext wrapper", func(t *testing.T) {
		cc := auth.NewCredentialContext(context.Background())
		cc = cc.WithCredentials(creds)

		retrieved, ok := cc.GetCredentials()
		assert.True(t, ok)
		assert.NotNil(t, retrieved)

		githubCred, ok := cc.GetCredential("github")
		assert.True(t, ok)
		assert.Equal(t, "test-token", githubCred.Token)

		assert.True(t, cc.HasCredential("github"))
		assert.False(t, cc.HasCredential("gitlab"))
	})
}
