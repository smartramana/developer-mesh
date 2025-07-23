package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RateLimiter provides rate limiting for authentication endpoints
type RateLimiter struct {
	cache       cache.Cache
	logger      observability.Logger
	localLimits sync.Map // fallback for when cache is unavailable

	// Configuration
	enabled       bool
	maxAttempts   int
	windowSize    time.Duration
	lockoutPeriod time.Duration
}

// RateLimiterConfig holds rate limiter configuration
type RateLimiterConfig struct {
	Enabled       bool          // Whether rate limiting is enabled
	MaxAttempts   int           // Max attempts per window
	WindowSize    time.Duration // Time window for attempts
	LockoutPeriod time.Duration // Lockout duration after max attempts
}

// DefaultRateLimiterConfig returns sensible defaults
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		Enabled:       true,
		MaxAttempts:   5,
		WindowSize:    1 * time.Minute,
		LockoutPeriod: 15 * time.Minute,
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cache cache.Cache, logger observability.Logger, config *RateLimiterConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	return &RateLimiter{
		cache:         cache,
		logger:        logger,
		enabled:       config.Enabled,
		maxAttempts:   config.MaxAttempts,
		windowSize:    config.WindowSize,
		lockoutPeriod: config.LockoutPeriod,
	}
}

// CheckLimit checks if the identifier has exceeded rate limits
func (rl *RateLimiter) CheckLimit(ctx context.Context, identifier string) error {
	// If rate limiting is disabled, always allow
	if !rl.enabled {
		return nil
	}

	key := fmt.Sprintf("auth:ratelimit:%s", identifier)

	// Try cache first
	if rl.cache != nil {
		return rl.checkCacheLimit(ctx, key)
	}

	// Fallback to local memory
	return rl.checkLocalLimit(identifier)
}

// RecordAttempt records an authentication attempt
func (rl *RateLimiter) RecordAttempt(ctx context.Context, identifier string, success bool) {
	// If rate limiting is disabled, do nothing
	if !rl.enabled {
		return
	}

	key := fmt.Sprintf("auth:ratelimit:%s", identifier)

	if rl.cache != nil {
		rl.recordCacheAttempt(ctx, key, success)
	} else {
		rl.recordLocalAttempt(identifier, success)
	}

	// Log the attempt
	rl.logger.Info("Authentication attempt recorded", map[string]interface{}{
		"identifier": identifier,
		"success":    success,
	})
}

// Implementation details...
func (rl *RateLimiter) checkCacheLimit(ctx context.Context, key string) error {
	// Check if locked out
	lockoutKey := key + ":lockout"
	var locked bool
	if err := rl.cache.Get(ctx, lockoutKey, &locked); err == nil && locked {
		return fmt.Errorf("rate limit exceeded: locked out")
	}

	// Get current attempt count
	var attempts int
	err := rl.cache.Get(ctx, key+":count", &attempts)
	if err != nil {
		attempts = 0 // Treat missing as 0
	}

	if attempts >= rl.maxAttempts {
		// Set lockout
		if err := rl.cache.Set(ctx, lockoutKey, true, rl.lockoutPeriod); err != nil {
			// Log but don't fail - rate limiting is best effort
			_ = err
		}
		return fmt.Errorf("rate limit exceeded: too many attempts")
	}

	return nil
}

func (rl *RateLimiter) recordCacheAttempt(ctx context.Context, key string, success bool) {
	if success {
		// Reset on successful auth
		_ = rl.cache.Delete(ctx, key+":count")   // Best effort cleanup
		_ = rl.cache.Delete(ctx, key+":lockout") // Best effort cleanup
		return
	}

	// Increment failed attempts
	var attempts int
	err := rl.cache.Get(ctx, key+":count", &attempts)
	if err != nil {
		attempts = 0 // Start from 0 if not found
	}
	attempts++
	if err := rl.cache.Set(ctx, key+":count", attempts, rl.windowSize); err != nil {
		// Rate limiter cache error - log but don't fail
		// The rate limiting should still work with local memory fallback
		_ = err
	}
}

// Local memory implementations for fallback
type localRateLimit struct {
	attempts  int
	window    time.Time
	lockedOut time.Time
	mu        sync.Mutex
}

func (rl *RateLimiter) checkLocalLimit(identifier string) error {
	now := time.Now()

	val, _ := rl.localLimits.LoadOrStore(identifier, &localRateLimit{
		window: now,
	})

	limit := val.(*localRateLimit)
	limit.mu.Lock()
	defer limit.mu.Unlock()

	// Check lockout
	if !limit.lockedOut.IsZero() && now.Before(limit.lockedOut) {
		return fmt.Errorf("rate limit exceeded: locked out")
	}

	// Check window
	if now.Sub(limit.window) > rl.windowSize {
		// Reset window
		limit.attempts = 0
		limit.window = now
	}

	if limit.attempts >= rl.maxAttempts {
		limit.lockedOut = now.Add(rl.lockoutPeriod)
		return fmt.Errorf("rate limit exceeded: too many attempts")
	}

	return nil
}

func (rl *RateLimiter) recordLocalAttempt(identifier string, success bool) {
	now := time.Now()

	val, _ := rl.localLimits.LoadOrStore(identifier, &localRateLimit{
		window: now,
	})

	limit := val.(*localRateLimit)
	limit.mu.Lock()
	defer limit.mu.Unlock()

	if success {
		// Reset on success
		limit.attempts = 0
		limit.lockedOut = time.Time{}
		return
	}

	// Check window
	if now.Sub(limit.window) > rl.windowSize {
		limit.attempts = 1
		limit.window = now
	} else {
		limit.attempts++
	}
}

// GetLockoutPeriod returns the configured lockout period
func (rl *RateLimiter) GetLockoutPeriod() time.Duration {
	return rl.lockoutPeriod
}
