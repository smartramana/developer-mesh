package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BulkheadConfig defines configuration for a bulkhead
type BulkheadConfig struct {
	Name           string
	MaxConcurrent  int
	MaxWaitingTime time.Duration
}

// Bulkhead defines the interface for a bulkhead
type Bulkhead interface {
	// Execute executes a function with bulkhead protection
	Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error)
	
	// Name returns the bulkhead name
	Name() string
	
	// CurrentExecutions returns the number of current executions
	CurrentExecutions() int
	
	// RemainingExecutions returns the number of remaining executions
	RemainingExecutions() int
}

// DefaultBulkhead is the default implementation of Bulkhead
type DefaultBulkhead struct {
	config          BulkheadConfig
	semaphore       chan struct{}
	currentCount    int
	mu              sync.Mutex
}

// NewBulkhead creates a new bulkhead
func NewBulkhead(config BulkheadConfig) Bulkhead {
	// Set default values if not provided
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}
	
	return &DefaultBulkhead{
		config:       config,
		semaphore:    make(chan struct{}, config.MaxConcurrent),
		currentCount: 0,
	}
}

// Execute executes a function with bulkhead protection
func (b *DefaultBulkhead) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	// Create a context with timeout if MaxWaitingTime is set
	var ctxToUse context.Context
	var cancel context.CancelFunc
	
	if b.config.MaxWaitingTime > 0 {
		ctxToUse, cancel = context.WithTimeout(ctx, b.config.MaxWaitingTime)
		defer cancel()
	} else {
		ctxToUse = ctx
	}
	
	// Try to acquire semaphore
	select {
	case b.semaphore <- struct{}{}:
		// Acquired semaphore
		b.incrementCount()
		defer func() {
			<-b.semaphore // Release semaphore
			b.decrementCount()
		}()
		
		// Execute the function
		return fn()
	case <-ctxToUse.Done():
		// Timeout or cancellation
		return nil, fmt.Errorf("bulkhead '%s' rejected execution: %w", b.config.Name, ctxToUse.Err())
	}
}

// Name returns the bulkhead name
func (b *DefaultBulkhead) Name() string {
	return b.config.Name
}

// CurrentExecutions returns the number of current executions
func (b *DefaultBulkhead) CurrentExecutions() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentCount
}

// RemainingExecutions returns the number of remaining executions
func (b *DefaultBulkhead) RemainingExecutions() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.config.MaxConcurrent - b.currentCount
}

// incrementCount increments the current count
func (b *DefaultBulkhead) incrementCount() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.currentCount++
}

// decrementCount decrements the current count
func (b *DefaultBulkhead) decrementCount() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.currentCount--
}

// BulkheadManager manages multiple bulkheads
type BulkheadManager struct {
	bulkheads map[string]Bulkhead
	mu        sync.RWMutex
}

// NewBulkheadManager creates a new bulkhead manager
func NewBulkheadManager(configs map[string]BulkheadConfig) *BulkheadManager {
	manager := &BulkheadManager{
		bulkheads: make(map[string]Bulkhead),
	}
	
	// Create bulkheads from configs
	for name, config := range configs {
		manager.bulkheads[name] = NewBulkhead(config)
	}
	
	return manager
}

// Get gets a bulkhead by name
func (m *BulkheadManager) Get(name string) (Bulkhead, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	bulkhead, exists := m.bulkheads[name]
	return bulkhead, exists
}

// Register registers a new bulkhead
func (m *BulkheadManager) Register(name string, config BulkheadConfig) Bulkhead {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	bulkhead := NewBulkhead(config)
	m.bulkheads[name] = bulkhead
	return bulkhead
}

// Execute executes a function with bulkhead protection
func (m *BulkheadManager) Execute(ctx context.Context, name string, fn func() (interface{}, error)) (interface{}, error) {
	m.mu.RLock()
	bulkhead, exists := m.bulkheads[name]
	m.mu.RUnlock()
	
	if !exists {
		// Create a default bulkhead if it doesn't exist
		config := BulkheadConfig{
			Name:          name,
			MaxConcurrent: 10,
		}
		bulkhead = m.Register(name, config)
	}
	
	return bulkhead.Execute(ctx, fn)
}
