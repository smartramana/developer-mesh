package core

import (
	"fmt"
	"sync"
)

// TestMockEngine is a mock implementation of the Engine for testing
type TestMockEngine struct {
	adapters       map[string]interface{}
	mu             sync.RWMutex
	ContextManager interface{} // Added to satisfy tests
}

// NewTestMockEngine creates a new mock engine for testing
func NewTestMockEngine() *TestMockEngine {
	return &TestMockEngine{
		adapters: make(map[string]interface{}),
	}
}

// RegisterAdapter registers an adapter with the mock engine
func (m *TestMockEngine) RegisterAdapter(name string, adapter interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adapters[name] = adapter
}

// GetAdapter returns an adapter by name
func (m *TestMockEngine) GetAdapter(name string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	adapter, ok := m.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter not found: %s", name)
	}

	return adapter, nil
}

// Health returns a map of component health statuses
func (m *TestMockEngine) Health() map[string]string {
	return map[string]string{
		"mock_engine": "healthy",
	}
}

// ProcessEvent processes an event
func (m *TestMockEngine) ProcessEvent(event interface{}) error {
	return nil
}
