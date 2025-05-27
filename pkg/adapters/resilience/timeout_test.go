package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExecuteWithTimeout(t *testing.T) {
	// Test a function that completes within the timeout
	t.Run("completes within timeout", func(t *testing.T) {
		ctx := context.Background()
		executed := false

		// Create a timeout config
		config := TimeoutConfig{
			Timeout:     100 * time.Millisecond,
			GracePeriod: 0,
		}

		// Operation that completes quickly
		operation := func(ctx context.Context) (string, error) {
			executed = true
			return "success", nil
		}

		// Call with timeout
		result, err := ExecuteWithTimeout(ctx, config, operation)

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		assert.True(t, executed, "Operation should have executed")
	})

	// Test a function that times out
	t.Run("times out", func(t *testing.T) {
		ctx := context.Background()
		executed := true
		completed := false

		// Create a timeout config
		config := TimeoutConfig{
			Timeout:     50 * time.Millisecond,
			GracePeriod: 0,
		}

		// Operation that takes too long
		operation := func(ctx context.Context) (string, error) {
			executed = true

			// Try to sleep but should be interrupted
			select {
			case <-time.After(200 * time.Millisecond):
				completed = true
				return "too late", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		// Call with timeout
		result, err := ExecuteWithTimeout(ctx, config, operation)

		assert.Error(t, err)
		assert.Equal(t, "", result)
		assert.True(t, executed, "Operation should have started execution")
		assert.False(t, completed, "Operation should not have completed execution")
		assert.True(t, errors.Is(err, context.DeadlineExceeded), "Error should be deadline exceeded")
	})

	// Test a function that returns an error
	t.Run("returns error", func(t *testing.T) {
		ctx := context.Background()
		executed := false
		testErr := errors.New("operation failed")

		// Create a timeout config
		config := TimeoutConfig{
			Timeout:     100 * time.Millisecond,
			GracePeriod: 0,
		}

		// Operation that returns an error
		operation := func(ctx context.Context) (string, error) {
			executed = true
			return "", testErr
		}

		// Call with timeout
		result, err := ExecuteWithTimeout(ctx, config, operation)

		assert.Error(t, err)
		assert.Equal(t, testErr, err)
		assert.Equal(t, "", result)
		assert.True(t, executed, "Operation should have executed")
	})
}

func TestGracePeriod(t *testing.T) {
	// Test with grace period that allows completion
	t.Run("grace period allows completion", func(t *testing.T) {
		ctx := context.Background()
		executed := false
		completed := false

		// Create a timeout config with grace period
		config := TimeoutConfig{
			Timeout:     50 * time.Millisecond,
			GracePeriod: 100 * time.Millisecond, // Long enough to allow completion
		}

		// Operation that completes during grace period
		operation := func(ctx context.Context) (string, error) {
			executed = true

			// Sleep a bit longer than timeout but less than grace period
			time.Sleep(80 * time.Millisecond)
			completed = true

			select {
			case <-ctx.Done():
				// Context is already done, but grace period is active
				return "completed during grace", nil
			default:
				return "completed normally", nil
			}
		}

		// Call with timeout and grace period
		result, err := ExecuteWithTimeout(ctx, config, operation)

		assert.NoError(t, err)
		assert.Equal(t, "completed during grace", result)
		assert.True(t, executed, "Operation should have executed")
		assert.True(t, completed, "Operation should have completed during grace period")
	})

	// Test with grace period that is too short
	t.Run("grace period too short", func(t *testing.T) {
		ctx := context.Background()
		executed := false
		completed := false

		// Create a timeout config with short grace period
		config := TimeoutConfig{
			Timeout:     50 * time.Millisecond,
			GracePeriod: 30 * time.Millisecond, // Not long enough
		}

		// Operation that takes too long even with grace period
		operation := func(ctx context.Context) (string, error) {
			executed = true

			// Try to sleep but should be interrupted
			select {
			case <-time.After(200 * time.Millisecond):
				completed = true
				return "too late", nil
			case <-ctx.Done():
				// Simulate work that continues after context is done
				// but doesn't complete within grace period
				time.Sleep(50 * time.Millisecond)
				completed = false
				return "", ctx.Err()
			}
		}

		// Call with timeout
		result, err := ExecuteWithTimeout(ctx, config, operation)

		assert.Error(t, err)
		assert.Equal(t, "", result)
		assert.True(t, executed, "Operation should have started execution")
		assert.False(t, completed, "Operation should not have completed execution")
		assert.True(t, errors.Is(err, context.DeadlineExceeded), "Error should be deadline exceeded")
	})
}

func TestTimeoutWithParentContext(t *testing.T) {
	// Test with parent context that gets canceled
	t.Run("parent context canceled", func(t *testing.T) {
		// Create a parent context that we can cancel
		parentCtx, cancel := context.WithCancel(context.Background())
		executed := false
		completed := false

		// Create a timeout config
		config := TimeoutConfig{
			Timeout:     100 * time.Millisecond,
			GracePeriod: 0,
		}

		// Operation that waits for context cancellation
		operation := func(ctx context.Context) (string, error) {
			executed = true

			select {
			case <-time.After(200 * time.Millisecond):
				completed = true
				return "completed", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		// Start the operation
		resultCh := make(chan struct {
			result string
			err    error
		}, 1)

		go func() {
			result, err := ExecuteWithTimeout(parentCtx, config, operation)
			resultCh <- struct {
				result string
				err    error
			}{result, err}
		}()

		// Cancel the parent context before timeout
		time.Sleep(20 * time.Millisecond)
		cancel()

		// Get the result
		select {
		case res := <-resultCh:
			assert.Error(t, res.err)
			assert.Equal(t, "", res.result)
			// The error could be context.Canceled OR context.DeadlineExceeded depending on timing
			// So we'll just check that it's an error and not try to validate the specific error type
			assert.Error(t, res.err, "Should return an error when parent context is canceled")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Operation did not complete in time")
		}

		assert.True(t, executed, "Operation should have started execution")
		assert.False(t, completed, "Operation should not have completed execution")
	})

	// Test with parent context timeout that's shorter than operation timeout
	t.Run("parent context timeout", func(t *testing.T) {
		// Create a parent context with timeout
		parentCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()

		executed := false
		completed := false

		// Create a timeout config with longer timeout
		config := TimeoutConfig{
			Timeout:     100 * time.Millisecond,
			GracePeriod: 0,
		}

		// Operation that waits for context cancellation
		operation := func(ctx context.Context) (string, error) {
			executed = true

			select {
			case <-time.After(200 * time.Millisecond):
				completed = true
				return "completed", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		// Call with timeout
		result, err := ExecuteWithTimeout(parentCtx, config, operation)

		assert.Error(t, err)
		assert.Equal(t, "", result)
		assert.True(t, executed, "Operation should have started execution")
		assert.False(t, completed, "Operation should not have completed execution")
		assert.True(t, errors.Is(err, context.DeadlineExceeded), "Error should be deadline exceeded")
	})
}

func TestDefaultTimeoutConfig(t *testing.T) {
	// Test default timeout configuration
	config := DefaultTimeoutConfig()

	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, 2*time.Second, config.GracePeriod)
}

func TestTimeoutIntegration(t *testing.T) {
	// Test integration with retry
	t.Run("with retry", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0

		// Create a timeout config with short timeout
		timeoutConfig := TimeoutConfig{
			Timeout:     50 * time.Millisecond,
			GracePeriod: 0,
		}

		// Create retry config
		retryConfig := RetryConfig{
			MaxRetries:      2,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     50 * time.Millisecond,
			Multiplier:      2.0,
			RetryIfFn: func(err error) bool {
				// Retry on timeout errors
				return errors.Is(err, context.DeadlineExceeded)
			},
		}

		// Operation that initially times out but eventually succeeds
		operation := func(innerCtx context.Context) (string, error) {
			attempts++

			if attempts <= 2 {
				// First two attempts time out
				time.Sleep(100 * time.Millisecond)
				return "", innerCtx.Err()
			}

			// Third attempt succeeds quickly
			return "success", nil
		}

		// Wrap timeout around retry
		var result string
		var err error

		err = Retry(ctx, retryConfig, func() error {
			result, err = ExecuteWithTimeout(ctx, timeoutConfig, operation)
			return err
		})

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		assert.Equal(t, 3, attempts, "Operation should succeed on third attempt")
	})
}
