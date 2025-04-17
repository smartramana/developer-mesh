package core

import (
	"fmt"
	"sync"
)

// MockEngine is a mock implementation of the Engine for testing
type MockEngine struct {
	adapters map[string]interface{}
	mu       sync.RWMutex
}

// NewMockEngine creates a new mock engine for testing
func NewMockEngine() *MockEngine {
	return &MockEngine{
		adapters: make(map[string]interface{}),
	}
}

// RegisterAdapter registers an adapter with the mock engine
func (m *MockEngine) RegisterAdapter(name string, adapter interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adapters[name] = adapter
}

// GetAdapter returns an adapter by name
func (m *MockEngine) GetAdapter(name string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	adapter, ok := m.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter not found: %s", name)
	}
	
	return adapter, nil
}

// Health returns a map of component health statuses
func (m *MockEngine) Health() map[string]string {
	return map[string]string{
		"mock_engine": "healthy",
	}
}
