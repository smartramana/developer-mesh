package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/queue"
	"github.com/S-Corkum/devops-mcp/internal/worker"
	"github.com/go-redis/redis/v8"
)

type redisIdempotencyAdapter struct {
	client *redis.Client
}

func (r *redisIdempotencyAdapter) Exists(ctx context.Context, key string) (int64, error) {
	return r.client.Exists(ctx, key).Result()
}
func (r *redisIdempotencyAdapter) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func run(ctx context.Context, sqsClient worker.SQSReceiverDeleter, redisClient worker.RedisIdempotency, processFunc func(queue.SQSEvent) error) error {
	log.Println("Starting SQS worker...")
	return worker.RunWorker(ctx, sqsClient, redisClient, processFunc)
}

func main() {
	ctx := context.Background()

	// Initialize SQS client
	sqsClient, err := queue.NewSQSClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize SQS client: %v", err)
	}

	// Initialize Redis client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	redisAdapter := &redisIdempotencyAdapter{client: redisClient}

	// Use the real event processing function
	processFunc := worker.ProcessSQSEvent

	err = run(ctx, sqsClient, redisAdapter, processFunc)
	if err != nil {
		log.Fatalf("Worker exited with error: %v", err)
	}
}
