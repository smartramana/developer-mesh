// Package github provides a GitHub API adapter for the MCP system
package github

import (
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


// Note: The following methods are implemented in adapter.go:
// - Type() string
// - Version() string
// - Health() string
// - HandleWebhook(ctx context.Context, eventType string, payload []byte) error
// - Close() error
