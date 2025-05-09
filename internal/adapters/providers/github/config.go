// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"fmt"
	"time"
	
	adapterConfig "github.com/S-Corkum/devops-mcp/internal/adapters/config"
)

// Default configuration values
const (
	// DefaultTimeout is the default timeout for GitHub API requests
	DefaultTimeout = 10 * time.Second
	
	// MinimumTimeout is the minimum allowed timeout for GitHub API requests
	MinimumTimeout = 1 * time.Second
	
	// MaximumTimeout is the maximum allowed timeout for GitHub API requests
	MaximumTimeout = 60 * time.Second
)

// Feature constants for more type safety
const (
	FeatureIssues       = "issues"
	FeaturePullRequests = "pull_requests"
	FeatureRepositories = "repositories"
	FeatureComments     = "comments"
)

// Config holds configuration for the GitHub adapter.
// This structure defines all settings required to interact with the GitHub API.
type Config struct {
	// Authentication (either Token or App authentication is required)
	Token        string `yaml:"token" json:"token"`               // Personal access token
	AppID        string `yaml:"app_id" json:"app_id"`             // GitHub App ID
	InstallID    string `yaml:"install_id" json:"install_id"`     // GitHub App Installation ID
	PrivateKey   string `yaml:"private_key" json:"private_key"`   // GitHub App private key (PEM format)

	// API settings
	BaseURL      string        `yaml:"base_url" json:"base_url"`         // GitHub API base URL (for Enterprise)
	UploadURL    string        `yaml:"upload_url" json:"upload_url"`     // GitHub upload URL (for Enterprise)
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`           // Timeout for API requests

	// Features
	EnabledFeatures []string `yaml:"enabled_features" json:"enabled_features"` // Enabled adapter features

	// Webhook control
	DisableWebhooks bool `yaml:"disable_webhooks" json:"disable_webhooks"` // Disable webhook processing if true

	// Default repository settings
	DefaultOwner string `yaml:"default_owner" json:"default_owner"` // Default repository owner/org
	DefaultRepo  string `yaml:"default_repo" json:"default_repo"`   // Default repository name

	// Common adapter configuration
	Resilience    adapterConfig.ResilienceConfig    `yaml:"resilience" json:"resilience"`       // Resilience settings
	Security      adapterConfig.SecurityConfig      `yaml:"security" json:"security"`           // Security settings
	Observability adapterConfig.ObservabilityConfig `yaml:"observability" json:"observability"` // Observability settings
}

// DefaultConfig returns default GitHub adapter configuration.
// This provides sensible defaults for all configuration options.
func DefaultConfig() Config {
	defaultAdapter := adapterConfig.DefaultAdapterConfig()
	
	return Config{
		Timeout: DefaultTimeout,
		EnabledFeatures: []string{
			FeatureIssues,
			FeaturePullRequests, 
			FeatureRepositories, 
			FeatureComments,
		},
		Resilience:    defaultAdapter.Resilience,
		Security:      defaultAdapter.Security,
		Observability: defaultAdapter.Observability,
	}
}

// Clone creates a deep copy of the configuration
func (c Config) Clone() Config {
	newConfig := c
	
	// Deep copy slices
	if c.EnabledFeatures != nil {
		newConfig.EnabledFeatures = make([]string, len(c.EnabledFeatures))
		copy(newConfig.EnabledFeatures, c.EnabledFeatures)
	}
	
	// Deep copy nested configs if needed
	// For now, the nested configs are simple enough that they don't need special handling
	
	return newConfig
}

// ValidateConfig validates the GitHub adapter configuration.
// Returns true if valid, false if not valid along with a list of validation errors.
func ValidateConfig(config Config) (bool, []string) {
	var errors []string
	
	// Validate authentication settings
	if config.Token == "" && (config.AppID == "" || config.InstallID == "" || config.PrivateKey == "") {
		errors = append(errors, "either token or app authentication is required")
	}
	
	// Validate timeout
	if config.Timeout <= 0 {
		errors = append(errors, "timeout must be positive")
	} else if config.Timeout < MinimumTimeout {
		errors = append(errors, fmt.Sprintf("timeout must be at least %s", MinimumTimeout))
	} else if config.Timeout > MaximumTimeout {
		errors = append(errors, fmt.Sprintf("timeout must not exceed %s", MaximumTimeout))
	}
	
	// Validate features
	if len(config.EnabledFeatures) == 0 {
		errors = append(errors, "at least one feature must be enabled")
	} else {
		// Check for unknown features
		validFeatures := map[string]bool{
			FeatureIssues:       true,
			FeaturePullRequests: true,
			FeatureRepositories: true,
			FeatureComments:     true,
		}
		
		for _, feature := range config.EnabledFeatures {
			if !validFeatures[feature] {
				errors = append(errors, fmt.Sprintf("unknown feature: %s", feature))
			}
		}
	}
	
	// Validate default repository settings if any features require them
	requiresRepo := false
	for _, feature := range config.EnabledFeatures {
		if feature == FeatureIssues || feature == FeaturePullRequests || feature == FeatureComments {
			requiresRepo = true
			break
		}
	}
	
	if requiresRepo && (config.DefaultOwner == "" || config.DefaultRepo == "") {
		errors = append(errors, "default owner and repository are required for the enabled features")
	}
	
	// Validate URLs if provided
	if config.BaseURL != "" && config.UploadURL == "" {
		errors = append(errors, "upload URL must be provided when base URL is set")
	}
	
	// Validate resilience configuration using the common validator
	validator := adapterConfig.DefaultConfigValidator{}
	adapterCfg := adapterConfig.AdapterConfig{
		Type:          "github",
		Resilience:    config.Resilience,
		Security:      config.Security,
		Observability: config.Observability,
	}
	
	valid, adapterErrors := validator.Validate(adapterCfg)
	if !valid {
		errors = append(errors, adapterErrors...)
	}
	
	return len(errors) == 0, errors
}

// IsFeatureEnabled checks if a specific feature is enabled in the configuration
func (c Config) IsFeatureEnabled(feature string) bool {
	for _, f := range c.EnabledFeatures {
		if f == feature {
			return true
		}
	}
	return false
}

// GetTimeout returns the configured timeout or the default if not set
func (c Config) GetTimeout() time.Duration {
	if c.Timeout <= 0 {
		return DefaultTimeout
	}
	return c.Timeout
}
