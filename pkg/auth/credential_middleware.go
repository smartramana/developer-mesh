package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
)

// CredentialExtractionMiddleware extracts tool credentials from requests
func CredentialExtractionMiddleware(logger observability.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only process if this is a tool action request
		if !isToolRequest(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Read body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Error("Failed to read request body", map[string]interface{}{
				"error": err.Error(),
				"path":  c.Request.URL.Path,
			})
			c.Next()
			return
		}

		// Restore body for next handler
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Try to extract credentials from request body
		var body map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &body); err == nil {
			if credsData, exists := body["credentials"]; exists {
				creds := parseToolCredentials(credsData, logger)
				if creds != nil {
					// Add to context
					ctx := WithToolCredentials(c.Request.Context(), creds)
					c.Request = c.Request.WithContext(ctx)

					// Log that credentials were provided (without values)
					logger.Debug("Tool credentials provided", map[string]interface{}{
						"tools":      creds.SanitizedForLogging(),
						"path":       c.Request.URL.Path,
						"method":     c.Request.Method,
						"user_agent": c.Request.UserAgent(),
					})
				}
			}

			// Store parsed body for reuse
			c.Set("parsed_body", body)
		}

		c.Next()
	}
}

// isToolRequest checks if the request is for a tool action
func isToolRequest(path string) bool {
	// Check if path matches tool action pattern
	// Examples:
	// - /api/v1/tools/github/actions/create_issue
	// - /api/v1/tools/jira/actions/create_ticket
	return strings.Contains(path, "/tools/") && strings.Contains(path, "/actions/")
}

// parseToolCredentials safely parses credentials from request data
func parseToolCredentials(data interface{}, logger observability.Logger) *models.ToolCredentials {
	// Marshal and unmarshal to handle type conversion
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		logger.Debug("Failed to marshal credentials data", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}

	var creds models.ToolCredentials
	if err := json.Unmarshal(jsonBytes, &creds); err != nil {
		logger.Debug("Failed to unmarshal credentials", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}

	// Validate at least one credential is present
	hasAnyCredential := creds.GitHub != nil && creds.GitHub.Token != ""
	if creds.Jira != nil && creds.Jira.Token != "" {
		hasAnyCredential = true
	}
	if creds.SonarQube != nil && creds.SonarQube.Token != "" {
		hasAnyCredential = true
	}
	if creds.Artifactory != nil && creds.Artifactory.Token != "" {
		hasAnyCredential = true
	}
	if creds.Jenkins != nil && creds.Jenkins.Token != "" {
		hasAnyCredential = true
	}
	if creds.GitLab != nil && creds.GitLab.Token != "" {
		hasAnyCredential = true
	}
	if creds.Bitbucket != nil && creds.Bitbucket.Token != "" {
		hasAnyCredential = true
	}
	if len(creds.Custom) > 0 {
		for _, v := range creds.Custom {
			if v != nil && v.Token != "" {
				hasAnyCredential = true
				break
			}
		}
	}

	if !hasAnyCredential {
		return nil
	}

	return &creds
}

// CredentialValidationMiddleware validates that required credentials are present
func CredentialValidationMiddleware(requiredTools []string, logger observability.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if no tools are required
		if len(requiredTools) == 0 {
			c.Next()
			return
		}

		// Get credentials from context
		creds, ok := GetToolCredentials(c.Request.Context())
		if !ok || creds == nil {
			// Check if we have a service account fallback
			if !hasServiceAccountFallback(c) {
				c.JSON(401, gin.H{
					"error": "Missing required credentials",
					"required_tools": requiredTools,
				})
				c.Abort()
				return
			}
			// Service account available, continue
			c.Next()
			return
		}

		// Check each required tool
		missingTools := []string{}
		for _, tool := range requiredTools {
			if !creds.HasCredentialFor(tool) {
				// Check if service account is available for this tool
				if !hasServiceAccountForTool(c, tool) {
					missingTools = append(missingTools, tool)
				}
			}
		}

		if len(missingTools) > 0 {
			c.JSON(401, gin.H{
				"error": "Missing credentials for required tools",
				"missing_tools": missingTools,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// hasServiceAccountFallback checks if service account fallback is available
func hasServiceAccountFallback(c *gin.Context) bool {
	// Check configuration or context for service account availability
	// This would be set by the server configuration
	fallback, exists := c.Get("service_account_fallback_enabled")
	if !exists {
		return false
	}
	
	enabled, ok := fallback.(bool)
	return ok && enabled
}

// hasServiceAccountForTool checks if a service account is available for a specific tool
func hasServiceAccountForTool(c *gin.Context, tool string) bool {
	// Check if service account exists for this tool
	serviceAccounts, exists := c.Get("available_service_accounts")
	if !exists {
		return false
	}
	
	accounts, ok := serviceAccounts.(map[string]bool)
	if !ok {
		return false
	}
	
	return accounts[tool]
}

// ExtractCredentialsFromBody is a helper to extract credentials from a parsed body
func ExtractCredentialsFromBody(c *gin.Context) (*models.ToolCredentials, bool) {
	// First check context (already extracted by middleware)
	if creds, ok := GetToolCredentials(c.Request.Context()); ok {
		return creds, true
	}

	// Try to get from parsed body
	if body, exists := c.Get("parsed_body"); exists {
		if bodyMap, ok := body.(map[string]interface{}); ok {
			if credsData, exists := bodyMap["credentials"]; exists {
				creds := parseToolCredentials(credsData, observability.DefaultLogger)
				return creds, creds != nil
			}
		}
	}

	// Try to parse from request body
	var req models.CredentialRequest
	if err := c.ShouldBindJSON(&req); err == nil {
		return req.Credentials, req.Credentials != nil
	}

	return nil, false
}