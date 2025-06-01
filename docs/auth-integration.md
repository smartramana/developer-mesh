# Authentication System Integration

## Overview

A centralized authentication system has been implemented in the DevOps MCP platform to provide consistent authentication and authorization across all services.

## Implementation Details

### New Package: `pkg/auth`

The new auth package provides:

1. **Service-based Architecture**
   - Central `auth.Service` that manages all authentication operations
   - Configuration-driven with sensible defaults
   - Support for both API keys and JWT tokens

2. **Authentication Types**
   - API Key authentication (header-based)
   - JWT token authentication
   - Configurable auth type selection

3. **Middleware Support**
   - Gin middleware: `authService.GinMiddleware()`
   - Standard HTTP middleware: `authService.StandardMiddleware()`
   - Scope-based authorization: `authService.RequireScopes()`

4. **Features**
   - In-memory API key storage (development)
   - Database storage support (production)
   - Cache integration for performance
   - Multi-tenancy support
   - Audit logging capabilities
   - Rate limiting protection

### Service Integration

#### REST API Service (`apps/rest-api`)

- Added auth service initialization in `server.go`
- Replaced legacy `AuthMiddleware` with centralized auth middleware
- Maintains backward compatibility during transition
- API keys are loaded from configuration

#### MCP Server (`apps/mcp-server`)

- Added auth service initialization in `server.go`
- Applied auth middleware to API v1 routes
- Uses same auth configuration as REST API

#### Worker Service (`apps/worker`)

- No changes required (no HTTP endpoints)

### Database Schema

A new migration has been added for API key storage:

```sql
CREATE TABLE api_keys (
    key VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    scopes TEXT[] DEFAULT '{}',
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used TIMESTAMP,
    active BOOLEAN NOT NULL DEFAULT true
);
```

### Configuration

Auth configuration is integrated with existing service configs:

```yaml
api:
  auth:
    jwt_secret: "your-secret-key"
    api_keys:
      admin-key: "admin"
      read-key: "read"
```

### Security Improvements

1. **Centralized Management**: Single source of truth for authentication logic
2. **Scope-based Access**: Fine-grained permission control
3. **Token Expiration**: Configurable expiration for both API keys and JWTs
4. **Audit Trail**: Last-used tracking for API keys
5. **Rate Limiting**: Protection against brute force attacks

## Migration Guide

### For Service Developers

1. Replace old middleware:
   ```go
   // Old
   v1.Use(AuthMiddleware("api_key"))
   
   // New
   v1.Use(authService.GinMiddleware(auth.TypeAPIKey))
   ```

2. Initialize auth service:
   ```go
   authConfig := auth.DefaultConfig()
   authConfig.JWTSecret = cfg.Auth.JWTSecret
   authService := auth.NewService(authConfig, db, cache, logger)
   ```

3. Load API keys:
   ```go
   authService.InitializeDefaultAPIKeys(apiKeyMap)
   ```

### For API Consumers

No changes required - the API continues to accept:
- API keys via `Authorization: Bearer <key>` header
- API keys via custom header (configurable)
- JWT tokens via `Authorization: Bearer <jwt>` header

## Testing

The auth package includes comprehensive tests:

```bash
# Run auth package tests
go test ./pkg/auth/...

# Run integration tests
go test ./test/integration/...
```

## Future Enhancements

1. **OAuth2 Support**: Add OAuth2 provider integration
2. **RBAC**: Role-based access control
3. **API Key Rotation**: Automatic key rotation policies
4. **Metrics**: Authentication metrics and monitoring
5. **Session Management**: Stateful session support
6. **MFA**: Multi-factor authentication

## File Structure

```
pkg/auth/
├── auth.go           # Core service implementation
├── middleware.go     # Gin and HTTP middleware
├── auth_test.go      # Unit tests
├── example_test.go   # Usage examples
└── README.md         # Package documentation

apps/rest-api/migrations/sql/
└── 006_create_api_keys_table.up.sql   # Database migration
```

## Benefits

1. **Consistency**: Same auth logic across all services
2. **Maintainability**: Single package to update
3. **Security**: Centralized security updates
4. **Flexibility**: Easy to add new auth methods
5. **Performance**: Built-in caching support
6. **Testability**: Comprehensive test coverage

## Backward Compatibility

The implementation maintains backward compatibility:
- Legacy middleware still works (marked for deprecation)
- Existing API keys continue to function
- No breaking changes to API contracts

## Conclusion

The centralized authentication system provides a solid foundation for secure, scalable authentication across the DevOps MCP platform. It simplifies authentication management while providing flexibility for future enhancements.