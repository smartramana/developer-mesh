package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"mcp-server/internal/config"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many concurrent requests")
)

// CircuitBreaker provides fault tolerance for database operations
type CircuitBreaker struct {
	config *config.CircuitBreakerConfig

	state        State
	failures     int
	successes    int
	lastFailTime time.Time

	concurrentRequests int

	mu      sync.RWMutex
	logger  observability.Logger
	metrics observability.MetricsClient
}

// NewCircuitBreaker creates a circuit breaker for database operations
func NewCircuitBreaker(
	cfg *config.CircuitBreakerConfig,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *CircuitBreaker {
	return &CircuitBreaker{
		config:  cfg,
		state:   StateClosed,
		logger:  logger,
		metrics: metrics,
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, operation string, fn func() error) error {
	if !cb.config.Enabled {
		return fn()
	}

	// Check if we can proceed
	if err := cb.canExecute(); err != nil {
		cb.metrics.IncrementCounterWithLabels("circuit_breaker_rejected", 1, map[string]string{"operation": operation})
		return err
	}

	// Track concurrent requests
	cb.incrementConcurrent()
	defer cb.decrementConcurrent()

	// Execute the function
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	// Update circuit breaker state based on result
	cb.recordResult(err)

	// Record metrics
	cb.metrics.RecordOperation("circuit_breaker", operation, err == nil, duration.Seconds(), map[string]string{
		"state": cb.getStateName(),
	})

	return err
}

// ExecuteWithFallback runs a function with circuit breaker and fallback
func (cb *CircuitBreaker) ExecuteWithFallback(
	ctx context.Context,
	operation string,
	fn func() error,
	fallback func() error,
) error {
	err := cb.Execute(ctx, operation, fn)
	if err == ErrCircuitOpen || err == ErrTooManyRequests {
		cb.logger.Warn("Circuit breaker triggered, using fallback", map[string]interface{}{
			"operation": operation,
			"error":     err,
		})
		return fallback()
	}
	return err
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.logger.Info("Circuit breaker manually reset", map[string]interface{}{})
}

// Internal methods

func (cb *CircuitBreaker) canExecute() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check concurrent request limit
	if cb.concurrentRequests >= cb.config.MaxConcurrentRequests {
		return ErrTooManyRequests
	}

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailTime) > cb.config.Timeout {
			cb.logger.Info("Circuit breaker timeout expired, moving to half-open", map[string]interface{}{})
			cb.state = StateHalfOpen
			cb.successes = 0
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		return nil

	default:
		return ErrCircuitOpen
	}
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		if err != nil {
			cb.failures++
			if cb.failures >= cb.config.FailureThreshold {
				cb.logger.Error("Circuit breaker opening due to failures", map[string]interface{}{
					"failures":  cb.failures,
					"threshold": cb.config.FailureThreshold,
				})
				cb.state = StateOpen
				cb.lastFailTime = time.Now()
			}
		} else {
			cb.failures = 0
		}

	case StateHalfOpen:
		if err != nil {
			cb.logger.Warn("Circuit breaker reopening from half-open state", map[string]interface{}{})
			cb.state = StateOpen
			cb.lastFailTime = time.Now()
		} else {
			cb.successes++
			if cb.successes >= cb.config.SuccessThreshold {
				cb.logger.Info("Circuit breaker closing from half-open state", map[string]interface{}{
					"successes": cb.successes,
				})
				cb.state = StateClosed
				cb.failures = 0
			}
		}
	}
}

func (cb *CircuitBreaker) incrementConcurrent() {
	cb.mu.Lock()
	cb.concurrentRequests++
	cb.mu.Unlock()
}

func (cb *CircuitBreaker) decrementConcurrent() {
	cb.mu.Lock()
	cb.concurrentRequests--
	cb.mu.Unlock()
}

func (cb *CircuitBreaker) getStateName() string {
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreakerGroup manages multiple circuit breakers for different operations
type CircuitBreakerGroup struct {
	breakers map[string]*CircuitBreaker
	config   *config.CircuitBreakerConfig
	logger   observability.Logger
	metrics  observability.MetricsClient
	mu       sync.RWMutex
}

// NewCircuitBreakerGroup creates a group of circuit breakers
func NewCircuitBreakerGroup(
	cfg *config.CircuitBreakerConfig,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *CircuitBreakerGroup {
	return &CircuitBreakerGroup{
		breakers: make(map[string]*CircuitBreaker),
		config:   cfg,
		logger:   logger,
		metrics:  metrics,
	}
}

// GetBreaker returns a circuit breaker for a specific operation type
func (cbg *CircuitBreakerGroup) GetBreaker(operation string) *CircuitBreaker {
	cbg.mu.RLock()
	breaker, exists := cbg.breakers[operation]
	cbg.mu.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker if it doesn't exist
	cbg.mu.Lock()
	defer cbg.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists = cbg.breakers[operation]; exists {
		return breaker
	}

	breaker = NewCircuitBreaker(cbg.config, cbg.logger, cbg.metrics)
	cbg.breakers[operation] = breaker
	return breaker
}

// ResetAll resets all circuit breakers in the group
func (cbg *CircuitBreakerGroup) ResetAll() {
	cbg.mu.RLock()
	defer cbg.mu.RUnlock()

	for _, breaker := range cbg.breakers {
		breaker.Reset()
	}
}
