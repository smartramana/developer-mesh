package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
)

// taskRepository implements TaskRepository with production features
type taskRepository struct {
	*BaseRepository

	// Task-specific configuration
	batchSize int
}

// NewTaskRepository creates a production-ready task repository
func NewTaskRepository(
	writeDB, readDB *sqlx.DB,
	cache cache.Cache,
	logger observability.Logger,
	tracer observability.StartSpanFunc,
	metrics observability.MetricsClient,
) interfaces.TaskRepository {
	config := BaseRepositoryConfig{
		QueryTimeout: 30 * time.Second,
		MaxRetries:   3,
		CacheTimeout: 5 * time.Minute,
	}

	// Create circuit breaker for external service calls
	cbConfig := resilience.CircuitBreakerConfig{
		FailureThreshold: 5,
		ResetTimeout:     30 * time.Second,
	}
	cb := resilience.NewCircuitBreaker("task_repository", cbConfig, logger, metrics)
	config.CircuitBreaker = cb

	repo := &taskRepository{
		BaseRepository: NewBaseRepository(writeDB, readDB, cache, logger, tracer, metrics, config),
		batchSize:      1000,
	}

	// Optionally start background tasks
	// go repo.monitorPoolStats()
	// go repo.prepareCommonStatements()

	return repo
}

// WithTx returns a repository instance that uses the provided transaction
func (r *taskRepository) WithTx(tx types.Transaction) interfaces.TaskRepository {
	// For now, we return the same instance as transactions are handled at method level
	return &taskRepository{
		BaseRepository: r.BaseRepository,
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
			_ = tx.Rollback()
			return nil, errors.Wrap(err, "failed to set transaction timeout")
		}
	}

	return &pgTransaction{tx: tx, logger: r.logger, startTime: time.Now()}, nil
}

// Create inserts a new task with retry logic
func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
	ctx, span := r.tracer(ctx, "TaskRepository.Create")
	defer span.End()

	_, err := r.ExecuteWithCircuitBreaker(ctx, "task_create", func() (interface{}, error) {
		return nil, r.createWithRetry(ctx, task)
	})
	return err
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
	timer := r.metrics.StartTimer("repository_query_duration", map[string]string{"operation": "create"})
	defer timer()

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
	stmt, err := r.GetPreparedStatement("create_task", query, r.writeDB)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}

	err = stmt.GetContext(ctx, &returnedID, task)
	if err != nil {
		r.metrics.IncrementCounterWithLabels("repository_errors", 1, map[string]string{"operation": "create", "error_type": classifyError(err)})
		if err == sql.ErrNoRows {
			// Conflict - task already exists
			return types.ErrAlreadyExists
		}
		return errors.Wrap(err, "failed to create task")
	}

	// Invalidate cache
	r.invalidateTaskCache(ctx, task)

	r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{"operation": "create", "result": "success"})
	return nil
}

// CreateBatch performs bulk insert using COPY
func (r *taskRepository) CreateBatch(ctx context.Context, tasks []*models.Task) error {
	ctx, span := r.tracer(ctx, "TaskRepository.CreateBatch")
	defer span.End()

	if len(tasks) == 0 {
		return nil
	}

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
	defer func() { _ = tx.Rollback() }()

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
	defer func() {
		if err := stmt.Close(); err != nil {
			r.logger.Error("Failed to close statement", map[string]interface{}{"error": err.Error()})
		}
	}()

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

	r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{"operation": "batch_insert", "result": "success"})
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
		r.metrics.IncrementCounterWithLabels("repository_cache_hits", 1, map[string]string{"type": "task"})
		return &task, nil
	}

	r.metrics.IncrementCounterWithLabels("repository_cache_misses", 1, map[string]string{"type": "task"})

	// Query database (use read replica)
	_, cbErr := r.ExecuteWithCircuitBreaker(ctx, "task_get", func() (interface{}, error) {
		return nil, r.doGet(ctx, id, &task)
	})
	err = cbErr

	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := r.cache.Set(ctx, cacheKey, &task, 5*time.Minute); err != nil {
		r.logger.Warn("Failed to cache task", map[string]interface{}{"error": err.Error(), "task_id": task.ID})
	}

	return &task, nil
}

func (r *taskRepository) doGet(ctx context.Context, id uuid.UUID, task *models.Task) error {
	timer := r.metrics.StartTimer("repository_query_duration", map[string]string{"operation": "get"})
	defer timer()

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
		r.metrics.IncrementCounterWithLabels("repository_errors", 1, map[string]string{"operation": "get", "error_type": classifyError(err)})
		return errors.Wrap(err, "failed to get task")
	}

	r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{"operation": "get", "result": "success"})
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
			// Create a copy to avoid G601 implicit memory aliasing
			taskCopy := task
			tasks = append(tasks, &taskCopy)
			r.metrics.IncrementCounterWithLabels("repository_cache_hits", 1, map[string]string{"type": "task"})
		} else {
			uncachedIDs = append(uncachedIDs, id)
			r.metrics.IncrementCounterWithLabels("repository_cache_misses", 1, map[string]string{"type": "task"})
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
			if err := r.cache.Set(ctx, cacheKey, task, 5*time.Minute); err != nil {
				r.logger.Warn("Failed to cache task", map[string]interface{}{"error": err.Error(), "task_id": task.ID})
			}
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

func (r *taskRepository) doGetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error) {
	timer := r.metrics.StartTimer("repository_query_duration", map[string]string{"operation": "get_batch"})
	defer timer()

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
		r.metrics.IncrementCounterWithLabels("repository_errors", 1, map[string]string{"operation": "get_batch", "error_type": classifyError(err)})
		return nil, errors.Wrap(err, "failed to get tasks")
	}

	r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{"operation": "get_batch", "result": "success"})
	return tasks, nil
}

// GetForUpdate retrieves a task with a row lock for update
func (r *taskRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetForUpdate")
	defer span.End()

	var task models.Task
	_, err := r.ExecuteWithCircuitBreaker(ctx, "task_get_for_update", func() (interface{}, error) {
		return nil, r.doGetForUpdate(ctx, id, &task)
	})

	if err != nil {
		return nil, err
	}

	return &task, nil
}

func (r *taskRepository) doGetForUpdate(ctx context.Context, id uuid.UUID, task *models.Task) error {
	timer := r.metrics.StartTimer("repository_query_duration", map[string]string{"operation": "get_for_update"})
	defer timer()

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
		r.metrics.IncrementCounterWithLabels("repository_errors", 1, map[string]string{"operation": "get_for_update", "error_type": classifyError(err)})
		return errors.Wrap(err, "failed to get task for update")
	}

	r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{"operation": "get_for_update", "result": "success"})
	return nil
}

// Update updates a task
func (r *taskRepository) Update(ctx context.Context, task *models.Task) error {
	ctx, span := r.tracer(ctx, "TaskRepository.Update")
	defer span.End()

	_, err := r.ExecuteWithCircuitBreaker(ctx, "task_update", func() (interface{}, error) {
		return nil, r.doUpdate(ctx, task)
	})
	return err
}

func (r *taskRepository) doUpdate(ctx context.Context, task *models.Task) error {
	timer := r.metrics.StartTimer("repository_query_duration", map[string]string{"operation": "update"})
	defer timer()

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

	stmt, err := r.GetPreparedStatement("update_task", query, r.writeDB)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}

	result, err := stmt.ExecContext(ctx, task)
	if err != nil {
		r.metrics.IncrementCounterWithLabels("repository_errors", 1, map[string]string{"operation": "update", "error_type": classifyError(err)})
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

	r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{"operation": "update", "result": "success"})
	return nil
}

// UpdateWithVersion updates a task with optimistic locking
func (r *taskRepository) UpdateWithVersion(ctx context.Context, task *models.Task, expectedVersion int) error {
	ctx, span := r.tracer(ctx, "TaskRepository.UpdateWithVersion")
	defer span.End()

	_, err := r.ExecuteWithCircuitBreaker(ctx, "task_update_with_version", func() (interface{}, error) {
		return nil, r.doUpdateWithVersion(ctx, task, expectedVersion)
	})
	return err
}

func (r *taskRepository) doUpdateWithVersion(ctx context.Context, task *models.Task, expectedVersion int) error {
	timer := r.metrics.StartTimer("repository_query_duration", map[string]string{"operation": "update_with_version"})
	defer timer()

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

	stmt, err := r.GetPreparedStatement("update_task_with_version", query, r.writeDB)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}

	result, err := stmt.ExecContext(ctx, taskWithVer)
	if err != nil {
		r.metrics.IncrementCounterWithLabels("repository_errors", 1, map[string]string{"operation": "update_with_version", "error_type": classifyError(err)})
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

	r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{"operation": "update_with_version", "result": "success"})
	return nil
}

// Delete performs a hard delete of a task
func (r *taskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer(ctx, "TaskRepository.Delete")
	defer span.End()

	// Use BaseRepository's ExecuteQuery for the delete operation
	return r.ExecuteQuery(ctx, "delete_task", func(ctx context.Context) error {
		// First check if task has subtasks
		var subtaskCount int
		err := r.readDB.GetContext(ctx, &subtaskCount,
			"SELECT COUNT(*) FROM tasks WHERE parent_task_id = $1 AND deleted_at IS NULL", id)
		if err != nil {
			return errors.Wrap(err, "failed to check subtasks")
		}

		if subtaskCount > 0 {
			return errors.New("cannot delete task with active subtasks")
		}

		// Get task info for cache invalidation
		task, err := r.Get(ctx, id)
		if err != nil {
			return err
		}

		// Use transaction for the delete
		return r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
			// Delete the task
			result, err := tx.ExecContext(ctx, "DELETE FROM tasks WHERE id = $1", id)
			if err != nil {
				return errors.Wrap(err, "failed to delete task")
			}

			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return errors.Wrap(err, "failed to get rows affected")
			}

			if rowsAffected == 0 {
				return interfaces.ErrNotFound
			}

			// Invalidate cache
			r.invalidateTaskCache(ctx, task)

			r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{
				"operation": "delete",
				"result":    "success",
			})
			return nil
		})
	})
}

// Helper methods

func (r *taskRepository) invalidateTaskCache(ctx context.Context, task *models.Task) {
	keys := []string{
		fmt.Sprintf("task:%s", task.ID),
		fmt.Sprintf("tasks:tenant:%s", task.TenantID),
	}

	if task.AssignedTo != nil {
		keys = append(keys, fmt.Sprintf("tasks:agent:%s", *task.AssignedTo))
	}

	if task.ParentTaskID != nil {
		keys = append(keys, fmt.Sprintf("subtasks:%s", *task.ParentTaskID))
	}

	for _, key := range keys {
		_ = r.CacheDelete(ctx, key)
	}
}

// Removed unused monitorPoolStats and prepareCommonStatements methods
// These can be implemented when monitoring and statement preparation are needed

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
