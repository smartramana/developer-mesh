# Session Management Implementation Plan

## Overview
This document provides a step-by-step implementation plan for the Edge MCP Session Management System with zero-error tolerance.

## Implementation Phases

### Phase 1: Database Layer (Day 1-2)

#### Task 1.1: Create and Deploy Migration
**Files to create:**
- `apps/rest-api/migrations/sql/000028_edge_mcp_sessions.up.sql`
- `apps/rest-api/migrations/sql/000028_edge_mcp_sessions.down.sql`

**Validation Steps:**
```bash
# Test migration locally
cd apps/rest-api
migrate -path migrations/sql -database "postgresql://devmesh:devmesh@localhost:5432/devmesh_development?sslmode=disable" up

# Verify tables created
psql -h localhost -U devmesh -d devmesh_development -c "\dt mcp.*session*"

# Test rollback
migrate -path migrations/sql -database "postgresql://devmesh:devmesh@localhost:5432/devmesh_development?sslmode=disable" down 1
```

#### Task 1.2: Create Model Definitions
**Files to create:**
- `pkg/models/session.go`
- `pkg/models/session_test.go`

**Validation Steps:**
```bash
# Compile check
cd pkg/models
go build ./...

# Run tests
go test -v -race ./...

# Check for proper JSON/DB tags
go vet ./...
```

### Phase 2: Repository Layer (Day 3-4)

#### Task 2.1: Implement Session Repository
**Files to create:**
- `pkg/repository/session_repository.go`
- `pkg/repository/session_repository_test.go`
- `pkg/repository/mocks/mock_session_repository.go`

**Implementation Checklist:**
- [ ] CreateSession with transaction support
- [ ] GetSession with cache check
- [ ] UpdateSession with optimistic locking
- [ ] TerminateSession with audit logging
- [ ] ListActiveSessions with pagination
- [ ] CleanupExpiredSessions with batch processing
- [ ] RecordToolExecution with metrics update
- [ ] GetSessionMetrics with aggregation

**Validation Steps:**
```bash
# Generate mocks
cd pkg/repository
mockgen -source=session_repository.go -destination=mocks/mock_session_repository.go

# Run unit tests
go test -v -cover ./...

# Run with race detector
go test -race ./...

# Benchmark critical operations
go test -bench=. -benchmem ./...
```

### Phase 3: Service Layer (Day 5-6)

#### Task 3.1: Implement Session Service
**Files to create:**
- `pkg/services/session_service.go`
- `pkg/services/session_service_impl.go`
- `pkg/services/session_service_test.go`

**Implementation Checklist:**
- [ ] Input validation for all methods
- [ ] Encryption service integration
- [ ] Cache management (set, get, invalidate)
- [ ] Tenant limit enforcement
- [ ] Metrics emission
- [ ] Error wrapping with context
- [ ] Concurrent access handling
- [ ] Transaction management

**Validation Steps:**
```bash
# Unit tests with mocked dependencies
go test -v -cover ./pkg/services/

# Integration tests with real Redis
INTEGRATION_TEST=true go test -v ./pkg/services/

# Load test session creation
go test -bench=BenchmarkCreateSession -benchtime=10s
```

### Phase 4: REST API Layer (Day 7-8)

#### Task 4.1: Implement API Handlers
**Files to create:**
- `apps/rest-api/internal/api/session_handler.go`
- `apps/rest-api/internal/api/session_handler_test.go`

**Implementation Checklist:**
- [ ] Request validation and sanitization
- [ ] Tenant isolation enforcement
- [ ] Rate limiting per endpoint
- [ ] Proper HTTP status codes
- [ ] Error response formatting
- [ ] Request/response logging
- [ ] OpenAPI documentation

**Files to modify:**
- `apps/rest-api/internal/api/server.go` (add routes)
- `apps/rest-api/cmd/api/main.go` (wire dependencies)

**Validation Steps:**
```bash
# Compile and run
cd apps/rest-api
go build ./cmd/api
./api

# Test endpoints
curl -X POST http://localhost:8081/api/v1/sessions \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"edge_mcp_id":"test-123","client_type":"claude-code"}'

# Run API tests
go test -v ./internal/api/

# Check for SQL injection
sqlmap -u "http://localhost:8081/api/v1/sessions/test" --headers="X-API-Key: $API_KEY"
```

### Phase 5: Integration & Enhancement (Day 9-10)

#### Task 5.1: Update Auth Package
**Files to modify:**
- `pkg/auth/session_provider.go` (new file)
- `pkg/auth/manager.go` (integrate session provider)
- `pkg/auth/README.md` (update documentation)

**Implementation Checklist:**
- [ ] Session token generation
- [ ] Session validation middleware
- [ ] Refresh token rotation
- [ ] Session revocation support

#### Task 5.2: Add Scheduled Jobs
**Files to create:**
- `apps/rest-api/internal/scheduler/session_cleanup.go`

**Implementation:**
```go
func StartSessionCleanup(service services.SessionService) {
    ticker := time.NewTicker(5 * time.Minute)
    go func() {
        for range ticker.C {
            ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
            if err := service.CleanupExpiredSessions(ctx); err != nil {
                logger.Error("Cleanup failed", err)
            }
            cancel()
        }
    }()
}
```

### Phase 6: Monitoring & Observability (Day 11)

#### Task 6.1: Add Metrics
**Files to modify:**
- `pkg/observability/metrics.go`

**Metrics to add:**
```go
var (
    SessionsCreated = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "sessions_created_total",
            Help: "Total number of sessions created",
        },
        []string{"tenant_id", "client_type"},
    )
    
    ActiveSessions = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "sessions_active",
            Help: "Number of active sessions",
        },
        []string{"tenant_id"},
    )
    
    SessionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "session_duration_seconds",
            Help: "Session duration in seconds",
            Buckets: prometheus.ExponentialBuckets(60, 2, 10),
        },
        []string{"tenant_id", "client_type"},
    )
)
```

#### Task 6.2: Add Dashboards
**Files to create:**
- `deployments/grafana/dashboards/session-management.json`

### Phase 7: Testing & Validation (Day 12-13)

#### Task 7.1: Integration Test Suite
**Files to create:**
- `test/integration/session_management_test.go`

**Test Scenarios:**
```go
func TestSessionLifecycle(t *testing.T) {
    // 1. Create session
    // 2. Update activity
    // 3. Record tool executions
    // 4. Get session (verify metrics)
    // 5. Terminate session
    // 6. Verify terminated state
}

func TestConcurrentSessions(t *testing.T) {
    // Create 100 sessions concurrently
    // Verify no race conditions
    // Check tenant limits enforced
}

func TestSessionExpiry(t *testing.T) {
    // Create session with 1s TTL
    // Wait for expiry
    // Run cleanup
    // Verify session expired
}
```

#### Task 7.2: Load Testing
**Files to create:**
- `test/load/session_load_test.go`

**Load Test Targets:**
- 1000 sessions/second creation rate
- 10000 concurrent active sessions
- 100000 tool executions/hour
- <100ms p99 latency for all operations

### Phase 8: Documentation (Day 14)

#### Task 8.1: API Documentation
**Files to update:**
- `docs/api/sessions.md`
- `api/openapi.yaml`

#### Task 8.2: Developer Guide
**Files to create:**
- `docs/guides/session-management-guide.md`

## Critical Path Dependencies

```mermaid
graph TD
    A[Database Migration] --> B[Model Definitions]
    B --> C[Repository Layer]
    C --> D[Service Layer]
    D --> E[API Handlers]
    E --> F[Integration Tests]
    
    B --> G[Mock Generation]
    G --> C
    
    D --> H[Cache Integration]
    D --> I[Encryption Integration]
    
    E --> J[Route Registration]
    J --> K[API Testing]
    
    F --> L[Load Testing]
    L --> M[Production Deployment]
```

## Validation Checkpoints

### Checkpoint 1: Database Layer (End of Day 2)
- [ ] Migration runs without errors
- [ ] Rollback works correctly
- [ ] Indexes created and optimized
- [ ] Foreign keys enforced
- [ ] Models compile without warnings

### Checkpoint 2: Repository Layer (End of Day 4)
- [ ] All CRUD operations tested
- [ ] No SQL injection vulnerabilities
- [ ] Proper error handling
- [ ] 100% test coverage
- [ ] No race conditions

### Checkpoint 3: Service Layer (End of Day 6)
- [ ] Business logic validated
- [ ] Encryption working
- [ ] Cache hit ratio > 80%
- [ ] Metrics emitted correctly
- [ ] Transaction rollback tested

### Checkpoint 4: API Layer (End of Day 8)
- [ ] All endpoints responding
- [ ] Authentication enforced
- [ ] Rate limiting working
- [ ] OpenAPI spec valid
- [ ] No security vulnerabilities

### Checkpoint 5: Integration (End of Day 10)
- [ ] Edge-mcp connects successfully
- [ ] Sessions created and tracked
- [ ] Tool executions recorded
- [ ] Cleanup job running
- [ ] Metrics visible in Grafana

## Risk Mitigation

### Risk 1: Database Migration Failure
**Mitigation:**
- Test on staging first
- Have rollback script ready
- Take database backup before migration
- Use transaction for entire migration

### Risk 2: Performance Degradation
**Mitigation:**
- Implement caching from start
- Use connection pooling
- Add indexes on frequently queried columns
- Batch operations where possible

### Risk 3: Security Vulnerabilities
**Mitigation:**
- Use parameterized queries only
- Encrypt sensitive data
- Validate all inputs
- Use prepared statements
- Regular security scans

### Risk 4: Memory Leaks
**Mitigation:**
- Proper context cancellation
- Close all resources in defer
- Use sync.Pool for frequently allocated objects
- Regular profiling with pprof

## Rollback Plan

### Database Rollback
```bash
# Rollback migration
migrate -path migrations/sql -database $DATABASE_URL down 1

# Restore from backup if needed
pg_restore -h localhost -U devmesh -d devmesh_development backup.dump
```

### Code Rollback
```bash
# Revert to previous version
git revert HEAD~5..HEAD
git push origin main

# Or use previous Docker image
docker-compose down
docker-compose up -d --scale rest-api=3 rest-api:previous-tag
```

## Success Criteria

### Functional Requirements
- ✅ Sessions created for all edge-mcp connections
- ✅ Tool executions tracked accurately
- ✅ Session expiry handled automatically
- ✅ Passthrough auth encrypted and working
- ✅ Metrics and monitoring operational

### Non-Functional Requirements
- ✅ <100ms p99 latency for session operations
- ✅ Support 10,000 concurrent sessions
- ✅ 99.99% uptime
- ✅ Zero data loss
- ✅ PCI compliance for auth data

### Quality Gates
- ✅ 90% code coverage
- ✅ No critical security vulnerabilities
- ✅ All integration tests passing
- ✅ Load tests meeting SLA
- ✅ Documentation complete

## Command Reference

### Development Commands
```bash
# Run tests
make test

# Run specific service tests
cd apps/rest-api && go test -v ./internal/api/

# Generate mocks
make generate-mocks

# Run linter
make lint

# Format code
make fmt

# Build all services
make build

# Run locally
docker-compose up -d
```

### Debugging Commands
```bash
# Check session in database
psql -h localhost -U devmesh -d devmesh_development \
  -c "SELECT * FROM mcp.edge_mcp_sessions WHERE session_id = 'xxx';"

# Monitor Redis cache
redis-cli monitor | grep session

# Check API logs
docker-compose logs -f rest-api | grep session

# Profile memory usage
go tool pprof http://localhost:8081/debug/pprof/heap

# Trace requests
curl -H "X-Trace-ID: test-123" http://localhost:8081/api/v1/sessions
```

### Production Commands
```bash
# Check session metrics
curl http://localhost:8081/metrics | grep session

# Manual cleanup
curl -X POST http://localhost:8081/admin/sessions/cleanup \
  -H "X-Admin-Key: $ADMIN_KEY"

# Export session data
psql -h $DB_HOST -U $DB_USER -d $DB_NAME \
  -c "\COPY (SELECT * FROM mcp.edge_mcp_sessions WHERE created_at > '2024-01-01') TO '/tmp/sessions.csv' CSV HEADER;"
```

## Team Responsibilities

### Backend Team
- Database migration
- Repository implementation
- Service layer
- API handlers

### DevOps Team
- Deployment pipeline
- Monitoring setup
- Load testing
- Production rollout

### Security Team
- Security review
- Penetration testing
- Compliance verification

### QA Team
- Integration testing
- Performance testing
- User acceptance testing

## Timeline Summary

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| Database Layer | 2 days | Migration deployed |
| Repository Layer | 2 days | Data access complete |
| Service Layer | 2 days | Business logic ready |
| API Layer | 2 days | Endpoints operational |
| Integration | 2 days | System connected |
| Monitoring | 1 day | Observability ready |
| Testing | 2 days | All tests passing |
| Documentation | 1 day | Docs complete |
| **Total** | **14 days** | **Production ready** |

## Conclusion

This implementation plan ensures:
- **Zero-error tolerance** through extensive validation
- **Systematic approach** with clear dependencies
- **Risk mitigation** at every phase
- **Comprehensive testing** before production
- **Full observability** from day one

Follow this plan step-by-step for successful implementation of the Edge MCP Session Management System.