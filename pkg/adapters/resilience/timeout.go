package resilience

import (
	"context"
	"fmt"
	"time"
)

// TimeoutConfig defines configuration for timeouts
type TimeoutConfig struct {
	Timeout     time.Duration
	GracePeriod time.Duration // Additional time for cleanup
}

// ExecuteWithTimeout executes a function with a timeout
func ExecuteWithTimeout[T any](ctx context.Context, config TimeoutConfig, operation func(context.Context) (T, error)) (T, error) {
	var result T

	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	// Create a channel for the result
	resultCh := make(chan struct {
		value T
		err   error
	}, 1)

	// Execute the operation in a goroutine
	go func() {
		value, err := operation(timeoutCtx)
		resultCh <- struct {
			value T
			err   error
		}{value, err}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultCh:
		return res.value, res.err
	case <-timeoutCtx.Done():
		// If we have a grace period, wait a bit longer for the operation to complete
		if config.GracePeriod > 0 {
			graceCh := time.After(config.GracePeriod)
			select {
			case res := <-resultCh:
				return res.value, res.err
			case <-graceCh:
				// Grace period expired, return timeout error
				return result, fmt.Errorf("operation timed out after %v (plus %v grace period): %w",
					config.Timeout, config.GracePeriod, context.DeadlineExceeded)
			}
		}

		return result, fmt.Errorf("operation timed out after %v: %w",
			config.Timeout, context.DeadlineExceeded)
	}
}

// DefaultTimeoutConfig returns a default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Timeout:     10 * time.Second,
		GracePeriod: 2 * time.Second,
	}
}
