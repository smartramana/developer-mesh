package resilience

import (
	"sync"
	"time"
)

// DefaultPeriod defines the default period for rate limiters
var DefaultPeriod = time.Minute

// RateLimiterConfig holds configuration for a rate limiter
type RateLimiterConfig struct {
	Limit       int           // Maximum requests per period
	Period      time.Duration // Time period for the limit
	BurstFactor int           // Burst factor (multiplier for limit for short bursts)
}

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	name       string
	config     RateLimiterConfig
	tokens     int
	lastRefill time.Time
	mutex      sync.Mutex
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(name string, config RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		name:       name,
		config:     config,
		tokens:     config.Limit,
		lastRefill: time.Now(),
	}
}

// RateLimiterManager manages multiple rate limiters
type RateLimiterManager struct {
	limiters map[string]*RateLimiter
	mutex    sync.RWMutex
}

// NewRateLimiterManager creates a new rate limiter manager
func NewRateLimiterManager(defaultConfigs map[string]RateLimiterConfig) *RateLimiterManager {
	manager := &RateLimiterManager{
		limiters: make(map[string]*RateLimiter),
	}

	// Create rate limiters from default configs
	for name, config := range defaultConfigs {
		manager.limiters[name] = NewRateLimiter(name, config)
	}

	return manager
}

// GetRateLimiter gets a rate limiter by name, creating it if it doesn't exist
func (m *RateLimiterManager) GetRateLimiter(name string) *RateLimiter {
	m.mutex.RLock()
	limiter, exists := m.limiters[name]
	m.mutex.RUnlock()

	if exists {
		return limiter
	}

	// Use a default configuration if the rate limiter doesn't exist
	defaultConfig := RateLimiterConfig{
		Limit:       100,
		Period:      time.Minute,
		BurstFactor: 3,
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check again in case it was created while we were waiting for the lock
	limiter, exists = m.limiters[name]
	if exists {
		return limiter
	}

	// Create a new rate limiter
	limiter = NewRateLimiter(name, defaultConfig)
	m.limiters[name] = limiter

	return limiter
}
