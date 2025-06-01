package github

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/github"
	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPassthroughAdapterFactory tests the adapter factory functionality
func TestPassthroughAdapterFactory(t *testing.T) {
	logger := observability.NewLogger("test")
	metricsClient := observability.NewNoOpMetricsClient()
	
	// Create a mock service adapter
	serviceConfig := github.DefaultConfig()
	serviceConfig.Auth = github.AuthConfig{
		Type:  "token",
		Token: "service-account-token",
	}
	serviceAdapter, err := github.NewGitHubAdapter(*serviceConfig)
	require.NoError(t, err)
	
	// Create factory
	factory := NewPassthroughAdapterFactory(
		serviceAdapter,
		logger,
		metricsClient,
		true, // Allow fallback
	)
	
	t.Run("GetAdapter with user credentials", func(t *testing.T) {
		// Create context with user credentials
		ctx := context.Background()
		creds := &models.ToolCredentials{
			GitHub: &models.TokenCredential{
				Token: "ghp_user_token_123456789",
				Type:  "pat",
			},
		}
		ctx = auth.WithToolCredentials(ctx, creds)
		
		// Get adapter
		adapter, err := factory.GetAdapter(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
	})
	
	t.Run("GetAdapter with service account fallback", func(t *testing.T) {
		// Context without credentials
		ctx := context.Background()
		
		// Get adapter - should fallback to service account
		adapter, err := factory.GetAdapter(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, serviceAdapter, adapter) // Should be same instance
	})
	
	t.Run("GetAdapter with expired credentials", func(t *testing.T) {
		// Create context with expired credentials
		ctx := context.Background()
		creds := &models.ToolCredentials{
			GitHub: &models.TokenCredential{
				Token:     "ghp_expired_token",
				Type:      "pat",
				ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
			},
		}
		ctx = auth.WithToolCredentials(ctx, creds)
		
		// Get adapter - should fail
		adapter, err := factory.GetAdapter(ctx)
		assert.Error(t, err)
		assert.Nil(t, adapter)
		assert.Contains(t, err.Error(), "expired")
	})
	
	t.Run("GetAdapter with OAuth token", func(t *testing.T) {
		// Create context with OAuth token
		ctx := context.Background()
		creds := &models.ToolCredentials{
			GitHub: &models.TokenCredential{
				Token: "gho_oauth_token_123",
				Type:  "oauth",
			},
		}
		ctx = auth.WithToolCredentials(ctx, creds)
		
		// Get adapter
		adapter, err := factory.GetAdapter(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
	})
	
	t.Run("GetAdapter with GitHub Enterprise", func(t *testing.T) {
		// Create context with enterprise credentials
		ctx := context.Background()
		creds := &models.ToolCredentials{
			GitHub: &models.TokenCredential{
				Token:   "ghp_enterprise_token",
				Type:    "pat",
				BaseURL: "https://github.enterprise.com/",
			},
		}
		ctx = auth.WithToolCredentials(ctx, creds)
		
		// Get adapter
		adapter, err := factory.GetAdapter(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
	})
	
	t.Run("Adapter caching", func(t *testing.T) {
		// Create context with credentials
		ctx := context.Background()
		creds := &models.ToolCredentials{
			GitHub: &models.TokenCredential{
				Token: "ghp_cached_token_123456789",
				Type:  "pat",
			},
		}
		ctx = auth.WithToolCredentials(ctx, creds)
		
		// Get adapter twice
		_, err := factory.GetAdapter(ctx)
		assert.NoError(t, err)
		
		_, err = factory.GetAdapter(ctx)
		assert.NoError(t, err)
		
		// Should be cached (but we can't directly compare since NewGitHubAdapter creates new instances)
	})
	
	t.Run("ValidateCredential", func(t *testing.T) {
		ctx := context.Background()
		
		// Valid credential
		validCred := &models.TokenCredential{
			Token: "ghp_valid_token",
			Type:  "pat",
		}
		err := factory.ValidateCredential(ctx, validCred)
		assert.NoError(t, err)
		
		// Empty credential
		emptyCred := &models.TokenCredential{
			Token: "",
		}
		err = factory.ValidateCredential(ctx, emptyCred)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
		
		// Nil credential
		err = factory.ValidateCredential(ctx, nil)
		assert.Error(t, err)
	})
	
	t.Run("GetAuthenticatedUser", func(t *testing.T) {
		// With user credentials
		ctx := context.Background()
		creds := &models.ToolCredentials{
			GitHub: &models.TokenCredential{
				Token: "ghp_auth_user_token",
				Type:  "pat",
			},
		}
		ctx = auth.WithToolCredentials(ctx, creds)
		
		userInfo, err := factory.GetAuthenticatedUser(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "user_credential", userInfo["auth_method"])
		assert.Equal(t, false, userInfo["is_service_account"])
		assert.Equal(t, "github", userInfo["adapter_type"])
		assert.Equal(t, "1.0.0", userInfo["adapter_version"])
		
		// Without user credentials (service account)
		ctx = context.Background()
		userInfo, err = factory.GetAuthenticatedUser(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "service_account", userInfo["auth_method"])
		assert.Equal(t, true, userInfo["is_service_account"])
	})
	
	t.Run("ClearCache", func(t *testing.T) {
		// Add some adapters to cache
		ctx := context.Background()
		for i := 0; i < 3; i++ {
			creds := &models.ToolCredentials{
				GitHub: &models.TokenCredential{
					Token: fmt.Sprintf("ghp_cache_test_%d", i),
					Type:  "pat",
				},
			}
			ctx := auth.WithToolCredentials(ctx, creds)
			_, err := factory.GetAdapter(ctx)
			assert.NoError(t, err)
		}
		
		// Clear cache
		factory.ClearCache()
		// Cache is cleared, but we can't directly verify internal state
	})
	
	t.Run("No fallback allowed", func(t *testing.T) {
		// Create factory without fallback
		factoryNoFallback := NewPassthroughAdapterFactory(
			serviceAdapter,
			logger,
			metricsClient,
			false, // No fallback
		)
		
		// Context without credentials
		ctx := context.Background()
		
		// Should fail
		adapter, err := factoryNoFallback.GetAdapter(ctx)
		assert.Error(t, err)
		assert.Nil(t, adapter)
		assert.Contains(t, err.Error(), "no GitHub credentials available")
	})
}