# OpenAPI Documentation vs Implementation Discrepancies Report

## Summary
This report compares the endpoints documented in `/docs/swagger/openapi.yaml` with the actual implementation found in the codebase.

## Major Discrepancies Found

### 1. Context Endpoints Path Mismatch
**Issue**: The OpenAPI documentation and implementation use different paths for context endpoints.

- **OpenAPI Documentation**: `/mcp/context`, `/mcp/contexts`
- **REST API Implementation**: `/api/v1/contexts`
- **MCP Server Implementation**: `/api/v1/mcp/context`, `/api/v1/mcp/contexts`

This is a significant issue as the documented paths don't match what's actually implemented in the REST API.

### 2. Missing Authentication Endpoints
**Issue**: The OpenAPI documentation references auth endpoints that don't appear to be implemented:

**Documented but not found in implementation**:
- POST `/api/v1/auth/token`
- POST `/api/v1/auth/refresh`
- POST `/api/v1/auth/revoke`
- POST `/api/v1/auth/validate`
- GET `/api/v1/auth/api-keys`
- POST `/api/v1/auth/api-keys`
- DELETE `/api/v1/auth/api-keys/{key_id}`

These endpoints are referenced in the OpenAPI spec but no implementation was found. The authentication appears to be handled entirely through middleware without dedicated endpoints.

### 3. Missing Swagger/Documentation Endpoints
**Issue**: The implementation includes documentation endpoints not in the OpenAPI spec:

**Implemented but not documented**:
- GET `/swagger/*any` (Swagger UI)
- GET `/redoc` (ReDoc documentation)

### 4. Missing Agent Endpoints
**Issue**: Some agent endpoints exist in implementation but aren't documented:

**Implemented but not documented**:
- GET `/api/v1/agents/{id}/workload`
- POST `/api/v1/agents/{id}/heartbeat`

### 5. Missing Embeddings Endpoint
**Issue**: Usage endpoint exists in implementation but isn't properly documented:

**Implemented but not documented**:
- GET `/api/v1/embeddings/usage`

### 6. Vector Endpoints Discrepancy
**Issue**: The implementation includes additional vector endpoints not in the OpenAPI spec:

**Implemented but might not be documented correctly**:
- POST `/api/v1/vectors/store`
- GET `/api/v1/vectors/context/{context_id}`
- DELETE `/api/v1/vectors/context/{context_id}`
- GET `/api/v1/vectors/models`
- GET `/api/v1/vectors/context/{context_id}/model/{model_id}`
- DELETE `/api/v1/vectors/context/{context_id}/model/{model_id}`

### 7. WebSocket Endpoint
**Issue**: MCP Server has a WebSocket endpoint not documented in OpenAPI:

**Implemented but not documented**:
- GET `/ws` (WebSocket connection)

### 8. MCP Server vs REST API Differences
The MCP Server and REST API have different endpoint sets:

**MCP Server specific**:
- Uses `/api/v1/mcp/context` paths
- Has WebSocket support at `/ws`
- Focuses on MCP protocol implementation

**REST API specific**:
- Uses `/api/v1/contexts` paths
- Has full CRUD operations for various resources
- Includes webhook management endpoints

## Recommendations

1. **Standardize Context Paths**: Choose either `/contexts` or `/mcp/context` and use consistently across both implementations and documentation.

2. **Implement or Remove Auth Endpoints**: Either implement the documented auth endpoints or remove them from the OpenAPI spec and document that authentication is handled via middleware.

3. **Document All Endpoints**: Add missing endpoints to the OpenAPI specification:
   - Swagger/ReDoc endpoints
   - Agent workload and heartbeat endpoints
   - WebSocket endpoint
   - Additional vector endpoints

4. **Separate OpenAPI Specs**: Consider maintaining separate OpenAPI specifications for:
   - REST API Server
   - MCP Server
   - Or clearly indicate which endpoints belong to which server

5. **Add Server-Specific Tags**: Use OpenAPI tags or extensions to indicate which server implements which endpoints.

6. **Validate Regularly**: Implement automated validation to ensure OpenAPI spec stays in sync with implementation.

## File References

- OpenAPI Specification: `/docs/swagger/openapi.yaml`
- REST API Implementation: `/apps/rest-api/internal/api/server.go`
- MCP Server Implementation: `/apps/mcp-server/internal/api/server.go`
- Context API Handler: `/apps/rest-api/internal/api/context/handlers.go`
- MCP API Handler: `/apps/rest-api/internal/api/mcp_api.go`