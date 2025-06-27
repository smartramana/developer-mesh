package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/api"
	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/core"

	"github.com/S-Corkum/devops-mcp/pkg/common/aws"
	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/common/config"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/interfaces"
	"github.com/S-Corkum/devops-mcp/pkg/metrics"
	"github.com/S-Corkum/devops-mcp/pkg/observability"

	// Import PostgreSQL driver
	_ "github.com/lib/pq"
)

func main() {
	// Initialize secure random seed
	initSecureRandom()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle health check flag
	if len(os.Args) > 1 && os.Args[1] == "-health-check" {
		// Simple health check for Docker HEALTHCHECK
		os.Exit(0)
	}

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

	// Initialize connection helper
	connHelper := api.NewConnectionHelper(logger)

	// Wait for dependencies if in container environment
	if os.Getenv("ENVIRONMENT") == "docker" {
		deps := []string{"database", "redis"}
		if err := connHelper.WaitForDependencies(ctx, deps); err != nil {
			logger.Warn("Failed to wait for dependencies", map[string]any{
				"error": err.Error(),
			})
		}
	}

	// Initialize metrics
	metricsClient := metrics.NewClient(cfg.Metrics)
	defer func() {
		if err := metricsClient.Close(); err != nil {
			logger.Warn("Failed to close metrics client", map[string]any{"error": err})
		}
	}()

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

	// Initialize database with retry logic
	db, err = connHelper.ConnectToDatabase(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
	}()

	// Prepare cache config with AWS integration if needed
	var cacheClient cache.Cache
	// Initialize cache using the cache config from the configuration
	cacheConfig := cfg.Cache

	// Initialize cache with retry logic and graceful degradation
	cacheClient, err = connHelper.ConnectToCache(ctx, cacheConfig)
	if err != nil {
		logger.Warn("Cache initialization failed, running without cache", map[string]any{
			"error": err.Error(),
		})
		// Create a no-op cache for graceful degradation
		cacheClient = cache.NewNoOpCache()
	}
	if cacheClient != nil {
		defer func() {
			if err := cacheClient.Close(); err != nil {
				logger.Warn("Failed to close cache client", map[string]any{"error": err})
			}
		}()
	}

	// Initialize core engine with logger
	engine := core.NewEngine(logger)
	defer func() {
		if err := engine.Shutdown(ctx); err != nil {
			logger.Error("Failed to shutdown engine", map[string]any{
				"error": err.Error(),
			})
		}
	}()

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
		RateLimit: parseRateLimitConfig(cfg.API.RateLimit),
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

// initSecureRandom is kept for backward compatibility but is no longer needed
// as of Go 1.20, the global random number generator is automatically seeded
func initSecureRandom() {
	// Go 1.20+ automatically seeds the global random number generator
	// No manual seeding is required
	log.Println("Random number generator is automatically seeded (Go 1.20+)")
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

// parseRateLimitConfig parses rate limit configuration from either a map or integer
func parseRateLimitConfig(input any) api.RateLimitConfig {
	config := api.RateLimitConfig{
		Enabled:     false,
		Limit:       100,
		Period:      time.Minute,
		BurstFactor: 3,
	}

	if input == nil {
		return config
	}

	// Handle backward compatibility - rate_limit as integer
	switch v := input.(type) {
	case int:
		config.Enabled = true
		config.Limit = v
		return config
	case float64: // JSON numbers are parsed as float64
		config.Enabled = true
		config.Limit = int(v)
		return config
	case map[string]any:
		// Handle structured rate_limit configuration
		if enabled, ok := v["enabled"].(bool); ok {
			config.Enabled = enabled
		}
		if limit, ok := v["limit"].(int); ok {
			config.Limit = limit
		} else if limit, ok := v["limit"].(float64); ok {
			config.Limit = int(limit)
		}
		if period, ok := v["period"].(string); ok {
			if d, err := time.ParseDuration(period); err == nil {
				config.Period = d
			}
		}
		if burstFactor, ok := v["burst_factor"].(int); ok {
			config.BurstFactor = burstFactor
		} else if burstFactor, ok := v["burst_factor"].(float64); ok {
			config.BurstFactor = int(burstFactor)
		}
		return config
	default:
		// Log warning and return default config
		fmt.Printf("Warning: unexpected type for rate_limit configuration: %T\n", input)
		return config
	}
}
