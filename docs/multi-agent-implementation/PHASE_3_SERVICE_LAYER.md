# Phase 3: Service Layer Implementation

## Overview
This phase implements a production-grade business logic layer with enterprise features including distributed transactions, event sourcing, rate limiting, circuit breakers, and comprehensive security. Services orchestrate multi-agent collaboration with resilience, observability, and compliance.

## Timeline
**Duration**: 6-7 days
**Prerequisites**: Phase 2 (Repository Layer) completed
**Deliverables**:
- 4 core service implementations with 50+ methods each
- Business rule engine with dynamic policies
- Distributed transaction coordinator with saga pattern
- Event sourcing with replay capability
- Rate limiting and quota management
- Circuit breakers and health checks
- Comprehensive audit trail

## Service Design Principles

1. **Domain-Driven Design**: Services align with business domains
2. **Transaction Boundaries**: Clear aggregate boundaries with saga orchestration
3. **Event-Driven Architecture**: Event sourcing with CQRS patterns
4. **Resilience First**: Circuit breakers, retries, and fallbacks
5. **Security by Design**: Authorization, sanitization, and audit
6. **Performance Optimized**: Async processing, batching, and caching
7. **Observable**: Metrics, tracing, and structured logging
8. **Compliance Ready**: Audit trails and policy enforcement

## Core Service Architecture

### Base Service Framework

```go
// File: pkg/services/base_service.go
package services

import (
    "context"
    "time"
    
    "github.com/google/uuid"
    "github.com/sony/gobreaker"
    
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    "github.com/S-Corkum/devops-mcp/pkg/events"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
    "github.com/S-Corkum/devops-mcp/pkg/resilience"
    "github.com/S-Corkum/devops-mcp/pkg/rules"
)

// ServiceConfig provides common configuration for all services
type ServiceConfig struct {
    // Resilience
    CircuitBreaker    *gobreaker.Settings
    RetryPolicy       resilience.RetryPolicy
    TimeoutPolicy     resilience.TimeoutPolicy
    BulkheadPolicy    resilience.BulkheadPolicy
    
    // Rate Limiting
    RateLimiter       RateLimiterConfig
    QuotaManager      QuotaManagerConfig
    
    // Security
    Authorizer        auth.Authorizer
    Sanitizer         security.Sanitizer
    EncryptionService security.EncryptionService
    
    // Observability
    Logger            observability.Logger
    Metrics           observability.MetricsClient
    Tracer            observability.Tracer
    
    // Business Rules
    RuleEngine        rules.Engine
    PolicyManager     rules.PolicyManager
}

// BaseService provides common functionality for all services
type BaseService struct {
    config         ServiceConfig
    eventStore     events.Store
    eventPublisher events.Publisher
    healthChecker  health.Checker
}

// WithTransaction executes a function within a transaction with saga support
func (s *BaseService) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error {
    // Start distributed transaction
    tx, err := s.startDistributedTransaction(ctx)
    if err != nil {
        return err
    }
    
    // Setup compensation
    compensator := NewCompensator(s.logger)
    ctx = context.WithValue(ctx, "compensator", compensator)
    
    // Execute function
    err = fn(ctx, tx)
    if err != nil {
        // Run compensations
        if compErr := compensator.Compensate(ctx); compErr != nil {
            s.logger.Error("Compensation failed", map[string]interface{}{
                "error": compErr,
                "original_error": err,
            })
        }
        tx.Rollback()
        return err
    }
    
    // Commit transaction
    if err := tx.Commit(); err != nil {
        compensator.Compensate(ctx)
        return err
    }
    
    return nil
}

// PublishEvent publishes an event with versioning and metadata
func (s *BaseService) PublishEvent(ctx context.Context, eventType string, aggregate AggregateRoot, data interface{}) error {
    event := &events.DomainEvent{
        ID:            uuid.New(),
        Type:          eventType,
        AggregateID:   aggregate.GetID(),
        AggregateType: aggregate.GetType(),
        Version:       aggregate.GetVersion(),
        Timestamp:     time.Now(),
        Data:          data,
        Metadata: events.Metadata{
            TenantID:      auth.GetTenantID(ctx),
            UserID:        auth.GetUserID(ctx),
            CorrelationID: observability.GetCorrelationID(ctx),
            CausationID:   observability.GetCausationID(ctx),
        },
    }
    
    // Store event
    if err := s.eventStore.Append(ctx, event); err != nil {
        return err
    }
    
    // Publish for projections
    return s.eventPublisher.Publish(ctx, event)
}

// CheckRateLimit enforces rate limiting per tenant/agent
func (s *BaseService) CheckRateLimit(ctx context.Context, resource string) error {
    tenantID := auth.GetTenantID(ctx)
    agentID := auth.GetAgentID(ctx)
    
    // Check tenant limit
    if err := s.config.RateLimiter.Check(ctx, fmt.Sprintf("tenant:%s:%s", tenantID, resource)); err != nil {
        return ErrRateLimitExceeded
    }
    
    // Check agent limit
    if err := s.config.RateLimiter.Check(ctx, fmt.Sprintf("agent:%s:%s", agentID, resource)); err != nil {
        return ErrRateLimitExceeded
    }
    
    return nil
}

// CheckQuota verifies resource quota
func (s *BaseService) CheckQuota(ctx context.Context, resource string, amount int64) error {
    tenantID := auth.GetTenantID(ctx)
    
    quota, err := s.config.QuotaManager.GetQuota(ctx, tenantID, resource)
    if err != nil {
        return err
    }
    
    usage, err := s.config.QuotaManager.GetUsage(ctx, tenantID, resource)
    if err != nil {
        return err
    }
    
    if usage + amount > quota {
        return ErrQuotaExceeded
    }
    
    return s.config.QuotaManager.IncrementUsage(ctx, tenantID, resource, amount)
}
```

### Enhanced Task Service

```go
// File: pkg/services/task_service.go
package services

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/pkg/errors"
    "golang.org/x/sync/errgroup"
    
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
    "github.com/S-Corkum/devops-mcp/pkg/rules"
)

// TaskService handles task lifecycle with production features
type TaskService interface {
    // Task lifecycle with idempotency
    Create(ctx context.Context, task *models.Task, idempotencyKey string) error
    CreateBatch(ctx context.Context, tasks []*models.Task) error
    Get(ctx context.Context, id uuid.UUID) (*models.Task, error)
    GetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error)
    Update(ctx context.Context, task *models.Task) error
    Delete(ctx context.Context, id uuid.UUID) error
    
    // Task assignment with load balancing
    AssignTask(ctx context.Context, taskID uuid.UUID, agentID string) error
    AutoAssignTask(ctx context.Context, taskID uuid.UUID, strategy AssignmentStrategy) error
    DelegateTask(ctx context.Context, delegation *models.TaskDelegation) error
    AcceptTask(ctx context.Context, taskID uuid.UUID, agentID string) error
    RejectTask(ctx context.Context, taskID uuid.UUID, agentID string, reason string) error
    
    // Task execution with monitoring
    StartTask(ctx context.Context, taskID uuid.UUID, agentID string) error
    UpdateProgress(ctx context.Context, taskID uuid.UUID, progress int, message string) error
    CompleteTask(ctx context.Context, taskID uuid.UUID, agentID string, result interface{}) error
    FailTask(ctx context.Context, taskID uuid.UUID, agentID string, error string) error
    RetryTask(ctx context.Context, taskID uuid.UUID) error
    
    // Advanced querying with caching
    GetAgentTasks(ctx context.Context, agentID string, filters repository.TaskFilters) (*repository.TaskPage, error)
    GetAvailableTasks(ctx context.Context, agentID string, capabilities []string) ([]*models.Task, error)
    SearchTasks(ctx context.Context, query string, filters repository.TaskFilters) (*repository.TaskSearchResult, error)
    GetTaskTimeline(ctx context.Context, taskID uuid.UUID) ([]*repository.TaskEvent, error)
    
    // Distributed task management
    CreateDistributedTask(ctx context.Context, task *models.DistributedTask) error
    SubmitSubtaskResult(ctx context.Context, parentTaskID, subtaskID uuid.UUID, result interface{}) error
    GetTaskTree(ctx context.Context, rootTaskID uuid.UUID) (*models.TaskTree, error)
    CancelTaskTree(ctx context.Context, rootTaskID uuid.UUID, reason string) error
    
    // Workflow integration
    CreateWorkflowTask(ctx context.Context, workflowID, stepID uuid.UUID, params map[string]interface{}) (*models.Task, error)
    CompleteWorkflowTask(ctx context.Context, taskID uuid.UUID, output interface{}) error
    
    // Analytics and reporting
    GetTaskStats(ctx context.Context, filters repository.TaskFilters) (*repository.TaskStats, error)
    GetAgentPerformance(ctx context.Context, agentID string, period time.Duration) (*models.AgentPerformance, error)
    GenerateTaskReport(ctx context.Context, filters repository.TaskFilters, format string) ([]byte, error)
    
    // Maintenance
    ArchiveCompletedTasks(ctx context.Context, before time.Time) (int64, error)
    RebalanceTasks(ctx context.Context) error
}

type taskService struct {
    BaseService
    
    // Dependencies
    repo              repository.TaskRepository
    agentService      AgentService
    notifier          NotificationService
    workflowService   WorkflowService
    
    // Configuration
    assignmentEngine  *AssignmentEngine
    aggregator        *ResultAggregator
    
    // Caching
    taskCache         cache.Cache
    statsCache        cache.Cache
    
    // Background workers
    progressTracker   *ProgressTracker
    taskRebalancer    *TaskRebalancer
    
    // Synchronization
    taskLocks         sync.Map // map[uuid.UUID]*sync.Mutex
}

// NewTaskService creates a production-ready task service
func NewTaskService(
    config ServiceConfig,
    repo repository.TaskRepository,
    agentService AgentService,
    notifier NotificationService,
) TaskService {
    s := &taskService{
        BaseService:      NewBaseService(config),
        repo:             repo,
        agentService:     agentService,
        notifier:         notifier,
        assignmentEngine: NewAssignmentEngine(config.RuleEngine),
        aggregator:       NewResultAggregator(),
        taskCache:        cache.NewLRU(10000, 5*time.Minute),
        statsCache:       cache.NewLRU(1000, 1*time.Minute),
    }
    
    // Start background workers
    s.progressTracker = NewProgressTracker(s)
    s.taskRebalancer = NewTaskRebalancer(s, config.RuleEngine)
    
    go s.progressTracker.Start()
    go s.taskRebalancer.Start()
    
    return s
}

// Create creates a task with full validation and idempotency
func (s *taskService) Create(ctx context.Context, task *models.Task, idempotencyKey string) error {
    span, ctx := s.tracer.Start(ctx, "TaskService.Create")
    defer span.End()
    
    // Check rate limit
    if err := s.CheckRateLimit(ctx, "task:create"); err != nil {
        return err
    }
    
    // Check quota
    if err := s.CheckQuota(ctx, "tasks", 1); err != nil {
        return err
    }
    
    // Idempotency check
    if idempotencyKey != "" {
        existingID, err := s.checkIdempotency(ctx, idempotencyKey)
        if err == nil && existingID != uuid.Nil {
            task.ID = existingID
            return nil
        }
    }
    
    // Sanitize input
    if err := s.sanitizeTask(task); err != nil {
        return errors.Wrap(err, "input sanitization failed")
    }
    
    // Validate with business rules
    if err := s.validateTask(ctx, task); err != nil {
        return errors.Wrap(err, "task validation failed")
    }
    
    // Check authorization
    if err := s.authorizeTaskCreation(ctx, task); err != nil {
        return errors.Wrap(err, "authorization failed")
    }
    
    // Execute in transaction
    err := s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
        // Set metadata
        task.ID = uuid.New()
        task.TenantID = auth.GetTenantID(ctx)
        task.CreatedBy = auth.GetAgentID(ctx)
        task.Status = models.TaskStatusPending
        task.Version = 1
        
        // Set defaults from policy
        if err := s.applyTaskDefaults(ctx, task); err != nil {
            return err
        }
        
        // Auto-assign if enabled
        if task.AssignedTo == "" && s.shouldAutoAssign(ctx, task) {
            agent, err := s.assignmentEngine.FindBestAgent(ctx, task)
            if err == nil && agent != nil {
                task.AssignedTo = agent.ID
                task.Status = models.TaskStatusAssigned
                task.AssignedAt = timePtr(time.Now())
            }
        }
        
        // Create task
        if err := s.repo.WithTx(tx).Create(ctx, task); err != nil {
            return errors.Wrap(err, "failed to create task")
        }
        
        // Store idempotency key
        if idempotencyKey != "" {
            if err := s.storeIdempotencyKey(ctx, idempotencyKey, task.ID); err != nil {
                return err
            }
        }
        
        // Publish event
        if err := s.PublishEvent(ctx, "TaskCreated", task, task); err != nil {
            return err
        }
        
        return nil
    })
    
    if err != nil {
        return err
    }
    
    // Post-creation actions (outside transaction)
    s.executePostCreationActions(ctx, task)
    
    return nil
}

// CreateDistributedTask creates a task with subtasks using saga pattern
func (s *taskService) CreateDistributedTask(ctx context.Context, dt *models.DistributedTask) error {
    span, ctx := s.tracer.Start(ctx, "TaskService.CreateDistributedTask")
    defer span.End()
    
    // Validate distributed task
    if err := s.validateDistributedTask(ctx, dt); err != nil {
        return err
    }
    
    // Start saga
    saga := NewTaskCreationSaga(s, dt)
    compensator := NewCompensator(s.logger)
    
    // Execute saga steps
    err := s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
        // Step 1: Create main task
        mainTask, err := saga.CreateMainTask(ctx, tx)
        if err != nil {
            return err
        }
        compensator.AddCompensation(func() error {
            return saga.DeleteMainTask(ctx, mainTask.ID)
        })
        
        // Step 2: Validate agent availability
        agentMap, err := saga.ValidateAgents(ctx, dt.Subtasks)
        if err != nil {
            return err
        }
        
        // Step 3: Create subtasks in parallel
        g, gctx := errgroup.WithContext(ctx)
        subtaskIDs := make([]uuid.UUID, len(dt.Subtasks))
        var mu sync.Mutex
        
        for i, subtaskDef := range dt.Subtasks {
            i := i
            subtaskDef := subtaskDef
            
            g.Go(func() error {
                subtask, err := saga.CreateSubtask(gctx, tx, mainTask, subtaskDef, agentMap[subtaskDef.AgentID])
                if err != nil {
                    return err
                }
                
                mu.Lock()
                subtaskIDs[i] = subtask.ID
                mu.Unlock()
                
                compensator.AddCompensation(func() error {
                    return saga.DeleteSubtask(ctx, subtask.ID)
                })
                
                return nil
            })
        }
        
        if err := g.Wait(); err != nil {
            return err
        }
        
        // Step 4: Update main task with subtask references
        mainTask.Parameters["subtask_ids"] = subtaskIDs
        if err := s.repo.WithTx(tx).Update(ctx, mainTask); err != nil {
            return err
        }
        
        // Step 5: Publish events
        if err := saga.PublishEvents(ctx); err != nil {
            return err
        }
        
        dt.ID = mainTask.ID
        dt.SubtaskIDs = subtaskIDs
        
        return nil
    })
    
    if err != nil {
        // Run compensations
        if compErr := compensator.Compensate(ctx); compErr != nil {
            s.logger.Error("Saga compensation failed", map[string]interface{}{
                "error": compErr,
                "original_error": err,
            })
        }
        return err
    }
    
    // Notify agents
    s.notifyDistributedTaskCreated(ctx, dt)
    
    return nil
}

// DelegateTask handles task delegation with policy enforcement
func (s *taskService) DelegateTask(ctx context.Context, delegation *models.TaskDelegation) error {
    span, ctx := s.tracer.Start(ctx, "TaskService.DelegateTask")
    defer span.End()
    
    // Rate limiting
    if err := s.CheckRateLimit(ctx, "task:delegate"); err != nil {
        return err
    }
    
    // Get task with lock
    task, err := s.repo.GetForUpdate(ctx, delegation.TaskID)
    if err != nil {
        return errors.Wrap(err, "failed to get task")
    }
    
    // Validate delegation
    if err := s.validateDelegation(ctx, task, delegation); err != nil {
        return err
    }
    
    // Check delegation policy
    decision, err := s.config.RuleEngine.Evaluate(ctx, "task.delegation", map[string]interface{}{
        "task":       task,
        "delegation": delegation,
        "from_agent": delegation.FromAgentID,
        "to_agent":   delegation.ToAgentID,
    })
    
    if err != nil {
        return errors.Wrap(err, "policy evaluation failed")
    }
    
    if !decision.Allowed {
        return ErrDelegationDenied{Reason: decision.Reason}
    }
    
    // Apply delegation
    return s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
        // Create delegation record
        delegation.ID = uuid.New()
        delegation.DelegatedAt = time.Now()
        
        if err := s.repo.WithTx(tx).CreateDelegation(ctx, delegation); err != nil {
            return errors.Wrap(err, "failed to create delegation")
        }
        
        // Update task
        previousAssignee := task.AssignedTo
        task.AssignedTo = delegation.ToAgentID
        task.Status = models.TaskStatusAssigned
        task.AssignedAt = timePtr(time.Now())
        task.Version++
        
        if err := s.repo.WithTx(tx).UpdateWithVersion(ctx, task, task.Version-1); err != nil {
            if errors.Is(err, repository.ErrOptimisticLock) {
                return ErrConcurrentModification
            }
            return errors.Wrap(err, "failed to update task")
        }
        
        // Publish event
        event := &TaskDelegatedEvent{
            Task:         task,
            Delegation:   delegation,
            PreviousAgent: previousAssignee,
        }
        
        if err := s.PublishEvent(ctx, "TaskDelegated", task, event); err != nil {
            return err
        }
        
        return nil
    })
}

// Helper methods

func (s *taskService) sanitizeTask(task *models.Task) error {
    // Sanitize string fields
    task.Title = s.config.Sanitizer.SanitizeString(task.Title)
    task.Description = s.config.Sanitizer.SanitizeString(task.Description)
    
    // Sanitize parameters
    if task.Parameters != nil {
        sanitized, err := s.config.Sanitizer.SanitizeJSON(task.Parameters)
        if err != nil {
            return err
        }
        task.Parameters = sanitized
    }
    
    return nil
}

func (s *taskService) validateTask(ctx context.Context, task *models.Task) error {
    // Basic validation
    if task.Type == "" {
        return ValidationError{Field: "type", Message: "required"}
    }
    
    if task.Title == "" {
        return ValidationError{Field: "title", Message: "required"}
    }
    
    if len(task.Title) > 500 {
        return ValidationError{Field: "title", Message: "exceeds maximum length"}
    }
    
    // Business rule validation
    rules, err := s.config.PolicyManager.GetRules(ctx, "task.validation")
    if err != nil {
        return err
    }
    
    for _, rule := range rules {
        result, err := rule.Evaluate(ctx, task)
        if err != nil {
            return err
        }
        
        if !result.Valid {
            return ValidationError{Field: result.Field, Message: result.Message}
        }
    }
    
    return nil
}

func (s *taskService) authorizeTaskCreation(ctx context.Context, task *models.Task) error {
    decision := s.config.Authorizer.Authorize(ctx, auth.Permission{
        Resource: "task",
        Action:   "create",
        Conditions: map[string]interface{}{
            "type":     task.Type,
            "priority": task.Priority,
        },
    })
    
    if !decision.Allowed {
        return ErrUnauthorized{
            Action: "create task",
            Reason: decision.Reason,
        }
    }
    
    return nil
}

func (s *taskService) applyTaskDefaults(ctx context.Context, task *models.Task) error {
    defaults, err := s.config.PolicyManager.GetDefaults(ctx, "task", task.Type)
    if err != nil {
        return err
    }
    
    if task.Priority == "" {
        task.Priority = defaults.GetString("priority", models.TaskPriorityNormal)
    }
    
    if task.MaxRetries == 0 {
        task.MaxRetries = defaults.GetInt("max_retries", 3)
    }
    
    if task.TimeoutSeconds == 0 {
        task.TimeoutSeconds = defaults.GetInt("timeout_seconds", 3600)
    }
    
    // Apply default parameters
    if defaultParams := defaults.GetMap("parameters"); defaultParams != nil {
        if task.Parameters == nil {
            task.Parameters = make(map[string]interface{})
        }
        
        for k, v := range defaultParams {
            if _, exists := task.Parameters[k]; !exists {
                task.Parameters[k] = v
            }
        }
    }
    
    return nil
}

func (s *taskService) executePostCreationActions(ctx context.Context, task *models.Task) {
    // Notify assigned agent
    if task.AssignedTo != "" {
        if err := s.notifier.NotifyTaskAssigned(ctx, task.AssignedTo, task); err != nil {
            s.logger.Error("Failed to notify agent", map[string]interface{}{
                "task_id":  task.ID,
                "agent_id": task.AssignedTo,
                "error":    err,
            })
        }
    }
    
    // Update caches
    s.taskCache.Delete(fmt.Sprintf("agent:%s:tasks", task.AssignedTo))
    s.statsCache.Delete(fmt.Sprintf("tenant:%s:stats", task.TenantID))
    
    // Schedule monitoring
    s.progressTracker.Track(task.ID)
    
    // Metrics
    s.metrics.Increment("task.created", 1, map[string]string{
        "type":     task.Type,
        "priority": string(task.Priority),
        "assigned": fmt.Sprintf("%t", task.AssignedTo != ""),
    })
}
```

### Assignment Engine

```go
// File: pkg/services/assignment_engine.go
package services

import (
    "context"
    "sort"
    "sync"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/S-Corkum/devops-mcp/pkg/rules"
)

// AssignmentStrategy defines how tasks are assigned to agents
type AssignmentStrategy interface {
    Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error)
}

// AssignmentEngine handles intelligent task assignment
type AssignmentEngine struct {
    ruleEngine   rules.Engine
    strategies   map[string]AssignmentStrategy
    scoreCache   sync.Map // map[string]float64 - agent scores
    
    // Metrics
    assignments  map[string]int
    assignmentMu sync.RWMutex
}

func NewAssignmentEngine(ruleEngine rules.Engine) *AssignmentEngine {
    e := &AssignmentEngine{
        ruleEngine:  ruleEngine,
        strategies:  make(map[string]AssignmentStrategy),
        assignments: make(map[string]int),
    }
    
    // Register default strategies
    e.RegisterStrategy("round_robin", &RoundRobinStrategy{})
    e.RegisterStrategy("least_loaded", &LeastLoadedStrategy{})
    e.RegisterStrategy("capability_match", &CapabilityMatchStrategy{})
    e.RegisterStrategy("performance_based", &PerformanceBasedStrategy{})
    e.RegisterStrategy("cost_optimized", &CostOptimizedStrategy{})
    
    return e
}

// FindBestAgent finds the best agent for a task using rules and strategies
func (e *AssignmentEngine) FindBestAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
    // Get assignment rules
    assignmentRules, err := e.ruleEngine.GetRules(ctx, "task.assignment", map[string]interface{}{
        "task_type": task.Type,
        "priority":  task.Priority,
    })
    
    if err != nil {
        return nil, err
    }
    
    // Get eligible agents
    eligibleAgents, err := e.getEligibleAgents(ctx, task, assignmentRules)
    if err != nil {
        return nil, err
    }
    
    if len(eligibleAgents) == 0 {
        return nil, ErrNoEligibleAgents
    }
    
    // Determine strategy
    strategy := e.determineStrategy(ctx, task, assignmentRules)
    
    // Apply strategy
    return strategy.Assign(ctx, task, eligibleAgents)
}

func (e *AssignmentEngine) getEligibleAgents(ctx context.Context, task *models.Task, rules []rules.Rule) ([]*models.Agent, error) {
    // Get all available agents
    agents, err := agentService.GetAvailableAgents(ctx)
    if err != nil {
        return nil, err
    }
    
    // Filter by rules
    var eligible []*models.Agent
    for _, agent := range agents {
        eligible := true
        
        for _, rule := range rules {
            result, err := rule.Evaluate(ctx, map[string]interface{}{
                "task":  task,
                "agent": agent,
            })
            
            if err != nil {
                continue
            }
            
            if !result.Satisfied {
                eligible = false
                break
            }
        }
        
        if eligible {
            eligible = append(eligible, agent)
        }
    }
    
    return eligible, nil
}

// CapabilityMatchStrategy assigns tasks based on agent capabilities
type CapabilityMatchStrategy struct{}

func (s *CapabilityMatchStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
    // Extract required capabilities
    requiredCaps := extractRequiredCapabilities(task)
    
    // Score agents based on capability match
    type agentScore struct {
        agent *models.Agent
        score float64
    }
    
    scores := make([]agentScore, 0, len(agents))
    
    for _, agent := range agents {
        score := calculateCapabilityScore(agent.Capabilities, requiredCaps)
        scores = append(scores, agentScore{agent: agent, score: score})
    }
    
    // Sort by score
    sort.Slice(scores, func(i, j int) bool {
        return scores[i].score > scores[j].score
    })
    
    if len(scores) > 0 && scores[0].score > 0 {
        return scores[0].agent, nil
    }
    
    return nil, ErrNoCapableAgent
}

// PerformanceBasedStrategy assigns tasks based on agent performance metrics
type PerformanceBasedStrategy struct {
    performanceCache sync.Map
}

func (s *PerformanceBasedStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
    var bestAgent *models.Agent
    bestScore := 0.0
    
    for _, agent := range agents {
        // Get cached performance or calculate
        var perf AgentPerformance
        if cached, ok := s.performanceCache.Load(agent.ID); ok {
            perf = cached.(AgentPerformance)
        } else {
            // Calculate performance metrics
            perf = calculateAgentPerformance(ctx, agent.ID, task.Type)
            s.performanceCache.Store(agent.ID, perf)
        }
        
        // Calculate score based on:
        // - Success rate
        // - Average completion time
        // - Current load
        score := perf.SuccessRate * 0.4 +
                (1.0 - perf.LoadFactor) * 0.3 +
                perf.SpeedScore * 0.3
        
        if score > bestScore {
            bestScore = score
            bestAgent = agent
        }
    }
    
    return bestAgent, nil
}
```

### Workflow Service with Saga Orchestration

```go
// File: pkg/services/workflow_service.go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/pkg/errors"
    "golang.org/x/sync/errgroup"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
)

// WorkflowService orchestrates multi-agent workflows with saga pattern
type WorkflowService interface {
    // Workflow management
    Create(ctx context.Context, workflow *models.Workflow) error
    Get(ctx context.Context, id uuid.UUID) (*models.Workflow, error)
    Update(ctx context.Context, workflow *models.Workflow) error
    Delete(ctx context.Context, id uuid.UUID) error
    List(ctx context.Context, filters repository.WorkflowFilters) (*repository.WorkflowPage, error)
    
    // Version control
    CreateVersion(ctx context.Context, workflowID uuid.UUID, changes string) (*models.Workflow, error)
    GetVersion(ctx context.Context, workflowID uuid.UUID, version int) (*models.Workflow, error)
    ListVersions(ctx context.Context, workflowID uuid.UUID) ([]*models.WorkflowVersion, error)
    
    // Execution with monitoring
    Execute(ctx context.Context, workflowID uuid.UUID, input map[string]interface{}) (*models.WorkflowExecution, error)
    ExecuteWithContext(ctx context.Context, execution *models.WorkflowExecutionRequest) (*models.WorkflowExecution, error)
    GetExecution(ctx context.Context, executionID uuid.UUID) (*models.WorkflowExecution, error)
    ListExecutions(ctx context.Context, workflowID uuid.UUID, filters repository.ExecutionFilters) (*repository.ExecutionPage, error)
    
    // Execution control
    PauseExecution(ctx context.Context, executionID uuid.UUID) error
    ResumeExecution(ctx context.Context, executionID uuid.UUID) error
    CancelExecution(ctx context.Context, executionID uuid.UUID, reason string) error
    RetryExecution(ctx context.Context, executionID uuid.UUID, fromStep string) error
    
    // Collaborative workflows
    CreateCollaborative(ctx context.Context, workflow *models.CollaborativeWorkflow) error
    ExecuteCollaborative(ctx context.Context, workflowID uuid.UUID, input map[string]interface{}) (*models.WorkflowExecution, error)
    
    // Step management
    CompleteStep(ctx context.Context, executionID uuid.UUID, stepID string, output interface{}) error
    FailStep(ctx context.Context, executionID uuid.UUID, stepID string, error string) error
    GetNextSteps(ctx context.Context, executionID uuid.UUID) ([]*models.WorkflowStep, error)
    GetStepStatus(ctx context.Context, executionID uuid.UUID, stepID string) (*models.StepStatus, error)
    
    // Analytics
    GetWorkflowMetrics(ctx context.Context, workflowID uuid.UUID) (*models.WorkflowMetrics, error)
    GetExecutionTrace(ctx context.Context, executionID uuid.UUID) (*models.ExecutionTrace, error)
    GetWorkflowInsights(ctx context.Context, workflowID uuid.UUID, period time.Duration) (*models.WorkflowInsights, error)
    
    // Template management
    CreateTemplate(ctx context.Context, template *models.WorkflowTemplate) error
    GetTemplate(ctx context.Context, templateID uuid.UUID) (*models.WorkflowTemplate, error)
    ListTemplates(ctx context.Context, category string) ([]*models.WorkflowTemplate, error)
    CreateFromTemplate(ctx context.Context, templateID uuid.UUID, params map[string]interface{}) (*models.Workflow, error)
}

type workflowService struct {
    BaseService
    
    // Dependencies
    repo            repository.WorkflowRepository
    taskService     TaskService
    agentService    AgentService
    notifier        NotificationService
    
    // Execution management
    executor        *WorkflowExecutor
    orchestrator    *SagaOrchestrator
    scheduler       *WorkflowScheduler
    
    // State management
    executionCache  cache.Cache
    stepCache       cache.Cache
    
    // Active executions
    activeExecutions sync.Map // map[uuid.UUID]*ExecutionContext
}

// WorkflowExecutor handles workflow execution with resilience
type WorkflowExecutor struct {
    service         *workflowService
    sagaOrchestrator *SagaOrchestrator
    circuitBreaker  resilience.CircuitBreaker
    retryPolicy     resilience.RetryPolicy
}

// Execute runs a workflow with full monitoring and resilience
func (e *WorkflowExecutor) Execute(ctx context.Context, workflow *models.Workflow, input map[string]interface{}) (*models.WorkflowExecution, error) {
    // Create execution record
    execution := &models.WorkflowExecution{
        ID:          uuid.New(),
        WorkflowID:  workflow.ID,
        TenantID:    workflow.TenantID,
        Status:      models.WorkflowStatusPending,
        Input:       input,
        Context:     make(map[string]interface{}),
        StepResults: make(map[string]interface{}),
        TriggeredBy: auth.GetAgentID(ctx),
        StartedAt:   time.Now(),
    }
    
    // Initialize execution context
    execCtx := &ExecutionContext{
        execution:    execution,
        workflow:     workflow,
        steps:        make(map[string]*StepContext),
        saga:         e.sagaOrchestrator.CreateSaga(execution.ID),
        compensator:  NewCompensator(e.service.logger),
        eventBus:     NewEventBus(),
        mu:           sync.RWMutex{},
    }
    
    // Store in active executions
    e.service.activeExecutions.Store(execution.ID, execCtx)
    defer e.service.activeExecutions.Delete(execution.ID)
    
    // Execute workflow
    err := e.executeWorkflow(ctx, execCtx)
    
    // Update final status
    if err != nil {
        execution.Status = models.WorkflowStatusFailed
        execution.Error = err.Error()
    } else {
        execution.Status = models.WorkflowStatusCompleted
    }
    execution.CompletedAt = timePtr(time.Now())
    
    // Save execution
    if saveErr := e.service.repo.UpdateExecution(ctx, execution); saveErr != nil {
        e.service.logger.Error("Failed to save execution", map[string]interface{}{
            "execution_id": execution.ID,
            "error":        saveErr,
        })
    }
    
    return execution, err
}

func (e *WorkflowExecutor) executeWorkflow(ctx context.Context, execCtx *ExecutionContext) error {
    // Validate agents availability
    if err := e.validateAgents(ctx, execCtx); err != nil {
        return err
    }
    
    // Execute based on workflow type
    switch execCtx.workflow.Type {
    case models.WorkflowTypeSequential:
        return e.executeSequential(ctx, execCtx)
    case models.WorkflowTypeParallel:
        return e.executeParallel(ctx, execCtx)
    case models.WorkflowTypeConditional:
        return e.executeConditional(ctx, execCtx)
    case models.WorkflowTypeCollaborative:
        return e.executeCollaborative(ctx, execCtx)
    default:
        return fmt.Errorf("unsupported workflow type: %s", execCtx.workflow.Type)
    }
}

func (e *WorkflowExecutor) executeSequential(ctx context.Context, execCtx *ExecutionContext) error {
    steps := execCtx.workflow.Steps
    
    for _, step := range steps {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        
        // Check if step should be skipped
        if e.shouldSkipStep(execCtx, step) {
            continue
        }
        
        // Execute step with saga
        stepCtx := e.createStepContext(execCtx, step)
        
        err := execCtx.saga.ExecuteStep(ctx, step.ID, func(ctx context.Context) error {
            return e.executeStep(ctx, execCtx, stepCtx)
        }, func(ctx context.Context) error {
            return e.compensateStep(ctx, execCtx, stepCtx)
        })
        
        if err != nil {
            // Handle step failure
            if e.shouldRetryStep(stepCtx, err) {
                err = e.retryStep(ctx, execCtx, stepCtx)
            }
            
            if err != nil && !step.ContinueOnError {
                return err
            }
        }
        
        // Update execution state
        execCtx.updateStepResult(step.ID, stepCtx.output)
    }
    
    return nil
}

func (e *WorkflowExecutor) executeParallel(ctx context.Context, execCtx *ExecutionContext) error {
    g, gctx := errgroup.WithContext(ctx)
    
    for _, step := range execCtx.workflow.Steps {
        step := step // Capture loop variable
        
        g.Go(func() error {
            // Check dependencies
            if err := e.waitForDependencies(gctx, execCtx, step); err != nil {
                return err
            }
            
            stepCtx := e.createStepContext(execCtx, step)
            
            return execCtx.saga.ExecuteStep(gctx, step.ID, func(ctx context.Context) error {
                return e.executeStep(ctx, execCtx, stepCtx)
            }, func(ctx context.Context) error {
                return e.compensateStep(ctx, execCtx, stepCtx)
            })
        })
    }
    
    return g.Wait()
}

func (e *WorkflowExecutor) executeStep(ctx context.Context, execCtx *ExecutionContext, stepCtx *StepContext) error {
    step := stepCtx.step
    
    // Update status
    stepCtx.status = models.StepStatusRunning
    stepCtx.startedAt = time.Now()
    
    // Notify step started
    e.service.notifier.NotifyStepStarted(ctx, execCtx.execution.ID, step.ID)
    
    // Create task for step
    task := &models.Task{
        Type:        fmt.Sprintf("workflow_step_%s", step.Action),
        Title:       fmt.Sprintf("%s - %s", execCtx.workflow.Name, step.Name),
        Description: step.Description,
        Priority:    e.mapStepPriority(step),
        AssignedTo:  step.AgentID,
        Parameters: map[string]interface{}{
            "workflow_id":   execCtx.workflow.ID,
            "execution_id":  execCtx.execution.ID,
            "step_id":       step.ID,
            "step_input":    stepCtx.input,
            "step_config":   step.Config,
        },
    }
    
    // Execute with timeout
    timeoutCtx, cancel := context.WithTimeout(ctx, step.Timeout)
    defer cancel()
    
    // Create and execute task
    err := e.service.taskService.Create(timeoutCtx, task, "")
    if err != nil {
        return err
    }
    
    // Wait for task completion
    output, err := e.waitForTaskCompletion(timeoutCtx, task.ID)
    if err != nil {
        stepCtx.status = models.StepStatusFailed
        stepCtx.error = err.Error()
        return err
    }
    
    // Update step context
    stepCtx.status = models.StepStatusCompleted
    stepCtx.output = output
    stepCtx.completedAt = timePtr(time.Now())
    
    // Notify step completed
    e.service.notifier.NotifyStepCompleted(ctx, execCtx.execution.ID, step.ID, output)
    
    return nil
}

// SagaOrchestrator manages distributed transactions
type SagaOrchestrator struct {
    logger       observability.Logger
    eventStore   events.Store
    stateStore   StateStore
    compensators sync.Map // map[uuid.UUID]*Compensator
}

func (o *SagaOrchestrator) CreateSaga(executionID uuid.UUID) *Saga {
    saga := &Saga{
        ID:           uuid.New(),
        ExecutionID:  executionID,
        Steps:        make([]SagaStep, 0),
        Compensators: make(map[string]CompensationFunc),
        State:        SagaStatePending,
    }
    
    o.compensators.Store(saga.ID, saga)
    return saga
}

type Saga struct {
    ID           uuid.UUID
    ExecutionID  uuid.UUID
    Steps        []SagaStep
    Compensators map[string]CompensationFunc
    State        SagaState
    mu           sync.Mutex
}

func (s *Saga) ExecuteStep(ctx context.Context, stepID string, action ActionFunc, compensation CompensationFunc) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Record step
    step := SagaStep{
        ID:        stepID,
        StartedAt: time.Now(),
        State:     SagaStepStateRunning,
    }
    s.Steps = append(s.Steps, step)
    
    // Store compensator
    s.Compensators[stepID] = compensation
    
    // Execute action
    err := action(ctx)
    
    // Update step state
    for i, st := range s.Steps {
        if st.ID == stepID {
            if err != nil {
                s.Steps[i].State = SagaStepStateFailed
                s.Steps[i].Error = err.Error()
            } else {
                s.Steps[i].State = SagaStepStateCompleted
            }
            s.Steps[i].CompletedAt = timePtr(time.Now())
            break
        }
    }
    
    return err
}

func (s *Saga) Compensate(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.State = SagaStateCompensating
    
    // Execute compensations in reverse order
    for i := len(s.Steps) - 1; i >= 0; i-- {
        step := s.Steps[i]
        
        if step.State != SagaStepStateCompleted {
            continue
        }
        
        if compensator, ok := s.Compensators[step.ID]; ok {
            if err := compensator(ctx); err != nil {
                s.logger.Error("Compensation failed", map[string]interface{}{
                    "saga_id": s.ID,
                    "step_id": step.ID,
                    "error":   err,
                })
            }
        }
    }
    
    s.State = SagaStateCompensated
    return nil
}
```

### Workspace Service with Distributed State Management

```go
// File: pkg/services/workspace_service.go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/pkg/errors"
    
    "github.com/S-Corkum/devops-mcp/pkg/collaboration"
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
)

// WorkspaceService manages shared workspaces with distributed state
type WorkspaceService interface {
    // Workspace lifecycle
    Create(ctx context.Context, workspace *models.Workspace) error
    Get(ctx context.Context, id uuid.UUID) (*models.Workspace, error)
    Update(ctx context.Context, workspace *models.Workspace) error
    Delete(ctx context.Context, id uuid.UUID) error
    Archive(ctx context.Context, id uuid.UUID) error
    
    // Member management with permissions
    AddMember(ctx context.Context, member *models.WorkspaceMember) error
    RemoveMember(ctx context.Context, workspaceID uuid.UUID, agentID string) error
    UpdateMemberRole(ctx context.Context, workspaceID uuid.UUID, agentID string, role string) error
    UpdateMemberPermissions(ctx context.Context, workspaceID uuid.UUID, agentID string, permissions []string) error
    ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*models.WorkspaceMember, error)
    GetMemberActivity(ctx context.Context, workspaceID uuid.UUID) ([]*models.MemberActivity, error)
    
    // State management with CRDT
    GetState(ctx context.Context, workspaceID uuid.UUID) (*models.WorkspaceState, error)
    UpdateState(ctx context.Context, workspaceID uuid.UUID, operation *models.StateOperation) error
    MergeState(ctx context.Context, workspaceID uuid.UUID, remoteState *models.WorkspaceState) error
    GetStateHistory(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*models.StateSnapshot, error)
    RestoreState(ctx context.Context, workspaceID uuid.UUID, snapshotID uuid.UUID) error
    
    // Document management
    CreateDocument(ctx context.Context, doc *models.SharedDocument) error
    GetDocument(ctx context.Context, docID uuid.UUID) (*models.SharedDocument, error)
    UpdateDocument(ctx context.Context, docID uuid.UUID, operation *collaboration.DocumentOperation) error
    ListDocuments(ctx context.Context, workspaceID uuid.UUID) ([]*models.SharedDocument, error)
    
    // Real-time collaboration
    BroadcastToMembers(ctx context.Context, workspaceID uuid.UUID, message interface{}) error
    SendToMember(ctx context.Context, workspaceID uuid.UUID, agentID string, message interface{}) error
    GetPresence(ctx context.Context, workspaceID uuid.UUID) ([]*models.MemberPresence, error)
    UpdatePresence(ctx context.Context, workspaceID uuid.UUID, agentID string, status string) error
    
    // Search and discovery
    ListByAgent(ctx context.Context, agentID string) ([]*models.Workspace, error)
    SearchWorkspaces(ctx context.Context, query string, filters repository.WorkspaceFilters) (*repository.WorkspaceSearchResult, error)
    GetRecommendedWorkspaces(ctx context.Context, agentID string) ([]*models.Workspace, error)
    
    // Analytics
    GetWorkspaceStats(ctx context.Context, workspaceID uuid.UUID) (*models.WorkspaceStats, error)
    GetCollaborationMetrics(ctx context.Context, workspaceID uuid.UUID, period time.Duration) (*models.CollaborationMetrics, error)
}

type workspaceService struct {
    BaseService
    
    // Dependencies
    repo             repository.WorkspaceRepository
    docRepo          repository.DocumentRepository
    notifier         NotificationService
    
    // State management
    stateManager     *DistributedStateManager
    conflictResolver *ConflictResolver
    
    // Real-time
    presenceTracker  *PresenceTracker
    eventBroadcaster *EventBroadcaster
    
    // Caching
    workspaceCache   cache.Cache
    stateCache       cache.Cache
    
    // Active workspaces
    activeWorkspaces sync.Map // map[uuid.UUID]*WorkspaceContext
}

// DistributedStateManager handles workspace state with CRDT
type DistributedStateManager struct {
    store            StateStore
    vectorClockManager *collaboration.VectorClockManager
    crdtEngine       *collaboration.CRDTEngine
    logger           observability.Logger
}

func (m *DistributedStateManager) UpdateState(ctx context.Context, workspaceID uuid.UUID, agentID string, operation *models.StateOperation) error {
    // Get current state with lock
    state, err := m.store.GetStateForUpdate(ctx, workspaceID)
    if err != nil {
        return err
    }
    
    // Update vector clock
    state.VectorClock.Increment(agentID)
    
    // Apply operation based on type
    switch operation.Type {
    case "set":
        state.Data = m.crdtEngine.Set(state.Data, operation.Path, operation.Value, state.VectorClock)
    case "increment":
        state.Data = m.crdtEngine.Increment(state.Data, operation.Path, operation.Delta, state.VectorClock)
    case "add_to_set":
        state.Data = m.crdtEngine.AddToSet(state.Data, operation.Path, operation.Value, state.VectorClock)
    case "remove_from_set":
        state.Data = m.crdtEngine.RemoveFromSet(state.Data, operation.Path, operation.Value, state.VectorClock)
    default:
        return fmt.Errorf("unsupported operation type: %s", operation.Type)
    }
    
    // Increment version
    state.Version++
    state.LastModifiedBy = agentID
    state.LastModifiedAt = time.Now()
    
    // Save state
    if err := m.store.SaveState(ctx, workspaceID, state); err != nil {
        return err
    }
    
    // Create snapshot if needed
    if state.Version%100 == 0 {
        go m.createSnapshot(context.Background(), workspaceID, state)
    }
    
    return nil
}

func (m *DistributedStateManager) MergeState(ctx context.Context, workspaceID uuid.UUID, remoteState *models.WorkspaceState) error {
    // Get local state
    localState, err := m.store.GetState(ctx, workspaceID)
    if err != nil {
        return err
    }
    
    // Compare vector clocks
    comparison := localState.VectorClock.Compare(remoteState.VectorClock)
    
    switch comparison {
    case collaboration.ClockEqual:
        // States are identical
        return nil
        
    case collaboration.ClockBefore:
        // Remote is newer, accept it
        return m.store.SaveState(ctx, workspaceID, remoteState)
        
    case collaboration.ClockAfter:
        // Local is newer, keep it
        return nil
        
    case collaboration.ClockConcurrent:
        // Concurrent changes, merge required
        mergedState := m.crdtEngine.Merge(localState, remoteState)
        mergedState.Version = max(localState.Version, remoteState.Version) + 1
        
        return m.store.SaveState(ctx, workspaceID, mergedState)
    }
    
    return nil
}

// ConflictResolver handles merge conflicts
type ConflictResolver struct {
    strategies map[string]MergeStrategy
    logger     observability.Logger
}

type MergeStrategy interface {
    CanResolve(conflict *MergeConflict) bool
    Resolve(ctx context.Context, conflict *MergeConflict) (interface{}, error)
}

// LastWriteWinsStrategy resolves conflicts by choosing the most recent write
type LastWriteWinsStrategy struct{}

func (s *LastWriteWinsStrategy) CanResolve(conflict *MergeConflict) bool {
    return true // Can resolve any conflict
}

func (s *LastWriteWinsStrategy) Resolve(ctx context.Context, conflict *MergeConflict) (interface{}, error) {
    if conflict.LocalTimestamp.After(conflict.RemoteTimestamp) {
        return conflict.LocalValue, nil
    }
    return conflict.RemoteValue, nil
}

// ThreeWayMergeStrategy performs a three-way merge using a common ancestor
type ThreeWayMergeStrategy struct {
    ancestorStore AncestorStore
}

func (s *ThreeWayMergeStrategy) CanResolve(conflict *MergeConflict) bool {
    // Can only resolve if we have a common ancestor
    _, err := s.ancestorStore.GetCommonAncestor(conflict.LocalVersion, conflict.RemoteVersion)
    return err == nil
}

func (s *ThreeWayMergeStrategy) Resolve(ctx context.Context, conflict *MergeConflict) (interface{}, error) {
    ancestor, err := s.ancestorStore.GetCommonAncestor(conflict.LocalVersion, conflict.RemoteVersion)
    if err != nil {
        return nil, err
    }
    
    // Perform three-way merge
    return performThreeWayMerge(ancestor, conflict.LocalValue, conflict.RemoteValue)
}

// Create creates a new workspace with validation
func (s *workspaceService) Create(ctx context.Context, workspace *models.Workspace) error {
    span, ctx := s.tracer.Start(ctx, "WorkspaceService.Create")
    defer span.End()
    
    // Rate limiting
    if err := s.CheckRateLimit(ctx, "workspace:create"); err != nil {
        return err
    }
    
    // Check quota
    if err := s.CheckQuota(ctx, "workspaces", 1); err != nil {
        return err
    }
    
    // Validate workspace
    if err := s.validateWorkspace(ctx, workspace); err != nil {
        return err
    }
    
    // Check authorization
    if err := s.authorizeWorkspaceCreation(ctx, workspace); err != nil {
        return err
    }
    
    return s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
        // Set metadata
        workspace.ID = uuid.New()
        workspace.TenantID = auth.GetTenantID(ctx)
        workspace.OwnerID = auth.GetAgentID(ctx)
        workspace.CreatedAt = time.Now()
        workspace.StateVersion = 1
        
        // Set defaults
        if workspace.Type == "" {
            workspace.Type = models.WorkspaceTypeGeneral
        }
        
        if workspace.Visibility == "" {
            workspace.Visibility = models.VisibilityPrivate
        }
        
        if workspace.MaxMembers == 0 {
            workspace.MaxMembers = s.getDefaultMaxMembers(workspace.Type)
        }
        
        // Initialize state with CRDT
        if workspace.State == nil {
            workspace.State = make(map[string]interface{})
        }
        
        // Create workspace
        if err := s.repo.WithTx(tx).Create(ctx, workspace); err != nil {
            return errors.Wrap(err, "failed to create workspace")
        }
        
        // Add creator as owner
        owner := &models.WorkspaceMember{
            WorkspaceID: workspace.ID,
            AgentID:     workspace.OwnerID,
            TenantID:    workspace.TenantID,
            Role:        models.RoleOwner,
            Permissions: s.getOwnerPermissions(),
        }
        
        if err := s.repo.WithTx(tx).AddMember(ctx, owner); err != nil {
            return errors.Wrap(err, "failed to add owner")
        }
        
        // Initialize workspace context
        s.initializeWorkspaceContext(workspace)
        
        // Publish event
        if err := s.PublishEvent(ctx, "WorkspaceCreated", workspace, workspace); err != nil {
            return err
        }
        
        return nil
    })
}

// UpdateState updates workspace state with conflict resolution
func (s *workspaceService) UpdateState(ctx context.Context, workspaceID uuid.UUID, operation *models.StateOperation) error {
    span, ctx := s.tracer.Start(ctx, "WorkspaceService.UpdateState")
    defer span.End()
    
    // Get workspace context
    wctx, err := s.getWorkspaceContext(ctx, workspaceID)
    if err != nil {
        return err
    }
    
    // Check permissions
    agentID := auth.GetAgentID(ctx)
    if !wctx.hasPermission(agentID, "state:write") {
        return ErrInsufficientPermissions
    }
    
    // Apply distributed lock
    lock, err := s.acquireStateLock(ctx, workspaceID, 5*time.Second)
    if err != nil {
        return errors.Wrap(err, "failed to acquire state lock")
    }
    defer lock.Release()
    
    // Update state with CRDT
    if err := s.stateManager.UpdateState(ctx, workspaceID, agentID, operation); err != nil {
        return err
    }
    
    // Broadcast state change
    s.broadcastStateChange(ctx, wctx, operation)
    
    return nil
}

func (s *workspaceService) initializeWorkspaceContext(workspace *models.Workspace) {
    ctx := &WorkspaceContext{
        workspace:        workspace,
        members:          make(map[string]*models.WorkspaceMember),
        presence:         make(map[string]*models.MemberPresence),
        stateSubscribers: make(map[string]chan<- StateUpdate),
        eventBus:         NewEventBus(),
        mu:               sync.RWMutex{},
    }
    
    s.activeWorkspaces.Store(workspace.ID, ctx)
    
    // Start presence tracking
    go s.presenceTracker.TrackWorkspace(workspace.ID)
}

func (s *workspaceService) broadcastStateChange(ctx context.Context, wctx *WorkspaceContext, operation *models.StateOperation) {
    update := StateUpdate{
        WorkspaceID: wctx.workspace.ID,
        Operation:   operation,
        Timestamp:   time.Now(),
        AgentID:     auth.GetAgentID(ctx),
    }
    
    wctx.mu.RLock()
    subscribers := make([]chan<- StateUpdate, 0, len(wctx.stateSubscribers))
    for _, ch := range wctx.stateSubscribers {
        subscribers = append(subscribers, ch)
    }
    wctx.mu.RUnlock()
    
    // Non-blocking send to all subscribers
    for _, ch := range subscribers {
        select {
        case ch <- update:
        default:
            // Subscriber is not ready, skip
        }
    }
}
```

## Event Sourcing and CQRS

```go
// File: pkg/services/event_sourcing.go
package services

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
    "github.com/S-Corkum/devops-mcp/pkg/events"
)

// EventStore provides event persistence
type EventStore interface {
    Append(ctx context.Context, event *events.DomainEvent) error
    GetEvents(ctx context.Context, aggregateID uuid.UUID, fromVersion int) ([]*events.DomainEvent, error)
    GetEventsByType(ctx context.Context, eventType string, from time.Time) ([]*events.DomainEvent, error)
    CreateSnapshot(ctx context.Context, aggregateID uuid.UUID, snapshot interface{}) error
    GetSnapshot(ctx context.Context, aggregateID uuid.UUID) (interface{}, error)
}

// EventProjector handles event projections for read models
type EventProjector interface {
    Project(ctx context.Context, event *events.DomainEvent) error
    Rebuild(ctx context.Context, aggregateID uuid.UUID) error
    GetProjection(ctx context.Context, projectionID string) (interface{}, error)
}

// AggregateRoot base for event-sourced entities
type AggregateRoot interface {
    GetID() uuid.UUID
    GetType() string
    GetVersion() int
    ApplyEvent(event *events.DomainEvent)
    GetUncommittedEvents() []*events.DomainEvent
    MarkEventsAsCommitted()
}

// BaseAggregate provides common aggregate functionality
type BaseAggregate struct {
    ID                uuid.UUID
    Type              string
    Version           int
    uncommittedEvents []*events.DomainEvent
}

func (a *BaseAggregate) RecordEvent(eventType string, data interface{}) {
    event := &events.DomainEvent{
        ID:            uuid.New(),
        Type:          eventType,
        AggregateID:   a.ID,
        AggregateType: a.Type,
        Version:       a.Version + 1,
        Timestamp:     time.Now(),
        Data:          data,
    }
    
    a.uncommittedEvents = append(a.uncommittedEvents, event)
    a.Version++
}

// EventSourcingRepository provides event-sourced persistence
type EventSourcingRepository struct {
    eventStore EventStore
    projector  EventProjector
    logger     observability.Logger
}

func (r *EventSourcingRepository) Save(ctx context.Context, aggregate AggregateRoot) error {
    // Get uncommitted events
    events := aggregate.GetUncommittedEvents()
    
    // Append events to store
    for _, event := range events {
        if err := r.eventStore.Append(ctx, event); err != nil {
            return err
        }
        
        // Project event
        if err := r.projector.Project(ctx, event); err != nil {
            r.logger.Error("Projection failed", map[string]interface{}{
                "event_id": event.ID,
                "error":    err,
            })
        }
    }
    
    // Mark events as committed
    aggregate.MarkEventsAsCommitted()
    
    // Create snapshot periodically
    if aggregate.GetVersion()%100 == 0 {
        go r.createSnapshot(context.Background(), aggregate)
    }
    
    return nil
}

func (r *EventSourcingRepository) Load(ctx context.Context, aggregateID uuid.UUID, aggregate AggregateRoot) error {
    // Try to load from snapshot
    snapshot, err := r.eventStore.GetSnapshot(ctx, aggregateID)
    if err == nil && snapshot != nil {
        // Restore from snapshot
        if err := r.restoreFromSnapshot(aggregate, snapshot); err != nil {
            return err
        }
    }
    
    // Load events after snapshot
    events, err := r.eventStore.GetEvents(ctx, aggregateID, aggregate.GetVersion())
    if err != nil {
        return err
    }
    
    // Apply events
    for _, event := range events {
        aggregate.ApplyEvent(event)
    }
    
    return nil
}
```

## Rate Limiting and Quota Management

```go
// File: pkg/services/rate_limiter.go
package services

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/go-redis/redis_rate/v10"
    "github.com/google/uuid"
)

// RateLimiter provides rate limiting functionality
type RateLimiter interface {
    Check(ctx context.Context, key string) error
    CheckWithLimit(ctx context.Context, key string, limit int, window time.Duration) error
    GetRemaining(ctx context.Context, key string) (int, error)
    Reset(ctx context.Context, key string) error
}

// RedisRateLimiter implements RateLimiter using Redis
type RedisRateLimiter struct {
    limiter  *redis_rate.Limiter
    configs  map[string]RateLimitConfig
    mu       sync.RWMutex
}

type RateLimitConfig struct {
    Limit    int
    Window   time.Duration
    BurstSize int
}

func NewRedisRateLimiter(client *redis.Client) RateLimiter {
    rl := &RedisRateLimiter{
        limiter: redis_rate.NewLimiter(client),
        configs: make(map[string]RateLimitConfig),
    }
    
    // Load default configurations
    rl.loadDefaultConfigs()
    
    return rl
}

func (rl *RedisRateLimiter) Check(ctx context.Context, key string) error {
    // Get configuration for key pattern
    config := rl.getConfig(key)
    
    res, err := rl.limiter.Allow(ctx, key, redis_rate.PerDuration(config.Limit, config.Window))
    if err != nil {
        return err
    }
    
    if res.Allowed == 0 {
        return ErrRateLimitExceeded{
            Key:       key,
            Limit:     config.Limit,
            Window:    config.Window,
            RetryAfter: res.RetryAfter,
        }
    }
    
    return nil
}

func (rl *RedisRateLimiter) loadDefaultConfigs() {
    rl.configs["tenant:*:task:create"] = RateLimitConfig{
        Limit:    1000,
        Window:   time.Hour,
        BurstSize: 100,
    }
    
    rl.configs["agent:*:task:create"] = RateLimitConfig{
        Limit:    100,
        Window:   time.Hour,
        BurstSize: 10,
    }
    
    rl.configs["tenant:*:workspace:create"] = RateLimitConfig{
        Limit:    50,
        Window:   time.Hour,
        BurstSize: 5,
    }
}

// QuotaManager manages resource quotas
type QuotaManager interface {
    GetQuota(ctx context.Context, tenantID uuid.UUID, resource string) (int64, error)
    GetUsage(ctx context.Context, tenantID uuid.UUID, resource string) (int64, error)
    IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int64) error
    SetQuota(ctx context.Context, tenantID uuid.UUID, resource string, limit int64) error
    GetQuotaStatus(ctx context.Context, tenantID uuid.UUID) (*QuotaStatus, error)
}

type QuotaStatus struct {
    TenantID uuid.UUID
    Quotas   map[string]QuotaInfo
}

type QuotaInfo struct {
    Resource string
    Limit    int64
    Used     int64
    Available int64
    Period   string
}

// RedisQuotaManager implements QuotaManager using Redis
type RedisQuotaManager struct {
    client   *redis.Client
    repo     repository.QuotaRepository
    notifier NotificationService
    logger   observability.Logger
}

func (m *RedisQuotaManager) IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int64) error {
    // Get current usage and quota
    quota, err := m.GetQuota(ctx, tenantID, resource)
    if err != nil {
        return err
    }
    
    key := fmt.Sprintf("quota:%s:%s:usage", tenantID, resource)
    
    // Increment atomically
    newUsage, err := m.client.IncrBy(ctx, key, amount).Result()
    if err != nil {
        return err
    }
    
    // Check if exceeded
    if newUsage > quota {
        // Rollback
        m.client.DecrBy(ctx, key, amount)
        
        // Notify quota exceeded
        m.notifyQuotaExceeded(ctx, tenantID, resource, quota, newUsage)
        
        return ErrQuotaExceeded{
            TenantID: tenantID,
            Resource: resource,
            Limit:    quota,
            Current:  newUsage,
        }
    }
    
    // Check if approaching limit
    if float64(newUsage) > float64(quota)*0.8 {
        m.notifyQuotaWarning(ctx, tenantID, resource, quota, newUsage)
    }
    
    return nil
}
```

## Business Rule Engine

```go
// File: pkg/rules/engine.go
package rules

import (
    "context"
    "fmt"
    
    "github.com/antonmedv/expr"
    "github.com/google/uuid"
)

// Engine evaluates business rules
type Engine interface {
    Evaluate(ctx context.Context, ruleName string, data interface{}) (*Decision, error)
    GetRules(ctx context.Context, category string, filters map[string]interface{}) ([]Rule, error)
    RegisterRule(ctx context.Context, rule Rule) error
    UpdateRule(ctx context.Context, ruleID uuid.UUID, updates map[string]interface{}) error
}

// Rule represents a business rule
type Rule struct {
    ID          uuid.UUID
    Name        string
    Category    string
    Expression  string
    Priority    int
    Enabled     bool
    Metadata    map[string]interface{}
}

// Decision represents the result of rule evaluation
type Decision struct {
    Allowed  bool
    Reason   string
    Score    float64
    Metadata map[string]interface{}
}

// ExprEngine implements Engine using expr library
type ExprEngine struct {
    repo      RuleRepository
    cache     cache.Cache
    compiler  *expr.Env
    logger    observability.Logger
}

func NewExprEngine(repo RuleRepository) Engine {
    env := expr.Env(
        // Register custom functions
        expr.Function("hasCapability", hasCapability),
        expr.Function("isAvailable", isAvailable),
        expr.Function("workloadScore", workloadScore),
        expr.Function("performanceScore", performanceScore),
    )
    
    return &ExprEngine{
        repo:     repo,
        cache:    cache.NewLRU(1000, 10*time.Minute),
        compiler: &env,
    }
}

func (e *ExprEngine) Evaluate(ctx context.Context, ruleName string, data interface{}) (*Decision, error) {
    // Get rules for category
    rules, err := e.getRulesWithCache(ctx, ruleName)
    if err != nil {
        return nil, err
    }
    
    // Sort by priority
    sort.Slice(rules, func(i, j int) bool {
        return rules[i].Priority > rules[j].Priority
    })
    
    // Evaluate rules
    for _, rule := range rules {
        if !rule.Enabled {
            continue
        }
        
        // Compile expression
        program, err := expr.Compile(rule.Expression, e.compiler)
        if err != nil {
            e.logger.Error("Failed to compile rule", map[string]interface{}{
                "rule_id": rule.ID,
                "error":   err,
            })
            continue
        }
        
        // Run expression
        result, err := expr.Run(program, data)
        if err != nil {
            e.logger.Error("Failed to evaluate rule", map[string]interface{}{
                "rule_id": rule.ID,
                "error":   err,
            })
            continue
        }
        
        // Process result
        decision := e.processResult(rule, result)
        if !decision.Allowed {
            return decision, nil
        }
    }
    
    return &Decision{Allowed: true}, nil
}

// PolicyManager manages dynamic policies
type PolicyManager interface {
    GetPolicy(ctx context.Context, policyName string) (*Policy, error)
    GetDefaults(ctx context.Context, resource, resourceType string) (Defaults, error)
    UpdatePolicy(ctx context.Context, policy *Policy) error
    ValidatePolicy(ctx context.Context, policy *Policy) error
}

type Policy struct {
    ID       uuid.UUID
    Name     string
    Resource string
    Rules    []PolicyRule
    Defaults map[string]interface{}
    Version  int
}

type PolicyRule struct {
    Condition string
    Effect    string // "allow" or "deny"
    Actions   []string
    Resources []string
}
```

## Error Types

```go
// File: pkg/services/errors.go
package services

import (
    "fmt"
    "time"
    
    "github.com/google/uuid"
)

// Service errors
var (
    ErrRateLimitExceeded    = errors.New("rate limit exceeded")
    ErrQuotaExceeded        = errors.New("quota exceeded")
    ErrUnauthorized         = errors.New("unauthorized")
    ErrInsufficientPermissions = errors.New("insufficient permissions")
    ErrConcurrentModification = errors.New("concurrent modification")
    ErrNoEligibleAgents     = errors.New("no eligible agents")
    ErrNoCapableAgent       = errors.New("no capable agent")
    ErrDelegationDenied     = errors.New("delegation denied")
)

// ValidationError represents a validation failure
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// ErrRateLimitExceeded provides rate limit details
type ErrRateLimitExceeded struct {
    Key        string
    Limit      int
    Window     time.Duration
    RetryAfter time.Duration
}

func (e ErrRateLimitExceeded) Error() string {
    return fmt.Sprintf("rate limit exceeded: %s (limit: %d per %s, retry after: %s)",
        e.Key, e.Limit, e.Window, e.RetryAfter)
}

// ErrQuotaExceeded provides quota details
type ErrQuotaExceeded struct {
    TenantID uuid.UUID
    Resource string
    Limit    int64
    Current  int64
}

func (e ErrQuotaExceeded) Error() string {
    return fmt.Sprintf("quota exceeded: %s for tenant %s (limit: %d, current: %d)",
        e.Resource, e.TenantID, e.Limit, e.Current)
}

// ErrDelegationDenied provides delegation denial details
type ErrDelegationDenied struct {
    Reason string
}

func (e ErrDelegationDenied) Error() string {
    return fmt.Sprintf("delegation denied: %s", e.Reason)
}

// ErrUnauthorized provides authorization failure details
type ErrUnauthorized struct {
    Action string
    Reason string
}

func (e ErrUnauthorized) Error() string {
    return fmt.Sprintf("unauthorized: %s - %s", e.Action, e.Reason)
}
```

## Service Testing

```go
// File: pkg/services/task_service_test.go
package services_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/golang/mock/gomock"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/S-Corkum/devops-mcp/pkg/models"
    "github.com/S-Corkum/devops-mcp/pkg/services"
    "github.com/S-Corkum/devops-mcp/test/mocks"
)

func TestTaskService_CreateDistributedTask(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    // Setup mocks
    mockRepo := mocks.NewMockTaskRepository(ctrl)
    mockAgentService := mocks.NewMockAgentService(ctrl)
    mockNotifier := mocks.NewMockNotificationService(ctrl)
    mockEventPublisher := mocks.NewMockEventPublisher(ctrl)
    mockRuleEngine := mocks.NewMockRuleEngine(ctrl)
    
    config := services.ServiceConfig{
        RuleEngine: mockRuleEngine,
        Logger:     testLogger,
        Metrics:    testMetrics,
    }
    
    service := services.NewTaskService(
        config,
        mockRepo,
        mockAgentService,
        mockNotifier,
    )
    
    ctx := context.WithValue(context.Background(), "tenant_id", testTenantID)
    ctx = context.WithValue(ctx, "agent_id", "coordinator-agent")
    
    t.Run("successful distributed task with saga", func(t *testing.T) {
        dt := &models.DistributedTask{
            Type:        "parallel_analysis",
            Title:       "Analyze codebase",
            Description: "Parallel code analysis across modules",
            Priority:    "high",
            Subtasks: []models.Subtask{
                {
                    ID:          "subtask-1",
                    AgentID:     "agent-1",
                    Description: "Analyze module A",
                    Parameters:  map[string]interface{}{"module": "A"},
                },
                {
                    ID:          "subtask-2",
                    AgentID:     "agent-2",
                    Description: "Analyze module B",
                    Parameters:  map[string]interface{}{"module": "B"},
                },
            },
            Aggregation: models.AggregationConfig{
                Method:     "combine_results",
                WaitForAll: true,
                Timeout:    3600,
            },
        }
        
        // Mock expectations
        gomock.InOrder(
            // Validate agents
            mockAgentService.EXPECT().
                GetAgent(gomock.Any(), "agent-1").
                Return(&models.Agent{ID: "agent-1", Status: "available"}, nil),
            
            mockAgentService.EXPECT().
                GetAgent(gomock.Any(), "agent-2").
                Return(&models.Agent{ID: "agent-2", Status: "available"}, nil),
            
            // Create main task
            mockRepo.EXPECT().
                Create(gomock.Any(), gomock.Any()).
                DoAndReturn(func(ctx context.Context, task *models.Task) error {
                    assert.Equal(t, dt.Type, task.Type)
                    assert.Equal(t, dt.Title, task.Title)
                    assert.True(t, task.Parameters["distributed"].(bool))
                    task.ID = uuid.New()
                    return nil
                }),
            
            // Create subtasks (parallel)
            mockRepo.EXPECT().
                Create(gomock.Any(), gomock.Any()).
                Times(2).
                Return(nil),
            
            // Update main task
            mockRepo.EXPECT().
                Update(gomock.Any(), gomock.Any()).
                Return(nil),
            
            // Publish events
            mockEventPublisher.EXPECT().
                Publish(gomock.Any(), gomock.Any()).
                Times(3). // Main task + 2 subtasks
                Return(nil),
            
            // Notifications
            mockNotifier.EXPECT().
                NotifyTaskAssigned(gomock.Any(), "agent-1", gomock.Any()).
                Return(nil),
            
            mockNotifier.EXPECT().
                NotifyTaskAssigned(gomock.Any(), "agent-2", gomock.Any()).
                Return(nil),
        )
        
        // Execute
        err := service.CreateDistributedTask(ctx, dt)
        
        // Assert
        require.NoError(t, err)
        assert.NotEqual(t, uuid.Nil, dt.ID)
        assert.Len(t, dt.SubtaskIDs, 2)
    })
    
    t.Run("rollback on subtask failure", func(t *testing.T) {
        dt := &models.DistributedTask{
            Type:  "test_task",
            Title: "Test",
            Subtasks: []models.Subtask{
                {ID: "subtask-1", AgentID: "agent-1"},
                {ID: "subtask-2", AgentID: "agent-2"},
            },
        }
        
        // Mock expectations
        gomock.InOrder(
            // Successful main task creation
            mockRepo.EXPECT().
                Create(gomock.Any(), gomock.Any()).
                Return(nil),
            
            // First subtask succeeds
            mockRepo.EXPECT().
                Create(gomock.Any(), gomock.Any()).
                Return(nil),
            
            // Second subtask fails
            mockRepo.EXPECT().
                Create(gomock.Any(), gomock.Any()).
                Return(errors.New("database error")),
            
            // Compensation: delete first subtask
            mockRepo.EXPECT().
                Delete(gomock.Any(), gomock.Any()).
                Return(nil),
            
            // Compensation: delete main task
            mockRepo.EXPECT().
                Delete(gomock.Any(), gomock.Any()).
                Return(nil),
        )
        
        // Execute
        err := service.CreateDistributedTask(ctx, dt)
        
        // Assert
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "database error")
    })
}

// Benchmark tests
func BenchmarkTaskService_Create(b *testing.B) {
    // Setup service with mocks
    service := setupBenchmarkService(b)
    ctx := context.Background()
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            task := &models.Task{
                Type:  "benchmark_task",
                Title: "Benchmark",
            }
            _ = service.Create(ctx, task, "")
        }
    })
}
```

## Production Readiness Checklist

### Core Features
- ✅ Domain-driven service design
- ✅ Distributed transaction support with saga pattern
- ✅ Event sourcing with CQRS
- ✅ Comprehensive rate limiting
- ✅ Quota management with alerts
- ✅ Circuit breakers for resilience
- ✅ Business rule engine
- ✅ Policy-based authorization
- ✅ Input sanitization
- ✅ Distributed state management with CRDT
- ✅ Conflict resolution strategies
- ✅ Real-time collaboration features
- ✅ Async processing with queues
- ✅ Batch operations support
- ✅ Result pagination
- ✅ Comprehensive caching
- ✅ Idempotency support

### Observability
- ✅ Distributed tracing
- ✅ Structured logging
- ✅ Business metrics
- ✅ Performance metrics
- ✅ Health checks
- ✅ Audit trails

### Security
- ✅ Field-level permissions
- ✅ Authorization caching
- ✅ Input validation
- ✅ SQL injection prevention
- ✅ Rate limiting per tenant/agent
- ✅ Quota enforcement

### Testing
- ✅ Unit tests with mocks
- ✅ Integration tests
- ✅ Saga compensation tests
- ✅ Load tests
- ✅ Chaos tests
- ✅ 90% coverage target

## Performance Targets

| Operation | Target | Notes |
|-----------|--------|-------|
| Task Creation | < 50ms | Including validation and assignment |
| Distributed Task | < 200ms | For 10 subtasks |
| State Update | < 20ms | With CRDT merge |
| Workflow Execution | < 100ms | Initiation only |
| Rule Evaluation | < 5ms | Per rule |
| Cache Hit | < 1ms | L1 cache |

## Next Steps

After completing Phase 3:
1. Deploy services to test environment
2. Configure rule engine with business policies
3. Set up rate limits and quotas
4. Run integration tests with real dependencies
5. Performance test with production-like load
6. Configure monitoring dashboards
7. Train operations team on saga compensation
8. Document business rule authoring