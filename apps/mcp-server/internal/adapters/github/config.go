package github

import (
	"time"
)

// Config holds configuration for the GitHub adapter
type Config struct {
	// Authentication settings
	Token                string `mapstructure:"token"`
	AppID                string `mapstructure:"app_id"`
	AppPrivateKey        string `mapstructure:"app_private_key"`
	AppInstallationID    string `mapstructure:"app_installation_id"`
	UseApp               bool   `mapstructure:"use_app"`
	OAuthToken           string `mapstructure:"oauth_token"`
	OAuthClientID        string `mapstructure:"oauth_client_id"`
	OAuthClientSecret    string `mapstructure:"oauth_client_secret"`
	
	// Connection settings
	BaseURL              string        `mapstructure:"base_url"`
	UploadURL            string        `mapstructure:"upload_url"`
	GraphQLURL           string        `mapstructure:"graphql_url"`
	RequestTimeout       time.Duration `mapstructure:"request_timeout"`
	MaxIdleConns         int           `mapstructure:"max_idle_conns"`
	MaxConnsPerHost      int           `mapstructure:"max_conns_per_host"`
	MaxIdleConnsPerHost  int           `mapstructure:"max_idle_conns_per_host"`
	IdleConnTimeout      time.Duration `mapstructure:"idle_conn_timeout"`
	KeepAlive            time.Duration `mapstructure:"keep_alive"`
	TLSHandshakeTimeout  time.Duration `mapstructure:"tls_handshake_timeout"`
	ResponseHeaderTimeout time.Duration `mapstructure:"response_header_timeout"`
	ExpectContinueTimeout time.Duration `mapstructure:"expect_continue_timeout"`
	
	// Rate limiting settings
	RateLimit            bool          `mapstructure:"rate_limit"`
	RateLimitPerHour     int           `mapstructure:"rate_limit_per_hour"`
	GraphQLRateLimitPerHour int        `mapstructure:"graphql_rate_limit_per_hour"`
	RateLimitBurst       int           `mapstructure:"rate_limit_burst"`
	RateLimitBackoffFactor float64     `mapstructure:"rate_limit_backoff_factor"`
	RateLimitBackoffMax  time.Duration `mapstructure:"rate_limit_backoff_max"`
	RateLimitJitter      float64       `mapstructure:"rate_limit_jitter"`
	
	// Webhook settings
	WebhookSecret        string        `mapstructure:"webhook_secret"`
	WebhookDeliveryCache time.Duration `mapstructure:"webhook_delivery_cache"`
	WebhookWorkers       int           `mapstructure:"webhook_workers"`
	WebhookQueueSize     int           `mapstructure:"webhook_queue_size"`
	WebhookMaxRetries    int           `mapstructure:"webhook_max_retries"`
	DisableSignatureValidation bool    `mapstructure:"disable_signature_validation"`
	WebhookRetryInitialBackoff time.Duration `mapstructure:"webhook_retry_initial_backoff"`
	WebhookRetryMaxBackoff time.Duration `mapstructure:"webhook_retry_max_backoff"`
	WebhookRetryBackoffFactor float64   `mapstructure:"webhook_retry_backoff_factor"`
	WebhookRetryJitter    float64       `mapstructure:"webhook_retry_jitter"`
	WebhookValidatePayload bool         `mapstructure:"webhook_validate_payload"`
	
	// Pagination settings
	DefaultPageSize      int           `mapstructure:"default_page_size"`
	MaxPageSize          int           `mapstructure:"max_page_size"`
	MaxPages             int           `mapstructure:"max_pages"`
	
	// API preferences
	PreferGraphQLForReads bool          `mapstructure:"prefer_graphql_for_reads"`
	EnableWebhookFiltering bool         `mapstructure:"enable_webhook_filtering"`
	AutoCreateWebhookHandlers bool      `mapstructure:"auto_create_webhook_handlers"`
	DisableWebhooks       bool          `mapstructure:"disable_webhooks"`
	
	// Feature flags
	EnableGraphQLBuilder  bool          `mapstructure:"enable_graphql_builder"`
	EnableResilientRetry  bool          `mapstructure:"enable_resilient_retry"`
	EnableDistributedCache bool         `mapstructure:"enable_distributed_cache"`
	
	// Distributed cache settings
	CacheType            string        `mapstructure:"cache_type"` // "memory", "redis"
	CacheRedisURL        string        `mapstructure:"cache_redis_url"`
	CacheKeyPrefix       string        `mapstructure:"cache_key_prefix"`
	CacheDefaultTTL      time.Duration `mapstructure:"cache_default_ttl"`
}

// DefaultConfig returns a default configuration for the GitHub adapter
func DefaultConfig() *Config {
	return &Config{
		// Default to GitHub.com
		BaseURL:              "https://api.github.com/",
		UploadURL:            "https://uploads.github.com/",
		GraphQLURL:           "https://api.github.com/graphql",
		
		// Default connection settings
		RequestTimeout:        30 * time.Second,
		MaxIdleConns:          10,
		MaxConnsPerHost:       10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		
		// Default rate limiting
		RateLimit:              true,
		RateLimitPerHour:       5000,   // Default GitHub API rate limit
		GraphQLRateLimitPerHour: 5000,  // Default GitHub GraphQL API rate limit
		RateLimitBurst:          100,
		RateLimitBackoffFactor:  2.0,
		RateLimitBackoffMax:     1 * time.Hour,
		RateLimitJitter:         0.2,
		
		// Default webhook settings
		WebhookWorkers:                5,
		WebhookQueueSize:              100,
		WebhookMaxRetries:             3,
		WebhookDeliveryCache:          24 * time.Hour, // Cache delivery IDs for 24 hours
		WebhookRetryInitialBackoff:    1 * time.Second,
		WebhookRetryMaxBackoff:        1 * time.Hour,
		WebhookRetryBackoffFactor:     2.0,
		WebhookRetryJitter:            0.2,
		WebhookValidatePayload:        true,
		
		// Default pagination settings
		DefaultPageSize:        100,
		MaxPageSize:            100,
		MaxPages:               10,
		
		// Default API preferences
		PreferGraphQLForReads:    true, // Use GraphQL for read operations when possible
		EnableWebhookFiltering:   true,
		AutoCreateWebhookHandlers: true,
		DisableWebhooks:           false,
		
		// Default feature flags
		EnableGraphQLBuilder:     true,
		EnableResilientRetry:     true,
		EnableDistributedCache:   false,
		
		// Default cache settings
		CacheType:               "memory",
		CacheKeyPrefix:          "github:",
		CacheDefaultTTL:         1 * time.Hour,
	}
}

// AuthConfig returns the authentication configuration
func (c *Config) AuthConfig() *AuthConfig {
	return &AuthConfig{
		Token:             c.Token,
		AppID:             c.AppID,
		AppPrivateKey:     c.AppPrivateKey,
		AppInstallationID: c.AppInstallationID,
		UseApp:            c.UseApp,
		OAuthToken:        c.OAuthToken,
		OAuthClientID:     c.OAuthClientID,
		OAuthClientSecret: c.OAuthClientSecret,
	}
}

// WebhookConfig returns the webhook configuration
func (c *Config) WebhookConfig() *WebhookConfig {
	return &WebhookConfig{
		Secret:           c.WebhookSecret,
		Workers:          c.WebhookWorkers,
		QueueSize:        c.WebhookQueueSize,
		MaxRetries:       c.WebhookMaxRetries,
		DeliveryCacheTTL: c.WebhookDeliveryCache,
		RetryConfig: &RetryConfig{
			InitialBackoff: c.WebhookRetryInitialBackoff,
			MaxBackoff:     c.WebhookRetryMaxBackoff,
			BackoffFactor:  c.WebhookRetryBackoffFactor,
			Jitter:         c.WebhookRetryJitter,
		},
		ValidatePayload:  c.WebhookValidatePayload,
	}
}

// RateLimitConfig returns the rate limit configuration
func (c *Config) RateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:       c.RateLimit,
		PerHour:       c.RateLimitPerHour,
		GraphQLPerHour: c.GraphQLRateLimitPerHour,
		Burst:         c.RateLimitBurst,
		BackoffFactor: c.RateLimitBackoffFactor,
		BackoffMax:    c.RateLimitBackoffMax,
		Jitter:        c.RateLimitJitter,
	}
}

// CacheConfig returns the cache configuration
func (c *Config) CacheConfig() *CacheConfig {
	return &CacheConfig{
		Type:       c.CacheType,
		RedisURL:   c.CacheRedisURL,
		KeyPrefix:  c.CacheKeyPrefix,
		DefaultTTL: c.CacheDefaultTTL,
	}
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Token             string
	AppID             string
	AppPrivateKey     string
	AppInstallationID string
	UseApp            bool
	OAuthToken        string
	OAuthClientID     string
	OAuthClientSecret string
}

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
	Secret           string
	Workers          int
	QueueSize        int
	MaxRetries       int
	DeliveryCacheTTL time.Duration
	RetryConfig      *RetryConfig
	ValidatePayload  bool
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
	Jitter         float64
}

// RateLimitConfig holds rate limit configuration
type RateLimitConfig struct {
	Enabled       bool
	PerHour       int
	GraphQLPerHour int
	Burst         int
	BackoffFactor float64
	BackoffMax    time.Duration
	Jitter        float64
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Type       string
	RedisURL   string
	KeyPrefix  string
	DefaultTTL time.Duration
}
