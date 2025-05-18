// Package core provides the central engine for coordinating API subsystems
package core

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Engine is the central component that coordinates between different subsystems
type Engine struct {
	adapters       map[string]interface{}
	contextManager ContextManagerInterface
	logger         observability.Logger
	mutex          sync.RWMutex
}

// ContextManagerInterface defines the interface for context management
type ContextManagerInterface interface {
	CreateContext(ctx context.Context, context *mcp.Context) (*mcp.Context, error)
	GetContext(ctx context.Context, contextID string) (*mcp.Context, error)
	UpdateContext(ctx context.Context, contextID string, context *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error)
	DeleteContext(ctx context.Context, contextID string) error
	ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error)
	SearchInContext(ctx context.Context, contextID, query string) ([]mcp.ContextItem, error)
	SummarizeContext(ctx context.Context, contextID string) (string, error)
}

// NewEngine creates a new engine instance
func NewEngine(logger observability.Logger) *Engine {
	if logger == nil {
		logger = observability.NewLogger("core-engine")
	}
	
	return &Engine{
		adapters: make(map[string]interface{}),
		logger:   logger,
		mutex:    sync.RWMutex{},
	}
}

// RegisterAdapter registers an adapter with the engine
func (e *Engine) RegisterAdapter(name string, adapter interface{}) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	e.adapters[name] = adapter
	e.logger.Info(fmt.Sprintf("Registered adapter: %s", name), nil)
}

// GetAdapter retrieves a registered adapter by name
func (e *Engine) GetAdapter(name string) (interface{}, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	adapter, ok := e.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter not found: %s", name)
	}
	
	return adapter, nil
}

// SetContextManager sets the context manager
func (e *Engine) SetContextManager(manager ContextManagerInterface) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	e.contextManager = manager
	e.logger.Info("Set context manager", nil)
}

// GetContextManager returns the current context manager
func (e *Engine) GetContextManager() ContextManagerInterface {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	if e.contextManager == nil {
		e.logger.Warn("Context manager not initialized, returning mock implementation", nil)
		return NewMockContextManager()
	}
	
	return e.contextManager
}

// Health returns the health status of all components
func (e *Engine) Health() map[string]string {
	health := make(map[string]string)
	
	// Add engine health
	health["core_engine"] = "healthy"
	
	// Add context manager health
	if e.contextManager != nil {
		health["context_manager"] = "healthy"
	} else {
		health["context_manager"] = "not_initialized"
	}
	
	// In a real implementation, this would check the health of each adapter
	// For now, we'll just report that they're healthy
	e.mutex.RLock()
	for name := range e.adapters {
		health[fmt.Sprintf("adapter_%s", name)] = "healthy"
	}
	e.mutex.RUnlock()
	
	return health
}

// Shutdown gracefully shuts down the engine and its components
func (e *Engine) Shutdown(ctx context.Context) error {
	e.logger.Info("Shutting down engine", nil)
	
	// Clean up adapters if they implement a Close or Shutdown method
	e.mutex.RLock()
	for name, adapter := range e.adapters {
		// Try with Close method
		if closer, ok := adapter.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				e.logger.Warn(fmt.Sprintf("Error closing adapter %s: %v", name, err), nil)
			}
			continue
		}
		
		// Try with Shutdown method that takes context
		if shutdown, ok := adapter.(interface{ Shutdown(context.Context) error }); ok {
			if err := shutdown.Shutdown(ctx); err != nil {
				e.logger.Warn(fmt.Sprintf("Error shutting down adapter %s: %v", name, err), nil)
			}
		}
	}
	e.mutex.RUnlock()
	
	return nil
}
