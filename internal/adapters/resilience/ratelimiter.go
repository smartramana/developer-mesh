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
		limiter: rate.NewLimiter(rate.Limit(config.Rate), config.Burst),
		config:  config,
	}
}

// Wait blocks until rate limit allows an event or context is done
func (rl *DefaultRateLimiter) Wait(ctx context.Context) error {
	// Create a context with timeout if WaitLimit is set
	if rl.config.WaitLimit > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rl.config.WaitLimit)
		defer cancel()
	}
	
	return rl.limiter.Wait(ctx)
}

// Allow returns true if the rate limit allows an event at time now
func (rl *DefaultRateLimiter) Allow() bool {
	return rl.limiter.Allow()
}

// Name returns the rate limiter name
func (rl *DefaultRateLimiter) Name() string {
	return rl.config.Name
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
func (m *RateLimiterManager) Execute(ctx context.Context, name string, fn func() (interface{}, error)) (interface{}, error) {
	// Wait for rate limiter
	if err := m.Wait(ctx, name); err != nil {
		return nil, fmt.Errorf("rate limit exceeded for %s: %w", name, err)
	}
	
	// Execute the function
	return fn()
}
