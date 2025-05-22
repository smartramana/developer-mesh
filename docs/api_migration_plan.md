# API Migration Plan

## Goal
Relocate all application-specific code from `pkg/api` to `apps/mcp-server/internal/api` while maintaining clean architectural boundaries between library and application-specific code.

## Files to Migrate

### Core Server Components
- `pkg/api/server.go` → `apps/mcp-server/internal/api/server.go`
- `pkg/api/server_vector.go` → `apps/mcp-server/internal/api/server_vector.go`
- `pkg/api/config.go` → `apps/mcp-server/internal/api/config.go`
- `pkg/api/docs.go` → `apps/mcp-server/internal/api/docs.go`
- `pkg/api/errors.go` → `apps/mcp-server/internal/api/errors.go`

### API Endpoints
- `pkg/api/agent_api.go` → `apps/mcp-server/internal/api/handlers/agent_api.go`
- `pkg/api/model_api.go` → `apps/mcp-server/internal/api/handlers/model_api.go`
- `pkg/api/tool_api.go` → `apps/mcp-server/internal/api/handlers/tool_api.go`
- `pkg/api/vector_api.go` → `apps/mcp-server/internal/api/proxies/vector_api.go`
- `pkg/api/vector_handlers.go` → `apps/mcp-server/internal/api/handlers/vector_handlers.go`
- `pkg/api/vector_adapter.go` → `apps/mcp-server/internal/api/proxies/vector_adapter.go`
- `pkg/api/search_handlers.go` → `apps/mcp-server/internal/api/handlers/search_handlers.go`
- `pkg/api/search_routes.go` → `apps/mcp-server/internal/api/handlers/search_routes.go`
- `pkg/api/mcp_api.go` → `apps/mcp-server/internal/api/handlers/mcp_api.go`

### Middleware & Utility Components
- `pkg/api/middleware.go` → `apps/mcp-server/internal/api/middleware.go`
- `pkg/api/tracing_middleware.go` → `apps/mcp-server/internal/api/tracing_middleware.go`
- `pkg/api/versioning.go` → `apps/mcp-server/internal/api/versioning.go`

### Webhook Handling
- `pkg/api/webhook_server.go` → `apps/mcp-server/internal/api/webhooks/webhook_server.go`
- `pkg/api/webhooks/webhooks.go` → `apps/mcp-server/internal/api/webhooks/webhooks.go`

### Subpackages
- `pkg/api/context/handlers.go` → `apps/mcp-server/internal/api/context/handlers.go`
- `pkg/api/handlers/relationship_handler.go` → `apps/mcp-server/internal/api/handlers/relationship_handler.go`
- `pkg/api/responses/json.go` → `apps/mcp-server/internal/api/responses/json.go`

## Implementation Approach

1. **Create Clean Interfaces**:
   - Define clear interfaces in the `pkg/` directories that application code will consume
   - Ensure these interfaces are domain-focused rather than application-specific

2. **Establish Package Conventions**:
   - `internal/api/handlers` - API endpoint handlers
   - `internal/api/proxies` - Adapters for external services
   - `internal/api/responses` - Response formatting utilities
   - `internal/api/webhooks` - Webhook handling code
   - `internal/api/context` - Context management for API
   - `internal/api/tools` - External tool integration endpoints

3. **Import Path Updates**:
   - Update all imports from `github.com/S-Corkum/devops-mcp/pkg/api/...` to `github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/api/...`
   - Replace direct imports of application code with interfaces from appropriate `pkg/` libraries

4. **Testing Strategy**:
   - Incrementally move files and verify compilation after each major component
   - Ensure all tests are updated with the new import paths
   - Run integration tests to verify end-to-end functionality

## Migration Order
1. First migrate utilities and shared components (responses, errors)
2. Then middleware and server configuration
3. Next migrate API handlers
4. Finally migrate the core server components
