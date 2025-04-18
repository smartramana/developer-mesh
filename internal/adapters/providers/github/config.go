package github

import (
	"time"
	
	adapterConfig "github.com/S-Corkum/mcp-server/internal/adapters/config"
)

// Config holds configuration for the GitHub adapter
type Config struct {
	// Authentication
	Token        string `yaml:"token" json:"token"`
	AppID        string `yaml:"app_id" json:"app_id"`
	InstallID    string `yaml:"install_id" json:"install_id"`
	PrivateKey   string `yaml:"private_key" json:"private_key"`
	
	// API settings
	BaseURL      string        `yaml:"base_url" json:"base_url"`
	UploadURL    string        `yaml:"upload_url" json:"upload_url"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
	
	// Features
	EnabledFeatures []string `yaml:"enabled_features" json:"enabled_features"`
	
	// Default repository settings
	DefaultOwner string `yaml:"default_owner" json:"default_owner"`
	DefaultRepo  string `yaml:"default_repo" json:"default_repo"`
	
	// Common adapter configuration
	Resilience    adapterConfig.ResilienceConfig    `yaml:"resilience" json:"resilience"`
	Security      adapterConfig.SecurityConfig      `yaml:"security" json:"security"`
	Observability adapterConfig.ObservabilityConfig `yaml:"observability" json:"observability"`
}

// DefaultConfig returns default GitHub adapter configuration
func DefaultConfig() Config {
	defaultAdapter := adapterConfig.DefaultAdapterConfig()
	
	return Config{
		Timeout:         10 * time.Second,
		EnabledFeatures: []string{"issues", "pull_requests", "repositories", "comments"},
		Resilience:      defaultAdapter.Resilience,
		Security:        defaultAdapter.Security,
		Observability:   defaultAdapter.Observability,
	}
}

// ValidateConfig validates GitHub adapter configuration
func ValidateConfig(config Config) (bool, []string) {
	var errors []string
	
	// Validate authentication settings
	if config.Token == "" && (config.AppID == "" || config.InstallID == "" || config.PrivateKey == "") {
		errors = append(errors, "either token or app authentication is required")
	}
	
	// Validate timeout
	if config.Timeout <= 0 {
		errors = append(errors, "timeout must be positive")
	}
	
	// Validate default repository settings if any features require them
	requiresRepo := false
	for _, feature := range config.EnabledFeatures {
		if feature == "issues" || feature == "pull_requests" || feature == "comments" {
			requiresRepo = true
			break
		}
	}
	
	if requiresRepo && (config.DefaultOwner == "" || config.DefaultRepo == "") {
		errors = append(errors, "default owner and repository are required for the enabled features")
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
