package config

import (
	"os"

	commonconfig "github.com/S-Corkum/devops-mcp/pkg/common/config"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// NewWebhookConfig creates a new WebhookConfig with default values
func NewWebhookConfig() *commonconfig.WebhookConfig {
	// Default configuration
	// Create the config structure manually with unexported fields
	cfg := commonconfig.NewWebhookConfig()

	// We need to set these manually through env vars or just use the defaults from common/config

	// Load from environment variables
	if val := os.Getenv("MCP_WEBHOOK_ENABLED"); val == "true" {
		// Since we can't modify unexported fields directly, we'll recreate a new WebhookConfig
		// This would ideally be solved by adding setter methods to WebhookConfig in common/config
		newCfg := commonconfig.NewWebhookConfig()
		// For now, we'll just use the default configuration which has these values already set
		return newCfg
	}

	// Since the common/config WebhookConfig doesn't have exported fields or setter methods,
	// we'll need to recreate the configuration object with the default values it provides
	// and implement any custom logic in the calling code instead.

	// Validate configuration
	if cfg.GitHubSecret() == "" {
		observability.DefaultLogger.Warn("GitHub webhook configuration does not have a secret configured", nil)
	}

	return cfg
}