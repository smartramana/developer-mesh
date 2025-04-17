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
	"github.com/S-Corkum/mcp-server/internal/aws"
	"github.com/S-Corkum/mcp-server/internal/cache"
	"github.com/S-Corkum/mcp-server/internal/config"
	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/metrics"
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/S-Corkum/mcp-server/internal/storage"
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

	// Check if IRSA is enabled (IAM Roles for Service Accounts)
	if aws.IsIRSAEnabled() {
		log.Println("IRSA (IAM Roles for Service Accounts) is enabled for AWS services")
	}

	// Prepare database config with AWS integration if needed
	var dbConfig database.Config
	if cfg.AWS.RDS.UseIAMAuth && aws.IsIRSAEnabled() {
		log.Println("Using IAM authentication for RDS")
		dbConfig = database.Config{
			Driver:         "postgres",
			UseAWS:         true,
			UseIAM:         true,
			RDSConfig:      &cfg.AWS.RDS,
			MaxOpenConns:   cfg.AWS.RDS.MaxOpenConns,
			MaxIdleConns:   cfg.AWS.RDS.MaxIdleConns,
			ConnMaxLifetime: cfg.AWS.RDS.ConnMaxLifetime,
		}
	} else {
		dbConfig = cfg.Database
	}

	// Initialize database
	db, err := database.NewDatabase(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Get the underlying sqlx.DB connection for the repository
	sqlxDB := db.GetDB()

	// Prepare cache config with AWS integration if needed
	var cacheConfig cache.RedisConfig
	if cfg.AWS.ElastiCache.UseIAMAuth && aws.IsIRSAEnabled() {
		log.Println("Using IAM authentication for ElastiCache")
		cacheConfig = cache.RedisConfig{
			Type:              "redis_cluster",
			UseAWS:            true,
			ClusterMode:       cfg.AWS.ElastiCache.ClusterMode,
			ElastiCacheConfig: &cfg.AWS.ElastiCache,
			MaxRetries:        cfg.AWS.ElastiCache.MaxRetries,
			DialTimeout:       cfg.AWS.ElastiCache.DialTimeout,
			ReadTimeout:       cfg.AWS.ElastiCache.ReadTimeout,
			WriteTimeout:      cfg.AWS.ElastiCache.WriteTimeout,
			PoolSize:          cfg.AWS.ElastiCache.PoolSize,
			MinIdleConns:      cfg.AWS.ElastiCache.MinIdleConnections,
			PoolTimeout:       cfg.AWS.ElastiCache.PoolTimeout,
		}
	} else {
		cacheConfig = cfg.Cache
	}

	// Initialize cache
	cacheClient, err := cache.NewCache(ctx, cacheConfig)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cacheClient.Close()

	// Initialize storage if S3 is configured
	var s3Client *storage.S3Client
	var contextStorage providers.ContextStorage
	var contextManager interfaces.ContextManager
	
	if cfg.Storage.Type == "s3" && cfg.Storage.ContextStorage.Provider == "s3" {
		log.Println("Initializing S3 storage for contexts")
		
		// Create storage.S3Client with proper AWS authentication configuration
		s3Config := storage.S3Config{
			Region:           cfg.Storage.S3.Region,
			Bucket:           cfg.Storage.S3.Bucket,
			Endpoint:         cfg.Storage.S3.Endpoint,
			ForcePathStyle:   cfg.Storage.S3.ForcePathStyle,
			UploadPartSize:   cfg.Storage.S3.UploadPartSize,
			DownloadPartSize: cfg.Storage.S3.DownloadPartSize,
			Concurrency:      cfg.Storage.S3.Concurrency,
			RequestTimeout:   cfg.Storage.S3.RequestTimeout,
			AWSConfig: storage.AWSConfig{
				UseIAMAuth: cfg.AWS.S3.UseIAMAuth,
				Region:     cfg.AWS.S3.AuthConfig.Region,
				Endpoint:   cfg.AWS.S3.AuthConfig.Endpoint,
				AssumeRole: cfg.AWS.S3.AuthConfig.AssumeRole,
			},
		}
		
		// Log if we'll be using IAM authentication for S3
		if cfg.AWS.S3.UseIAMAuth && aws.IsIRSAEnabled() {
			log.Println("Using IAM authentication for S3")
		}
		
		// Create the S3Client directly using the storage package
		s3Client, err = storage.NewS3Client(ctx, s3Config)
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

	// Initialize embedding repository for vector operations
	embeddingRepo := repository.NewEmbeddingRepository(sqlxDB)
	
	// Initialize API server
	server := api.NewServer(engine, embeddingRepo, cfg.API)

	// Determine the correct port based on environment
	port := cfg.GetListenPort()
	log.Printf("Configured to listen on port %d", port)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", cfg.API.ListenAddress)
		
		// If we're in production and TLS is configured, use HTTPS
		if cfg.IsProduction() && cfg.API.TLSCertFile != "" && cfg.API.TLSKeyFile != "" {
			log.Println("Starting server with TLS (HTTPS)")
			if err := server.StartTLS(cfg.API.TLSCertFile, cfg.API.TLSKeyFile); err != nil {
				log.Fatalf("Failed to start server with TLS: %v", err)
			}
		} else {
			// Otherwise use HTTP
			if err := server.Start(); err != nil {
				log.Fatalf("Failed to start server: %v", err)
			}
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
		// Use the global Seed function which works across all supported Go versions
		mathrand.Seed(time.Now().UnixNano())
		return
	}
	
	// Seed the global random generator with our secure random value
	mathrand.Seed(val.Int64())
	log.Println("Initialized secure random generator")
}

// validateConfiguration validates critical configuration settings
func validateConfiguration(cfg *config.Config) error {
	// Check database configuration
	if cfg.Database.DSN == "" && (cfg.Database.Host == "" || cfg.Database.Port == 0 || cfg.Database.Database == "") {
		// If we're using AWS RDS with IAM authentication, we don't need DSN or database credentials
		if !(cfg.AWS.RDS.UseIAMAuth && cfg.AWS.RDS.Host != "") {
			return fmt.Errorf("invalid database configuration: DSN or host/port/database must be provided")
		}
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
