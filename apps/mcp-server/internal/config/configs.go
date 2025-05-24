// Package config provides configuration structures for the MCP application
package config

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
	AWS              *AWSConfig    `mapstructure:"aws"`
}

// AWSConfig holds configuration for AWS services in the Core Engine
type AWSConfig struct {
	S3 *S3Config `mapstructure:"s3"`
}

// S3Config holds configuration for S3
//
// Region: AWS region for S3 buckets (e.g., "us-west-2").
// Endpoint: Custom S3-compatible endpoint (for local testing or alternative providers).
// AssumeRole: IAM role ARN to assume for S3 access (optional).
// ForcePathStyle: Use path-style URLs for S3 (required for LocalStack or custom endpoints).
type S3Config struct {
	Bucket           string        `mapstructure:"bucket"`                 // Name of the S3 bucket to use for storage
	UseIAMAuth       bool          `mapstructure:"use_iam_auth"`           // Enable IAM-based authentication (IRSA, EC2 roles, etc.)
	Region           string        `mapstructure:"region"`                 // AWS region for S3 buckets (e.g., "us-west-2")
	Endpoint         string        `mapstructure:"endpoint"`               // Custom S3-compatible endpoint URL (optional)
	AssumeRole       string        `mapstructure:"assume_role"`            // IAM role ARN to assume for S3 access (optional)
	ForcePathStyle   bool          `mapstructure:"force_path_style"`       // Use path-style URLs for S3 (required for LocalStack or custom endpoints)
	UploadPartSize   int           `mapstructure:"upload_part_size"`       // Part size (in bytes) for multipart uploads
	DownloadPartSize int           `mapstructure:"download_part_size"`     // Part size (in bytes) for multipart downloads
	Concurrency      int           `mapstructure:"concurrency"`            // Number of concurrent upload/download workers
	RequestTimeout   time.Duration `mapstructure:"request_timeout"`        // Timeout for S3 requests
	Encryption       string        `mapstructure:"server_side_encryption"` // Server-side encryption method (e.g., "AES256")
}

// APIConfig holds configuration for the API server
type APIConfig struct {
	ListenAddress string          `mapstructure:"listen_address"`
	ReadTimeout   time.Duration   `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration   `mapstructure:"write_timeout"`
	IdleTimeout   time.Duration   `mapstructure:"idle_timeout"`
	EnableCORS    bool            `mapstructure:"enable_cors"`
	EnableSwagger bool            `mapstructure:"enable_swagger"`
	TLSCertFile   string          `mapstructure:"tls_cert_file"`
	TLSKeyFile    string          `mapstructure:"tls_key_file"`
	BasePath      string          `mapstructure:"base_path"`
	LogRequests   bool            `mapstructure:"log_requests"`
	CORSOrigins   []string        `mapstructure:"cors_origins"`
	RateLimit     RateLimitConfig `mapstructure:"rate_limit"`
	Auth          AuthConfig      `mapstructure:"auth"`
	Webhook       WebhookConfig   `mapstructure:"webhook"`
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
	RequireAuth           bool              `mapstructure:"require_auth"`
	JWTSecret             string            `mapstructure:"jwt_secret"`
	JWTExpiration         time.Duration     `mapstructure:"jwt_expiration"`
	TokenRenewalThreshold time.Duration     `mapstructure:"token_renewal_threshold"`
	APIKeys               map[string]string `mapstructure:"api_keys"`
}

// WebhookConfig holds configuration for all webhooks
type WebhookConfig struct {
	EnabledField                 bool     `mapstructure:"enabled"`
	GitHubEndpointField          string   `mapstructure:"github_endpoint"`
	GitHubSecretField            string   `mapstructure:"github_secret"`
	GitHubIPValidationField      bool     `mapstructure:"github_ip_validation"`
	GitHubAllowedEventsField     []string `mapstructure:"github_allowed_events"`
}

// WebhookConfigInterface defines an interface for accessing webhook configuration
type WebhookConfigInterface interface {
	Enabled() bool
	GitHubSecret() string
	GitHubEndpoint() string
	GitHubIPValidationEnabled() bool
	GitHubAllowedEvents() []string
}

// Ensure WebhookConfig implements WebhookConfigInterface
var _ WebhookConfigInterface = (*WebhookConfig)(nil)

// Enabled returns whether webhooks are enabled
func (w *WebhookConfig) Enabled() bool {
	return w.EnabledField
}

// GitHubSecret returns the GitHub webhook secret
func (w *WebhookConfig) GitHubSecret() string {
	return w.GitHubSecretField
}

// GitHubEndpoint returns the GitHub webhook endpoint
func (w *WebhookConfig) GitHubEndpoint() string {
	return w.GitHubEndpointField
}

// GitHubIPValidationEnabled returns whether GitHub IP validation is enabled
func (w *WebhookConfig) GitHubIPValidationEnabled() bool {
	return w.GitHubIPValidationField
}

// GitHubAllowedEvents returns the list of allowed GitHub events
func (w *WebhookConfig) GitHubAllowedEvents() []string {
	return w.GitHubAllowedEventsField
}

// WebhookEndpointConfig holds configuration for a webhook endpoint
type WebhookEndpointConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Secret  string `mapstructure:"secret"`
}
