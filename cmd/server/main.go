package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	mathrand "math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/S-Corkum/mcp-server/internal/api"
	"github.com/S-Corkum/mcp-server/internal/cache"
	"github.com/S-Corkum/mcp-server/internal/config"
	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/metrics"
	"github.com/S-Corkum/mcp-server/internal/storage"
	aws "github.com/S-Corkum/mcp-server/internal/storage"
	"github.com/S-Corkum/mcp-server/internal/storage/providers"
	
	// Import PostgreSQL driver
	_ "github.com/lib/pq"
)

func main() {
	// Initialize secure random seed
	initSecureRandom()
	
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Validate critical configuration
	if err := validateConfiguration(cfg); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize metrics
	metricsClient := metrics.NewClient(cfg.Metrics)
	defer metricsClient.Close()

	// Initialize database
	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize cache
	cacheClient, err := cache.NewCache(ctx, cfg.Cache)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cacheClient.Close()

	// Initialize storage if S3 is configured
	var s3Client *aws.S3Client
	var contextStorage providers.ContextStorage
	var contextManager interfaces.ContextManager
	
	if cfg.Storage.Type == "s3" && cfg.Storage.ContextStorage.Provider == "s3" {
		log.Println("Initializing S3 storage for contexts")
		s3Client, err = aws.NewS3Client(ctx, cfg.Storage.S3)
		if err != nil {
			log.Fatalf("Failed to initialize S3 client: %v", err)
		}
		
		// Initialize context storage provider
		contextStorage = providers.NewS3ContextStorage(s3Client, cfg.Storage.ContextStorage.S3PathPrefix)
		
		// Create context reference table if it doesn't exist
		err = db.CreateContextReferenceTable(ctx)
		if err != nil {
			log.Fatalf("Failed to create context reference table: %v", err)
		}
		
		// Initialize S3-backed context manager
		contextManager = core.NewS3ContextManager(db, cacheClient, contextStorage)
		log.Println("S3 context storage initialized successfully")
	} else {
		// Use standard database-backed context manager
		contextManager = core.NewContextManager(db, cacheClient)
		log.Println("Database context storage initialized")
	}
	
	// Initialize core engine with the appropriate context manager
	engine, err := core.NewEngine(ctx, cfg.Engine, db, cacheClient, metricsClient, contextManager)
	if err != nil {
		log.Fatalf("Failed to initialize core engine: %v", err)
	}
	defer engine.Shutdown(ctx)

	// Initialize API server
	server := api.NewServer(engine, cfg.API)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", cfg.API.ListenAddress)
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Received shutdown signal")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Shutdown API server first
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// initSecureRandom initializes the math/rand package with a secure seed
func initSecureRandom() {
	// Generate a secure random seed using crypto/rand
	max := big.NewInt(int64(1) << 62)
	val, err := rand.Int(rand.Reader, max)
	if err != nil {
		// If we can't get a secure seed, use time as a fallback
		log.Printf("Warning: unable to generate secure random seed: %v", err)
		mathrand.Seed(time.Now().UnixNano())
		return
	}
	
	// Seed math/rand with the secure value
	mathrand.Seed(val.Int64())
	log.Println("Initialized secure random generator")
}

// validateConfiguration validates critical configuration settings
func validateConfiguration(cfg *config.Config) error {
	// Check database configuration
	if cfg.Database.DSN == "" && (cfg.Database.Host == "" || cfg.Database.Port == 0 || cfg.Database.Database == "") {
		return fmt.Errorf("invalid database configuration: DSN or host/port/database must be provided")
	}
	
	// Validate API configuration
	if cfg.API.ReadTimeout == 0 || cfg.API.WriteTimeout == 0 || cfg.API.IdleTimeout == 0 {
		return fmt.Errorf("invalid API timeouts: must be greater than 0")
	}
	
	// Check webhook secrets if webhooks are enabled
	if cfg.API.Webhooks.GitHub.Enabled && cfg.API.Webhooks.GitHub.Secret == "" {
		log.Println("Warning: GitHub webhooks enabled without a secret - consider adding a secret for security")
	}
	
	if cfg.API.Webhooks.Harness.Enabled && cfg.API.Webhooks.Harness.Secret == "" {
		log.Println("Warning: Harness webhooks enabled without a secret - consider adding a secret for security")
	}
	
	if cfg.API.Webhooks.SonarQube.Enabled && cfg.API.Webhooks.SonarQube.Secret == "" {
		log.Println("Warning: SonarQube webhooks enabled without a secret - consider adding a secret for security")
	}
	
	if cfg.API.Webhooks.Artifactory.Enabled && cfg.API.Webhooks.Artifactory.Secret == "" {
		log.Println("Warning: Artifactory webhooks enabled without a secret - consider adding a secret for security")
	}
	
	if cfg.API.Webhooks.Xray.Enabled && cfg.API.Webhooks.Xray.Secret == "" {
		log.Println("Warning: Xray webhooks enabled without a secret - consider adding a secret for security")
	}
	
	return nil
}
