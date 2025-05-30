package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/queue"
	"github.com/go-redis/redis/v8"
	"worker/internal/worker"
)

// Version information (set via ldflags during build)
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Command-line flags
var (
	showVersion = flag.Bool("version", false, "Show version information and exit")
	healthCheck = flag.Bool("health-check", false, "Perform health check and exit")
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
	flag.Parse()

	// Show version information if requested
	if *showVersion {
		fmt.Printf("Worker\nVersion: %s\nBuild Time: %s\nGit Commit: %s\n", version, buildTime, gitCommit)
		os.Exit(0)
	}

	// Perform health check if requested
	if *healthCheck {
		if err := performHealthCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start worker in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := runWorker(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		cancel()
		// Give worker time to shut down gracefully
		time.Sleep(5 * time.Second)
	case err := <-errChan:
		log.Fatalf("Worker error: %v", err)
	}

	log.Println("Worker stopped")
}

func runWorker(ctx context.Context) error {
	// Load SQS adapter configuration from environment variables
	sqsConfig := queue.LoadSQSConfigFromEnv()
	log.Printf("SQS Configuration: mockMode=%v, useLocalStack=%v, endpoint=%s",
		sqsConfig.MockMode, sqsConfig.UseLocalStack, sqsConfig.Endpoint)

	// Initialize SQS client with the adapter pattern
	// The adapter handles production, LocalStack, and mock environments transparently
	sqsAdapter, err := queue.NewSQSClientAdapter(ctx, sqsConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize SQS client adapter: %w", err)
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
	
	// Test Redis connection
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	
	redisAdapter := &redisIdempotencyAdapter{client: redisClient}

	// Use the real event processing function
	processFunc := worker.ProcessSQSEvent

	log.Println("Starting worker with SQS adapter...")
	return run(ctx, workerAdapter, redisAdapter, processFunc)
}

// performHealthCheck performs a basic health check
func performHealthCheck() error {
	// Basic health check - verify the application can start
	// Check Redis connectivity
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}
	
	return nil
}