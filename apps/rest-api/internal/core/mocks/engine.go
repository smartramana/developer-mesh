// Package mocks provides mock implementations for testing
package mocks

import (
	"rest-api/internal/core"
	"rest-api/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockEngine is a mock implementation that mimics the core.Engine type
type MockEngine struct {
	mock.Mock
	ContextManager     core.ContextManagerInterface
	AgentRepo          repository.AgentRepository
	ModelRepo          repository.ModelRepository
	VectorRepo         repository.VectorAPIRepository
	SearchRepo         repository.SearchRepository
	adapters           map[string]interface{}
}

// NewMockEngine creates a new mock engine for testing
func NewMockEngine() *MockEngine {
	return &MockEngine{
		ContextManager: NewMockContextManager(),
		adapters:       make(map[string]interface{}),
	}
}

// RegisterAdapter registers an adapter with the engine
func (e *MockEngine) RegisterAdapter(name string, adapter interface{}) {
	e.adapters[name] = adapter
}

// GetAdapter retrieves a registered adapter by name
func (e *MockEngine) GetAdapter(name string) (interface{}, error) {
	adapter, ok := e.adapters[name]
	if !ok {
		return nil, nil // Return nil, nil for testing simplicity
	}
	return adapter, nil
}

// SetContextManager sets the context manager
func (e *MockEngine) SetContextManager(manager core.ContextManagerInterface) {
	e.ContextManager = manager
}

// GetContextManager returns the context manager
func (e *MockEngine) GetContextManager() core.ContextManagerInterface {
	return e.ContextManager
}

// GetAgentRepository returns the agent repository
func (e *MockEngine) GetAgentRepository() repository.AgentRepository {
	return e.AgentRepo
}

// GetModelRepository returns the model repository
func (e *MockEngine) GetModelRepository() repository.ModelRepository {
	return e.ModelRepo
}

// GetVectorRepository returns the vector repository
func (e *MockEngine) GetVectorRepository() repository.VectorAPIRepository {
	return e.VectorRepo
}

// GetSearchRepository returns the search repository
func (e *MockEngine) GetSearchRepository() repository.SearchRepository {
	return e.SearchRepo
}

// Health returns the health status of the engine and its components
func (e *MockEngine) Health() map[string]string {
	args := e.Called()
	if args.Get(0) == nil {
		return map[string]string{
			"status": "healthy",
		}
	}
	return args.Get(0).(map[string]string)
}
