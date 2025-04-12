package api

import "time"

// Config holds configuration for the API server
type Config struct {
	ListenAddress string        `mapstructure:"listen_address"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
	IdleTimeout   time.Duration `mapstructure:"idle_timeout"`
	BasePath      string        `mapstructure:"base_path"`
	EnableCORS    bool          `mapstructure:"enable_cors"`
	LogRequests   bool          `mapstructure:"log_requests"`

	// Rate limiting configuration
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`

	// Webhook configuration
	Webhooks WebhookConfig `mapstructure:"webhooks"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	Limit      int           `mapstructure:"limit"`
	Burst      int           `mapstructure:"burst"`
	Expiration time.Duration `mapstructure:"expiration"`
}

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
	GitHub      WebhookProviderConfig `mapstructure:"github"`
	Harness     WebhookProviderConfig `mapstructure:"harness"`
	SonarQube   WebhookProviderConfig `mapstructure:"sonarqube"`
	Artifactory WebhookProviderConfig `mapstructure:"artifactory"`
	Xray        WebhookProviderConfig `mapstructure:"xray"`
}

// WebhookProviderConfig holds configuration for a webhook provider
type WebhookProviderConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Secret  string `mapstructure:"secret"`
	Path    string `mapstructure:"path"`
}
