package core

import (
	"context"
	"sync"
	"testing"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	adaptermocks "github.com/S-Corkum/mcp-server/internal/adapters/mocks"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
)

// A minimal test for the Engine.Health method
func TestEngineHealth(t *testing.T) {
	// Create mock adapter
	mockAdapter := &adaptermocks.MockAdapter{}
	mockAdapter.On("Health").Return("healthy")
	
	// Create a simplified engine for testing
	adaptersMap := make(map[string]adapters.Adapter)
	adaptersMap["test"] = mockAdapter
	
	engine := &Engine{
		adapters: adaptersMap,
	}
	
	// Test health
	health := engine.Health()
	
	// Verify
	assert.NotNil(t, health)
	assert.Equal(t, "healthy", health["engine"])
	assert.Equal(t, "healthy", health["test"])
}

// A minimal test for the Engine.Shutdown method
func TestEngineShutdown(t *testing.T) {
	// Create mock adapter
	mockAdapter := &adaptermocks.MockAdapter{}
	mockAdapter.On("Close").Return(nil)
	
	// Create context for shutdown
	ctx := context.Background()
	engineCtx, cancel := context.WithCancel(ctx)
	
	// Create a simplified engine for testing
	adaptersMap := make(map[string]adapters.Adapter)
	adaptersMap["test"] = mockAdapter
	
	engine := &Engine{
		adapters: adaptersMap,
		ctx:      engineCtx,
		cancel:   cancel,
		events:   make(chan mcp.Event, 10),
		wg:       sync.WaitGroup{},
	}
	
	// Add a worker
	engine.wg.Add(1)
	go func() {
		defer engine.wg.Done()
		<-engine.ctx.Done()
	}()
	
	// Test shutdown
	err := engine.Shutdown(ctx)
	
	// Verify
	assert.NoError(t, err)
	mockAdapter.AssertExpectations(t)
}

// A minimal test for the Engine.GetAdapter method
func TestGetAdapter(t *testing.T) {
	// Create mock adapter
	mockAdapter := &adaptermocks.MockAdapter{}
	
	// Create a simplified engine for testing
	adaptersMap := make(map[string]adapters.Adapter)
	adaptersMap["test"] = mockAdapter
	
	engine := &Engine{
		adapters: adaptersMap,
	}
	
	// Test getting an existing adapter
	adapter, err := engine.GetAdapter("test")
	assert.NoError(t, err)
	assert.Equal(t, mockAdapter, adapter)
	
	// Test getting a non-existent adapter
	adapter, err = engine.GetAdapter("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, adapter)
}
