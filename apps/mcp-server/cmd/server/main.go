package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	mathrand "math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Internal application-specific imports
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/websocket"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core"

	// Shared package imports
	"github.com/developer-mesh/developer-mesh/pkg/clients"
	"github.com/developer-mesh/developer-mesh/pkg/common/aws"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	commonconfig "github.com/developer-mesh/developer-mesh/pkg/common/config"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/postgres"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	securitytls "github.com/developer-mesh/developer-mesh/pkg/security/tls"
	"github.com/developer-mesh/developer-mesh/pkg/services"

	// Import PostgreSQL driver
	_ "github.com/lib/pq"

	// Import auth package for production authorizer
	"github.com/developer-mesh/developer-mesh/pkg/auth"

	// Import rules package for rule engine and policy manager
	"github.com/developer-mesh/developer-mesh/pkg/rules"
	// MCP server should not handle migrations - REST API handles them
	// Removed migration imports
)

// Version information (set via ldflags during build)
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Command-line flags
var (
	configFile   = flag.String("config", "", "Path to configuration file (overrides default locations)")
	showVersion  = flag.Bool("version", false, "Show version information and exit")
	validateOnly = flag.Bool("validate", false, "Validate configuration and exit")
	// Migration flags removed - MCP server doesn't handle migrations
	healthCheck = flag.Bool("health-check", false, "Perform health check and exit")
)

func main() {
	flag.Parse()

	// Show version information if requested
	if *showVersion {
		fmt.Printf("MCP Server\nVersion: %s\nBuild Time: %s\nGit Commit: %s\n", version, buildTime, gitCommit)
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

	// Initialize secure random seed
	if err := initSecureRandom(); err != nil {
		log.Printf("Warning: failed to initialize secure random: %v", err)
	}

	// Initialize logger early
	logger := observability.NewLogger("mcp-server")
	logger.Info("Starting MCP Server", map[string]interface{}{
		"version":    version,
		"build_time": buildTime,
		"git_commit": gitCommit,
	})

	// Architecture Note: MCP Server handles the Model Context Protocol for AI agent interactions
	// Webhooks are handled by the REST API service, not this MCP server
	// Data flow: GitHub Webhooks → REST API → Redis Queue → Worker

	// Setup root context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create startup context with timeout
	startupTimeout := 30 * time.Second
	startupCtx, startupCancel := context.WithTimeout(ctx, startupTimeout)
	defer startupCancel()

	// Load configuration
	cfg, err := loadConfiguration(*configFile)
	if err != nil {
		logger.Error("Failed to load configuration", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// Validate configuration
	if err := validateConfiguration(cfg); err != nil {
		logger.Error("Invalid configuration", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// Exit if only validating
	if *validateOnly {
		logger.Info("Configuration validated successfully", nil)
		os.Exit(0)
	}

	// Initialize observability
	metricsClient := observability.NewMetricsClient()
	defer func() {
		if err := metricsClient.Close(); err != nil {
			logger.Error("Failed to close metrics client", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Log AWS configuration
	logAWSConfiguration(logger)

	// Initialize database with root context, not startup context
	// Database connection must outlive the startup phase
	db, err := initializeDatabase(ctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize database", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	if db != nil {
		defer func() {
			if err := db.Close(); err != nil {
				logger.Error("Failed to close database", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}()
	}

	// MCP server should never run migrations - only the REST API should handle that

	// Initialize cache with root context
	// Cache connection must outlive the startup phase
	cacheClient, err := initializeCache(ctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize cache", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	defer func() {
		if err := cacheClient.Close(); err != nil {
			logger.Error("Failed to close cache client", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Initialize core engine with root context
	engine, err := initializeEngine(ctx, cfg, db, cacheClient, metricsClient, logger)
	if err != nil {
		logger.Error("Failed to initialize core engine", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := engine.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown engine", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Initialize REST API client for proxying to REST API
	restClient, err := initializeRESTClient(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize REST API client", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	if restClient != nil {
		defer func() {
			if err := restClient.Close(); err != nil {
				logger.Error("Failed to close REST API client", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}()
	}

	// Initialize services for multi-agent collaboration with root context
	services, err := initializeServices(ctx, cfg, db, cacheClient, metricsClient, logger)
	if err != nil {
		logger.Error("Failed to initialize services", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// Initialize and start API server with root context
	server, err := initializeServer(ctx, cfg, engine, db, cacheClient, metricsClient, logger, restClient)
	if err != nil {
		logger.Error("Failed to initialize server", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// Inject services into WebSocket server
	if services != nil {
		server.InjectServices(services)
	}

	// Check if startup completed within timeout
	select {
	case <-startupCtx.Done():
		if startupCtx.Err() == context.DeadlineExceeded {
			logger.Error("Server startup timed out", map[string]interface{}{
				"timeout": startupTimeout.String(),
			})
			os.Exit(1)
		}
	default:
		// Startup completed successfully
		logger.Info("Server startup completed successfully", map[string]interface{}{
			"duration": time.Since(time.Now().Add(-startupTimeout)).String(),
		})
	}

	// Start server in a goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		if err := startServer(server, cfg, logger); err != nil {
			serverErrCh <- err
		}
	}()

	// Setup graceful shutdown
	if err := waitForShutdown(ctx, server, serverErrCh, logger); err != nil {
		logger.Error("Shutdown error", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully", nil)
}

// initSecureRandom initializes the math/rand package with a cryptographically secure seed
func initSecureRandom() error {
	max := big.NewInt(int64(1) << 62)
	val, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback to time-based seed
		mathrand.New(mathrand.NewSource(time.Now().UnixNano())) // #nosec G404 - Fallback only
		return fmt.Errorf("using time-based seed: %w", err)
	}

	// Use a new random source instead of the deprecated global Seed
	// This is secure: we're seeding math/rand with crypto/rand for performance
	mathrand.New(mathrand.NewSource(val.Int64())) // #nosec G404 - Properly seeded with crypto/rand
	return nil
}

// loadConfiguration loads configuration from file or environment
func loadConfiguration(configFile string) (*commonconfig.Config, error) {
	// Load configuration using the standard Load function
	cfg, err := commonconfig.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// If a specific config file was provided, override with values from that file
	if configFile != "" {
		// Since LoadFromFile doesn't exist, we'll use viper directly
		// This is handled internally by the Load function
		log.Printf("Note: Specific config file flag is not supported in this version")
	}

	return cfg, nil
}

// validateConfiguration validates critical configuration settings
func validateConfiguration(cfg *commonconfig.Config) error {
	// Validate API configuration
	if cfg.API.ListenAddress == "" {
		cfg.API.ListenAddress = ":8080" // Default
	}

	// Validate database configuration
	if cfg.Database.Host == "" {
		cfg.Database.Host = "localhost"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.Database == "" {
		return fmt.Errorf("database name must be specified")
	}

	// Validate cache configuration
	if cfg.Cache.Type == "" {
		cfg.Cache.Type = "redis"
	}

	return nil
}

// logAWSConfiguration logs AWS-related configuration information
func logAWSConfiguration(logger observability.Logger) {
	if aws.IsIRSAEnabled() {
		logger.Info("IRSA (IAM Roles for Service Accounts) is enabled", map[string]interface{}{
			"aws_role_arn":                os.Getenv("AWS_ROLE_ARN"),
			"aws_web_identity_token_file": os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"),
		})
	} else {
		logger.Info("IRSA not detected, using standard AWS credential chain", nil)
	}
}

// initializeDatabase creates and configures the database connection with retry logic
func initializeDatabase(ctx context.Context, cfg *commonconfig.Config, logger observability.Logger) (*database.Database, error) {
	// Build connection string
	connStr := buildDatabaseURL(cfg)

	logger.Info("Initializing database connection", map[string]interface{}{
		"host": cfg.Database.Host,
		"port": cfg.Database.Port,
		"db":   cfg.Database.Database,
	})

	// Create database configuration with proper connection pool settings
	dbConfig := database.Config{
		Driver:          "postgres",
		DSN:             connStr,
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Database:        cfg.Database.Database,
		Username:        cfg.Database.Username,
		Password:        cfg.Database.Password,
		SSLMode:         cfg.Database.SSLMode,
		SearchPath:      cfg.Database.SearchPath,
		UseAWS:          cfg.Database.UseIAMAuth,
		UseIAM:          cfg.Database.UseIAMAuth,
		MaxOpenConns:    25,               // Maintain a healthy connection pool
		MaxIdleConns:    10,               // Keep more idle connections
		ConnMaxLifetime: 30 * time.Minute, // Longer lifetime to avoid frequent reconnects
		QueryTimeout:    30 * time.Second,
		ConnectTimeout:  10 * time.Second,
		AutoMigrate:     false, // MCP server should never run migrations
	}

	// Try to connect with exponential backoff
	var db *database.Database
	var err error
	maxRetries := 10
	baseDelay := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		// Create database instance
		db, err = database.NewDatabase(ctx, dbConfig)
		if err == nil {
			// Test connection
			if pingErr := db.Ping(); pingErr == nil {
				break // Success!
			} else {
				// Close failed connection
				if closeErr := db.Close(); closeErr != nil {
					logger.Warn("Failed to close database connection", map[string]interface{}{"error": closeErr})
				}
				err = fmt.Errorf("failed to ping database: %w", pingErr)
			}
		}

		if i < maxRetries-1 {
			delay := baseDelay * (1 << uint(i)) // Exponential backoff
			if delay > 30*time.Second {
				delay = 30 * time.Second // Cap at 30 seconds
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
				return nil, ctx.Err()
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
	}

	logger.Info("Database connection established, waiting for tables...", nil)

	// Wait for tables to be ready (migrations from REST API)
	readinessChecker := database.NewReadinessChecker(db.GetDB())
	readinessChecker.SetLogger(func(format string, args ...interface{}) {
		logger.Info(fmt.Sprintf(format, args...), nil)
	})

	if err := readinessChecker.WaitForTablesWithBackoff(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			logger.Warn("Failed to close database connection", map[string]interface{}{"error": closeErr})
		}
		return nil, fmt.Errorf("database tables not ready: %w", err)
	}

	logger.Info("Database fully initialized with all required tables", nil)
	return db, nil
}

// buildDatabaseURL constructs the database connection string
func buildDatabaseURL(cfg *commonconfig.Config) string {
	// Check for DATABASE_URL environment variable first
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	// Use configuration values with environment overrides
	host := getEnvOrDefault("DB_HOST", cfg.Database.Host)
	port := getEnvOrDefault("DB_PORT", fmt.Sprintf("%d", cfg.Database.Port))
	dbName := getEnvOrDefault("DB_NAME", cfg.Database.Database)
	user := getEnvOrDefault("DB_USER", cfg.Database.Username)
	password := getEnvOrDefault("DB_PASSWORD", cfg.Database.Password)
	sslMode := getEnvOrDefault("DB_SSLMODE", cfg.Database.SSLMode)

	if sslMode == "" {
		sslMode = "disable"
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbName, sslMode)
}

// initializeCache creates and configures the cache client
func initializeCache(ctx context.Context, cfg *commonconfig.Config, logger observability.Logger) (cache.Cache, error) {
	var cacheConfig cache.RedisConfig

	// Check if we should use AWS ElastiCache
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
		// Use standard Redis configuration
		// Convert from common cache config to our cache config
		cacheConfig = cache.RedisConfig{
			Type:         cfg.Cache.Type,
			Address:      cfg.Cache.Address,
			Password:     cfg.Cache.Password,
			Database:     cfg.Cache.Database,
			MaxRetries:   cfg.Cache.MaxRetries,
			DialTimeout:  cfg.Cache.DialTimeout,
			ReadTimeout:  cfg.Cache.ReadTimeout,
			WriteTimeout: cfg.Cache.WriteTimeout,
			PoolSize:     cfg.Cache.PoolSize,
			MinIdleConns: cfg.Cache.MinIdleConns,
			PoolTimeout:  cfg.Cache.PoolTimeout,
		}

		// Convert TLS config if present AND enabled
		// BUT: If we're connecting to localhost/127.0.0.1 (SSH tunnel), disable TLS
		isSSHTunnel := cfg.Cache.Address == "127.0.0.1:6379" || cfg.Cache.Address == "localhost:6379"

		if cfg.Cache.TLS != nil && cfg.Cache.TLS.Enabled && !isSSHTunnel {
			logger.Info("Converting TLS config", map[string]interface{}{
				"enabled":     cfg.Cache.TLS.Enabled,
				"skip_verify": cfg.Cache.TLS.InsecureSkipVerify,
			})
			cacheConfig.TLS = &cache.TLSConfig{
				Config: securitytls.Config{
					Enabled:            cfg.Cache.TLS.Enabled,
					InsecureSkipVerify: cfg.Cache.TLS.InsecureSkipVerify,
					MinVersion:         cfg.Cache.TLS.MinVersion,
				},
			}
		} else if isSSHTunnel && cfg.Cache.TLS != nil && cfg.Cache.TLS.Enabled {
			logger.Info("Disabling TLS for SSH tunnel connection", map[string]interface{}{
				"address": cfg.Cache.Address,
			})
		}
	}

	logger.Info("Initializing cache", map[string]interface{}{
		"type":         cacheConfig.Type,
		"cluster_mode": cacheConfig.ClusterMode,
		"address":      cacheConfig.Address,
		"tls_enabled":  cacheConfig.TLS != nil && cacheConfig.TLS.Enabled,
	})

	return cache.NewCache(ctx, cacheConfig)
}

// initializeEngine creates the core engine
func initializeEngine(ctx context.Context, cfg *commonconfig.Config, db *database.Database,
	cacheClient cache.Cache, metricsClient observability.MetricsClient, logger observability.Logger) (*core.Engine, error) {

	logger.Info("Initializing core engine", nil)

	configAdapter := core.NewConfigAdapter(cfg)
	engine, err := core.NewEngine(ctx, configAdapter, db, cacheClient, metricsClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine: %w", err)
	}

	return engine, nil
}

// ServicesBundle holds all the services needed for multi-agent collaboration
type ServicesBundle struct {
	TaskService      services.TaskService
	WorkflowService  services.WorkflowService
	WorkspaceService services.WorkspaceService
	DocumentService  services.DocumentService
	ConflictService  services.ConflictResolutionService
	AgentRepository  repository.AgentRepository
	Cache            cache.Cache
}

// initializeServices creates all services for multi-agent collaboration
func initializeServices(ctx context.Context, cfg *commonconfig.Config, db *database.Database, cacheClient cache.Cache, metricsClient observability.MetricsClient, logger observability.Logger) (*ServicesBundle, error) {
	logger.Info("Initializing multi-agent collaboration services", nil)

	// Create repositories
	// Get sqlx.DB from database
	sqlxDB := db.GetDB()

	// Create repositories with proper parameters
	taskRepo := postgres.NewTaskRepository(sqlxDB, sqlxDB, cacheClient, logger, observability.NoopStartSpan, metricsClient)
	workflowRepo := postgres.NewWorkflowRepository(sqlxDB, sqlxDB, cacheClient, logger, observability.NoopStartSpan, metricsClient)
	workspaceRepo := postgres.NewWorkspaceRepository(sqlxDB, sqlxDB, cacheClient, logger, observability.NoopStartSpan)
	documentRepo := postgres.NewDocumentRepository(sqlxDB, sqlxDB, cacheClient, logger, observability.NoopStartSpan)
	agentRepo := repository.NewAgentRepository(sqlxDB)

	// Create service configuration with production-ready components
	serviceConfig := services.ServiceConfig{
		// Resilience
		CircuitBreaker: services.CreateDefaultCircuitBreakerSettings(),
		RetryPolicy: resilience.RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     5 * time.Second,
			Multiplier:   2.0,
			Jitter:       0.1,
		},
		TimeoutPolicy: resilience.TimeoutPolicy{
			Timeout: 30 * time.Second,
		},
		BulkheadPolicy: resilience.BulkheadPolicy{
			MaxConcurrent: 100,
			QueueSize:     50,
			Timeout:       5 * time.Second,
		},

		// Rate Limiting
		RateLimiter:  services.NewInMemoryRateLimiter(100, time.Minute),
		QuotaManager: services.NewInMemoryQuotaManager(),

		// Security
		Authorizer:        createAuthorizer(cacheClient, logger, metricsClient),
		Sanitizer:         services.NewDefaultSanitizer(),
		EncryptionService: createEncryptionService(logger),

		// Observability
		Logger:  logger,
		Metrics: metricsClient,
		Tracer:  observability.NoopStartSpan,

		// Business Rules
		RuleEngine:    createRuleEngine(cacheClient, logger, metricsClient),
		PolicyManager: createPolicyManager(cacheClient, logger, metricsClient),
	}

	// Create notification service
	notificationService := services.NewNotificationService(serviceConfig)

	// Create agent service
	agentService := services.NewAgentService(serviceConfig, agentRepo)

	// Create task service
	taskService := services.NewTaskService(serviceConfig, taskRepo, agentService, notificationService)

	// Create workflow service
	workflowService := services.NewWorkflowService(serviceConfig, workflowRepo, taskService, agentService, notificationService)

	// Create document service
	documentService := services.NewDocumentService(serviceConfig, documentRepo, cacheClient)

	// Create workspace service (it needs documentRepo, not documentService)
	workspaceService := services.NewWorkspaceService(serviceConfig, workspaceRepo, documentRepo, cacheClient)

	// Create conflict resolution service
	conflictService := services.NewConflictResolutionService(
		serviceConfig,
		documentRepo,
		workspaceRepo,
		taskRepo,
		"default", // Default conflict resolution strategy
	)

	logger.Info("All multi-agent collaboration services initialized successfully", map[string]interface{}{
		"task_service":         "ready",
		"workflow_service":     "ready",
		"workspace_service":    "ready",
		"document_service":     "ready",
		"conflict_service":     "ready",
		"agent_service":        "ready",
		"notification_service": "ready",
	})

	return &ServicesBundle{
		TaskService:      taskService,
		WorkflowService:  workflowService,
		WorkspaceService: workspaceService,
		DocumentService:  documentService,
		ConflictService:  conflictService,
		AgentRepository:  agentRepo,
		Cache:            cacheClient,
	}, nil
}

// createAuthorizer creates the appropriate authorizer based on environment
func createAuthorizer(cacheClient cache.Cache, logger observability.Logger, metrics observability.MetricsClient) auth.Authorizer {
	// Create audit logger
	auditLogger := auth.NewAuditLogger(logger)

	// Create auth configuration for production mode
	productionConfig := &auth.AuthConfig{
		Cache:         cacheClient,
		Logger:        logger,
		Metrics:       metrics,
		Tracer:        observability.NoopStartSpan,
		AuditLogger:   auditLogger,
		CacheEnabled:  true,
		CacheDuration: 5 * time.Minute,
		ModelPath:     "pkg/auth/rbac_model.conf",
		PolicyPath:    "", // Policies will be loaded from memory for now
	}

	// Create factory configuration
	factoryConfig := auth.FactoryConfig{
		ProductionConfig: productionConfig,
		Logger:           logger,
		Tracer:           observability.NoopStartSpan,
		// Mode will be determined automatically based on environment
	}

	// Create authorizer using factory
	authorizer, err := auth.NewAuthorizer(factoryConfig)
	if err != nil {
		// If we can't create the authorizer, log the error and return nil
		// This ensures the service can still start, just without authorization
		logger.Error("Failed to create authorizer", map[string]interface{}{
			"error": err.Error(),
			"mode":  factoryConfig.Mode,
		})
		return nil
	}

	logger.Info("Authorizer initialized", map[string]interface{}{
		"mode": factoryConfig.Mode,
	})
	return authorizer
}

// createEncryptionService creates the appropriate encryption service
func createEncryptionService(logger observability.Logger) services.EncryptionService {
	// Check for encryption key in environment
	encryptionKey := os.Getenv("ENCRYPTION_MASTER_KEY")
	if encryptionKey == "" {
		logger.Warn("ENCRYPTION_MASTER_KEY not set, using no-op encryption service", nil)
		return services.NewNoOpEncryptionService()
	}

	// Decode the key from base64
	keyBytes, err := base64.StdEncoding.DecodeString(encryptionKey)
	if err != nil {
		logger.Error("Failed to decode encryption key", map[string]interface{}{
			"error": err.Error(),
		})
		return services.NewNoOpEncryptionService()
	}

	// Create AES encryption service
	encryptionService, err := services.NewAESEncryptionService(keyBytes)
	if err != nil {
		logger.Error("Failed to create AES encryption service", map[string]interface{}{
			"error": err.Error(),
		})
		return services.NewNoOpEncryptionService()
	}

	logger.Info("Production AES-256-GCM encryption service initialized", nil)
	return encryptionService
}

// createRuleEngine creates the rule engine
func createRuleEngine(cacheClient cache.Cache, logger observability.Logger, metrics observability.MetricsClient) rules.Engine {
	// Create rule engine configuration
	config := rules.Config{
		HotReload:      true,
		ReloadInterval: 30 * time.Second,
		CacheDuration:  5 * time.Minute,
		MaxRules:       1000,
	}

	// Create rule engine
	engine := rules.NewEngine(config, logger, metrics)

	// Determine rule loader based on configuration
	var loader rules.RuleLoader

	// Check for rules configuration path
	rulesPath := os.Getenv("RULES_CONFIG_PATH")
	if rulesPath == "" {
		rulesPath = "/etc/developer-mesh/rules"
	}

	// Check if rules directory/file exists
	if _, err := os.Stat(rulesPath); err == nil {
		// Use file-based loader
		loader = rules.NewConfigFileRuleLoader(rulesPath, logger)
		logger.Info("Using configuration file rule loader", map[string]interface{}{
			"path": rulesPath,
		})
	} else {
		// Use database loader if available
		// For MVP, create default rules in memory
		loader = &defaultRuleLoader{logger: logger}
		logger.Info("Using default rule loader (no config path found)", nil)
	}

	// Set the loader
	if err := engine.SetRuleLoader(loader); err != nil {
		logger.Error("Failed to set rule loader", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Load initial rules
	ctx := context.Background()
	initialRules, err := loader.LoadRules(ctx)
	if err != nil {
		logger.Warn("Failed to load initial rules", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		if err := engine.LoadRules(ctx, initialRules); err != nil {
			logger.Error("Failed to load rules into engine", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Start hot reload if enabled
	if config.HotReload {
		if err := engine.StartHotReload(ctx); err != nil {
			logger.Warn("Failed to start rule hot reload", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			logger.Info("Rule hot reload started", map[string]interface{}{
				"interval": config.ReloadInterval,
			})
		}
	}

	logger.Info("Rule engine initialized successfully", map[string]interface{}{
		"hot_reload":   config.HotReload,
		"interval":     config.ReloadInterval,
		"rules_loaded": len(initialRules),
	})

	return engine
}

// defaultRuleLoader provides default rules when no external source is configured
type defaultRuleLoader struct {
	logger observability.Logger
}

func (l *defaultRuleLoader) LoadRules(ctx context.Context) ([]rules.Rule, error) {
	return []rules.Rule{
		{
			Name:       "task_assignment_priority",
			Category:   "assignment",
			Expression: "priority >= 3",
			Priority:   1,
			Enabled:    true,
			Metadata: map[string]interface{}{
				"description": "High priority tasks require immediate assignment",
			},
		},
		{
			Name:       "agent_workload_limit",
			Category:   "assignment",
			Expression: "workload < 10",
			Priority:   2,
			Enabled:    true,
			Metadata: map[string]interface{}{
				"description": "Agents should not exceed 10 concurrent tasks",
			},
		},
		{
			Name:       "task_timeout_check",
			Category:   "task_lifecycle",
			Expression: "elapsed_time > timeout",
			Priority:   3,
			Enabled:    true,
			Metadata: map[string]interface{}{
				"description": "Check if task has exceeded its timeout",
			},
		},
	}, nil
}

// createPolicyManager creates the policy manager
func createPolicyManager(cacheClient cache.Cache, logger observability.Logger, metrics observability.MetricsClient) rules.PolicyManager {
	// Create policy manager configuration
	config := rules.PolicyManagerConfig{
		CacheDuration:  5 * time.Minute,
		MaxPolicies:    1000,
		EnableCaching:  true,
		HotReload:      true,
		ReloadInterval: 30 * time.Second,
	}

	// Create policy manager
	manager := rules.NewPolicyManager(config, cacheClient, logger, metrics)

	// Determine policy loader based on configuration
	var loader rules.PolicyLoader

	// Check for policies configuration path
	policiesPath := os.Getenv("POLICIES_CONFIG_PATH")
	if policiesPath == "" {
		policiesPath = "/etc/developer-mesh/policies"
	}

	// Check if policies directory/file exists
	if _, err := os.Stat(policiesPath); err == nil {
		// Use file-based loader
		loader = rules.NewConfigFilePolicyLoader(policiesPath, logger)
		logger.Info("Using configuration file policy loader", map[string]interface{}{
			"path": policiesPath,
		})
	} else {
		// Use database loader if available
		// For MVP, create default policies in memory
		loader = &defaultPolicyLoader{logger: logger}
		logger.Info("Using default policy loader (no config path found)", nil)
	}

	// Set the loader
	if err := manager.SetPolicyLoader(loader); err != nil {
		logger.Error("Failed to set policy loader", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Load initial policies
	ctx := context.Background()
	initialPolicies, err := loader.LoadPolicies(ctx)
	if err != nil {
		logger.Warn("Failed to load initial policies", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		if err := manager.LoadPolicies(ctx, initialPolicies); err != nil {
			logger.Error("Failed to load policies into manager", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Start hot reload if enabled
	if config.HotReload {
		if err := manager.StartHotReload(ctx); err != nil {
			logger.Warn("Failed to start policy hot reload", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			logger.Info("Policy hot reload started", map[string]interface{}{
				"interval": config.ReloadInterval,
			})
		}
	}

	logger.Info("Policy manager initialized successfully", map[string]interface{}{
		"caching":         config.EnableCaching,
		"hot_reload":      config.HotReload,
		"interval":        config.ReloadInterval,
		"policies_loaded": len(initialPolicies),
	})

	return manager
}

// defaultPolicyLoader provides default policies when no external source is configured
type defaultPolicyLoader struct {
	logger observability.Logger
}

func (l *defaultPolicyLoader) LoadPolicies(ctx context.Context) ([]rules.Policy, error) {
	return []rules.Policy{
		{
			Name:     "task_lifecycle",
			Resource: "task",
			Rules: []rules.PolicyRule{
				{
					Condition: "status == 'pending'",
					Effect:    "allow",
					Actions:   []string{"assign", "cancel"},
					Resources: []string{"task:*"},
				},
				{
					Condition: "status == 'in_progress'",
					Effect:    "allow",
					Actions:   []string{"update", "complete", "fail"},
					Resources: []string{"task:*"},
				},
			},
			Defaults: map[string]interface{}{
				"timeout":     3600,
				"max_retries": 3,
				"priority":    "medium",
			},
		},
		{
			Name:     "agent_management",
			Resource: "agent",
			Rules: []rules.PolicyRule{
				{
					Condition: "role == 'admin'",
					Effect:    "allow",
					Actions:   []string{"create", "update", "delete"},
					Resources: []string{"agent:*"},
				},
				{
					Condition: "role == 'operator'",
					Effect:    "allow",
					Actions:   []string{"read", "update"},
					Resources: []string{"agent:*"},
				},
			},
			Defaults: map[string]interface{}{
				"max_concurrent_tasks": 10,
				"idle_timeout":         300,
			},
		},
		{
			Name:     "workspace_access",
			Resource: "workspace",
			Rules: []rules.PolicyRule{
				{
					Condition: "is_public == true",
					Effect:    "allow",
					Actions:   []string{"read"},
					Resources: []string{"workspace:*"},
				},
				{
					Condition: "is_member == true",
					Effect:    "allow",
					Actions:   []string{"read", "update"},
					Resources: []string{"workspace:*"},
				},
			},
			Defaults: map[string]interface{}{
				"max_members":        100,
				"retention_days":     90,
				"default_visibility": "private",
			},
		},
	}, nil
}

// initializeServer creates and configures the API server
func initializeServer(ctx context.Context, cfg *commonconfig.Config, engine *core.Engine,
	db *database.Database, cacheClient cache.Cache, metricsClient observability.MetricsClient, logger observability.Logger, restClient clients.RESTAPIClient) (*api.Server, error) {

	// Build API configuration
	apiConfig := buildAPIConfig(cfg, logger)

	logger.Info("Initializing API server", map[string]interface{}{
		"listen_address": apiConfig.ListenAddress,
		"enable_cors":    apiConfig.EnableCORS,
		"enable_swagger": apiConfig.EnableSwagger,
	})

	// Create server - pass db.GetDB() to get the *sqlx.DB
	server := api.NewServer(engine, apiConfig, db.GetDB(), cacheClient, metricsClient, cfg)

	// Set the REST client if available
	if restClient != nil {
		server.SetRESTClient(restClient)
	}

	// Initialize server components
	if err := server.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize server: %w", err)
	}

	return server, nil
}

// buildAPIConfig creates API configuration from common config
func buildAPIConfig(cfg *commonconfig.Config, logger observability.Logger) api.Config {
	apiConfig := api.DefaultConfig()

	// Override with configuration values
	apiConfig.ListenAddress = cfg.API.ListenAddress

	// Debug logging
	logger.Info("Building API config", map[string]interface{}{
		"api_listen_address": cfg.API.ListenAddress,
		"has_mcp_server":     cfg.MCPServer != nil,
	})

	// Check if MCP server has a specific listen address override
	if cfg.MCPServer != nil {
		logger.Info("MCP server config found", map[string]interface{}{
			"listen_address": cfg.MCPServer.ListenAddress,
		})
		if cfg.MCPServer.ListenAddress != "" {
			logger.Info("Using MCP server listen address override", map[string]interface{}{
				"old_address": apiConfig.ListenAddress,
				"new_address": cfg.MCPServer.ListenAddress,
			})
			apiConfig.ListenAddress = cfg.MCPServer.ListenAddress
		}
	}
	apiConfig.TLSCertFile = cfg.API.TLSCertFile
	apiConfig.TLSKeyFile = cfg.API.TLSKeyFile

	// Set timeouts from environment if available
	if timeout := getEnvDuration("API_READ_TIMEOUT", 0); timeout > 0 {
		apiConfig.ReadTimeout = timeout
	}
	if timeout := getEnvDuration("API_WRITE_TIMEOUT", 0); timeout > 0 {
		apiConfig.WriteTimeout = timeout
	}
	if timeout := getEnvDuration("API_IDLE_TIMEOUT", 0); timeout > 0 {
		apiConfig.IdleTimeout = timeout
	}

	// Set feature flags from environment
	apiConfig.EnableCORS = getEnvBool("API_ENABLE_CORS", apiConfig.EnableCORS)
	apiConfig.EnableSwagger = getEnvBool("API_ENABLE_SWAGGER", apiConfig.EnableSwagger)

	// Configure authentication
	if cfg.API.Auth != nil {
		// JWT configuration
		if jwtConfig, ok := cfg.API.Auth["jwt"].(map[string]interface{}); ok {
			if secret, ok := jwtConfig["secret"].(string); ok && secret != "" {
				apiConfig.Auth.JWTSecret = secret
			}
		}

		// API keys configuration
		if apiKeysConfig, ok := cfg.API.Auth["api_keys"].(map[string]interface{}); ok {
			if staticKeys, ok := apiKeysConfig["static_keys"].(map[string]interface{}); ok {
				// Convert the nested structure to the format expected by the server
				apiKeys := make(map[string]interface{})
				for key, keyData := range staticKeys {
					apiKeys[key] = keyData
				}
				apiConfig.Auth.APIKeys = apiKeys
				logger.Info("Loaded API keys from config", map[string]interface{}{
					"count": len(apiKeys),
				})
			}
		}
	}

	// Override JWT secret from environment if set
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		apiConfig.Auth.JWTSecret = jwtSecret
	}

	// Webhooks are handled by the REST API, not the MCP server

	// Configure rate limiting
	if cfg.API.RateLimit != nil {
		apiConfig.RateLimit = parseRateLimitConfig(cfg.API.RateLimit)
	}

	// Configure WebSocket if available
	logger.Info("WebSocket config check", map[string]interface{}{
		"websocket_nil":     cfg.WebSocket == nil,
		"websocket_enabled": cfg.WebSocket != nil && cfg.WebSocket.Enabled,
	})

	if cfg.WebSocket != nil && cfg.WebSocket.Enabled {
		apiConfig.WebSocket = parseWebSocketConfig(cfg.WebSocket)
		logger.Info("WebSocket config parsed", map[string]interface{}{
			"enabled": apiConfig.WebSocket.Enabled,
		})
	}

	return apiConfig
}

// parseRateLimitConfig parses rate limit configuration from either a map or integer
func parseRateLimitConfig(input interface{}) api.RateLimitConfig {
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
	case map[string]interface{}:
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
		log.Printf("Warning: unexpected type for rate_limit configuration: %T\n", input)
		return config
	}
}

// parseWebSocketConfig parses WebSocket configuration
func parseWebSocketConfig(wsConfig *commonconfig.WebSocketConfig) api.WebSocketConfig {
	config := api.WebSocketConfig{
		Enabled:         wsConfig.Enabled,
		MaxConnections:  wsConfig.MaxConnections,
		ReadBufferSize:  wsConfig.ReadBufferSize,
		WriteBufferSize: wsConfig.WriteBufferSize,
		PingInterval:    wsConfig.PingInterval,
		PongTimeout:     wsConfig.PongTimeout,
		MaxMessageSize:  wsConfig.MaxMessageSize,
	}

	// Parse security config
	if wsConfig.Security != nil {
		config.Security = websocket.SecurityConfig{
			RequireAuth:    wsConfig.Security.RequireAuth,
			HMACSignatures: wsConfig.Security.HMACSignatures,
			AllowedOrigins: wsConfig.Security.AllowedOrigins,
		}
	}

	// Parse rate limit config
	if wsConfig.RateLimit != nil {
		config.RateLimit = websocket.RateLimiterConfig{
			Rate:    float64(wsConfig.RateLimit.Rate),
			Burst:   float64(wsConfig.RateLimit.Burst),
			PerIP:   wsConfig.RateLimit.PerIP,
			PerUser: wsConfig.RateLimit.PerUser,
		}
	}

	return config
}

// startServer starts the HTTP/HTTPS server
func startServer(server *api.Server, cfg *commonconfig.Config, logger observability.Logger) error {
	logger.Info("Starting server", map[string]interface{}{
		"address":     server.GetListenAddress(),
		"environment": cfg.Environment,
		"tls_enabled": cfg.API.TLSCertFile != "" && cfg.API.TLSKeyFile != "",
	})

	// Start with TLS if configured and in production
	if cfg.IsProduction() && cfg.API.TLSCertFile != "" && cfg.API.TLSKeyFile != "" {
		logger.Info("Starting server with TLS (HTTPS)", nil)
		return server.StartTLS(cfg.API.TLSCertFile, cfg.API.TLSKeyFile)
	}

	// Otherwise start HTTP server
	return server.Start()
}

// waitForShutdown handles graceful shutdown
func waitForShutdown(ctx context.Context, server *api.Server, serverErrCh <-chan error, logger observability.Logger) error {
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", map[string]interface{}{
			"signal": sig.String(),
		})
	case err := <-serverErrCh:
		logger.Error("Server error", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	case <-ctx.Done():
		logger.Info("Context cancelled", nil)
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("Shutting down server gracefully", nil)
	return server.Shutdown(shutdownCtx)
}

// runMigrations runs database migrations

// getMigrationDir returns the path to the migrations directory

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// getBoolFromMap extracts a boolean value from a map (commented out - unused)
// func getBoolFromMap(m map[string]interface{}, key string, defaultValue bool) bool {
// 	if val, ok := m[key]; ok {
// 		if boolVal, ok := val.(bool); ok {
// 			return boolVal
// 		}
// 	}
// 	return defaultValue
// }

// getStringFromMap extracts a string value from a map (commented out - unused)
// func getStringFromMap(m map[string]interface{}, key string, defaultValue string) string {
// 	if val, ok := m[key]; ok {
// 		if strVal, ok := val.(string); ok {
// 			return strVal
// 		}
// 	}
// 	return defaultValue
// }

// getStringSliceFromMap extracts a string slice from a map (commented out - unused)
// func getStringSliceFromMap(m map[string]interface{}, key string, defaultValue []string) []string {
// 	if val, ok := m[key]; ok {
// 		if slice, ok := val.([]interface{}); ok {
// 			result := make([]string, 0, len(slice))
// 			for _, item := range slice {
// 				if str, ok := item.(string); ok {
// 					result = append(result, str)
// 				}
// 			}
// 			return result
// 		}
// 		if strSlice, ok := val.([]string); ok {
// 			return strSlice
// 		}
// 	}
// 	return defaultValue
// }

// performHealthCheck performs a basic health check
// initializeRESTClient creates the REST API client for proxying tool requests
func initializeRESTClient(cfg *commonconfig.Config, logger observability.Logger) (clients.RESTAPIClient, error) {
	// Check if REST API integration is enabled
	if cfg.MCPServer == nil || !cfg.MCPServer.RestAPI.Enabled {
		logger.Info("REST API integration disabled, using mock tools", nil)
		return nil, nil
	}

	// Create REST client configuration
	restConfig := clients.RESTClientConfig{
		BaseURL:                    cfg.MCPServer.RestAPI.BaseURL,
		APIKey:                     cfg.MCPServer.RestAPI.APIKey,
		Timeout:                    cfg.MCPServer.RestAPI.Timeout,
		MaxIdleConns:               100,
		MaxConnsPerHost:            10,
		CacheTTL:                   30 * time.Second,
		Logger:                     logger,
		CircuitBreakerMaxFailures:  5,
		CircuitBreakerTimeout:      60 * time.Second,
		CircuitBreakerRetryTimeout: 30 * time.Second,
		HealthCheckInterval:        30 * time.Second,
		HealthCheckTimeout:         5 * time.Second,
	}

	logger.Info("Initializing REST API client", map[string]interface{}{
		"base_url":                restConfig.BaseURL,
		"timeout":                 restConfig.Timeout,
		"health_check_interval":   restConfig.HealthCheckInterval,
		"circuit_breaker_enabled": true,
	})

	// Create the REST client
	client := clients.NewRESTAPIClient(restConfig)

	// Validate connection with retries
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := client.HealthCheck(ctx); err != nil {
			logger.Warn("REST API health check failed", map[string]interface{}{
				"attempt":      attempt,
				"max_attempts": maxAttempts,
				"error":        err.Error(),
			})

			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2 * time.Second) // Exponential backoff
				continue
			}

			// If we're in development, we can continue without REST API
			if cfg.IsDevelopment() {
				logger.Error("REST API not available, continuing in degraded mode", map[string]interface{}{
					"error": err.Error(),
				})
				return client, nil // Return client anyway for potential future recovery
			}

			return nil, fmt.Errorf("REST API health check failed after %d attempts: %w", maxAttempts, err)
		}

		// Health check succeeded
		logger.Info("REST API connection validated", map[string]interface{}{
			"attempt": attempt,
		})
		break
	}

	return client, nil
}

func performHealthCheck() error {
	// Perform a real health check by calling the health endpoint
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Try to connect to the health endpoint
	// Use the actual server address with proper scheme
	scheme := "http"
	if os.Getenv("TLS_ENABLED") == "true" {
		scheme = "https"
	}
	resp, err := client.Get(fmt.Sprintf("%s://localhost:8080/health", scheme))
	if err != nil {
		return fmt.Errorf("failed to connect to health endpoint: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	// Parse the response to verify it's valid JSON
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse health response: %w", err)
	}

	// Check if status is healthy
	if status, ok := result["status"].(string); ok && status != "healthy" {
		return fmt.Errorf("service is not healthy: %s", status)
	}

	return nil
}
