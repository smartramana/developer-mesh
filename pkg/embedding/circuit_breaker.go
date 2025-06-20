package embedding

import (
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config              CircuitBreakerConfig
	state               CircuitBreakerState
	failureCount        int
	successCount        int
	lastFailureTime     time.Time
	lastStateChangeTime time.Time
	halfOpenRequests    int
	mu                  sync.RWMutex
}

// CircuitBreakerConfig configures a circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold    int
	SuccessThreshold    int
	Timeout             time.Duration
	HalfOpenMaxRequests int
}

// CircuitBreakerStatus represents the current status
type CircuitBreakerStatus struct {
	State               string    `json:"state"`
	FailureCount        int       `json:"failure_count"`
	SuccessCount        int       `json:"success_count"`
	LastFailureTime     time.Time `json:"last_failure_time,omitempty"`
	LastStateChangeTime time.Time `json:"last_state_change_time"`
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:              config,
		state:               StateClosed,
		lastStateChangeTime: time.Now(),
	}
}

// CanRequest checks if a request can be made
func (cb *CircuitBreaker) CanRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastStateChangeTime) >= cb.config.Timeout {
			cb.transitionToHalfOpen()
			return true
		}
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests < cb.config.HalfOpenMaxRequests {
			cb.halfOpenRequests++
			return true
		}
		return false
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.transitionToClosed()
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.transitionToOpen()
		}
	case StateHalfOpen:
		// Any failure in half-open state transitions back to open
		cb.transitionToOpen()
	}
}

// Status returns the current status
func (cb *CircuitBreaker) Status() *CircuitBreakerStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return &CircuitBreakerStatus{
		State:               string(cb.state),
		FailureCount:        cb.failureCount,
		SuccessCount:        cb.successCount,
		LastFailureTime:     cb.lastFailureTime,
		LastStateChangeTime: cb.lastStateChangeTime,
	}
}

// HealthScore returns a health score between 0 and 1
func (cb *CircuitBreaker) HealthScore() float64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		// Healthy state with decreasing score based on failure count
		return 1.0 - (float64(cb.failureCount) / float64(cb.config.FailureThreshold))
	case StateHalfOpen:
		// Half health for half-open state
		return 0.5
	case StateOpen:
		// Unhealthy
		return 0.0
	default:
		return 0.0
	}
}

// Private methods for state transitions

func (cb *CircuitBreaker) transitionToOpen() {
	cb.state = StateOpen
	cb.lastStateChangeTime = time.Now()
	cb.successCount = 0
	cb.halfOpenRequests = 0
}

func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.state = StateHalfOpen
	cb.lastStateChangeTime = time.Now()
	cb.successCount = 0
	cb.halfOpenRequests = 0
}

func (cb *CircuitBreaker) transitionToClosed() {
	cb.state = StateClosed
	cb.lastStateChangeTime = time.Now()
	cb.failureCount = 0
	cb.halfOpenRequests = 0
}
