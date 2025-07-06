# Multi-Tenant API Key Implementation Plan

> **Goal**: Extend the existing nginx + auth service to support multiple API key types and token passthrough
> **Timeline**: 2 weeks
> **Approach**: Incremental changes to existing services without adding new infrastructure
> **Status**: Phase 1 Complete ✅ | Phase 2 Partially Complete ⚠️

## 🎯 Phase 1 Completion Summary

Completed on 2025-07-06:
- ✅ Created database migration for API key types (000023_api_key_types.up.sql)
- ✅ Created secure E2E setup script (no hardcoded keys)
- ✅ Updated auth models to support KeyType
- ✅ Added metadata field to User struct
- ✅ Updated ValidateAPIKey to include key type in metadata
- ✅ Created comprehensive unit tests for KeyType
- ✅ All tests passing, code formatted and linted

## 📋 Executive Summary

This plan extends the current DevOps MCP authentication system to support:
- Multiple API key types (admin, agent, gateway, user)
- Secure token passthrough for GitHub/GitLab/Bitbucket
- Per-tenant rate limiting and configuration
- E2E test support with proper API keys

All changes build on the existing nginx reverse proxy and auth service - no new gateway required.

## 🎯 Problem We're Solving

1. **E2E Tests Failing**: Production expects API keys in database, not static configuration
2. **Multi-Agent Support**: Need different API keys for different agent types
3. **Local MCP Integration**: Users need to pass their GitHub tokens through safely
4. **Per-Tenant Limits**: Different rate limits and features per tenant

## 📦 Phase 1: Database Schema Updates (Day 1-2) ✅

### 1.1 Create Migration for API Key Types ✅

```sql
-- migrations/000003_api_key_types.up.sql
BEGIN;

-- Add key type and gateway features to api_keys
ALTER TABLE mcp.api_keys 
ADD COLUMN IF NOT EXISTS key_type VARCHAR(50) NOT NULL DEFAULT 'user',
ADD COLUMN IF NOT EXISTS parent_key_id UUID REFERENCES mcp.api_keys(id),
ADD COLUMN IF NOT EXISTS allowed_services TEXT[] DEFAULT '{}';

-- Create tenant configuration table
CREATE TABLE IF NOT EXISTS mcp.tenant_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID UNIQUE NOT NULL,
    
    -- Rate limit overrides
    rate_limit_config JSONB NOT NULL DEFAULT '{}',
    
    -- Service tokens (encrypted)
    service_tokens JSONB DEFAULT '{}', -- {"github": "encrypted_token", ...}
    
    -- Allowed origins for CORS
    allowed_origins TEXT[] DEFAULT '{}',
    
    -- Feature flags
    features JSONB DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Add indexes
CREATE INDEX idx_api_keys_type ON mcp.api_keys(key_type, tenant_id) WHERE is_active = true;
CREATE INDEX idx_api_keys_parent ON mcp.api_keys(parent_key_id) WHERE parent_key_id IS NOT NULL;

-- Update existing keys to have type 'user'
UPDATE mcp.api_keys SET key_type = 'user' WHERE key_type IS NULL;

COMMIT;
```

### 1.2 Insert E2E Test API Key ✅

Created a secure shell script that:
- Reads E2E_API_KEY from environment variable
- Generates SHA256 hash dynamically
- Inserts key into production database with proper tenant setup
- No hardcoded API keys in the codebase

```bash
# scripts/setup_e2e_api_key.sh
#!/bin/bash
# Usage: E2E_API_KEY=your_key ./setup_e2e_api_key.sh
# Reads API key from environment, hashes it, and inserts into database
```

### Tasks for Claude Code: ✅
```bash
# Create and run migration
make migration name=api_key_types
make migrate-up

# Run E2E setup script
E2E_API_KEY=$E2E_API_KEY ./scripts/setup_e2e_api_key.sh

# Verify
psql $DATABASE_URL -c "SELECT key_prefix, key_type, name FROM mcp.api_keys WHERE key_prefix = 'cacacb6b'"
```

## 📦 Phase 2: Extend Auth Service (Day 3-5) ⚠️ Partially Complete

### 2.1 Add Key Type Support ✅

```go
// pkg/auth/key_types.go
package auth

import (
    "context"
    "database/sql/driver"
)

// KeyType represents the type of API key
type KeyType string

const (
    KeyTypeUser    KeyType = "user"     // Regular user access
    KeyTypeAdmin   KeyType = "admin"    // Full system access
    KeyTypeAgent   KeyType = "agent"    // AI agents
    KeyTypeGateway KeyType = "gateway"  // Local MCP instances
)

// Valid returns true if the key type is valid
func (kt KeyType) Valid() bool {
    switch kt {
    case KeyTypeUser, KeyTypeAdmin, KeyTypeAgent, KeyTypeGateway:
        return true
    default:
        return false
    }
}

// Scan implements sql.Scanner for database operations
func (kt *KeyType) Scan(value interface{}) error {
    if value == nil {
        *kt = KeyTypeUser
        return nil
    }
    
    switch v := value.(type) {
    case string:
        *kt = KeyType(v)
    case []byte:
        *kt = KeyType(string(v))
    default:
        return fmt.Errorf("cannot scan %T into KeyType", value)
    }
    
    if !kt.Valid() {
        *kt = KeyTypeUser
    }
    
    return nil
}

// Value implements driver.Valuer for database operations
func (kt KeyType) Value() (driver.Value, error) {
    return string(kt), nil
}
```

### 2.2 Enhanced API Key Structure ✅

```go
// pkg/auth/auth.go - Update APIKey struct
type APIKey struct {
    Key        string     `db:"key"`
    KeyHash    string     `db:"key_hash"`
    KeyPrefix  string     `db:"key_prefix"`
    TenantID   string     `db:"tenant_id"`
    UserID     string     `db:"user_id"`
    Name       string     `db:"name"`
    KeyType    KeyType    `db:"key_type"`      // NEW
    Scopes     []string   `db:"scopes"`
    ExpiresAt  *time.Time `db:"expires_at"`
    CreatedAt  time.Time  `db:"created_at"`
    LastUsed   *time.Time `db:"last_used"`
    Active     bool       `db:"is_active"`
    
    // Gateway-specific fields
    ParentKeyID      *string  `db:"parent_key_id"`     // NEW
    AllowedServices  []string `db:"allowed_services"`  // NEW
    
    // Rate limiting
    RateLimitRequests      int `db:"rate_limit_requests"`
    RateLimitWindowSeconds int `db:"rate_limit_window_seconds"`
}
```

### 2.3 Update Validation to Include Key Type ✅

```go
// pkg/auth/auth.go - Update ValidateAPIKey method
func (s *Service) ValidateAPIKey(ctx context.Context, apiKey string) (*User, error) {
    if apiKey == "" {
        return nil, ErrNoAPIKey
    }

    // Check cache first if enabled
    if s.config != nil && s.config.CacheEnabled && s.cache != nil {
        cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
        var cachedUser User
        if err := s.cache.Get(ctx, cacheKey, &cachedUser); err == nil {
            cachedUser.AuthType = TypeAPIKey
            return &cachedUser, nil
        }
    }

    // Hash the API key
    keyHash := s.hashAPIKey(apiKey)
    
    // Query database with key type
    var key APIKey
    query := `
        SELECT 
            key_hash, key_prefix, tenant_id, user_id, name, key_type,
            scopes, is_active, expires_at, rate_limit_requests,
            rate_limit_window_seconds, parent_key_id, allowed_services
        FROM mcp.api_keys 
        WHERE key_hash = $1 AND is_active = true
    `
    
    err := s.db.GetContext(ctx, &key, query, keyHash)
    if err != nil {
        if err == sql.ErrNoRows {
            s.logger.Warn("API key not found", map[string]interface{}{
                "key_prefix": getKeyPrefix(apiKey),
            })
            return nil, ErrInvalidAPIKey
        }
        return nil, err
    }
    
    // Check expiration
    if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
        return nil, ErrTokenExpired
    }
    
    // Update last used timestamp asynchronously
    go s.updateLastUsed(context.Background(), key.KeyHash)
    
    // Create user with key type in metadata
    user := &User{
        ID:       key.UserID,
        TenantID: key.TenantID,
        Scopes:   key.Scopes,
        AuthType: TypeAPIKey,
        Metadata: map[string]interface{}{
            "key_type":         key.KeyType,
            "key_name":         key.Name,
            "allowed_services": key.AllowedServices,
        },
    }
    
    // Cache the user
    if s.config != nil && s.config.CacheEnabled && s.cache != nil {
        cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
        _ = s.cache.Set(ctx, cacheKey, user, s.config.CacheTTL)
    }
    
    return user, nil
}
```

### 2.4 Add Method to Create Different Key Types

```go
// pkg/auth/api_keys.go
package auth

type CreateAPIKeyRequest struct {
    Name            string   `json:"name" binding:"required"`
    TenantID        string   `json:"tenant_id" binding:"required"`
    UserID          string   `json:"user_id"`
    KeyType         KeyType  `json:"key_type" binding:"required"`
    Scopes          []string `json:"scopes"`
    ExpiresAt       *time.Time `json:"expires_at"`
    
    // Gateway-specific
    AllowedServices []string `json:"allowed_services,omitempty"`
    ParentKeyID     *string  `json:"parent_key_id,omitempty"`
    
    // Rate limiting
    RateLimit       *int     `json:"rate_limit,omitempty"`
}

func (s *Service) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (*APIKey, error) {
    // Validate key type
    if !req.KeyType.Valid() {
        return nil, fmt.Errorf("invalid key type: %s", req.KeyType)
    }
    
    // Generate secure random key
    keyBytes := make([]byte, 32)
    if _, err := rand.Read(keyBytes); err != nil {
        return nil, err
    }
    
    // Create key string: prefix + base64(random)
    keyString := fmt.Sprintf("%s_%s", generatePrefix(req.KeyType), base64.URLEncoding.EncodeToString(keyBytes))
    keyHash := s.hashAPIKey(keyString)
    keyPrefix := keyString[:8]
    
    // Set default rate limit based on key type
    rateLimit := 100
    if req.RateLimit != nil {
        rateLimit = *req.RateLimit
    } else {
        switch req.KeyType {
        case KeyTypeAdmin:
            rateLimit = 10000
        case KeyTypeGateway:
            rateLimit = 5000
        case KeyTypeAgent:
            rateLimit = 1000
        }
    }
    
    // Insert into database
    query := `
        INSERT INTO mcp.api_keys (
            key_hash, key_prefix, tenant_id, user_id, name, key_type,
            scopes, is_active, expires_at, rate_limit_requests,
            rate_limit_window_seconds, parent_key_id, allowed_services
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
        ) RETURNING id, created_at
    `
    
    var id string
    var createdAt time.Time
    
    err := s.db.QueryRowContext(ctx, query,
        keyHash, keyPrefix, req.TenantID, req.UserID, req.Name, req.KeyType,
        pq.Array(req.Scopes), true, req.ExpiresAt, rateLimit, 60,
        req.ParentKeyID, pq.Array(req.AllowedServices),
    ).Scan(&id, &createdAt)
    
    if err != nil {
        return nil, err
    }
    
    // Log API key creation
    s.logger.Info("API key created", map[string]interface{}{
        "key_id":    id,
        "key_type":  req.KeyType,
        "tenant_id": req.TenantID,
        "key_name":  req.Name,
    })
    
    return &APIKey{
        Key:       keyString,  // Only returned once
        KeyPrefix: keyPrefix,
        TenantID:  req.TenantID,
        UserID:    req.UserID,
        Name:      req.Name,
        KeyType:   req.KeyType,
        Scopes:    req.Scopes,
        Active:    true,
        CreatedAt: createdAt,
    }, nil
}

func generatePrefix(keyType KeyType) string {
    switch keyType {
    case KeyTypeAdmin:
        return "adm"
    case KeyTypeGateway:
        return "gw"
    case KeyTypeAgent:
        return "agt"
    default:
        return "usr"
    }
}
```

### Tasks for Claude Code:
```bash
# Update auth service
make test pkg=pkg/auth

# Generate API key management endpoints
make generate-crud model=api_key

# Test key creation
go test -v ./pkg/auth -run TestCreateAPIKey
```

## 📦 Phase 3: Token Passthrough (Day 6-8)

### 3.1 Create Passthrough Context

```go
// pkg/auth/passthrough.go
package auth

import (
    "context"
    "errors"
)

type contextKey string

const (
    passthroughTokenKey contextKey = "passthrough_token"
    tokenProviderKey    contextKey = "token_provider"
    gatewayIDKey        contextKey = "gateway_id"
)

// PassthroughToken represents a token to be passed to external services
type PassthroughToken struct {
    Provider string // github, gitlab, bitbucket
    Token    string
    Scopes   []string
}

// WithPassthroughToken adds a passthrough token to the context
func WithPassthroughToken(ctx context.Context, token PassthroughToken) context.Context {
    return context.WithValue(ctx, passthroughTokenKey, token)
}

// GetPassthroughToken retrieves a passthrough token from the context
func GetPassthroughToken(ctx context.Context) (*PassthroughToken, bool) {
    token, ok := ctx.Value(passthroughTokenKey).(PassthroughToken)
    if !ok {
        return nil, false
    }
    return &token, true
}
```

### 3.2 Update Middleware for Gateway Keys

```go
// pkg/auth/middleware.go - Add to existing AuthMiddleware
func (m *AuthMiddleware) HandleRequestWithPassthrough() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Standard auth validation
        m.HandleRequest()(c)
        
        // If authenticated, check for gateway features
        if user, exists := c.Get("user"); exists {
            u := user.(*User)
            
            // Check if this is a gateway key
            if metadata, ok := u.Metadata.(map[string]interface{}); ok {
                if keyType, ok := metadata["key_type"].(string); ok && keyType == string(KeyTypeGateway) {
                    // Extract passthrough token from header
                    if userToken := c.GetHeader("X-User-Token"); userToken != "" {
                        provider := c.GetHeader("X-Token-Provider")
                        
                        // Validate provider is allowed
                        allowedServices, _ := metadata["allowed_services"].([]string)
                        if !contains(allowedServices, provider) {
                            c.AbortWithStatusJSON(403, gin.H{
                                "error": fmt.Sprintf("Provider %s not allowed for this gateway key", provider),
                            })
                            return
                        }
                        
                        // Add to context
                        ctx := WithPassthroughToken(c.Request.Context(), PassthroughToken{
                            Provider: provider,
                            Token:    userToken,
                        })
                        c.Request = c.Request.WithContext(ctx)
                    }
                }
            }
        }
        
        c.Next()
    }
}
```

### 3.3 Update GitHub Adapter

```go
// pkg/adapters/github/factory.go - Update CreateClient method
func (f *ClientFactory) CreateClient(ctx context.Context) (*github.Client, error) {
    // Check for passthrough token first
    if token, ok := GetPassthroughToken(ctx); ok && token.Provider == "github" {
        f.logger.Info("Using passthrough GitHub token", map[string]interface{}{
            "has_token": true,
        })
        
        ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token.Token})
        tc := oauth2.NewClient(ctx, ts)
        return github.NewClient(tc), nil
    }
    
    // Fall back to service token
    return f.createServiceClient(ctx)
}
```

### Tasks for Claude Code:
```bash
# Update middleware
make test pkg=pkg/auth/middleware

# Update adapters
make update-adapter adapter=github
make update-adapter adapter=gitlab

# Test passthrough
make test-passthrough
```

## 📦 Phase 4: Per-Tenant Configuration (Day 9-10)

### 4.1 Tenant Configuration Service

```go
// pkg/services/tenant_config.go
package services

type TenantConfigService struct {
    db     *sqlx.DB
    cache  cache.Cache
    logger observability.Logger
}

type TenantConfig struct {
    TenantID        string            `db:"tenant_id"`
    RateLimitConfig json.RawMessage   `db:"rate_limit_config"`
    ServiceTokens   map[string]string `db:"-"` // Decrypted in memory
    AllowedOrigins  []string          `db:"allowed_origins"`
    Features        map[string]bool   `db:"-"`
}

func (s *TenantConfigService) GetConfig(ctx context.Context, tenantID string) (*TenantConfig, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("tenant:config:%s", tenantID)
    
    var config TenantConfig
    if err := s.cache.Get(ctx, cacheKey, &config); err == nil {
        return &config, nil
    }
    
    // Query database
    query := `
        SELECT tenant_id, rate_limit_config, service_tokens, 
               allowed_origins, features
        FROM mcp.tenant_config
        WHERE tenant_id = $1
    `
    
    err := s.db.GetContext(ctx, &config, query, tenantID)
    if err != nil {
        if err == sql.ErrNoRows {
            // Return default config
            return s.defaultConfig(tenantID), nil
        }
        return nil, err
    }
    
    // Decrypt service tokens
    if err := s.decryptTokens(&config); err != nil {
        s.logger.Error("Failed to decrypt tokens", map[string]interface{}{
            "tenant_id": tenantID,
            "error":     err.Error(),
        })
    }
    
    // Cache for 5 minutes
    _ = s.cache.Set(ctx, cacheKey, config, 5*time.Minute)
    
    return &config, nil
}
```

### 4.2 Apply Tenant Config in Auth

```go
// pkg/auth/tenant_aware.go
package auth

func (s *Service) ValidateAPIKeyWithTenantConfig(ctx context.Context, apiKey string) (*User, *TenantConfig, error) {
    // Validate API key
    user, err := s.ValidateAPIKey(ctx, apiKey)
    if err != nil {
        return nil, nil, err
    }
    
    // Load tenant config
    config, err := s.tenantConfigService.GetConfig(ctx, user.TenantID)
    if err != nil {
        // Log but don't fail - use defaults
        s.logger.Warn("Failed to load tenant config", map[string]interface{}{
            "tenant_id": user.TenantID,
            "error":     err.Error(),
        })
        config = s.tenantConfigService.DefaultConfig(user.TenantID)
    }
    
    return user, config, nil
}
```

### Tasks for Claude Code:
```bash
# Create tenant config service
make generate-service name=tenant_config

# Add encryption for tokens
make add-encryption service=tenant_config

# Test tenant config
make test pkg=pkg/services/tenant_config
```

## 📦 Phase 5: Testing & Documentation (Day 11-14)

### 5.1 Fix E2E Tests

```go
// test/e2e/setup_test.go
package e2e

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestE2ESetup(t *testing.T) {
    // Verify API key is in database
    db := setupTestDB(t)
    
    var exists bool
    err := db.Get(&exists, `
        SELECT EXISTS(
            SELECT 1 FROM mcp.api_keys 
            WHERE key_prefix = 'cacacb6b' 
            AND key_type = 'admin'
            AND is_active = true
        )
    `)
    require.NoError(t, err)
    require.True(t, exists, "E2E API key not found in database")
}
```

### 5.2 Integration Tests

```go
// test/integration/gateway_test.go
package integration

func TestGatewayKeyPassthrough(t *testing.T) {
    // Create gateway key
    gatewayKey, err := authService.CreateAPIKey(ctx, CreateAPIKeyRequest{
        Name:     "Test Gateway",
        TenantID: testTenantID,
        KeyType:  auth.KeyTypeGateway,
        AllowedServices: []string{"github"},
    })
    require.NoError(t, err)
    
    // Make request with passthrough
    req := httptest.NewRequest("GET", "/api/v1/github/user", nil)
    req.Header.Set("Authorization", "Bearer " + gatewayKey.Key)
    req.Header.Set("X-User-Token", "ghp_test_token")
    req.Header.Set("X-Token-Provider", "github")
    
    // Should use passthrough token
    resp := httptest.NewRecorder()
    router.ServeHTTP(resp, req)
    
    assert.Equal(t, 200, resp.Code)
}
```

### 5.3 Create Admin CLI Tool

```bash
#!/bin/bash
# scripts/manage-api-keys.sh

case "$1" in
    create)
        curl -X POST $API_URL/api/v1/admin/keys \
            -H "Authorization: Bearer $ADMIN_KEY" \
            -H "Content-Type: application/json" \
            -d "{
                \"name\": \"$2\",
                \"tenant_id\": \"$3\",
                \"key_type\": \"$4\",
                \"scopes\": [\"read\", \"write\"]
            }"
        ;;
    list)
        psql $DATABASE_URL -c "
            SELECT key_prefix, name, key_type, tenant_id, created_at 
            FROM mcp.api_keys 
            WHERE is_active = true 
            ORDER BY created_at DESC 
            LIMIT 20
        "
        ;;
    revoke)
        psql $DATABASE_URL -c "
            UPDATE mcp.api_keys 
            SET is_active = false 
            WHERE key_prefix = '$2'
        "
        ;;
esac
```

### Tasks for Claude Code:
```bash
# Run all tests
make test-all

# Generate documentation
make docs-api-keys

# Create usage examples
make examples type=gateway
```

## 🚀 Deployment Checklist

- [ ] Database migrations applied to production
- [ ] E2E API key inserted in production database
- [ ] Auth service deployed with key type support
- [ ] REST API deployed with passthrough middleware
- [ ] MCP server deployed with enhanced auth
- [ ] Nginx configuration updated (if using Lua)
- [ ] Monitoring alerts configured for new metrics
- [ ] Documentation updated
- [ ] Admin CLI tool deployed
- [ ] E2E tests passing in production

## 📊 Success Metrics

1. **E2E Tests**: All tests passing with real API key
2. **Performance**: <5ms added latency for auth
3. **Security**: No token leakage in logs
4. **Compatibility**: Existing API keys still work
5. **Multi-tenant**: Per-tenant rate limits enforced

## 🔧 Quick Commands

```bash
# Setup E2E key in production
make prod-setup-e2e

# Create new gateway key
./scripts/manage-api-keys.sh create "Local MCP Gateway" "tenant-123" "gateway"

# Monitor auth performance
make monitor-auth

# Run security scan
make security-scan target=auth
```

This implementation plan provides a practical path to multi-tenant API keys without adding complex infrastructure. It leverages your existing nginx and auth service while adding the features needed for E2E tests and multi-agent support.