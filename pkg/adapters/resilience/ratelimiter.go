package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter defines the interface for a rate limiter
type RateLimiter interface {
	// Wait blocks until rate limit allows an event or context is done
	Wait(ctx context.Context) error

	// Allow returns true if the rate limit allows an event at time now
	Allow() bool

	// Name returns the rate limiter name
	Name() string
}

// RateLimiterConfig defines configuration for a rate limiter
type RateLimiterConfig struct {
	Name      string
	Rate      float64 // Rate per second
	Burst     int
	WaitLimit time.Duration
}

// DefaultRateLimiter is the default implementation of RateLimiter
type DefaultRateLimiter struct {
	limiter *rate.Limiter
	config  RateLimiterConfig
	// Reset time based on GitHub API headers
	resetTime     time.Time
	resetLock     sync.RWMutex
	dynamicFactor float64 // Dynamic adjustment factor based on rate limit usage
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimiterConfig) RateLimiter {
	// Set default values if not provided
	if config.Rate <= 0 {
		config.Rate = 100 // Default to 100 requests per second
	}
	if config.Burst <= 0 {
		config.Burst = 10
	}
	if config.WaitLimit <= 0 {
		config.WaitLimit = 5 * time.Second
	}

	return &DefaultRateLimiter{
		limiter:       rate.NewLimiter(rate.Limit(config.Rate), config.Burst),
		config:        config,
		dynamicFactor: 1.0, // Start with no adjustment
	}
}

// Wait blocks until rate limit allows an event or context is done
func (rl *DefaultRateLimiter) Wait(ctx context.Context) error {
	// Check if we're approaching a GitHub API reset time
	rl.resetLock.RLock()
	resetTime := rl.resetTime
	rl.resetLock.RUnlock()

	if !resetTime.IsZero() {
		timeUntilReset := time.Until(resetTime)

		// If we're very close to the reset time (within 5 seconds)
		// and we know our rate limit is almost exhausted,
		// it might be better to wait for the reset
		if timeUntilReset > 0 && timeUntilReset < 5*time.Second && rl.dynamicFactor < 0.3 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(timeUntilReset + 100*time.Millisecond): // Add a small buffer
				// We've waited for the reset, now we can proceed
				return nil
			}
		}
	}

	// Create a context with timeout if WaitLimit is set
	waitCtx := ctx
	var cancel context.CancelFunc

	if rl.config.WaitLimit > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, rl.config.WaitLimit)
		defer cancel()
	}

	return rl.limiter.Wait(waitCtx)
}

// Allow returns true if the rate limit allows an event at time now
func (rl *DefaultRateLimiter) Allow() bool {
	return rl.limiter.Allow()
}

// Name returns the rate limiter name
func (rl *DefaultRateLimiter) Name() string {
	return rl.config.Name
}

// GitHubRateLimitInfo represents GitHub API rate limit information
type GitHubRateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
	Used      int
}

// AdjustRateLimit adjusts the rate limiter based on GitHub API rate limit headers
// This method is used by the GitHub REST client to dynamically adjust the rate limiter
func (rl *DefaultRateLimiter) AdjustRateLimit(info GitHubRateLimitInfo) {
	rl.resetLock.Lock()
	defer rl.resetLock.Unlock()

	// Store reset time
	rl.resetTime = info.Reset

	// Calculate remaining rate
	remainingRate := float64(info.Remaining)

	// Time until reset
	timeUntilReset := time.Until(info.Reset)
	if timeUntilReset <= 0 {
		// If reset time is in the past, no need to adjust
		return
	}

	// Calculate requests per second we can make until reset
	// Add a safety margin by reducing by 10%
	requestsPerSecond := remainingRate / timeUntilReset.Seconds() * 0.9

	// Don't let the rate go below 1 request per minute
	minRate := 1.0 / 60.0
	if requestsPerSecond < minRate {
		requestsPerSecond = minRate
	}

	// Don't exceed the configured max rate
	if requestsPerSecond > rl.config.Rate {
		requestsPerSecond = rl.config.Rate
	}

	// Calculate dynamic adjustment factor
	usageRatio := float64(info.Used) / float64(info.Limit)

	// If we've used more than 75% of our quota, start being more conservative
	if usageRatio > 0.75 {
		// Gradually reduce rate as we approach the limit
		rl.dynamicFactor = 1.0 - ((usageRatio - 0.75) * 2.0) // Range from 1.0 to 0.5
		requestsPerSecond *= rl.dynamicFactor
	} else {
		rl.dynamicFactor = 1.0
	}

	// Set the new rate limiter with calculated rate
	// Note: We keep the same burst size
	rl.limiter.SetLimit(rate.Limit(requestsPerSecond))
}

// RateLimiterManager manages multiple rate limiters
type RateLimiterManager struct {
	limiters map[string]RateLimiter
	mu       sync.RWMutex
}

// NewRateLimiterManager creates a new rate limiter manager
func NewRateLimiterManager(configs map[string]RateLimiterConfig) *RateLimiterManager {
	manager := &RateLimiterManager{
		limiters: make(map[string]RateLimiter),
	}

	// Create rate limiters from configs
	for name, config := range configs {
		manager.limiters[name] = NewRateLimiter(config)
	}

	return manager
}

// Get gets a rate limiter by name
func (m *RateLimiterManager) Get(name string) (RateLimiter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limiter, exists := m.limiters[name]
	return limiter, exists
}

// Register registers a new rate limiter
func (m *RateLimiterManager) Register(name string, config RateLimiterConfig) RateLimiter {
	m.mu.Lock()
	defer m.mu.Unlock()

	limiter := NewRateLimiter(config)
	m.limiters[name] = limiter
	return limiter
}

// Wait waits for a rate limiter, creating a default one if it doesn't exist
func (m *RateLimiterManager) Wait(ctx context.Context, name string) error {
	m.mu.RLock()
	limiter, exists := m.limiters[name]
	m.mu.RUnlock()

	if !exists {
		// Create a default rate limiter if it doesn't exist
		config := RateLimiterConfig{
			Name:  name,
			Rate:  100,
			Burst: 10,
		}
		limiter = m.Register(name, config)
	}

	return limiter.Wait(ctx)
}

// Execute executes a function with rate limiting
func (m *RateLimiterManager) Execute(ctx context.Context, name string, fn func() (any, error)) (any, error) {
	// Wait for rate limiter
	if err := m.Wait(ctx, name); err != nil {
		return nil, fmt.Errorf("rate limit exceeded for %s: %w", name, err)
	}

	// Execute the function
	return fn()
}
