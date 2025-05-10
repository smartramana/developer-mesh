package worker

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/observability"
	"github.com/S-Corkum/devops-mcp/internal/queue"
)

var (
	logger             = observability.NewLogger("worker.processor")
	successCount int64  = 0
	failureCount int64  = 0
	totalDuration int64 = 0 // nanoseconds
)

// ProcessSQSEvent contains the actual business logic for handling webhook events.
// Returns error if processing fails (to trigger SQS retry).
func ProcessSQSEvent(event queue.SQSEvent) error {
	start := time.Now()
	logger.Info("Processing SQS event", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"event_type":  event.EventType,
		"repo":        event.RepoName,
		"sender":      event.SenderName,
	})

	// Unmarshal payload for further processing
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		atomic.AddInt64(&failureCount, 1)
		logger.Error("Failed to unmarshal payload", map[string]interface{}{
			"delivery_id": event.DeliveryID,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	logger.Debug("Payload keys", map[string]interface{}{"keys": keys(payload)})

	// Example: Add real business logic here
	if event.EventType == "push" {
		atomic.AddInt64(&failureCount, 1)
		logger.Warn("Push event processing failed (simulated)", map[string]interface{}{
			"delivery_id": event.DeliveryID,
		})
		return fmt.Errorf("simulated failure for push event")
	}

	// Simulate processing time
	time.Sleep(200 * time.Millisecond)

	atomic.AddInt64(&successCount, 1)
	dur := time.Since(start).Nanoseconds()
	atomic.AddInt64(&totalDuration, dur)
	logger.Info("Successfully processed event", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"duration_ms": float64(dur) / 1e6,
		"successes":   atomic.LoadInt64(&successCount),
		"failures":    atomic.LoadInt64(&failureCount),
	})
	return nil
}

func keys(m map[string]interface{}) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	return k
}
