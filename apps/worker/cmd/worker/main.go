package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/S-Corkum/devops-mcp/apps/worker/internal/queue"
	"github.com/S-Corkum/devops-mcp/apps/worker/internal/worker"
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

// workerSQSAdapter is an adapter that makes our SQSClientAdapter compatible with worker.SQSReceiverDeleter
type workerSQSAdapter struct {
	adapter queue.SQSAdapter
}

// ReceiveEvents implements the worker.SQSReceiverDeleter interface
func (w *workerSQSAdapter) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.SQSEvent, []string, error) {
	log.Printf("workerSQSAdapter: Receiving events (max: %d, wait: %ds)", maxMessages, waitSeconds)
	return w.adapter.ReceiveEvents(ctx, maxMessages, waitSeconds)
}

// DeleteMessage implements the worker.SQSReceiverDeleter interface
func (w *workerSQSAdapter) DeleteMessage(ctx context.Context, receiptHandle string) error {
	log.Printf("workerSQSAdapter: Deleting message")
	return w.adapter.DeleteMessage(ctx, receiptHandle)
}

func main() {
	ctx := context.Background()

	// Load SQS adapter configuration from environment variables
	sqsConfig := queue.LoadSQSConfigFromEnv()
	log.Printf("SQS Configuration: mockMode=%v, useLocalStack=%v, endpoint=%s", 
		sqsConfig.MockMode, sqsConfig.UseLocalStack, sqsConfig.Endpoint)

	// Initialize SQS client with the adapter pattern
	// The adapter handles production, LocalStack, and mock environments transparently
	sqsAdapter, err := queue.NewSQSClientAdapter(ctx, sqsConfig)
	if err != nil {
		log.Fatalf("Failed to initialize SQS client adapter: %v", err)
	}

	// Wrap our SQS adapter in the worker-compatible adapter
	workerAdapter := &workerSQSAdapter{adapter: sqsAdapter}

	// Initialize Redis client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	log.Printf("Connecting to Redis at %s", redisAddr)
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	redisAdapter := &redisIdempotencyAdapter{client: redisClient}

	// Use the real event processing function
	processFunc := worker.ProcessSQSEvent

	log.Println("Starting worker with SQS adapter...")
	err = run(ctx, workerAdapter, redisAdapter, processFunc)
	if err != nil {
		log.Fatalf("Worker exited with error: %v", err)
	}
}
