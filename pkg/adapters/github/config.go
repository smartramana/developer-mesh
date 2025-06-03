package github

import (
	"time"
)

// Config holds configuration for the GitHub adapter
type Config struct {
	// Authentication settings
	Auth AuthConfig `yaml:"auth" json:"auth"`

	// Connection settings
	BaseURL             string        `yaml:"base_url" json:"base_url"`
	UploadURL           string        `yaml:"upload_url" json:"upload_url"`
	GraphQLURL          string        `yaml:"graphql_url" json:"graphql_url"`
	RequestTimeout      time.Duration `yaml:"request_timeout" json:"request_timeout"`
	MaxIdleConns        int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	MaxConnsPerHost     int           `yaml:"max_conns_per_host" json:"max_conns_per_host"`
	MaxIdleConnsPerHost int           `yaml:"max_idle_conns_per_host" json:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`

	// Rate limiting settings
	RateLimit      float64       `yaml:"rate_limit" json:"rate_limit"`
	RateLimitBurst int           `yaml:"rate_limit_burst" json:"rate_limit_burst"`
	RateLimitWait  time.Duration `yaml:"rate_limit_wait" json:"rate_limit_wait"`

	// Webhook settings
	WebhooksEnabled         bool          `yaml:"webhooks_enabled" json:"webhooks_enabled"`
	WebhookSecret           string        `yaml:"webhook_secret" json:"webhook_secret"`
	WebhookWorkers          int           `yaml:"webhook_workers" json:"webhook_workers"`
	WebhookQueueSize        int           `yaml:"webhook_queue_size" json:"webhook_queue_size"`
	WebhookRetryEnabled     bool          `yaml:"webhook_retry_enabled" json:"webhook_retry_enabled"`
	WebhookRetryMaxAttempts int           `yaml:"webhook_retry_max_attempts" json:"webhook_retry_max_attempts"`
	WebhookRetryDelay       time.Duration `yaml:"webhook_retry_delay" json:"webhook_retry_delay"`
	// ForceTerminateWorkersOnTimeout is used in testing to prevent goroutine leaks
	ForceTerminateWorkersOnTimeout bool `yaml:"force_terminate_workers_on_timeout" json:"force_terminate_workers_on_timeout"`

	// Pagination settings
	DefaultPageSize int `yaml:"default_page_size" json:"default_page_size"`
	MaxPageSize     int `yaml:"max_page_size" json:"max_page_size"`
}

// AuthConfig holds authentication configuration for GitHub
type AuthConfig struct {
	Type           string `yaml:"type" json:"type"`
	Token          string `yaml:"token" json:"token"`
	AppID          int64  `yaml:"app_id" json:"app_id"`
	InstallationID int64  `yaml:"installation_id" json:"installation_id"`
	PrivateKey     string `yaml:"private_key" json:"private_key"`
}

// DefaultConfig returns a default configuration for the GitHub adapter
func DefaultConfig() *Config {
	return &Config{
		Auth: AuthConfig{
			Type: "token",
		},

		// Default to GitHub.com
		BaseURL:    "https://api.github.com/",
		UploadURL:  "https://uploads.github.com/",
		GraphQLURL: "https://api.github.com/graphql",

		// Default connection settings
		RequestTimeout:      30 * time.Second,
		MaxIdleConns:        10,
		MaxConnsPerHost:     10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,

		// Default rate limiting
		RateLimit:      5000 / 3600, // Default GitHub API rate limit per second
		RateLimitBurst: 100,
		RateLimitWait:  5 * time.Second,

		// Default webhook settings
		WebhooksEnabled:                true,
		WebhookWorkers:                 5,
		WebhookQueueSize:               100,
		WebhookRetryEnabled:            true,
		WebhookRetryMaxAttempts:        3,
		WebhookRetryDelay:              5 * time.Second,
		ForceTerminateWorkersOnTimeout: true, // Default to true to prevent goroutine leaks in tests

		// Default pagination settings
		DefaultPageSize: 100,
		MaxPageSize:     100,
	}
}
