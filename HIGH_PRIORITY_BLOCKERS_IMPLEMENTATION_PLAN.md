# High Priority Blockers Implementation Plan (27 TODOs)

## Overview

These 27 high-priority TODOs are important but not blocking core functionality. They can be implemented in parallel with or after the critical blockers.

## Phase 1: Service Layer Enhancements (Week 4)

### Session 1: Assignment Engine Completion (6 TODOs)
**File**: `pkg/services/assignment_engine.go`

**Implementation request for Claude**:
```
"Complete the assignment engine implementation in pkg/services/assignment_engine.go:

1. getEligibleAgents method:
   - Call agentService.GetAvailableAgents(ctx)
   - Filter by agent status (Active only)
   - Filter by capabilities matching task requirements
   - Apply any rule-based filtering
   - Return filtered list

2. determineStrategy method:
   - Check task priority (High/Urgent -> performance_based)
   - Check task type (cost_sensitive -> cost_optimized)
   - Check rules from rule engine if available
   - Default to capability_match

3. LeastLoadedStrategy.Assign implementation:
   - For each agent, call agentService.GetAgentWorkload(ctx, agentID)
   - Calculate load score: activeTasks * 0.7 + queuedTasks * 0.3
   - Select agent with lowest score
   - Handle tie-breaking with random selection

4. CostOptimizedStrategy.Assign implementation:
   - Get agent cost rates from agent metadata
   - Factor in estimated task duration
   - Calculate cost = rate * estimated_hours
   - Select lowest cost agent with required capabilities

5. CapabilityMatchStrategy.extractCapabilities:
   - Parse task.Type for required skills
   - Extract from task.Parameters["required_capabilities"]
   - Return deduplicated list

6. PerformanceBasedStrategy.calculatePerformance:
   - Get agent performance metrics from agentService
   - Calculate score: successRate * 0.5 + (1/avgCompletionTime) * 0.3 + availability * 0.2
   - Return normalized score (0-1)

Use these patterns:
- Error handling: wrap all external calls
- Metrics: track assignment decisions
- Logging: log strategy selection reasons
- Caching: cache agent workload for 30 seconds

NO TODOs, complete implementation only."
```

### Session 2: Task Helpers & Progress Tracking (3 TODOs)
**File**: `pkg/services/task_helpers.go`

**Implementation request for Claude**:
```
"Complete the helper implementations in pkg/services/task_helpers.go:

1. ProgressTracker.checkProgress implementation:
   - For each tracked task:
     * Get task from service (with caching)
     * Check if status changed -> untrack if terminal
     * Check if overdue -> send notification
     * Check if stuck (no update in 15 min) -> log warning
     * Update metrics for long-running tasks

2. ResultAggregator.majorityVote implementation:
   - Group results by value (using deep equality)
   - Count occurrences of each unique result
   - Return result with highest count
   - Handle ties by returning first occurrence
   - Include vote counts in metadata

3. TaskCreationSaga.PublishEvents implementation:
   - Check if eventBus is available in service config
   - For each event in saga:
     * Set correlation ID
     * Set timestamp
     * Publish to appropriate topic
   - Handle publish failures gracefully
   - Log all published events

Include error handling and metrics for each operation."
```

### Session 3: Base Service Transaction Support (1 TODO)
**File**: `pkg/services/base_service.go`

**Implementation request for Claude**:
```
"Implement distributed transaction support in pkg/services/base_service.go:

Implement StartDistributedTransaction method:
- Generate transaction ID (UUID)
- Create transaction context with ID
- Register with transaction coordinator if available
- Set transaction timeout (default 30s)
- Add transaction metadata to context
- Return wrapped context

Pattern to follow:
func (s *BaseService) StartDistributedTransaction(ctx context.Context, opts ...TxOption) (context.Context, func(), func()) {
    txID := uuid.New()
    ctx = context.WithValue(ctx, "tx_id", txID)
    
    // Apply options
    config := defaultTxConfig
    for _, opt := range opts {
        opt(&config)
    }
    
    // Create timeout context
    ctx, cancel := context.WithTimeout(ctx, config.Timeout)
    
    // Commit function
    commit := func() {
        // Commit logic
        s.logger.Info("Transaction committed", map[string]interface{}{"tx_id": txID})
    }
    
    // Rollback function  
    rollback := func() {
        // Rollback logic
        s.logger.Error("Transaction rolled back", map[string]interface{}{"tx_id": txID})
        cancel()
    }
    
    return ctx, commit, rollback
}

Include transaction options for timeout and isolation level."
```

## Phase 2: API & WebSocket Layer (Week 4-5)

### Session 4: WebSocket Authentication (2 TODOs)
**File**: `apps/mcp-server/internal/api/websocket/auth.go`

**Implementation request for Claude**:
```
"Enable and complete JWT authentication in apps/mcp-server/internal/api/websocket/auth.go:

1. Uncomment and implement JWT validation:
   - Parse JWT from Authorization header or query param
   - Validate signature with configured secret/key
   - Check expiration and standard claims
   - Extract user ID and tenant ID from claims
   - Store in connection context

2. Uncomment and implement enhanced auth features:
   - API key validation as alternative to JWT
   - Rate limiting by user/tenant
   - IP whitelist checking
   - Role-based connection limits
   - Audit logging for auth events

Use the existing auth patterns from the codebase.
Integrate with ProductionAuthorizer for permission checks.

Example JWT validation:
token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method")
    }
    return []byte(secret), nil
})

if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
    userID := claims["user_id"].(string)
    tenantID := claims["tenant_id"].(string)
    // Store in context
}"
```

### Session 5: WebSocket Handlers & Monitoring (3 TODOs)
**Files**: `apps/mcp-server/internal/api/websocket/handlers.go`, `monitoring.go`, `pool.go`

**Implementation request for Claude**:
```
"Complete WebSocket infrastructure in these files:

1. handlers.go - Uncomment message handlers:
   - Implement each handler method following existing patterns
   - Add request validation
   - Add authorization checks
   - Include error responses
   - Add metrics for each message type

2. monitoring.go - Implement server uptime tracking:
   - Track server start time
   - Calculate uptime duration
   - Expose uptime metrics
   - Add health check endpoint data

3. pool.go - Implement connection pooling:
   - Maintain pool of reusable connections
   - Implement connection lifecycle (acquire/release)
   - Add pool metrics (size, available, in-use)
   - Handle connection health checks
   - Implement pool sizing limits

Follow WebSocket patterns from the existing code."
```

### Session 6: REST API Embedding Service (3 TODOs)
**File**: `apps/rest-api/internal/api/embedding_api.go`

**Implementation request for Claude**:
```
"Create proper service integrations in apps/rest-api/internal/api/embedding_api.go:

1. Create search service integration:
   - Extract search interface from ServiceV2
   - Create adapter to use vector search
   - Implement query building
   - Handle result transformation

2. Create embedding service integration:
   - Use ServiceV2's embedding components
   - Implement batch processing
   - Add caching for embeddings
   - Handle rate limiting

3. Implement metrics integration:
   - Access MetricsRepository from ServiceV2
   - Expose embedding metrics
   - Track search performance
   - Add custom business metrics

Use dependency injection patterns from the codebase."
```

## Phase 3: Infrastructure & Tools (Week 5)

### Session 7: Main Server Configuration (3 TODOs)
**File**: `apps/mcp-server/cmd/server/main.go`

**Implementation request for Claude**:
```
"Complete server initialization in apps/mcp-server/cmd/server/main.go:

1. Implement RuleEngine initialization:
   - Create rules.NewEngine(config)
   - Load rules from configuration/database
   - Set up rule hot-reloading
   - Integrate with services

2. Implement PolicyManager initialization:
   - Create policy manager instance
   - Load policies from configuration
   - Set up policy caching
   - Integrate with authorization

3. Implement migration logic:
   - Use golang-migrate/migrate library
   - Run migrations from migrations/ directory
   - Handle migration versioning
   - Add rollback support
   - Log migration status

Example pattern:
if !*skipMigration {
    m, err := migrate.New(
        "file://migrations",
        databaseURL,
    )
    if err != nil {
        return errors.Wrap(err, "failed to create migrator")
    }
    
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return errors.Wrap(err, "failed to run migrations")
    }
    
    logger.Info("Migrations completed successfully")
}"
```

### Session 8: Embedding Manager & Tools (2 TODOs)
**Files**: `apps/mcp-server/internal/core/embedding_manager.go`, `apps/mcp-server/internal/api/server.go`

**Implementation request for Claude**:
```
"Complete infrastructure components:

1. embedding_manager.go - Implement repository processing:
   - Clone/pull repository using go-git
   - Walk file tree with filepath.Walk
   - Extract code files by extension
   - Use chunking service to split code
   - Generate embeddings in batches of 100
   - Store in vector database (pgvector)
   - Track processing progress
   - Handle incremental updates

Example pattern:
func (em *EmbeddingManager) ProcessRepository(ctx context.Context, repoURL string) error {
    // Clone or update repo
    repo, err := git.PlainClone(localPath, false, &git.CloneOptions{
        URL: repoURL,
    })
    
    // Walk files
    err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
        if shouldProcess(path) {
            content, _ := os.ReadFile(path)
            chunks := em.chunker.Chunk(content)
            embeddings := em.embedder.GenerateBatch(ctx, chunks)
            em.store.SaveEmbeddings(ctx, path, embeddings)
        }
        return nil
    })
}

2. server.go - Set tool registry and event bus:
   - Initialize tool registry: tools.NewRegistry()
   - Register GitHub, Jira, Slack tools
   - Initialize event bus: events.NewBus()
   - Set up event handlers for tool events
   - Wire to WebSocket connection manager

Use existing patterns for tool registration."
```

### Session 9: Worker SQS Integration (5 TODOs)
**File**: `apps/worker/cmd/sqs-test.go`

**Implementation request for Claude**:
```
"Fix context.TODO usage in apps/worker/cmd/sqs-test.go:

Replace all context.TODO() with proper context management:

1. Main context setup:
   ctx := context.Background()
   
2. Add timeout contexts for each operation:
   - GetQueueUrl: 
     ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
     defer cancel()
     
   - SendMessage: 10 second timeout
   - ReceiveMessage: 30 second timeout (long polling)
   - DeleteMessage: 5 second timeout

3. Add retry logic with exponential backoff:
   var result *sqs.SendMessageOutput
   err := retry.Do(
       func() error {
           ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
           defer cancel()
           
           var err error
           result, err = client.SendMessage(ctx, input)
           return err
       },
       retry.Attempts(3),
       retry.Delay(time.Second),
       retry.DelayType(retry.BackOffDelay),
   )

4. Add proper error handling:
   - Check for AWS specific errors
   - Log errors with context
   - Return wrapped errors

5. Add graceful shutdown:
   sigChan := make(chan os.Signal, 1)
   signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
   
   go func() {
       <-sigChan
       cancel() // Cancel all operations
   }()

NO context.TODO() should remain."
```

## Phase 4: Search & Language Parsers (Week 5-6)

### Session 10: Search & Embedding Services (5 TODOs)
**Files**: Various service files

**Implementation request for Claude**:
```
"Implement search and embedding enhancements:

1. pkg/repository/search/repository.go - Vector search options:
   type SearchOptions struct {
       SimilarityThreshold float32  // Min similarity score (0-1)
       MetadataFilters     map[string]interface{}
       HybridSearch        bool     // Combine text + vector
       RankingAlgorithm    string   // "cosine", "euclidean", "dot_product"
       MaxResults          int
   }
   
   Implement options in Search method:
   - Apply similarity threshold in WHERE clause
   - Add metadata filtering with JSONB queries
   - Combine with full-text search if hybrid
   - Use specified distance function

2. pkg/embedding/service_v2.go - Batch processing:
   func (s *Service) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
       const batchSize = 100
       
       // Process in batches
       var results [][]float32
       for i := 0; i < len(texts); i += batchSize {
           end := min(i+batchSize, len(texts))
           batch := texts[i:end]
           
           // Generate embeddings with retry
           embeddings, err := s.generateWithRetry(ctx, batch)
           if err != nil {
               return nil, err
           }
           
           results = append(results, embeddings...)
           
           // Progress callback
           if s.progressFunc != nil {
               s.progressFunc(float64(end) / float64(len(texts)))
           }
       }
       
       return results, nil
   }

3. Other services - Add proper context propagation and error handling."
```

### Session 11: Language Parser Enhancements (6 TODOs)
**Files**: `pkg/chunking/parsers/*.go`

**Low Priority - Can be deferred**

These parser enhancements are nice-to-have and can be implemented later:
- Python decorator support
- Java field extraction  
- JavaScript object literal methods
- Kotlin/Rust/HCL improvements

## Implementation Priority

### Week 4: Service Layer & API
1. Assignment Engine (Critical for task distribution)
2. WebSocket Auth (Security requirement)
3. Base Service Transactions (Data consistency)

### Week 5: Infrastructure
1. Main Server Configuration (System startup)
2. SQS Worker Fixes (Message processing)
3. Embedding Manager (Search functionality)

### Week 6: Enhancements (Optional)
1. Search improvements
2. Parser enhancements
3. Monitoring additions

## Success Metrics for High Priority Items

- [ ] Assignment engine fully functional
- [ ] WebSocket authentication enabled
- [ ] All context.TODO replaced with proper contexts
- [ ] Rule engine and policy manager initialized
- [ ] Database migrations automated
- [ ] No ignored errors in high-traffic paths
- [ ] All services properly initialized (no nil)
- [ ] >85% test coverage maintained
- [ ] make lint continues to show 0 errors

## Testing Requirements

Each implementation must include:
1. Unit tests with mocks
2. Integration tests where applicable
3. Error scenario coverage
4. Performance benchmarks for critical paths
5. Race condition tests for concurrent code

## Production Validation

After implementing high priority items:
```bash
# Run full validation suite
make pre-commit
make test-aws-services
make integration-test

# Check for any remaining TODOs
grep -r "TODO" pkg/ apps/ --include="*.go" | wc -l
# Should show significant reduction

# Verify no nil services
grep -r "= nil" apps/*/cmd/ --include="*.go"
# Should return nothing except error assignments
```

## AWS Service Integration

All implementations must use REAL AWS services:
- **ElastiCache**: Connection via SSH tunnel (127.0.0.1:6379)
- **S3**: IP-restricted bucket (sean-mcp-dev-contexts)
- **SQS**: Production queue URL
- **Bedrock**: With cost limits ($0.10 per session)

## Key Implementation Patterns

### Error Handling
```go
if err != nil {
    logger.Error("Operation failed", map[string]interface{}{
        "error": err.Error(),
        "context": additionalInfo,
    })
    metrics.IncrementError("operation_name", err)
    return errors.Wrap(err, "descriptive message")
}
```

### Metrics Pattern
```go
timer := prometheus.NewTimer(metrics.operationDuration.WithLabelValues("operation"))
defer timer.ObserveDuration()

// Operation code

metrics.IncrementCounter("operation.success", 1.0)
```

### Context Pattern
```go
ctx, span := tracer.Start(ctx, "Service.Method")
defer span.End()

// Add timeout
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

This plan provides detailed implementation instructions for all high-priority blockers, optimized for Claude Code with production requirements.