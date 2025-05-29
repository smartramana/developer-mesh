# Production Test Plan for DevOps MCP Platform

This is the definitive test plan to ensure the DevOps MCP platform is production-ready with zero tolerance for faults. The platform consists of REST APIs with full CRUD operations, MCP servers, and Go workers.

## Executive Summary

- **Total Test Duration**: ~6-8 hours for full validation
- **Quick Validation**: ~30 minutes for critical path testing
- **Target Metrics**: Zero critical bugs, <0.1% error rate, p99 latency <100ms for reads, <200ms for writes

## Test Phases

### Phase 1: Code Quality & Security (45 minutes)

#### 1.1 Unit Tests with Coverage
```bash
# Run all unit tests with coverage
make test-coverage

# Verify coverage > 80% for critical paths
# Check coverage report
make test-coverage-html
```

**Pass Criteria**: 
- All unit tests pass
- Code coverage > 80% for business logic
- No race conditions detected

#### 1.2 Security Scanning
```bash
# Static Application Security Testing (SAST)
gosec -fmt=json -out=security-report.json ./...

# Dependency vulnerability scanning
go list -json -m all | nancy sleuth

# Container vulnerability scanning
trivy image devops-mcp-mcp-server:latest
trivy image devops-mcp-rest-api:latest
trivy image devops-mcp-worker:latest
```

**Pass Criteria**:
- Zero high/critical vulnerabilities
- All medium vulnerabilities have mitigation plans
- No hardcoded secrets detected

#### 1.3 Code Quality
```bash
# Linting
make lint

# API contract validation
go test ./apps/rest-api/internal/api -tags=contract -v
```

### Phase 2: Integration Testing (1 hour)

#### 2.1 Database Integration
```bash
# Run integration tests
make test-integration

# Test database operations
go test ./pkg/database -run TestTransactions -v
go test ./pkg/database -run TestACIDCompliance -v
```

#### 2.2 Service Integration
```bash
# GitHub integration
make test-github

# Event system integration
go test ./pkg/events -tags=integration -v

# Cache integration
go test ./pkg/cache -tags=integration -v
```

**Pass Criteria**:
- All integrations functional
- Transaction rollback working
- Event delivery guaranteed

### Phase 3: API Testing (1.5 hours)

#### 3.1 Authentication & Authorization
```bash
# Test all auth scenarios
./scripts/test-auth-scenarios.sh

# Test multi-tenant isolation
./scripts/test-tenant-isolation-writes.sh
```

#### 3.2 CRUD Operations
```bash
# Test write validation
./scripts/test-write-validation.sh

# Test idempotency
./scripts/test-idempotency.sh

# Full CRUD cycle tests
go test ./apps/rest-api/internal/api -run TestCRUD -v
```

#### 3.3 Data Integrity
```bash
# Concurrent write tests
go test ./apps/rest-api/internal/api -run TestConcurrentWrites -race

# Foreign key constraints
go test ./pkg/database -run TestForeignKeyConstraints -v

# Cross-service consistency
go test ./apps/rest-api/internal/api -run TestConsistency -v
```

**Pass Criteria**:
- Zero authentication bypasses
- Complete multi-tenant isolation
- All CRUD operations validated
- Idempotent operations working
- No data corruption under concurrent load

### Phase 4: Performance Testing (2 hours)

#### 4.1 Response Time Validation
```bash
# Validate SLA compliance
./scripts/validate-response-times.sh
```

**SLA Targets**:
- Health checks: p99 < 50ms
- Read operations: p99 < 100ms
- Write operations: p99 < 200ms
- Delete operations: p99 < 100ms

#### 4.2 Load Testing
```bash
# Mixed read/write load test
k6 run scripts/k6-read-write-load-test.js

# Expected results:
# - Support 100 concurrent users
# - Error rate < 0.1%
# - No memory leaks
# - Stable performance
```

#### 4.3 Stress Testing
```bash
# Database connection pool limits
go test ./pkg/database -run BenchmarkConnectionPool -bench=.

# Queue throughput
go test ./apps/worker -run BenchmarkMessageProcessing -bench=.
```

### Phase 5: MCP Server Validation (1 hour)

#### 5.1 Protocol Compliance
```bash
go test ./apps/mcp-server/internal/api -run TestProtocolCompliance -v
go test ./apps/mcp-server/internal/api -run TestMessageFormats -v
```

#### 5.2 Context Management
```bash
go test ./apps/mcp-server/internal/core -run TestContextLifecycle -v
go test ./apps/mcp-server/internal/core -run TestTokenManagement -v
go test ./apps/mcp-server/internal/core -run TestContextTruncation -v
```

#### 5.3 Multi-Tenant Operations
```bash
go test ./apps/mcp-server/internal/core -run TestTenantIsolation -v
go test ./apps/mcp-server/internal/core -run TestResourceQuotas -v
```

**Pass Criteria**:
- Protocol fully compliant
- Context management efficient
- Token limits enforced
- Complete tenant isolation

### Phase 6: Worker Testing (45 minutes)

#### 6.1 Message Processing
```bash
go test ./apps/worker/internal/worker -run TestMessageOrdering -v
go test ./apps/worker/internal/worker -run TestPoisonMessageHandling -v
go test ./apps/worker/internal/worker -run TestRetryLogic -v
```

#### 6.2 Reliability
```bash
go test ./apps/worker/internal/worker -run TestIdempotency -v
go test ./apps/worker/internal/worker -run TestJobRecovery -v
go test ./apps/worker/internal/worker -run TestGracefulShutdown -v
```

**Pass Criteria**:
- Message ordering preserved
- Poison messages handled
- Retry logic working
- Jobs recoverable after restart

### Phase 7: End-to-End Testing (1 hour)

#### 7.1 Functional Testing
```bash
# Run complete functional test suite
make test-functional

# API workflow testing
./scripts/test-api-workflows.sh
```

#### 7.2 Webhook Testing
```bash
./scripts/test-github-webhook.sh
./scripts/verify_github_integration_complete.sh
```

### Phase 8: Stability Testing (4+ hours)

#### 8.1 Soak Test
```bash
# 4-hour stability test
k6 run scripts/k6-soak-test.js

# Monitor for:
# - Memory leaks
# - Performance degradation
# - Error rate increases
```

**Pass Criteria**:
- Memory usage stable
- No performance degradation
- Error rate remains < 0.1%

## Critical Success Metrics

### Performance
| Operation | Target p99 Latency | Max Error Rate |
|-----------|-------------------|----------------|
| Health Check | < 50ms | 0% |
| Read Operations | < 100ms | 0.1% |
| Create Operations | < 200ms | 0.1% |
| Update Operations | < 150ms | 0.1% |
| Delete Operations | < 100ms | 0.1% |
| Bulk Operations | < 5s (1000 records) | 0.5% |

### Reliability
- **Uptime**: 99.9% availability
- **Data Integrity**: Zero data loss or corruption
- **Recovery Time**: < 30 seconds for service restart
- **Concurrent Users**: Support 100+ concurrent users

### Security
- **Authentication**: Required on all endpoints
- **Authorization**: Complete multi-tenant isolation
- **Input Validation**: Block all malicious inputs
- **Audit Trail**: Complete for all write operations

## Quick Validation Checklist (30 minutes)

For rapid validation before deployments:

```bash
# 1. Unit tests (5 min)
make test

# 2. Security scan (5 min)
gosec ./... | grep -E "Severity: (HIGH|CRITICAL)"

# 3. Integration tests (10 min)
make test-integration

# 4. API validation (5 min)
./scripts/validate-endpoints.sh
./scripts/health-check.sh

# 5. Performance check (5 min)
./scripts/validate-response-times.sh
```

## Production Readiness Checklist

### Must Pass All:

- [ ] **Testing**
  - [ ] Unit test coverage > 80%
  - [ ] All integration tests passing
  - [ ] Zero critical security vulnerabilities
  - [ ] Performance SLAs met

- [ ] **API Operations**
  - [ ] All CRUD operations validated
  - [ ] Input validation comprehensive
  - [ ] Error responses consistent
  - [ ] Rate limiting active

- [ ] **Data Integrity**
  - [ ] ACID transactions working
  - [ ] Foreign key constraints enforced
  - [ ] No cross-tenant data leaks
  - [ ] Audit logging complete

- [ ] **Reliability**
  - [ ] Graceful degradation implemented
  - [ ] Circuit breakers configured
  - [ ] Retry logic tested
  - [ ] No memory leaks in 4-hour test

- [ ] **Operational**
  - [ ] Health checks accurate
  - [ ] Metrics exposed for monitoring
  - [ ] Logs structured and useful
  - [ ] Database backup tested

## Red Flags - Block Production If:

1. Any failing unit or integration test
2. Security vulnerabilities rated High or Critical
3. API response times consistently > SLA
4. Memory leaks detected in soak test
5. Cross-tenant data access possible
6. Missing authentication on any endpoint
7. Data corruption under concurrent load
8. Error rate > 0.1% under normal load

## Test Execution Commands

```bash
# Full test suite (6-8 hours)
make test && \
make test-integration && \
./scripts/test-auth-scenarios.sh && \
./scripts/test-write-validation.sh && \
./scripts/test-tenant-isolation-writes.sh && \
k6 run scripts/k6-read-write-load-test.js && \
make test-functional && \
k6 run scripts/k6-soak-test.js

# Quick validation (30 minutes)
make test && \
gosec ./... && \
./scripts/validate-endpoints.sh && \
./scripts/validate-response-times.sh
```

## Post-Production Monitoring

After deployment, monitor these metrics:
- API response times (p50, p95, p99)
- Error rates by endpoint
- Database connection pool usage
- Memory and CPU utilization
- Queue depth and processing time
- Cache hit rates

## Future Enhancements

When chaos engineering tools become available:
- Database failure simulation
- Network partition testing
- Service dependency failures
- Resource exhaustion scenarios
- Cascading failure prevention

This comprehensive test plan ensures the DevOps MCP platform is production-ready with industry-leading reliability and performance.