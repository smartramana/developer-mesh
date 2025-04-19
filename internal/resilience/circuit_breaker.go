package resilience

import (
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

// Circuit breaker states
const (
	CircuitBreakerClosed CircuitBreakerState = iota // Normal operation, requests allowed
	CircuitBreakerOpen                              // Tripped, requests blocked
	CircuitBreakerHalfOpen                          // Testing if service is healthy
)

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold   int           // Number of failures before tripping
	ResetTimeout       time.Duration // Time before attempting retry
	SuccessThreshold   int           // Number of successes needed to close circuit
	TimeoutThreshold   time.Duration // Request timeout threshold
	MaxRequestsHalfOpen int           // Max requests in half-open state
}

// Counts holds metrics used by the circuit breaker
type Counts struct {
	Successes            int       // Successful requests
	Failures             int       // Failed requests
	ConsecutiveSuccesses int       // Consecutive successful requests
	ConsecutiveFailures  int       // Consecutive failed requests
	Timeout              int       // Timed out requests
	ShortCircuited       int       // Requests that were short-circuited
	LastSuccess          time.Time // Time of last successful request
	LastFailure          time.Time // Time of last failed request
	LastTimeout          time.Time // Time of last timeout
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name          string
	config        CircuitBreakerConfig
	state         CircuitBreakerState
	counts        Counts
	lastStateChange time.Time
	mutex         sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:          name,
		config:        config,
		state:         CircuitBreakerClosed,
		lastStateChange: time.Now(),
	}
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(defaultConfigs map[string]CircuitBreakerConfig) *CircuitBreakerManager {
	manager := &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}

	// Create circuit breakers from default configs
	for name, config := range defaultConfigs {
		manager.breakers[name] = NewCircuitBreaker(name, config)
	}

	return manager
}

// GetCircuitBreaker gets a circuit breaker by name, creating it if it doesn't exist
func (m *CircuitBreakerManager) GetCircuitBreaker(name string) *CircuitBreaker {
	m.mutex.RLock()
	breaker, exists := m.breakers[name]
	m.mutex.RUnlock()

	if exists {
		return breaker
	}

	// Use a default configuration if the circuit breaker doesn't exist
	defaultConfig := CircuitBreakerConfig{
		FailureThreshold:   5,
		ResetTimeout:       30 * time.Second,
		SuccessThreshold:   2,
		TimeoutThreshold:   5 * time.Second,
		MaxRequestsHalfOpen: 1,
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check again in case it was created while we were waiting for the lock
	breaker, exists = m.breakers[name]
	if exists {
		return breaker
	}

	// Create a new circuit breaker
	breaker = NewCircuitBreaker(name, defaultConfig)
	m.breakers[name] = breaker

	return breaker
}
