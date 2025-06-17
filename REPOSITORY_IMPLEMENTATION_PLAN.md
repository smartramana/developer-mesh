# Repository Layer Implementation Plan

## Overview

This document provides a detailed implementation plan for completing the Phase 2 repository layer of the multi-agent collaboration system. All repository interfaces have been defined and basic structures created, but the actual database operations need to be implemented.

## Current State

- ✅ Repository interfaces defined with 40+ methods each
- ✅ PostgreSQL repository structures created
- ✅ Database migrations applied
- ⏳ All methods return "not implemented"
- ⏳ Transaction support needs implementation
- ⏳ Circuit breaker integration pending
- ⏳ Caching layer integration needed

## Implementation Categories

### 1. Transaction Support Implementation

#### 1.1 Fix Transaction Wrapping in WithTx Methods

**Location**: All repository implementations
- `pkg/repository/postgres/task_repository.go`
- `pkg/repository/postgres/workflow_repository.go`
- `pkg/repository/postgres/workspace_repository.go`
- `pkg/repository/postgres/document_repository.go`

**Current State**:
```go
func (r *taskRepository) WithTx(tx types.Transaction) interfaces.TaskRepository {
    // TODO: Implement proper transaction support
    return r
}
```

**Implementation**:
```go
func (r *taskRepository) WithTx(tx types.Transaction) interfaces.TaskRepository {
    pgTx, ok := tx.(*pgTransaction)
    if !ok {
        panic("invalid transaction type")
    }
    
    // Create new repository instance with transaction
    return &taskRepository{
        writeDB: pgTx.tx, // Use transaction for writes
        readDB:  pgTx.tx, // Use transaction for reads (consistency)
        cache:   r.cache,
        logger:  r.logger.WithField("tx", true),
        tracer:  r.tracer,
    }
}
```

#### 1.2 Implement BeginTx Method

**Implementation**:
```go
func (r *taskRepository) BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error) {
    span, ctx := r.tracer(ctx, "repository.task.BeginTx")
    defer span.End()
    
    sqlOpts := &sql.TxOptions{}
    if opts != nil {
        sqlOpts.Isolation = sql.IsolationLevel(opts.Isolation)
        sqlOpts.ReadOnly = opts.ReadOnly
    }
    
    tx, err := r.writeDB.BeginTxx(ctx, sqlOpts)
    if err != nil {
        return nil, errors.Wrap(err, "failed to begin transaction")
    }
    
    return &pgTransaction{
        tx:        tx,
        logger:    r.logger,
        startTime: time.Now(),
    }, nil
}
```

### 2. Circuit Breaker Integration

**Current Issue**: The resilience package's CircuitBreaker doesn't have an Execute method.

#### Option A - Extend Resilience Package (Recommended)

```go
// In pkg/resilience/circuit_breaker.go
type CircuitBreaker interface {
    Execute(ctx context.Context, fn func() error) error
    ExecuteWithFallback(ctx context.Context, fn func() error, fallback func() error) error
    GetState() State
    Reset()
}
```

#### Option B - Wrapper Pattern

```go
// In repository package
type circuitBreakerWrapper struct {
    cb resilience.CircuitBreaker
}

func (w *circuitBreakerWrapper) execute(ctx context.Context, fn func() error) error {
    // Check circuit breaker state
    if !w.cb.IsAvailable() {
        return ErrCircuitOpen
    }
    
    err := fn()
    if err != nil {
        w.cb.RecordFailure()
        return err
    }
    
    w.cb.RecordSuccess()
    return nil
}
```

### 3. Database Operations Implementation

#### 3.1 Task Repository

##### Create Method
```go
func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
    span, ctx := r.tracer(ctx, "repository.task.Create")
    defer span.End()
    
    query := `
        INSERT INTO tasks (
            id, tenant_id, type, status, priority, title, description,
            parameters, assigned_to, delegated_from, parent_task_id,
            created_by, created_at, updated_at, started_at, deadline
        ) VALUES (
            :id, :tenant_id, :type, :status, :priority, :title, :description,
            :parameters, :assigned_to, :delegated_from, :parent_task_id,
            :created_by, :created_at, :updated_at, :started_at, :deadline
        )`
    
    // Set timestamps
    now := time.Now()
    task.CreatedAt = now
    task.UpdatedAt = now
    if task.ID == uuid.Nil {
        task.ID = uuid.New()
    }
    
    // Execute with circuit breaker
    err := r.executeWithCircuitBreaker(ctx, func() error {
        _, err := r.writeDB.NamedExecContext(ctx, query, task)
        return err
    })
    
    if err != nil {
        r.logger.Error("Failed to create task", map[string]interface{}{
            "task_id": task.ID,
            "error":   err.Error(),
        })
        return errors.Wrap(err, "failed to create task")
    }
    
    // Invalidate cache
    r.invalidateTaskCache(ctx, task.TenantID)
    
    return nil
}
```

##### Get Method with Caching
```go
func (r *taskRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
    span, ctx := r.tracer(ctx, "repository.task.Get")
    defer span.End()
    
    // Check cache first
    cacheKey := fmt.Sprintf("task:%s", id)
    if cached, err := r.cache.Get(ctx, cacheKey); err == nil {
        var task models.Task
        if err := json.Unmarshal(cached, &task); err == nil {
            r.logger.Debug("Cache hit for task", map[string]interface{}{"task_id": id})
            return &task, nil
        }
    }
    
    query := `
        SELECT * FROM tasks 
        WHERE id = $1 AND deleted_at IS NULL`
    
    var task models.Task
    err := r.executeWithCircuitBreaker(ctx, func() error {
        return r.readDB.GetContext(ctx, &task, query, id)
    })
    
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.ErrNotFound
        }
        return nil, errors.Wrap(err, "failed to get task")
    }
    
    // Cache the result
    r.cacheTask(ctx, &task)
    
    return &task, nil
}
```

##### Batch Operations
```go
func (r *taskRepository) CreateBatch(ctx context.Context, tasks []*models.Task) error {
    span, ctx := r.tracer(ctx, "repository.task.CreateBatch")
    defer span.End()
    
    if len(tasks) == 0 {
        return nil
    }
    
    // Use COPY for bulk insert
    tx, err := r.writeDB.BeginTx(ctx, nil)
    if err != nil {
        return errors.Wrap(err, "failed to begin transaction")
    }
    defer tx.Rollback()
    
    stmt, err := tx.PrepareContext(ctx, pq.CopyIn("tasks",
        "id", "tenant_id", "type", "status", "priority", "title",
        "description", "parameters", "assigned_to", "delegated_from",
        "parent_task_id", "created_by", "created_at", "updated_at",
    ))
    if err != nil {
        return errors.Wrap(err, "failed to prepare copy statement")
    }
    defer stmt.Close()
    
    now := time.Now()
    for _, task := range tasks {
        if task.ID == uuid.Nil {
            task.ID = uuid.New()
        }
        task.CreatedAt = now
        task.UpdatedAt = now
        
        _, err = stmt.ExecContext(ctx,
            task.ID, task.TenantID, task.Type, task.Status, task.Priority,
            task.Title, task.Description, task.Parameters, task.AssignedTo,
            task.DelegatedFrom, task.ParentTaskID, task.CreatedBy,
            task.CreatedAt, task.UpdatedAt,
        )
        if err != nil {
            return errors.Wrap(err, "failed to copy task")
        }
    }
    
    _, err = stmt.ExecContext(ctx)
    if err != nil {
        return errors.Wrap(err, "failed to flush copy")
    }
    
    return tx.Commit()
}
```

##### List with Filters
```go
func (r *taskRepository) List(ctx context.Context, tenantID uuid.UUID, filters interfaces.TaskFilters) ([]*models.Task, error) {
    span, ctx := r.tracer(ctx, "repository.task.List")
    defer span.End()
    
    query := `
        SELECT * FROM tasks 
        WHERE tenant_id = $1 AND deleted_at IS NULL`
    
    args := []interface{}{tenantID}
    argCount := 1
    
    // Build dynamic query based on filters
    if len(filters.Status) > 0 {
        argCount++
        query += fmt.Sprintf(" AND status = ANY($%d)", argCount)
        args = append(args, pq.Array(filters.Status))
    }
    
    if len(filters.Type) > 0 {
        argCount++
        query += fmt.Sprintf(" AND type = ANY($%d)", argCount)
        args = append(args, pq.Array(filters.Type))
    }
    
    if filters.AssignedTo != nil {
        argCount++
        query += fmt.Sprintf(" AND assigned_to = $%d", argCount)
        args = append(args, *filters.AssignedTo)
    }
    
    if filters.CreatedAfter != nil {
        argCount++
        query += fmt.Sprintf(" AND created_at > $%d", argCount)
        args = append(args, *filters.CreatedAfter)
    }
    
    // Add sorting
    switch filters.SortBy {
    case "created_at":
        query += " ORDER BY created_at"
    case "priority":
        query += " ORDER BY priority"
    case "deadline":
        query += " ORDER BY deadline"
    default:
        query += " ORDER BY created_at"
    }
    
    if filters.SortOrder == types.SortOrderDesc {
        query += " DESC"
    } else {
        query += " ASC"
    }
    
    // Add pagination
    if filters.Limit > 0 {
        argCount++
        query += fmt.Sprintf(" LIMIT $%d", argCount)
        args = append(args, filters.Limit)
    }
    
    if filters.Offset > 0 {
        argCount++
        query += fmt.Sprintf(" OFFSET $%d", argCount)
        args = append(args, filters.Offset)
    }
    
    var tasks []*models.Task
    err := r.executeWithCircuitBreaker(ctx, func() error {
        return r.readDB.SelectContext(ctx, &tasks, query, args...)
    })
    
    if err != nil {
        return nil, errors.Wrap(err, "failed to list tasks")
    }
    
    return tasks, nil
}
```

#### 3.2 Workflow Repository

##### Create with Saga Pattern Support
```go
func (r *workflowRepository) Create(ctx context.Context, workflow *models.Workflow) error {
    span, ctx := r.tracer(ctx, "repository.workflow.Create")
    defer span.End()
    
    // Start transaction for consistency
    tx, err := r.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Insert workflow
    workflowQuery := `
        INSERT INTO workflows (
            id, tenant_id, name, type, status, definition,
            current_stage, metadata, created_by, created_at, updated_at
        ) VALUES (
            :id, :tenant_id, :name, :type, :status, :definition,
            :current_stage, :metadata, :created_by, :created_at, :updated_at
        )`
    
    now := time.Now()
    workflow.CreatedAt = now
    workflow.UpdatedAt = now
    if workflow.ID == uuid.Nil {
        workflow.ID = uuid.New()
    }
    
    _, err = r.writeDB.NamedExecContext(ctx, workflowQuery, workflow)
    if err != nil {
        return errors.Wrap(err, "failed to create workflow")
    }
    
    // Insert stages if workflow has definition
    if workflow.Definition != nil {
        stages, ok := workflow.Definition["stages"].([]interface{})
        if ok {
            for i, stage := range stages {
                stageMap := stage.(map[string]interface{})
                stageQuery := `
                    INSERT INTO workflow_stages (
                        id, workflow_id, stage_index, name, type,
                        config, status, created_at
                    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
                
                _, err = tx.ExecContext(ctx, stageQuery,
                    uuid.New(), workflow.ID, i,
                    stageMap["name"], stageMap["type"],
                    stageMap["config"], "pending", now,
                )
                if err != nil {
                    return errors.Wrap(err, "failed to create workflow stage")
                }
            }
        }
    }
    
    return tx.Commit()
}
```

##### Update Workflow Progress
```go
func (r *workflowRepository) UpdateProgress(ctx context.Context, workflowID uuid.UUID, stageIndex int, status string) error {
    span, ctx := r.tracer(ctx, "repository.workflow.UpdateProgress")
    defer span.End()
    
    tx, err := r.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Update current stage
    _, err = tx.ExecContext(ctx, `
        UPDATE workflows 
        SET current_stage = $2, updated_at = $3
        WHERE id = $1`,
        workflowID, stageIndex, time.Now())
    
    if err != nil {
        return errors.Wrap(err, "failed to update workflow progress")
    }
    
    // Update stage status
    _, err = tx.ExecContext(ctx, `
        UPDATE workflow_stages 
        SET status = $3, completed_at = $4
        WHERE workflow_id = $1 AND stage_index = $2`,
        workflowID, stageIndex, status, time.Now())
    
    if err != nil {
        return errors.Wrap(err, "failed to update stage status")
    }
    
    // Record transition
    _, err = tx.ExecContext(ctx, `
        INSERT INTO workflow_transitions (
            id, workflow_id, from_stage, to_stage, transition_time
        ) VALUES ($1, $2, $3, $4, $5)`,
        uuid.New(), workflowID, stageIndex-1, stageIndex, time.Now())
    
    if err != nil {
        return errors.Wrap(err, "failed to record transition")
    }
    
    return tx.Commit()
}
```

#### 3.3 Document Repository

##### Optimistic Locking Implementation
```go
func (r *documentRepository) UpdateWithLock(ctx context.Context, doc *models.SharedDocument, lockOwner string) error {
    span, ctx := r.tracer(ctx, "repository.document.UpdateWithLock")
    defer span.End()
    
    query := `
        UPDATE shared_documents SET
            title = :title,
            content = :content,
            content_type = :content_type,
            vector_clock = :vector_clock,
            version = version + 1,
            updated_at = :updated_at,
            last_modified_by = :last_modified_by
        WHERE id = :id 
            AND version = :version
            AND (lock_owner = :lock_owner OR lock_expires_at < NOW())
            AND deleted_at IS NULL`
    
    doc.UpdatedAt = time.Now()
    doc.Version++
    
    result, err := r.writeDB.NamedExecContext(ctx, query, map[string]interface{}{
        "id":               doc.ID,
        "title":            doc.Title,
        "content":          doc.Content,
        "content_type":     doc.ContentType,
        "vector_clock":     doc.VectorClock,
        "version":          doc.Version - 1, // Check against old version
        "updated_at":       doc.UpdatedAt,
        "last_modified_by": doc.LastModifiedBy,
        "lock_owner":       lockOwner,
    })
    
    if err != nil {
        return errors.Wrap(err, "failed to update document")
    }
    
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return errors.Wrap(err, "failed to get rows affected")
    }
    
    if rowsAffected == 0 {
        return errors.ErrOptimisticLock
    }
    
    // Invalidate cache
    r.invalidateDocumentCache(ctx, doc.ID)
    
    return nil
}
```

##### Operation Recording with Sequence Numbers
```go
func (r *documentRepository) RecordOperation(ctx context.Context, op *models.DocumentOperation) error {
    span, ctx := r.tracer(ctx, "repository.document.RecordOperation")
    defer span.End()
    
    // Get next sequence number atomically
    var sequence int64
    err := r.writeDB.GetContext(ctx, &sequence, `
        UPDATE shared_documents 
        SET last_sequence = last_sequence + 1
        WHERE id = $1
        RETURNING last_sequence`, op.DocumentID)
    
    if err != nil {
        return errors.Wrap(err, "failed to get sequence number")
    }
    
    op.Sequence = sequence
    op.ID = uuid.New()
    op.AppliedAt = time.Now()
    
    query := `
        INSERT INTO document_operations (
            id, document_id, sequence, operation_type, path,
            value, agent_id, vector_clock, applied_at
        ) VALUES (
            :id, :document_id, :sequence, :operation_type, :path,
            :value, :agent_id, :vector_clock, :applied_at
        )`
    
    _, err = r.writeDB.NamedExecContext(ctx, query, op)
    if err != nil {
        return errors.Wrap(err, "failed to record operation")
    }
    
    return nil
}
```

##### Lock Management
```go
func (r *documentRepository) AcquireLock(ctx context.Context, docID uuid.UUID, agentID string, duration time.Duration) error {
    span, ctx := r.tracer(ctx, "repository.document.AcquireLock")
    defer span.End()
    
    expiresAt := time.Now().Add(duration)
    
    query := `
        UPDATE shared_documents 
        SET lock_owner = $2, 
            lock_expires_at = $3,
            updated_at = $4
        WHERE id = $1 
            AND (lock_owner IS NULL OR lock_expires_at < NOW())
            AND deleted_at IS NULL`
    
    result, err := r.writeDB.ExecContext(ctx, query, docID, agentID, expiresAt, time.Now())
    if err != nil {
        return errors.Wrap(err, "failed to acquire lock")
    }
    
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return errors.Wrap(err, "failed to get rows affected")
    }
    
    if rowsAffected == 0 {
        return errors.ErrLockAcquisitionFailed
    }
    
    return nil
}
```

#### 3.4 Workspace Repository

##### Member Management with Activity Tracking
```go
func (r *workspaceRepository) AddMember(ctx context.Context, member *models.WorkspaceMember) error {
    span, ctx := r.tracer(ctx, "repository.workspace.AddMember")
    defer span.End()
    
    // Check workspace exists and user has permission
    workspace, err := r.Get(ctx, member.WorkspaceID)
    if err != nil {
        return err
    }
    
    // Check if member already exists
    existing, err := r.GetMember(ctx, member.WorkspaceID, member.AgentID)
    if err == nil && existing != nil {
        return errors.ErrAlreadyExists
    }
    
    query := `
        INSERT INTO workspace_members (
            workspace_id, agent_id, role, permissions,
            joined_at, last_activity_at
        ) VALUES (
            :workspace_id, :agent_id, :role, :permissions,
            :joined_at, :last_activity_at
        )`
    
    now := time.Now()
    member.JoinedAt = now
    member.LastActivityAt = now
    
    _, err = r.writeDB.NamedExecContext(ctx, query, member)
    if err != nil {
        return errors.Wrap(err, "failed to add member")
    }
    
    // Record activity
    activity := &models.WorkspaceActivity{
        WorkspaceID: member.WorkspaceID,
        AgentID:     member.AgentID,
        Action:      "member_added",
        Details:     map[string]interface{}{"role": member.Role},
        Timestamp:   now,
    }
    
    return r.RecordActivity(ctx, member.WorkspaceID, activity)
}
```

##### State Management
```go
func (r *workspaceRepository) UpdateState(ctx context.Context, workspaceID uuid.UUID, state map[string]interface{}, version int64) error {
    span, ctx := r.tracer(ctx, "repository.workspace.UpdateState")
    defer span.End()
    
    stateJSON, err := json.Marshal(state)
    if err != nil {
        return errors.Wrap(err, "failed to marshal state")
    }
    
    query := `
        UPDATE workspaces 
        SET state = $2,
            state_version = state_version + 1,
            updated_at = $3
        WHERE id = $1 
            AND state_version = $4
            AND deleted_at IS NULL`
    
    result, err := r.writeDB.ExecContext(ctx, query, 
        workspaceID, stateJSON, time.Now(), version)
    
    if err != nil {
        return errors.Wrap(err, "failed to update state")
    }
    
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return errors.Wrap(err, "failed to get rows affected")
    }
    
    if rowsAffected == 0 {
        return errors.ErrOptimisticLock
    }
    
    return nil
}
```

### 4. Helper Methods Implementation

#### 4.1 Caching Helpers

```go
// Cache invalidation helpers
func (r *taskRepository) invalidateTaskCache(ctx context.Context, tenantID uuid.UUID) {
    pattern := fmt.Sprintf("task:*:tenant:%s", tenantID)
    if err := r.cache.Delete(ctx, pattern); err != nil {
        r.logger.Warn("Failed to invalidate task cache", map[string]interface{}{
            "pattern": pattern,
            "error":   err.Error(),
        })
    }
}

func (r *taskRepository) cacheTask(ctx context.Context, task *models.Task) {
    data, err := json.Marshal(task)
    if err != nil {
        return
    }
    
    cacheKey := fmt.Sprintf("task:%s", task.ID)
    if err := r.cache.Set(ctx, cacheKey, data, 5*time.Minute); err != nil {
        r.logger.Warn("Failed to cache task", map[string]interface{}{
            "task_id": task.ID,
            "error":   err.Error(),
        })
    }
}
```

#### 4.2 Circuit Breaker Wrapper

```go
func (r *taskRepository) executeWithCircuitBreaker(ctx context.Context, fn func() error) error {
    // TODO: Implement when circuit breaker has Execute method
    // For now, execute directly
    return fn()
}
```

#### 4.3 Query Builders

```go
type queryBuilder struct {
    query     strings.Builder
    args      []interface{}
    argCount  int
}

func newQueryBuilder(base string, args ...interface{}) *queryBuilder {
    qb := &queryBuilder{
        args:     args,
        argCount: len(args),
    }
    qb.query.WriteString(base)
    return qb
}

func (qb *queryBuilder) addCondition(condition string, arg interface{}) {
    qb.argCount++
    qb.query.WriteString(fmt.Sprintf(" AND %s = $%d", condition, qb.argCount))
    qb.args = append(qb.args, arg)
}

func (qb *queryBuilder) addInCondition(column string, values interface{}) {
    qb.argCount++
    qb.query.WriteString(fmt.Sprintf(" AND %s = ANY($%d)", column, qb.argCount))
    qb.args = append(qb.args, pq.Array(values))
}

func (qb *queryBuilder) build() (string, []interface{}) {
    return qb.query.String(), qb.args
}
```

### 5. Implementation Timeline

#### Phase 2A - Core Implementation (Week 1)
- [ ] Fix transaction support across all repositories
- [ ] Implement basic CRUD operations for all entities
- [ ] Add caching layer integration
- [ ] Implement error handling patterns

#### Phase 2B - Advanced Features (Week 2)
- [ ] Implement batch operations using PostgreSQL COPY
- [ ] Add optimistic locking for documents
- [ ] Implement operation sequencing and vector clocks
- [ ] Add search and filtering capabilities

#### Phase 2C - Production Features (Week 3)
- [ ] Circuit breaker integration
- [ ] Performance optimizations
- [ ] Monitoring and metrics
- [ ] Data validation and integrity checks

### 6. Testing Strategy

#### Unit Tests

```go
func TestTaskRepository_CreateAndGet(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()
    
    sqlxDB := sqlx.NewDb(db, "postgres")
    cache := mocks.NewMockCache()
    logger := observability.NewLogger("test")
    tracer := observability.NoOpTracer
    
    repo := NewTaskRepository(sqlxDB, sqlxDB, cache, logger, tracer)
    
    task := &models.Task{
        ID:       uuid.New(),
        TenantID: uuid.New(),
        Type:     "analysis",
        Status:   models.TaskStatusPending,
        Title:    "Test Task",
    }
    
    // Test Create
    mock.ExpectExec("INSERT INTO tasks").
        WithArgs(sqlmock.AnyArg()).
        WillReturnResult(sqlmock.NewResult(1, 1))
    
    err = repo.Create(context.Background(), task)
    assert.NoError(t, err)
    
    // Test Get with cache miss
    cache.On("Get", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
    
    rows := sqlmock.NewRows([]string{"id", "tenant_id", "type", "status", "title"}).
        AddRow(task.ID, task.TenantID, task.Type, task.Status, task.Title)
    
    mock.ExpectQuery("SELECT \\* FROM tasks").
        WithArgs(task.ID).
        WillReturnRows(rows)
    
    cache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
    
    retrieved, err := repo.Get(context.Background(), task.ID)
    assert.NoError(t, err)
    assert.Equal(t, task.ID, retrieved.ID)
}
```

#### Integration Tests

```go
func TestTaskRepository_ConcurrentOperations(t *testing.T) {
    // Use real PostgreSQL instance
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    repo := NewTaskRepository(db, db, cache, logger, tracer)
    
    // Test concurrent task creation
    var wg sync.WaitGroup
    errors := make(chan error, 10)
    
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            
            task := &models.Task{
                TenantID: testTenantID,
                Type:     "concurrent",
                Title:    fmt.Sprintf("Task %d", idx),
            }
            
            if err := repo.Create(context.Background(), task); err != nil {
                errors <- err
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // Check for errors
    for err := range errors {
        t.Errorf("Concurrent operation failed: %v", err)
    }
}
```

#### Performance Benchmarks

```go
func BenchmarkTaskRepository_Create(b *testing.B) {
    db := setupBenchDB(b)
    defer cleanupBenchDB(b, db)
    
    repo := NewTaskRepository(db, db, cache, logger, tracer)
    ctx := context.Background()
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        task := &models.Task{
            TenantID: uuid.New(),
            Type:     "benchmark",
            Title:    fmt.Sprintf("Task %d", i),
        }
        
        if err := repo.Create(ctx, task); err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkTaskRepository_BatchCreate(b *testing.B) {
    db := setupBenchDB(b)
    defer cleanupBenchDB(b, db)
    
    repo := NewTaskRepository(db, db, cache, logger, tracer)
    ctx := context.Background()
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        tasks := make([]*models.Task, 100)
        for j := 0; j < 100; j++ {
            tasks[j] = &models.Task{
                TenantID: uuid.New(),
                Type:     "benchmark",
                Title:    fmt.Sprintf("Task %d-%d", i, j),
            }
        }
        
        if err := repo.CreateBatch(ctx, tasks); err != nil {
            b.Fatal(err)
        }
    }
}
```

### 7. Migration Support

```sql
-- 000009_add_repository_indexes.up.sql
-- Performance indexes for repository queries

-- Task indexes
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_status ON tasks(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_delegated_from ON tasks(delegated_from) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_task_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_deadline ON tasks(deadline) WHERE deleted_at IS NULL AND deadline IS NOT NULL;

-- Workflow indexes
CREATE INDEX IF NOT EXISTS idx_workflows_tenant_status ON workflows(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflows_tenant_type ON workflows(tenant_id, type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflow_transitions_workflow ON workflow_transitions(workflow_id);

-- Document indexes
CREATE INDEX IF NOT EXISTS idx_documents_workspace ON shared_documents(workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_lock ON shared_documents(lock_owner, lock_expires_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_operations_document_seq ON document_operations(document_id, sequence);
CREATE INDEX IF NOT EXISTS idx_operations_pending ON document_operations(document_id) WHERE applied_at IS NULL;

-- Workspace indexes
CREATE INDEX IF NOT EXISTS idx_workspace_members_workspace ON workspace_members(workspace_id);
CREATE INDEX IF NOT EXISTS idx_workspace_members_agent ON workspace_members(agent_id);
CREATE INDEX IF NOT EXISTS idx_workspace_activity ON workspace_activity(workspace_id, timestamp DESC);
```

```sql
-- 000009_add_repository_indexes.down.sql
DROP INDEX IF EXISTS idx_tasks_tenant_status;
DROP INDEX IF EXISTS idx_tasks_assigned_to;
DROP INDEX IF EXISTS idx_tasks_delegated_from;
DROP INDEX IF EXISTS idx_tasks_parent;
DROP INDEX IF EXISTS idx_tasks_deadline;
DROP INDEX IF EXISTS idx_workflows_tenant_status;
DROP INDEX IF EXISTS idx_workflows_tenant_type;
DROP INDEX IF EXISTS idx_workflow_transitions_workflow;
DROP INDEX IF EXISTS idx_documents_workspace;
DROP INDEX IF EXISTS idx_documents_lock;
DROP INDEX IF EXISTS idx_operations_document_seq;
DROP INDEX IF EXISTS idx_operations_pending;
DROP INDEX IF EXISTS idx_workspace_members_workspace;
DROP INDEX IF EXISTS idx_workspace_members_agent;
DROP INDEX IF EXISTS idx_workspace_activity;
```

### 8. Error Handling

```go
// Common repository errors
var (
    ErrNotFound              = errors.New("entity not found")
    ErrAlreadyExists         = errors.New("entity already exists")
    ErrOptimisticLock        = errors.New("optimistic lock failed")
    ErrLockAcquisitionFailed = errors.New("failed to acquire lock")
    ErrCircuitOpen           = errors.New("circuit breaker is open")
    ErrInvalidTransaction    = errors.New("invalid transaction type")
)

// Error wrapper for repository operations
func wrapRepositoryError(err error, operation string, details map[string]interface{}) error {
    if err == nil {
        return nil
    }
    
    // Check for known errors
    if errors.Is(err, sql.ErrNoRows) {
        return ErrNotFound
    }
    
    // PostgreSQL specific errors
    var pgErr *pq.Error
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505": // unique_violation
            return ErrAlreadyExists
        case "40001": // serialization_failure
            return ErrOptimisticLock
        }
    }
    
    // Wrap with context
    return errors.Wrapf(err, "repository operation failed: %s", operation)
}
```

### 9. Monitoring and Metrics

```go
// Metrics for repository operations
type repositoryMetrics struct {
    operationDuration *prometheus.HistogramVec
    operationErrors   *prometheus.CounterVec
    cacheHits         *prometheus.CounterVec
    cacheMisses       *prometheus.CounterVec
}

func newRepositoryMetrics(subsystem string) *repositoryMetrics {
    return &repositoryMetrics{
        operationDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Subsystem: subsystem,
                Name:      "operation_duration_seconds",
                Help:      "Duration of repository operations",
            },
            []string{"operation"},
        ),
        operationErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Subsystem: subsystem,
                Name:      "operation_errors_total",
                Help:      "Total number of repository operation errors",
            },
            []string{"operation", "error_type"},
        ),
        cacheHits: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Subsystem: subsystem,
                Name:      "cache_hits_total",
                Help:      "Total number of cache hits",
            },
            []string{"entity_type"},
        ),
        cacheMisses: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Subsystem: subsystem,
                Name:      "cache_misses_total",
                Help:      "Total number of cache misses",
            },
            []string{"entity_type"},
        ),
    }
}

// Usage in repository methods
func (r *taskRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
    timer := prometheus.NewTimer(r.metrics.operationDuration.WithLabelValues("get"))
    defer timer.ObserveDuration()
    
    // ... implementation ...
}
```

### 10. Production Checklist

- [ ] All repository methods implemented
- [ ] Transaction support tested
- [ ] Caching layer integrated
- [ ] Circuit breaker patterns applied
- [ ] Error handling standardized
- [ ] Performance indexes created
- [ ] Monitoring metrics added
- [ ] Unit tests written (>85% coverage)
- [ ] Integration tests passing
- [ ] Performance benchmarks acceptable
- [ ] Documentation updated
- [ ] Code reviewed
- [ ] Security audit completed

## Next Steps

After completing the repository layer implementation:

1. **Phase 3**: Implement the Service Layer
   - Business logic implementation
   - Transaction orchestration
   - Event publishing
   - Validation rules

2. **Phase 4**: Implement WebSocket Handlers
   - Real-time communication
   - Binary protocol optimization
   - Connection management
   - Message routing

3. **Phase 5**: Implement Conflict Resolution
   - CRDT implementation
   - Vector clock management
   - Merge strategies
   - Conflict detection

4. **Phase 6**: Complete Testing and Monitoring
   - End-to-end tests
   - Load testing
   - Chaos testing
   - Production monitoring