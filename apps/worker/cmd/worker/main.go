package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/worker/internal/worker"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/common/config"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	pkgworker "github.com/developer-mesh/developer-mesh/pkg/worker"
	redis "github.com/redis/go-redis/v9"
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

// redisIdempotencyAdapter adapts Redis client to the worker interface
type redisIdempotencyAdapter struct {
	client *redis.Client
}

func (r *redisIdempotencyAdapter) Exists(ctx context.Context, key string) (int64, error) {
	return r.client.Exists(ctx, key).Result()
}

func (r *redisIdempotencyAdapter) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// validateEmbeddingConfig validates embedding configuration parameters
func validateEmbeddingConfig(cfg *config.EmbeddingConfig) error {
	if cfg == nil {
		return fmt.Errorf("embedding configuration is nil")
	}

	// Validate Bedrock configuration if enabled
	if cfg.Providers.Bedrock.Enabled {
		region := cfg.Providers.Bedrock.Region
		if region == "" {
			return fmt.Errorf("bedrock region is required when bedrock is enabled")
		}

		// Validate AWS region format (e.g., us-east-1, eu-west-2)
		// Pattern: 2 lowercase letters, dash, lowercase region name, dash, single digit
		if len(region) < 9 {
			return fmt.Errorf("bedrock region format appears invalid: %s", region)
		}
	}

	// Validate OpenAI configuration if enabled
	if cfg.Providers.OpenAI.Enabled {
		if cfg.Providers.OpenAI.APIKey == "" {
			return fmt.Errorf("openai API key is required when openai is enabled")
		}
	}

	// Validate Google configuration if enabled
	if cfg.Providers.Google.Enabled {
		if cfg.Providers.Google.APIKey == "" {
			return fmt.Errorf("google API key is required when google is enabled")
		}
	}

	return nil
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
		fmt.Println("Health check passed")
		os.Exit(0)
	}

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
	// Initialize logger - use standard logger for debugging
	logger := observability.NewStandardLogger("worker")

	// Initialize Redis queue client
	queueClient, err := queue.NewClient(ctx, &queue.Config{
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize queue client: %w", err)
	}
	defer func() {
		if err := queueClient.Close(); err != nil {
			logger.Warn("Failed to close queue client", map[string]interface{}{"error": err.Error()})
		}
	}()

	// Initialize Redis client for idempotency
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	log.Printf("Connecting to Redis at %s", redisAddr)

	// Configure Redis options with TLS support
	redisOptions := &redis.Options{
		Addr: redisAddr,
	}

	// Check if TLS is enabled
	if os.Getenv("REDIS_TLS_ENABLED") == "true" {
		log.Printf("Redis TLS enabled")
		redisOptions.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: os.Getenv("REDIS_TLS_SKIP_VERIFY") == "true", // #nosec G402 - Configurable for dev
		}
	}

	redisClient := redis.NewClient(redisOptions)

	// Test Redis connection
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	redisAdapter := &redisIdempotencyAdapter{client: redisClient}

	// Get database connection
	dbPort := 5432
	if portStr := os.Getenv("DATABASE_PORT"); portStr != "" {
		if _, err := fmt.Sscanf(portStr, "%d", &dbPort); err != nil {
			logger.Error("Invalid database port", map[string]interface{}{
				"port":  portStr,
				"error": err.Error(),
			})
			dbPort = 5432 // Use default
		}
	}

	dbConfig := database.Config{
		Driver:     "postgres",
		Host:       os.Getenv("DATABASE_HOST"),
		Port:       dbPort,
		Database:   os.Getenv("DATABASE_NAME"),
		Username:   os.Getenv("DATABASE_USER"),
		Password:   os.Getenv("DATABASE_PASSWORD"),
		SSLMode:    os.Getenv("DATABASE_SSL_MODE"),
		SearchPath: os.Getenv("DATABASE_SEARCH_PATH"),
	}

	if dbConfig.Host == "" {
		dbConfig.Host = "localhost"
	}
	if dbConfig.SSLMode == "" {
		dbConfig.SSLMode = "disable"
	}
	if dbConfig.SearchPath == "" {
		dbConfig.SearchPath = "mcp,public"
	}

	// Connect to database with retry logic
	var db *database.Database
	maxRetries := 10
	baseDelay := 1 * time.Second

	logger.Info("Connecting to database with retry logic", map[string]interface{}{
		"host":     dbConfig.Host,
		"database": dbConfig.Database,
	})

	for i := 0; i < maxRetries; i++ {
		db, err = database.NewDatabase(ctx, dbConfig)
		if err == nil {
			// Test connection
			if pingErr := db.Ping(); pingErr == nil {
				break // Success!
			} else {
				// Close failed connection
				if closeErr := db.Close(); closeErr != nil {
					logger.Warn("Failed to close database connection", map[string]interface{}{"error": closeErr.Error()})
				}
				err = fmt.Errorf("failed to ping database: %w", pingErr)
			}
		}

		if i < maxRetries-1 {
			delay := baseDelay * (1 << uint(i)) // Exponential backoff
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			logger.Info("Database connection failed, retrying...", map[string]interface{}{
				"attempt":      i + 1,
				"max_attempts": maxRetries,
				"delay":        delay.String(),
				"error":        err.Error(),
			})

			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
	}

	logger.Info("Database connection established, waiting for tables...", nil)

	// Wait for tables to be ready
	readinessChecker := database.NewReadinessChecker(db.GetDB())
	readinessChecker.SetLogger(func(format string, args ...interface{}) {
		logger.Info(fmt.Sprintf(format, args...), nil)
	})

	if err := readinessChecker.WaitForTablesWithBackoff(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			logger.Warn("Failed to close database connection", map[string]interface{}{"error": closeErr.Error()})
		}
		return fmt.Errorf("database tables not ready: %w", err)
	}

	logger.Info("Database fully initialized with all required tables", nil)

	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close database connection", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Initialize cache client for embedding service
	// Create Redis cache configuration from existing Redis client
	cacheConfig := cache.RedisConfig{
		Address:      redisAddr,
		Database:     0,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
		PoolTimeout:  3,
	}

	// Initialize cache client
	cacheClient, err := cache.NewCache(ctx, cacheConfig)
	if err != nil {
		logger.Warn("Failed to initialize cache client", map[string]interface{}{
			"error": err.Error(),
		})
		// Continue without cache - embedding service will work, just slower
	} else {
		logger.Info("Cache client initialized successfully", map[string]interface{}{
			"address": redisAddr,
		})
	}

	// Initialize embedding service for context processing (optional)
	var contextEmbeddingProcessor *worker.ContextEmbeddingProcessor

	// Load full configuration to support all embedding options
	embeddingConfig, err := config.Load()
	if err != nil {
		logger.Warn("Failed to load configuration, context embeddings disabled", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		logger.Info("Configuration loaded successfully", map[string]interface{}{
			"bedrock_enabled": embeddingConfig.Embedding.Providers.Bedrock.Enabled,
			"bedrock_region":  embeddingConfig.Embedding.Providers.Bedrock.Region,
			"openai_enabled":  embeddingConfig.Embedding.Providers.OpenAI.Enabled,
			"google_enabled":  embeddingConfig.Embedding.Providers.Google.Enabled,
		})
	}
	if err == nil && embeddingConfig.Embedding.Providers.Bedrock.Enabled {
		// Validate Bedrock configuration
		if err := validateEmbeddingConfig(&embeddingConfig.Embedding); err != nil {
			logger.Warn("Invalid embedding configuration, context embeddings disabled", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			logger.Info("Initializing embedding service", map[string]interface{}{
				"provider":     "bedrock",
				"region":       embeddingConfig.Embedding.Providers.Bedrock.Region,
				"has_endpoint": embeddingConfig.Embedding.Providers.Bedrock.Endpoint != "",
			})

			// Create embedding service using the factory with cache client
			embeddingService, err := embedding.CreateEmbeddingServiceV2(embeddingConfig, *db, cacheClient)
			if err != nil {
				logger.Warn("Failed to create embedding service, context embeddings disabled", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				// Create context repository
				contextRepo := repository.NewPostgresContextRepository(db.GetDB())

				// Create context embedding processor
				contextEmbeddingProcessor = worker.NewContextEmbeddingProcessor(
					embeddingService,
					contextRepo,
					logger,
					nil,
				)

				logger.Info("Context embedding processor initialized successfully", map[string]interface{}{
					"provider": "bedrock",
					"region":   embeddingConfig.Embedding.Providers.Bedrock.Region,
				})
			}
		}
	} else {
		logger.Info("Bedrock embedding provider not enabled, context embedding processor disabled", map[string]interface{}{
			"openai_enabled": embeddingConfig.Embedding.Providers.OpenAI.Enabled,
			"google_enabled": embeddingConfig.Embedding.Providers.Google.Enabled,
		})
	}

	// Create event processor with retry and DLQ support
	eventProcessor, err := worker.NewEventProcessor(logger, nil, db.GetDB(), queueClient)
	if err != nil {
		return fmt.Errorf("failed to create event processor: %w", err)
	}

	// Add context embedding processor if available
	if contextEmbeddingProcessor != nil {
		eventProcessor.SetContextEmbeddingProcessor(contextEmbeddingProcessor)
		logger.Info("Context embedding processor attached to event processor", nil)
	}

	// Create processor function that uses the new event processor
	processorFunc := func(event queue.Event) error {
		return eventProcessor.ProcessEvent(ctx, event)
	}

	// Create Redis worker
	redisWorker, err := pkgworker.NewRedisWorker(&pkgworker.Config{
		QueueClient:    queueClient,
		RedisClient:    redisAdapter,
		Processor:      processorFunc,
		Logger:         logger,
		ConsumerName:   fmt.Sprintf("worker-%s", os.Getenv("HOSTNAME")),
		IdempotencyTTL: 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("failed to create worker: %w", err)
	}

	// Create DLQ handler for periodic processing
	dlqHandler := worker.NewDLQHandler(db.GetDB(), logger, nil, queueClient)
	dlqWorker := worker.NewDLQWorker(dlqHandler, logger, 5*time.Minute)

	// Create metrics collector and performance monitor
	tracer := observability.GetTracer()
	metricsClient := observability.NewMetricsClient()
	metricsCollector := worker.NewMetricsCollector(metricsClient, tracer)

	// Create performance monitor
	perfMonitor := worker.NewPerformanceMonitor(metricsCollector, logger, 30*time.Second)

	// Create health checker
	healthChecker := worker.NewHealthChecker(db, redisClient, queueClient, metricsCollector, logger)

	// Start health endpoint in background
	go func() {
		healthAddr := os.Getenv("HEALTH_ENDPOINT")
		if healthAddr == "" {
			healthAddr = ":8088"
		}
		if err := healthChecker.StartHealthEndpoint(healthAddr); err != nil {
			log.Printf("Health endpoint error: %v", err)
		}
	}()

	// Start performance monitor in background
	go func() {
		if err := perfMonitor.Run(ctx); err != nil {
			log.Printf("Performance monitor error: %v", err)
		}
	}()

	// Start DLQ worker in background
	go func() {
		if err := dlqWorker.Run(ctx); err != nil {
			log.Printf("DLQ worker error: %v", err)
		}
	}()

	log.Println("Starting Redis worker with retry and DLQ support...")
	log.Printf("Health endpoint available at %s/health", os.Getenv("HEALTH_ENDPOINT"))

	// DEBUG: Log before calling Run
	log.Println("DEBUG: About to call redisWorker.Run(ctx)")
	err = redisWorker.Run(ctx)
	log.Printf("DEBUG: redisWorker.Run() returned with error: %v", err)
	return err
}

// performHealthCheck performs a basic health check
func performHealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check Redis connectivity
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Configure Redis options with TLS support
	redisOptions := &redis.Options{
		Addr: redisAddr,
	}

	// Check if TLS is enabled
	if os.Getenv("REDIS_TLS_ENABLED") == "true" {
		redisOptions.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: os.Getenv("REDIS_TLS_SKIP_VERIFY") == "true", // #nosec G402
		}
	}

	redisClient := redis.NewClient(redisOptions)
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("Failed to close redis client: %v", err)
		}
	}()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	// Check queue connectivity
	queueClient, err := queue.NewClient(ctx, &queue.Config{})
	if err != nil {
		return fmt.Errorf("queue client health check failed: %w", err)
	}
	defer func() {
		if err := queueClient.Close(); err != nil {
			log.Printf("Failed to close queue client: %v", err)
		}
	}()

	if err := queueClient.Health(ctx); err != nil {
		return fmt.Errorf("queue health check failed: %w", err)
	}

	return nil
}
