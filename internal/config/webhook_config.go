package config

import (
	"os"
	"strings"

	"github.com/S-Corkum/devops-mcp/internal/observability"
)

// WebhookConfig implements the WebhookConfigInterface
type WebhookConfig struct {
	enabled             bool
	githubSecret        string
	githubEndpoint      string
	githubIPValidation  bool
	githubAllowedEvents []string
}

// Enabled returns whether webhooks are enabled
func (c *WebhookConfig) Enabled() bool {
	return c.enabled
}

// GitHubSecret returns the GitHub webhook secret
func (c *WebhookConfig) GitHubSecret() string {
	return c.githubSecret
}

// GitHubEndpoint returns the GitHub webhook endpoint path
func (c *WebhookConfig) GitHubEndpoint() string {
	return c.githubEndpoint
}

// GitHubIPValidationEnabled returns whether GitHub IP validation is enabled
func (c *WebhookConfig) GitHubIPValidationEnabled() bool {
	return c.githubIPValidation
}

// GitHubAllowedEvents returns the list of allowed GitHub event types
func (c *WebhookConfig) GitHubAllowedEvents() []string {
	return c.githubAllowedEvents
}

// NewWebhookConfig creates a new WebhookConfig with default values
func NewWebhookConfig() *WebhookConfig {
	// Default configuration
	config := &WebhookConfig{
		enabled:             false,
		githubEndpoint:      "/api/webhooks/github",
		githubIPValidation:  true,
		githubAllowedEvents: []string{"push", "pull_request", "issues", "issue_comment", "release"},
	}

	// Load from environment variables
	if val := os.Getenv("MCP_WEBHOOK_ENABLED"); val == "true" {
		config.enabled = true
	}

	if val := os.Getenv("MCP_GITHUB_WEBHOOK_SECRET"); val != "" {
		config.githubSecret = val
		// Only enable GitHub webhook if a secret is provided
		config.enabled = true
	}

	if val := os.Getenv("MCP_GITHUB_WEBHOOK_ENDPOINT"); val != "" {
		config.githubEndpoint = val
	}

	if val := os.Getenv("MCP_GITHUB_IP_VALIDATION"); val == "false" {
		config.githubIPValidation = false
	}

	if val := os.Getenv("MCP_GITHUB_ALLOWED_EVENTS"); val != "" {
		config.githubAllowedEvents = strings.Split(val, ",")
	}

	// Validate configuration
	if config.enabled && config.githubSecret == "" {
		observability.DefaultLogger.Warn("GitHub webhook is enabled but no secret is configured, webhook will be disabled", nil)
		config.enabled = false
	}

	return config
}