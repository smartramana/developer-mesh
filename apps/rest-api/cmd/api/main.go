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
	"strings"
	"syscall"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common/aws"
	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/common/config"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/interfaces"
	"github.com/S-Corkum/devops-mcp/pkg/metrics"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"rest-api/internal/api"
	"rest-api/internal/core"

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

	// DEBUG: Print loaded webhook config
	fmt.Printf("[DEBUG] Loaded webhook config: %+v\n", cfg.API.Webhook)

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
		logger.Info("IRSA (IAM Roles for Service Accounts) is enabled for AWS services", map[string]any{
			"aws_role_arn":                os.Getenv("AWS_ROLE_ARN"),
			"aws_web_identity_token_file": os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"),
		})
	} else {
		logger.Info("IRSA not detected, will use standard AWS credential provider chain if IAM auth is enabled", nil)
	}

	// Prepare database config
	var db *database.Database
	dbConfig := database.Config{
		Driver:          cfg.Database.Driver,
		DSN:             cfg.Database.DSN,
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Database:        cfg.Database.Database,
		Username:        cfg.Database.Username,
		Password:        cfg.Database.Password,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}

	// Check if we should use IAM authentication
	if cfg.Database.UseIAMAuth && aws.IsIRSAEnabled() {
		logger.Info("Using IAM authentication for RDS", nil)
		dbConfig.UseAWS = true
		dbConfig.UseIAM = true
		dbConfig.AWSRegion = cfg.Database.AuthConfig.Region
		dbConfig.RDSHost = cfg.Database.Host
		dbConfig.RDSPort = cfg.Database.Port
		dbConfig.RDSDatabase = cfg.Database.Database
		dbConfig.RDSUsername = cfg.Database.Username
		dbConfig.RDSTokenExpiration = cfg.Database.TokenExpiration
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
	// Initialize cache using the cache config from the configuration
	cacheConfig := cfg.Cache

	// Initialize cache
	cacheClient, err = cache.NewCache(ctx, cacheConfig)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cacheClient.Close()

	// Initialize core engine with logger
	engine := core.NewEngine(logger)
	defer engine.Shutdown(ctx)

	// Initialize context manager based on environment configuration
	useMock := os.Getenv("USE_MOCK_CONTEXT_MANAGER")
	if strings.ToLower(useMock) == "true" {
		logger.Info("Using mock context manager as specified by environment", nil)
		engine.SetContextManager(core.NewMockContextManager())
	} else {
		logger.Info("Using production context manager", nil)
		// Create metrics client for context manager
		ctxMetrics := observability.NewMetricsClient()
		engine.SetContextManager(core.NewContextManager(db.GetDB(), logger, ctxMetrics))
	}

	// Convert configuration to api.Config
	apiConfig := api.Config{
		ListenAddress: cfg.API.ListenAddress,
		ReadTimeout:   30 * time.Second, // Default value
		WriteTimeout:  30 * time.Second, // Default value
		IdleTimeout:   90 * time.Second, // Default value
		EnableCORS:    cfg.API.EnableCORS,
		EnableSwagger: cfg.API.EnableSwagger,
		TLSCertFile:   cfg.API.TLSCertFile,
		TLSKeyFile:    cfg.API.TLSKeyFile,
		Auth: api.AuthConfig{
			JWTSecret: getStringFromConfig(cfg.API.Auth, "jwt_secret"),
			APIKeys:   cfg.API.Auth["api_keys"],
		},
		RateLimit: api.RateLimitConfig{
			Enabled:     false,       // Default disabled
			Limit:       100,         // Default limit
			Period:      time.Minute, // Default value
			BurstFactor: 3,           // Default value
		},
		// Use default webhook configuration
		Webhook: interfaces.WebhookConfig{
			EnabledField:             false,
			GitHubEndpointField:      "/api/webhooks/github",
			GitHubSecretField:        "",
			GitHubIPValidationField:  true,
			GitHubAllowedEventsField: []string{"push", "pull_request"},
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
	// Note: Passing nil for the old config type as it's only used for vector DB which we configure separately
	server := api.NewServer(engine, apiConfig, db.GetDB(), obsMetricsClient, nil)

	// Initialize server components
	if err := server.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize server components: %v", err)
	}

	// Determine the correct port based on environment
	port := cfg.GetListenPort()
	logger.Info("Server configuration", map[string]any{
		"port":      port,
		"env":       cfg.Environment,
		"vector_db": cfg.Database.Vector.Enabled,
	})

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server", map[string]any{
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
		logger.Error("API server shutdown error", map[string]any{
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
		// If we're using IAM authentication from Database config, we don't need DSN or database credentials
		if !cfg.Database.UseIAMAuth {
			return fmt.Errorf("invalid database configuration: DSN or host/port/database must be provided")
		}
	}

	// Basic API validation
	if cfg.API.ListenAddress == "" {
		return fmt.Errorf("invalid API configuration: listen address must be provided")
	}

	return nil
}

// getStringFromConfig safely extracts a string value from a configuration map
func getStringFromConfig(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
