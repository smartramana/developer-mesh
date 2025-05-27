// Package github provides a GitHub API adapter for the MCP system
package github

import (
	"context"
	"errors"
)

// This file contains legacy functions and will be phased out after migration
// Config and GitHubAdapter have been migrated to config.go and adapter.go

// NewGitHubAdapter creates a new GitHub adapter with the given configuration
// This is a legacy function, use New() from adapter.go instead
func NewGitHubAdapter(config Config) (*GitHubAdapter, error) {
	if config.Auth.Type == "" || (config.Auth.Type == "token" && config.Auth.Token == "") {
		return nil, errors.New("GitHub authentication is required")
	}

	// Create a pointer to the config to satisfy the adapter's requirements
	cfg := &config
	return &GitHubAdapter{
		config: cfg,
	}, nil
}

// ExecuteAction executes a GitHub API action with the given parameters
func (g *GitHubAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]any) (any, error) {
	// This is a stub implementation to satisfy compilation
	// In real implementation, this would dispatch to the appropriate GitHub API endpoint
	return nil, errors.New("not implemented")
}

// Note: The following methods are implemented in adapter.go:
// - Type() string
// - Version() string
// - Health() string
// - HandleWebhook(ctx context.Context, eventType string, payload []byte) error
// - Close() error
