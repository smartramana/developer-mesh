# Phase 2: Repository Layer Implementation

## Overview
This phase implements a production-grade data access layer with enterprise features including connection pooling, distributed transactions, multi-level caching, comprehensive observability, and robust error handling. All repositories follow established patterns while adding collaboration-specific optimizations.

## Timeline
**Duration**: 6-7 days
**Prerequisites**: Phase 1 (Database Schema) completed
**Deliverables**:
- 4 repository interfaces with 40+ methods each
- PostgreSQL implementations with connection pooling
- Multi-level caching with Redis and in-memory layers
- Comprehensive test suites (90% coverage target)
- Performance benchmarks and query optimization
- Distributed transaction support
- Read replica routing

## Repository Design Principles

1. **Interface-First Design**: Define comprehensive contracts before implementation
2. **Generic Repository Pattern**: Leverage and extend existing `Repository[T]` base
3. **Multi-Tenant Isolation**: Row-level security with tenant context propagation
4. **Connection Management**: Smart pooling with health checks and circuit breakers
5. **Caching Strategy**: Multi-level caching with consistency guarantees
6. **Error Resilience**: Retry logic, deadlock detection, and graceful degradation
7. **Performance First**: Query optimization, batch operations, and streaming
8. **Observable**: Distributed tracing, metrics, and slow query logging

## Enhanced Repository Interfaces

### 1. Task Repository Interface

```go
// File: pkg/repository/task_repository.go
package repository

import (
    "context"
    "io"
    "time"
    
    "github.com/google/uuid"
    "github.com/S-Corkum/devops-mcp/pkg/models"
)

// TaskFilters defines comprehensive filtering options for task queries
type TaskFilters struct {
    Status         []string
    Priority       []string
    Types          []string
    AssignedTo     *string
    CreatedBy      *string
    CreatedAfter   *time.Time
    CreatedBefore  *time.Time
    UpdatedAfter   *time.Time
    ParentTaskID   *uuid.UUID
    HasSubtasks    *bool
    Tags           []string
    Capabilities   []string
    Limit          int
    Offset         int
    Cursor         string // For cursor-based pagination
    SortBy         string
    SortOrder      string
}

// TaskStats represents aggregated task statistics
type TaskStats struct {
    TotalCount         int64
    CompletedCount     int64
    FailedCount        int64
    AverageCompletion  time.Duration
    P95Completion      time.Duration
    P99Completion      time.Duration
    ByStatus           map[string]int64
    ByPriority         map[string]int64
    ByAgent            map[string]int64
}

// TaskRepository defines the comprehensive interface for task persistence
type TaskRepository interface {
    // Transaction support
    WithTx(tx Transaction) TaskRepository
    BeginTx(ctx context.Context, opts *TxOptions) (Transaction, error)
    
    // Basic CRUD operations with optimistic locking
    Create(ctx context.Context, task *models.Task) error
    CreateBatch(ctx context.Context, tasks []*models.Task) error
    Get(ctx context.Context, id uuid.UUID) (*models.Task, error)
    GetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error)
    GetForUpdate(ctx context.Context, id uuid.UUID) (*models.Task, error) // SELECT FOR UPDATE
    Update(ctx context.Context, task *models.Task) error
    UpdateWithVersion(ctx context.Context, task *models.Task, expectedVersion int) error
    Delete(ctx context.Context, id uuid.UUID) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
    
    // Query operations with cursor pagination
    ListByAgent(ctx context.Context, agentID string, filters TaskFilters) (*TaskPage, error)
    ListByTenant(ctx context.Context, tenantID uuid.UUID, filters TaskFilters) (*TaskPage, error)
    GetSubtasks(ctx context.Context, parentTaskID uuid.UUID) ([]*models.Task, error)
    GetTaskTree(ctx context.Context, rootTaskID uuid.UUID, maxDepth int) (*models.TaskTree, error)
    StreamTasks(ctx context.Context, filters TaskFilters) (<-chan *models.Task, <-chan error)
    
    // Task assignment operations with audit
    AssignToAgent(ctx context.Context, taskID uuid.UUID, agentID string, assignedBy string) error
    UnassignTask(ctx context.Context, taskID uuid.UUID, reason string) error
    UpdateStatus(ctx context.Context, taskID uuid.UUID, status string, metadata map[string]interface{}) error
    BulkUpdateStatus(ctx context.Context, updates []StatusUpdate) error
    IncrementRetryCount(ctx context.Context, taskID uuid.UUID) (int, error)
    
    // Delegation operations
    CreateDelegation(ctx context.Context, delegation *models.TaskDelegation) error
    GetDelegationHistory(ctx context.Context, taskID uuid.UUID) ([]*models.TaskDelegation, error)
    GetDelegationsToAgent(ctx context.Context, agentID string, since time.Time) ([]*models.TaskDelegation, error)
    GetDelegationChain(ctx context.Context, taskID uuid.UUID) ([]*models.DelegationNode, error)
    
    // Bulk operations with COPY support
    BulkInsert(ctx context.Context, tasks []*models.Task) error
    BulkUpdate(ctx context.Context, updates []TaskUpdate) error
    BatchUpdateStatus(ctx context.Context, taskIDs []uuid.UUID, status string) error
    ArchiveTasks(ctx context.Context, before time.Time) (int64, error)
    
    // Execution and scheduling
    GetTasksForExecution(ctx context.Context, agentID string, limit int) ([]*models.Task, error)
    GetOverdueTasks(ctx context.Context, threshold time.Duration) ([]*models.Task, error)
    GetTasksBySchedule(ctx context.Context, schedule string) ([]*models.Task, error)
    LockTaskForExecution(ctx context.Context, taskID uuid.UUID, agentID string, duration time.Duration) error
    
    // Analytics and reporting
    GetTaskStats(ctx context.Context, tenantID uuid.UUID, period time.Duration) (*TaskStats, error)
    GetAgentWorkload(ctx context.Context, agentIDs []string) (map[string]*AgentWorkload, error)
    GetTaskTimeline(ctx context.Context, taskID uuid.UUID) ([]*TaskEvent, error)
    GenerateTaskReport(ctx context.Context, filters TaskFilters, format string) (io.Reader, error)
    
    // Search operations
    SearchTasks(ctx context.Context, query string, filters TaskFilters) (*TaskSearchResult, error)
    GetSimilarTasks(ctx context.Context, taskID uuid.UUID, limit int) ([]*models.Task, error)
    
    // Maintenance operations
    VacuumTasks(ctx context.Context) error
    RebuildTaskIndexes(ctx context.Context) error
    ValidateTaskIntegrity(ctx context.Context) (*IntegrityReport, error)
}

// Supporting types
type TaskPage struct {
    Tasks      []*models.Task
    TotalCount int64
    HasMore    bool
    NextCursor string
}

type StatusUpdate struct {
    TaskID   uuid.UUID
    Status   string
    Metadata map[string]interface{}
}

type TaskUpdate struct {
    TaskID  uuid.UUID
    Updates map[string]interface{}
}

type AgentWorkload struct {
    PendingCount    int
    ActiveCount     int
    CompletedToday  int
    AverageTime     time.Duration
    CurrentCapacity float64
}

type TaskEvent struct {
    Timestamp time.Time
    EventType string
    AgentID   string
    Details   map[string]interface{}
}

type TaskSearchResult struct {
    Tasks      []*models.Task
    TotalCount int64
    Facets     map[string]map[string]int64
    Highlights map[uuid.UUID][]string
}

type IntegrityReport struct {
    CheckedCount     int64
    OrphanedTasks    []uuid.UUID
    InvalidStatuses  []uuid.UUID
    MissingRelations []uuid.UUID
}
```

### 2. Enhanced PostgreSQL Implementation

```go
// File: pkg/repository/postgres/task_repository.go
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
    "github.com/S-Corkum/devops-mcp/pkg/repository"
    "github.com/S-Corkum/devops-mcp/pkg/resilience"
)

// taskRepository implements TaskRepository with production features
type taskRepository struct {
    writeDB        *sqlx.DB
    readDB         *sqlx.DB           // Read replica
    cache          cache.MultiLevel   // L1 (memory) + L2 (Redis) cache
    logger         observability.Logger
    metrics        *repositoryMetrics
    tracer         observability.Tracer
    circuitBreaker resilience.CircuitBreaker
    
    // Prepared statements cache
    stmtCache      map[string]*sqlx.Stmt
    stmtCacheMu    sync.RWMutex
    
    // Configuration
    queryTimeout   time.Duration
    maxRetries     int
    batchSize      int
}

// repositoryMetrics holds Prometheus metrics
type repositoryMetrics struct {
    queries        *prometheus.CounterVec
    queryDuration  *prometheus.HistogramVec
    cacheHits      *prometheus.CounterVec
    cacheMisses    *prometheus.CounterVec
    errors         *prometheus.CounterVec
    poolStats      *prometheus.GaugeVec
}

// NewTaskRepository creates a production-ready task repository
func NewTaskRepository(
    writeDB, readDB *sqlx.DB,
    cache cache.MultiLevel,
    logger observability.Logger,
    tracer observability.Tracer,
    opts ...RepositoryOption,
) repository.TaskRepository {
    repo := &taskRepository{
        writeDB:        writeDB,
        readDB:         readDB,
        cache:          cache,
        logger:         logger,
        tracer:         tracer,
        stmtCache:      make(map[string]*sqlx.Stmt),
        queryTimeout:   30 * time.Second,
        maxRetries:     3,
        batchSize:      1000,
    }
    
    // Apply options
    for _, opt := range opts {
        opt(repo)
    }
    
    // Initialize metrics
    repo.metrics = initializeMetrics()
    
    // Initialize circuit breaker
    repo.circuitBreaker = resilience.NewCircuitBreaker(
        "task_repository",
        resilience.WithErrorThreshold(0.5),
        resilience.WithTimeout(10 * time.Second),
        resilience.WithMaxConcurrent(100),
    )
    
    // Start background tasks
    go repo.monitorPoolStats()
    go repo.prepareCommonStatements()
    
    return repo
}

// WithTx returns a repository instance that uses the provided transaction
func (r *taskRepository) WithTx(tx repository.Transaction) repository.TaskRepository {
    pgTx, ok := tx.(*pgTransaction)
    if !ok {
        panic("invalid transaction type")
    }
    
    // Return new instance with transaction
    return &taskRepository{
        writeDB:        pgTx.tx,
        readDB:         pgTx.tx, // Both use transaction connection
        cache:          r.cache,
        logger:         r.logger,
        metrics:        r.metrics,
        tracer:         r.tracer,
        circuitBreaker: r.circuitBreaker,
        stmtCache:      r.stmtCache,
        queryTimeout:   r.queryTimeout,
        maxRetries:     r.maxRetries,
        batchSize:      r.batchSize,
    }
}

// BeginTx starts a new transaction with options
func (r *taskRepository) BeginTx(ctx context.Context, opts *repository.TxOptions) (repository.Transaction, error) {
    span, ctx := r.tracer.Start(ctx, "TaskRepository.BeginTx")
    defer span.End()
    
    txOpts := &sql.TxOptions{
        Isolation: sql.LevelReadCommitted, // Default
        ReadOnly:  false,
    }
    
    if opts != nil {
        switch opts.Isolation {
        case repository.IsolationSerializable:
            txOpts.Isolation = sql.LevelSerializable
        case repository.IsolationRepeatableRead:
            txOpts.Isolation = sql.LevelRepeatableRead
        case repository.IsolationReadCommitted:
            txOpts.Isolation = sql.LevelReadCommitted
        case repository.IsolationReadUncommitted:
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
    
    return &pgTransaction{tx: tx, logger: r.logger}, nil
}

// Create inserts a new task with retry logic
func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
    span, ctx := r.tracer.Start(ctx, "TaskRepository.Create")
    defer span.End()
    
    return r.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
        return r.createWithRetry(ctx, task)
    })
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
        ON CONFLICT (id) DO NOTHING
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
            return repository.ErrAlreadyExists
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
    span, ctx := r.tracer.Start(ctx, "TaskRepository.CreateBatch")
    defer span.End()
    
    if len(tasks) == 0 {
        return nil
    }
    
    return r.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
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
    })
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
    span, ctx := r.tracer.Start(ctx, "TaskRepository.Get")
    defer span.End()
    
    // Try L1 cache (in-memory)
    cacheKey := fmt.Sprintf("task:%s", id)
    if task, found := r.cache.GetL1(ctx, cacheKey); found {
        r.metrics.cacheHits.WithLabelValues("L1").Inc()
        return task.(*models.Task), nil
    }
    
    // Try L2 cache (Redis)
    var task models.Task
    if err := r.cache.GetL2(ctx, cacheKey, &task); err == nil {
        r.metrics.cacheHits.WithLabelValues("L2").Inc()
        // Promote to L1
        r.cache.SetL1(ctx, cacheKey, &task, 5*time.Minute)
        return &task, nil
    }
    
    r.metrics.cacheMisses.WithLabelValues("all").Inc()
    
    // Query database (use read replica)
    err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
        return r.doGet(ctx, id, &task)
    })
    
    if err != nil {
        return nil, err
    }
    
    // Cache in both levels
    r.cache.SetMultiLevel(ctx, cacheKey, &task, 5*time.Minute, 30*time.Minute)
    
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
            return repository.ErrNotFound
        }
        r.metrics.errors.WithLabelValues("get", classifyError(err)).Inc()
        return errors.Wrap(err, "failed to get task")
    }
    
    r.metrics.queries.WithLabelValues("get", "success").Inc()
    return nil
}

// StreamTasks returns a channel for streaming large result sets
func (r *taskRepository) StreamTasks(ctx context.Context, filters repository.TaskFilters) (<-chan *models.Task, <-chan error) {
    taskChan := make(chan *models.Task, 100)
    errChan := make(chan error, 1)
    
    go func() {
        defer close(taskChan)
        defer close(errChan)
        
        // Use cursor-based pagination for streaming
        cursor := ""
        filters.Limit = 1000 // Process in chunks
        
        for {
            select {
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            default:
            }
            
            filters.Cursor = cursor
            page, err := r.listWithCursor(ctx, filters)
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

// Helper methods

func (r *taskRepository) getOrPrepareStmt(ctx context.Context, name, query string) (*sqlx.NamedStmt, error) {
    r.stmtCacheMu.RLock()
    stmt, exists := r.stmtCache[name]
    r.stmtCacheMu.RUnlock()
    
    if exists {
        return stmt.NamedStmt, nil
    }
    
    // Prepare statement
    r.stmtCacheMu.Lock()
    defer r.stmtCacheMu.Unlock()
    
    // Double-check after acquiring write lock
    if stmt, exists := r.stmtCache[name]; exists {
        return stmt.NamedStmt, nil
    }
    
    namedStmt, err := r.writeDB.PrepareNamedContext(ctx, query)
    if err != nil {
        return nil, err
    }
    
    r.stmtCache[name] = &sqlx.Stmt{NamedStmt: namedStmt}
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
```

### 3. Multi-Level Caching Implementation

```go
// File: pkg/cache/multilevel.go
package cache

import (
    "context"
    "sync"
    "time"
    
    "github.com/bluele/gcache"
    "github.com/go-redis/redis/v8"
    "github.com/vmihailenco/msgpack/v5"
)

// MultiLevel implements a two-level cache (memory + Redis)
type MultiLevel interface {
    GetL1(ctx context.Context, key string) (interface{}, bool)
    GetL2(ctx context.Context, key string, dest interface{}) error
    SetL1(ctx context.Context, key string, value interface{}, ttl time.Duration)
    SetL2(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    SetMultiLevel(ctx context.Context, key string, value interface{}, l1TTL, l2TTL time.Duration)
    Delete(ctx context.Context, key string)
    
    // Cache warming
    Warm(ctx context.Context, keys []string, loader func(string) (interface{}, error))
    
    // Stampede protection
    GetOrLoad(ctx context.Context, key string, loader func() (interface{}, error), ttl time.Duration) (interface{}, error)
}

type multiLevelCache struct {
    l1Cache      gcache.Cache
    l2Client     *redis.Client
    logger       Logger
    loadingKeys  sync.Map // For stampede protection
    
    // Configuration
    l1Size       int
    namespace    string
}

func NewMultiLevelCache(redisClient *redis.Client, opts ...CacheOption) MultiLevel {
    c := &multiLevelCache{
        l2Client:  redisClient,
        l1Size:    10000,
        namespace: "mcp",
    }
    
    // Apply options
    for _, opt := range opts {
        opt(c)
    }
    
    // Initialize L1 cache with LRU eviction
    c.l1Cache = gcache.New(c.l1Size).
        LRU().
        Build()
    
    return c
}

// GetOrLoad implements singleflight pattern for stampede protection
func (c *multiLevelCache) GetOrLoad(ctx context.Context, key string, loader func() (interface{}, error), ttl time.Duration) (interface{}, error) {
    // Check L1
    if val, found := c.GetL1(ctx, key); found {
        return val, nil
    }
    
    // Check L2
    var result interface{}
    if err := c.GetL2(ctx, key, &result); err == nil {
        c.SetL1(ctx, key, result, ttl)
        return result, nil
    }
    
    // Implement singleflight to prevent cache stampede
    type loadResult struct {
        value interface{}
        err   error
    }
    
    // Check if already loading
    if ch, loading := c.loadingKeys.LoadOrStore(key, make(chan loadResult, 1)); loading {
        // Wait for result
        select {
        case res := <-ch.(chan loadResult):
            return res.value, res.err
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    
    // We are the loader
    ch := make(chan loadResult, 1)
    c.loadingKeys.Store(key, ch)
    defer c.loadingKeys.Delete(key)
    
    // Load the value
    value, err := loader()
    result = loadResult{value: value, err: err}
    
    // Notify waiters
    close(ch)
    
    if err != nil {
        return nil, err
    }
    
    // Cache the result
    c.SetMultiLevel(ctx, key, value, ttl, ttl*2)
    
    return value, nil
}
```

### 4. Transaction Support

```go
// File: pkg/repository/postgres/transaction.go
package postgres

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    
    "github.com/jmoiron/sqlx"
    "github.com/pkg/errors"
    
    "github.com/S-Corkum/devops-mcp/pkg/observability"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
)

// pgTransaction wraps sqlx.Tx with additional features
type pgTransaction struct {
    tx         *sqlx.Tx
    logger     observability.Logger
    startTime  time.Time
    savepoints []string
    closed     bool
}

// Execute runs a function within the transaction
func (t *pgTransaction) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
    if t.closed {
        return errors.New("transaction already closed")
    }
    
    return fn(ctx)
}

// Savepoint creates a savepoint for nested transactions
func (t *pgTransaction) Savepoint(ctx context.Context, name string) error {
    if t.closed {
        return errors.New("transaction already closed")
    }
    
    if name == "" {
        name = fmt.Sprintf("sp_%d", len(t.savepoints))
    }
    
    _, err := t.tx.ExecContext(ctx, "SAVEPOINT "+name)
    if err != nil {
        return errors.Wrap(err, "failed to create savepoint")
    }
    
    t.savepoints = append(t.savepoints, name)
    return nil
}

// RollbackToSavepoint rolls back to a specific savepoint
func (t *pgTransaction) RollbackToSavepoint(ctx context.Context, name string) error {
    if t.closed {
        return errors.New("transaction already closed")
    }
    
    _, err := t.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name)
    if err != nil {
        return errors.Wrap(err, "failed to rollback to savepoint")
    }
    
    // Remove savepoints after this one
    for i := len(t.savepoints) - 1; i >= 0; i-- {
        if t.savepoints[i] == name {
            t.savepoints = t.savepoints[:i+1]
            break
        }
    }
    
    return nil
}

// Commit commits the transaction with timing metrics
func (t *pgTransaction) Commit() error {
    if t.closed {
        return errors.New("transaction already closed")
    }
    
    duration := time.Since(t.startTime)
    err := t.tx.Commit()
    t.closed = true
    
    if err != nil {
        t.logger.Error("Transaction commit failed", map[string]interface{}{
            "duration_ms": duration.Milliseconds(),
            "error":       err.Error(),
        })
        return errors.Wrap(err, "failed to commit transaction")
    }
    
    t.logger.Debug("Transaction committed", map[string]interface{}{
        "duration_ms": duration.Milliseconds(),
        "savepoints":  len(t.savepoints),
    })
    
    return nil
}

// Rollback rolls back the transaction
func (t *pgTransaction) Rollback() error {
    if t.closed {
        return nil
    }
    
    err := t.tx.Rollback()
    t.closed = true
    
    if err != nil && err != sql.ErrTxDone {
        return errors.Wrap(err, "failed to rollback transaction")
    }
    
    return nil
}
```

### 5. Comprehensive Testing Strategy

```go
// File: pkg/repository/postgres/task_repository_test.go
package postgres_test

import (
    "context"
    "database/sql"
    "sync"
    "testing"
    "time"
    
    "github.com/DATA-DOG/go-sqlmock"
    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
    pgRepo "github.com/S-Corkum/devops-mcp/pkg/repository/postgres"
    "github.com/S-Corkum/devops-mcp/test/fixtures"
)

// TestTaskRepository_Transactions tests transaction support
func TestTaskRepository_Transactions(t *testing.T) {
    ctx := context.Background()
    db := setupTestDB(t)
    defer db.Close()
    
    repo := setupRepository(db)
    
    t.Run("Commit transaction", func(t *testing.T) {
        tx, err := repo.BeginTx(ctx, &repository.TxOptions{
            Isolation: repository.IsolationSerializable,
            Timeout:   5 * time.Second,
        })
        require.NoError(t, err)
        
        txRepo := repo.WithTx(tx)
        
        // Create task in transaction
        task := fixtures.NewTask()
        err = txRepo.Create(ctx, task)
        require.NoError(t, err)
        
        // Task shouldn't be visible outside transaction
        _, err = repo.Get(ctx, task.ID)
        assert.Equal(t, repository.ErrNotFound, err)
        
        // Commit
        err = tx.Commit()
        require.NoError(t, err)
        
        // Now task should be visible
        retrieved, err := repo.Get(ctx, task.ID)
        require.NoError(t, err)
        assert.Equal(t, task.ID, retrieved.ID)
    })
    
    t.Run("Rollback transaction", func(t *testing.T) {
        tx, err := repo.BeginTx(ctx, nil)
        require.NoError(t, err)
        
        txRepo := repo.WithTx(tx)
        
        // Create task in transaction
        task := fixtures.NewTask()
        err = txRepo.Create(ctx, task)
        require.NoError(t, err)
        
        // Rollback
        err = tx.Rollback()
        require.NoError(t, err)
        
        // Task should not exist
        _, err = repo.Get(ctx, task.ID)
        assert.Equal(t, repository.ErrNotFound, err)
    })
    
    t.Run("Savepoint support", func(t *testing.T) {
        tx, err := repo.BeginTx(ctx, nil)
        require.NoError(t, err)
        defer tx.Rollback()
        
        txRepo := repo.WithTx(tx)
        
        // Create first task
        task1 := fixtures.NewTask()
        err = txRepo.Create(ctx, task1)
        require.NoError(t, err)
        
        // Create savepoint
        err = tx.Savepoint(ctx, "before_task2")
        require.NoError(t, err)
        
        // Create second task
        task2 := fixtures.NewTask()
        err = txRepo.Create(ctx, task2)
        require.NoError(t, err)
        
        // Rollback to savepoint
        err = tx.RollbackToSavepoint(ctx, "before_task2")
        require.NoError(t, err)
        
        // Commit transaction
        err = tx.Commit()
        require.NoError(t, err)
        
        // First task should exist
        _, err = repo.Get(ctx, task1.ID)
        assert.NoError(t, err)
        
        // Second task should not exist
        _, err = repo.Get(ctx, task2.ID)
        assert.Equal(t, repository.ErrNotFound, err)
    })
}

// TestTaskRepository_ConcurrentAccess tests concurrent operations
func TestTaskRepository_ConcurrentAccess(t *testing.T) {
    ctx := context.Background()
    db := setupTestDB(t)
    defer db.Close()
    
    repo := setupRepository(db)
    
    t.Run("Concurrent creates", func(t *testing.T) {
        const goroutines = 10
        const tasksPerGoroutine = 100
        
        var wg sync.WaitGroup
        errors := make(chan error, goroutines*tasksPerGoroutine)
        
        for i := 0; i < goroutines; i++ {
            wg.Add(1)
            go func(workerID int) {
                defer wg.Done()
                
                for j := 0; j < tasksPerGoroutine; j++ {
                    task := fixtures.NewTask()
                    task.Title = fmt.Sprintf("Task %d-%d", workerID, j)
                    
                    if err := repo.Create(ctx, task); err != nil {
                        errors <- err
                    }
                }
            }(i)
        }
        
        wg.Wait()
        close(errors)
        
        // Check for errors
        var errCount int
        for err := range errors {
            t.Errorf("Error creating task: %v", err)
            errCount++
        }
        
        assert.Equal(t, 0, errCount)
    })
    
    t.Run("Optimistic locking", func(t *testing.T) {
        // Create task
        task := fixtures.NewTask()
        err := repo.Create(ctx, task)
        require.NoError(t, err)
        
        // Load task in two "sessions"
        task1, err := repo.Get(ctx, task.ID)
        require.NoError(t, err)
        
        task2, err := repo.Get(ctx, task.ID)
        require.NoError(t, err)
        
        // Update task1
        task1.Status = "in_progress"
        err = repo.UpdateWithVersion(ctx, task1, task1.Version)
        require.NoError(t, err)
        
        // Try to update task2 (should fail)
        task2.Status = "completed"
        err = repo.UpdateWithVersion(ctx, task2, task2.Version)
        assert.Equal(t, repository.ErrOptimisticLock, err)
    })
}

// Benchmark tests
func BenchmarkTaskRepository_Create(b *testing.B) {
    ctx := context.Background()
    db := setupBenchDB(b)
    defer db.Close()
    
    repo := setupRepository(db)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            task := fixtures.NewTask()
            _ = repo.Create(ctx, task)
        }
    })
}

func BenchmarkTaskRepository_Get(b *testing.B) {
    ctx := context.Background()
    db := setupBenchDB(b)
    defer db.Close()
    
    repo := setupRepository(db)
    
    // Create tasks
    taskIDs := make([]uuid.UUID, 1000)
    for i := 0; i < len(taskIDs); i++ {
        task := fixtures.NewTask()
        _ = repo.Create(ctx, task)
        taskIDs[i] = task.ID
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            _, _ = repo.Get(ctx, taskIDs[i%len(taskIDs)])
            i++
        }
    })
}

// Integration test with real PostgreSQL
func TestTaskRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    ctx := context.Background()
    
    // Start PostgreSQL container
    pgContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:14-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second),
        ),
    )
    require.NoError(t, err)
    defer pgContainer.Terminate(ctx)
    
    // Get connection string
    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)
    
    // Connect to database
    db, err := sqlx.Connect("postgres", connStr)
    require.NoError(t, err)
    defer db.Close()
    
    // Run migrations
    err = runMigrations(db)
    require.NoError(t, err)
    
    // Create repository
    repo := setupRepository(db)
    
    // Run comprehensive tests
    t.Run("CRUD operations", func(t *testing.T) {
        testCRUDOperations(t, ctx, repo)
    })
    
    t.Run("Batch operations", func(t *testing.T) {
        testBatchOperations(t, ctx, repo)
    })
    
    t.Run("Query operations", func(t *testing.T) {
        testQueryOperations(t, ctx, repo)
    })
    
    t.Run("Stream operations", func(t *testing.T) {
        testStreamOperations(t, ctx, repo)
    })
}
```

## Performance Optimization

### 1. Connection Pool Configuration

```go
// File: pkg/repository/postgres/config.go

// OptimalPoolConfig returns production-ready connection pool settings
func OptimalPoolConfig() *PoolConfig {
    return &PoolConfig{
        // Write pool (primary)
        WritePool: DBPoolConfig{
            MaxOpenConns:     30,
            MaxIdleConns:     10,
            ConnMaxLifetime:  30 * time.Minute,
            ConnMaxIdleTime:  5 * time.Minute,
            HealthCheckPeriod: 1 * time.Minute,
        },
        // Read pool (replicas)
        ReadPool: DBPoolConfig{
            MaxOpenConns:     50,
            MaxIdleConns:     20,
            ConnMaxLifetime:  30 * time.Minute,
            ConnMaxIdleTime:  5 * time.Minute,
            HealthCheckPeriod: 1 * time.Minute,
        },
    }
}
```

### 2. Query Optimization

```sql
-- File: migrations/0024_query_optimization.sql

-- Create materialized view for task statistics
CREATE MATERIALIZED VIEW mv_task_stats AS
SELECT 
    tenant_id,
    date_trunc('hour', created_at) as hour,
    status,
    priority,
    COUNT(*) as count,
    AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) as avg_duration,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - created_at))) as median_duration,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - created_at))) as p95_duration
FROM tasks
WHERE deleted_at IS NULL
GROUP BY tenant_id, date_trunc('hour', created_at), status, priority;

-- Create index for refresh
CREATE UNIQUE INDEX idx_mv_task_stats ON mv_task_stats(tenant_id, hour, status, priority);

-- Refresh every hour
CREATE OR REPLACE FUNCTION refresh_task_stats() RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_task_stats;
END;
$$ LANGUAGE plpgsql;

-- Schedule refresh with pg_cron (if available)
-- SELECT cron.schedule('refresh-task-stats', '0 * * * *', 'SELECT refresh_task_stats()');
```

## Monitoring and Observability

### 1. Repository Metrics

```go
// File: pkg/repository/postgres/metrics.go

func initializeMetrics() *repositoryMetrics {
    return &repositoryMetrics{
        queries: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "repository_queries_total",
                Help: "Total number of repository queries",
            },
            []string{"operation", "status"},
        ),
        queryDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "repository_query_duration_seconds",
                Help:    "Query duration in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
            },
            []string{"operation"},
        ),
        cacheHits: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "repository_cache_hits_total",
                Help: "Total number of cache hits",
            },
            []string{"level"},
        ),
        cacheMisses: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "repository_cache_misses_total",
                Help: "Total number of cache misses",
            },
            []string{"level"},
        ),
        errors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "repository_errors_total",
                Help: "Total number of repository errors",
            },
            []string{"operation", "error_type"},
        ),
        poolStats: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "repository_pool_connections",
                Help: "Database connection pool statistics",
            },
            []string{"pool", "state"},
        ),
    }
}
```

### 2. Slow Query Logging

```go
// File: pkg/repository/postgres/logging.go

// LogSlowQuery logs queries that exceed threshold
func LogSlowQuery(logger Logger, query string, args []interface{}, duration time.Duration, threshold time.Duration) {
    if duration > threshold {
        logger.Warn("Slow query detected", map[string]interface{}{
            "query":       query,
            "args":        args,
            "duration_ms": duration.Milliseconds(),
            "threshold_ms": threshold.Milliseconds(),
        })
    }
}
```

## Security Considerations

### 1. SQL Injection Prevention

```go
// ValidateIdentifier ensures database identifiers are safe
func ValidateIdentifier(identifier string) error {
    if !identifierRegex.MatchString(identifier) {
        return errors.New("invalid identifier")
    }
    return nil
}

var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
```

### 2. Query Parameter Sanitization

```go
// SanitizeParams cleans parameters before query execution
func SanitizeParams(params map[string]interface{}) map[string]interface{} {
    cleaned := make(map[string]interface{})
    for k, v := range params {
        // Remove null bytes
        if str, ok := v.(string); ok {
            cleaned[k] = strings.ReplaceAll(str, "\x00", "")
        } else {
            cleaned[k] = v
        }
    }
    return cleaned
}
```

## Migration Guide

### From In-Memory to PostgreSQL

```go
// File: pkg/repository/migration/progressive_migration.go

// ProgressiveMigrator allows gradual migration from in-memory to PostgreSQL
type ProgressiveMigrator struct {
    inMemory  repository.TaskRepository
    postgres  repository.TaskRepository
    readRatio float64 // Percentage of reads from PostgreSQL
    logger    Logger
}

func (m *ProgressiveMigrator) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
    // Gradually increase PostgreSQL reads
    if rand.Float64() < m.readRatio {
        task, err := m.postgres.Get(ctx, id)
        if err == nil {
            return task, nil
        }
        m.logger.Warn("PostgreSQL read failed, falling back", map[string]interface{}{
            "error": err,
            "id":    id,
        })
    }
    
    // Fall back to in-memory
    return m.inMemory.Get(ctx, id)
}
```

## Production Readiness Checklist

- ✅ Connection pooling with health checks
- ✅ Read replica support for scaling
- ✅ Transaction support with savepoints
- ✅ Retry logic with exponential backoff
- ✅ Deadlock detection and recovery
- ✅ Multi-level caching with stampede protection
- ✅ Batch operations using COPY
- ✅ Streaming for large result sets
- ✅ Cursor-based pagination
- ✅ Query timeout enforcement
- ✅ Slow query logging
- ✅ Comprehensive metrics
- ✅ Distributed tracing
- ✅ SQL injection prevention
- ✅ Optimistic locking support
- ✅ Circuit breaker for resilience
- ✅ 90% test coverage target

## Performance Benchmarks

Expected performance with proper configuration:

| Operation | Target | Notes |
|-----------|--------|-------|
| Single Create | < 5ms | With cache invalidation |
| Batch Create (1000) | < 100ms | Using COPY |
| Single Get (cached) | < 0.1ms | L1 cache hit |
| Single Get (DB) | < 2ms | With index |
| List (100 items) | < 10ms | With pagination |
| Stream (10K items) | < 1s | Initial response |
| Transaction Commit | < 20ms | Normal load |

## Next Steps

After completing Phase 2:
1. Deploy repository layer to test environment
2. Run performance benchmarks against production-like data
3. Configure monitoring dashboards
4. Load test with expected traffic patterns
5. Implement gradual migration from in-memory
6. Train team on transaction patterns
7. Document query optimization findings