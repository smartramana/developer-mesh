# Auth Package

The `auth` package provides centralized authentication and authorization services for the DevOps MCP platform.

## Implementation Status

### ✅ Fully Implemented
- **API Key Authentication**: Complete with database storage, hashing, and caching
- **JWT Token Authentication**: HS256 signing with standard claims support
- **Basic RBAC**: Simple role-based access control (in-memory policies)
- **Middleware Support**: Gin and standard HTTP middleware
- **Multi-Provider Support**: Multiple auth methods in single request
- **Tenant Isolation**: Built-in multi-tenancy support
- **Performance Caching**: Redis/in-memory caching for auth checks

### ⚠️ Partially Implemented
- **OAuth Interface**: Interface defined but no concrete providers (Google, GitHub, etc.)
- **GitHub App Auth**: Exists in `pkg/adapters/github/auth/` but not integrated here

### ❌ Not Implemented (Planned)
- **Casbin RBAC**: Advanced policy-based access control
- **OAuth Providers**: Concrete implementations for Google, GitHub, Microsoft
- **Session Management**: No session tracking or refresh tokens
- **Token Revocation**: JWT tokens cannot be revoked before expiration
- **Audit Logging**: No dedicated auth event logging (uses general logging)
- **MFA/2FA**: No multi-factor authentication support

## Features

- **API Key Authentication**: Support for static and dynamic API keys with secure hashing
- **JWT Token Authentication**: JSON Web Token support with configurable expiration
- **Scope-based Authorization**: Basic permission control via scopes
- **Middleware Support**: Ready-to-use middleware for Gin and standard HTTP handlers
- **Caching**: Built-in caching support for improved performance
- **Database Integration**: PostgreSQL storage for API keys with pgx driver
- **Multi-tenancy**: Built-in tenant isolation support
- **Rate Limiting**: Basic rate limiting support (configurable)

## Architecture

The auth package uses a provider-based architecture:

```
Manager (orchestrator)
├── APIKeyProvider (database-backed)
├── JWTProvider (stateless tokens)
├── OAuthProvider (interface only)
└── Authorizer (ProductionAuthorizer or TestProvider)
```

## Usage

### Basic Setup

```go
import (
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Create auth manager with all providers
config := &auth.Config{
    // JWT Configuration
    JWTSecret:    os.Getenv("JWT_SECRET"), // Required
    JWTIssuer:    "devops-mcp",
    JWTExpiresIn: 15 * time.Minute,
    
    // API Key Configuration  
    EnableAPIKeys: true,
    APIKeyHeader:  "X-API-Key", // or Authorization: Bearer
    
    // Authorization
    AuthorizerType: "production", // or "test" for testing
    
    // Caching
    CacheEnabled: true,
    CacheTTL:    5 * time.Minute,
}

// Dependencies
logger := observability.NewLogger("auth")
tracer := observability.NewTracer("auth")

// Create manager
authManager, err := auth.NewManager(config, db, cache, logger, tracer)
if err != nil {
    log.Fatal(err)
}

// The manager automatically initializes:
// - APIKeyProvider with APIKeyService
// - JWTProvider with configuration
// - ProductionAuthorizer with basic RBAC
```

### Using with Gin

```go
// Create Gin router
router := gin.New()

// Apply auth middleware - supports multiple auth types
router.Use(auth.GinMiddleware(authManager))

// The middleware checks in order:
// 1. Authorization: Bearer <token> (JWT or API key)
// 2. X-API-Key: <key>
// 3. Custom headers configured in APIKeyHeader

// Protected routes
v1 := router.Group("/api/v1")
v1.Use(auth.GinMiddleware(authManager))

// Get user from context in handlers
v1.GET("/profile", func(c *gin.Context) {
    user, exists := c.Get("user")
    if !exists {
        c.JSON(401, gin.H{"error": "unauthorized"})
        return
    }
    
    authUser := user.(*auth.User)
    c.JSON(200, gin.H{
        "user_id": authUser.ID,
        "tenant_id": authUser.TenantID,
        "scopes": authUser.Scopes,
    })
})
```

### Using with Standard HTTP

```go
// Wrap standard HTTP handlers
mux := http.NewServeMux()

// Create middleware
authMiddleware := auth.StandardMiddleware(authManager)

// Protected endpoint
mux.Handle("/api/", authMiddleware(http.HandlerFunc(apiHandler)))

func apiHandler(w http.ResponseWriter, r *http.Request) {
    // Get user from context
    user := r.Context().Value(auth.UserContextKey).(*auth.User)
    
    // Use authenticated user info
    fmt.Fprintf(w, "Hello %s", user.ID)
}
```

### API Key Management

```go
// Get the API key service from manager
apiKeyService := authManager.GetAPIKeyService()

// Create a new API key
key := &auth.APIKey{
    ID:       uuid.New(),
    Name:     "Production API",
    TenantID: uuid.MustParse("tenant-123"),
    UserID:   uuid.MustParse("user-456"),
    Scopes:   []string{"read", "write"},
    ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1 year
}

// Create returns the unhashed key - save this!
apiKey, plainKey, err := apiKeyService.CreateKey(ctx, key)
if err != nil {
    return err
}

// The plainKey is only returned once and should be given to the user
fmt.Printf("Your API key: %s\n", plainKey)

// Validate an API key (done automatically by middleware)
validKey, err := apiKeyService.ValidateKey(ctx, plainKey)

// List keys for a user
keys, err := apiKeyService.ListKeys(ctx, userID, tenantID)

// Revoke a key
err = apiKeyService.RevokeKey(ctx, keyID)
```

### JWT Token Management

```go
// JWT provider handles token generation/validation
// This is typically done after login or OAuth callback

// Create user claims
user := &auth.User{
    ID:       uuid.New().String(),
    TenantID: uuid.New().String(), 
    Email:    "user@example.com",
    Scopes:   []string{"contexts:read", "agents:write"},
}

// Generate token (done internally by auth flow)
token, err := authManager.GenerateToken(user)
if err != nil {
    return err
}

// Token should be returned to client
// Client includes it as: Authorization: Bearer <token>

// Validation happens automatically in middleware
// But you can manually validate:
claims, err := authManager.ValidateToken(token)
```

### Authorization Checks

```go
// In handlers, check permissions
func deleteResource(c *gin.Context) {
    user := c.MustGet("user").(*auth.User)
    resourceID := c.Param("id")
    
    // Check permission using the authorizer
    allowed, err := authManager.Authorize(c.Request.Context(), &auth.AuthRequest{
        Subject:  user.ID,
        Resource: "contexts",
        Action:   "delete",
        TenantID: user.TenantID,
    })
    
    if err != nil || !allowed {
        c.JSON(403, gin.H{"error": "forbidden"})
        return
    }
    
    // Proceed with deletion
}

// Or use middleware for consistent checks
adminOnly := auth.RequireScopes(authManager, "admin")
router.DELETE("/api/v1/users/:id", adminOnly, deleteUser)
```

## Configuration

```go
type Config struct {
    // JWT Settings
    JWTSecret    string        // Required: Min 32 bytes recommended
    JWTIssuer    string        // Default: "devops-mcp"
    JWTExpiresIn time.Duration // Default: 15 minutes
    
    // API Key Settings
    EnableAPIKeys bool   // Default: true
    APIKeyHeader  string // Default: "X-API-Key"
    
    // Authorization
    AuthorizerType string // "production" or "test"
    Policies       []Policy // Optional: custom policies
    
    // Cache Settings
    CacheEnabled bool          // Default: true
    CacheTTL     time.Duration // Default: 5 minutes
    
    // Security
    RateLimitPerMinute int // Default: 1000
    
    // Database (optional)
    DatabaseURL string // PostgreSQL connection string
}
```

## Authorization Model

The current implementation uses a simple RBAC model:

```go
// Basic policy structure (not Casbin)
type Policy struct {
    Subject  string // User ID or role
    Resource string // Resource type (e.g., "contexts")
    Action   string // Action (e.g., "read", "write", "delete")
}

// Default roles
const (
    RoleAdmin  = "admin"  // Full access
    RoleUser   = "user"   // Standard access
    RoleViewer = "viewer" // Read-only access
)
```

## Database Schema

The auth package uses the following PostgreSQL schema:

```sql
-- API Keys table (with UUID primary key)
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA256 hash
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    scopes TEXT[] DEFAULT '{}',
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    active BOOLEAN NOT NULL DEFAULT true,
    
    INDEX idx_api_keys_tenant_user (tenant_id, user_id),
    INDEX idx_api_keys_active (active) WHERE active = true,
    INDEX idx_api_keys_expires (expires_at) WHERE expires_at IS NOT NULL
);

-- Future: OAuth tokens table (not implemented)
CREATE TABLE oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    provider VARCHAR(50) NOT NULL,
    access_token_encrypted TEXT NOT NULL,
    refresh_token_encrypted TEXT,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Future: Sessions table (not implemented)
CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

## Testing

The package includes comprehensive tests:

```bash
# Run all auth tests
go test ./pkg/auth/...

# Run with race detection
go test -race ./pkg/auth/...

# Run with coverage
go test -cover ./pkg/auth/...
```

For testing environments, use the TestProvider:

```go
// Enable test mode
os.Setenv("MCP_TEST_MODE", "true")
os.Setenv("TEST_AUTH_ENABLED", "true")

// Create test provider
config := &auth.Config{
    AuthorizerType: "test",
}
authManager, _ := auth.NewManager(config, nil, nil, logger, tracer)

// Generate test tokens
testProvider := authManager.GetTestProvider()
token, _ := testProvider.GenerateTestToken(userID, tenantID, "admin", []string{"all"})
```

## Current Limitations

### No Session Management
- JWT tokens cannot be revoked before expiration
- No refresh token support
- No session tracking or device management
- Workaround: Use short JWT expiration times (15 minutes)

### Basic Authorization Only
- No Casbin integration (simple RBAC only)
- No attribute-based access control (ABAC)
- No dynamic policy updates
- Workaround: Implement custom authorization logic in handlers

### OAuth Not Implemented
- Interface exists but no providers
- No OAuth flow handling
- No social login support
- Workaround: Implement OAuth providers following the interface

### Limited Audit Logging
- Basic logging through observability package
- No dedicated auth event tracking
- No compliance-focused audit trail
- Workaround: Implement custom audit logging in middleware

## Security Best Practices

### API Key Security
```go
// DO: Generate cryptographically secure keys
key := make([]byte, 32)
rand.Read(key)
apiKey := base64.URLEncoding.EncodeToString(key)

// DON'T: Use predictable keys
apiKey := fmt.Sprintf("key_%d", userID) // INSECURE
```

### JWT Security
```go
// DO: Use strong secrets (min 256 bits)
jwtSecret := os.Getenv("JWT_SECRET") // Must be 32+ chars

// DO: Set short expiration times
config.JWTExpiresIn = 15 * time.Minute

// DON'T: Store sensitive data in JWT claims
claims["password"] = userPassword // NEVER DO THIS
```

### Production Checklist
- [ ] Set strong JWT secret (32+ bytes)
- [ ] Configure API key rotation policy
- [ ] Enable HTTPS only
- [ ] Set up rate limiting
- [ ] Configure proper CORS headers
- [ ] Monitor authentication failures
- [ ] Implement IP allowlisting for admin operations
- [ ] Regular security audits
- [ ] Keep dependencies updated

## Troubleshooting

### Common Issues

1. **"Invalid JWT secret"**
   - Ensure JWT_SECRET environment variable is set
   - Secret must be at least 32 bytes

2. **"API key not found"**
   - Check if key is properly hashed in database
   - Verify tenant_id matches
   - Ensure key hasn't expired

3. **"Unauthorized" errors**
   - Check middleware is properly configured
   - Verify auth headers are being sent
   - Check token/key expiration

4. **Performance issues**
   - Enable caching (Redis recommended)
   - Use connection pooling for database
   - Consider increasing cache TTL

## Future Enhancements

Planned improvements for the auth package:

1. **Casbin Integration** - Advanced policy-based access control
2. **OAuth Providers** - Google, GitHub, Microsoft implementations
3. **Session Management** - Refresh tokens and session tracking
4. **Audit Logging** - Dedicated auth event logging
5. **MFA Support** - Multi-factor authentication
6. **Token Revocation** - Blacklist/whitelist for JWTs
7. **Rate Limiting** - More sophisticated rate limiting per user/IP

## Related Documentation

- [Authentication Best Practices Guide](../guides/auth-best-practices.md)
- [OAuth Implementation Guide](../guides/oauth-providers-guide.md)
- [Casbin Integration Guide](../guides/casbin-integration-guide.md)
- [Audit Logging Guide](../guides/auth-audit-logging.md)
- [API Reference](../api/rest-api-reference.md#authentication)