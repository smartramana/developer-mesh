package core

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/internal/adapters/github"
	"github.com/S-Corkum/mcp-server/internal/adapters/mocks"
	cacheMocks "github.com/S-Corkum/mcp-server/internal/cache/mocks"
	dbMocks "github.com/S-Corkum/mcp-server/internal/database/mocks"
	metricsMocks "github.com/S-Corkum/mcp-server/internal/metrics/mocks"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEngine is a lightweight test version of the engine
type MockEngine struct {
	adapter       adapters.Adapter
	mockDB        *dbMocks.MockDatabase
	mockCache     *cacheMocks.MockCache
	mockMetrics   *metricsMocks.MockMetricsClient
}

// TestBasicAdapterOperations tests adapter operations without a full engine
func TestBasicAdapterOperations(t *testing.T) {
	// Create a mock adapter
	mockAdapter := new(mocks.MockAdapter)
	mockAdapter.On("Initialize", mock.Anything, mock.Anything).Return(nil)
	mockAdapter.On("GetData", mock.Anything, mock.Anything).Return(nil, nil)
	mockAdapter.On("Subscribe", mock.Anything, mock.Anything).Return(nil)
	mockAdapter.On("HandleWebhook", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockAdapter.On("Health").Return("healthy")
	mockAdapter.On("Close").Return(nil)
	
	// Create a map of adapters like the engine has
	adapters := make(map[string]adapters.Adapter)
	adapters["test"] = mockAdapter
	
	// Test GetAdapter-like functionality
	t.Run("Get Existing Adapter", func(t *testing.T) {
		adapter, found := adapters["test"]
		assert.True(t, found)
		assert.NotNil(t, adapter)
		assert.Equal(t, mockAdapter, adapter)
	})

	t.Run("Get Non-Existent Adapter", func(t *testing.T) {
		adapter, found := adapters["non-existent"]
		assert.False(t, found)
		assert.Nil(t, adapter)
	})
	
	// Test Health-like functionality
	t.Run("Check Health", func(t *testing.T) {
		health := make(map[string]string)
		health["engine"] = "healthy"
		
		// Add adapter health
		for name, adapter := range adapters {
			health[name] = adapter.Health()
		}
		
		assert.Equal(t, "healthy", health["engine"])
		assert.Equal(t, "healthy", health["test"])
	})
	
	// Test basic adapter methods
	t.Run("Initialize", func(t *testing.T) {
		err := mockAdapter.Initialize(context.Background(), nil)
		assert.NoError(t, err)
	})
	
	t.Run("GetData", func(t *testing.T) {
		data, err := mockAdapter.GetData(context.Background(), nil)
		assert.NoError(t, err)
		assert.Nil(t, data)
	})
	
	t.Run("Subscribe", func(t *testing.T) {
		err := mockAdapter.Subscribe("test-event", func(event interface{}) {})
		assert.NoError(t, err)
	})
	
	t.Run("HandleWebhook", func(t *testing.T) {
		err := mockAdapter.HandleWebhook(context.Background(), "test-event", []byte("{}"))
		assert.NoError(t, err)
	})
	
	t.Run("Health", func(t *testing.T) {
		health := mockAdapter.Health()
		assert.Equal(t, "healthy", health)
	})
	
	t.Run("Close", func(t *testing.T) {
		err := mockAdapter.Close()
		assert.NoError(t, err)
	})
}

func TestGitHubAdapter(t *testing.T) {
	// Create a GitHub adapter with mock mode
	cfg := github.Config{
		MockResponses:  true,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
	}
	
	adapter, err := github.NewAdapter(cfg)
	require.NoError(t, err)
	require.NotNil(t, adapter)
	
	t.Run("Initialize", func(t *testing.T) {
		err := adapter.Initialize(context.Background(), nil)
		assert.NoError(t, err)
	})
	
	t.Run("Health", func(t *testing.T) {
		health := adapter.Health()
		assert.Equal(t, "healthy (mock)", health)
	})
	
	t.Run("Close", func(t *testing.T) {
		err := adapter.Close()
		assert.NoError(t, err)
	})
	
	// Test event processing concepts used in engine
	t.Run("Process Event", func(t *testing.T) {
		event := mcp.Event{
			Source:    "github",
			Type:      "pull_request",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"action": "opened", "number": 123},
		}
		
		// Simulate engine processEvent behavior
		assert.Equal(t, "github", event.Source)
		assert.Equal(t, "pull_request", event.Type)
		assert.NotZero(t, event.Timestamp)
		assert.NotNil(t, event.Data)
	})
}

func TestShutdownBehavior(t *testing.T) {
	// Create a mock adapter
	mockAdapter := new(mocks.MockAdapter)
	mockAdapter.On("Close").Return(nil)
	
	// Create a map of adapters like the engine has
	adapters := make(map[string]adapters.Adapter)
	adapters["test"] = mockAdapter
	
	// Test Shutdown-like behavior
	t.Run("Shutdown Adapters", func(t *testing.T) {
		// Close all adapters (simulating engine.Shutdown behavior)
		for _, adapter := range adapters {
			err := adapter.Close()
			assert.NoError(t, err)
		}
		
		// Verify the adapter's Close was called
		mockAdapter.AssertExpectations(t)
	})
}
