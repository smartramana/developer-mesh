# Worker Module Documentation

## Overview

The Worker Module provides a standardized way to process SQS events within the developer-mesh system. This package has been fully migrated as part of the Go Workspace migration and follows the forward-only migration approach.

## Key Components

### SQS Event Processing

The `worker` package provides the following key components:

1. **ProcessSQSEvent**: Core function to process an SQS event.
   ```go
   func ProcessSQSEvent(event queue.SQSEvent) error
   ```

2. **SQSReceiverDeleter**: Interface defining the necessary operations for receiving and deleting SQS messages.
   ```go
   type SQSReceiverDeleter interface {
       ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.SQSEvent, []string, error)
       DeleteMessage(ctx context.Context, receiptHandle string) error
   }
   ```

3. **RedisIdempotency**: Interface for implementing idempotent event processing.
   ```go
   type RedisIdempotency interface {
       Exists(ctx context.Context, key string) (int64, error)
       Set(ctx context.Context, key string, value string, ttl time.Duration) error
   }
   ```

4. **RunWorker**: Function to start a worker process that continuously polls for SQS events.
   ```go
   func RunWorker(ctx context.Context, sqsClient SQSReceiverDeleter, redisClient RedisIdempotency, processFunc func(queue.SQSEvent) error) error
   ```

## Usage Examples

### Processing Events with RunWorker

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/worker"
	"github.com/go-redis/redis/v8"
)

func main() {
	// Initialize context
	ctx := context.Background()

	// Initialize SQS client
	sqsConfig := queue.LoadSQSConfigFromEnv()
	sqsClient, err := queue.NewSQSClientAdapter(ctx, sqsConfig)
	if err != nil {
		log.Fatalf("Failed to initialize SQS client: %v", err)
	}

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Create Redis adapter that implements RedisIdempotency
	redisAdapter := &redisAdapter{client: redisClient}

	// Start worker
	err = worker.RunWorker(ctx, sqsClient, redisAdapter, worker.ProcessSQSEvent)
	if err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}

// Example Redis adapter implementation
type redisAdapter struct {
	client *redis.Client
}

func (r *redisAdapter) Exists(ctx context.Context, key string) (int64, error) {
	return r.client.Exists(ctx, key).Result()
}

func (r *redisAdapter) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}
```

### Custom Event Processing

You can implement custom event processing by creating your own processing function:

```go
func customEventProcessor(event queue.SQSEvent) error {
	log.Printf("Processing event: %s (type: %s)", event.DeliveryID, event.EventType)
	
	// Custom business logic here
	
	return nil
}

// Then use it with RunWorker
worker.RunWorker(ctx, sqsClient, redisAdapter, customEventProcessor)
```

## Migration Status

The worker module has been fully migrated from internal implementations to the pkg structure. All tests are passing with the following alignments:

1. **Type Alignment**: All worker components now use `pkg/queue` types instead of internal types
2. **Interface Consistency**: SQSReceiverDeleter interface matches in both implementations
3. **Deprecation**: All internal implementations have been properly deprecated with notices

## Best Practices

1. **Always use pkg implementations**: Import and use `github.com/developer-mesh/developer-mesh/pkg/worker` and `github.com/developer-mesh/developer-mesh/pkg/queue`
2. **Idempotent Processing**: Always implement idempotent event processing to prevent duplicate processing
3. **Error Handling**: Return errors from `ProcessSQSEvent` to trigger SQS retries, or nil to acknowledge successful processing
4. **Observability**: Use the logger with appropriate context to ensure proper monitoring and debugging

## Troubleshooting

Common issues and solutions:

1. **SQS Connectivity**: Ensure proper AWS credentials and endpoint configuration
2. **Redis Connectivity**: Verify Redis connection parameters and authentication
3. **Message Format**: Ensure SQS messages are properly formatted as `queue.SQSEvent`
4. **Error Handling**: Check error handling in custom processors to ensure proper retry behavior
