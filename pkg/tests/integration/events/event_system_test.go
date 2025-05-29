// Package events contains integration tests for the event system
package events

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common/events"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventSystemIntegration(t *testing.T) {
	// Note: The integration package provides test helpers but this test doesn't need them currently

	t.Run("EventBus publishes and subscribers receive events", func(t *testing.T) {
		// Create event bus with appropriate queue size
		eventBus := events.NewEventBus(100)
		require.NotNil(t, eventBus)

		// Set up event capture using a channel to avoid race conditions
		capturedEvents := make(chan *models.Event, 10)

		// Create subscriber that captures events to our channel
		handler := func(ctx context.Context, event *models.Event) error {
			// Store the event for verification
			capturedEvents <- event
			return nil
		}

		// Subscribe to test event
		testEventType := events.EventType("test.event")
		eventBus.Subscribe(testEventType, handler)

		// Create and publish test event
		ctx := context.Background()
		testEvent := &models.Event{
			Type:      string(testEventType),
			Timestamp: time.Now(),
			Source:    "integration-test",
			Data:      map[string]interface{}{"key": "value"},
		}

		// Publish the event
		eventBus.Publish(ctx, testEvent)

		// Wait for the event with timeout
		var receivedEvent *models.Event
		select {
		case receivedEvent = <-capturedEvents:
			// Event received successfully
		case <-time.After(1 * time.Second):
			require.Fail(t, "Timed out waiting for event")
		}

		// Verify event properties
		require.NotNil(t, receivedEvent)
		assert.Equal(t, string(testEventType), receivedEvent.Type)
		assert.Equal(t, "integration-test", receivedEvent.Source)
		assert.Equal(t, "value", receivedEvent.Data.(map[string]interface{})["key"])

		// Close the event bus
		eventBus.Close()
	})

	t.Run("EventBus integrates with observability", func(t *testing.T) {
		// Set up logger with test prefix
		logger := observability.NewLogger("event-test")
		require.NotNil(t, logger)

		// Create event bus with proper queue size
		eventBus := events.NewEventBus(10)
		require.NotNil(t, eventBus)

		// Use channels to safely capture events and avoid race conditions
		eventChan := make(chan *models.Event, 10)

		// Create event handler that logs with observability
		handler := func(ctx context.Context, event *models.Event) error {
			// Log event using observability package
			logger.Info("Received event", map[string]interface{}{
				"type":   event.Type,
				"source": event.Source,
			})

			// Send to channel for test verification
			eventChan <- event
			return nil
		}

		// Subscribe to system events
		eventBus.Subscribe(events.EventSystemStartup, handler)
		eventBus.Subscribe(events.EventSystemShutdown, handler)

		// Create test context
		ctx := context.Background()

		// Publish system events
		startupEvent := &models.Event{
			Type:      string(events.EventSystemStartup),
			Timestamp: time.Now(),
			Source:    "system",
		}
		eventBus.Publish(ctx, startupEvent)

		shutdownEvent := &models.Event{
			Type:      string(events.EventSystemShutdown),
			Timestamp: time.Now(),
			Source:    "system",
		}
		eventBus.Publish(ctx, shutdownEvent)

		// Use a map to track received event types
		receivedEvents := make(map[string]bool)

		// Wait for events with timeout
		for i := 0; i < 2; i++ {
			select {
			case event := <-eventChan:
				// Mark this event type as received
				receivedEvents[event.Type] = true
			case <-time.After(1 * time.Second):
				require.Fail(t, "Timed out waiting for events")
			}
		}

		// Verify both event types were received
		assert.True(t, receivedEvents[string(events.EventSystemStartup)])
		assert.True(t, receivedEvents[string(events.EventSystemShutdown)])

		// Close the event bus
		eventBus.Close()
	})
}
