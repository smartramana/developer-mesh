# Memory Matrix: Docker Build Failure Fixes

## Issue Analysis

### Root Cause
Docker builds are failing because type definitions exist but aren't being found during the build process. This indicates a Go workspace or module boundary issue.

### Failing Types Location Map
```
TracingHandler         -> apps/mcp-server/internal/api/websocket/tracing_handler.go
Tool                   -> apps/mcp-server/internal/api/websocket/types.go
ToolExecutionStatus    -> apps/mcp-server/internal/api/websocket/types.go
TruncatedContext      -> apps/mcp-server/internal/api/websocket/types.go
ContextStats          -> apps/mcp-server/internal/api/websocket/types.go
ConversationSessionManager -> MISSING (needs creation)
SubscriptionManager   -> MISSING (needs creation)
WorkflowEngine        -> MISSING (needs creation)
TaskManager           -> MISSING (needs creation)
NotificationManager   -> MISSING (needs creation)
```

### Failing Functions Location Map (rest-api)
```
NewSearchServiceAdapter     -> apps/rest-api/internal/api/embedding_adapters.go (EXISTS)
NewMetricsRepositoryAdapter -> apps/rest-api/internal/api/embedding_adapters.go (EXISTS)
CustomRecoveryMiddleware    -> apps/rest-api/internal/api/panic_recovery.go (EXISTS)
SetupPrometheusHandler      -> apps/rest-api/internal/api/metrics_handler.go (EXISTS)
```

## Fix Strategy

### Phase 1: Create Missing Type Definitions
1. Create `apps/mcp-server/internal/api/websocket/managers.go` with:
   - ConversationSessionManager
   - SubscriptionManager
   - WorkflowEngine
   - TaskManager
   - NotificationManager

### Phase 2: Fix Docker Build Context
1. Ensure go.work is properly set up in Dockerfile
2. Add explicit workspace sync in build process
3. Verify all internal packages are accessible

### Phase 3: Validate Import Paths
1. Check all import statements use full paths
2. Remove any relative imports
3. Ensure no circular dependencies

## Implementation Order
1. Create missing type definitions (stub implementations)
2. Update Dockerfiles to properly handle Go workspace
3. Test Docker builds locally
4. Commit and push fixes