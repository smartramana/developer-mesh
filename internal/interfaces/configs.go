package interfaces

import (
	"time"
)

// CoreConfig holds configuration for the core engine
type CoreConfig struct {
	EventBufferSize  int           `mapstructure:"event_buffer_size"`
	ConcurrencyLimit int           `mapstructure:"concurrency_limit"`
	EventTimeout     time.Duration `mapstructure:"event_timeout"`
	MaxToolDuration  time.Duration `mapstructure:"max_tool_duration"`
	DefaultModelID   string        `mapstructure:"default_model_id"`
	LogEvents        bool          `mapstructure:"log_events"`
}

// APIConfig holds configuration for the API server
type APIConfig struct {
	ListenAddress string        `mapstructure:"listen_address"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
	IdleTimeout   time.Duration `mapstructure:"idle_timeout"`
	EnableCORS    bool          `mapstructure:"enable_cors"`
	EnableSwagger bool          `mapstructure:"enable_swagger"`
	TLSCertFile   string        `mapstructure:"tls_cert_file"`
	TLSKeyFile    string        `mapstructure:"tls_key_file"`
	BasePath      string        `mapstructure:"base_path"`
	LogRequests   bool          `mapstructure:"log_requests"`
	CORSOrigins   []string      `mapstructure:"cors_origins"`
	RateLimit     RateLimitConfig `mapstructure:"rate_limit"`
	Auth          AuthConfig    `mapstructure:"auth"`
	Webhooks      WebhookConfig `mapstructure:"webhooks"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	Limit      int           `mapstructure:"limit"`
	Burst      int           `mapstructure:"burst"`
	Expiration time.Duration `mapstructure:"expiration"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	RequireAuth           bool          `mapstructure:"require_auth"`
	JWTSecret             string        `mapstructure:"jwt_secret"`
	JWTExpiration         time.Duration `mapstructure:"jwt_expiration"`
	TokenRenewalThreshold time.Duration `mapstructure:"token_renewal_threshold"`
	APIKeys               map[string]string `mapstructure:"api_keys"`
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
