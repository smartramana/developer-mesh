package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
)

// BaseRepository provides common functionality for all repositories
type BaseRepository struct {
	writeDB *sqlx.DB
	readDB  *sqlx.DB
	tx      *sqlx.Tx // Transaction, if operating within one
	cache   cache.Cache
	logger  observability.Logger
	tracer  observability.StartSpanFunc
	metrics observability.MetricsClient
	cb      *resilience.CircuitBreaker

	// Prepared statements cache
	stmtCache   map[string]*sqlx.NamedStmt
	stmtCacheMu sync.RWMutex

	// Configuration
	queryTimeout time.Duration
	maxRetries   int
	cacheTimeout time.Duration
}

// BaseRepositoryConfig holds configuration for BaseRepository
type BaseRepositoryConfig struct {
	QueryTimeout   time.Duration
	MaxRetries     int
	CacheTimeout   time.Duration
	CircuitBreaker *resilience.CircuitBreaker
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(
	writeDB, readDB *sqlx.DB,
	cache cache.Cache,
	logger observability.Logger,
	tracer observability.StartSpanFunc,
	metrics observability.MetricsClient,
	config BaseRepositoryConfig,
) *BaseRepository {
	if config.QueryTimeout == 0 {
		config.QueryTimeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.CacheTimeout == 0 {
		config.CacheTimeout = 5 * time.Minute
	}

	return &BaseRepository{
		writeDB:      writeDB,
		readDB:       readDB,
		tx:           nil,
		cache:        cache,
		logger:       logger,
		tracer:       tracer,
		metrics:      metrics,
		cb:           config.CircuitBreaker,
		stmtCache:    make(map[string]*sqlx.NamedStmt),
		queryTimeout: config.QueryTimeout,
		maxRetries:   config.MaxRetries,
		cacheTimeout: config.CacheTimeout,
	}
}

// WithTx creates a new repository instance that uses the provided transaction
func (r *BaseRepository) WithTx(tx *sqlx.Tx) *BaseRepository {
	return &BaseRepository{
		writeDB:      r.writeDB,
		readDB:       r.readDB,
		tx:           tx,
		cache:        r.cache,
		logger:       r.logger,
		tracer:       r.tracer,
		metrics:      r.metrics,
		cb:           r.cb,
		stmtCache:    r.stmtCache,
		queryTimeout: r.queryTimeout,
		maxRetries:   r.maxRetries,
		cacheTimeout: r.cacheTimeout,
	}
}

// WithTransaction executes a function within a database transaction
func (r *BaseRepository) WithTransaction(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	ctx, span := r.tracer(ctx, "BaseRepository.WithTransaction")
	defer span.End()

	timer := r.metrics.StartTimer("repository_transaction_duration", nil)
	defer timer()

	tx, err := r.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		r.metrics.IncrementCounter("repository_transaction_errors", 1)
		r.logger.Error("Failed to begin transaction", map[string]interface{}{
			"error": err.Error(),
		})
		return errors.Wrap(err, "failed to begin transaction")
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // Re-throw panic after rollback
		}
	}()

	err = fn(tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			r.logger.Error("Failed to rollback transaction", map[string]interface{}{
				"error": rbErr.Error(),
			})
		}
		r.metrics.IncrementCounter("repository_transaction_rollbacks", 1)
		return err
	}

	if err := tx.Commit(); err != nil {
		r.metrics.IncrementCounter("repository_transaction_errors", 1)
		r.logger.Error("Failed to commit transaction", map[string]interface{}{
			"error": err.Error(),
		})
		return errors.Wrap(err, "failed to commit transaction")
	}

	r.metrics.IncrementCounter("repository_transaction_commits", 1)
	return nil
}

// WithTransactionOptions executes a function within a database transaction with options
func (r *BaseRepository) WithTransactionOptions(ctx context.Context, opts *types.TxOptions, fn func(tx *sqlx.Tx) error) error {
	ctx, span := r.tracer(ctx, "BaseRepository.WithTransactionOptions")
	defer span.End()

	timer := r.metrics.StartTimer("repository_transaction_duration", nil)
	defer timer()

	var txOpts *sql.TxOptions
	if opts != nil {
		txOpts = &sql.TxOptions{
			Isolation: sql.IsolationLevel(opts.Isolation),
			ReadOnly:  opts.ReadOnly,
		}
	}

	tx, err := r.writeDB.BeginTxx(ctx, txOpts)
	if err != nil {
		r.metrics.IncrementCounter("repository_transaction_errors", 1)
		r.logger.Error("Failed to begin transaction with options", map[string]interface{}{
			"error":     err.Error(),
			"isolation": opts.Isolation,
			"read_only": opts.ReadOnly,
		})
		return errors.Wrap(err, "failed to begin transaction")
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	err = fn(tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			r.logger.Error("Failed to rollback transaction", map[string]interface{}{
				"error": rbErr.Error(),
			})
		}
		r.metrics.IncrementCounter("repository_transaction_rollbacks", 1)
		return err
	}

	if err := tx.Commit(); err != nil {
		r.metrics.IncrementCounter("repository_transaction_errors", 1)
		r.logger.Error("Failed to commit transaction", map[string]interface{}{
			"error": err.Error(),
		})
		return errors.Wrap(err, "failed to commit transaction")
	}

	r.metrics.IncrementCounter("repository_transaction_commits", 1)
	return nil
}

// GetPreparedStatement gets or creates a prepared statement
func (r *BaseRepository) GetPreparedStatement(name, query string, db *sqlx.DB) (*sqlx.NamedStmt, error) {
	r.stmtCacheMu.RLock()
	stmt, exists := r.stmtCache[name]
	r.stmtCacheMu.RUnlock()

	if exists {
		return stmt, nil
	}

	r.stmtCacheMu.Lock()
	defer r.stmtCacheMu.Unlock()

	// Double-check in case another goroutine created it
	if stmt, exists := r.stmtCache[name]; exists {
		return stmt, nil
	}

	stmt, err := db.PrepareNamed(query)
	if err != nil {
		r.logger.Error("Failed to prepare statement", map[string]interface{}{
			"error": err.Error(),
			"name":  name,
		})
		return nil, errors.Wrapf(err, "failed to prepare statement %s", name)
	}

	r.stmtCache[name] = stmt
	return stmt, nil
}

// CacheGet retrieves a value from cache with proper error handling and metrics
func (r *BaseRepository) CacheGet(ctx context.Context, key string, dest interface{}) error {
	ctx, span := r.tracer(ctx, "BaseRepository.CacheGet")
	defer span.End()

	timer := r.metrics.StartTimer("repository_cache_operation_duration", map[string]string{
		"operation": "get",
	})
	defer timer()

	err := r.cache.Get(ctx, key, dest)
	if err != nil {
		if err == cache.ErrNotFound {
			r.metrics.IncrementCounterWithLabels("repository_cache_operations", 1, map[string]string{
				"operation": "get",
				"result":    "miss",
			})
			return err
		}
		r.metrics.IncrementCounterWithLabels("repository_cache_operations", 1, map[string]string{
			"operation": "get",
			"result":    "error",
		})
		r.logger.Error("Cache get error", map[string]interface{}{
			"error": err.Error(),
			"key":   key,
		})
		return err
	}

	r.metrics.IncrementCounterWithLabels("repository_cache_operations", 1, map[string]string{
		"operation": "get",
		"result":    "hit",
	})
	return nil
}

// CacheSet stores a value in cache with proper error handling and metrics
func (r *BaseRepository) CacheSet(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	ctx, span := r.tracer(ctx, "BaseRepository.CacheSet")
	defer span.End()

	timer := r.metrics.StartTimer("repository_cache_operation_duration", map[string]string{
		"operation": "set",
	})
	defer timer()

	if ttl == 0 {
		ttl = r.cacheTimeout
	}

	err := r.cache.Set(ctx, key, value, ttl)
	if err != nil {
		r.metrics.IncrementCounterWithLabels("repository_cache_operations", 1, map[string]string{
			"operation": "set",
			"result":    "error",
		})
		r.logger.Error("Cache set error", map[string]interface{}{
			"error": err.Error(),
			"key":   key,
			"ttl":   ttl.String(),
		})
		return err
	}

	r.metrics.IncrementCounterWithLabels("repository_cache_operations", 1, map[string]string{
		"operation": "set",
		"result":    "success",
	})
	return nil
}

// CacheDelete removes a value from cache
func (r *BaseRepository) CacheDelete(ctx context.Context, key string) error {
	ctx, span := r.tracer(ctx, "BaseRepository.CacheDelete")
	defer span.End()

	timer := r.metrics.StartTimer("repository_cache_operation_duration", map[string]string{
		"operation": "delete",
	})
	defer timer()

	err := r.cache.Delete(ctx, key)
	if err != nil {
		r.metrics.IncrementCounterWithLabels("repository_cache_operations", 1, map[string]string{
			"operation": "delete",
			"result":    "error",
		})
		r.logger.Error("Cache delete error", map[string]interface{}{
			"error": err.Error(),
			"key":   key,
		})
		return err
	}

	r.metrics.IncrementCounterWithLabels("repository_cache_operations", 1, map[string]string{
		"operation": "delete",
		"result":    "success",
	})
	return nil
}

// TranslateError converts database errors to domain errors
func (r *BaseRepository) TranslateError(err error, entity string) error {
	if err == nil {
		return nil
	}

	// Check for no rows error
	if err == sql.ErrNoRows {
		return interfaces.ErrNotFound
	}

	// Check for PostgreSQL errors
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Code {
		case "23505": // unique_violation
			return interfaces.ErrDuplicate
		case "23503": // foreign_key_violation
			return errors.Wrap(interfaces.ErrValidation, "foreign key constraint violation")
		case "23502": // not_null_violation
			return errors.Wrap(interfaces.ErrValidation, "required field missing")
		case "23514": // check_violation
			// Include the constraint name in the error for debugging
			return errors.Wrapf(interfaces.ErrValidation, "check constraint violation: %s", pqErr.Constraint)
		case "40001": // serialization_failure
			return interfaces.ErrOptimisticLock
		}
	}

	// Log unexpected errors
	r.logger.Error("Unexpected database error", map[string]interface{}{
		"error":  err.Error(),
		"entity": entity,
	})

	return errors.Wrapf(err, "database error for %s", entity)
}

// ExecuteWithCircuitBreaker executes a function with circuit breaker protection
func (r *BaseRepository) ExecuteWithCircuitBreaker(ctx context.Context, name string, fn func() (interface{}, error)) (interface{}, error) {
	if r.cb == nil {
		// No circuit breaker configured, execute directly
		return fn()
	}

	ctx, span := r.tracer(ctx, fmt.Sprintf("BaseRepository.ExecuteWithCircuitBreaker.%s", name))
	defer span.End()

	result, err := r.cb.Execute(ctx, fn)
	if err != nil {
		r.metrics.IncrementCounterWithLabels("repository_circuit_breaker_errors", 1, map[string]string{
			"operation": name,
		})
		return nil, err
	}

	return result, nil
}

// ExecuteQuery executes a query with timeout and metrics
func (r *BaseRepository) ExecuteQuery(ctx context.Context, operation string, fn func(ctx context.Context) error) error {
	ctx, span := r.tracer(ctx, fmt.Sprintf("BaseRepository.ExecuteQuery.%s", operation))
	defer span.End()

	timer := r.metrics.StartTimer("repository_query_duration", map[string]string{
		"operation": operation,
	})
	defer timer()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	err := fn(ctx)
	if err != nil {
		r.metrics.IncrementCounterWithLabels("repository_query_errors", 1, map[string]string{
			"operation": operation,
			"error":     classifyDBError(err),
		})
		return err
	}

	r.metrics.IncrementCounterWithLabels("repository_query_success", 1, map[string]string{
		"operation": operation,
	})
	return nil
}

// ExecuteQueryWithRetry executes a query with retry logic
func (r *BaseRepository) ExecuteQueryWithRetry(ctx context.Context, operation string, fn func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt < r.maxRetries; attempt++ {
		err := r.ExecuteQuery(ctx, operation, fn)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on certain errors
		if err == interfaces.ErrNotFound || err == interfaces.ErrDuplicate || err == interfaces.ErrValidation {
			return err
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Log retry attempt
		r.logger.Warn("Retrying query after error", map[string]interface{}{
			"operation": operation,
			"attempt":   attempt + 1,
			"error":     err.Error(),
		})

		// Exponential backoff
		backoff := time.Duration(attempt+1) * 100 * time.Millisecond
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return errors.Wrapf(lastErr, "query failed after %d attempts", r.maxRetries)
}

// InvalidateCachePattern invalidates cache entries matching a pattern
func (r *BaseRepository) InvalidateCachePattern(ctx context.Context, pattern string) error {
	_, span := r.tracer(ctx, "BaseRepository.InvalidateCachePattern")
	defer span.End()

	// Most cache implementations don't support pattern deletion
	// This is a best-effort approach - log the pattern for monitoring
	r.logger.Info("Cache invalidation requested", map[string]interface{}{
		"pattern": pattern,
	})

	// For Redis-based caches, this could be implemented with SCAN and DEL
	// For now, we'll just log it
	r.metrics.IncrementCounterWithLabels("repository_cache_invalidations", 1, map[string]string{
		"pattern": pattern,
	})

	return nil
}

// GetMetrics returns current repository metrics
func (r *BaseRepository) GetMetrics() map[string]interface{} {
	// This would return current metrics snapshot
	// Implementation depends on metrics client capabilities
	return map[string]interface{}{
		"prepared_statements": len(r.stmtCache),
	}
}

// Close cleans up resources
func (r *BaseRepository) Close() error {
	r.stmtCacheMu.Lock()
	defer r.stmtCacheMu.Unlock()

	var errs []error

	// Close all prepared statements
	for name, stmt := range r.stmtCache {
		if err := stmt.Close(); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to close statement %s", name))
		}
	}

	r.stmtCache = make(map[string]*sqlx.NamedStmt)

	if len(errs) > 0 {
		return errors.Errorf("failed to close %d statements", len(errs))
	}

	return nil
}

// classifyDBError categorizes errors for metrics
func classifyDBError(err error) string {
	if err == nil {
		return "none"
	}

	switch err {
	case sql.ErrNoRows, interfaces.ErrNotFound:
		return "not_found"
	case interfaces.ErrDuplicate:
		return "duplicate"
	case interfaces.ErrValidation:
		return "validation"
	case interfaces.ErrOptimisticLock:
		return "optimistic_lock"
	case context.DeadlineExceeded:
		return "timeout"
	case context.Canceled:
		return "cancelled"
	}

	if pqErr, ok := err.(*pq.Error); ok {
		return string(pqErr.Code)
	}

	return "unknown"
}
