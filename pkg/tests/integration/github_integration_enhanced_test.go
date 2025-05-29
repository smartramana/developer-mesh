//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/github"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/require"
)

// setupGitHubAdapter creates a GitHub adapter configured for either mock or real API
func setupGitHubAdapter(t *testing.T) (*github.Adapter, *github.Config) {
	logger := observability.NewLogger("github-test")
	config := github.DefaultConfig()

	// Check if we should use real GitHub API
	if os.Getenv("USE_GITHUB_MOCK") == "false" {
		// Real GitHub API configuration
		config.BaseURL = "https://api.github.com/"
		config.MockResponses = false
		
		// Ensure we have credentials
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			t.Skip("GITHUB_TOKEN not set for real API testing")
		}
		config.Auth.Token = token
		
		// Optional: Configure GitHub App if available
		if appID := os.Getenv("GITHUB_APP_ID"); appID != "" {
			config.Auth.AppID = appID
			config.Auth.PrivateKeyPath = os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
		}
		
		t.Logf("Using real GitHub API at %s", config.BaseURL)
	} else {
		// Mock server configuration (existing behavior)
		config.MockResponses = true
		config.BaseURL = "http://localhost:8081/mock-github"
		config.Auth.Token = "test-token"
		
		t.Logf("Using mock GitHub API at %s", config.BaseURL)
	}

	adapter, err := github.NewAdapter(config, logger)
	require.NoError(t, err)

	return adapter, config
}

// getTestOrgAndRepo returns the organization and repository for testing
func getTestOrgAndRepo(t *testing.T) (string, string) {
	if os.Getenv("USE_GITHUB_MOCK") == "false" {
		org := os.Getenv("GITHUB_TEST_ORG")
		repo := os.Getenv("GITHUB_TEST_REPO")
		
		if org == "" || repo == "" {
			t.Skip("GITHUB_TEST_ORG and GITHUB_TEST_REPO must be set for real API testing")
		}
		
		return org, repo
	}
	
	// Mock values
	return "test-org", "test-repo"
}

// TestGitHubIntegrationEnhanced tests GitHub integration with both mock and real API support
func TestGitHubIntegrationEnhanced(t *testing.T) {
	adapter, config := setupGitHubAdapter(t)
	require.NotNil(t, adapter)

	ctx := context.Background()
	org, repo := getTestOrgAndRepo(t)

	t.Run("GetRepository", func(t *testing.T) {
		if config.MockResponses {
			t.Skip("Skipping for mock API - implement mock handler first")
		}

		result, err := adapter.GetRepository(ctx, org, repo)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, repo, result.Name)
	})

	t.Run("ListPullRequests", func(t *testing.T) {
		if config.MockResponses {
			t.Skip("Skipping for mock API - implement mock handler first")
		}

		prs, err := adapter.ListPullRequests(ctx, org, repo, "all")
		require.NoError(t, err)
		// PRs might be empty, that's okay
		t.Logf("Found %d pull requests", len(prs))
	})

	t.Run("WebhookSignature", func(t *testing.T) {
		secret := "test-secret"
		if !config.MockResponses {
			secret = os.Getenv("GITHUB_WEBHOOK_SECRET")
			if secret == "" {
				t.Skip("GITHUB_WEBHOOK_SECRET not set")
			}
		}

		payload := []byte(`{"action":"opened"}`)
		signature := adapter.GenerateWebhookSignature(payload, secret)
		require.NotEmpty(t, signature)

		valid := adapter.ValidateWebhookSignature(payload, signature, secret)
		require.True(t, valid)
	})
}