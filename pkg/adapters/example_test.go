package adapters_test

import (
	"context"
	"fmt"
	"time"
	
	"github.com/S-Corkum/devops-mcp/pkg/adapters"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

func ExampleManager() {
	// Create a logger
	logger := observability.NewLogger("adapter-example")
	
	// Create adapter manager
	manager := adapters.NewManager(logger)
	
	// Configure GitHub adapter
	manager.SetConfig("github", adapters.Config{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		ProviderConfig: map[string]interface{}{
			"token": "ghp_your_github_token",
		},
	})
	
	// Get GitHub adapter
	ctx := context.Background()
	adapter, err := manager.GetAdapter(ctx, "github")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	// Use the adapter
	repos, err := adapter.ListRepositories(ctx, "octocat")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("Found %d repositories\n", len(repos))
}

func ExampleSourceControlAdapter() {
	// This example shows how to use the adapter interface
	ctx := context.Background()
	
	// Assume we have an adapter (GitHub, GitLab, etc.)
	var adapter adapters.SourceControlAdapter
	
	// Get a specific repository
	repo, err := adapter.GetRepository(ctx, "owner", "repo")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("Repository: %s\n", repo.Name)
	fmt.Printf("URL: %s\n", repo.URL)
	
	// Create an issue
	issue := &adapters.Issue{
		Title:       "Bug: Something is broken",
		Description: "Details about the bug...",
		Labels:      []string{"bug", "high-priority"},
	}
	
	createdIssue, err := adapter.CreateIssue(ctx, "owner", "repo", issue)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("Created issue #%d\n", createdIssue.Number)
}