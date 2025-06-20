# Critical Blockers Implementation Plan - Production-Ready with Claude Code Opus 4

## Executive Summary

This plan leverages Claude Code's Opus 4 capabilities to rapidly implement 91 critical TODOs following CLAUDE.md production requirements. Using Claude's ability to generate complete, production-ready implementations with REAL AWS services, we can reduce the timeline from 8-10 weeks to **2-3 weeks** while maintaining production quality.

## Production Requirements (from CLAUDE.md)

### Critical Rules
- **NO TODOs**: Every implementation must be complete
- **NO nil services**: All services must be properly initialized
- **NO ignored errors**: Full error handling required
- **REAL AWS only**: ElastiCache, S3, SQS, Bedrock - no LocalStack
- **Test coverage >85%**: Required for every implementation
- **make lint → 0 errors**: Must pass before any commit

## Claude Code Opus 4 Optimization Strategy

### Key Advantages to Leverage
1. **Parallel Implementation**: Claude can implement multiple methods simultaneously
2. **Pattern Recognition**: Claude excels at following established patterns
3. **Complete Implementations**: No TODO placeholders - full production code
4. **Automatic Test Generation**: Unit and integration tests alongside implementations
5. **Consistent Code Style**: Maintains codebase conventions automatically

### Implementation Approach
- **Batch Similar Methods**: Group similar repository methods for parallel implementation
- **Template-Based Generation**: Use existing patterns as templates
- **Multi-File Edits**: Implement related methods across files in single sessions
- **Test-Driven**: Generate tests first to validate implementations
- **Production Focus**: Every implementation uses real AWS services
- **Pre-commit validation**: Run `make pre-commit` after each session

## Phase Structure (2-3 Weeks Total)

### Phase 1: Infrastructure & Foundation (1-2 days)

#### Pre-Session Setup (REQUIRED)
```bash
# Start ElastiCache tunnel
./scripts/aws/connect-elasticache.sh  # Keep running

# Verify AWS connectivity
make test-aws-services

# Ensure clean state
make lint  # Must be 0 errors
grep -r "TODO" pkg/ apps/ --include="*.go"  # Must return nothing
```

#### Session 1: Circuit Breaker Complete Implementation
**Prompt Strategy**: Provide existing circuit breaker interface and resilience patterns from codebase

**Implementation request for Claude**:
```
"Implement the complete production-ready circuit breaker in pkg/resilience/circuit_breaker.go.

Here's the existing interface that must be implemented:
[paste CircuitBreaker interface]

Here's our metrics pattern from observability package:
[paste metrics example]

Specific requirements:
1. Execute method signature: func (cb *circuitBreaker) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error)
2. States: Closed (normal), Open (failing), HalfOpen (testing)
3. Configuration:
   - MaxRequests: 5 (in half-open state)
   - FailureThreshold: 0.6 (60% failure rate)
   - Timeout: 30 seconds (before trying half-open)
   - MinimumRequestCount: 10 (before evaluating failure rate)
4. Thread-safe using sync/atomic for counters
5. Metrics to track:
   - circuit_breaker_requests_total{state,status}
   - circuit_breaker_state_changes_total{from,to}
   - circuit_breaker_current_state{name}
6. Logging pattern: cb.logger.Info/Error with structured fields
7. Tests must include:
   - State transition tests
   - Concurrent access test with -race flag
   - Benchmark for Execute method
   - Table-driven tests for all scenarios

Example of our error handling pattern:
if err != nil {
    cb.metrics.incrementFailure()
    cb.logger.Error("Circuit breaker execution failed", map[string]interface{}{
        "error": err.Error(),
        "state": cb.getState(),
    })
    return nil, errors.Wrap(err, "circuit breaker execution failed")
}

NO TODOs, complete implementation only."
```

**Expected Output**:
- Complete `pkg/resilience/circuit_breaker.go` implementation
- Full test suite with race detection
- Metrics integration
- ~300-400 lines of production code

#### Session 2: Observability Context Management
**Parallel Implementation**:
- Context storage/retrieval
- Shutdown orchestration
- Tracing integration fixes

### Phase 2: Repository Layer Batch Implementation (3-4 days)

#### Session 3: Repository Base Infrastructure
**Create base repository pattern that all repositories will inherit**

**Claude prompt optimization**:
```
"Create a base repository implementation with:
1. Transaction support (look at task repository for pattern)
2. Caching layer integration
3. Prepared statement management
4. Common error handling
5. Metrics and tracing

This will be inherited by all repositories to reduce duplication."
```

#### Session 4-5: Workflow Repository - Complete Implementation
**Batch Implementation Strategy**: Group methods by similarity

**Pre-implementation requirements**:
```go
// Must use these production patterns:
// - Read/write DB separation (r.writeDB, r.readDB)
// - Redis caching with ElastiCache (r.cache)
// - Prepared statements for performance
// - Circuit breaker integration
// - Full metrics and tracing
```

**Group 1 - CRUD Operations** (Single Claude session):

**Detailed prompt for Claude**:
```
"Implement these workflow repository methods in pkg/repository/postgres/workflow_repository.go:

1. Create(ctx context.Context, workflow *models.Workflow) error
   - Generate UUID if not provided
   - Set created_at, updated_at timestamps
   - Initialize version to 1
   - Use named query with :field syntax
   - Handle unique constraint violations -> ErrAlreadyExists

2. Get(ctx context.Context, id uuid.UUID) (*models.Workflow, error)
   - Check cache first: key = fmt.Sprintf("workflow:%s", id)
   - Use readDB for query
   - Cache result for 5 minutes on success
   - Handle sql.ErrNoRows -> ErrNotFound

3. Update(ctx context.Context, workflow *models.Workflow) error
   - Increment version for optimistic locking
   - Update updated_at timestamp
   - Check affected rows (0 = not found or version mismatch)
   - Invalidate cache after success

4. Delete(ctx context.Context, id uuid.UUID) error
   - Check for active executions first
   - Return error if workflow has running executions
   - Hard delete (not soft delete for this method)

5. SoftDelete(ctx context.Context, id uuid.UUID) error
   - Set deleted_at timestamp
   - Keep record in database
   - Invalidate cache

6. GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Workflow, error)
   - Enforce tenant isolation in WHERE clause
   - Use prepared statement
   - Cache with key: fmt.Sprintf("workflow:tenant:%s:name:%s", tenantID, name)

Use these exact patterns from our task repository:
[paste example Create method]
[paste example caching pattern]
[paste example error handling]

SQL queries to use:
- createWorkflowQuery = `INSERT INTO workflows (...) VALUES (...)`
- getWorkflowQuery = `SELECT * FROM workflows WHERE id = $1 AND deleted_at IS NULL`
- updateWorkflowQuery = `UPDATE workflows SET ... WHERE id = :id AND version = :old_version`

Every method must have tracing, metrics, proper error handling, and caching."
```

**Group 2 - Query Operations** (Single Claude session):
- ListByTenant, ListByType, Search
- Pagination and filtering
- Cache integration

**Group 3 - Execution Management** (Single Claude session):
- CreateExecution, GetExecution, UpdateExecution
- GetActiveExecutions, GetExecutionsByWorkflow
- UpdateStepStatus with state validation

**Claude optimization tip**: Provide the task repository implementation as reference - Claude will follow the patterns exactly while adapting for workflows.

#### Session 6-7: Task Repository Methods - Batch Implementation

**Strategic grouping for parallel implementation**:

**Group 1 - List/Query Methods**:
```
"Implement these task repository methods in pkg/repository/postgres/task_repository_methods.go:

1. ListByAgent(ctx, agentID string, filters types.TaskFilters) (*interfaces.TaskPage, error)
   Query pattern:
   SELECT * FROM tasks 
   WHERE assigned_to = $1 
   AND ($2::text[] IS NULL OR status = ANY($2))
   AND deleted_at IS NULL
   ORDER BY created_at DESC
   LIMIT $3 OFFSET $4
   
   - Use readDB
   - Build dynamic WHERE clause from filters
   - Include pagination metadata
   - Cache key: fmt.Sprintf("tasks:agent:%s:page:%d", agentID, offset/limit)

2. ListByTenant(ctx, tenantID uuid.UUID, filters types.TaskFilters) (*interfaces.TaskPage, error)
   - Similar to ListByAgent but filter by tenant_id
   - Support filtering by status, priority, type
   - Return TaskPage with TotalCount, HasMore, NextCursor

3. GetSubtasks(ctx, parentTaskID uuid.UUID) ([]*models.Task, error)
   Query: SELECT * FROM tasks WHERE parent_task_id = $1 ORDER BY created_at
   - No pagination needed (subtasks are limited)
   - Cache with parent task

4. GetTaskTree(ctx, rootTaskID uuid.UUID, maxDepth int) (*models.TaskTree, error)
   Use recursive CTE:
   WITH RECURSIVE task_tree AS (
     SELECT *, 0 as depth FROM tasks WHERE id = $1
     UNION ALL
     SELECT t.*, tt.depth + 1 
     FROM tasks t
     JOIN task_tree tt ON t.parent_task_id = tt.id
     WHERE tt.depth < $2
   )
   SELECT * FROM task_tree ORDER BY depth, created_at
   
   - Build tree structure in memory
   - Return models.TaskTree{Root: *Task, Children: map[uuid.UUID][]*Task}

Follow these patterns exactly:
[paste pagination example]
[paste filter building example]
[paste CTE example from codebase]

All methods must handle errors properly, use caching, and include metrics."
```

**Group 2 - Bulk Operations**:
```
Implement all bulk operations using PostgreSQL best practices:
- BulkInsert (using COPY protocol)
- BulkUpdate
- BatchUpdateStatus
- ArchiveTasks
```

**Group 3 - Analytics & Search**:
```
Implement analytics and search methods:
- GetTaskStats (with aggregations)
- SearchTasks (full-text + vector)
- GetAgentWorkload
- GenerateTaskReport
```

### Phase 3: Workflow Service - Rapid Implementation (3-4 days)

#### Session 8-9: Core Workflow Operations

**Optimization**: Implement related methods together

**Batch 1 - Workflow Management**:
```
"Implement all workflow CRUD operations in the service layer:
- UpdateWorkflow (with version control)
- DeleteWorkflow (check running executions)
- ListWorkflows (with filtering)
- SearchWorkflows
- ValidateWorkflow

Follow the existing service patterns and include error handling, authorization, and metrics."
```

**Batch 2 - Template Operations**:
```
"Implement complete template management:
- CreateTemplate
- InstantiateTemplate  
- UpdateTemplate
- ValidateTemplate

Include parameter substitution and version management."
```

#### Session 10-11: Execution Engine

**Strategic Implementation Order**:

**Step 1 - Basic Execution Types**:
```
"Implement all basic step execution methods:
- executeSequentialStep
- executeScriptStep
- executeApprovalStep
- executeWebhookStep

Include state management, error handling, and retry logic. Follow our existing patterns."
```

**Step 2 - Advanced Flow Control**:
```
"Implement complex execution patterns:
- executeParallelStep (fork/join)
- executeConditionalStep (branching)
- handleBranching
- executeCompensation (rollback)

Ensure proper context propagation and error aggregation."
```

### Phase 4: Integration & Polish (2-3 days)

#### Session 12: Comprehensive Testing
**Claude's test generation capabilities**:
```
"Generate comprehensive integration tests for:
1. All repository methods
2. Workflow execution scenarios
3. Error conditions and edge cases
4. Concurrent operations

Use table-driven tests and our existing test patterns."
```

#### Session 13: Performance & Optimization
- Query optimization
- Index recommendations
- Cache strategy validation

## Claude Code Best Practices

### 1. Prompt Engineering for Opus 4

**Effective prompt structure**:
```
"Implement [specific methods] with these requirements:
1. [Specific requirement]
2. [Pattern to follow - reference existing code]
3. [Error handling approach]
4. [Testing requirements]

Here's an example from our codebase: [paste relevant code]

Ensure: No TODOs, complete error handling, proper metrics, follows our patterns exactly."
```

### 2. Batch Implementation Strategy

**Optimal batch sizes**:
- 5-8 similar methods per session
- Related functionality in single prompts
- Complete vertical slices (repo + service + tests)

### 3. Pattern Reinforcement

**Provide Claude with**:
- Existing implementation examples
- Interface definitions
- Error types and handling patterns
- Metric and logging conventions

### 4. Quality Assurance with Claude

**After each implementation session**:
```
"Review the implementation for:
1. Completeness (no TODOs)
2. Error handling coverage
3. Test coverage
4. Pattern consistency
5. Performance considerations

Fix any issues found."
```

## Accelerated Timeline

### Week 1
- **Day 1**: Infrastructure (Circuit Breaker, Observability)
- **Day 2-3**: Workflow Repository (all 22 methods)
- **Day 4-5**: Task Repository Methods (all 30 methods)

### Week 2  
- **Day 1-2**: Workflow Service Core (CRUD, Templates)
- **Day 3-4**: Execution Engine (all step types)
- **Day 5**: Integration testing

### Week 3
- **Day 1-2**: Performance optimization
- **Day 3**: Final testing and validation

## Success Metrics

### Implementation Quality Checks
- [ ] 0 TODOs in implementation
- [ ] All methods have error handling
- [ ] Consistent pattern usage
- [ ] Complete test coverage
- [ ] Metrics and tracing integrated

### Claude Session Metrics
- Average methods implemented per session: 5-8
- Code quality score: >95%
- Test coverage: >85%
- Pattern consistency: 100%

## Risk Mitigation

### Claude-Specific Risks
1. **Pattern Drift**
   - Mitigation: Always provide examples
   - Check: Review after each session

2. **Over-Engineering**
   - Mitigation: Specify "follow existing patterns exactly"
   - Check: Complexity metrics

3. **Test Coverage Gaps**
   - Mitigation: Request tests explicitly
   - Check: Run coverage after each session

## Recommended Implementation Order

### Priority 1: Unblock Everything
1. Circuit Breaker Execute method
2. Base repository infrastructure
3. Transaction support

### Priority 2: Core Functionality
1. Workflow repository CRUD
2. Task repository queries
3. Workflow service basics

### Priority 3: Advanced Features
1. Execution engine
2. Analytics methods
3. Bulk operations

## Claude Code Session Templates

### Template 1: Repository Method Implementation
```
"Implement the following PRODUCTION repository methods for [Repository]:

Methods to implement:
- [Method1] - [brief description]
- [Method2] - [brief description]
- [Method3] - [brief description]

Production Requirements (from CLAUDE.md):
1. Use read/write DB separation (readDB for queries, writeDB for mutations)
2. Redis caching via ElastiCache connection (use r.cache)
3. Prepared statements for all queries
4. Circuit breaker wrapping for resilience
5. Full error handling: sql.ErrNoRows → ErrNotFound, unique violations, etc.
6. Metrics: query duration, cache hit/miss, error counts
7. Distributed tracing with proper span management
8. Transaction support with isolation levels
9. Test coverage >85% with table-driven tests

Reference implementation: [paste similar method]

CRITICAL: NO TODOs, NO nil checks skipped, NO errors ignored.
Must pass: make lint && make test"
```

### Template 2: Service Layer Implementation (Workflow Service Example)
```
"Implement these workflow service methods in pkg/services/workflow_service_impl.go:

1. UpdateWorkflow(ctx context.Context, id uuid.UUID, updates *models.WorkflowUpdate) error
   Implementation requirements:
   - Authorization: s.authorizer.Authorize(ctx, "workflow", "update", id)
   - Get existing workflow first to validate state
   - Validate updates (no changing type if executions exist)
   - Use transaction for atomic update
   - Increment version
   - Audit log the change
   - Clear caches
   
2. executeSequentialStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (*models.StepResult, error)
   Flow:
   - Update execution current_step
   - Create task for step using taskService
   - Wait for task completion (with timeout)
   - Update step status
   - Store step result
   - Determine next step
   
   Error handling:
   - Task creation failure -> compensation
   - Timeout -> mark step failed, trigger retry policy
   - Task failure -> check retry policy

3. executeParallelStep(ctx context.Context, execution *models.WorkflowExecution, step *models.WorkflowStep) (*models.StepResult, error)
   Implementation:
   - Create tasks for all parallel branches
   - Use errgroup for concurrent execution
   - Wait for all or fail fast based on config
   - Aggregate results
   - Handle partial failures

Example patterns from our codebase:

// Authorization pattern
if err := s.authorizer.Authorize(ctx, "workflow", "execute", workflow.ID); err != nil {
    return nil, errors.Wrap(err, "unauthorized")
}

// Circuit breaker pattern
result, err := s.circuitBreaker.Execute(ctx, func() (interface{}, error) {
    return s.repo.GetWorkflow(ctx, id)
})

// Transaction pattern
err := s.repo.WithTransaction(ctx, func(tx types.Transaction) error {
    // Multiple operations
    return nil
})

// Metric pattern
timer := prometheus.NewTimer(s.metrics.stepDuration.WithLabelValues(step.Type))
defer timer.ObserveDuration()

Each method must have complete error handling, no TODOs."
```

### Template 3: Batch Test Generation
```
"Generate comprehensive tests for these implementations in [*_test.go file]:
[paste methods that were implemented]

Test requirements with specific examples:

1. Table-driven tests structure:
   tests := []struct {
       name    string
       setup   func() // Prepare mocks/data
       input   inputType
       want    wantType
       wantErr error
   }{
       {
           name: "successful case",
           setup: func() {
               mockRepo.EXPECT().Method().Return(result, nil)
           },
           input: validInput,
           want: expectedResult,
           wantErr: nil,
       },
       {
           name: "database error",
           setup: func() {
               mockRepo.EXPECT().Method().Return(nil, sql.ErrNoRows)
           },
           input: validInput,
           want: nil,
           wantErr: ErrNotFound,
       },
   }

2. Mock setup using gomock:
   ctrl := gomock.NewController(t)
   defer ctrl.Finish()
   mockRepo := mocks.NewMockRepository(ctrl)
   mockCache := mocks.NewMockCache(ctrl)

3. Concurrent test example:
   t.Run("concurrent operations", func(t *testing.T) {
       var wg sync.WaitGroup
       errors := make([]error, 10)
       
       for i := 0; i < 10; i++ {
           wg.Add(1)
           go func(idx int) {
               defer wg.Done()
               errors[idx] = service.Method(ctx, input)
           }(i)
       }
       
       wg.Wait()
       // Assert no errors
   })

4. Benchmark example:
   func BenchmarkMethod(b *testing.B) {
       // Setup
       b.ResetTimer()
       for i := 0; i < b.N; i++ {
           _, _ = service.Method(ctx, input)
       }
   }

5. Integration test with real database:
   func TestIntegration(t *testing.T) {
       if testing.Short() {
           t.Skip("skipping integration test")
       }
       // Use test database
       db := testutil.SetupTestDB(t)
       repo := postgres.NewRepository(db)
       // Test with real repository
   }

Ensure >85% coverage. Use 'go test -race' for all concurrent tests."
```

## Monitoring Progress

### Daily Checklist
- [ ] Methods implemented count
- [ ] Tests written and passing
- [ ] Integration verified
- [ ] No new TODOs introduced
- [ ] Patterns consistent

### Quality Gates (REQUIRED after each session)
```bash
# Must pass ALL of these:
make lint                                    # 0 errors required
make test                                    # All tests passing
make tc                                      # >85% coverage
grep -r "TODO" pkg/ apps/ --include="*.go"  # Must return NOTHING
grep -r "= nil" apps/*/cmd/                 # No nil services
make test-aws-services                      # AWS connectivity verified
```

### Production Validation Checklist
- [ ] All methods have complete error handling
- [ ] Circuit breakers protect external calls
- [ ] Caching uses real ElastiCache (not in-memory)
- [ ] Metrics exported to Prometheus format
- [ ] Tracing includes all operations
- [ ] No hardcoded values - use config
- [ ] Tenant isolation enforced
- [ ] Cost controls implemented (Bedrock limits)

---

This optimized plan leverages Claude Code's strengths to dramatically accelerate implementation while maintaining high quality. The key is batching similar work and providing clear patterns for Claude to follow.