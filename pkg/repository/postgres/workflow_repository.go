package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
	"github.com/S-Corkum/devops-mcp/pkg/repository/types"
	"github.com/S-Corkum/devops-mcp/pkg/resilience"
)

// workflowRepository implements WorkflowRepository with production features
type workflowRepository struct {
	*BaseRepository
}

// NewWorkflowRepository creates a production-ready workflow repository
func NewWorkflowRepository(
	writeDB, readDB *sqlx.DB,
	cache cache.Cache,
	logger observability.Logger,
	tracer observability.StartSpanFunc,
	metrics observability.MetricsClient,
) interfaces.WorkflowRepository {
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
	cb := resilience.NewCircuitBreaker("workflow_repository", cbConfig, logger, metrics)
	config.CircuitBreaker = cb

	return &workflowRepository{
		BaseRepository: NewBaseRepository(writeDB, readDB, cache, logger, tracer, metrics, config),
	}
}

// WithTx returns a repository instance that uses the provided transaction
func (r *workflowRepository) WithTx(tx types.Transaction) interfaces.WorkflowRepository {
	// Create a new repository instance with the same configuration but using the transaction
	// For now, we return the same instance as transactions are handled at method level
	return &workflowRepository{
		BaseRepository: r.BaseRepository,
	}
}

// BeginTx starts a new transaction with options
func (r *workflowRepository) BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.BeginTx")
	defer span.End()

	txOpts := &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
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

// Create creates a new workflow
func (r *workflowRepository) Create(ctx context.Context, workflow *models.Workflow) error {
	ctx, span := r.tracer(ctx, "WorkflowRepository.Create")
	defer span.End()

	return r.ExecuteQueryWithRetry(ctx, "create_workflow", func(ctx context.Context) error {
		// Generate ID and timestamps if not provided
		if workflow.ID == uuid.Nil {
			workflow.ID = uuid.New()
		}
		now := time.Now()
		workflow.CreatedAt = now
		workflow.UpdatedAt = now
		workflow.Version = 1
		workflow.IsActive = true // Default to active

		// Prepare the query
		query := `
			INSERT INTO workflows (
				id, tenant_id, name, description, type,
				version, created_by, agents, steps,
				config, tags, is_active, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9,
				$10, $11, $12, $13, $14
			)
			ON CONFLICT (id) DO NOTHING
			RETURNING id`

		// Execute the query
		var returnedID uuid.UUID
		err := r.writeDB.QueryRowContext(ctx, query,
			workflow.ID,
			workflow.TenantID,
			workflow.Name,
			workflow.Description,
			workflow.Type,
			workflow.Version,
			workflow.CreatedBy,
			workflow.Agents,
			workflow.Steps,
			workflow.Config,
			workflow.Tags,
			workflow.IsActive,
			workflow.CreatedAt,
			workflow.UpdatedAt,
		).Scan(&returnedID)

		if err != nil {
			if err == sql.ErrNoRows {
				// Conflict - workflow already exists
				return interfaces.ErrDuplicate
			}
			return r.TranslateError(err, "workflow")
		}

		// Clear tenant cache
		cacheKey := fmt.Sprintf("workflows:tenant:%s", workflow.TenantID)
		if err := r.CacheDelete(ctx, cacheKey); err != nil {
			r.logger.Warn("Failed to clear tenant cache", map[string]interface{}{
				"error": err.Error(),
				"key":   cacheKey,
			})
		}

		r.logger.Info("Workflow created", map[string]interface{}{
			"workflow_id": workflow.ID,
			"tenant_id":   workflow.TenantID,
			"name":        workflow.Name,
		})

		return nil
	})
}

// Get retrieves a workflow by ID
func (r *workflowRepository) Get(ctx context.Context, id uuid.UUID) (*models.Workflow, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.Get")
	defer span.End()

	// Try cache first
	cacheKey := fmt.Sprintf("workflow:%s", id)
	var workflow models.Workflow
	err := r.CacheGet(ctx, cacheKey, &workflow)
	if err == nil {
		r.metrics.IncrementCounter("workflow_cache_hits", 1)
		return &workflow, nil
	}

	r.metrics.IncrementCounter("workflow_cache_misses", 1)

	// Query database using read replica
	err = r.ExecuteQuery(ctx, "get_workflow", func(ctx context.Context) error {
		query := `
			SELECT 
				id, tenant_id, name, description, type,
				version, created_by, agents, steps,
				config, tags, is_active, created_at, updated_at, deleted_at
			FROM workflows
			WHERE id = $1 AND deleted_at IS NULL`

		return r.readDB.GetContext(ctx, &workflow, query, id)
	})

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, interfaces.ErrNotFound
		}
		return nil, errors.Wrap(err, "failed to get workflow")
	}

	// Cache the result for 5 minutes
	if err := r.CacheSet(ctx, cacheKey, &workflow, 5*time.Minute); err != nil {
		r.logger.Warn("Failed to cache workflow", map[string]interface{}{
			"error": err.Error(),
			"key":   cacheKey,
		})
	}

	return &workflow, nil
}

// GetByName retrieves a workflow by name
func (r *workflowRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Workflow, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.GetByName")
	defer span.End()

	// Try cache first
	cacheKey := fmt.Sprintf("workflow:tenant:%s:name:%s", tenantID, name)
	var workflow models.Workflow
	err := r.CacheGet(ctx, cacheKey, &workflow)
	if err == nil {
		r.metrics.IncrementCounter("workflow_cache_hits", 1)
		return &workflow, nil
	}

	r.metrics.IncrementCounter("workflow_cache_misses", 1)

	// Query database with tenant isolation
	err = r.ExecuteQuery(ctx, "get_workflow_by_name", func(ctx context.Context) error {
		query := `
			SELECT 
				id, tenant_id, name, description, type,
				version, created_by, agents, steps,
				config, tags, is_active, created_at, updated_at, deleted_at
			FROM workflows
			WHERE tenant_id = $1 AND name = $2 AND deleted_at IS NULL`

		return r.readDB.GetContext(ctx, &workflow, query, tenantID, name)
	})

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, interfaces.ErrNotFound
		}
		return nil, errors.Wrap(err, "failed to get workflow by name")
	}

	// Cache the result
	if err := r.CacheSet(ctx, cacheKey, &workflow, 5*time.Minute); err != nil {
		r.logger.Warn("Failed to cache workflow", map[string]interface{}{
			"error": err.Error(),
			"key":   cacheKey,
		})
	}

	return &workflow, nil
}

// Update updates a workflow
func (r *workflowRepository) Update(ctx context.Context, workflow *models.Workflow) error {
	ctx, span := r.tracer(ctx, "WorkflowRepository.Update")
	defer span.End()

	return r.ExecuteQuery(ctx, "update_workflow", func(ctx context.Context) error {
		// Store old version for optimistic locking
		oldVersion := workflow.Version

		// Increment version and update timestamp
		workflow.Version++
		workflow.UpdatedAt = time.Now()

		// Prepare the update query with optimistic locking
		query := `
			UPDATE workflows SET 
				name = $1,
				description = $2,
				type = $3,
				agents = $4,
				steps = $5,
				config = $6,
				tags = $7,
				is_active = $8,
				updated_at = $9,
				version = $10
			WHERE id = $11 AND version = $12 AND deleted_at IS NULL`

		// Execute the update
		result, err := r.writeDB.ExecContext(ctx, query,
			workflow.Name,
			workflow.Description,
			workflow.Type,
			workflow.Agents,
			workflow.Steps,
			workflow.Config,
			workflow.Tags,
			workflow.IsActive,
			workflow.UpdatedAt,
			workflow.Version,
			workflow.ID,
			oldVersion,
		)
		if err != nil {
			return r.TranslateError(err, "workflow")
		}

		// Check if any rows were affected
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}

		if rowsAffected == 0 {
			// Check if workflow exists
			var exists bool
			err = r.readDB.GetContext(ctx, &exists,
				"SELECT EXISTS(SELECT 1 FROM workflows WHERE id = $1 AND deleted_at IS NULL)",
				workflow.ID)
			if err != nil {
				return errors.Wrap(err, "failed to check workflow existence")
			}
			if !exists {
				return interfaces.ErrNotFound
			}
			// Version mismatch
			return interfaces.ErrOptimisticLock
		}

		// Invalidate cache
		cacheKeys := []string{
			fmt.Sprintf("workflow:%s", workflow.ID),
			fmt.Sprintf("workflows:tenant:%s", workflow.TenantID),
			fmt.Sprintf("workflow:tenant:%s:name:%s", workflow.TenantID, workflow.Name),
		}
		for _, key := range cacheKeys {
			if err := r.CacheDelete(ctx, key); err != nil {
				r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
					"error": err.Error(),
					"key":   key,
				})
			}
		}

		r.logger.Info("Workflow updated", map[string]interface{}{
			"workflow_id": workflow.ID,
			"version":     workflow.Version,
		})

		return nil
	})
}

// Delete deletes a workflow
func (r *workflowRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer(ctx, "WorkflowRepository.Delete")
	defer span.End()

	return r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		// Check for active executions first
		var activeCount int
		checkQuery := `
			SELECT COUNT(*) 
			FROM workflow_executions 
			WHERE workflow_id = $1 
			AND status IN ('pending', 'running', 'paused')`

		err := tx.GetContext(ctx, &activeCount, checkQuery, id)
		if err != nil {
			return errors.Wrap(err, "failed to check active executions")
		}

		if activeCount > 0 {
			return errors.Errorf("cannot delete workflow with %d active executions", activeCount)
		}

		// Get workflow info for cache invalidation
		var workflow models.Workflow
		getQuery := `SELECT tenant_id, name FROM workflows WHERE id = $1 AND deleted_at IS NULL`
		err = tx.GetContext(ctx, &workflow, getQuery, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return interfaces.ErrNotFound
			}
			return errors.Wrap(err, "failed to get workflow")
		}

		// Perform hard delete
		deleteQuery := `DELETE FROM workflows WHERE id = $1`
		result, err := tx.ExecContext(ctx, deleteQuery, id)
		if err != nil {
			return r.TranslateError(err, "workflow")
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}

		if rowsAffected == 0 {
			return interfaces.ErrNotFound
		}

		// Invalidate all related caches
		cacheKeys := []string{
			fmt.Sprintf("workflow:%s", id),
			fmt.Sprintf("workflows:tenant:%s", workflow.TenantID),
			fmt.Sprintf("workflow:tenant:%s:name:%s", workflow.TenantID, workflow.Name),
			fmt.Sprintf("workflow:executions:%s", id),
			fmt.Sprintf("workflow:stats:%s", id),
		}

		for _, key := range cacheKeys {
			if err := r.CacheDelete(ctx, key); err != nil {
				r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
					"error": err.Error(),
					"key":   key,
				})
			}
		}

		r.logger.Info("Workflow deleted", map[string]interface{}{
			"workflow_id": id,
		})

		return nil
	})
}

// List retrieves workflows for a specific tenant
func (r *workflowRepository) List(ctx context.Context, tenantID uuid.UUID, filters interfaces.WorkflowFilters) ([]*models.Workflow, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.List")
	defer span.End()

	var workflows []*models.Workflow

	err := r.ExecuteQuery(ctx, "list_workflows", func(ctx context.Context) error {
		// Build dynamic query based on filters
		query := `
			SELECT 
				id, tenant_id, name, description, type,
				version, created_by, agents, steps,
				config, tags, is_active, created_at, updated_at, deleted_at
			FROM workflows
			WHERE tenant_id = $1 AND deleted_at IS NULL`

		args := []interface{}{tenantID}
		argIndex := 2

		// Apply filters
		if len(filters.Type) > 0 {
			query += fmt.Sprintf(" AND type = ANY($%d)", argIndex)
			args = append(args, pq.Array(filters.Type))
			argIndex++
		}

		if filters.IsActive != nil {
			query += fmt.Sprintf(" AND is_active = $%d", argIndex)
			args = append(args, *filters.IsActive)
			argIndex++
		}

		if filters.CreatedBy != nil {
			query += fmt.Sprintf(" AND created_by = $%d", argIndex)
			args = append(args, *filters.CreatedBy)
			argIndex++
		}

		if filters.CreatedAfter != nil {
			query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
			args = append(args, *filters.CreatedAfter)
			argIndex++
		}

		if filters.CreatedBefore != nil {
			query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
			args = append(args, *filters.CreatedBefore)
			argIndex++
		}

		if len(filters.Tags) > 0 {
			query += fmt.Sprintf(" AND tags && $%d", argIndex) // Array overlap operator
			args = append(args, pq.Array(filters.Tags))
			argIndex++
		}

		// Apply sorting
		sortBy := "created_at"
		if filters.SortBy != "" {
			// Validate sort field to prevent SQL injection
			validSortFields := map[string]bool{
				"created_at": true,
				"updated_at": true,
				"name":       true,
				"type":       true,
			}
			if validSortFields[filters.SortBy] {
				sortBy = filters.SortBy
			}
		}

		sortOrder := "DESC"
		if filters.SortOrder == types.SortAsc {
			sortOrder = "ASC"
		}

		query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

		// Apply pagination
		if filters.Limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", argIndex)
			args = append(args, filters.Limit)
			argIndex++
		}

		if filters.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIndex)
			args = append(args, filters.Offset)
		}

		return r.readDB.SelectContext(ctx, &workflows, query, args...)
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to list workflows")
	}

	return workflows, nil
}

// ListByType retrieves workflows by type
func (r *workflowRepository) ListByType(ctx context.Context, workflowType string) ([]*models.Workflow, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.ListByType")
	defer span.End()

	var workflows []*models.Workflow

	err := r.ExecuteQuery(ctx, "list_workflows_by_type", func(ctx context.Context) error {
		query := `
			SELECT 
				id, tenant_id, name, description, type,
				version, created_by, agents, steps,
				config, tags, is_active, created_at, updated_at, deleted_at
			FROM workflows
			WHERE type = $1 AND deleted_at IS NULL AND is_active = true
			ORDER BY created_at DESC`

		return r.readDB.SelectContext(ctx, &workflows, query, workflowType)
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to list workflows by type")
	}

	return workflows, nil
}

// SearchWorkflows searches workflows by query
func (r *workflowRepository) SearchWorkflows(ctx context.Context, searchQuery string, filters interfaces.WorkflowFilters) ([]*models.Workflow, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.SearchWorkflows")
	defer span.End()

	var workflows []*models.Workflow

	err := r.ExecuteQuery(ctx, "search_workflows", func(ctx context.Context) error {
		// Build search query with full-text search
		query := `
			SELECT 
				id, tenant_id, name, description, type,
				version, created_by, agents, steps,
				config, tags, is_active, created_at, updated_at, deleted_at
			FROM workflows
			WHERE deleted_at IS NULL
			AND (
				name ILIKE $1 
				OR description ILIKE $1
				OR $1 = ANY(tags)
			)`

		// Prepare search pattern
		searchPattern := "%" + searchQuery + "%"
		args := []interface{}{searchPattern}
		argIndex := 2

		// Apply additional filters
		if len(filters.Type) > 0 {
			query += fmt.Sprintf(" AND type = ANY($%d)", argIndex)
			args = append(args, pq.Array(filters.Type))
			argIndex++
		}

		if filters.IsActive != nil {
			query += fmt.Sprintf(" AND is_active = $%d", argIndex)
			args = append(args, *filters.IsActive)
			argIndex++
		}

		if filters.CreatedBy != nil {
			query += fmt.Sprintf(" AND created_by = $%d", argIndex)
			args = append(args, *filters.CreatedBy)
			argIndex++
		}

		if filters.CreatedAfter != nil {
			query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
			args = append(args, *filters.CreatedAfter)
			argIndex++
		}

		if filters.CreatedBefore != nil {
			query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
			args = append(args, *filters.CreatedBefore)
			argIndex++
		}

		// Apply sorting
		sortBy := "created_at"
		if filters.SortBy != "" {
			validSortFields := map[string]bool{
				"created_at": true,
				"updated_at": true,
				"name":       true,
				"type":       true,
			}
			if validSortFields[filters.SortBy] {
				sortBy = filters.SortBy
			}
		}

		sortOrder := "DESC"
		if filters.SortOrder == types.SortAsc {
			sortOrder = "ASC"
		}

		query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

		// Apply pagination
		if filters.Limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", argIndex)
			args = append(args, filters.Limit)
			argIndex++
		}

		if filters.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIndex)
			args = append(args, filters.Offset)
		}

		return r.readDB.SelectContext(ctx, &workflows, query, args...)
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to search workflows")
	}

	return workflows, nil
}

// SoftDelete soft deletes a workflow
func (r *workflowRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer(ctx, "WorkflowRepository.SoftDelete")
	defer span.End()

	return r.ExecuteQueryWithRetry(ctx, "soft_delete_workflow", func(ctx context.Context) error {
		now := time.Now()

		// Get workflow info for cache invalidation
		var workflow models.Workflow
		getQuery := `SELECT tenant_id, name FROM workflows WHERE id = $1 AND deleted_at IS NULL`
		err := r.readDB.GetContext(ctx, &workflow, getQuery, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return interfaces.ErrNotFound
			}
			return errors.Wrap(err, "failed to get workflow")
		}

		// Soft delete by setting deleted_at timestamp
		query := `
			UPDATE workflows 
			SET deleted_at = $1, updated_at = $1
			WHERE id = $2 AND deleted_at IS NULL`

		result, err := r.writeDB.ExecContext(ctx, query, now, id)
		if err != nil {
			return r.TranslateError(err, "workflow")
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}

		if rowsAffected == 0 {
			return interfaces.ErrNotFound
		}

		// Invalidate cache
		cacheKeys := []string{
			fmt.Sprintf("workflow:%s", id),
			fmt.Sprintf("workflows:tenant:%s", workflow.TenantID),
			fmt.Sprintf("workflow:tenant:%s:name:%s", workflow.TenantID, workflow.Name),
		}

		for _, key := range cacheKeys {
			if err := r.CacheDelete(ctx, key); err != nil {
				r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
					"error": err.Error(),
					"key":   key,
				})
			}
		}

		r.logger.Info("Workflow soft deleted", map[string]interface{}{
			"workflow_id": id,
		})

		return nil
	})
}

// CreateExecution creates a workflow execution record
func (r *workflowRepository) CreateExecution(ctx context.Context, execution *models.WorkflowExecution) error {
	ctx, span := r.tracer(ctx, "WorkflowRepository.CreateExecution")
	defer span.End()

	return r.ExecuteQueryWithRetry(ctx, "create_execution", func(ctx context.Context) error {
		// Generate ID and timestamps if not provided
		if execution.ID == uuid.Nil {
			execution.ID = uuid.New()
		}
		now := time.Now()
		execution.StartedAt = now
		execution.UpdatedAt = now
		if execution.Status == "" {
			execution.Status = models.WorkflowStatusPending
		}

		// Marshal context and state to JSON
		contextJSON, err := json.Marshal(execution.Context)
		if err != nil {
			return errors.Wrap(err, "failed to marshal context")
		}

		stateJSON, err := json.Marshal(execution.State)
		if err != nil {
			return errors.Wrap(err, "failed to marshal state")
		}

		// Prepare the query
		query := `
			INSERT INTO workflow_executions (
				id, workflow_id, tenant_id, status,
				context, state, initiated_by, error,
				started_at, updated_at
			) VALUES (
				$1, $2, $3, $4,
				$5, $6, $7, $8,
				$9, $10
			)
			ON CONFLICT (id) DO NOTHING
			RETURNING id`

		// Execute the query
		var returnedID uuid.UUID
		err = r.writeDB.QueryRowContext(ctx, query,
			execution.ID,
			execution.WorkflowID,
			execution.TenantID,
			execution.Status,
			contextJSON,
			stateJSON,
			execution.InitiatedBy,
			execution.Error,
			execution.StartedAt,
			execution.UpdatedAt,
		).Scan(&returnedID)

		if err != nil {
			if err == sql.ErrNoRows {
				// Conflict - execution already exists
				return interfaces.ErrDuplicate
			}
			return r.TranslateError(err, "workflow_execution")
		}

		// Clear workflow executions cache
		cacheKey := fmt.Sprintf("workflow:executions:%s", execution.WorkflowID)
		if err := r.CacheDelete(ctx, cacheKey); err != nil {
			r.logger.Warn("Failed to clear executions cache", map[string]interface{}{
				"error": err.Error(),
				"key":   cacheKey,
			})
		}

		r.logger.Info("Workflow execution created", map[string]interface{}{
			"execution_id": execution.ID,
			"workflow_id":  execution.WorkflowID,
			"status":       execution.Status,
		})

		return nil
	})
}

// GetExecution retrieves a workflow execution
func (r *workflowRepository) GetExecution(ctx context.Context, id uuid.UUID) (*models.WorkflowExecution, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.GetExecution")
	defer span.End()

	// Try cache first
	cacheKey := fmt.Sprintf("execution:%s", id)
	var execution models.WorkflowExecution
	err := r.CacheGet(ctx, cacheKey, &execution)
	if err == nil {
		r.metrics.IncrementCounter("execution_cache_hits", 1)
		return &execution, nil
	}

	r.metrics.IncrementCounter("execution_cache_misses", 1)

	// Query database
	err = r.ExecuteQuery(ctx, "get_execution", func(ctx context.Context) error {
		query := `
			SELECT 
				id, workflow_id, tenant_id, status,
				context, state, initiated_by, error,
				started_at, completed_at, updated_at
			FROM workflow_executions
			WHERE id = $1`

		return r.readDB.GetContext(ctx, &execution, query, id)
	})

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, interfaces.ErrNotFound
		}
		return nil, errors.Wrap(err, "failed to get workflow execution")
	}

	// Cache the result
	if err := r.CacheSet(ctx, cacheKey, &execution, 5*time.Minute); err != nil {
		r.logger.Warn("Failed to cache execution", map[string]interface{}{
			"error": err.Error(),
			"key":   cacheKey,
		})
	}

	return &execution, nil
}

// UpdateExecution updates a workflow execution
func (r *workflowRepository) UpdateExecution(ctx context.Context, execution *models.WorkflowExecution) error {
	ctx, span := r.tracer(ctx, "WorkflowRepository.UpdateExecution")
	defer span.End()

	return r.ExecuteQueryWithRetry(ctx, "update_execution", func(ctx context.Context) error {
		// Update timestamp
		execution.UpdatedAt = time.Now()

		// Marshal context and state to JSON
		contextJSON, err := json.Marshal(execution.Context)
		if err != nil {
			return errors.Wrap(err, "failed to marshal context")
		}

		stateJSON, err := json.Marshal(execution.State)
		if err != nil {
			return errors.Wrap(err, "failed to marshal state")
		}

		// Prepare the update query
		query := `
			UPDATE workflow_executions SET 
				status = $1,
				context = $2,
				state = $3,
				error = $4,
				completed_at = $5,
				updated_at = $6
			WHERE id = $7`

		// Execute the update
		result, err := r.writeDB.ExecContext(ctx, query,
			execution.Status,
			contextJSON,
			stateJSON,
			execution.Error,
			execution.CompletedAt,
			execution.UpdatedAt,
			execution.ID,
		)
		if err != nil {
			return r.TranslateError(err, "workflow_execution")
		}

		// Check if any rows were affected
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}

		if rowsAffected == 0 {
			return interfaces.ErrNotFound
		}

		// Invalidate cache
		cacheKeys := []string{
			fmt.Sprintf("execution:%s", execution.ID),
			fmt.Sprintf("workflow:executions:%s", execution.WorkflowID),
			fmt.Sprintf("workflow:stats:%s", execution.WorkflowID),
		}

		for _, key := range cacheKeys {
			if err := r.CacheDelete(ctx, key); err != nil {
				r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
					"error": err.Error(),
					"key":   key,
				})
			}
		}

		r.logger.Info("Workflow execution updated", map[string]interface{}{
			"execution_id": execution.ID,
			"status":       execution.Status,
		})

		return nil
	})
}

// GetActiveExecutions retrieves active workflow executions
func (r *workflowRepository) GetActiveExecutions(ctx context.Context, workflowID uuid.UUID) ([]*models.WorkflowExecution, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.GetActiveExecutions")
	defer span.End()

	var executions []*models.WorkflowExecution

	err := r.ExecuteQuery(ctx, "get_active_executions", func(ctx context.Context) error {
		query := `
			SELECT 
				id, workflow_id, tenant_id, status,
				context, state, initiated_by, error,
				started_at, completed_at, updated_at
			FROM workflow_executions
			WHERE workflow_id = $1 
			AND status IN ('pending', 'running', 'paused')
			ORDER BY started_at DESC`

		return r.readDB.SelectContext(ctx, &executions, query, workflowID)
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to get active executions")
	}

	return executions, nil
}

// ListExecutions retrieves all executions for a workflow
func (r *workflowRepository) ListExecutions(ctx context.Context, workflowID uuid.UUID, limit int) ([]*models.WorkflowExecution, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.ListExecutions")
	defer span.End()

	// Try cache first
	cacheKey := fmt.Sprintf("workflow:executions:%s:%d", workflowID, limit)
	var executions []*models.WorkflowExecution
	err := r.CacheGet(ctx, cacheKey, &executions)
	if err == nil {
		r.metrics.IncrementCounter("execution_list_cache_hits", 1)
		return executions, nil
	}

	r.metrics.IncrementCounter("execution_list_cache_misses", 1)

	err = r.ExecuteQuery(ctx, "list_executions", func(ctx context.Context) error {
		query := `
			SELECT 
				id, workflow_id, tenant_id, status,
				context, state, initiated_by, error,
				started_at, completed_at, updated_at
			FROM workflow_executions
			WHERE workflow_id = $1
			ORDER BY started_at DESC`

		if limit > 0 {
			query += " LIMIT $2"
			return r.readDB.SelectContext(ctx, &executions, query, workflowID, limit)
		}

		return r.readDB.SelectContext(ctx, &executions, query, workflowID)
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to list executions")
	}

	// Cache the result for a shorter duration
	if err := r.CacheSet(ctx, cacheKey, executions, 1*time.Minute); err != nil {
		r.logger.Warn("Failed to cache execution list", map[string]interface{}{
			"error": err.Error(),
			"key":   cacheKey,
		})
	}

	return executions, nil
}

// UpdateStepStatus updates the status of a workflow step
func (r *workflowRepository) UpdateStepStatus(ctx context.Context, executionID uuid.UUID, stepID string, status string, output map[string]interface{}) error {
	ctx, span := r.tracer(ctx, "WorkflowRepository.UpdateStepStatus")
	defer span.End()

	return r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		// First, get the current execution to update its state
		var execution models.WorkflowExecution
		getQuery := `
			SELECT id, workflow_id, state 
			FROM workflow_executions 
			WHERE id = $1`

		err := tx.GetContext(ctx, &execution, getQuery, executionID)
		if err != nil {
			if err == sql.ErrNoRows {
				return interfaces.ErrNotFound
			}
			return errors.Wrap(err, "failed to get execution")
		}

		// Update or create step status in state JSON
		if execution.State == nil {
			execution.State = make(models.JSONMap)
		}

		stepStatuses, ok := execution.State["step_statuses"].(map[string]interface{})
		if !ok {
			stepStatuses = make(map[string]interface{})
			execution.State["step_statuses"] = stepStatuses
		}

		// Create step status entry
		stepStatus := map[string]interface{}{
			"step_id":      stepID,
			"status":       status,
			"output":       output,
			"completed_at": time.Now(),
		}
		stepStatuses[stepID] = stepStatus

		// Marshal updated state
		stateJSON, err := json.Marshal(execution.State)
		if err != nil {
			return errors.Wrap(err, "failed to marshal state")
		}

		// Update execution with new state
		updateQuery := `
			UPDATE workflow_executions 
			SET state = $1, updated_at = $2
			WHERE id = $3`

		_, err = tx.ExecContext(ctx, updateQuery, stateJSON, time.Now(), executionID)
		if err != nil {
			return errors.Wrap(err, "failed to update execution state")
		}

		// Invalidate cache
		cacheKeys := []string{
			fmt.Sprintf("execution:%s", executionID),
			fmt.Sprintf("workflow:executions:%s", execution.WorkflowID),
		}

		for _, key := range cacheKeys {
			if err := r.CacheDelete(ctx, key); err != nil {
				r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
					"error": err.Error(),
					"key":   key,
				})
			}
		}

		r.logger.Info("Step status updated", map[string]interface{}{
			"execution_id": executionID,
			"step_id":      stepID,
			"status":       status,
		})

		return nil
	})
}

// GetWorkflowStats retrieves workflow execution statistics
func (r *workflowRepository) GetWorkflowStats(ctx context.Context, workflowID uuid.UUID, period time.Duration) (*interfaces.WorkflowStats, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.GetWorkflowStats")
	defer span.End()

	// Try cache first
	cacheKey := fmt.Sprintf("workflow:stats:%s:%s", workflowID, period.String())
	var stats interfaces.WorkflowStats
	err := r.CacheGet(ctx, cacheKey, &stats)
	if err == nil {
		r.metrics.IncrementCounter("workflow_stats_cache_hits", 1)
		return &stats, nil
	}

	r.metrics.IncrementCounter("workflow_stats_cache_misses", 1)

	err = r.ExecuteQuery(ctx, "get_workflow_stats", func(ctx context.Context) error {
		startTime := time.Now().Add(-period)

		// Get basic counts
		countQuery := `
			SELECT 
				COUNT(*) as total_runs,
				COUNT(CASE WHEN status = 'completed' THEN 1 END) as successful_runs,
				COUNT(CASE WHEN status IN ('failed', 'timeout') THEN 1 END) as failed_runs
			FROM workflow_executions
			WHERE workflow_id = $1 AND started_at >= $2`

		err := r.readDB.QueryRowContext(ctx, countQuery, workflowID, startTime).Scan(
			&stats.TotalRuns,
			&stats.SuccessfulRuns,
			&stats.FailedRuns,
		)
		if err != nil {
			return errors.Wrap(err, "failed to get basic stats")
		}

		// Get average and P95 runtime for completed executions
		timingQuery := `
			SELECT 
				COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0) as avg_runtime,
				COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - started_at))), 0) as p95_runtime
			FROM workflow_executions
			WHERE workflow_id = $1 
			AND started_at >= $2
			AND completed_at IS NOT NULL
			AND status = 'completed'`

		var avgRuntimeSeconds, p95RuntimeSeconds float64
		err = r.readDB.QueryRowContext(ctx, timingQuery, workflowID, startTime).Scan(
			&avgRuntimeSeconds,
			&p95RuntimeSeconds,
		)
		if err != nil {
			return errors.Wrap(err, "failed to get timing stats")
		}

		stats.AverageRuntime = time.Duration(avgRuntimeSeconds * float64(time.Second))
		stats.P95Runtime = time.Duration(p95RuntimeSeconds * float64(time.Second))

		// Get status breakdown
		statusQuery := `
			SELECT status, COUNT(*) as count
			FROM workflow_executions
			WHERE workflow_id = $1 AND started_at >= $2
			GROUP BY status`

		rows, err := r.readDB.QueryContext(ctx, statusQuery, workflowID, startTime)
		if err != nil {
			return errors.Wrap(err, "failed to get status breakdown")
		}
		defer func() {
			if err := rows.Close(); err != nil {
				r.logger.Error("Failed to close rows", map[string]interface{}{"error": err.Error()})
			}
		}()

		stats.ByStatus = make(map[string]int64)
		for rows.Next() {
			var status string
			var count int64
			if err := rows.Scan(&status, &count); err != nil {
				return errors.Wrap(err, "failed to scan status row")
			}
			stats.ByStatus[status] = count
		}

		return rows.Err()
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to get workflow stats")
	}

	// Cache the result for 5 minutes
	if err := r.CacheSet(ctx, cacheKey, &stats, 5*time.Minute); err != nil {
		r.logger.Warn("Failed to cache workflow stats", map[string]interface{}{
			"error": err.Error(),
			"key":   cacheKey,
		})
	}

	return &stats, nil
}

// ArchiveOldExecutions archives old workflow executions
func (r *workflowRepository) ArchiveOldExecutions(ctx context.Context, before time.Time) (int64, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.ArchiveOldExecutions")
	defer span.End()

	var totalArchived int64

	err := r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		// First, copy old executions to archive table (if it exists)
		archiveQuery := `
			INSERT INTO workflow_executions_archive 
			SELECT * FROM workflow_executions 
			WHERE completed_at < $1 
			AND status IN ('completed', 'failed', 'cancelled', 'timeout')
			ON CONFLICT (id) DO NOTHING`

		// Try to archive (ignore error if archive table doesn't exist)
		_, archiveErr := tx.ExecContext(ctx, archiveQuery, before)
		if archiveErr != nil {
			// Log but don't fail if archive table doesn't exist
			r.logger.Warn("Failed to archive executions (archive table may not exist)", map[string]interface{}{
				"error": archiveErr.Error(),
			})
		}

		// Delete old executions
		deleteQuery := `
			DELETE FROM workflow_executions 
			WHERE completed_at < $1 
			AND status IN ('completed', 'failed', 'cancelled', 'timeout')`

		result, err := tx.ExecContext(ctx, deleteQuery, before)
		if err != nil {
			return errors.Wrap(err, "failed to delete old executions")
		}

		totalArchived, err = result.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}

		// Clear all execution-related caches
		if err := r.InvalidateCachePattern(ctx, "execution:*"); err != nil {
			r.logger.Warn("Failed to clear execution caches", map[string]interface{}{
				"error": err.Error(),
			})
		}

		if err := r.InvalidateCachePattern(ctx, "workflow:executions:*"); err != nil {
			r.logger.Warn("Failed to clear workflow execution caches", map[string]interface{}{
				"error": err.Error(),
			})
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	r.logger.Info("Archived old executions", map[string]interface{}{
		"count":  totalArchived,
		"before": before,
	})

	return totalArchived, nil
}

// GetExecutionTimeline retrieves the timeline of events for a workflow execution
func (r *workflowRepository) GetExecutionTimeline(ctx context.Context, executionID uuid.UUID) ([]*models.ExecutionEvent, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.GetExecutionTimeline")
	defer span.End()

	var events []*models.ExecutionEvent

	err := r.ExecuteQuery(ctx, "get_execution_timeline", func(ctx context.Context) error {
		// Get execution details and construct timeline from state
		var execution models.WorkflowExecution
		query := `
			SELECT id, workflow_id, state, started_at, completed_at, status, initiated_by
			FROM workflow_executions
			WHERE id = $1`

		err := r.readDB.GetContext(ctx, &execution, query, executionID)
		if err != nil {
			if err == sql.ErrNoRows {
				return interfaces.ErrNotFound
			}
			return errors.Wrap(err, "failed to get execution")
		}

		// Start event
		events = append(events, &models.ExecutionEvent{
			Timestamp:   execution.StartedAt,
			EventType:   "execution_started",
			Description: fmt.Sprintf("Workflow execution started by %s", execution.InitiatedBy),
			Details: map[string]interface{}{
				"status":       execution.Status,
				"initiated_by": execution.InitiatedBy,
			},
		})

		// Extract step events from state
		if execution.State != nil {
			if stepStatuses, ok := execution.State["step_statuses"].(map[string]interface{}); ok {
				for stepID, statusData := range stepStatuses {
					if stepInfo, ok := statusData.(map[string]interface{}); ok {
						// Step started event
						if startedAt, ok := stepInfo["started_at"].(string); ok {
							if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
								events = append(events, &models.ExecutionEvent{
									Timestamp:   t,
									EventType:   "step_started",
									StepID:      stepID,
									Description: fmt.Sprintf("Step %s started", stepID),
									Details:     stepInfo,
								})
							}
						}

						// Step completed event
						if completedAt, ok := stepInfo["completed_at"].(string); ok {
							if t, err := time.Parse(time.RFC3339, completedAt); err == nil {
								status := "completed"
								if s, ok := stepInfo["status"].(string); ok {
									status = s
								}
								events = append(events, &models.ExecutionEvent{
									Timestamp:   t,
									EventType:   "step_" + status,
									StepID:      stepID,
									Description: fmt.Sprintf("Step %s %s", stepID, status),
									Details:     stepInfo,
								})
							}
						}
					}
				}
			}
		}

		// Completion event
		if execution.CompletedAt != nil {
			events = append(events, &models.ExecutionEvent{
				Timestamp:   *execution.CompletedAt,
				EventType:   "execution_completed",
				Description: fmt.Sprintf("Workflow execution completed with status: %s", execution.Status),
				Details: map[string]interface{}{
					"status":   execution.Status,
					"duration": execution.CompletedAt.Sub(execution.StartedAt).String(),
				},
			})
		}

		// Sort events by timestamp
		sort.Slice(events, func(i, j int) bool {
			return events[i].Timestamp.Before(events[j].Timestamp)
		})

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to get execution timeline")
	}

	return events, nil
}

// GetStepStatus retrieves the status of a specific step in a workflow execution
func (r *workflowRepository) GetStepStatus(ctx context.Context, executionID uuid.UUID, stepID string) (*models.StepStatus, error) {
	ctx, span := r.tracer(ctx, "WorkflowRepository.GetStepStatus")
	defer span.End()

	var stepStatus models.StepStatus

	err := r.ExecuteQuery(ctx, "get_step_status", func(ctx context.Context) error {
		// Get execution state
		var execution models.WorkflowExecution
		query := `
			SELECT state 
			FROM workflow_executions 
			WHERE id = $1`

		err := r.readDB.GetContext(ctx, &execution, query, executionID)
		if err != nil {
			if err == sql.ErrNoRows {
				return interfaces.ErrNotFound
			}
			return errors.Wrap(err, "failed to get execution")
		}

		// Extract step status from state
		if execution.State != nil {
			if stepStatuses, ok := execution.State["step_statuses"].(map[string]interface{}); ok {
				if stepData, ok := stepStatuses[stepID].(map[string]interface{}); ok {
					// Populate StepStatus struct
					stepStatus.StepID = stepID

					if status, ok := stepData["status"].(string); ok {
						stepStatus.Status = status
					}

					if agentID, ok := stepData["agent_id"].(string); ok {
						stepStatus.AgentID = agentID
					}

					if input, ok := stepData["input"].(map[string]interface{}); ok {
						stepStatus.Input = input
					}

					if output, ok := stepData["output"].(map[string]interface{}); ok {
						stepStatus.Output = output
					}

					if errStr, ok := stepData["error"].(string); ok {
						stepStatus.Error = errStr
					}

					if retryCount, ok := stepData["retry_count"].(float64); ok {
						stepStatus.RetryCount = int(retryCount)
					}

					// Parse timestamps
					if startedAt, ok := stepData["started_at"].(string); ok {
						if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
							stepStatus.StartedAt = &t
						}
					}

					if completedAt, ok := stepData["completed_at"].(string); ok {
						if t, err := time.Parse(time.RFC3339, completedAt); err == nil {
							stepStatus.CompletedAt = &t
						}
					}

					return nil
				}
			}
		}

		return errors.Errorf("step %s not found in execution %s", stepID, executionID)
	})

	if err != nil {
		return nil, err
	}

	return &stepStatus, nil
}

// ValidateWorkflowIntegrity validates the integrity of a workflow
func (r *workflowRepository) ValidateWorkflowIntegrity(ctx context.Context, workflowID uuid.UUID) error {
	ctx, span := r.tracer(ctx, "WorkflowRepository.ValidateWorkflowIntegrity")
	defer span.End()

	return r.ExecuteQuery(ctx, "validate_workflow_integrity", func(ctx context.Context) error {
		// Get workflow
		workflow, err := r.Get(ctx, workflowID)
		if err != nil {
			return err
		}

		// Validate workflow structure
		if workflow.Name == "" {
			return errors.New("workflow name cannot be empty")
		}

		if workflow.Type == "" {
			return errors.New("workflow type cannot be empty")
		}

		// Validate workflow type
		validTypes := map[models.WorkflowType]bool{
			models.WorkflowTypeSequential:    true,
			models.WorkflowTypeParallel:      true,
			models.WorkflowTypeConditional:   true,
			models.WorkflowTypeCollaborative: true,
		}
		if !validTypes[workflow.Type] {
			return errors.Errorf("invalid workflow type: %s", workflow.Type)
		}

		// Validate agents configuration
		if len(workflow.Agents) == 0 {
			return errors.New("workflow must have at least one agent")
		}

		// Validate steps configuration
		if len(workflow.Steps) == 0 {
			return errors.New("workflow must have at least one step")
		}

		// Validate step references
		for stepID, stepData := range workflow.Steps {
			if step, ok := stepData.(map[string]interface{}); ok {
				// Check if step has required fields
				if _, hasType := step["type"]; !hasType {
					return errors.Errorf("step %s missing type", stepID)
				}

				// Check agent reference
				if agentID, hasAgent := step["agent_id"].(string); hasAgent {
					if _, agentExists := workflow.Agents[agentID]; !agentExists {
						return errors.Errorf("step %s references non-existent agent %s", stepID, agentID)
					}
				}
			}
		}

		// Check for orphaned executions
		var orphanedCount int
		orphanQuery := `
			SELECT COUNT(*) 
			FROM workflow_executions 
			WHERE workflow_id = $1 
			AND status IN ('running', 'paused') 
			AND updated_at < NOW() - INTERVAL '24 hours'`

		err = r.readDB.GetContext(ctx, &orphanedCount, orphanQuery, workflowID)
		if err != nil {
			return errors.Wrap(err, "failed to check orphaned executions")
		}

		if orphanedCount > 0 {
			r.logger.Warn("Workflow has orphaned executions", map[string]interface{}{
				"workflow_id": workflowID,
				"count":       orphanedCount,
			})
		}

		r.logger.Info("Workflow integrity validated", map[string]interface{}{
			"workflow_id": workflowID,
		})

		return nil
	})
}
