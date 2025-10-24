package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	redisClient "github.com/developer-mesh/developer-mesh/pkg/redis"
)

// RedisStreamsPublisher publishes events to Redis Streams
type RedisStreamsPublisher struct {
	client     *redisClient.StreamsClient
	streamName string
	logger     observability.Logger
}

// NewRedisStreamsPublisher creates a new Redis Streams event publisher
func NewRedisStreamsPublisher(
	client *redisClient.StreamsClient,
	streamName string,
	logger observability.Logger,
) *RedisStreamsPublisher {
	return &RedisStreamsPublisher{
		client:     client,
		streamName: streamName,
		logger:     logger,
	}
}

// Publish publishes an event to Redis Streams
func (p *RedisStreamsPublisher) Publish(ctx context.Context, event interface{}) error {
	// Extract base event to get type and tenant
	var eventType string
	var tenantID string

	// Use type assertion to get event metadata
	switch e := event.(type) {
	case *TaskCreatedEvent:
		eventType = e.Type
		tenantID = e.TenantID
	case *TaskStatusChangedEvent:
		eventType = e.Type
		tenantID = e.TenantID
	case *TaskCompletedEvent:
		eventType = e.Type
		tenantID = e.TenantID
	case *TaskFailedEvent:
		eventType = e.Type
		tenantID = e.TenantID
	case *TaskDelegatedEvent:
		eventType = e.Type
		tenantID = e.TenantID
	case *TaskAcceptedEvent:
		eventType = e.Type
		tenantID = e.TenantID
	case *TaskRejectedEvent:
		eventType = e.Type
		tenantID = e.TenantID
	case *TaskEscalatedEvent:
		eventType = e.Type
		tenantID = e.TenantID
	default:
		return fmt.Errorf("unsupported event type: %T", event)
	}

	// Serialize event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("Failed to marshal event", map[string]interface{}{
			"error":      err.Error(),
			"event_type": eventType,
		})
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Prepare stream values
	values := map[string]interface{}{
		"event_type": eventType,
		"tenant_id":  tenantID,
		"payload":    string(eventJSON),
	}

	// Add to Redis Stream
	messageID, err := p.client.AddToStream(ctx, p.streamName, values)
	if err != nil {
		p.logger.Error("Failed to publish event to Redis Stream", map[string]interface{}{
			"error":       err.Error(),
			"event_type":  eventType,
			"tenant_id":   tenantID,
			"stream_name": p.streamName,
		})
		return fmt.Errorf("failed to add event to stream: %w", err)
	}

	p.logger.Debug("Published event to Redis Stream", map[string]interface{}{
		"event_type":  eventType,
		"tenant_id":   tenantID,
		"stream_name": p.streamName,
		"message_id":  messageID,
	})

	return nil
}

// PublishBatch publishes multiple events to Redis Streams
func (p *RedisStreamsPublisher) PublishBatch(ctx context.Context, events []interface{}) error {
	for _, event := range events {
		if err := p.Publish(ctx, event); err != nil {
			// Log error but continue with other events
			p.logger.Error("Failed to publish event in batch", map[string]interface{}{
				"error": err.Error(),
			})
			// Return the first error
			return err
		}
	}
	return nil
}

// Close closes the publisher
func (p *RedisStreamsPublisher) Close() error {
	// Redis client is shared, so we don't close it here
	p.logger.Info("Redis Streams publisher closed", map[string]interface{}{
		"stream_name": p.streamName,
	})
	return nil
}
