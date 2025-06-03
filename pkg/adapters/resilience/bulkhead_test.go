package resilience

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBulkheadConfig(t *testing.T) {
	// Test default values
	config := BulkheadConfig{
		Name:           "test-bulkhead",
		MaxConcurrent:  0, // Should get default value
		MaxWaitingTime: 0, // No wait time
	}

	bulkhead := NewBulkhead(config)
	assert.NotNil(t, bulkhead)
	assert.Equal(t, "test-bulkhead", bulkhead.Name())

	// The default value for MaxConcurrent should be applied
	assert.Equal(t, 10, bulkhead.RemainingExecutions())
}

func TestBulkheadExecute(t *testing.T) {
	// Create a bulkhead with 3 max concurrent operations
	config := BulkheadConfig{
		Name:           "test-execute",
		MaxConcurrent:  3,
		MaxWaitingTime: 100 * time.Millisecond,
	}

	bulkhead := NewBulkhead(config)
	assert.NotNil(t, bulkhead)

	// Test successful execution
	t.Run("successful execution", func(t *testing.T) {
		executed := false

		result, err := bulkhead.Execute(context.Background(), func() (interface{}, error) {
			executed = true
			return "success", nil
		})

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		assert.True(t, executed)
	})

	// Test error execution
	t.Run("error execution", func(t *testing.T) {
		executed := false
		testErr := fmt.Errorf("test error")

		result, err := bulkhead.Execute(context.Background(), func() (interface{}, error) {
			executed = true
			return nil, testErr
		})

		assert.Error(t, err)
		assert.Equal(t, testErr, err)
		assert.Nil(t, result)
		assert.True(t, executed)
	})

	// Test concurrent executions
	t.Run("concurrent executions", func(t *testing.T) {
		var concurrentCount int32
		var maxConcurrent int32
		var wg sync.WaitGroup
		numOperations := 10

		// Start multiple operations
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				_, err := bulkhead.Execute(context.Background(), func() (interface{}, error) {
					// Increment concurrent count
					current := atomic.AddInt32(&concurrentCount, 1)
					defer atomic.AddInt32(&concurrentCount, -1)

					// Track max concurrency
					for {
						max := atomic.LoadInt32(&maxConcurrent)
						if current <= max {
							break
						}
						if atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
							break
						}
					}

					// Simulate work
					time.Sleep(50 * time.Millisecond)

					return fmt.Sprintf("result-%d", index), nil
				})

				// Some operations may be rejected due to timeout
				if err != nil {
					assert.Contains(t, err.Error(), "rejected execution")
				}
			}(i)
		}

		// Wait for all operations to complete
		wg.Wait()

		// Verify max concurrency did not exceed the limit
		assert.LessOrEqual(t, maxConcurrent, int32(3),
			"Max concurrency (%d) should not exceed bulkhead limit (3)", maxConcurrent)
	})
}

func TestBulkheadTimeout(t *testing.T) {
	// Create a bulkhead with 1 max concurrent operation and short timeout
	config := BulkheadConfig{
		Name:           "test-timeout",
		MaxConcurrent:  1,
		MaxWaitingTime: 50 * time.Millisecond,
	}

	bulkhead := NewBulkhead(config)
	assert.NotNil(t, bulkhead)

	// Start a long-running operation that occupies the bulkhead
	var firstDone bool
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		_, _ = bulkhead.Execute(context.Background(), func() (interface{}, error) {
			// Occupy the bulkhead for a while
			time.Sleep(200 * time.Millisecond)
			firstDone = true
			return "first", nil
		})
	}()

	// Give the first operation time to start
	time.Sleep(10 * time.Millisecond)

	// Try to execute another operation while the bulkhead is full
	start := time.Now()
	result, err := bulkhead.Execute(context.Background(), func() (interface{}, error) {
		return "second", nil
	})
	elapsed := time.Since(start)

	// The second operation should be rejected after the timeout
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rejected execution")
	assert.Nil(t, result)

	// Timeout should have occurred after MaxWaitingTime
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	assert.Less(t, elapsed, 150*time.Millisecond, "Operation should timeout quickly")

	// Wait for the first operation to complete
	wg.Wait()
	assert.True(t, firstDone)
}

func TestBulkheadCounts(t *testing.T) {
	// Create a bulkhead
	config := BulkheadConfig{
		Name:           "test-counts",
		MaxConcurrent:  5,
		MaxWaitingTime: 0,
	}

	bulkhead := NewBulkhead(config)
	assert.NotNil(t, bulkhead)

	// Initial counts
	assert.Equal(t, 0, bulkhead.CurrentExecutions())
	assert.Equal(t, 5, bulkhead.RemainingExecutions())

	// Acquire 3 slots
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, _ = bulkhead.Execute(context.Background(), func() (interface{}, error) {
				time.Sleep(100 * time.Millisecond)
				return nil, nil
			})
		}()
	}

	// Wait for operations to start
	time.Sleep(10 * time.Millisecond)

	// Check counts
	assert.Equal(t, 3, bulkhead.CurrentExecutions())
	assert.Equal(t, 2, bulkhead.RemainingExecutions())

	// Wait for operations to complete
	wg.Wait()

	// Check counts after completion
	assert.Equal(t, 0, bulkhead.CurrentExecutions())
	assert.Equal(t, 5, bulkhead.RemainingExecutions())
}
