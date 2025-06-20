package resilience

import (
	"context"
	"time"
)

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       float64
}

// TimeoutPolicy defines timeout behavior
type TimeoutPolicy struct {
	Timeout time.Duration
}

// BulkheadPolicy defines bulkhead behavior
type BulkheadPolicy struct {
	MaxConcurrent int
	QueueSize     int
	Timeout       time.Duration
}

// CircuitBreakerWithExecute extends CircuitBreaker with Execute method
type CircuitBreakerWithExecute interface {
	CircuitBreaker
	Execute(ctx context.Context, fn func() error) error
	ExecuteWithFallback(ctx context.Context, fn func() error, fallback func() error) error
}
