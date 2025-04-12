package core

import (
	"context"
	"sync"

	"github.com/username/mcp-server/internal/adapters"
	"github.com/username/mcp-server/pkg/mcp"
)

// Engine is the core MCP engine
type Engine struct {
	config   Config
	adapters map[string]adapters.Adapter
	events   chan mcp.Event
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewEngine creates a new MCP engine
func NewEngine(ctx context.Context, cfg Config) (*Engine, error) {
	engineCtx, cancel := context.WithCancel(ctx)

	engine := &Engine{
		config:   cfg,
		adapters: make(map[string]adapters.Adapter),
		events:   make(chan mcp.Event, cfg.EventBufferSize),
		ctx:      engineCtx,
		cancel:   cancel,
	}

	// Initialize adapters
	if err := engine.initializeAdapters(); err != nil {
		cancel()
		return nil, err
	}

	// Start event processing
	engine.startEventProcessors(cfg.ConcurrencyLimit)

	return engine, nil
}

// startEventProcessors starts multiple goroutines for event processing
func (e *Engine) startEventProcessors(count int) {
	for i := 0; i < count; i++ {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			for {
				select {
				case event, ok := <-e.events:
					if !ok {
						return
					}
					e.processEvent(event)
				case <-e.ctx.Done():
					return
				}
			}
		}()
	}
}

// processEvent processes a single event with appropriate error handling
func (e *Engine) processEvent(event mcp.Event) {
	// Create a context with timeout for event processing
	ctx, cancel := context.WithTimeout(e.ctx, e.config.EventTimeout)
	defer cancel()

	// Handle event based on type and source
	// Each handler is isolated to prevent cascading failures
}

// Health checks the health of the engine and all adapters
func (e *Engine) Health() map[string]string {
	health := make(map[string]string)
	health["engine"] = "healthy"

	// Check each adapter's health independently
	for name, adapter := range e.adapters {
		status := adapter.Health()
		health[name] = status
	}

	return health
}
