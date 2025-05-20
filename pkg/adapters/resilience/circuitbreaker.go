package resilience

import (
	"context"
	"time"
	"sync"

	"github.com/sony/gobreaker"
)

// CircuitBreaker defines the interface for a circuit breaker
type CircuitBreaker interface {
	// Execute executes a function with circuit breaker protection
	Execute(func() (interface{}, error)) (interface{}, error)
	
	// ExecuteContext executes a function with circuit breaker protection and context
	ExecuteContext(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error)
	
	// IsOpen returns true if the circuit breaker is currently open
	IsOpen() bool
	
	// Reset resets the circuit breaker to closed state
	Reset()
	
	// Trip trips the circuit breaker to open state
	Trip()
	
	// Name returns the circuit breaker name
	Name() string
}

// CircuitBreakerConfig defines configuration for a circuit breaker
type CircuitBreakerConfig struct {
	Name             string
	MaxRequests      uint32
	Interval         time.Duration
	Timeout          time.Duration
	ReadyToTrip      func(counts gobreaker.Counts) bool
	OnStateChange    func(name string, from gobreaker.State, to gobreaker.State)
	IsSuccessful     func(err error) bool
}

// DefaultCircuitBreaker is the default implementation of CircuitBreaker
type DefaultCircuitBreaker struct {
	breaker *gobreaker.CircuitBreaker
	config  CircuitBreakerConfig
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) CircuitBreaker {
	// Set default values if not provided
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.MaxRequests == 0 {
		config.MaxRequests = 1
	}
	if config.ReadyToTrip == nil {
		config.ReadyToTrip = func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.5
		}
	}
	
	// Create circuit breaker settings
	settings := gobreaker.Settings{
		Name:          config.Name,
		MaxRequests:   config.MaxRequests,
		Interval:      config.Interval,
		Timeout:       config.Timeout,
		ReadyToTrip:   config.ReadyToTrip,
		OnStateChange: config.OnStateChange,
		IsSuccessful:  config.IsSuccessful,
	}
	
	return &DefaultCircuitBreaker{
		breaker: gobreaker.NewCircuitBreaker(settings),
		config:  config,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *DefaultCircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	return cb.breaker.Execute(fn)
}

// ExecuteContext executes a function with circuit breaker protection and context
func (cb *DefaultCircuitBreaker) ExecuteContext(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	return cb.breaker.Execute(func() (interface{}, error) {
		return fn(ctx)
	})
}

// IsOpen returns true if the circuit breaker is currently open
func (cb *DefaultCircuitBreaker) IsOpen() bool {
	return cb.breaker.State() == gobreaker.StateOpen
}

// Reset resets the circuit breaker to closed state
func (cb *DefaultCircuitBreaker) Reset() {
	// Call the reset method if available
	if resetable, ok := interface{}(cb.breaker).(interface{ Reset() }); ok {
		resetable.Reset()
	}
}

// Trip trips the circuit breaker to open state
func (cb *DefaultCircuitBreaker) Trip() {
	// Call the trip method if available
	if trippable, ok := interface{}(cb.breaker).(interface{ Trip() }); ok {
		trippable.Trip()
	}
}

// Name returns the circuit breaker name
func (cb *DefaultCircuitBreaker) Name() string {
	return cb.config.Name
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(configs map[string]CircuitBreakerConfig) *CircuitBreakerManager {
	manager := &CircuitBreakerManager{
		breakers: make(map[string]CircuitBreaker),
	}
	
	// Create circuit breakers from configs
	for name, config := range configs {
		manager.breakers[name] = NewCircuitBreaker(config)
	}
	
	return manager
}

// Get gets a circuit breaker by name
func (m *CircuitBreakerManager) Get(name string) (CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	breaker, exists := m.breakers[name]
	return breaker, exists
}

// Register registers a new circuit breaker
func (m *CircuitBreakerManager) Register(name string, config CircuitBreakerConfig) CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	breaker := NewCircuitBreaker(config)
	m.breakers[name] = breaker
	return breaker
}

// Execute executes a function with circuit breaker protection
func (m *CircuitBreakerManager) Execute(ctx context.Context, name string, fn func() (interface{}, error)) (interface{}, error) {
	m.mu.RLock()
	breaker, exists := m.breakers[name]
	m.mu.RUnlock()
	
	if !exists {
		// Create a default circuit breaker if it doesn't exist
		config := CircuitBreakerConfig{
			Name: name,
		}
		breaker = m.Register(name, config)
	}
	
	return breaker.Execute(fn)
}
