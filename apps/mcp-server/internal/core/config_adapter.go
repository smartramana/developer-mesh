package core

import (
	"github.com/S-Corkum/devops-mcp/pkg/common/aws"
	commonconfig "github.com/S-Corkum/devops-mcp/pkg/common/config"
)

// ConfigAdapter adapts commonconfig.Config to implement CoreConfig interface
type ConfigAdapter struct {
	cfg *commonconfig.Config
}

// NewConfigAdapter creates a new config adapter
func NewConfigAdapter(cfg *commonconfig.Config) *ConfigAdapter {
	return &ConfigAdapter{cfg: cfg}
}

// GetString gets a string value from the configuration
func (c *ConfigAdapter) GetString(key string) string {
	// This is a simplified implementation - you might need to expand this
	// based on your specific configuration structure
	switch key {
	case "database.dsn":
		return c.cfg.Database.DSN
	case "api.listen_address":
		return c.cfg.API.ListenAddress
	default:
		return ""
	}
}

// AWS returns the AWS configuration
func (c *ConfigAdapter) AWS() *aws.AWSConfig {
	// Since aws.AWSConfig is an alias for storage.AWSConfig, 
	// we need to return the S3's AWS config
	return &aws.AWSConfig{
		UseIAMAuth: c.cfg.AWS.S3.AWSConfig.UseIAMAuth,
		Region:     c.cfg.AWS.S3.AWSConfig.Region,
		Endpoint:   c.cfg.AWS.S3.AWSConfig.Endpoint,
		AssumeRole: c.cfg.AWS.S3.AWSConfig.AssumeRole,
	}
}

// S3 returns the S3 configuration if available
func (c *ConfigAdapter) S3() *aws.S3Config {
	return &c.cfg.AWS.S3
}

// ConcurrencyLimit returns the concurrency limit
func (c *ConfigAdapter) ConcurrencyLimit() int {
	// Return the configured value or a default
	if c.cfg.Engine.ConcurrencyLimit > 0 {
		return c.cfg.Engine.ConcurrencyLimit
	}
	return 10
}