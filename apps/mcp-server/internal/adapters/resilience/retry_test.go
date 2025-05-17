package resilience

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryBasic(t *testing.T) {
	// Create a test context
	ctx := context.Background()

	// Test successful operation on first try
	t.Run("success on first attempt", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return nil
		}
		
		config := DefaultRetryConfig()
		
		err := Retry(ctx, config, operation)
		
		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	// Test operation that fails then succeeds
	t.Run("success after retry", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		}
		
		config := DefaultRetryConfig()
		
		err := Retry(ctx, config, operation)
		
		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})
	
	// Test operation that always fails
	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return errors.New("persistent error")
		}
		
		config := DefaultRetryConfig()
		config.MaxRetries = 3 // Override max retries
		
		err := Retry(ctx, config, operation)
		
		assert.Error(t, err)
		assert.Equal(t, 4, attempts) // Initial attempt + 3 retries
	})
}

func TestRetryWithResultBasic(t *testing.T) {
	// Create a test context
	ctx := context.Background()

	// Test successful operation on first try
	t.Run("success on first attempt", func(t *testing.T) {
		attempts := 0
		operation := func() (string, error) {
			attempts++
			return "success", nil
		}
		
		config := DefaultRetryConfig()
		
		result, err := RetryWithResult(ctx, config, operation)
		
		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		assert.Equal(t, 1, attempts)
	})

	// Test operation that fails then succeeds
	t.Run("success after retry", func(t *testing.T) {
		attempts := 0
		operation := func() (string, error) {
			attempts++
			if attempts < 3 {
				return "", errors.New("temporary error")
			}
			return "success after retry", nil
		}
		
		config := DefaultRetryConfig()
		
		result, err := RetryWithResult(ctx, config, operation)
		
		assert.NoError(t, err)
		assert.Equal(t, "success after retry", result)
		assert.Equal(t, 3, attempts)
	})
	
	// Test operation that always fails
	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		operation := func() (string, error) {
			attempts++
			return "", errors.New("persistent error")
		}
		
		config := DefaultRetryConfig()
		config.MaxRetries = 3 // Override max retries
		
		result, err := RetryWithResult(ctx, config, operation)
		
		assert.Error(t, err)
		assert.Equal(t, "", result)
		assert.Equal(t, 4, attempts) // Initial attempt + 3 retries
	})
}

func TestRetryIfFn(t *testing.T) {
	// Create a test context
	ctx := context.Background()

	// Test retry if function
	t.Run("custom retry if function", func(t *testing.T) {
		retryableErr := errors.New("retryable error")
		nonRetryableErr := errors.New("non-retryable error")
		
		attempts := 0
		operation := func() error {
			attempts++
			
			if attempts == 1 {
				return retryableErr
			} else if attempts == 2 {
				return nonRetryableErr
			}
			
			return nil
		}
		
		config := DefaultRetryConfig()
		config.RetryIfFn = func(err error) bool {
			return err == retryableErr
		}
		
		err := Retry(ctx, config, operation)
		
		assert.Error(t, err)
		assert.Equal(t, nonRetryableErr, err)
		assert.Equal(t, 2, attempts)
	})
}

func TestRetryableErrorType(t *testing.T) {
	// Test creating and using RetryableError
	t.Run("retryable error", func(t *testing.T) {
		originalErr := errors.New("original error")
		retryableErr := NewRetryableError(originalErr)
		
		// Test error string
		assert.Equal(t, originalErr.Error(), retryableErr.Error())
		
		// Test unwrapping
		assert.Equal(t, originalErr, errors.Unwrap(retryableErr))
		
		// Test IsRetryableError function
		assert.True(t, IsRetryableError(retryableErr))
		assert.False(t, IsRetryableError(originalErr))
		assert.False(t, IsRetryableError(nil))
	})
}

func TestRetryWithContext(t *testing.T) {
	// Test that retry respects context cancellation
	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		
		attempts := 0
		operation := func() error {
			attempts++
			time.Sleep(50 * time.Millisecond)
			return errors.New("always fails")
		}
		
		config := DefaultRetryConfig()
		config.InitialInterval = 100 * time.Millisecond
		
		// Cancel the context after the first attempt
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		
		err := Retry(ctx, config, operation)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
		assert.Equal(t, 1, attempts, "Should only make one attempt before context cancellation")
	})
	
	// Test that retry respects context timeout
	t.Run("context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		
		attempts := 0
		operation := func() error {
			attempts++
			time.Sleep(50 * time.Millisecond)
			return errors.New("always fails")
		}
		
		config := DefaultRetryConfig()
		config.InitialInterval = 20 * time.Millisecond
		
		err := Retry(ctx, config, operation)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
		assert.GreaterOrEqual(t, attempts, 2, "Should make at least 2 attempts before timeout")
		assert.LessOrEqual(t, attempts, 4, "Should not make too many attempts")
	})
}

func TestDefaultRetryConfigValues(t *testing.T) {
	// Test the default retry configuration
	config := DefaultRetryConfig()
	
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.InitialInterval)
	assert.Equal(t, 10*time.Second, config.MaxInterval)
	assert.Equal(t, 2.0, config.Multiplier)
	assert.Equal(t, 30*time.Second, config.MaxElapsedTime)
	assert.NotNil(t, config.RetryIfFn)
	
	// Test that RetryIfFn returns true for any error
	assert.True(t, config.RetryIfFn(errors.New("any error")))
}

func TestRetryIntegration(t *testing.T) {
	// This test simulates a more realistic scenario with a function that fails in different ways
	ctx := context.Background()
	
	// Test retry with eventual success
	t.Run("success after retries", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			if attempts <= 2 {
				return errors.New("temporary error, needs retry")
			}
			return nil
		}
		
		config := DefaultRetryConfig()
		
		err := Retry(ctx, config, operation)
		assert.NoError(t, err)
	})
	
	// Test retry with permanent failure
	t.Run("permanent failure", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return errors.New("permanent error, but retriable")
		}
		
		config := DefaultRetryConfig()
		config.MaxRetries = 2
		
		err := Retry(ctx, config, operation)
		assert.Error(t, err)
	})
	
	// Test with non-retriable error
	t.Run("non-retriable error", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return fmt.Errorf("non-retriable error: %w", errors.New("stop retrying"))
		}
		
		config := DefaultRetryConfig()
		config.RetryIfFn = func(err error) bool {
			return !strings.Contains(err.Error(), "non-retriable error")
		}
		
		err := Retry(ctx, config, operation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non-retriable error")
	})
}
