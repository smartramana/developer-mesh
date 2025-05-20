package resilience

import (
	"time"
)

// Allow checks if a request is allowed by the rate limiter
func (r *RateLimiter) Allow() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	// Refill tokens based on time passed since last refill
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	
	if elapsed > 0 {
		// Calculate how many tokens to add based on elapsed time
		tokensToAdd := int(elapsed.Seconds() * float64(r.config.Limit) / r.config.Period.Seconds())
		
		if tokensToAdd > 0 {
			// Add tokens, but don't exceed the limit
			r.tokens = min(r.tokens+tokensToAdd, r.config.Limit)
			r.lastRefill = now
		}
	}
	
	// Check if we have enough tokens
	if r.tokens > 0 {
		r.tokens--
		return true
	}
	
	return false
}

// AdjustRateLimit adjusts the rate limiter based on API rate limit information
func (r *RateLimiter) AdjustRateLimit(info interface{}) {
	// This method can be implemented if needed to dynamically adjust rate limits
	// based on information returned from the API
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
