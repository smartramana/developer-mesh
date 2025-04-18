package core

import (
	"context"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
)

// Adapter defines the core interface for all adapters
type Adapter interface {
	// Initialize sets up the adapter with configuration
	Initialize(ctx context.Context, config interface{}) error

	// GetData retrieves data from the external service
	GetData(ctx context.Context, query interface{}) (interface{}, error)
	
	// ExecuteAction executes an action with context awareness
	ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error)
	
	// HandleWebhook processes webhook events from the external service
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error
	
	// Subscribe registers a callback for a specific event type
	Subscribe(eventType string, callback func(interface{})) error
	
	// IsSafeOperation determines if an operation is safe to perform
	IsSafeOperation(operation string, params map[string]interface{}) (bool, error)

	// Health returns the health status of the adapter
	Health() string

	// Close gracefully shuts down the adapter
	Close() error
	
	// Version returns the adapter's version
	Version() string
	
	// SupportsFeature checks if the adapter supports a specific feature
	SupportsFeature(feature string) bool
}

// BaseAdapter provides common functionality for adapters
type BaseAdapter struct {
	AdapterType     string
	AdapterVersion  string
	Features        map[string]bool
	RetryConfig     resilience.RetryConfig
	Timeout         time.Duration
	SafeMode        bool // When true, enforces safety checks on all operations
	CircuitBreaker  resilience.CircuitBreaker
	RateLimiter     resilience.RateLimiter
	MetricsEnabled  bool
}

// Version returns the adapter's version
func (b *BaseAdapter) Version() string {
	return b.AdapterVersion
}

// SupportsFeature checks if the adapter supports a specific feature
func (b *BaseAdapter) SupportsFeature(feature string) bool {
	supported, exists := b.Features[feature]
	return exists && supported
}

// Health returns a simple health status string
func (b *BaseAdapter) Health() string {
	if b.CircuitBreaker != nil && b.CircuitBreaker.IsOpen() {
		return "unhealthy: circuit breaker open"
	}
	return "healthy"
}

// WithFeatures adds feature support information to the adapter
func (b *BaseAdapter) WithFeatures(features map[string]bool) *BaseAdapter {
	b.Features = features
	return b
}

// WithRetry configures retry settings for the adapter
func (b *BaseAdapter) WithRetry(config resilience.RetryConfig) *BaseAdapter {
	b.RetryConfig = config
	return b
}

// WithCircuitBreaker adds a circuit breaker to the adapter
func (b *BaseAdapter) WithCircuitBreaker(cb resilience.CircuitBreaker) *BaseAdapter {
	b.CircuitBreaker = cb
	return b
}

// WithRateLimiter adds a rate limiter to the adapter
func (b *BaseAdapter) WithRateLimiter(rl resilience.RateLimiter) *BaseAdapter {
	b.RateLimiter = rl
	return b
}

// WithTimeout sets the timeout for adapter operations
func (b *BaseAdapter) WithTimeout(timeout time.Duration) *BaseAdapter {
	b.Timeout = timeout
	return b
}

// WithSafeMode enables or disables safe mode
func (b *BaseAdapter) WithSafeMode(enabled bool) *BaseAdapter {
	b.SafeMode = enabled
	return b
}

// WithMetrics enables or disables metrics collection
func (b *BaseAdapter) WithMetrics(enabled bool) *BaseAdapter {
	b.MetricsEnabled = enabled
	return b
}
