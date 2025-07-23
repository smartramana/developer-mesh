package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
)

// SoftDelete marks a task as deleted
func (r *taskRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer(ctx, "TaskRepository.SoftDelete")
	defer span.End()

	_, err := r.ExecuteWithCircuitBreaker(ctx, "task_soft_delete", func() (interface{}, error) {
		return nil, r.doSoftDelete(ctx, id)
	})
	return err
}

func (r *taskRepository) doSoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tasks SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.writeDB.ExecContext(ctx, query, id)
	if err != nil {
		r.metrics.IncrementCounterWithLabels("repository_errors", 1, map[string]string{"operation": "soft_delete", "error_type": classifyDBError(err)})
		return errors.Wrap(err, "failed to soft delete task")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return types.ErrNotFound
	}

	// Invalidate cache
	_ = r.cache.Delete(ctx, fmt.Sprintf("task:%s", id))

	r.metrics.IncrementCounterWithLabels("repository_queries", 1, map[string]string{"operation": "soft_delete", "result": "success"})
	return nil
}

// ListByAgent retrieves tasks assigned to a specific agent
func (r *taskRepository) ListByAgent(ctx context.Context, agentID string, filters types.TaskFilters) (*interfaces.TaskPage, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.ListByAgent")
	defer span.End()

	// Add agent filter
	filters.AssignedTo = &agentID

	return r.listTasks(ctx, filters)
}

// ListByTenant retrieves tasks for a specific tenant
func (r *taskRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, filters types.TaskFilters) (*interfaces.TaskPage, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.ListByTenant")
	defer span.End()

	// Build query with filters
	query, args := r.buildTaskQuery(tenantID, filters)

	// Add COUNT query for total
	countQuery := strings.Replace(query, "SELECT *", "SELECT COUNT(*)", 1)
	countQuery = strings.Split(countQuery, "ORDER BY")[0] // Remove ORDER BY for count

	var totalCount int64
	err := r.readDB.GetContext(ctx, &totalCount, countQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to count tasks")
	}

	// Get tasks
	var tasks []*models.Task
	err = r.readDB.SelectContext(ctx, &tasks, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list tasks")
	}

	// Determine if there are more results
	hasMore := filters.Limit > 0 && len(tasks) == filters.Limit

	// Generate next cursor
	var nextCursor string
	if hasMore && len(tasks) > 0 {
		lastTask := tasks[len(tasks)-1]
		nextCursor = fmt.Sprintf("%s_%s", lastTask.CreatedAt.Format(time.RFC3339Nano), lastTask.ID)
	}

	return &interfaces.TaskPage{
		Tasks:      tasks,
		TotalCount: totalCount,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetSubtasks retrieves all subtasks of a parent task
func (r *taskRepository) GetSubtasks(ctx context.Context, parentTaskID uuid.UUID) ([]*models.Task, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetSubtasks")
	defer span.End()

	query := `
		SELECT * FROM tasks 
		WHERE parent_task_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`

	var tasks []*models.Task
	err := r.readDB.SelectContext(ctx, &tasks, query, parentTaskID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get subtasks")
	}

	return tasks, nil
}

// GetTaskTree retrieves a hierarchical task structure
func (r *taskRepository) GetTaskTree(ctx context.Context, rootTaskID uuid.UUID, maxDepth int) (*models.TaskTree, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetTaskTree")
	defer span.End()

	// Get root task
	rootTask, err := r.Get(ctx, rootTaskID)
	if err != nil {
		return nil, err
	}

	tree := &models.TaskTree{
		Root:     rootTask,
		Children: make(map[uuid.UUID][]*models.Task),
		Depth:    0,
	}

	// Recursively load children
	err = r.loadTaskChildren(ctx, tree, rootTaskID, 0, maxDepth)
	if err != nil {
		return nil, err
	}

	return tree, nil
}

func (r *taskRepository) loadTaskChildren(ctx context.Context, tree *models.TaskTree, parentID uuid.UUID, currentDepth, maxDepth int) error {
	if currentDepth >= maxDepth {
		return nil
	}

	children, err := r.GetSubtasks(ctx, parentID)
	if err != nil {
		return err
	}

	if len(children) > 0 {
		tree.Children[parentID] = children
		if currentDepth+1 > tree.Depth {
			tree.Depth = currentDepth + 1
		}

		// Load children's children
		for _, child := range children {
			err = r.loadTaskChildren(ctx, tree, child.ID, currentDepth+1, maxDepth)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// StreamTasks returns a channel for streaming large result sets
func (r *taskRepository) StreamTasks(ctx context.Context, filters types.TaskFilters) (<-chan *models.Task, <-chan error) {
	taskChan := make(chan *models.Task, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(taskChan)
		defer close(errChan)

		// Use cursor-based pagination for streaming
		cursor := filters.Cursor
		filters.Limit = 1000 // Process in chunks

		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			filters.Cursor = cursor
			page, err := r.listTasksWithCursor(ctx, filters)
			if err != nil {
				errChan <- err
				return
			}

			// Send tasks to channel
			for _, task := range page.Tasks {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				case taskChan <- task:
				}
			}

			if !page.HasMore {
				return
			}

			cursor = page.NextCursor
		}
	}()

	return taskChan, errChan
}

// AssignToAgent assigns a task to an agent
func (r *taskRepository) AssignToAgent(ctx context.Context, taskID uuid.UUID, agentID string, assignedBy string) error {
	ctx, span := r.tracer(ctx, "TaskRepository.AssignToAgent")
	defer span.End()

	query := `
		UPDATE tasks 
		SET assigned_to = $2, 
		    assigned_at = CURRENT_TIMESTAMP,
		    status = 'assigned',
		    updated_at = CURRENT_TIMESTAMP,
		    version = version + 1
		WHERE id = $1 AND deleted_at IS NULL 
		  AND status IN ('pending', 'rejected')`

	result, err := r.writeDB.ExecContext(ctx, query, taskID, agentID)
	if err != nil {
		return errors.Wrap(err, "failed to assign task")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		// Check if task exists and is in valid state
		var task models.Task
		err = r.readDB.GetContext(ctx, &task, "SELECT status FROM tasks WHERE id = $1 AND deleted_at IS NULL", taskID)
		if err == sql.ErrNoRows {
			return types.ErrNotFound
		}
		if err != nil {
			return errors.Wrap(err, "failed to check task status")
		}
		return errors.New("task cannot be assigned in current status: " + string(task.Status))
	}

	// Invalidate cache
	_ = r.cache.Delete(ctx, fmt.Sprintf("task:%s", taskID))

	return nil
}

// UnassignTask removes assignment from a task
func (r *taskRepository) UnassignTask(ctx context.Context, taskID uuid.UUID, reason string) error {
	ctx, span := r.tracer(ctx, "TaskRepository.UnassignTask")
	defer span.End()

	query := `
		UPDATE tasks 
		SET assigned_to = NULL, 
		    assigned_at = NULL,
		    status = 'pending',
		    updated_at = CURRENT_TIMESTAMP,
		    version = version + 1
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.writeDB.ExecContext(ctx, query, taskID)
	if err != nil {
		return errors.Wrap(err, "failed to unassign task")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return types.ErrNotFound
	}

	// Invalidate cache
	_ = r.cache.Delete(ctx, fmt.Sprintf("task:%s", taskID))

	return nil
}

// UpdateStatus updates the status of a task
func (r *taskRepository) UpdateStatus(ctx context.Context, taskID uuid.UUID, status string, metadata map[string]interface{}) error {
	ctx, span := r.tracer(ctx, "TaskRepository.UpdateStatus")
	defer span.End()

	return r.ExecuteQuery(ctx, "update_status", func(ctx context.Context) error {
		// Get current task to validate status transition
		task, err := r.Get(ctx, taskID)
		if err != nil {
			return err
		}

		// Validate status transition
		oldStatus := models.TaskStatus(task.Status)
		newStatus := models.TaskStatus(status)
		if !canTransition(oldStatus, newStatus) {
			return errors.Errorf("invalid status transition from %s to %s", oldStatus, newStatus)
		}

		// Set status-specific timestamps
		statusTime := time.Now()
		var query string

		switch status {
		case "in_progress":
			query = `
				UPDATE tasks 
				SET status = $2, 
				    started_at = $3,
				    updated_at = $3,
				    version = version + 1
				WHERE id = $1 AND deleted_at IS NULL`
		case "completed", "failed", "cancelled", "timeout":
			query = `
				UPDATE tasks 
				SET status = $2, 
				    completed_at = $3,
				    updated_at = $3,
				    version = version + 1
				WHERE id = $1 AND deleted_at IS NULL`
		default:
			query = `
				UPDATE tasks 
				SET status = $2, 
				    updated_at = $3,
				    version = version + 1
				WHERE id = $1 AND deleted_at IS NULL`
		}

		result, err := r.writeDB.ExecContext(ctx, query, taskID, status, statusTime)
		if err != nil {
			return errors.Wrap(err, "failed to update task status")
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}

		if rowsAffected == 0 {
			return interfaces.ErrNotFound
		}

		// Store metadata if provided
		if len(metadata) > 0 {
			// This would update a separate metadata table or the task result field
			metadataJSON, err := json.Marshal(metadata)
			if err != nil {
				return errors.Wrap(err, "failed to marshal metadata")
			}

			_, err = r.writeDB.ExecContext(ctx,
				"UPDATE tasks SET result = $2 WHERE id = $1",
				taskID, metadataJSON)
			if err != nil {
				return errors.Wrap(err, "failed to update task metadata")
			}
		}

		// Invalidate cache
		r.invalidateTaskCache(ctx, task)

		r.metrics.IncrementCounterWithLabels("repository_status_updates", 1, map[string]string{
			"from_status": string(oldStatus),
			"to_status":   status,
		})

		return nil
	})
}

// canTransition validates if a status transition is allowed
func canTransition(from, to models.TaskStatus) bool {
	// Define valid transitions
	validTransitions := map[models.TaskStatus][]models.TaskStatus{
		models.TaskStatusPending:    {models.TaskStatusAssigned, models.TaskStatusCancelled},
		models.TaskStatusAssigned:   {models.TaskStatusAccepted, models.TaskStatusRejected, models.TaskStatusCancelled},
		models.TaskStatusAccepted:   {models.TaskStatusInProgress, models.TaskStatusCancelled},
		models.TaskStatusRejected:   {models.TaskStatusPending, models.TaskStatusAssigned},
		models.TaskStatusInProgress: {models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusTimeout, models.TaskStatusCancelled},
		// Terminal states cannot transition
		models.TaskStatusCompleted: {},
		models.TaskStatusFailed:    {},
		models.TaskStatusCancelled: {},
		models.TaskStatusTimeout:   {},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, status := range allowed {
		if status == to {
			return true
		}
	}

	return false
}

// BulkUpdateStatus updates status for multiple tasks
func (r *taskRepository) BulkUpdateStatus(ctx context.Context, updates []interfaces.StatusUpdate) error {
	ctx, span := r.tracer(ctx, "TaskRepository.BulkUpdateStatus")
	defer span.End()

	tx, err := r.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			r.logger.Error("Failed to rollback transaction", map[string]interface{}{"error": err.Error()})
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE tasks 
		SET status = $2, 
		    updated_at = CURRENT_TIMESTAMP,
		    version = version + 1
		WHERE id = $1 AND deleted_at IS NULL`)
	if err != nil {
		return errors.Wrap(err, "failed to prepare statement")
	}
	defer func() { _ = stmt.Close() }()

	for _, update := range updates {
		_, err = stmt.ExecContext(ctx, update.TaskID, update.Status)
		if err != nil {
			return errors.Wrapf(err, "failed to update task %s", update.TaskID)
		}

		// Invalidate cache
		_ = r.cache.Delete(ctx, fmt.Sprintf("task:%s", update.TaskID))
	}

	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

// IncrementRetryCount increments the retry count for a task
func (r *taskRepository) IncrementRetryCount(ctx context.Context, taskID uuid.UUID) (int, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.IncrementRetryCount")
	defer span.End()

	var newCount int
	query := `
		UPDATE tasks 
		SET retry_count = retry_count + 1,
		    updated_at = CURRENT_TIMESTAMP,
		    version = version + 1
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING retry_count`

	err := r.writeDB.GetContext(ctx, &newCount, query, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, types.ErrNotFound
		}
		return 0, errors.Wrap(err, "failed to increment retry count")
	}

	// Invalidate cache
	_ = r.cache.Delete(ctx, fmt.Sprintf("task:%s", taskID))

	return newCount, nil
}

// CreateDelegation creates a task delegation record
func (r *taskRepository) CreateDelegation(ctx context.Context, delegation *models.TaskDelegation) error {
	ctx, span := r.tracer(ctx, "TaskRepository.CreateDelegation")
	defer span.End()

	query := `
		INSERT INTO task_delegations (
			id, task_id, task_created_at, from_agent_id, to_agent_id,
			reason, delegation_type, metadata, delegated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)`

	if delegation.ID == uuid.Nil {
		delegation.ID = uuid.New()
	}
	if delegation.DelegatedAt.IsZero() {
		delegation.DelegatedAt = time.Now()
	}

	_, err := r.writeDB.ExecContext(ctx, query,
		delegation.ID, delegation.TaskID, delegation.TaskCreatedAt,
		delegation.FromAgentID, delegation.ToAgentID,
		delegation.Reason, delegation.DelegationType, delegation.Metadata,
		delegation.DelegatedAt,
	)

	if err != nil {
		return errors.Wrap(err, "failed to create delegation")
	}

	return nil
}

// GetDelegationHistory retrieves delegation history for a task
func (r *taskRepository) GetDelegationHistory(ctx context.Context, taskID uuid.UUID) ([]*models.TaskDelegation, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetDelegationHistory")
	defer span.End()

	query := `
		SELECT * FROM task_delegations 
		WHERE task_id = $1 
		ORDER BY delegated_at DESC`

	var delegations []*models.TaskDelegation
	err := r.readDB.SelectContext(ctx, &delegations, query, taskID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get delegation history")
	}

	return delegations, nil
}

// GetDelegationsToAgent retrieves delegations to a specific agent
func (r *taskRepository) GetDelegationsToAgent(ctx context.Context, agentID string, since time.Time) ([]*models.TaskDelegation, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetDelegationsToAgent")
	defer span.End()

	query := `
		SELECT * FROM task_delegations 
		WHERE to_agent_id = $1 AND delegated_at >= $2
		ORDER BY delegated_at DESC`

	var delegations []*models.TaskDelegation
	err := r.readDB.SelectContext(ctx, &delegations, query, agentID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get delegations to agent")
	}

	return delegations, nil
}

// GetDelegationChain retrieves the complete delegation chain for a task
func (r *taskRepository) GetDelegationChain(ctx context.Context, taskID uuid.UUID) ([]*models.DelegationNode, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetDelegationChain")
	defer span.End()

	delegations, err := r.GetDelegationHistory(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Build linked list
	var head *models.DelegationNode
	var current *models.DelegationNode

	for i := len(delegations) - 1; i >= 0; i-- {
		node := &models.DelegationNode{
			Delegation: delegations[i],
		}

		if head == nil {
			head = node
			current = node
		} else {
			current.Next = node
			current = node
		}
	}

	// Convert to slice
	var chain []*models.DelegationNode
	for node := head; node != nil; node = node.Next {
		chain = append(chain, node)
	}

	return chain, nil
}

// Helper methods

func (r *taskRepository) buildTaskQuery(tenantID uuid.UUID, filters types.TaskFilters) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argCount := 1

	// Base query
	query := "SELECT * FROM tasks WHERE deleted_at IS NULL"

	// Tenant filter
	if tenantID != uuid.Nil {
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argCount))
		args = append(args, tenantID)
		argCount++
	}

	// Status filter
	if len(filters.Status) > 0 {
		conditions = append(conditions, fmt.Sprintf("status = ANY($%d)", argCount))
		args = append(args, pq.Array(filters.Status))
		argCount++
	}

	// Priority filter
	if len(filters.Priority) > 0 {
		conditions = append(conditions, fmt.Sprintf("priority = ANY($%d)", argCount))
		args = append(args, pq.Array(filters.Priority))
		argCount++
	}

	// Type filter
	if len(filters.Types) > 0 {
		conditions = append(conditions, fmt.Sprintf("type = ANY($%d)", argCount))
		args = append(args, pq.Array(filters.Types))
		argCount++
	}

	// Assigned to filter
	if filters.AssignedTo != nil {
		conditions = append(conditions, fmt.Sprintf("assigned_to = $%d", argCount))
		args = append(args, *filters.AssignedTo)
		argCount++
	}

	// Created by filter
	if filters.CreatedBy != nil {
		conditions = append(conditions, fmt.Sprintf("created_by = $%d", argCount))
		args = append(args, *filters.CreatedBy)
		argCount++
	}

	// Date filters
	if filters.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argCount))
		args = append(args, *filters.CreatedAfter)
		argCount++
	}

	if filters.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argCount))
		args = append(args, *filters.CreatedBefore)
		argCount++
	}

	// Parent task filter
	if filters.ParentTaskID != nil {
		conditions = append(conditions, fmt.Sprintf("parent_task_id = $%d", argCount))
		args = append(args, *filters.ParentTaskID)
	}

	// Add conditions to query
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	// Add sorting
	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := filters.SortOrder
	if sortOrder == "" {
		sortOrder = string(types.SortDesc)
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Add pagination
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filters.Limit)
	}
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filters.Offset)
	}

	return query, args
}

func (r *taskRepository) listTasks(ctx context.Context, filters types.TaskFilters) (*interfaces.TaskPage, error) {
	// Use cursor-based pagination if cursor is provided
	if filters.Cursor != "" {
		return r.listTasksWithCursor(ctx, filters)
	}
	// Otherwise use offset-based pagination
	return r.ListByTenant(ctx, uuid.Nil, filters)
}

func (r *taskRepository) listTasksWithCursor(ctx context.Context, filters types.TaskFilters) (*interfaces.TaskPage, error) {
	// Parse cursor if provided
	var cursorTime time.Time
	var cursorID uuid.UUID

	if filters.Cursor != "" {
		parts := strings.Split(filters.Cursor, "_")
		if len(parts) == 2 {
			cursorTime, _ = time.Parse(time.RFC3339Nano, parts[0])
			cursorID, _ = uuid.Parse(parts[1])
		}
	}

	// Build query with cursor
	var conditions []string
	var args []interface{}
	argCount := 1

	if !cursorTime.IsZero() && cursorID != uuid.Nil {
		conditions = append(conditions, fmt.Sprintf("(created_at, id) > ($%d, $%d)", argCount, argCount+1))
		args = append(args, cursorTime, cursorID)
		argCount += 2
	}

	// Add other filters
	if filters.AssignedTo != nil {
		conditions = append(conditions, fmt.Sprintf("assigned_to = $%d", argCount))
		args = append(args, *filters.AssignedTo)
	}

	conditions = append(conditions, "deleted_at IS NULL")

	query := "SELECT * FROM tasks"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at ASC, id ASC"
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filters.Limit+1) // Get one extra to check if there's more
	}

	var tasks []*models.Task
	err := r.readDB.SelectContext(ctx, &tasks, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list tasks with cursor")
	}

	// Check if there are more results
	hasMore := false
	if filters.Limit > 0 && len(tasks) > filters.Limit {
		hasMore = true
		tasks = tasks[:filters.Limit] // Remove the extra one
	}

	// Generate next cursor
	var nextCursor string
	if hasMore && len(tasks) > 0 {
		lastTask := tasks[len(tasks)-1]
		nextCursor = fmt.Sprintf("%s_%s", lastTask.CreatedAt.Format(time.RFC3339Nano), lastTask.ID)
	}

	return &interfaces.TaskPage{
		Tasks:      tasks,
		TotalCount: int64(len(tasks)), // Note: with cursor pagination, total count is not easily available
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// Implement remaining required methods as stubs for now

func (r *taskRepository) BulkInsert(ctx context.Context, tasks []*models.Task) error {
	return r.CreateBatch(ctx, tasks)
}

func (r *taskRepository) BulkUpdate(ctx context.Context, updates []interfaces.TaskUpdate) error {
	ctx, span := r.tracer(ctx, "TaskRepository.BulkUpdate")
	defer span.End()

	if len(updates) == 0 {
		return nil
	}

	return r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		for _, update := range updates {
			// Build dynamic update query based on fields in updates
			setClauses := []string{"updated_at = CURRENT_TIMESTAMP", "version = version + 1"}
			args := []interface{}{}
			argCount := 1

			for field, value := range update.Updates {
				setClauses = append(setClauses, fmt.Sprintf("%s = $%d", field, argCount))
				args = append(args, value)
				argCount++
			}

			args = append(args, update.TaskID)
			query := fmt.Sprintf(
				"UPDATE tasks SET %s WHERE id = $%d AND deleted_at IS NULL",
				strings.Join(setClauses, ", "),
				argCount,
			)

			_, err := tx.ExecContext(ctx, query, args...)
			if err != nil {
				return errors.Wrapf(err, "failed to update task %s", update.TaskID)
			}

			// Invalidate cache
			_ = r.CacheDelete(ctx, fmt.Sprintf("task:%s", update.TaskID))
		}

		return nil
	})
}

func (r *taskRepository) BatchUpdateStatus(ctx context.Context, taskIDs []uuid.UUID, status string) error {
	updates := make([]interfaces.StatusUpdate, len(taskIDs))
	for i, id := range taskIDs {
		updates[i] = interfaces.StatusUpdate{
			TaskID: id,
			Status: status,
		}
	}
	return r.BulkUpdateStatus(ctx, updates)
}

func (r *taskRepository) ArchiveTasks(ctx context.Context, before time.Time) (int64, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.ArchiveTasks")
	defer span.End()

	var archivedCount int64

	err := r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		// First, insert tasks into archive table
		archiveQuery := `
			INSERT INTO tasks_archive 
			SELECT * FROM tasks 
			WHERE completed_at < $1 
			  AND status IN ('completed', 'failed', 'cancelled')
			  AND deleted_at IS NULL`

		result, err := tx.ExecContext(ctx, archiveQuery, before)
		if err != nil {
			return errors.Wrap(err, "failed to archive tasks")
		}

		archivedCount, err = result.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get archived count")
		}

		// Then delete from main table
		if archivedCount > 0 {
			deleteQuery := `
				DELETE FROM tasks 
				WHERE completed_at < $1 
				  AND status IN ('completed', 'failed', 'cancelled')
				  AND deleted_at IS NULL`

			_, err = tx.ExecContext(ctx, deleteQuery, before)
			if err != nil {
				return errors.Wrap(err, "failed to delete archived tasks")
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	r.metrics.IncrementCounterWithLabels("repository_tasks_archived", float64(archivedCount), map[string]string{
		"before": before.Format(time.RFC3339),
	})

	return archivedCount, nil
}

func (r *taskRepository) GetTasksForExecution(ctx context.Context, agentID string, limit int) ([]*models.Task, error) {
	query := `
		SELECT * FROM tasks 
		WHERE assigned_to = $1 
		  AND status IN ('assigned', 'accepted')
		  AND deleted_at IS NULL
		ORDER BY priority DESC, created_at ASC
		LIMIT $2`

	var tasks []*models.Task
	err := r.readDB.SelectContext(ctx, &tasks, query, agentID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tasks for execution")
	}

	return tasks, nil
}

func (r *taskRepository) GetOverdueTasks(ctx context.Context, threshold time.Duration) ([]*models.Task, error) {
	query := `
		SELECT * FROM tasks 
		WHERE status = 'in_progress'
		  AND started_at IS NOT NULL
		  AND started_at + (timeout_seconds || ' seconds')::interval < CURRENT_TIMESTAMP
		  AND deleted_at IS NULL`

	var tasks []*models.Task
	err := r.readDB.SelectContext(ctx, &tasks, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get overdue tasks")
	}

	return tasks, nil
}

func (r *taskRepository) GetTasksBySchedule(ctx context.Context, schedule string) ([]*models.Task, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetTasksBySchedule")
	defer span.End()

	// This assumes tasks have a schedule field or metadata
	query := `
		SELECT * FROM tasks
		WHERE parameters->>'schedule' = $1
		  AND status = 'pending'
		  AND deleted_at IS NULL
		ORDER BY created_at ASC`

	var tasks []*models.Task
	err := r.readDB.SelectContext(ctx, &tasks, query, schedule)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tasks by schedule")
	}

	return tasks, nil
}

func (r *taskRepository) LockTaskForExecution(ctx context.Context, taskID uuid.UUID, agentID string, duration time.Duration) error {
	ctx, span := r.tracer(ctx, "TaskRepository.LockTaskForExecution")
	defer span.End()

	// Use PostgreSQL advisory lock
	return r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		// Try to acquire advisory lock for this task
		lockQuery := `SELECT pg_try_advisory_xact_lock($1, $2)`
		var locked bool
		err := tx.GetContext(ctx, &locked, lockQuery, int64(taskID.ID()), int64(0))
		if err != nil {
			return errors.Wrap(err, "failed to acquire advisory lock")
		}

		if !locked {
			return errors.New("task is already locked by another agent")
		}

		// Update task status to indicate it's being executed
		updateQuery := `
			UPDATE tasks 
			SET status = 'in_progress',
			    started_at = CURRENT_TIMESTAMP,
			    updated_at = CURRENT_TIMESTAMP,
			    version = version + 1
			WHERE id = $1 
			  AND assigned_to = $2
			  AND status IN ('assigned', 'accepted')
			  AND deleted_at IS NULL`

		result, err := tx.ExecContext(ctx, updateQuery, taskID, agentID)
		if err != nil {
			return errors.Wrap(err, "failed to update task status")
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "failed to get rows affected")
		}

		if rowsAffected == 0 {
			return errors.New("task not found or not in valid state for execution")
		}

		// Invalidate cache
		_ = r.CacheDelete(ctx, fmt.Sprintf("task:%s", taskID))

		return nil
	})
}

func (r *taskRepository) GetTaskStats(ctx context.Context, tenantID uuid.UUID, period time.Duration) (*interfaces.TaskStats, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetTaskStats")
	defer span.End()

	// Try cache first
	cacheKey := fmt.Sprintf("task_stats:%s:%s", tenantID, period.String())
	var stats interfaces.TaskStats
	err := r.CacheGet(ctx, cacheKey, &stats)
	if err == nil {
		return &stats, nil
	}

	// Calculate start time
	startTime := time.Now().Add(-period)

	// Get basic counts
	countQuery := `
		SELECT 
			COUNT(*) as total_count,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_count,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_count
		FROM tasks
		WHERE tenant_id = $1 
		  AND created_at >= $2
		  AND deleted_at IS NULL`

	err = r.readDB.GetContext(ctx, &stats, countQuery, tenantID, startTime)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get task counts")
	}

	// Get average completion time
	timingQuery := `
		SELECT 
			AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as average_completion,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - started_at))) as p95_completion,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - started_at))) as p99_completion
		FROM tasks
		WHERE tenant_id = $1 
		  AND created_at >= $2
		  AND status = 'completed'
		  AND started_at IS NOT NULL
		  AND completed_at IS NOT NULL
		  AND deleted_at IS NULL`

	var timing struct {
		AverageCompletion *float64 `db:"average_completion"`
		P95Completion     *float64 `db:"p95_completion"`
		P99Completion     *float64 `db:"p99_completion"`
	}

	err = r.readDB.GetContext(ctx, &timing, timingQuery, tenantID, startTime)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to get timing stats")
	}

	if timing.AverageCompletion != nil {
		stats.AverageCompletion = time.Duration(*timing.AverageCompletion * float64(time.Second))
	}
	if timing.P95Completion != nil {
		stats.P95Completion = time.Duration(*timing.P95Completion * float64(time.Second))
	}
	if timing.P99Completion != nil {
		stats.P99Completion = time.Duration(*timing.P99Completion * float64(time.Second))
	}

	// Get status breakdown
	statusQuery := `
		SELECT status, COUNT(*) as count
		FROM tasks
		WHERE tenant_id = $1 
		  AND created_at >= $2
		  AND deleted_at IS NULL
		GROUP BY status`

	var statusRows []struct {
		Status string `db:"status"`
		Count  int64  `db:"count"`
	}

	err = r.readDB.SelectContext(ctx, &statusRows, statusQuery, tenantID, startTime)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get status breakdown")
	}

	stats.ByStatus = make(map[string]int64)
	for _, row := range statusRows {
		stats.ByStatus[row.Status] = row.Count
	}

	// Get priority breakdown
	priorityQuery := `
		SELECT priority, COUNT(*) as count
		FROM tasks
		WHERE tenant_id = $1 
		  AND created_at >= $2
		  AND deleted_at IS NULL
		GROUP BY priority`

	var priorityRows []struct {
		Priority string `db:"priority"`
		Count    int64  `db:"count"`
	}

	err = r.readDB.SelectContext(ctx, &priorityRows, priorityQuery, tenantID, startTime)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get priority breakdown")
	}

	stats.ByPriority = make(map[string]int64)
	for _, row := range priorityRows {
		stats.ByPriority[row.Priority] = row.Count
	}

	// Get agent breakdown
	agentQuery := `
		SELECT assigned_to, COUNT(*) as count
		FROM tasks
		WHERE tenant_id = $1 
		  AND created_at >= $2
		  AND assigned_to IS NOT NULL
		  AND deleted_at IS NULL
		GROUP BY assigned_to`

	var agentRows []struct {
		AssignedTo string `db:"assigned_to"`
		Count      int64  `db:"count"`
	}

	err = r.readDB.SelectContext(ctx, &agentRows, agentQuery, tenantID, startTime)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get agent breakdown")
	}

	stats.ByAgent = make(map[string]int64)
	for _, row := range agentRows {
		stats.ByAgent[row.AssignedTo] = row.Count
	}

	// Cache the result
	_ = r.CacheSet(ctx, cacheKey, &stats, 5*time.Minute)

	return &stats, nil
}

func (r *taskRepository) GetAgentWorkload(ctx context.Context, agentIDs []string) (map[string]*interfaces.AgentWorkload, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetAgentWorkload")
	defer span.End()

	if len(agentIDs) == 0 {
		return make(map[string]*interfaces.AgentWorkload), nil
	}

	query := `
		SELECT 
			assigned_to as agent_id,
			COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending_count,
			COUNT(CASE WHEN status IN ('assigned', 'accepted', 'in_progress') THEN 1 END) as active_count,
			COUNT(CASE WHEN status = 'completed' AND DATE(completed_at) = CURRENT_DATE THEN 1 END) as completed_today,
			AVG(CASE WHEN status = 'completed' THEN EXTRACT(EPOCH FROM (completed_at - started_at)) END) as average_time
		FROM tasks
		WHERE assigned_to = ANY($1)
		  AND deleted_at IS NULL
		GROUP BY assigned_to`

	rows, err := r.readDB.QueryContext(ctx, query, pq.Array(agentIDs))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get agent workload")
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]*interfaces.AgentWorkload)
	for _, agentID := range agentIDs {
		result[agentID] = &interfaces.AgentWorkload{
			PendingCount:    0,
			ActiveCount:     0,
			CompletedToday:  0,
			AverageTime:     0,
			CurrentCapacity: 1.0,
		}
	}

	for rows.Next() {
		var (
			agentID        string
			pendingCount   int
			activeCount    int
			completedToday int
			averageTime    sql.NullFloat64
		)

		err := rows.Scan(&agentID, &pendingCount, &activeCount, &completedToday, &averageTime)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan workload row")
		}

		workload := &interfaces.AgentWorkload{
			PendingCount:   pendingCount,
			ActiveCount:    activeCount,
			CompletedToday: completedToday,
		}

		if averageTime.Valid {
			workload.AverageTime = time.Duration(averageTime.Float64 * float64(time.Second))
		}

		// Calculate capacity (simple formula: 1.0 - (active_count / 10))
		workload.CurrentCapacity = 1.0 - (float64(activeCount) / 10.0)
		if workload.CurrentCapacity < 0 {
			workload.CurrentCapacity = 0
		}

		result[agentID] = workload
	}

	return result, nil
}

func (r *taskRepository) GetTaskTimeline(ctx context.Context, taskID uuid.UUID) ([]*interfaces.TaskEvent, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetTaskTimeline")
	defer span.End()

	// This assumes an audit_log or task_events table exists
	query := `
		SELECT 
			timestamp,
			event_type,
			agent_id,
			details
		FROM task_events
		WHERE task_id = $1
		ORDER BY timestamp ASC`

	var events []*interfaces.TaskEvent
	err := r.ExecuteQuery(ctx, "get_task_timeline", func(ctx context.Context) error {
		rows, err := r.readDB.QueryContext(ctx, query, taskID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var event interfaces.TaskEvent
			var detailsJSON []byte

			err := rows.Scan(&event.Timestamp, &event.EventType, &event.AgentID, &detailsJSON)
			if err != nil {
				return errors.Wrap(err, "failed to scan event")
			}

			if len(detailsJSON) > 0 {
				err = json.Unmarshal(detailsJSON, &event.Details)
				if err != nil {
					return errors.Wrap(err, "failed to unmarshal event details")
				}
			}

			events = append(events, &event)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// If no events found in separate table, construct basic timeline from task history
	if len(events) == 0 {
		// Get task to construct basic timeline
		task, err := r.Get(ctx, taskID)
		if err != nil {
			return nil, err
		}

		// Created event
		events = append(events, &interfaces.TaskEvent{
			Timestamp: task.CreatedAt,
			EventType: "created",
			AgentID:   task.CreatedBy,
			Details: map[string]interface{}{
				"title":    task.Title,
				"priority": task.Priority,
			},
		})

		// Assigned event
		if task.AssignedAt != nil && task.AssignedTo != nil {
			events = append(events, &interfaces.TaskEvent{
				Timestamp: *task.AssignedAt,
				EventType: "assigned",
				AgentID:   *task.AssignedTo,
				Details:   map[string]interface{}{},
			})
		}

		// Started event
		if task.StartedAt != nil {
			events = append(events, &interfaces.TaskEvent{
				Timestamp: *task.StartedAt,
				EventType: "started",
				AgentID:   *task.AssignedTo,
				Details:   map[string]interface{}{},
			})
		}

		// Completed event
		if task.CompletedAt != nil {
			events = append(events, &interfaces.TaskEvent{
				Timestamp: *task.CompletedAt,
				EventType: string(task.Status),
				AgentID:   *task.AssignedTo,
				Details: map[string]interface{}{
					"result": task.Result,
					"error":  task.Error,
				},
			})
		}
	}

	return events, nil
}

func (r *taskRepository) GenerateTaskReport(ctx context.Context, filters types.TaskFilters, format string) (io.Reader, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GenerateTaskReport")
	defer span.End()

	// For now, return a simple CSV report
	if format != "csv" {
		return nil, errors.New("only CSV format is currently supported")
	}

	// Get tasks based on filters
	page, err := r.listTasks(ctx, filters)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tasks for report")
	}

	// Generate CSV
	var buf strings.Builder
	buf.WriteString("ID,Title,Status,Priority,AssignedTo,CreatedAt,CompletedAt\n")

	for _, task := range page.Tasks {
		assignedTo := ""
		if task.AssignedTo != nil {
			assignedTo = *task.AssignedTo
		}
		completedAt := ""
		if task.CompletedAt != nil {
			completedAt = task.CompletedAt.Format(time.RFC3339)
		}

		buf.WriteString(fmt.Sprintf("%s,%q,%s,%s,%s,%s,%s\n",
			task.ID,
			task.Title,
			task.Status,
			task.Priority,
			assignedTo,
			task.CreatedAt.Format(time.RFC3339),
			completedAt,
		))
	}

	return strings.NewReader(buf.String()), nil
}

func (r *taskRepository) SearchTasks(ctx context.Context, query string, filters types.TaskFilters) (*interfaces.TaskSearchResult, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.SearchTasks")
	defer span.End()

	if query == "" {
		return &interfaces.TaskSearchResult{
			Tasks:      []*models.Task{},
			TotalCount: 0,
			Facets:     make(map[string]map[string]int64),
			Highlights: make(map[uuid.UUID][]string),
		}, nil
	}

	// Build search query with full-text search
	searchQuery := `
		SELECT 
			*,
			ts_rank_cd(to_tsvector('english', title || ' ' || COALESCE(description, '')), 
			           plainto_tsquery('english', $1)) as rank
		FROM tasks
		WHERE to_tsvector('english', title || ' ' || COALESCE(description, '')) @@ plainto_tsquery('english', $1)
		  AND deleted_at IS NULL`

	args := []interface{}{query}
	argCount := 2

	// Add filters
	if len(filters.Status) > 0 {
		searchQuery += fmt.Sprintf(" AND status = ANY($%d)", argCount)
		args = append(args, pq.Array(filters.Status))
		argCount++
	}

	if filters.AssignedTo != nil {
		searchQuery += fmt.Sprintf(" AND assigned_to = $%d", argCount)
		args = append(args, *filters.AssignedTo)
	}

	searchQuery += " ORDER BY rank DESC"

	if filters.Limit > 0 {
		searchQuery += fmt.Sprintf(" LIMIT %d", filters.Limit)
	}
	if filters.Offset > 0 {
		searchQuery += fmt.Sprintf(" OFFSET %d", filters.Offset)
	}

	var tasks []*models.Task
	err := r.readDB.SelectContext(ctx, &tasks, searchQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to search tasks")
	}

	// Get total count
	countQuery := `
		SELECT COUNT(*)
		FROM tasks
		WHERE to_tsvector('english', title || ' ' || COALESCE(description, '')) @@ plainto_tsquery('english', $1)
		  AND deleted_at IS NULL`

	var totalCount int64
	err = r.readDB.GetContext(ctx, &totalCount, countQuery, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to count search results")
	}

	// Generate simple highlights (just return the search terms found)
	highlights := make(map[uuid.UUID][]string)
	for _, task := range tasks {
		highlights[task.ID] = []string{query}
	}

	return &interfaces.TaskSearchResult{
		Tasks:      tasks,
		TotalCount: totalCount,
		Facets:     make(map[string]map[string]int64), // Could implement faceted search here
		Highlights: highlights,
	}, nil
}

func (r *taskRepository) GetSimilarTasks(ctx context.Context, taskID uuid.UUID, limit int) ([]*models.Task, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.GetSimilarTasks")
	defer span.End()

	// Get the reference task
	task, err := r.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// For now, find tasks with similar type and title
	// In production, this would use vector similarity search
	query := `
		SELECT * FROM tasks
		WHERE id != $1
		  AND type = $2
		  AND deleted_at IS NULL
		  AND (
		    title ILIKE '%' || $3 || '%' OR
		    to_tsvector('english', title) @@ plainto_tsquery('english', $3)
		  )
		ORDER BY created_at DESC
		LIMIT $4`

	// Extract keywords from title (simple approach)
	keywords := strings.Fields(task.Title)
	searchTerm := ""
	if len(keywords) > 0 {
		searchTerm = keywords[0] // Use first word for now
	}

	var tasks []*models.Task
	err = r.readDB.SelectContext(ctx, &tasks, query, taskID, task.Type, searchTerm, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get similar tasks")
	}

	return tasks, nil
}

func (r *taskRepository) VacuumTasks(ctx context.Context) error {
	ctx, span := r.tracer(ctx, "TaskRepository.VacuumTasks")
	defer span.End()

	// Run VACUUM ANALYZE on tasks table
	_, err := r.writeDB.ExecContext(ctx, "VACUUM ANALYZE tasks")
	if err != nil {
		return errors.Wrap(err, "failed to vacuum tasks table")
	}

	r.metrics.IncrementCounter("repository_vacuum_operations", 1)
	return nil
}

func (r *taskRepository) RebuildTaskIndexes(ctx context.Context) error {
	ctx, span := r.tracer(ctx, "TaskRepository.RebuildTaskIndexes")
	defer span.End()

	// Rebuild indexes concurrently to avoid locking
	indexes := []string{
		"idx_tasks_tenant_id",
		"idx_tasks_assigned_to",
		"idx_tasks_status",
		"idx_tasks_created_at",
	}

	for _, indexName := range indexes {
		_, err := r.writeDB.ExecContext(ctx, fmt.Sprintf("REINDEX INDEX CONCURRENTLY %s", indexName))
		if err != nil {
			r.logger.Error("Failed to rebuild index", map[string]interface{}{
				"index": indexName,
				"error": err.Error(),
			})
			// Continue with other indexes even if one fails
		}
	}

	r.metrics.IncrementCounter("repository_index_rebuilds", 1)
	return nil
}

func (r *taskRepository) ValidateTaskIntegrity(ctx context.Context) (*types.IntegrityReport, error) {
	ctx, span := r.tracer(ctx, "TaskRepository.ValidateTaskIntegrity")
	defer span.End()

	report := &types.IntegrityReport{
		CheckedAt:    time.Now(),
		Issues:       []types.IntegrityIssue{},
		TotalChecked: 0,
		IssuesFound:  0,
	}

	// Check for orphaned subtasks
	orphanQuery := `
		SELECT COUNT(*) FROM tasks t1
		WHERE t1.parent_task_id IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM tasks t2 
		    WHERE t2.id = t1.parent_task_id
		  )`

	var orphanCount int
	err := r.readDB.GetContext(ctx, &orphanCount, orphanQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check orphaned tasks")
	}

	if orphanCount > 0 {
		report.Issues = append(report.Issues, types.IntegrityIssue{
			Type:        "orphaned_subtasks",
			Description: fmt.Sprintf("Found %d orphaned subtasks", orphanCount),
			Severity:    "warning",
		})
	}

	// Check for invalid status transitions
	invalidStatusQuery := `
		SELECT COUNT(*) FROM tasks
		WHERE (status = 'completed' AND completed_at IS NULL)
		   OR (status = 'in_progress' AND started_at IS NULL)
		   OR (status = 'assigned' AND assigned_to IS NULL)`

	var invalidStatusCount int
	err = r.readDB.GetContext(ctx, &invalidStatusCount, invalidStatusQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check invalid status")
	}

	if invalidStatusCount > 0 {
		report.Issues = append(report.Issues, types.IntegrityIssue{
			Type:        "invalid_status",
			Description: fmt.Sprintf("Found %d tasks with invalid status/timestamp combinations", invalidStatusCount),
			Severity:    "error",
		})
	}

	// Update counts
	report.IssuesFound = int64(len(report.Issues))

	// Get total task count
	var totalCount int64
	err = r.readDB.GetContext(ctx, &totalCount, "SELECT COUNT(*) FROM tasks WHERE deleted_at IS NULL")
	if err == nil {
		report.TotalChecked = totalCount
	}

	// Add recommendations
	if report.IssuesFound > 0 {
		report.Recommendations = []string{
			"Run cleanup job to fix orphaned subtasks",
			"Review task status transition logic",
			"Update tasks with missing timestamps",
		}
	}

	return report, nil
}
