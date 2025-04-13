package adapters

import (
	"context"
	"time"
)

// Adapter defines the interface for all external service adapters
type Adapter interface {
	// Initialize sets up the adapter with configuration
	Initialize(ctx context.Context, config interface{}) error

	// GetData retrieves data from the external service
	GetData(ctx context.Context, query interface{}) (interface{}, error)
	
	// ExecuteAction executes an action with context awareness
	ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error)
	
	// HandleWebhook processes webhook events from the external service
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error
	
	// Subscribe registers a callback for a specific event type
	Subscribe(eventType string, callback func(interface{})) error

	// Health returns the health status of the adapter
	Health() string

	// Close gracefully shuts down the adapter
	Close() error
}

// BaseAdapter provides common functionality for adapters
type BaseAdapter struct {
	RetryMax   int
	RetryDelay time.Duration
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
