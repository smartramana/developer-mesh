package adapters

import (
	"time"
	
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/safety"
)

// Ensure we're compatible with the interfaces package definition
type Adapter = interfaces.Adapter

// BaseAdapter provides common functionality for adapters
type BaseAdapter struct {
	RetryMax   int
	RetryDelay time.Duration
	SafeMode   bool // When true, enforces safety checks on all operations
}

// CallWithRetry executes a function with retry logic
func (b *BaseAdapter) CallWithRetry(fn func() error) error {
	var err error
	for i := 0; i <= b.RetryMax; i++ {
		if i > 0 {
			// Exponential backoff
			time.Sleep(b.RetryDelay * time.Duration(1<<uint(i-1)))
		}

		err = fn()
		if err == nil {
			return nil
		}

		// Check if we should retry based on the error
		if !b.isRetryable(err) {
			return err
		}
	}

	return err
}

// isRetryable determines if an error should trigger a retry
func (b *BaseAdapter) isRetryable(err error) bool {
	// Implement logic to determine if an error is retryable
	// This could check for network errors, rate limits, etc.
	// In a production system, this would have more sophisticated logic
	return true
}

// IsSafeOperation determines if an operation is safe to perform
// This is a default implementation that can be overridden by specific adapters
func (b *BaseAdapter) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	// If safe mode is disabled, all operations are considered safe
	if !b.SafeMode {
		return true, nil
	}
	
	// Use the default safety check
	return safety.DefaultCheck(operation, params)
}
