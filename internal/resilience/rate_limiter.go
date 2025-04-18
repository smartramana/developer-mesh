package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiterConfig holds configuration for rate limiters
type RateLimiterConfig struct {
	Name      string        `mapstructure:"name"`
	Rate      float64       `mapstructure:"rate"`      // Requests per second
	Burst     int           `mapstructure:"burst"`     // Maximum burst size
	WaitLimit time.Duration `mapstructure:"wait_limit"` // Maximum wait time
}

var (
	rateLimiters     = make(map[string]*rate.Limiter)
	rateLimiterMutex sync.RWMutex
)

// GetRateLimiter returns a rate limiter with the given name, creating it if it doesn't exist
func GetRateLimiter(name string, config RateLimiterConfig) *rate.Limiter {
	rateLimiterMutex.RLock()
	limiter, ok := rateLimiters[name]
	rateLimiterMutex.RUnlock()

	if ok {
		return limiter
	}

	// Not found, create a new one
	rateLimiterMutex.Lock()
	defer rateLimiterMutex.Unlock()

	// Check again in case it was created while we were waiting for the lock
	if limiter, ok := rateLimiters[name]; ok {
		return limiter
	}

	// Apply defaults if needed
	if config.Rate == 0 {
		config.Rate = 10 // 10 requests per second by default
	}
	if config.Burst == 0 {
		config.Burst = 20 // Allow bursts of 20 by default
	}

	// Create a new rate limiter
	limiter = rate.NewLimiter(rate.Limit(config.Rate), config.Burst)
	rateLimiters[name] = limiter
	return limiter
}

// ExecuteWithRateLimiter executes a function with a rate limiter
func ExecuteWithRateLimiter(ctx context.Context, name string, config RateLimiterConfig, fn func() (interface{}, error)) (interface{}, error) {
	limiter := GetRateLimiter(name, config)

	// Apply wait limit if configured
	var waitCtx context.Context
	var cancel context.CancelFunc
	if config.WaitLimit > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, config.WaitLimit)
		defer cancel()
	} else {
		waitCtx = ctx
	}

	// Wait for rate limiter
	if err := limiter.Wait(waitCtx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Execute the function
	return fn()
}

// RateLimiterManager manages a set of rate limiters
type RateLimiterManager struct {
	configs map[string]RateLimiterConfig
}

// NewRateLimiterManager creates a new rate limiter manager
func NewRateLimiterManager(configs map[string]RateLimiterConfig) *RateLimiterManager {
	return &RateLimiterManager{
		configs: configs,
	}
}

// Execute executes a function with a rate limiter
func (m *RateLimiterManager) Execute(ctx context.Context, name string, fn func() (interface{}, error)) (interface{}, error) {
	config, ok := m.configs[name]
	if !ok {
		// Use default config if no config is found
		config = RateLimiterConfig{
			Name:      name,
			Rate:      10,
			Burst:     20,
			WaitLimit: 5 * time.Second,
		}
	}

	return ExecuteWithRateLimiter(ctx, name, config, fn)
}

// Common rate limiter names
const (
	GitHubRateLimiter   = "github"
	S3RateLimiter       = "s3"
	DatabaseRateLimiter = "database"
	RedisRateLimiter    = "redis"
	VectorRateLimiter   = "vector"
	// AI model rate limiters
	AnthropicRateLimiter = "anthropic"
	BedrockRateLimiter   = "bedrock"
	OpenAIRateLimiter    = "openai"
)

// AI-specific rate limiters with token-based limiting
type TokenBucketRateLimiter struct {
	limiter *rate.Limiter
	mu      sync.Mutex
}

// NewTokenBucketRateLimiter creates a new token bucket rate limiter
func NewTokenBucketRateLimiter(tokensPerMinute int, maxBurst int) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(float64(tokensPerMinute)/60.0), maxBurst),
	}
}

// Allow checks if the given token count can be processed
func (t *TokenBucketRateLimiter) Allow(tokenCount int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.limiter.AllowN(time.Now(), tokenCount)
}

// Wait waits until the given token count can be processed
func (t *TokenBucketRateLimiter) Wait(ctx context.Context, tokenCount int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.limiter.WaitN(ctx, tokenCount)
}

// Reserve reserves the given token count
func (t *TokenBucketRateLimiter) Reserve(tokenCount int) *rate.Reservation {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.limiter.ReserveN(time.Now(), tokenCount)
}

// ShutdownRateLimiters closes all rate limiters
func ShutdownRateLimiters() {
	rateLimiterMutex.Lock()
	defer rateLimiterMutex.Unlock()

	rateLimiters = make(map[string]*rate.Limiter)
}
