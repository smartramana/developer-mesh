package auth

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http/httptest"
    "testing"

    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCredentialExtractionMiddleware(t *testing.T) {
    gin.SetMode(gin.TestMode)
    logger := observability.NewLogger("test")

    tests := []struct {
        name           string
        path           string
        body           interface{}
        expectCreds    bool
        expectedTools  []string
    }{
        {
            name: "Extract GitHub credentials",
            path: "/api/v1/tools/github/actions/create_issue",
            body: map[string]interface{}{
                "action": "create_issue",
                "parameters": map[string]interface{}{
                    "owner": "test",
                    "repo":  "test",
                },
                "credentials": map[string]interface{}{
                    "github": map[string]interface{}{
                        "token": "ghp_testtoken123456",
                        "type":  "pat",
                    },
                },
            },
            expectCreds:   true,
            expectedTools: []string{"github"},
        },
        {
            name: "No credentials provided",
            path: "/api/v1/tools/github/actions/list_repos",
            body: map[string]interface{}{
                "action": "list_repos",
                "parameters": map[string]interface{}{
                    "type": "public",
                },
            },
            expectCreds:   false,
            expectedTools: []string{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create router with middleware
            router := gin.New()
            router.Use(CredentialExtractionMiddleware(logger))

            // Test handler to verify credentials
            var capturedCreds *models.ToolCredentials
            router.POST("/*path", func(c *gin.Context) {
                creds, _ := GetToolCredentials(c.Request.Context())
                capturedCreds = creds
                c.JSON(200, gin.H{"ok": true})
            })

            // Create request
            body, err := json.Marshal(tt.body)
            require.NoError(t, err)

            req := httptest.NewRequest("POST", tt.path, bytes.NewBuffer(body))
            req.Header.Set("Content-Type", "application/json")

            // Execute request
            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            // Verify response
            assert.Equal(t, 200, w.Code)

            // Verify credentials
            if tt.expectCreds {
                assert.NotNil(t, capturedCreds)
                for _, tool := range tt.expectedTools {
                    assert.True(t, capturedCreds.HasCredentialFor(tool), 
                        "Expected credential for %s", tool)
                }
            } else {
                assert.Nil(t, capturedCreds)
            }
        })
    }
}

func TestCredentialSanitization(t *testing.T) {
    tests := []struct {
        name     string
        creds    *models.ToolCredentials
        expected map[string]interface{}
    }{
        {
            name: "Sanitize GitHub credential",
            creds: &models.ToolCredentials{
                GitHub: &models.TokenCredential{
                    Token: "ghp_secrettoken123456",
                    Type:  "pat",
                },
            },
            expected: map[string]interface{}{
                "github": map[string]interface{}{
                    "type":       "pat",
                    "has_token":  true,
                    "has_username": false,
                    "base_url":   "",
                    "token_hint": "...3456",
                },
            },
        },
        {
            name:     "Nil credentials",
            creds:    nil,
            expected: nil,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := tt.creds.SanitizedForLogging()
            assert.Equal(t, tt.expected, result)
        })
    }
}

func TestCredentialContext(t *testing.T) {
    ctx := context.Background()
    
    // Test adding and retrieving credentials
    creds := &models.ToolCredentials{
        GitHub: &models.TokenCredential{
            Token: "test-token",
            Type:  "pat",
        },
    }
    
    // Add credentials to context
    ctxWithCreds := WithToolCredentials(ctx, creds)
    
    // Retrieve credentials
    retrievedCreds, ok := GetToolCredentials(ctxWithCreds)
    assert.True(t, ok)
    assert.NotNil(t, retrievedCreds)
    assert.True(t, retrievedCreds.HasCredentialFor("github"))
    
    // Test specific tool credential
    githubCred, ok := GetToolCredential(ctxWithCreds, "github")
    assert.True(t, ok)
    assert.NotNil(t, githubCred)
    assert.Equal(t, "test-token", githubCred.Token)
    
    // Test missing tool
    jiraCred, ok := GetToolCredential(ctxWithCreds, "jira")
    assert.False(t, ok)
    assert.Nil(t, jiraCred)
}

func TestCredentialValidationMiddleware(t *testing.T) {
    gin.SetMode(gin.TestMode)
    logger := observability.NewLogger("test")

    tests := []struct {
        name          string
        requiredTools []string
        credentials   *models.ToolCredentials
        expectSuccess bool
    }{
        {
            name:          "All required credentials present",
            requiredTools: []string{"github"},
            credentials: &models.ToolCredentials{
                GitHub: &models.TokenCredential{
                    Token: "test-token",
                },
            },
            expectSuccess: true,
        },
        {
            name:          "Missing required credential",
            requiredTools: []string{"github", "jira"},
            credentials: &models.ToolCredentials{
                GitHub: &models.TokenCredential{
                    Token: "test-token",
                },
            },
            expectSuccess: false,
        },
        {
            name:          "No required tools",
            requiredTools: []string{},
            credentials:   nil,
            expectSuccess: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            router := gin.New()
            
            // Add credentials to context if provided
            if tt.credentials != nil {
                router.Use(func(c *gin.Context) {
                    ctx := WithToolCredentials(c.Request.Context(), tt.credentials)
                    c.Request = c.Request.WithContext(ctx)
                    c.Next()
                })
            }
            
            // Add validation middleware
            router.Use(CredentialValidationMiddleware(tt.requiredTools, logger))
            
            // Test handler
            router.GET("/test", func(c *gin.Context) {
                c.JSON(200, gin.H{"ok": true})
            })
            
            // Make request
            req := httptest.NewRequest("GET", "/test", nil)
            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)
            
            // Verify result
            if tt.expectSuccess {
                assert.Equal(t, 200, w.Code)
            } else {
                assert.Equal(t, 401, w.Code)
            }
        })
    }
}