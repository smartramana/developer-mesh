package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/tests/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventSystemIntegration(t *testing.T) {
	helper := integration.NewTestHelper(t)
	
	t.Run("EventBus publishes and subscribers receive events", func(t *testing.T) {
		// Create event bus
		eventBus := events.NewEventBus(100)
		require.NotNil(t, eventBus)
		
		// Set up event capture
		var capturedEvents []*mcp.Event
		var mu sync.Mutex
		
		// Create subscriber
		handler := func(ctx context.Context, event *mcp.Event) error {
			mu.Lock()
			defer mu.Unlock()
			capturedEvents = append(capturedEvents, event)
			return nil
		}
		
		// Subscribe to test event
		eventBus.Subscribe(events.EventType("test.event"), handler)
		
		// Create and publish test event
		ctx := context.Background()
		testEvent := &mcp.Event{
			Type:      "test.event",
			Timestamp: time.Now(),
			Source:    "integration-test",
			Data:      map[string]interface{}{"key": "value"},
		}
		
		// Publish the event
		eventBus.Publish(ctx, testEvent)
		
		// Allow time for async processing
		time.Sleep(100 * time.Millisecond)
		
		// Verify event was received
		mu.Lock()
		defer mu.Unlock()
		require.Len(t, capturedEvents, 1)
		assert.Equal(t, "test.event", capturedEvents[0].Type)
		assert.Equal(t, "integration-test", capturedEvents[0].Source)
		
		// Close the event bus
		eventBus.Close()
	})
	
	t.Run("Cross-package integration with observability", func(t *testing.T) {
		// Create logger
		logger := observability.NewLogger()
		
		// Create event bus with proper queue size
		eventBus := events.NewEventBus(10)
		require.NotNil(t, eventBus)
		
		// Create event receiver with observability
		receivedEvents := make(map[string]bool)
		var mu sync.Mutex
		
		handler := func(ctx context.Context, event *mcp.Event) error {
			// Log event using observability package
			logger.Info("Received event", map[string]interface{}{
				"type":   event.Type,
				"source": event.Source,
			})
			
			mu.Lock()
			receivedEvents[event.Type] = true
			mu.Unlock()
			
			return nil
		}
		
		// Subscribe to system events
		eventBus.Subscribe(events.EventSystemStartup, handler)
		eventBus.Subscribe(events.EventSystemShutdown, handler)
		
		// Publish system events
		ctx := context.Background()
		eventBus.Publish(ctx, &mcp.Event{
			Type:      string(events.EventSystemStartup),
			Timestamp: time.Now(),
			Source:    "system",
		})
		
		eventBus.Publish(ctx, &mcp.Event{
			Type:      string(events.EventSystemShutdown),
			Timestamp: time.Now(),
			Source:    "system",
		})
		
		// Allow time for async processing
		time.Sleep(100 * time.Millisecond)
		
		// Verify events were received
		mu.Lock()
		assert.True(t, receivedEvents[string(events.EventSystemStartup)])
		assert.True(t, receivedEvents[string(events.EventSystemShutdown)])
		mu.Unlock()
		
		// Close the event bus
		eventBus.Close()
	})
}
