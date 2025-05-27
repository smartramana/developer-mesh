package resilience

import (
	"context"
	"time"
)

// WithTimeout executes a function with a timeout
func WithTimeout[T any](operation func(context.Context) (T, error), timeout time.Duration) (T, error) {
	return WithTimeoutContext(context.Background(), operation, timeout)
}

// WithTimeoutContext executes a function with a timeout using the provided parent context
func WithTimeoutContext[T any](ctx context.Context, operation func(context.Context) (T, error), timeout time.Duration) (T, error) {
	config := TimeoutConfig{
		Timeout:     timeout,
		GracePeriod: 0,
	}
	return ExecuteWithTimeout(ctx, config, operation)
}

// ExecuteWithTimeoutSimple executes a void function with a timeout
func ExecuteWithTimeoutSimple(ctx context.Context, fn func(context.Context) error, timeout time.Duration) error {
	config := TimeoutConfig{
		Timeout:     timeout,
		GracePeriod: 0,
	}
	_, err := ExecuteWithTimeout(ctx, config, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}

// TimeoutMiddleware creates a middleware that adds a timeout to a function
func TimeoutMiddleware[T any](timeout time.Duration) func(func(context.Context) (T, error)) func(context.Context) (T, error) {
	return func(operation func(context.Context) (T, error)) func(context.Context) (T, error) {
		return func(ctx context.Context) (T, error) {
			return WithTimeoutContext(ctx, operation, timeout)
		}
	}
}

// ExecuteTimeoutMiddleware creates a middleware that adds a timeout to a void function
func ExecuteTimeoutMiddleware(timeout time.Duration) func(func(context.Context) error) func(context.Context) error {
	return func(operation func(context.Context) error) func(context.Context) error {
		return func(ctx context.Context) error {
			return ExecuteWithTimeoutSimple(ctx, operation, timeout)
		}
	}
}

// DynamicTimeoutMiddleware creates a middleware with dynamic timeout
func DynamicTimeoutMiddleware[T any, P any](timeoutProvider func(context.Context, P) time.Duration) func(func(context.Context, P) (T, error)) func(context.Context, P) (T, error) {
	return func(operation func(context.Context, P) (T, error)) func(context.Context, P) (T, error) {
		return func(ctx context.Context, params P) (T, error) {
			timeout := timeoutProvider(ctx, params)
			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			return operation(timeoutCtx, params)
		}
	}
}
