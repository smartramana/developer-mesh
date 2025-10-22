package middleware

import (
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/services"
	"github.com/gin-gonic/gin"
)

// UserCredentialMiddleware loads user credentials from database after authentication
// This middleware should be added AFTER the authentication middleware that sets user context
func UserCredentialMiddleware(credService *services.CredentialService, logger observability.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from auth context (set by authentication middleware)
		user, exists := auth.GetUserFromContext(c)
		if !exists || user == nil {
			// No authenticated user, continue without credentials
			logger.Debug("No authenticated user in context, skipping credential loading", map[string]interface{}{
				"path": c.Request.URL.Path,
			})
			c.Next()
			return
		}

		// Load all user credentials from database
		decryptedCredsMap, err := credService.GetAllUserCredentials(
			c.Request.Context(),
			user.TenantID.String(),
			user.ID.String(),
		)
		if err != nil {
			// Log but don't fail - tools can fallback to service accounts
			logger.Debug("No user credentials found or error loading", map[string]interface{}{
				"error":     err.Error(),
				"user_id":   user.ID.String(),
				"tenant_id": user.TenantID.String(),
			})
			c.Next()
			return
		}

		// No credentials configured for this user
		if len(decryptedCredsMap) == 0 {
			logger.Debug("User has no credentials configured", map[string]interface{}{
				"user_id":   user.ID.String(),
				"tenant_id": user.TenantID.String(),
			})
			c.Next()
			return
		}

		// Convert to ToolCredentials format and add to context
		toolCreds := convertDecryptedCredsMapToToolCredentials(decryptedCredsMap)
		ctx := auth.WithToolCredentials(c.Request.Context(), toolCreds)
		c.Request = c.Request.WithContext(ctx)

		logger.Debug("User credentials loaded", map[string]interface{}{
			"user_id":          user.ID.String(),
			"tenant_id":        user.TenantID.String(),
			"credential_count": len(decryptedCredsMap),
			"services":         getServiceNamesFromMap(decryptedCredsMap),
		})

		c.Next()
	}
}

// convertDecryptedCredsMapToToolCredentials converts database credentials map to ToolCredentials format
func convertDecryptedCredsMapToToolCredentials(decryptedCredsMap map[models.ServiceType]*models.DecryptedCredentials) *models.ToolCredentials {
	toolCreds := &models.ToolCredentials{
		Custom: make(map[string]*models.TokenCredential),
	}

	for serviceType, cred := range decryptedCredsMap {
		tokenCred := convertToTokenCredential(cred)
		if tokenCred == nil {
			continue
		}

		// Map to specific tool field or custom map
		switch serviceType {
		case models.ServiceTypeGitHub:
			toolCreds.GitHub = tokenCred
		case models.ServiceTypeJira:
			toolCreds.Jira = tokenCred
		case models.ServiceTypeSonarQube:
			toolCreds.SonarQube = tokenCred
		case models.ServiceTypeArtifactory:
			toolCreds.Artifactory = tokenCred
		case models.ServiceTypeJenkins:
			toolCreds.Jenkins = tokenCred
		case models.ServiceTypeGitLab:
			toolCreds.GitLab = tokenCred
		case models.ServiceTypeBitbucket:
			toolCreds.Bitbucket = tokenCred
		default:
			// Store in custom map for other services
			toolCreds.Custom[string(serviceType)] = tokenCred
		}
	}

	return toolCreds
}

// convertToTokenCredential converts DecryptedCredentials to TokenCredential
func convertToTokenCredential(cred *models.DecryptedCredentials) *models.TokenCredential {
	if cred == nil || cred.Credentials == nil {
		return nil
	}

	tokenCred := &models.TokenCredential{
		Type: "pat", // Default to personal access token
	}

	// Extract common fields from credentials
	if token, ok := cred.Credentials["token"]; ok {
		tokenCred.Token = token
	}
	if username, ok := cred.Credentials["username"]; ok {
		tokenCred.Username = username
	}
	if password, ok := cred.Credentials["password"]; ok {
		tokenCred.Password = password
	}
	if apiToken, ok := cred.Credentials["api_token"]; ok {
		tokenCred.Token = apiToken
	}
	if apiKey, ok := cred.Credentials["api_key"]; ok {
		tokenCred.Token = apiKey
	}
	if baseURL, ok := cred.Credentials["base_url"]; ok {
		tokenCred.BaseURL = baseURL
	}

	// Set type based on service
	switch cred.ServiceType {
	case models.ServiceTypeGitHub, models.ServiceTypeGitLab, models.ServiceTypeBitbucket:
		tokenCred.Type = "pat"
		tokenCred.HeaderName = "Authorization"
		tokenCred.HeaderPrefix = "Bearer"
	case models.ServiceTypeJira, models.ServiceTypeConfluence:
		if tokenCred.Username != "" && tokenCred.Password != "" {
			tokenCred.Type = "basic"
		} else {
			tokenCred.Type = "bearer"
		}
	case models.ServiceTypeSonarQube:
		tokenCred.Type = "bearer"
		tokenCred.HeaderName = "Authorization"
		tokenCred.HeaderPrefix = "Bearer"
	case models.ServiceTypeArtifactory:
		if tokenCred.Username != "" && tokenCred.Password != "" {
			tokenCred.Type = "basic"
		} else {
			tokenCred.Type = "api_key"
		}
	case models.ServiceTypeJenkins:
		tokenCred.Type = "basic"
	}

	return tokenCred
}

// getServiceNamesFromMap extracts service type names for logging
func getServiceNamesFromMap(credsMap map[models.ServiceType]*models.DecryptedCredentials) []string {
	names := make([]string, 0, len(credsMap))
	for serviceType := range credsMap {
		names = append(names, string(serviceType))
	}
	return names
}
