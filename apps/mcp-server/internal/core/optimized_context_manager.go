package core

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"mcp-server/internal/config"
	"mcp-server/internal/core/cache"
	"mcp-server/internal/core/resilience"
)

// OptimizedContextManager provides high-performance context management with caching and read replicas
type OptimizedContextManager struct {
	// Primary database for writes
	primaryDB *sql.DB
	
	// Read replicas for load distribution
	readReplicas []*sql.DB
	replicaMutex sync.RWMutex
	
	// Multi-level cache
	cache *cache.MultiLevelCache
	
	// Circuit breakers for resilience
	circuitBreakers *resilience.CircuitBreakerGroup
	
	// Configuration
	config *config.ContextManagerConfig
	
	// Dependencies
	logger  observability.Logger
	metrics observability.MetricsClient
	
	// Performance tracking
	slowQueryLog chan slowQuery
}

type slowQuery struct {
	query    string
	duration time.Duration
	params   []interface{}
}

// NewOptimizedContextManager creates a high-performance context manager
func NewOptimizedContextManager(
	cfg *config.ContextManagerConfig,
	primaryDB *sql.DB,
	redisClient *redis.Client,
	logger observability.Logger,
	metrics observability.MetricsClient,
) (*OptimizedContextManager, error) {
	// Configure primary database connection pool
	primaryDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	primaryDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	primaryDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	primaryDB.SetConnMaxIdleTime(cfg.Database.ConnMaxIdleTime)

	// Initialize read replicas
	readReplicas := make([]*sql.DB, 0, len(cfg.ReadReplicas))
	for _, replicaCfg := range cfg.ReadReplicas {
		replicaDB, err := sql.Open("postgres", replicaCfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to read replica: %w", err)
		}
		
		// Configure replica connection pool
		replicaDB.SetMaxOpenConns(replicaCfg.MaxOpenConns)
		replicaDB.SetMaxIdleConns(replicaCfg.MaxIdleConns)
		replicaDB.SetConnMaxLifetime(replicaCfg.ConnMaxLifetime)
		replicaDB.SetConnMaxIdleTime(replicaCfg.ConnMaxIdleTime)
		
		readReplicas = append(readReplicas, replicaDB)
	}

	// Initialize multi-level cache
	mlCache, err := cache.NewMultiLevelCache(&cfg.Cache, redisClient, logger, metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to create multi-level cache: %w", err)
	}

	// Initialize circuit breakers
	circuitBreakers := resilience.NewCircuitBreakerGroup(&cfg.CircuitBreaker, logger, metrics)

	manager := &OptimizedContextManager{
		primaryDB:       primaryDB,
		readReplicas:    readReplicas,
		cache:           mlCache,
		circuitBreakers: circuitBreakers,
		config:          cfg,
		logger:          logger,
		metrics:         metrics,
		slowQueryLog:    make(chan slowQuery, 100),
	}

	// Start health checks for read replicas
	go manager.monitorReplicaHealth()

	// Start slow query logger
	go manager.logSlowQueries()

	// Warm cache if configured
	if cfg.Cache.Warming.Enabled {
		go manager.warmCache()
	}

	return manager, nil
}

// CreateContext creates a new context with write-through caching
func (ocm *OptimizedContextManager) CreateContext(ctx context.Context, contextData *models.Context) (*models.Context, error) {
	startTime := time.Now()
	defer func() {
		ocm.recordLatency(ctx, "create_context", time.Since(startTime))
	}()

	// Use circuit breaker for database operations
	var createdContext *models.Context
	err := ocm.circuitBreakers.GetBreaker("create").Execute(ctx, "create_context", func() error {
		// Perform database insert
		query := `
			INSERT INTO mcp.contexts (agent_id, session_id, parent_id, content, metadata, 
			                         truncation_strategy, retention_policy, tags, relationships)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id, created_at, updated_at, version`

		err := ocm.primaryDB.QueryRowContext(
			ctx, query,
			contextData.AgentID, contextData.SessionID, contextData.ParentID,
			contextData.Content, contextData.Metadata, contextData.TruncationStrategy,
			contextData.RetentionPolicy, contextData.Tags, contextData.Relationships,
		).Scan(
			&contextData.ID, &contextData.CreatedAt, &contextData.UpdatedAt, &contextData.Version,
		)
		
		if err != nil {
			return fmt.Errorf("failed to create context: %w", err)
		}
		
		createdContext = contextData
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Write-through cache
	cacheKey := ocm.contextCacheKey(createdContext.ID)
	if err := ocm.cache.Set(ctx, cacheKey, createdContext); err != nil {
		ocm.logger.Warn("Failed to cache created context", "id", createdContext.ID, "error", err)
	}

	// Invalidate list caches
	ocm.invalidateListCaches(ctx, createdContext.AgentID, createdContext.SessionID)

	return createdContext, nil
}

// GetContext retrieves a context with cache-aside pattern and read replica support
func (ocm *OptimizedContextManager) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	startTime := time.Now()
	defer func() {
		ocm.recordLatency(ctx, "get_context", time.Since(startTime))
	}()

	cacheKey := ocm.contextCacheKey(contextID)

	// Try cache first
	cached, err := ocm.cache.Get(ctx, cacheKey)
	if err == nil && cached != nil {
		ocm.metrics.IncrementCounter(ctx, "context_cache_hit", 1)
		return cached, nil
	}

	// Cache miss - read from database with circuit breaker
	var contextData *models.Context
	err = ocm.circuitBreakers.GetBreaker("read").ExecuteWithFallback(
		ctx, "get_context",
		// Primary function - try read replica
		func() error {
			db := ocm.selectReadReplica()
			contextData, err = ocm.readContextFromDB(ctx, db, contextID)
			return err
		},
		// Fallback - use primary if replicas fail
		func() error {
			ocm.logger.Warn("Falling back to primary DB for read")
			contextData, err = ocm.readContextFromDB(ctx, ocm.primaryDB, contextID)
			return err
		},
	)

	if err != nil {
		return nil, err
	}

	// Cache the result
	if contextData != nil {
		if err := ocm.cache.Set(ctx, cacheKey, contextData); err != nil {
			ocm.logger.Warn("Failed to cache context", "id", contextID, "error", err)
		}
	}

	return contextData, nil
}

// UpdateContext updates a context with cache invalidation
func (ocm *OptimizedContextManager) UpdateContext(
	ctx context.Context,
	contextID string,
	updatedContext *models.Context,
	options *models.ContextUpdateOptions,
) (*models.Context, error) {
	startTime := time.Now()
	defer func() {
		ocm.recordLatency(ctx, "update_context", time.Since(startTime))
	}()

	var result *models.Context
	err := ocm.circuitBreakers.GetBreaker("update").Execute(ctx, "update_context", func() error {
		// Build dynamic update query based on options
		query, args := ocm.buildUpdateQuery(contextID, updatedContext, options)
		
		err := ocm.primaryDB.QueryRowContext(ctx, query, args...).Scan(
			&result.ID, &result.AgentID, &result.SessionID, &result.ParentID,
			&result.Content, &result.Metadata, &result.TruncationStrategy,
			&result.RetentionPolicy, &result.Tags, &result.Relationships,
			&result.CreatedAt, &result.UpdatedAt, &result.Version,
		)
		
		return err
	})

	if err != nil {
		return nil, err
	}

	// Invalidate cache
	cacheKey := ocm.contextCacheKey(contextID)
	if err := ocm.cache.Delete(ctx, cacheKey); err != nil {
		ocm.logger.Warn("Failed to invalidate cache", "id", contextID, "error", err)
	}

	// Invalidate list caches
	ocm.invalidateListCaches(ctx, result.AgentID, result.SessionID)

	return result, nil
}

// ListContexts retrieves contexts with caching and read replica support
func (ocm *OptimizedContextManager) ListContexts(
	ctx context.Context,
	agentID string,
	sessionID string,
	options map[string]interface{},
) ([]*models.Context, error) {
	startTime := time.Now()
	defer func() {
		ocm.recordLatency(ctx, "list_contexts", time.Since(startTime))
	}()

	// Build cache key from parameters
	cacheKey := ocm.listCacheKey(agentID, sessionID, options)

	// Build cache key from parameters
	_ = cacheKey // Note: List operations are not cached in this example to avoid complexity
	// In production, consider caching with short TTL and proper invalidation

	var contexts []*models.Context
	err := ocm.circuitBreakers.GetBreaker("list").ExecuteWithFallback(
		ctx, "list_contexts",
		// Primary function - use read replica
		func() error {
			db := ocm.selectReadReplica()
			var err error
			contexts, err = ocm.listContextsFromDB(ctx, db, agentID, sessionID, options)
			return err
		},
		// Fallback - use primary
		func() error {
			var err error
			contexts, err = ocm.listContextsFromDB(ctx, ocm.primaryDB, agentID, sessionID, options)
			return err
		},
	)

	return contexts, err
}

// Helper methods

func (ocm *OptimizedContextManager) selectReadReplica() *sql.DB {
	ocm.replicaMutex.RLock()
	defer ocm.replicaMutex.RUnlock()

	if len(ocm.readReplicas) == 0 {
		return ocm.primaryDB
	}

	// Simple random selection - could be enhanced with health-aware selection
	return ocm.readReplicas[rand.Intn(len(ocm.readReplicas))]
}

func (ocm *OptimizedContextManager) readContextFromDB(ctx context.Context, db *sql.DB, contextID string) (*models.Context, error) {
	query := `
		SELECT id, agent_id, session_id, parent_id, content, metadata,
		       truncation_strategy, retention_policy, tags, relationships,
		       created_at, updated_at, version
		FROM mcp.contexts
		WHERE id = $1`

	var context models.Context
	err := db.QueryRowContext(ctx, query, contextID).Scan(
		&context.ID, &context.AgentID, &context.SessionID, &context.ParentID,
		&context.Content, &context.Metadata, &context.TruncationStrategy,
		&context.RetentionPolicy, &context.Tags, &context.Relationships,
		&context.CreatedAt, &context.UpdatedAt, &context.Version,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}

	return &context, err
}

func (ocm *OptimizedContextManager) listContextsFromDB(
	ctx context.Context,
	db *sql.DB,
	agentID string,
	sessionID string,
	options map[string]interface{},
) ([]*models.Context, error) {
	// Build query with filters
	query := `
		SELECT id, agent_id, session_id, parent_id, content, metadata,
		       truncation_strategy, retention_policy, tags, relationships,
		       created_at, updated_at, version
		FROM mcp.contexts
		WHERE agent_id = $1`
	
	args := []interface{}{agentID}
	
	if sessionID != "" {
		query += " AND session_id = $2"
		args = append(args, sessionID)
	}

	// Add ordering and pagination
	query += " ORDER BY created_at DESC"
	
	if limit, ok := options["limit"].(int); ok {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	
	if offset, ok := options["offset"].(int); ok {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contexts []*models.Context
	for rows.Next() {
		var context models.Context
		err := rows.Scan(
			&context.ID, &context.AgentID, &context.SessionID, &context.ParentID,
			&context.Content, &context.Metadata, &context.TruncationStrategy,
			&context.RetentionPolicy, &context.Tags, &context.Relationships,
			&context.CreatedAt, &context.UpdatedAt, &context.Version,
		)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, &context)
	}

	return contexts, rows.Err()
}

func (ocm *OptimizedContextManager) buildUpdateQuery(
	contextID string,
	updatedContext *models.Context,
	options *models.ContextUpdateOptions,
) (string, []interface{}) {
	// Implementation would build dynamic UPDATE query based on options
	// This is a simplified version
	query := `
		UPDATE mcp.contexts
		SET content = $2, metadata = $3, updated_at = NOW(), version = version + 1
		WHERE id = $1
		RETURNING id, agent_id, session_id, parent_id, content, metadata,
		          truncation_strategy, retention_policy, tags, relationships,
		          created_at, updated_at, version`
	
	args := []interface{}{contextID, updatedContext.Content, updatedContext.Metadata}
	
	return query, args
}

func (ocm *OptimizedContextManager) contextCacheKey(contextID string) string {
	return fmt.Sprintf("context:%s", contextID)
}

func (ocm *OptimizedContextManager) listCacheKey(agentID, sessionID string, options map[string]interface{}) string {
	// Build deterministic cache key from parameters
	return fmt.Sprintf("list:%s:%s", agentID, sessionID)
}

func (ocm *OptimizedContextManager) invalidateListCaches(ctx context.Context, agentID, sessionID string) {
	pattern := fmt.Sprintf("list:%s:*", agentID)
	if err := ocm.cache.InvalidatePattern(ctx, pattern); err != nil {
		ocm.logger.Warn("Failed to invalidate list caches", "pattern", pattern, "error", err)
	}
}

func (ocm *OptimizedContextManager) recordLatency(ctx context.Context, operation string, duration time.Duration) {
	ocm.metrics.RecordLatency(ctx, "context_manager_operation", duration.Milliseconds(), "operation", operation)
	
	// Log slow queries
	if duration > ocm.config.Monitoring.SlowQueryThreshold {
		select {
		case ocm.slowQueryLog <- slowQuery{operation, duration, nil}:
		default:
			// Don't block if channel is full
		}
	}
}

func (ocm *OptimizedContextManager) monitorReplicaHealth() {
	ticker := time.NewTicker(ocm.config.Database.HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		ocm.replicaMutex.Lock()
		for i, replica := range ocm.readReplicas {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := replica.PingContext(ctx)
			cancel()
			
			if err != nil {
				ocm.logger.Error("Read replica health check failed", "replica", i, "error", err)
				ocm.metrics.IncrementCounter(context.Background(), "replica_health_check_failed", 1, "replica", fmt.Sprintf("%d", i))
			}
		}
		ocm.replicaMutex.Unlock()
	}
}

func (ocm *OptimizedContextManager) logSlowQueries() {
	for sq := range ocm.slowQueryLog {
		ocm.logger.Warn("Slow query detected",
			"operation", sq.query,
			"duration", sq.duration,
			"threshold", ocm.config.Monitoring.SlowQueryThreshold,
		)
		ocm.metrics.IncrementCounter(context.Background(), "slow_queries", 1, "operation", sq.query)
	}
}

func (ocm *OptimizedContextManager) warmCache() {
	// Implementation would load frequently accessed contexts
	ocm.logger.Info("Cache warming initiated")
	
	// Example: Load recent contexts
	ctx := context.Background()
	query := `
		SELECT id FROM mcp.contexts
		ORDER BY updated_at DESC
		LIMIT $1`
	
	rows, err := ocm.primaryDB.QueryContext(ctx, query, ocm.config.Cache.Warming.RecentContextsCount)
	if err != nil {
		ocm.logger.Error("Failed to query contexts for cache warming", "error", err)
		return
	}
	defer rows.Close()

	var contextIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		contextIDs = append(contextIDs, id)
	}

	// Warm cache with these contexts
	err = ocm.cache.WarmCache(ctx, contextIDs, func(id string) (*models.Context, error) {
		return ocm.readContextFromDB(ctx, ocm.primaryDB, id)
	})
	
	if err != nil {
		ocm.logger.Error("Cache warming failed", "error", err)
	} else {
		ocm.logger.Info("Cache warming completed", "contexts", len(contextIDs))
	}
}

// Close gracefully shuts down the context manager
func (ocm *OptimizedContextManager) Close() error {
	// Close read replicas
	for _, replica := range ocm.readReplicas {
		if err := replica.Close(); err != nil {
			ocm.logger.Error("Failed to close read replica", "error", err)
		}
	}

	// Close cache
	if err := ocm.cache.Close(); err != nil {
		ocm.logger.Error("Failed to close cache", "error", err)
	}

	// Close slow query channel
	close(ocm.slowQueryLog)

	return nil
}