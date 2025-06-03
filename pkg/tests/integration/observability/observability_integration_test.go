package observability

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservabilityStackIntegration(t *testing.T) {
	// Note: The integration package provides test helpers but this test doesn't need them currently

	t.Run("Logger and MetricsClient integration", func(t *testing.T) {
		// Create observability components
		logger := observability.NewLogger("integration-test")
		require.NotNil(t, logger)

		metricsClient := observability.NewMetricsClient()
		require.NotNil(t, metricsClient)

		// Test that logger can be used with metrics
		// This tests the interface compatibility across packages
		logger = logger.With(map[string]interface{}{
			"metrics_enabled": true,
		})

		// Log with metrics reference
		logger.Info("Starting integration test", map[string]interface{}{
			"component": "observability",
			"test":      "integration",
		})

		// Record a metric
		metricsClient.IncrementCounter("test.counter", 1.0)

		// Record a metric with labels using the new method signature
		metricsClient.IncrementCounterWithLabels("test.counter.with.labels", 1.0, map[string]string{
			"test": "integration",
		})

		// Log the recorded metric
		logger.Info("Recorded metric", map[string]interface{}{
			"metric_name":  "test.counter",
			"metric_value": 1.0,
		})
	})

	t.Run("Span creation and management", func(t *testing.T) {
		// Create tracer
		tracer := observability.GetTracer()
		require.NotNil(t, tracer)

		// Create root span
		ctx := context.Background()
		ctx, rootSpan := tracer.Start(ctx, "root-operation")
		require.NotNil(t, rootSpan)

		// Create child span
		ctx, childSpan := tracer.Start(ctx, "child-operation")
		require.NotNil(t, childSpan)

		// Skip adding attributes to span for now as the interface methods have changed
		// The span interface implementation needs further investigation

		// End spans in correct order
		childSpan.End()
		rootSpan.End()
	})

	t.Run("Cross-package integration with events", func(t *testing.T) {
		// Create observability components
		logger := observability.NewLogger("event-test")
		require.NotNil(t, logger)

		// Create event bus
		eventBus := events.NewEventBus(10)
		require.NotNil(t, eventBus)

		// Create event receiver with logging
		receivedEvents := make(map[string]bool)
		var mu sync.Mutex

		handler := func(ctx context.Context, event *models.Event) error {
			// Log event using observability package
			logger.Info("Handling event", map[string]interface{}{
				"event_type":   event.Type,
				"event_source": event.Source,
			})

			mu.Lock()
			receivedEvents[event.Type] = true
			mu.Unlock()
			return nil
		}

		// Subscribe to test event
		eventBus.Subscribe(events.EventType("metrics.collect"), handler)

		// Publish test event
		ctx := context.Background()
		eventBus.Publish(ctx, &models.Event{
			Type:      "metrics.collect",
			Timestamp: time.Now(),
			Source:    "observability-test",
			Data:      map[string]interface{}{"metric": "test.counter", "value": 1.0},
		})

		// Allow time for async processing
		time.Sleep(100 * time.Millisecond)

		// Verify event was received and logged
		mu.Lock()
		assert.True(t, receivedEvents["metrics.collect"])
		mu.Unlock()

		// Close the event bus
		eventBus.Close()
	})
}
