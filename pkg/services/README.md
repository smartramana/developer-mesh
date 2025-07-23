# Services Package

> **Purpose**: Business logic layer for the Developer Mesh platform
> **Status**: MVP Implementation with Production Architecture  
> **Dependencies**: Base service framework, repositories, domain models

## Overview

The `services` package implements the business logic layer for the Developer Mesh platform. It provides a production-ready architecture with MVP implementations. All services inherit from a common base that provides enterprise-grade patterns. Some features use simplified in-memory implementations suitable for development/MVP.

## Architecture

```
services/
├── base_service.go              # Foundation for all services
├── interfaces.go                # Service interfaces
├── task_service.go             # Task management
├── workflow_service.go         # Workflow orchestration
├── workspace_service.go        # Workspace management
├── document_service.go         # Document handling
├── agent_service_impl.go       # Agent management
├── assignment_engine.go        # Task assignment logic
├── document_lock_service.go    # Distributed locking
├── notification_service_impl.go # Event notifications
└── service_helpers.go          # Utility implementations
```

## Base Service Foundation

All services inherit from `BaseService`, providing:

### Core Features

```go
type BaseService struct {
    // Dependencies
    logger      Logger
    tracer      Tracer
    metrics     MetricsClient
    eventBus    EventBus
    
    // Security
    authorizer  Authorizer
    encryptor   EncryptionService
    sanitizer   Sanitizer
    
    // Resilience
    rateLimiter RateLimiter
    quotaMgr    QuotaManager
    circuitBreaker CircuitBreaker
    
    // State
    stateStore  StateStore
    cache       Cache
}
```

### Built-in Capabilities

1. **Distributed Transactions** (Local implementation, distributed-ready interface)
```go
// Two-phase commit support
tx, err := service.BeginDistributedTransaction(ctx, &TxOptions{
    Timeout: 30 * time.Second,
    Isolation: ReadCommitted,
})
defer tx.Rollback()

// Execute across services
err = tx.Execute(func() error {
    // Transactional operations
    return nil
})

err = tx.Commit()
```

2. **Rate Limiting & Quotas** (In-memory implementation, Redis-ready interface)
```go
// Check rate limits (currently in-memory, not distributed)
if err := service.CheckRateLimit(ctx, userID, "api_calls", 100); err != nil {
    return ErrRateLimitExceeded
}

// Check quotas (currently with hardcoded limits)
if err := service.CheckQuota(ctx, workspaceID, "storage", 1024*1024); err != nil {
    return ErrQuotaExceeded
}
```

3. **Event Publishing**
```go
// Publish domain events
err := service.PublishEvent(ctx, &TaskAssignedEvent{
    TaskID:  task.ID,
    AgentID: agent.ID,
    Time:    time.Now(),
})
```

## Service Implementations

### Task Service

Manages task lifecycle with state machine validation:

```go
// Initialize service
taskService := NewTaskServiceImpl(
    repo,
    assignmentEngine,
    cache,
    eventBus,
    logger,
    tracer,
)

// Create task
task, err := taskService.CreateTask(ctx, &CreateTaskRequest{
    Type:     "code_review",
    Priority: PriorityHigh,
    Parameters: map[string]interface{}{
        "repository": "developer-mesh",
        "pr_number": 123,
    },
})

// Assign to agent
agent, err := taskService.AssignTask(ctx, task.ID)

// Update progress
err = taskService.UpdateTaskProgress(ctx, task.ID, &ProgressUpdate{
    PercentComplete: 50,
    Status: "Analyzing code",
})

// Complete task
err = taskService.CompleteTask(ctx, task.ID, &TaskResult{
    Success: true,
    Output: analysisResults,
})
```

**Key Features:**
- State machine enforcement
- Automatic assignment via assignment engine
- Progress tracking
- Retry management
- Background rebalancing
- Performance metrics

### Workflow Service

Orchestrates multi-step workflows with dependencies:

```go
// Create workflow
workflow, err := workflowService.CreateWorkflow(ctx, &WorkflowDefinition{
    Name: "deployment-pipeline",
    Type: WorkflowTypeDAG,
    Steps: []WorkflowStep{
        {
            ID:   "build",
            Type: "build_code",
            Dependencies: []string{},
        },
        {
            ID:   "test",
            Type: "run_tests",
            Dependencies: []string{"build"},
        },
        {
            ID:   "deploy",
            Type: "deploy_app",
            Dependencies: []string{"test"},
        },
    },
})

// Execute workflow
execution, err := workflowService.ExecuteWorkflow(ctx, workflow.ID, &ExecutionParams{
    Variables: map[string]interface{}{
        "environment": "staging",
        "version": "1.2.3",
    },
})

// Monitor execution
status, err := workflowService.GetExecutionStatus(ctx, execution.ID)

// Handle step completion
err = workflowService.CompleteStep(ctx, execution.ID, "build", &StepResult{
    Success: true,
    Outputs: map[string]interface{}{
        "artifact": "app-1.2.3.tar.gz",
    },
})
```

**Workflow Types:**
- Sequential: Steps run in order
- Parallel: Steps run simultaneously
- DAG: Directed acyclic graph
- Saga: Distributed transactions
- State Machine: State-based execution
- Event-Driven: Triggered by events

### Workspace Service

Manages collaborative workspaces:

```go
// Create workspace
workspace, err := workspaceService.CreateWorkspace(ctx, &CreateWorkspaceRequest{
    Name: "frontend-team",
    Description: "Frontend development workspace",
    Settings: &WorkspaceSettings{
        IsPublic: false,
        Features: []string{"code_review", "ci_cd"},
        Quotas: map[string]int64{
            "storage_gb": 100,
            "members": 50,
        },
    },
})

// Add members
err = workspaceService.AddMember(ctx, workspace.ID, &Member{
    UserID: userID,
    Role:   RoleAdmin,
})

// Update resource usage
err = workspaceService.UpdateResourceUsage(ctx, workspace.ID, &ResourceUpdate{
    StorageBytes: 1024 * 1024 * 50, // 50MB
    CPUSeconds: 3600,
})

// Check limits
withinLimits, err := workspaceService.CheckResourceLimits(ctx, workspace.ID)
```

**Features:**
- Member management with roles
- Resource quotas and tracking
- Distributed state synchronization
- Activity monitoring
- Access control

### Document Service

Handles collaborative document editing:

```go
// Create document
doc, err := documentService.CreateDocument(ctx, &CreateDocumentRequest{
    WorkspaceID: workspace.ID,
    Name: "architecture.md",
    Type: DocumentTypeMarkdown,
    Content: "# System Architecture\n\n...",
})

// Lock for editing
lock, err := documentService.LockDocument(ctx, doc.ID, userID, &LockOptions{
    Duration: 5 * time.Minute,
    AutoRefresh: true,
})
defer documentService.UnlockDocument(ctx, lock.ID)

// Update with conflict detection
updated, err := documentService.UpdateDocument(ctx, &UpdateDocumentRequest{
    ID: doc.ID,
    Content: updatedContent,
    Version: doc.Version, // Optimistic locking
})

// Handle conflicts
if err == ErrDocumentConflict {
    conflicts, err := documentService.GetConflicts(ctx, doc.ID)
    resolved, err := documentService.ResolveConflicts(ctx, doc.ID, resolution)
}
```

**Features:**
- Version control
- Distributed locking
- Conflict detection and resolution
- Section-level operations
- Change tracking

### Agent Service

Manages AI agent lifecycle:

```go
// Register agent
agent, err := agentService.RegisterAgent(ctx, &RegisterAgentRequest{
    Name: "code-analyzer-1",
    Type: "analyzer",
    Capabilities: []Capability{
        CapabilityCodeAnalysis,
        CapabilitySecurityScan,
    },
    Endpoint: "ws://agent1:8080",
})

// Update status
err = agentService.UpdateAgentStatus(ctx, agent.ID, AgentStatusActive)

// Get workload
workload, err := agentService.GetAgentWorkload(ctx, agent.ID)

// Find available agents
agents, err := agentService.GetAvailableAgents(ctx, &AgentFilter{
    Capabilities: []Capability{CapabilityCodeAnalysis},
    MaxWorkload: 10,
})
```

## Assignment Engine

Intelligent task routing with multiple strategies:

```go
// Initialize with strategy
engine := NewAssignmentEngine(
    agentRepo,
    StrategyCapabilityMatch, // or RoundRobin, LeastLoaded, etc.
    logger,
)

// Assign task
agent, err := engine.AssignTask(ctx, &Task{
    Type: "security_scan",
    Requirements: []string{"security_scan", "code_analysis"},
    Priority: PriorityHigh,
})

// Custom rules
engine.AddRule(&AssignmentRule{
    Name: "high-priority-to-premium",
    Condition: func(task *Task) bool {
        return task.Priority == PriorityHigh
    },
    Filter: func(agents []*Agent) []*Agent {
        // Filter to premium agents
        return filterPremium(agents)
    },
})
```

**Built-in Strategies:**
- **Round Robin**: Even distribution
- **Least Loaded**: Based on workload
- **Capability Match**: Based on requirements
- **Random**: Random selection
- **Rule Based**: Custom rules
- **Cost Optimized**: Minimize cost
- **Performance**: Fastest agents

## Distributed Lock Service

Redis-based locking for distributed systems (requires Redis connection):

```go
// Initialize service
lockService := NewDocumentLockService(
    redisClient,
    logger,
    tracer,
)

// Acquire lock
lock, err := lockService.AcquireLock(ctx, &LockRequest{
    ResourceID: "doc-123",
    OwnerID: userID,
    Duration: 5 * time.Minute,
    Metadata: map[string]string{
        "operation": "edit",
    },
})

// Auto-refresh
ctx, cancel := context.WithCancel(ctx)
go lockService.AutoRefresh(ctx, lock.ID, 30*time.Second)
defer cancel()

// Check lock
isLocked, owner := lockService.IsLocked(ctx, "doc-123")

// Force unlock (admin)
err = lockService.ForceUnlock(ctx, "doc-123")
```

**Features:**
- Distributed locking with TTL (when Redis connected)
- Auto-refresh mechanism
- Lock timeout handling
- Metrics and monitoring
- Note: Requires Redis connection for distributed functionality

## Notification Service

Event-driven notifications:

```go
// Task notifications
err = notificationService.NotifyTaskAssigned(ctx, agentID, task)

// Broadcast to agents
err = notificationService.BroadcastToAgents(ctx, agentIDs, &Notification{
    Type: "system_update",
    Message: "Maintenance in 5 minutes",
    Priority: PriorityHigh,
})

// Workspace notifications
err = notificationService.NotifyWorkspaceEvent(ctx, workspaceID, &WorkspaceEvent{
    Type: "member_joined",
    Actor: userID,
    Timestamp: time.Now(),
})
```

## Error Handling

Comprehensive error types with context:

```go
// Service errors
var (
    ErrRateLimitExceeded = &ServiceError{
        Code: "RATE_LIMIT_EXCEEDED",
        Type: ErrorTypeRateLimit,
    }
    
    ErrQuotaExceeded = &ServiceError{
        Code: "QUOTA_EXCEEDED",
        Type: ErrorTypeQuota,
    }
    
    ErrUnauthorized = &ServiceError{
        Code: "UNAUTHORIZED",
        Type: ErrorTypeAuthorization,
    }
)

// Domain errors
var (
    ErrTaskNotFound = &TaskError{
        Code: "TASK_NOT_FOUND",
        Type: ErrorTypeNotFound,
    }
    
    ErrWorkflowInvalid = &WorkflowError{
        Code: "WORKFLOW_INVALID",
        Type: ErrorTypeValidation,
    }
)
```

## Resilience Patterns

### Circuit Breaker

```go
// Wrap external calls
result, err := service.WithCircuitBreaker("external-api", func() (interface{}, error) {
    return externalAPI.Call()
})
```

### Retry with Backoff

```go
// Retry failed operations
err := service.RetryWithBackoff(ctx, func() error {
    return unstableOperation()
}, RetryOptions{
    MaxAttempts: 3,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay: 5 * time.Second,
})
```

### Timeout Control

```go
// Set operation timeout
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

result, err := service.PerformOperation(ctx)
```

## Observability

### Metrics

```go
// Automatic metrics collection
- service_operations_total
- service_operation_duration_seconds
- service_errors_total
- service_rate_limit_hits_total
- service_quota_usage
- service_circuit_breaker_state
```

### Tracing

```go
// Automatic span creation
ctx, span := service.tracer.Start(ctx, "ServiceName.OperationName")
defer span.End()

// Add attributes
span.SetAttributes(
    attribute.String("task.id", taskID),
    attribute.Int("retry.count", retryCount),
)
```

### Logging

```go
// Structured logging
service.logger.Info("Task assigned",
    "task_id", task.ID,
    "agent_id", agent.ID,
    "duration", time.Since(start),
)
```

## Testing

### Unit Tests

```go
// Mock dependencies
func TestTaskService(t *testing.T) {
    mockRepo := mocks.NewMockRepository()
    mockCache := mocks.NewMockCache()
    
    service := NewTaskServiceImpl(
        mockRepo,
        mockEngine,
        mockCache,
        mockEventBus,
        logger,
        tracer,
    )
    
    // Test operations
    task, err := service.CreateTask(ctx, request)
    assert.NoError(t, err)
}
```

### Integration Tests

```go
// Test with real dependencies
func TestTaskServiceIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Setup test database
    db := setupTestDB(t)
    defer cleanupDB(db)
    
    // Test full workflow
}
```

## Best Practices

1. **Always use context**: Pass context for cancellation and tracing
2. **Handle errors explicitly**: Check all error returns
3. **Use transactions**: Wrap multi-step operations
4. **Set timeouts**: Prevent hanging operations
5. **Monitor metrics**: Track service health
6. **Test resilience**: Test failure scenarios
7. **Document APIs**: Keep interfaces well-documented

## Performance Considerations

- **Caching**: Use multi-level caching for read-heavy operations
- **Batch Operations**: Use bulk methods when available
- **Connection Pooling**: Reuse database connections
- **Async Processing**: Use background workers for long operations
- **Circuit Breakers**: Prevent cascade failures

## Implementation Notes

### MVP Implementations
The following features have simplified implementations suitable for development:

1. **Rate Limiting**: In-memory only, resets on restart
2. **Quota Management**: Hardcoded limits, not per-tenant
3. **Sanitization**: Basic pass-through, needs proper HTML/XSS protection
4. **Distributed Transactions**: Local coordinator, not truly distributed

### Production-Ready Features
The following are fully implemented:

1. **Encryption Service**: AES-GCM encryption
2. **Event Publishing**: Full event bus integration
3. **Service Architecture**: Clean interfaces and patterns
4. **Observability**: Logging, metrics, and tracing hooks

## Future Enhancements

### To Complete MVP
- [ ] Redis-based distributed rate limiting
- [ ] Per-tenant quota management
- [ ] HTML/XSS sanitization implementation
- [ ] Distributed transaction coordinator

### Post-MVP Features
- [ ] GraphQL API support
- [ ] Event sourcing for audit trail
- [ ] CQRS pattern implementation
- [ ] Saga orchestration improvements
- [ ] Advanced scheduling algorithms
- [ ] Machine learning for assignment

## References

- [Domain-Driven Design](https://martinfowler.com/tags/domain%20driven%20design.html)
- [Microservices Patterns](https://microservices.io/patterns/)
- [Distributed Systems](https://www.oreilly.com/library/view/designing-data-intensive-applications/9781449373320/)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)