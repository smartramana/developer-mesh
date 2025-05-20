package resilience

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// RetryConfig defines configuration for retries
type RetryConfig struct {
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	MaxElapsedTime  time.Duration
	RetryIfFn       func(error) bool
}

// Retry retries a function with exponential backoff
func Retry(ctx context.Context, config RetryConfig, operation func() error) error {
	// Create exponential backoff
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = config.InitialInterval
	b.MaxInterval = config.MaxInterval
	b.Multiplier = config.Multiplier
	b.MaxElapsedTime = config.MaxElapsedTime
	
	// If MaxRetries is set, limit the number of retries
	var backoffWithRetries backoff.BackOff = b
	if config.MaxRetries > 0 {
		backoffWithRetries = backoff.WithMaxRetries(b, uint64(config.MaxRetries))
	}
	
	// Create context-aware backoff
	ctxBackoff := backoff.WithContext(backoffWithRetries, ctx)
	
	// Define operation to retry
	return backoff.Retry(func() error {
		err := operation()
		
		// Check if we should retry this error
		if err != nil && config.RetryIfFn != nil && !config.RetryIfFn(err) {
			// Return a special error to stop retries
			return backoff.Permanent(err)
		}
		
		return err
	}, ctxBackoff)
}

// RetryWithResult retries a function with exponential backoff and returns a result
func RetryWithResult[T any](ctx context.Context, config RetryConfig, operation func() (T, error)) (T, error) {
	var result T
	var resultErr error
	
	// Wrap the operation to use with Retry
	operationWrapper := func() error {
		var err error
		result, err = operation()
		if err != nil {
			return err
		}
		
		// Store the result and return nil error to indicate success
		resultErr = nil
		return nil
	}
	
	// Execute the retry
	err := Retry(ctx, config, operationWrapper)
	if err != nil {
		return result, err
	}
	
	return result, resultErr
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		MaxElapsedTime:  30 * time.Second,
		RetryIfFn: func(err error) bool {
			// By default, retry on all errors
			return true
		},
	}
}

// IsRetryableError determines if an error should be retried
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Define your criteria for retryable errors
	var retryableError RetryableError
	if errors.As(err, &retryableError) {
		return true
	}
	
	// Add more criteria as needed, such as network errors, timeouts, etc.
	
	return false
}

// RetryableError is an error that can be retried
type RetryableError struct {
	Err error
}

// Error implements the error interface
func (e RetryableError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error) RetryableError {
	return RetryableError{Err: err}
}
