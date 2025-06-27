# WebSocket Production Readiness Plan

**Last Updated: 2025-06-26**

## Executive Summary
This plan outlines the necessary changes to make the DevOps MCP WebSocket services fully production-ready. Currently, the system has a mix of in-memory operations and database-backed services, causing inconsistencies and test failures. This plan addresses all aspects needed for a robust, scalable production system.

## Current State Analysis (UPDATED)

### ✅ Completed Items
1. **Service Injection**: WebSocket server now properly receives multi-agent services via `SetMultiAgentServices`
2. **Database Persistence**: Task creation handlers (`handleTaskCreate`, `handleTaskCreateAutoAssign`) now use taskService for database persistence
3. **Environment Configuration**: Database credentials properly sourced from .env
4. **Authorization Fix**: Updated role bindings to match actual user IDs (agent-1, agent-2, etc.)

### 🚧 In Progress
1. **Authorization Testing**: Verifying that the authorization fix allows agents to create tasks
2. **Build and Restart**: Services need to be rebuilt and restarted to apply authorization changes

### Issues Identified
1. ~~**Data Consistency**: Mixed use of in-memory and database storage~~ **PARTIALLY FIXED** - Task operations now use database
2. **Transaction Management**: Database transaction errors in workflow operations
3. ~~**Service Integration**: Incomplete integration between handlers and services~~ **PARTIALLY FIXED** - Services properly injected
4. **Error Handling**: Circuit breaker triggering due to missing entities
5. **Test Coverage**: Tests failing due to architectural inconsistencies
6. ~~**Authorization**: Agents couldn't create tasks due to role binding mismatch~~ **FIXED**

### Working Components
- WebSocket connection management
- Agent registration and discovery
- Basic workspace operations
- Capability-based task matching
- Service dependency injection
- Database-backed task creation

### Failing Components
- Task delegation and acceptance (needs testing after auth fix)
- Workflow coordination
- Distributed task execution
- Some state persistence operations

## Production Readiness Requirements

### 1. Data Layer Consistency
- All operations MUST use PostgreSQL for persistence
- Remove ALL in-memory fallbacks
- Implement proper database schema with indexes
- Add database connection pooling
- Implement read replicas for scaling

### 2. Service Layer Completeness
- Fully implement all service interfaces
- Remove mock responses
- Add proper transaction management
- Implement optimistic locking
- Add comprehensive error handling

### 3. Performance & Scalability
- Implement caching layer (Redis)
- Add message queuing for async operations
- Implement connection pooling
- Add rate limiting
- Optimize database queries

### 4. Reliability & Resilience
- Configure circuit breakers properly
- Add retry mechanisms
- Implement graceful degradation
- Add health checks
- Implement backup strategies

### 5. Security
- Implement proper authentication
- Add authorization checks
- Implement rate limiting
- Add input validation
- Implement audit logging

### 6. Observability
- Add comprehensive logging
- Implement distributed tracing
- Add metrics collection
- Create dashboards
- Set up alerting

## Implementation Plan

### Phase 1: Remove In-Memory Operations (Week 1) - **PARTIALLY COMPLETE**

#### 1.1 Update Task Handlers ✅ **COMPLETED**
```go
// Current (REMOVED) ✅
if s.taskService != nil {
    // Use service
} else {
    // Mock response
}

// Target (IMPLEMENTED) ✅
if s.taskService == nil {
    return nil, ErrServiceNotInitialized
}
// Use service only
```

**Files Updated:**
- ✅ `apps/mcp-server/internal/api/websocket/collaboration_handlers.go`
  - Removed mock responses from task-related handlers
  - `handleTaskCreateAutoAssign` now persists to database
  - All task operations now use taskService
  - Added proper error handling and idempotency keys

- ✅ `apps/mcp-server/internal/api/websocket/handlers.go`
  - Updated `handleTaskCreate` to use taskService with database persistence
  - Updated `handleTaskStatus`, `handleTaskCancel`, `handleTaskList`
  - Added proper tenant ID parsing and validation
  - Removed dependency on in-memory taskManager

**Additional Fixes Completed:**
- ✅ Service injection mechanism via `SetMultiAgentServices` in server.go
- ✅ Authorization role bindings updated to match actual user IDs
- ✅ Environment variable loading fixed in startup scripts

**Still Needed:**
- ⏳ Remove remaining mock responses from workflow handlers
- ⏳ Remove remaining mock responses from document handlers
- ⏳ Complete removal of in-memory agent registry

#### 1.2 Update Agent Registry - **PENDING**
```go
// Implement database-backed agent registry
type AgentRegistry struct {
    db       *sql.DB
    cache    cache.Cache
    logger   observability.Logger
    metrics  observability.MetricsClient
}

// Add methods:
- PersistAgent(agent *models.Agent) error
- GetAgent(id string) (*models.Agent, error)
- UpdateAgentStatus(id string, status string) error
- GetAgentsByCapability(capability string) ([]*models.Agent, error)
```

**New Files Needed:**
- `pkg/repository/agent_repository.go`
- `pkg/services/agent_service_impl.go`

#### 1.3 Fix Database Schema - **PENDING**
```sql
-- Add missing indexes
CREATE INDEX idx_tasks_tenant_status ON tasks(tenant_id, status);
CREATE INDEX idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX idx_agents_capabilities ON agents USING GIN(capabilities);
CREATE INDEX idx_workflow_executions_status ON workflow_executions(workflow_id, status);

-- Add missing columns
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMP;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS delegated_from VARCHAR(255);
```

**Files:**
- Create migration: `000021_production_indexes.up.sql`

### Phase 2: Fix Transaction Management (Week 2)

#### 2.1 Implement Unit of Work Pattern
```go
type UnitOfWork interface {
    BeginTx(ctx context.Context) (*sql.Tx, error)
    Commit(tx *sql.Tx) error
    Rollback(tx *sql.Tx) error
}

type WorkflowService struct {
    uow UnitOfWork
    // ... other dependencies
}

func (s *WorkflowService) ExecuteWorkflow(ctx context.Context, workflowID string) error {
    tx, err := s.uow.BeginTx(ctx)
    if err != nil {
        return err
    }
    defer s.uow.Rollback(tx)
    
    // All operations in transaction
    if err := s.createExecution(ctx, tx, workflowID); err != nil {
        return err
    }
    
    return s.uow.Commit(tx)
}
```

**Files to Create:**
- `pkg/database/unit_of_work.go`
- `pkg/repository/transaction_manager.go`

#### 2.2 Fix Workflow Service
- Implement proper transaction boundaries
- Add savepoints for nested transactions
- Implement compensation logic for failures

**Files to Update:**
- `pkg/services/workflow_service_impl.go`
- `apps/mcp-server/internal/api/websocket/workflow_engine.go`

### Phase 3: Complete Service Integration (Week 2-3)

#### 3.1 Task Service Completion
```go
// Complete implementation of TaskService interface
type taskServiceImpl struct {
    repo      repository.TaskRepository
    cache     cache.Cache
    publisher events.Publisher
    metrics   observability.MetricsClient
}

// Implement ALL methods:
- Create with idempotency
- DelegateTask with history tracking
- AcceptTask with validation
- CompleteTask with result storage
```

**Implementation Checklist:**
- [ ] Idempotency keys for creation
- [ ] Delegation history tracking
- [ ] State machine validation
- [ ] Event publishing for all state changes
- [ ] Metrics for all operations
- [ ] Circuit breaker integration

#### 3.2 Workspace Service
- Implement persistent workspace storage
- Add member management with roles
- Implement workspace activity tracking
- Add workspace limits and quotas

**Database Schema:**
```sql
CREATE TABLE workspace_members (
    workspace_id UUID REFERENCES workspaces(id),
    agent_id VARCHAR(255),
    role VARCHAR(50),
    joined_at TIMESTAMP DEFAULT NOW(),
    last_active TIMESTAMP,
    PRIMARY KEY (workspace_id, agent_id)
);

CREATE TABLE workspace_activities (
    id UUID PRIMARY KEY,
    workspace_id UUID REFERENCES workspaces(id),
    agent_id VARCHAR(255),
    activity_type VARCHAR(100),
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### Phase 4: Performance Optimization (Week 3)

#### 4.1 Caching Strategy
```go
// Implement multi-level caching
type CacheManager struct {
    l1Cache *ristretto.Cache  // In-memory
    l2Cache *redis.Client     // Redis
}

// Cache keys
const (
    AgentCacheKey     = "agent:%s"
    TaskCacheKey      = "task:%s"
    WorkspaceCacheKey = "workspace:%s"
)

// Cache with TTL
func (c *CacheManager) SetWithTTL(key string, value interface{}, ttl time.Duration) error
func (c *CacheManager) InvalidatePattern(pattern string) error
```

#### 4.2 Database Optimization
- Add connection pooling configuration
- Implement prepared statements
- Add query result caching
- Implement batch operations

**Configuration:**
```yaml
database:
  max_open_conns: 100
  max_idle_conns: 10
  conn_max_lifetime: 1h
  prepared_stmt_cache_size: 100
```

#### 4.3 Message Batching
```go
// Batch WebSocket messages
type MessageBatcher struct {
    batchSize    int
    flushInterval time.Duration
    messages     []Message
    mu           sync.Mutex
}

func (b *MessageBatcher) Add(msg Message)
func (b *MessageBatcher) Flush() []Message
```

### Phase 5: Reliability & Monitoring (Week 4)

#### 5.1 Circuit Breaker Configuration
```go
// Configure circuit breakers per service
type CircuitBreakerConfig struct {
    MaxRequests       uint32
    Interval          time.Duration
    Timeout           time.Duration
    FailureThreshold  float64
    SuccessThreshold  uint32
}

var circuitBreakerConfigs = map[string]CircuitBreakerConfig{
    "task_service": {
        MaxRequests:      100,
        Interval:         10 * time.Second,
        Timeout:          30 * time.Second,
        FailureThreshold: 0.5,
        SuccessThreshold: 5,
    },
    // ... other services
}
```

#### 5.2 Health Checks
```go
// Comprehensive health checks
type HealthChecker struct {
    checks []HealthCheck
}

type HealthCheck interface {
    Name() string
    Check(ctx context.Context) error
}

// Implement checks for:
- Database connectivity
- Redis connectivity
- SQS queue depth
- Service dependencies
- WebSocket connection count
```

#### 5.3 Metrics & Monitoring
```go
// Key metrics to track
type WebSocketMetrics struct {
    ConnectionsTotal      prometheus.Counter
    ConnectionsActive     prometheus.Gauge
    MessagesSent          prometheus.Counter
    MessagesReceived      prometheus.Counter
    MessageLatency        prometheus.Histogram
    TasksCreated          prometheus.Counter
    TasksCompleted        prometheus.Counter
    TasksFailed           prometheus.Counter
    WorkflowsExecuted     prometheus.Counter
    WorkflowDuration      prometheus.Histogram
    DatabaseQueryDuration prometheus.Histogram
    CacheHitRate          prometheus.Gauge
}

// Add tracing
func (s *Server) handleWithTracing(ctx context.Context, method string, handler func() error) error {
    span, ctx := s.tracer.Start(ctx, method)
    defer span.End()
    
    start := time.Now()
    err := handler()
    
    s.metrics.RecordLatency(method, time.Since(start))
    if err != nil {
        span.RecordError(err)
        s.metrics.RecordError(method)
    }
    
    return err
}
```

### Phase 6: Testing & Validation (Week 4-5)

#### 6.1 Integration Tests
```go
// Test with real database
func TestTaskLifecycleIntegration(t *testing.T) {
    // Setup: Create database transaction
    tx := db.Begin()
    defer tx.Rollback()
    
    // Create task via API
    // Delegate task
    // Accept task
    // Complete task
    // Verify all state transitions
}
```

#### 6.2 Load Testing
```yaml
# K6 load test script
import ws from 'k6/ws';

export let options = {
  stages: [
    { duration: '2m', target: 100 },  // Ramp up
    { duration: '5m', target: 100 },  // Stay at 100
    { duration: '2m', target: 200 },  // Ramp to 200
    { duration: '5m', target: 200 },  // Stay at 200
    { duration: '2m', target: 0 },    // Ramp down
  ],
  thresholds: {
    ws_connecting: ['p(95)<500'],     // 95% connect within 500ms
    ws_msgs_sent: ['rate>1000'],      // Send >1000 msgs/sec
    ws_session_duration: ['p(95)<30000'], // 95% sessions < 30s
  },
};
```

#### 6.3 Chaos Testing
- Implement database failure scenarios
- Test network partitions
- Simulate high latency
- Test resource exhaustion

## Timeline & Milestones (UPDATED)

### Week 1: Foundation - **IN PROGRESS**
- [🔄] Remove all in-memory operations (50% complete)
  - ✅ Task handlers updated to use database
  - ✅ Service injection implemented
  - ⏳ Workflow handlers still need updates
  - ⏳ Document handlers still need updates
  - ⏳ Agent registry needs database backing
- [ ] Update database schema
- [✅] Fix compilation errors
- [🔄] Basic integration tests passing (pending authorization fix verification)

### Week 2: Core Services
- [🔄] Complete task service implementation (partially done)
- [ ] Fix transaction management
- [ ] Implement workflow coordination
- [ ] Integration tests passing

### Week 3: Performance
- [ ] Implement caching layer
- [ ] Add connection pooling
- [ ] Optimize database queries
- [ ] Load tests passing

### Week 4: Reliability
- [ ] Configure circuit breakers
- [ ] Add comprehensive monitoring
- [ ] Implement health checks
- [ ] Chaos tests passing

### Week 5: Final Validation
- [ ] Security audit
- [ ] Performance benchmarking
- [ ] Documentation update
- [ ] Production deployment plan

**Legend:**
- ✅ Complete
- 🔄 In Progress
- ⏳ Pending
- [ ] Not Started

## Success Criteria

1. **All Tests Pass**: 100% of functional tests passing
2. **Performance**: 
   - <100ms p95 latency for operations
   - Support 10,000 concurrent connections
   - <1% error rate under load
3. **Reliability**:
   - 99.9% uptime
   - Graceful degradation under failure
   - Automatic recovery from transient failures
4. **Observability**:
   - Full distributed tracing
   - Comprehensive metrics
   - Actionable alerts
5. **Security**:
   - Passed security audit
   - No critical vulnerabilities
   - Audit trail for all operations

## Risk Mitigation

1. **Database Migration Risk**: 
   - Use feature flags for gradual rollout
   - Maintain backward compatibility
   - Have rollback plan

2. **Performance Regression**:
   - Continuous benchmarking
   - Canary deployments
   - Load test before each release

3. **Integration Complexity**:
   - Incremental changes
   - Comprehensive testing
   - Clear service boundaries

## Conclusion

This plan transforms the WebSocket services from a development prototype to a production-ready system. The key is removing all shortcuts (in-memory operations, mock responses) and implementing proper database-backed services with full observability and reliability features.

The implementation should be done incrementally, with each phase building on the previous one. Regular testing and validation ensure that we don't introduce regressions while improving the system.