package resilience

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerConfig holds configuration for circuit breakers
type CircuitBreakerConfig struct {
	Name         string        `mapstructure:"name"`
	MaxRequests  uint32        `mapstructure:"max_requests"`
	Interval     time.Duration `mapstructure:"interval"`
	Timeout      time.Duration `mapstructure:"timeout"`
	FailureRatio float64       `mapstructure:"failure_ratio"`
}

var (
	circuitBreakers     = make(map[string]*gobreaker.CircuitBreaker)
	circuitBreakerMutex sync.RWMutex
)

// GetCircuitBreaker returns a circuit breaker with the given name, creating it if it doesn't exist
func GetCircuitBreaker(name string, config CircuitBreakerConfig) *gobreaker.CircuitBreaker {
	circuitBreakerMutex.RLock()
	cb, ok := circuitBreakers[name]
	circuitBreakerMutex.RUnlock()

	if ok {
		return cb
	}

	// Not found, create a new one
	circuitBreakerMutex.Lock()
	defer circuitBreakerMutex.Unlock()

	// Check again in case it was created while we were waiting for the lock
	if cb, ok := circuitBreakers[name]; ok {
		return cb
	}

	// Apply defaults if needed
	if config.Name == "" {
		config.Name = name
	}
	if config.MaxRequests == 0 {
		config.MaxRequests = 5
	}
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.FailureRatio == 0 {
		config.FailureRatio = 0.5
	}

	settings := gobreaker.Settings{
		Name:        config.Name,
		MaxRequests: config.MaxRequests,
		Interval:    config.Interval,
		Timeout:     config.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= config.FailureRatio
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("Circuit breaker %s state change: %s -> %s", name, from, to)
		},
	}

	cb = gobreaker.NewCircuitBreaker(settings)
	circuitBreakers[name] = cb
	return cb
}

// ExecuteWithCircuitBreaker executes a function with a circuit breaker
func ExecuteWithCircuitBreaker(ctx context.Context, cbName string, config CircuitBreakerConfig, fn func() (interface{}, error)) (interface{}, error) {
	cb := GetCircuitBreaker(cbName, config)

	// Create a channel to collect the result
	resultCh := make(chan struct {
		result interface{}
		err    error
	}, 1)

	// Execute the function with the circuit breaker in a goroutine
	go func() {
		result, err := cb.Execute(fn)
		resultCh <- struct {
			result interface{}
			err    error
		}{result, err}
	}()

	// Wait for the result or context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		return res.result, res.err
	}
}

// CircuitBreakerManager manages a set of circuit breakers
type CircuitBreakerManager struct {
	configs map[string]CircuitBreakerConfig
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(configs map[string]CircuitBreakerConfig) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		configs: configs,
	}
}

// Execute executes a function with a circuit breaker
func (m *CircuitBreakerManager) Execute(ctx context.Context, name string, fn func() (interface{}, error)) (interface{}, error) {
	config, ok := m.configs[name]
	if !ok {
		// Use default config if no config is found
		config = CircuitBreakerConfig{
			Name:         name,
			MaxRequests:  5,
			Interval:     30 * time.Second,
			Timeout:      60 * time.Second,
			FailureRatio: 0.5,
		}
	}

	return ExecuteWithCircuitBreaker(ctx, name, config, fn)
}

// Common circuit breaker names
const (
	GitHubCircuitBreaker   = "github"
	S3CircuitBreaker       = "s3"
	DatabaseCircuitBreaker = "database"
	RedisCircuitBreaker    = "redis"
	VectorCircuitBreaker   = "vector"
)

// ShutdownCircuitBreakers closes all circuit breakers
func ShutdownCircuitBreakers() {
	circuitBreakerMutex.Lock()
	defer circuitBreakerMutex.Unlock()

	circuitBreakers = make(map[string]*gobreaker.CircuitBreaker)
	log.Println("Circuit breakers shut down")
}
