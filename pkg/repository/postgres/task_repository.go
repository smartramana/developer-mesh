package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
	"github.com/S-Corkum/devops-mcp/pkg/repository/types"
)

// taskRepository implements TaskRepository with production features
type taskRepository struct {
	writeDB        *sqlx.DB
	readDB         *sqlx.DB              // Read replica
	cache          cache.Cache // Cache interface (TODO: Implement MultiLevelCache)
	logger         observability.Logger
	metrics        *repositoryMetrics
	tracer         observability.StartSpanFunc // Function type for starting spans
	// TODO: Implement circuit breaker when resilience.CircuitBreaker has Execute method
	// circuitBreaker resilience.CircuitBreaker

	// Prepared statements cache
	stmtCache   map[string]*sqlx.NamedStmt
	stmtCacheMu sync.RWMutex

	// Configuration
	queryTimeout time.Duration
	maxRetries   int
	batchSize    int
}

// repositoryMetrics holds Prometheus metrics
type repositoryMetrics struct {
	queries       *prometheus.CounterVec
	queryDuration *prometheus.HistogramVec
	cacheHits     *prometheus.CounterVec
	cacheMisses   *prometheus.CounterVec
	errors        *prometheus.CounterVec
	poolStats     *prometheus.GaugeVec
}

// RepositoryOption configures the repository
type RepositoryOption func(*taskRepository)

// WithQueryTimeout sets the query timeout
func WithQueryTimeout(timeout time.Duration) RepositoryOption {
	return func(r *taskRepository) {
		r.queryTimeout = timeout
	}
}

// WithMaxRetries sets the maximum retry attempts
func WithMaxRetries(retries int) RepositoryOption {
	return func(r *taskRepository) {
		r.maxRetries = retries
	}
}

// WithBatchSize sets the batch operation size
func WithBatchSize(size int) RepositoryOption {
	return func(r *taskRepository) {
		r.batchSize = size
	}
}

// NewTaskRepository creates a production-ready task repository
func NewTaskRepository(
	writeDB, readDB *sqlx.DB,
	cache cache.Cache,
	logger observability.Logger,
	tracer observability.StartSpanFunc,
	opts ...RepositoryOption,
) interfaces.TaskRepository {
	repo := &taskRepository{
		writeDB:      writeDB,
		readDB:       readDB,
		cache:        cache,
		logger:       logger,
		tracer:       tracer,
		stmtCache:    make(map[string]*sqlx.NamedStmt),
		queryTimeout: 30 * time.Second,
		maxRetries:   3,
		batchSize:    1000,
	}

	// Apply options
	for _, opt := range opts {
		opt(repo)
	}

	// Initialize metrics
	repo.metrics = initializeMetrics()

	// TODO: Initialize circuit breaker when resilience package is complete
	// repo.circuitBreaker = resilience.NewCircuitBreaker(
	// 	"task_repository",
	// 	resilience.WithErrorThreshold(0.5),
	// 	resilience.WithTimeout(10*time.Second),
	// 	resilience.WithMaxConcurrent(100),
	// )

	// Start background tasks
	go repo.monitorPoolStats()
	go repo.prepareCommonStatements()

	return repo
}

// WithTx returns a repository instance that uses the provided transaction
func (r *taskRepository) WithTx(tx types.Transaction) interfaces.TaskRepository {
	// TODO: Implement proper transaction support
	// pgTx, ok := tx.(*pgTransaction)
	// if !ok {
	// 	panic("invalid transaction type")
	// }

	// TODO: Implement proper transaction support
	// For now, return a copy of the repository
	// This doesn't actually use the transaction
	return &taskRepository{
		// TODO: Need to implement proper transaction wrapping
		// For now, return the original repository
		// This is a limitation that needs to be addressed
		writeDB:        r.writeDB,
		readDB:         r.readDB,
		cache:          r.cache,
		logger:         r.logger,
		metrics:        r.metrics,
		tracer:         r.tracer,
		// TODO: Add circuitBreaker when implemented
		stmtCache:      r.stmtCache,
		queryTimeout:   r.queryTimeout,
		maxRetries:     r.maxRetries,
		batchSize:      r.batchSize,
	}
}

// BeginTx starts a new transaction with options
func (r *taskRepository) BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.BeginTx")
	defer span.End()

	txOpts := &sql.TxOptions{
		Isolation: sql.LevelReadCommitted, // Default
		ReadOnly:  false,
	}

	if opts != nil {
		switch opts.Isolation {
		case types.IsolationSerializable:
			txOpts.Isolation = sql.LevelSerializable
		case types.IsolationRepeatableRead:
			txOpts.Isolation = sql.LevelRepeatableRead
		case types.IsolationReadCommitted:
			txOpts.Isolation = sql.LevelReadCommitted
		case types.IsolationReadUncommitted:
			txOpts.Isolation = sql.LevelReadUncommitted
		}
		txOpts.ReadOnly = opts.ReadOnly
	}

	tx, err := r.writeDB.BeginTxx(ctx, txOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin transaction")
	}

	// Set transaction timeout
	if opts != nil && opts.Timeout > 0 {
		_, err = tx.ExecContext(ctx, "SET LOCAL statement_timeout = $1", opts.Timeout.Milliseconds())
		if err != nil {
			tx.Rollback()
			return nil, errors.Wrap(err, "failed to set transaction timeout")
		}
	}

	return &pgTransaction{tx: tx, logger: r.logger, startTime: time.Now()}, nil
}

// Create inserts a new task with retry logic
func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
	ctx, span := r.tracer(ctx, "TaskRepository.Create")
	defer span.End()

	// TODO: Wrap with circuit breaker when implemented
	// return r.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
	// 	return r.createWithRetry(ctx, task)
	// })
	return r.createWithRetry(ctx, task)
}

func (r *taskRepository) createWithRetry(ctx context.Context, task *models.Task) error {
	var err error
	for attempt := 0; attempt < r.maxRetries; attempt++ {
		err = r.doCreate(ctx, task)
		if err == nil {
			return nil
		}

		// Check if retryable
		if !isRetryableError(err) {
			return err
		}

		// Exponential backoff
		backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			continue
		}
	}
	return err
}

func (r *taskRepository) doCreate(ctx context.Context, task *models.Task) error {
	timer := prometheus.NewTimer(r.metrics.queryDuration.WithLabelValues("create"))
	defer timer.ObserveDuration()

	// Set query timeout
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query := `
		INSERT INTO tasks (
			id, tenant_id, type, status, priority,
			created_by, assigned_to, parent_task_id,
			title, description, parameters, 
			max_retries, timeout_seconds,
			created_at, updated_at
		) VALUES (
			:id, :tenant_id, :type, :status, :priority,
			:created_by, :assigned_to, :parent_task_id,
			:title, :description, :parameters,
			:max_retries, :timeout_seconds,
			:created_at, :updated_at
		)
		ON CONFLICT (id, created_at) DO NOTHING
		RETURNING id`

	// Generate ID and timestamps
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	task.Version = 1

	// Execute with named parameters
	var returnedID uuid.UUID
	stmt, err := r.getOrPrepareStmt(ctx, "create_task", query)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}

	err = stmt.GetContext(ctx, &returnedID, task)
	if err != nil {
		r.metrics.errors.WithLabelValues("create", classifyError(err)).Inc()
		if err == sql.ErrNoRows {
			// Conflict - task already exists
			return types.ErrAlreadyExists
		}
		return errors.Wrap(err, "failed to create task")
	}

	// Invalidate cache
	r.invalidateTaskCache(ctx, task)

	r.metrics.queries.WithLabelValues("create", "success").Inc()
	return nil
}

// CreateBatch performs bulk insert using COPY
func (r *taskRepository) CreateBatch(ctx context.Context, tasks []*models.Task) error {
	ctx, span := r.tracer(ctx, "TaskRepository.CreateBatch")
	defer span.End()

	if len(tasks) == 0 {
		return nil
	}

	// TODO: Wrap with circuit breaker when implemented
	// return r.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
	// Process in batches
	for i := 0; i < len(tasks); i += r.batchSize {
		end := i + r.batchSize
		if end > len(tasks) {
			end = len(tasks)
		}

		batch := tasks[i:end]
		if err := r.doBatchInsert(ctx, batch); err != nil {
			return errors.Wrapf(err, "failed to insert batch %d-%d", i, end)
		}
	}
	return nil
	// })
}

func (r *taskRepository) doBatchInsert(ctx context.Context, tasks []*models.Task) error {
	tx, err := r.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback()

	// Prepare COPY statement
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("tasks",
		"id", "tenant_id", "type", "status", "priority",
		"created_by", "assigned_to", "parent_task_id",
		"title", "description", "parameters",
		"max_retries", "timeout_seconds",
		"created_at", "updated_at",
	))
	if err != nil {
		return errors.Wrap(err, "failed to prepare copy statement")
	}
	defer stmt.Close()

	// Generate IDs and timestamps
	now := time.Now()
	for _, task := range tasks {
		if task.ID == uuid.Nil {
			task.ID = uuid.New()
		}
		task.CreatedAt = now
		task.UpdatedAt = now
		task.Version = 1

		// Convert parameters to JSON
		paramsJSON, err := json.Marshal(task.Parameters)
		if err != nil {
			return errors.Wrap(err, "failed to marshal parameters")
		}

		_, err = stmt.ExecContext(ctx,
			task.ID, task.TenantID, task.Type, task.Status, task.Priority,
			task.CreatedBy, task.AssignedTo, task.ParentTaskID,
			task.Title, task.Description, paramsJSON,
			task.MaxRetries, task.TimeoutSeconds,
			task.CreatedAt, task.UpdatedAt,
		)
		if err != nil {
			return errors.Wrap(err, "failed to execute copy")
		}
	}

	// Execute COPY
	_, err = stmt.ExecContext(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to complete copy")
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	// Invalidate cache for all tasks
	for _, task := range tasks {
		r.invalidateTaskCache(ctx, task)
	}

	r.metrics.queries.WithLabelValues("batch_insert", "success").Inc()
	return nil
}

// Get retrieves a task with multi-level caching
func (r *taskRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.Get")
	defer span.End()

	// Try cache first
	cacheKey := fmt.Sprintf("task:%s", id)
	var task models.Task
	err := r.cache.Get(ctx, cacheKey, &task)
	if err == nil {
		r.metrics.cacheHits.WithLabelValues("task").Inc()
		return &task, nil
	}

	r.metrics.cacheMisses.WithLabelValues("task").Inc()

	// Query database (use read replica)
	// TODO: Wrap with circuit breaker when implemented
	err = r.doGet(ctx, id, &task)

	if err != nil {
		return nil, err
	}

	// Cache the result
	r.cache.Set(ctx, cacheKey, &task, 5*time.Minute)

	return &task, nil
}

func (r *taskRepository) doGet(ctx context.Context, id uuid.UUID, task *models.Task) error {
	timer := prometheus.NewTimer(r.metrics.queryDuration.WithLabelValues("get"))
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query := `
		SELECT 
			id, tenant_id, type, status, priority,
			created_by, assigned_to, parent_task_id,
			title, description, parameters, result, error,
			max_retries, retry_count, timeout_seconds,
			created_at, assigned_at, started_at, completed_at, 
			updated_at, deleted_at, version
		FROM tasks
		WHERE id = $1 AND deleted_at IS NULL`

	err := r.readDB.GetContext(ctx, task, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		r.metrics.errors.WithLabelValues("get", classifyError(err)).Inc()
		return errors.Wrap(err, "failed to get task")
	}

	r.metrics.queries.WithLabelValues("get", "success").Inc()
	return nil
}

// GetBatch retrieves multiple tasks by IDs
func (r *taskRepository) GetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetBatch")
	defer span.End()

	if len(ids) == 0 {
		return []*models.Task{}, nil
	}

	// Try to get from cache first
	tasks := make([]*models.Task, 0, len(ids))
	uncachedIDs := make([]uuid.UUID, 0)

	for _, id := range ids {
		cacheKey := fmt.Sprintf("task:%s", id)
		var task models.Task
		err := r.cache.Get(ctx, cacheKey, &task)
		if err == nil {
			tasks = append(tasks, &task)
			r.metrics.cacheHits.WithLabelValues("task").Inc()
		} else {
			uncachedIDs = append(uncachedIDs, id)
			r.metrics.cacheMisses.WithLabelValues("task").Inc()
		}
	}

	// Get uncached tasks from database
	if len(uncachedIDs) > 0 {
		dbTasks, err := r.doGetBatch(ctx, uncachedIDs)
		if err != nil {
			return nil, err
		}

		// Cache the results
		for _, task := range dbTasks {
			cacheKey := fmt.Sprintf("task:%s", task.ID)
			r.cache.Set(ctx, cacheKey, task, 5*time.Minute)
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

func (r *taskRepository) doGetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error) {
	timer := prometheus.NewTimer(r.metrics.queryDuration.WithLabelValues("get_batch"))
	defer timer.ObserveDuration()

	query := `
		SELECT 
			id, tenant_id, type, status, priority,
			created_by, assigned_to, parent_task_id,
			title, description, parameters, result, error,
			max_retries, retry_count, timeout_seconds,
			created_at, assigned_at, started_at, completed_at, 
			updated_at, deleted_at, version
		FROM tasks
		WHERE id = ANY($1) AND deleted_at IS NULL`

	var tasks []*models.Task
	err := r.readDB.SelectContext(ctx, &tasks, query, pq.Array(ids))
	if err != nil {
		r.metrics.errors.WithLabelValues("get_batch", classifyError(err)).Inc()
		return nil, errors.Wrap(err, "failed to get tasks")
	}

	r.metrics.queries.WithLabelValues("get_batch", "success").Inc()
	return tasks, nil
}

// GetForUpdate retrieves a task with a row lock for update
func (r *taskRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetForUpdate")
	defer span.End()

	var task models.Task
	// TODO: Wrap with circuit breaker when implemented
	err := r.doGetForUpdate(ctx, id, &task)

	if err != nil {
		return nil, err
	}

	return &task, nil
}

func (r *taskRepository) doGetForUpdate(ctx context.Context, id uuid.UUID, task *models.Task) error {
	timer := prometheus.NewTimer(r.metrics.queryDuration.WithLabelValues("get_for_update"))
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query := `
		SELECT 
			id, tenant_id, type, status, priority,
			created_by, assigned_to, parent_task_id,
			title, description, parameters, result, error,
			max_retries, retry_count, timeout_seconds,
			created_at, assigned_at, started_at, completed_at, 
			updated_at, deleted_at, version
		FROM tasks
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`

	err := r.writeDB.GetContext(ctx, task, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		r.metrics.errors.WithLabelValues("get_for_update", classifyError(err)).Inc()
		return errors.Wrap(err, "failed to get task for update")
	}

	r.metrics.queries.WithLabelValues("get_for_update", "success").Inc()
	return nil
}

// Update updates a task
func (r *taskRepository) Update(ctx context.Context, task *models.Task) error {
	ctx, span := r.tracer(ctx, "TaskRepository.Update")
	defer span.End()

	// TODO: Wrap with circuit breaker when implemented
	return r.doUpdate(ctx, task)
}

func (r *taskRepository) doUpdate(ctx context.Context, task *models.Task) error {
	timer := prometheus.NewTimer(r.metrics.queryDuration.WithLabelValues("update"))
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	// Increment version and update timestamp
	task.Version++
	task.UpdatedAt = time.Now()

	query := `
		UPDATE tasks SET
			type = :type,
			status = :status,
			priority = :priority,
			assigned_to = :assigned_to,
			title = :title,
			description = :description,
			parameters = :parameters,
			result = :result,
			error = :error,
			retry_count = :retry_count,
			assigned_at = :assigned_at,
			started_at = :started_at,
			completed_at = :completed_at,
			updated_at = :updated_at,
			version = :version
		WHERE id = :id AND deleted_at IS NULL`

	stmt, err := r.getOrPrepareStmt(ctx, "update_task", query)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}

	result, err := stmt.ExecContext(ctx, task)
	if err != nil {
		r.metrics.errors.WithLabelValues("update", classifyError(err)).Inc()
		return errors.Wrap(err, "failed to update task")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return types.ErrNotFound
	}

	// Invalidate cache
	r.invalidateTaskCache(ctx, task)

	r.metrics.queries.WithLabelValues("update", "success").Inc()
	return nil
}

// UpdateWithVersion updates a task with optimistic locking
func (r *taskRepository) UpdateWithVersion(ctx context.Context, task *models.Task, expectedVersion int) error {
	ctx, span := r.tracer(ctx, "TaskRepository.UpdateWithVersion")
	defer span.End()

	// TODO: Wrap with circuit breaker when implemented
	return r.doUpdateWithVersion(ctx, task, expectedVersion)
}

func (r *taskRepository) doUpdateWithVersion(ctx context.Context, task *models.Task, expectedVersion int) error {
	timer := prometheus.NewTimer(r.metrics.queryDuration.WithLabelValues("update_with_version"))
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	// Increment version and update timestamp
	task.Version = expectedVersion + 1
	task.UpdatedAt = time.Now()

	query := `
		UPDATE tasks SET
			type = :type,
			status = :status,
			priority = :priority,
			assigned_to = :assigned_to,
			title = :title,
			description = :description,
			parameters = :parameters,
			result = :result,
			error = :error,
			retry_count = :retry_count,
			assigned_at = :assigned_at,
			started_at = :started_at,
			completed_at = :completed_at,
			updated_at = :updated_at,
			version = :version
		WHERE id = :id AND version = :expected_version AND deleted_at IS NULL`

	// Add expected version to task struct temporarily
	type taskWithExpectedVersion struct {
		*models.Task
		ExpectedVersion int `db:"expected_version"`
	}

	taskWithVer := &taskWithExpectedVersion{
		Task:            task,
		ExpectedVersion: expectedVersion,
	}

	stmt, err := r.getOrPrepareStmt(ctx, "update_task_with_version", query)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}

	result, err := stmt.ExecContext(ctx, taskWithVer)
	if err != nil {
		r.metrics.errors.WithLabelValues("update_with_version", classifyError(err)).Inc()
		return errors.Wrap(err, "failed to update task")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		// Check if task exists
		var exists bool
		err = r.readDB.GetContext(ctx, &exists, "SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1 AND deleted_at IS NULL)", task.ID)
		if err != nil {
			return errors.Wrap(err, "failed to check task existence")
		}
		if !exists {
			return types.ErrNotFound
		}
		return types.ErrOptimisticLock
	}

	// Invalidate cache
	r.invalidateTaskCache(ctx, task)

	r.metrics.queries.WithLabelValues("update_with_version", "success").Inc()
	return nil
}

// Continue with remaining methods...

// Helper methods

func (r *taskRepository) getOrPrepareStmt(ctx context.Context, name, query string) (*sqlx.NamedStmt, error) {
	r.stmtCacheMu.RLock()
	stmt, exists := r.stmtCache[name]
	r.stmtCacheMu.RUnlock()

	if exists {
		return stmt, nil
	}

	// Prepare statement
	r.stmtCacheMu.Lock()
	defer r.stmtCacheMu.Unlock()

	// Double-check after acquiring write lock
	if stmt, exists := r.stmtCache[name]; exists {
		return stmt, nil
	}

	namedStmt, err := r.writeDB.PrepareNamedContext(ctx, query)
	if err != nil {
		return nil, err
	}

	r.stmtCache[name] = namedStmt
	return namedStmt, nil
}

func (r *taskRepository) invalidateTaskCache(ctx context.Context, task *models.Task) {
	keys := []string{
		fmt.Sprintf("task:%s", task.ID),
		fmt.Sprintf("tasks:agent:%s", task.AssignedTo),
		fmt.Sprintf("tasks:tenant:%s", task.TenantID),
	}

	if task.ParentTaskID != nil {
		keys = append(keys, fmt.Sprintf("subtasks:%s", *task.ParentTaskID))
	}

	for _, key := range keys {
		r.cache.Delete(ctx, key)
	}
}

func (r *taskRepository) monitorPoolStats() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := r.writeDB.Stats()
		r.metrics.poolStats.WithLabelValues("write", "open").Set(float64(stats.OpenConnections))
		r.metrics.poolStats.WithLabelValues("write", "in_use").Set(float64(stats.InUse))
		r.metrics.poolStats.WithLabelValues("write", "idle").Set(float64(stats.Idle))

		stats = r.readDB.Stats()
		r.metrics.poolStats.WithLabelValues("read", "open").Set(float64(stats.OpenConnections))
		r.metrics.poolStats.WithLabelValues("read", "in_use").Set(float64(stats.InUse))
		r.metrics.poolStats.WithLabelValues("read", "idle").Set(float64(stats.Idle))
	}
}

func (r *taskRepository) prepareCommonStatements() {
	// Pre-prepare commonly used statements
	ctx := context.Background()
	commonQueries := map[string]string{
		"get_by_id": `SELECT * FROM tasks WHERE id = :id AND deleted_at IS NULL`,
		"get_by_tenant": `SELECT * FROM tasks WHERE tenant_id = :tenant_id AND deleted_at IS NULL ORDER BY created_at DESC LIMIT :limit OFFSET :offset`,
		"get_by_agent": `SELECT * FROM tasks WHERE assigned_to = :agent_id AND deleted_at IS NULL ORDER BY priority DESC, created_at ASC`,
	}

	for name, query := range commonQueries {
		_, err := r.getOrPrepareStmt(ctx, name, query)
		if err != nil {
			r.logger.Error("Failed to prepare statement", map[string]interface{}{
				"name":  name,
				"error": err.Error(),
			})
		}
	}
}

// Error classification helpers

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// PostgreSQL error codes
	if pgErr, ok := err.(*pq.Error); ok {
		switch pgErr.Code {
		case "40001": // Serialization failure
			return true
		case "40P01": // Deadlock detected
			return true
		case "53000": // Insufficient resources
			return true
		case "53100": // Disk full
			return false
		case "53200": // Out of memory
			return true
		case "53300": // Too many connections
			return true
		case "57014": // Query canceled
			return false
		case "58000": // System error
			return true
		case "58030": // IO error
			return true
		}
	}

	// Network errors
	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "broken pipe") {
		return true
	}

	return false
}

func classifyError(err error) string {
	if err == nil {
		return "none"
	}

	if pgErr, ok := err.(*pq.Error); ok {
		return string(pgErr.Code)
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, sql.ErrNoRows):
		return "not_found"
	case strings.Contains(err.Error(), "connection"):
		return "connection"
	default:
		return "other"
	}
}