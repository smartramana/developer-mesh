# Implementation Plan: Fix Missing Types and Functions

## Memory Matrix for Claude Code (Optimized for Opus 4)

### Context Anchors
- **Branch**: `fix/missing-types-and-functions`
- **Primary Files**: 
  - `apps/mcp-server/internal/api/websocket/agent_registry_interface.go`
  - `apps/mcp-server/internal/api/websocket/agent_collaboration.go` (NEW)
  - `apps/rest-api/internal/api/adapters.go` (NEW)
  - `apps/rest-api/internal/api/middleware.go` (NEW)
- **Key Patterns**: Follow existing codebase conventions, production-ready, no TODOs

### Implementation Order (Optimized for Parallel Execution)

#### Phase 1: MCP Server Types (Parallel Safe)
1. **File**: `apps/mcp-server/internal/api/websocket/agent_collaboration.go`
   - Define `DelegationResult` struct
   - Define `CollaborationSession` struct
   - Implement validation methods
   - Add comprehensive comments

#### Phase 2: REST API Adapters (Parallel Safe)
2. **File**: `apps/rest-api/internal/api/adapters.go`
   - Implement `NewSearchServiceAdapter()`
   - Implement `NewMetricsRepositoryAdapter()`
   - Follow adapter pattern from existing code

#### Phase 3: REST API Middleware (Parallel Safe)
3. **File**: `apps/rest-api/internal/api/middleware.go`
   - Implement `CustomRecoveryMiddleware()`
   - Implement `SetupPrometheusHandler()`
   - Use existing observability patterns

#### Phase 4: Dependency Updates (Sequential)
4. **Fix go.sum entries**
   - Run `go mod tidy` in each module
   - Ensure prometheus/client_golang is properly resolved

### Type Definitions

```go
// DelegationResult - Result of task delegation
type DelegationResult struct {
    ID          string
    FromAgentID string
    ToAgentID   string
    TaskID      string
    Status      string // "accepted", "rejected", "completed", "failed"
    Result      interface{}
    Error       string
    StartedAt   time.Time
    CompletedAt *time.Time
}

// CollaborationSession - Multi-agent collaboration session
type CollaborationSession struct {
    ID           string
    InitiatorID  string
    AgentIDs     []string
    Task         map[string]interface{}
    Strategy     string // "round-robin", "parallel", "hierarchical"
    Status       string // "active", "completed", "failed"
    CreatedAt    time.Time
    UpdatedAt    time.Time
    CompletedAt  *time.Time
    Results      map[string]interface{}
}
```

### Function Signatures

```go
// NewSearchServiceAdapter creates adapter for search service
func NewSearchServiceAdapter(searchService services.SearchService) *SearchServiceAdapter

// NewMetricsRepositoryAdapter creates adapter for metrics repository
func NewMetricsRepositoryAdapter(repo repository.MetricsRepository) *MetricsRepositoryAdapter

// CustomRecoveryMiddleware handles panic recovery with proper logging
func CustomRecoveryMiddleware(logger observability.Logger) gin.HandlerFunc

// SetupPrometheusHandler configures Prometheus metrics endpoint
func SetupPrometheusHandler(router *gin.Engine, path string)
```

### Error Handling Strategy
- All functions return proper errors
- Use `fmt.Errorf` with context
- Log errors at appropriate levels
- Include request IDs in error context

### Testing Requirements
- Unit tests for each new type/function
- Integration tests where applicable
- Minimum 85% coverage
- Table-driven tests

### Validation Rules
- DelegationResult: Valid agent IDs, non-empty task ID
- CollaborationSession: At least 2 agents, valid strategy
- All timestamps must be UTC
- All IDs must be valid UUIDs

## Execution Notes
- Each phase can be implemented in parallel
- Use existing patterns from codebase
- No placeholder implementations
- Full production-ready code only