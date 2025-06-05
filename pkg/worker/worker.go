package worker

import (
	"context"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/queue"
)

var (
	// Deprecated: This retry constant is currently unused but reserved for future implementation of retry handling
	maxRetries     = 5
	idempotencyTTL = 24 * time.Hour
)

type SQSReceiverDeleter interface {
	ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.SQSEvent, []string, error)
	DeleteMessage(ctx context.Context, receiptHandle string) error
}

type RedisIdempotency interface {
	Exists(ctx context.Context, key string) (int64, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

func RunWorker(ctx context.Context, sqsClient SQSReceiverDeleter, redisClient RedisIdempotency, processFunc func(queue.SQSEvent) error) error {
	for {
		events, handles, err := sqsClient.ReceiveEvents(ctx, 5, 10)
		if err != nil {
			return err
		}
		for i, event := range events {
			idKey := "github:webhook:processed:" + event.DeliveryID
			exists, err := redisClient.Exists(ctx, idKey)
			if err != nil {
				continue
			}
			if exists == 1 {
				_ = sqsClient.DeleteMessage(ctx, handles[i]) // Already processed, delete
				continue
			}
			err = processFunc(event)
			if err == nil {
				if err := redisClient.Set(ctx, idKey, "1", idempotencyTTL); err != nil {
					// Redis set error - log but don't fail since we've already processed the event
					_ = err
				}
				_ = sqsClient.DeleteMessage(ctx, handles[i])
			}
			// else: Let SQS retry by not deleting message
		}
	}
}
