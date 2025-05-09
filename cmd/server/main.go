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

	"github.com/S-Corkum/devops-mcp/internal/api"
	"github.com/S-Corkum/devops-mcp/internal/aws"
	"github.com/S-Corkum/devops-mcp/internal/cache"
	commonConfig "github.com/S-Corkum/devops-mcp/internal/common/config"
	"github.com/S-Corkum/devops-mcp/internal/config"
	"github.com/S-Corkum/devops-mcp/internal/core"
	"github.com/S-Corkum/devops-mcp/internal/database"
	"github.com/S-Corkum/devops-mcp/internal/metrics"
	"github.com/S-Corkum/devops-mcp/internal/observability"

	// Import PostgreSQL driver
	_ "github.com/lib/pq"
)

func main() {
	// Initialize secure random seed
	initSecureRandom()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate critical configuration
	if err := validateConfiguration(cfg); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize logging
	logger := observability.NewLogger("server")

	// Initialize metrics
	metricsClient := metrics.NewClient(cfg.Metrics)
	defer metricsClient.Close()

	// Check if IRSA is enabled (IAM Roles for Service Accounts)
	if aws.IsIRSAEnabled() {
		logger.Info("IRSA (IAM Roles for Service Accounts) is enabled for AWS services", map[string]interface{}{
			"aws_role_arn":                os.Getenv("AWS_ROLE_ARN"),
			"aws_web_identity_token_file": os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"),
		})
	} else {
		logger.Info("IRSA not detected, will use standard AWS credential provider chain if IAM auth is enabled", nil)
	}

	// Prepare database config with AWS integration if needed
	var db *database.Database
	var dbConfig database.Config
	if cfg.AWS.RDS.UseIAMAuth && aws.IsIRSAEnabled() {
		logger.Info("Using IAM authentication for RDS", nil)
		useAWS := true
		useIAM := true
		
		// Convert AWS RDSConfig to common config RDSConfig
		commonRDSConfig := &commonConfig.RDSConfig{
			Host:              cfg.AWS.RDS.Host,
			Port:              cfg.AWS.RDS.Port,
			Database:          cfg.AWS.RDS.Database,
			Username:          cfg.AWS.RDS.Username,
			Password:          cfg.AWS.RDS.Password,
			UseIAMAuth:        cfg.AWS.RDS.UseIAMAuth,
			TokenExpiration:   cfg.AWS.RDS.TokenExpiration,
			MaxOpenConns:      cfg.AWS.RDS.MaxOpenConns,
			MaxIdleConns:      cfg.AWS.RDS.MaxIdleConns,
			ConnMaxLifetime:   cfg.AWS.RDS.ConnMaxLifetime,
			EnablePooling:     cfg.AWS.RDS.EnablePooling,
			MinPoolSize:       cfg.AWS.RDS.MinPoolSize,
			MaxPoolSize:       cfg.AWS.RDS.MaxPoolSize,
			ConnectionTimeout: cfg.AWS.RDS.ConnectionTimeout,
			AuthConfig: struct {
				Region    string `mapstructure:"region"`
				Endpoint  string `mapstructure:"endpoint"`
				AssumeRole string `mapstructure:"assume_role"`
			}{
				Region:    cfg.AWS.RDS.AuthConfig.Region,
				Endpoint:  cfg.AWS.RDS.AuthConfig.Endpoint,
				AssumeRole: cfg.AWS.RDS.AuthConfig.AssumeRole,
			},
		}
		
		dbConfig = database.Config{
			Driver:          "postgres",
			UseAWS:          &useAWS,
			UseIAM:          &useIAM,
			RDSConfig:       commonRDSConfig,
			MaxOpenConns:    cfg.AWS.RDS.MaxOpenConns,
			MaxIdleConns:    cfg.AWS.RDS.MaxIdleConns,
			ConnMaxLifetime: cfg.AWS.RDS.ConnMaxLifetime,
		}
	} else {
		dbConfig = cfg.Database
	}

	// Initialize database
	db, err = database.NewDatabase(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	if db == nil || db.GetDB() == nil {
		log.Fatalf("Database pointer or underlying *sqlx.DB is nil after initialization!")
	}
	// Ping the DB to verify connection is alive
	if err := db.GetDB().Ping(); err != nil {
		log.Fatalf("Database connection is not alive (ping failed): %v", err)
	}
	log.Printf("Database initialized successfully: db=%p, sqlx.DB=%p", db, db.GetDB())
	defer db.Close()

	// Prepare cache config with AWS integration if needed
	var cacheClient cache.Cache
	var cacheConfig cache.RedisConfig
	if cfg.AWS.ElastiCache.UseIAMAuth && aws.IsIRSAEnabled() {
		logger.Info("Using IAM authentication for ElastiCache", nil)
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
	cacheClient, err = cache.NewCache(ctx, cacheConfig)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cacheClient.Close()

	// Initialize engine
	var engine *core.Engine
	engine, err = core.NewEngine(ctx, cfg.Engine, db, cacheClient, metricsClient)
	if err != nil {
		log.Fatalf("Failed to initialize core engine: %v", err)
	}
	defer engine.Shutdown(ctx)

	// Convert interfaces.APIConfig to api.Config
	apiConfig := api.Config{
		ListenAddress: cfg.API.ListenAddress,
		ReadTimeout:   cfg.API.ReadTimeout,
		WriteTimeout:  cfg.API.WriteTimeout,
		IdleTimeout:   cfg.API.IdleTimeout,
		EnableCORS:    cfg.API.EnableCORS,
		EnableSwagger: cfg.API.EnableSwagger,
		TLSCertFile:   cfg.API.TLSCertFile,
		TLSKeyFile:    cfg.API.TLSKeyFile,
		Auth: api.AuthConfig{
			JWTSecret: cfg.API.Auth.JWTSecret,
			APIKeys:   cfg.API.Auth.APIKeys,
		},
		RateLimit: api.RateLimitConfig{
			Enabled:     cfg.API.RateLimit.Enabled,
			Limit:       cfg.API.RateLimit.Limit,
			Period:      time.Minute, // Default value
			BurstFactor: 3,           // Default value
		},
		// Default values for other fields
		Versioning: api.VersioningConfig{
			Enabled:           true,
			DefaultVersion:    "1.0",
			SupportedVersions: []string{"1.0"},
		},
		Performance: api.DefaultConfig().Performance,
	}

	// Create a MetricsClient instance
	obsMetricsClient := observability.NewMetricsClient()
	
	// Initialize API server
	server := api.NewServer(engine, apiConfig, db.GetDB(), obsMetricsClient, cfg)

	// Initialize server components
	if err := server.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize server components: %v", err)
	}

	// Determine the correct port based on environment
	port := cfg.GetListenPort()
	logger.Info("Server configuration", map[string]interface{}{
		"port":      port,
		"env":       cfg.Environment,
		"vector_db": cfg.Database.Vector.Enabled,
	})

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server", map[string]interface{}{
			"address": cfg.API.ListenAddress,
		})

		// If we're in production and TLS is configured, use HTTPS
		if cfg.IsProduction() && cfg.API.TLSCertFile != "" && cfg.API.TLSKeyFile != "" {
			logger.Info("Starting server with TLS (HTTPS)", nil)
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
	logger.Info("Received shutdown signal", nil)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Shutdown API server first
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("API server shutdown error", map[string]interface{}{
			"error": err.Error(),
		})
	}

	logger.Info("Server stopped gracefully", nil)
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

	return nil
}
