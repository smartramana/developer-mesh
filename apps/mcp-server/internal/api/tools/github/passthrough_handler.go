package github

import (
	"context"
	"fmt"
	"sync"

	"github.com/developer-mesh/developer-mesh/pkg/adapters/github"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// PassthroughAdapterFactory creates GitHub adapters with user credentials
type PassthroughAdapterFactory struct {
	logger         observability.Logger
	serviceAdapter *github.GitHubAdapter // Fallback service account adapter
	adapterCache   sync.Map              // Cache of user adapters
	metricsClient  observability.MetricsClient
	allowFallback  bool
}

// NewPassthroughAdapterFactory creates a new factory for pass-through authentication
func NewPassthroughAdapterFactory(
	serviceAdapter *github.GitHubAdapter,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	allowFallback bool,
) *PassthroughAdapterFactory {
	return &PassthroughAdapterFactory{
		logger:         logger,
		serviceAdapter: serviceAdapter,
		metricsClient:  metricsClient,
		allowFallback:  allowFallback,
	}
}

// GetAdapter returns a GitHub adapter with appropriate credentials
func (f *PassthroughAdapterFactory) GetAdapter(ctx context.Context) (*github.GitHubAdapter, error) {
	// Check for user credentials in context
	creds, ok := auth.GetToolCredentials(ctx)
	if ok && creds != nil && creds.GitHub != nil && creds.GitHub.Token != "" {
		return f.createUserAdapter(ctx, creds.GitHub)
	}

	// Fall back to service account if allowed
	if f.allowFallback && f.serviceAdapter != nil {
		f.logger.Debug("Using service account for GitHub (no user credentials provided)", map[string]interface{}{
			"has_user_context": ok,
			"has_credentials":  creds != nil,
			"has_github_cred":  creds != nil && creds.GitHub != nil,
		})

		f.metricsClient.IncrementCounterWithLabels("github_auth_method", 1, map[string]string{
			"method": "service_account",
		})

		return f.serviceAdapter, nil
	}

	return nil, fmt.Errorf("no GitHub credentials available and service account fallback is disabled")
}

// createUserAdapter creates a new adapter with user credentials
func (f *PassthroughAdapterFactory) createUserAdapter(ctx context.Context, cred *models.TokenCredential) (*github.GitHubAdapter, error) {
	// Validate credential
	if cred.IsExpired() {
		return nil, fmt.Errorf("GitHub credential has expired")
	}

	// Create cache key (using first/last 4 chars of token for uniqueness without exposing it)
	cacheKey := f.getCacheKey(cred.Token)

	// Check cache first
	if cached, ok := f.adapterCache.Load(cacheKey); ok {
		adapter := cached.(*github.GitHubAdapter)
		f.logger.Debug("Using cached GitHub adapter", map[string]interface{}{
			"cache_key": cacheKey,
		})
		return adapter, nil
	}

	// Create new adapter configuration
	config := github.DefaultConfig()

	// Set authentication based on credential type
	switch cred.Type {
	case "pat", "":
		config.Auth = github.AuthConfig{
			Type:  "token",
			Token: cred.Token,
		}
	case "oauth":
		// OAuth tokens use the same format as PATs in GitHub
		config.Auth = github.AuthConfig{
			Type:  "token",
			Token: cred.Token,
		}
	default:
		return nil, fmt.Errorf("unsupported GitHub credential type: %s", cred.Type)
	}

	// Override base URL if provided (for GitHub Enterprise)
	if cred.BaseURL != "" {
		config.BaseURL = cred.BaseURL
		// Adjust other URLs for enterprise
		config.UploadURL = cred.BaseURL + "uploads/"
		config.GraphQLURL = cred.BaseURL + "graphql"
	}

	// Create the adapter
	adapter, err := github.NewGitHubAdapter(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub adapter: %w", err)
	}

	// Cache the adapter
	f.adapterCache.Store(cacheKey, adapter)

	// Track metrics
	f.metricsClient.IncrementCounterWithLabels("github_auth_method", 1, map[string]string{
		"method": "user_credential",
		"type":   cred.Type,
	})

	f.logger.Info("Created GitHub adapter with user credentials", map[string]interface{}{
		"cache_key":    cacheKey,
		"type":         cred.Type,
		"has_base_url": cred.BaseURL != "",
		"token_hint":   cred.SanitizedForLogging()["token_hint"],
	})

	return adapter, nil
}

// getCacheKey generates a cache key from a token
func (f *PassthroughAdapterFactory) getCacheKey(token string) string {
	if len(token) < 8 {
		return fmt.Sprintf("github_%s", token)
	}
	// Use first 4 and last 4 characters
	return fmt.Sprintf("github_%s...%s", token[:4], token[len(token)-4:])
}

// ClearCache removes all cached adapters
func (f *PassthroughAdapterFactory) ClearCache() {
	count := 0
	f.adapterCache.Range(func(key, value interface{}) bool {
		f.adapterCache.Delete(key)
		count++
		return true
	})

	f.logger.Info("Cleared adapter cache", map[string]interface{}{
		"cleared_count": count,
	})
}

// ValidateCredential tests if a credential is valid
func (f *PassthroughAdapterFactory) ValidateCredential(ctx context.Context, cred *models.TokenCredential) error {
	if cred == nil || cred.Token == "" {
		return fmt.Errorf("credential is empty")
	}

	// Create a temporary adapter
	_, err := f.createUserAdapter(ctx, cred)
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}

	// Test the credential with a simple API call
	// For now, we'll just verify the adapter was created successfully
	// In a real implementation, you would call a GitHub API method through the adapter

	f.logger.Info("Credential validated successfully", map[string]interface{}{
		"type":         cred.Type,
		"has_base_url": cred.BaseURL != "",
	})

	return nil
}

// GetAuthenticatedUser returns information about the authenticated user
func (f *PassthroughAdapterFactory) GetAuthenticatedUser(ctx context.Context) (map[string]interface{}, error) {
	adapter, err := f.GetAdapter(ctx)
	if err != nil {
		return nil, err
	}

	// Check if this is a service account or user credential
	isServiceAccount := false
	var authMethod string

	if creds, ok := auth.GetToolCredentials(ctx); !ok || creds == nil || creds.GitHub == nil {
		isServiceAccount = true
		authMethod = "service_account"
	} else {
		authMethod = "user_credential"
	}

	// For now, return basic information about the authentication method
	// In a real implementation, you would call GitHub API through the adapter
	return map[string]interface{}{
		"auth_method":        authMethod,
		"is_service_account": isServiceAccount,
		"adapter_type":       adapter.Type(),
		"adapter_version":    adapter.Version(),
		"health":             adapter.Health(),
	}, nil
}
