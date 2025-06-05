package embedding

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCircuitBreaker(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	assert.NotNil(t, cb)
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, 0, cb.failureCount)
	assert.Equal(t, 0, cb.successCount)
}

func TestCircuitBreakerStates(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}

	cb := NewCircuitBreaker(config)

	t.Run("closed state allows requests", func(t *testing.T) {
		assert.True(t, cb.CanRequest())
		assert.Equal(t, StateClosed, cb.state)
	})

	t.Run("transitions to open after failure threshold", func(t *testing.T) {
		// Record failures
		cb.RecordFailure()
		assert.Equal(t, StateClosed, cb.state)
		cb.RecordFailure()
		assert.Equal(t, StateClosed, cb.state)
		cb.RecordFailure()
		
		// Should transition to open
		assert.Equal(t, StateOpen, cb.state)
		assert.False(t, cb.CanRequest())
	})

	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		// Wait for timeout
		time.Sleep(config.Timeout + 10*time.Millisecond)
		
		// Should allow request and transition to half-open
		assert.True(t, cb.CanRequest())
		assert.Equal(t, StateHalfOpen, cb.state)
	})

	t.Run("half-open allows limited requests", func(t *testing.T) {
		// Reset to half-open state
		cb.state = StateHalfOpen
		cb.halfOpenRequests = 0

		// Should allow up to HalfOpenMaxRequests
		assert.True(t, cb.CanRequest())
		assert.Equal(t, 1, cb.halfOpenRequests)
		
		assert.True(t, cb.CanRequest())
		assert.Equal(t, 2, cb.halfOpenRequests)
		
		// Should deny after limit
		assert.False(t, cb.CanRequest())
	})

	t.Run("transitions to closed after success threshold", func(t *testing.T) {
		// Reset to half-open
		cb.state = StateHalfOpen
		cb.successCount = 0
		
		// Record successes
		cb.RecordSuccess()
		assert.Equal(t, StateHalfOpen, cb.state)
		cb.RecordSuccess()
		
		// Should transition to closed
		assert.Equal(t, StateClosed, cb.state)
		assert.Equal(t, 0, cb.failureCount)
	})

	t.Run("transitions back to open on half-open failure", func(t *testing.T) {
		// Reset to half-open
		cb.state = StateHalfOpen
		cb.successCount = 0
		
		// Any failure should transition to open
		cb.RecordFailure()
		assert.Equal(t, StateOpen, cb.state)
	})
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    10,
		SuccessThreshold:    5,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 5,
	}

	cb := NewCircuitBreaker(config)

	// Test concurrent failures
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cb.CanRequest() {
				cb.RecordFailure()
			}
		}()
	}
	wg.Wait()

	// Should be in open state
	assert.Equal(t, StateOpen, cb.state)
	assert.GreaterOrEqual(t, cb.failureCount, config.FailureThreshold)
}

func TestCircuitBreakerHealthScore(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	t.Run("closed state health score", func(t *testing.T) {
		cb.state = StateClosed
		cb.failureCount = 0
		assert.Equal(t, 1.0, cb.HealthScore())

		cb.failureCount = 2
		assert.Equal(t, 0.6, cb.HealthScore())

		cb.failureCount = 4
		assert.Equal(t, 0.2, cb.HealthScore())
	})

	t.Run("half-open state health score", func(t *testing.T) {
		cb.state = StateHalfOpen
		assert.Equal(t, 0.5, cb.HealthScore())
	})

	t.Run("open state health score", func(t *testing.T) {
		cb.state = StateOpen
		assert.Equal(t, 0.0, cb.HealthScore())
	})
}

func TestCircuitBreakerStatus(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	// Record some activity
	cb.RecordFailure()
	cb.RecordFailure()

	status := cb.Status()
	require.NotNil(t, status)
	assert.Equal(t, string(StateClosed), status.State)
	assert.Equal(t, 2, status.FailureCount)
	assert.Equal(t, 0, status.SuccessCount)
	assert.NotZero(t, status.LastFailureTime)
	assert.NotZero(t, status.LastStateChangeTime)
}

func TestCircuitBreakerResetOnSuccess(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	// Record some failures (but not enough to open)
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, 2, cb.failureCount)

	// Success should reset failure count
	cb.RecordSuccess()
	assert.Equal(t, 0, cb.failureCount)
}

func TestCircuitBreakerStateTransitionTiming(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	// Move to open state
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.state)

	// Should not allow request immediately
	assert.False(t, cb.CanRequest())

	// Should not allow request before timeout
	time.Sleep(25 * time.Millisecond)
	assert.False(t, cb.CanRequest())

	// Should allow request after timeout
	time.Sleep(30 * time.Millisecond)
	assert.True(t, cb.CanRequest())
	assert.Equal(t, StateHalfOpen, cb.state)
}

// Benchmark tests
func BenchmarkCircuitBreakerCanRequest(b *testing.B) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 3,
	}

	cb := NewCircuitBreaker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.CanRequest()
	}
}

func BenchmarkCircuitBreakerRecordSuccess(b *testing.B) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 3,
	}

	cb := NewCircuitBreaker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.RecordSuccess()
	}
}

func BenchmarkCircuitBreakerRecordFailure(b *testing.B) {
	config := CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 3,
	}

	cb := NewCircuitBreaker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.RecordFailure()
		// Reset if we hit open state to keep benchmark consistent
		if cb.state == StateOpen {
			cb.state = StateClosed
			cb.failureCount = 0
		}
	}
}