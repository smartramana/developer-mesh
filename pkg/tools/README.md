# Tools Package

## Overview

The `pkg/tools` package provides comprehensive functionality for dynamic tool management, OpenAPI integration, and intelligent operation resolution in Developer Mesh.

## Key Components

### Operation Resolver (`operation_resolver.go`)

Provides intelligent mapping between simple action names and OpenAPI operation IDs, solving the impedance mismatch between how AI agents call actions and how OpenAPI specifications define them.

**Features:**
- Maps simple verbs (`get`, `list`, `create`) to full operation IDs (`repos/get`, `issues/list`)
- Context-aware resolution using provided parameters
- Supports multiple naming conventions (slash/hyphen/underscore)
- Automatic disambiguation when multiple operations match
- Fuzzy matching for format variations

**Resolution Strategies (applied in order):**
1. **Direct Lookup** - Exact match for operation ID
2. **Contextual Resolution** - Uses parameters to infer resource type
3. **Simple Name Mapping** - Extracts and matches action verbs
4. **Fuzzy Matching** - Handles format variations
5. **Disambiguation Scoring** - Selects best match using parameter context

**Usage Example:**
```go
resolver := NewOperationResolver(logger)
resolver.BuildOperationMappings(openAPISpec, "github")

// Resolves "get" to "repos/get" when repo parameters are present
operation, err := resolver.ResolveOperation("get", map[string]interface{}{
    "owner": "org",
    "repo": "myrepo",
})
```

### Dynamic Tool Adapter (`adapters/dynamic_tool_adapter.go`)

Handles execution of dynamically discovered tools with OpenAPI specifications.

**Features:**
- Automatic OpenAPI spec fetching and caching
- Integrated operation resolution
- Support for multiple authentication methods
- Passthrough authentication for user tokens
- Request building from OpenAPI operations

### Discovery Service (`adapters/discovery_service.go`)

Automatically discovers API specifications from tool endpoints.

**Strategies:**
- Direct OpenAPI URL access
- Common path discovery (`/openapi.json`, `/swagger.json`)
- Subdomain exploration (`api.`, `docs.`)
- HTML parsing for API documentation links
- Well-known paths checking

### Health Check System (`health_check.go`, `health_scheduler.go`)

Monitors tool availability and performance.

**Features:**
- Periodic health checks with configurable intervals
- Response time tracking
- Circuit breaker integration
- Caching of health status

### Schema Generator (`schema_generator.go`)

Generates minimal JSON schemas for MCP tool definitions from OpenAPI specifications.

**Features:**
- Groups related operations to reduce schema size
- Extracts common parameters
- Creates simplified input schemas for AI agents

### Authentication (`dynamic_auth.go`, `passthrough_authenticator.go`)

Handles various authentication methods for dynamic tools.

**Supported Methods:**
- Bearer tokens
- API keys (header or query)
- Basic authentication
- Custom headers
- User token passthrough

## Integration with MCP

The tools package is integrated with the MCP (Model Context Protocol) server to enable:

1. **Dynamic tool discovery** - Tools are discovered and registered automatically
2. **Intelligent execution** - Simple action names from MCP are resolved to OpenAPI operations
3. **Parameter mapping** - MCP parameters are mapped to OpenAPI request structures
4. **Error handling** - Detailed error messages for debugging

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./pkg/tools/...

# Run operation resolver tests
go test -v ./pkg/tools -run TestOperationResolver

# Run with coverage
go test -cover ./pkg/tools/...
```

## Common Use Cases

### Adding a New Tool

```go
// Discover and register a tool
discoveryService := NewDiscoveryService(logger)
spec, err := discoveryService.DiscoverAPI(ctx, "https://api.github.com")

// Create adapter with operation resolution
adapter := NewDynamicToolAdapter(tool, specCache, encryptionSvc, logger)

// Execute action with intelligent resolution
result, err := adapter.ExecuteAction(ctx, "get", params)
```

### Resolving Ambiguous Actions

When multiple operations could match an action:

```go
// Parameters help disambiguate
params := map[string]interface{}{
    "issue_number": 123,  // Indicates issue operation
    "owner": "org",
    "repo": "myrepo",
}

// Resolver uses context to select "issues/get" over "repos/get"
operation, err := resolver.ResolveOperation("get", params)
```

## Troubleshooting

### "Operation not found" Errors

1. Check if the OpenAPI spec includes operation IDs
2. Verify parameters match the expected operation
3. Try using the full operation ID
4. Check logs for available operations

### Resolution Not Working

1. Ensure `BuildOperationMappings` was called with the spec
2. Verify the operation resolver is initialized
3. Check that parameters are being passed to resolution
4. Review disambiguation scoring in logs

## Dependencies

- `github.com/getkin/kin-openapi/openapi3` - OpenAPI 3.0 parsing
- `github.com/getkin/kin-openapi/routers` - OpenAPI routing
- Internal packages: `models`, `observability`, `repository`, `security`

## Future Enhancements

- [ ] Machine learning for operation matching patterns
- [ ] Custom resolution rules per tool provider
- [ ] Caching of resolution results
- [ ] Metrics for resolution accuracy
- [ ] Support for GraphQL specifications