//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/adapters"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/github"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/require"
)

// setupGitHubAdapter creates a GitHub adapter configured for either mock or real API
func setupGitHubAdapter(t *testing.T) (adapters.SourceControlAdapter, *github.Config) {
	logger := observability.NewLogger("github-test")
	config := github.DefaultConfig()

	// Check if we should use real GitHub API
	if os.Getenv("USE_GITHUB_MOCK") == "false" {
		// Real GitHub API configuration
		config.BaseURL = "https://api.github.com/"
		
		// Ensure we have credentials
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			t.Skip("GITHUB_TOKEN not set for real API testing")
		}
		config.Auth.Token = token

		// Optional: Configure GitHub App if available
		if appIDStr := os.Getenv("GITHUB_APP_ID"); appIDStr != "" {
			appID, err := strconv.ParseInt(appIDStr, 10, 64)
			if err == nil {
				config.Auth.AppID = appID
			}
		}

		t.Logf("Using real GitHub API at %s", config.BaseURL)
	} else {
		// Mock server configuration (existing behavior)
		config.BaseURL = "http://localhost:8081/mock-github"
		config.Auth.Token = "test-token"

		t.Logf("Using mock GitHub API at %s", config.BaseURL)
	}

	// Create manager and register GitHub provider
	manager := adapters.NewManager(logger)
	
	// Register GitHub provider (this would normally be done in init or main)
	github.Register(manager.GetFactory())
	
	// Convert github.Config to adapters.Config
	adapterConfig := adapters.Config{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RateLimit:  100,
		ProviderConfig: map[string]any{
			"github": config,
		},
	}
	
	// Set config for GitHub
	manager.SetConfig("github", adapterConfig)
	
	// Create adapter
	ctx := context.Background()
	adapter, err := manager.GetAdapter(ctx, "github")
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
	adapter, _ := setupGitHubAdapter(t)
	require.NotNil(t, adapter)

	t.Run("GetRepository", func(t *testing.T) {
		if os.Getenv("USE_GITHUB_MOCK") != "false" {
			t.Skip("Skipping for mock API - implement mock handler first")
		}

		// Note: GetRepository method may not exist on generic Adapter interface
		// This test may need to be refactored based on available methods
		t.Skip("GetRepository method needs to be implemented on Adapter interface")
	})

	t.Run("ListPullRequests", func(t *testing.T) {
		if os.Getenv("USE_GITHUB_MOCK") != "false" {
			t.Skip("Skipping for mock API - implement mock handler first")
		}

		// Note: ListPullRequests method may not exist on generic Adapter interface
		// This test may need to be refactored based on available methods
		t.Skip("ListPullRequests method needs to be implemented on Adapter interface")
	})

	t.Run("WebhookSignature", func(t *testing.T) {
		// For now, we'll skip this test as we need to access adapter-specific methods
		// that are not part of the standard adapter interface
		t.Skip("Skipping webhook signature test - needs to be updated to work with adapter interface")

		// The test needs to be redesigned to work without type assertions to the concrete GitHub adapter
		// Options include:
		// 1. Adding webhook validator methods to the main adapter interface
		// 2. Creating a test-specific adapter factory that exposes the needed methods
		// 3. Using a separate webhook validation utility that doesn't rely on adapter internals
	})
}
