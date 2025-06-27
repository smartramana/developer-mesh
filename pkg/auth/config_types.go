package auth

import (
	"time"
)

// AuthSystemConfig holds complete auth system configuration
type AuthSystemConfig struct {
	Service     *ServiceConfig
	RateLimiter *RateLimiterConfig
	APIKeys     map[string]APIKeySettings
}

// TestRateLimiterConfig returns test-friendly defaults
func TestRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		Enabled:       true, // Enable rate limiting for tests
		MaxAttempts:   3,    // Lower for faster tests
		WindowSize:    1 * time.Minute,
		LockoutPeriod: 5 * time.Minute,
	}
}
