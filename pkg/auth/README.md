# Auth Package

The `auth` package provides centralized authentication and authorization services for the DevOps MCP platform.

## Features

- **API Key Authentication**: Support for static and dynamic API keys
- **JWT Token Authentication**: JSON Web Token support with configurable expiration
- **Scope-based Authorization**: Fine-grained permission control
- **Middleware Support**: Ready-to-use middleware for Gin and standard HTTP handlers
- **Caching**: Built-in caching support for improved performance
- **Database Integration**: Optional database storage for API keys
- **Multi-tenancy**: Built-in tenant isolation support

## Usage

### Basic Setup

```go
import (
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    "github.com/S-Corkum/devops-mcp/pkg/common/cache"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Create auth configuration
config := auth.DefaultConfig()
config.JWTSecret = "your-secret-key"
config.EnableAPIKeys = true
config.EnableJWT = true

// Create auth service
logger := observability.NewLogger("auth")
authService := auth.NewService(config, db, cacheClient, logger)

// Initialize default API keys (optional)
apiKeys := map[string]string{
    "admin-key-123": "admin",
    "read-key-456":  "read",
}
authService.InitializeDefaultAPIKeys(apiKeys)
```

### Using with Gin

```go
// Create Gin router
router := gin.New()

// Apply auth middleware to all routes
router.Use(authService.GinMiddleware(auth.TypeAPIKey, auth.TypeJWT))

// Or apply to specific route groups
v1 := router.Group("/api/v1")
v1.Use(authService.GinMiddleware(auth.TypeAPIKey))

// Add scope-based authorization
adminRoutes := v1.Group("/admin")
adminRoutes.Use(authService.RequireScopes("admin"))
```

### Using with Standard HTTP

```go
// Create standard HTTP mux
mux := http.NewServeMux()

// Wrap handlers with auth middleware
authMiddleware := authService.StandardMiddleware(auth.TypeAPIKey)
mux.Handle("/api/", authMiddleware(apiHandler))
```

### API Key Management

```go
// Create a new API key
apiKey, err := authService.CreateAPIKey(
    ctx,
    "tenant-123",              // Tenant ID
    "user-456",                // User ID
    "Production API Key",      // Name
    []string{"read", "write"}, // Scopes
    nil,                       // No expiration
)

// Revoke an API key
err = authService.RevokeAPIKey(ctx, apiKey.Key)
```

### JWT Token Management

```go
// Generate a JWT token
user := &auth.User{
    ID:       "user-123",
    TenantID: "tenant-456",
    Email:    "user@example.com",
    Scopes:   []string{"read", "write"},
}
token, err := authService.GenerateJWT(ctx, user)

// Validate a JWT token
user, err = authService.ValidateJWT(ctx, token)
```

### Accessing User Information

```go
// In a Gin handler
func myHandler(c *gin.Context) {
    // Get authenticated user
    user, ok := auth.GetUserFromContext(c)
    if !ok {
        c.JSON(401, gin.H{"error": "unauthorized"})
        return
    }
    
    // Get tenant ID
    tenantID, ok := auth.GetTenantFromContext(c)
    if !ok {
        c.JSON(401, gin.H{"error": "missing tenant"})
        return
    }
    
    // Use user information
    fmt.Printf("User %s from tenant %s\n", user.ID, tenantID)
}
```

## Configuration

The auth package supports the following configuration options:

```go
type ServiceConfig struct {
    // JWT configuration
    JWTSecret      string        // Secret key for JWT signing
    JWTExpiration  time.Duration // Token expiration time
    
    // API key configuration
    APIKeyHeader   string // Custom header for API keys (default: X-API-Key)
    EnableAPIKeys  bool   // Enable API key authentication
    EnableJWT      bool   // Enable JWT authentication
    
    // Cache configuration
    CacheEnabled   bool          // Enable caching
    CacheTTL       time.Duration // Cache time-to-live
    
    // Security configuration
    MaxFailedAttempts int           // Max failed auth attempts
    LockoutDuration   time.Duration // Account lockout duration
}
```

## Scopes

The auth package uses a scope-based permission system:

- `read`: Read-only access to resources
- `write`: Create and update resources
- `admin`: Full administrative access

## Database Schema

If using database storage for API keys, create the following table:

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

## Testing

The package includes comprehensive tests. Run them with:

```bash
go test ./pkg/auth/...
```

## Migration Guide

If you're migrating from the old authentication system:

1. Replace `api.AuthMiddleware` with `authService.GinMiddleware`
2. Replace `api.InitAPIKeys` with `authService.InitializeDefaultAPIKeys`
3. Replace `api.InitJWT` with auth service configuration
4. Update type references from `AuthType*` to `auth.Type*`

## Security Considerations

- Store JWT secrets securely (use environment variables or secret management)
- Rotate API keys regularly
- Use HTTPS in production
- Enable rate limiting to prevent brute force attacks
- Monitor failed authentication attempts
- Consider implementing IP whitelisting for sensitive operations