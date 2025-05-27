// Package interfaces provides a compatibility layer for the various interface definitions
// that were moved to specific packages during the Go Workspace migration.
// This package is part of the migration to ensure backward compatibility
// with code still importing the old pkg/interfaces package path.
package interfaces

import (
	"context"
)

// Logger defines a minimal logging interface
type Logger interface {
	Info(args ...any)
	Infof(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Debug(args ...any)
	Debugf(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	With(key string, value any) Logger
}

// MetricsClient defines a minimal metrics interface
type MetricsClient interface {
	IncrementCounter(name string, value float64, labels map[string]string)
	RecordHistogram(name string, value float64, labels map[string]string)
	RecordGauge(name string, value float64, labels map[string]string)
}

// Cache defines a minimal cache interface
type Cache interface {
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, ttl int) error
	Delete(ctx context.Context, key string) error
}

// EventBus defines a minimal event bus interface
type EventBus interface {
	Publish(ctx context.Context, eventType string, event any) error
	Subscribe(eventType string, handler func(ctx context.Context, event any) error) error
	Unsubscribe(eventType string, handler func(ctx context.Context, event any) error) error
}

// Repository defines a minimal repository interface
type Repository interface {
	Get(ctx context.Context, id string) (any, error)
	List(ctx context.Context, filter any) ([]any, error)
	Create(ctx context.Context, entity any) (any, error)
	Update(ctx context.Context, entity any) (any, error)
	Delete(ctx context.Context, id string) error
}

// WebhookHandler defines a minimal webhook handler interface
type WebhookHandler interface {
	Handle(ctx context.Context, event any) error
}

// SQSReceiver defines a minimal SQS receiver interface
type SQSReceiver interface {
	ReceiveMessage(ctx context.Context, queueURL string, maxMessages int) ([]any, error)
}

// SQSDeleter defines a minimal SQS deleter interface
type SQSDeleter interface {
	DeleteMessage(ctx context.Context, queueURL string, receiptHandle string) error
}

// SQSReceiverDeleter combines SQSReceiver and SQSDeleter
type SQSReceiverDeleter interface {
	SQSReceiver
	SQSDeleter
}

// Adapter defines a minimal adapter interface
type Adapter interface {
	Name() string
	Close() error
}

// RateLimiter defines a minimal rate limiter interface
type RateLimiter interface {
	Allow() bool
	Wait(ctx context.Context) error
}

// CircuitBreaker defines a minimal circuit breaker interface
type CircuitBreaker interface {
	Execute(fn func() (any, error)) (any, error)
}

// Retry defines a minimal retry interface
type Retry interface {
	Execute(fn func() (any, error)) (any, error)
}

// ContextManager defines a minimal context manager interface
type ContextManager interface {
	CreateContext(ctx context.Context, tenantID, name string) (string, error)
	GetContext(ctx context.Context, contextID string) (any, error)
	UpdateContext(ctx context.Context, contextID string, data any) error
	DeleteContext(ctx context.Context, contextID string) error
}

// S3Client defines a minimal S3 client interface
type S3Client interface {
	PutObject(ctx context.Context, bucket, key string, body []byte) error
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	DeleteObject(ctx context.Context, bucket, key string) error
}

// WebhookConfig defines webhook configuration
type WebhookConfig struct {
	EnabledField             bool     `mapstructure:"enabled"`
	GitHubEndpointField      string   `mapstructure:"github_endpoint"`
	GitHubSecretField        string   `mapstructure:"github_secret"`
	GitHubIPValidationField  bool     `mapstructure:"github_ip_validation"`
	GitHubAllowedEventsField []string `mapstructure:"github_allowed_events"`
}

// WebhookConfigInterface defines an interface for accessing webhook configuration
type WebhookConfigInterface interface {
	Enabled() bool
	GitHubSecret() string
	GitHubEndpoint() string
	GitHubIPValidationEnabled() bool
	GitHubAllowedEvents() []string
}

// Enabled returns whether webhooks are enabled
func (c *WebhookConfig) Enabled() bool {
	return c.EnabledField
}

// GitHubSecret returns the GitHub webhook secret
func (c *WebhookConfig) GitHubSecret() string {
	return c.GitHubSecretField
}

// GitHubEndpoint returns the GitHub webhook endpoint
func (c *WebhookConfig) GitHubEndpoint() string {
	return c.GitHubEndpointField
}

// GitHubIPValidationEnabled returns whether GitHub IP validation is enabled
func (c *WebhookConfig) GitHubIPValidationEnabled() bool {
	return c.GitHubIPValidationField
}

// GitHubAllowedEvents returns the list of allowed GitHub events
func (c *WebhookConfig) GitHubAllowedEvents() []string {
	return c.GitHubAllowedEventsField
}

// APIConfig defines the API server configuration
type APIConfig struct {
	ListenAddress  string `mapstructure:"listen_address"`
	BaseURL        string `mapstructure:"base_url"`
	TLSCertFile    string `mapstructure:"tls_cert_file"`
	TLSKeyFile     string `mapstructure:"tls_key_file"`
	CORSAllowed    string `mapstructure:"cors_allowed"`
	RateLimit      int    `mapstructure:"rate_limit"`
	RequestTimeout int    `mapstructure:"request_timeout"`
}

// CoreConfig defines the engine core configuration
type CoreConfig struct {
	EventBufferSize  int `mapstructure:"event_buffer_size"`
	ConcurrencyLimit int `mapstructure:"concurrency_limit"`
	EventTimeout     any `mapstructure:"event_timeout"`
}
