package api

import (
	"time"
)

// Config holds configuration for the API server
type Config struct {
	ListenAddress string        `mapstructure:"listen_address"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
	IdleTimeout   time.Duration `mapstructure:"idle_timeout"`
	EnableCORS    bool          `mapstructure:"enable_cors"`
	TLSCertFile   string        `mapstructure:"tls_cert_file"`
	TLSKeyFile    string        `mapstructure:"tls_key_file"`
	Auth          AuthConfig    `mapstructure:"auth"`
	RateLimit     RateLimitConfig `mapstructure:"rate_limit"`
	AgentWebhook  AgentWebhookConfig `mapstructure:"agent_webhook"`
	Webhooks      WebhookConfig `mapstructure:"webhooks"`
}

// WebhookConfig holds configuration for all webhooks
type WebhookConfig struct {
	GitHub      WebhookEndpointConfig `mapstructure:"github"`
}

// WebhookEndpointConfig holds configuration for a webhook endpoint
type WebhookEndpointConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Secret  string `mapstructure:"secret"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret string            `mapstructure:"jwt_secret"`
	APIKeys   map[string]string `mapstructure:"api_keys"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	Limit       int           `mapstructure:"limit"`
	Period      time.Duration `mapstructure:"period"`
	BurstFactor int           `mapstructure:"burst_factor"`
}
