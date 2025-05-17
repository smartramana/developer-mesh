package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/S-Corkum/devops-mcp/apps/worker/internal/queue"
)

var (
	logger            = log.New(log.Writer(), "worker.processor: ", log.Ldate|log.Ltime|log.Lshortfile)
	successCount int64 = 0
	failureCount int64 = 0
	totalDuration int64 = 0 // nanoseconds
)

// ProcessSQSEvent contains the actual business logic for handling webhook events.
// Returns error if processing fails (to trigger SQS retry).
func ProcessSQSEvent(event queue.SQSEvent) error {
	start := time.Now()
	logger.Printf("Processing SQS event: delivery_id=%s event_type=%s repo=%s sender=%s",
		event.DeliveryID, event.EventType, event.RepoName, event.SenderName)

	// Unmarshal payload for further processing
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		atomic.AddInt64(&failureCount, 1)
		logger.Printf("ERROR: Failed to unmarshal payload: delivery_id=%s error=%s", 
			event.DeliveryID, err.Error())
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	logger.Printf("DEBUG: Payload keys: %v", keys(payload))

	// Example: Add real business logic here
	if event.EventType == "push" {
		atomic.AddInt64(&failureCount, 1)
		logger.Printf("WARNING: Push event processing failed (simulated): delivery_id=%s",
			event.DeliveryID)
		return fmt.Errorf("simulated failure for push event")
	}

	// Simulate processing time
	time.Sleep(200 * time.Millisecond)

	atomic.AddInt64(&successCount, 1)
	dur := time.Since(start).Nanoseconds()
	atomic.AddInt64(&totalDuration, dur)
	logger.Printf("Successfully processed event: delivery_id=%s duration_ms=%.2f successes=%d failures=%d",
		event.DeliveryID, float64(dur)/1e6, atomic.LoadInt64(&successCount), atomic.LoadInt64(&failureCount))
	return nil
}

func keys(m map[string]interface{}) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	return k
}
