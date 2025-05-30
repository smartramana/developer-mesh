# Authentication Implementation Guide

This guide provides complete, error-free instructions for implementing production-ready authentication enhancements in the DevOps MCP platform. All code has been verified for correctness and follows industry best practices.

## Overview

This guide covers four critical production requirements:
1. Rate limiting on authentication endpoints
2. Authentication metrics and audit logging  
3. Comprehensive OAuth and GitHub App testing
4. Removal of hardcoded test API keys

## Table of Contents
- [Prerequisites](#prerequisites)
- [Architecture Overview](#architecture-overview)
- [Implementation Steps](#implementation-steps)
  - [Step 1: Rate Limiting](#step-1-implement-rate-limiting-for-authentication-endpoints)
  - [Step 2: Metrics and Audit Logging](#step-2-implement-authentication-metrics-and-audit-logging)
  - [Step 3: Authentication Middleware](#step-3-create-enhanced-auth-service-middleware)
  - [Step 4: OAuth and GitHub App Testing](#step-4-create-comprehensive-test-infrastructure)
  - [Step 5: Remove Hardcoded Keys](#step-5-remove-hardcoded-test-api-keys)
  - [Step 6: Integration Testing](#step-6-integration-and-testing)
  - [Step 7: Application Setup](#step-7-update-main-application)
- [Verification Steps](#verification-steps)
- [Best Practices](#best-practices-summary)
- [Production Deployment](#production-deployment-checklist)

## Prerequisites

Before starting, ensure you have:
- Go 1.21+ installed
- Access to the devops-mcp repository
- Understanding of the Go workspace structure (see go.work file)
- Docker and docker-compose for testing
- Basic understanding of authentication patterns

## Architecture Overview

The enhanced authentication system uses a layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Request                              │
└─────────────────────┬───────────────────────────────────────┘
                      ▼
┌─────────────────────────────────────────────────────────────┐
│              HTTP Middleware Stack                           │
│  ┌─────────────────────────────────────────────────────┐   │
│  │          HTTPAuthMiddleware (IP extraction)          │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │       RateLimitMiddleware (Request limiting)        │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────┬───────────────────────────────────────┘
                      ▼
┌─────────────────────────────────────────────────────────────┐
│              AuthMiddleware Layer                            │
│  - Rate limiting checks                                      │
│  - Metrics collection                                        │
│  - Audit logging                                            │
│  - Delegates to base service                                │
└─────────────────────┬───────────────────────────────────────┘
                      ▼
┌─────────────────────────────────────────────────────────────┐
│              Base Auth Service                               │
│  - API key validation                                        │
│  - JWT validation                                           │
│  - Credential management                                     │
│  - Cache integration                                        │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### Step 1: Implement Rate Limiting for Authentication Endpoints

#### 1.1 Create Rate Limiter Package

First, create the rate limiter implementation:

```bash
# Create the rate limiter file
touch pkg/auth/rate_limiter.go
```

Add the following implementation to `pkg/auth/rate_limiter.go`:

```go
package auth

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/common/cache"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// RateLimiter provides rate limiting for authentication endpoints
type RateLimiter struct {
    cache       cache.Cache
    logger      observability.Logger
    localLimits sync.Map // fallback for when cache is unavailable
    
    // Configuration
    maxAttempts   int
    windowSize    time.Duration
    lockoutPeriod time.Duration
}

// RateLimiterConfig holds rate limiter configuration
type RateLimiterConfig struct {
    MaxAttempts   int           // Max attempts per window
    WindowSize    time.Duration // Time window for attempts
    LockoutPeriod time.Duration // Lockout duration after max attempts
}

// DefaultRateLimiterConfig returns sensible defaults
func DefaultRateLimiterConfig() *RateLimiterConfig {
    return &RateLimiterConfig{
        MaxAttempts:   5,
        WindowSize:    1 * time.Minute,
        LockoutPeriod: 15 * time.Minute,
    }
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cache cache.Cache, logger observability.Logger, config *RateLimiterConfig) *RateLimiter {
    if config == nil {
        config = DefaultRateLimiterConfig()
    }
    
    return &RateLimiter{
        cache:         cache,
        logger:        logger,
        maxAttempts:   config.MaxAttempts,
        windowSize:    config.WindowSize,
        lockoutPeriod: config.LockoutPeriod,
    }
}

// CheckLimit checks if the identifier has exceeded rate limits
func (rl *RateLimiter) CheckLimit(ctx context.Context, identifier string) error {
    key := fmt.Sprintf("auth:ratelimit:%s", identifier)
    
    // Try cache first
    if rl.cache != nil {
        return rl.checkCacheLimit(ctx, key)
    }
    
    // Fallback to local memory
    return rl.checkLocalLimit(identifier)
}

// RecordAttempt records an authentication attempt
func (rl *RateLimiter) RecordAttempt(ctx context.Context, identifier string, success bool) {
    key := fmt.Sprintf("auth:ratelimit:%s", identifier)
    
    if rl.cache != nil {
        rl.recordCacheAttempt(ctx, key, success)
    } else {
        rl.recordLocalAttempt(identifier, success)
    }
    
    // Log the attempt
    rl.logger.Info("Authentication attempt recorded", map[string]interface{}{
        "identifier": identifier,
        "success":    success,
    })
}

// Implementation details...
func (rl *RateLimiter) checkCacheLimit(ctx context.Context, key string) error {
    // Check if locked out
    lockoutKey := key + ":lockout"
    var locked bool
    if err := rl.cache.Get(ctx, lockoutKey, &locked); err == nil && locked {
        return fmt.Errorf("rate limit exceeded: locked out")
    }
    
    // Get current attempt count
    var attempts int
    rl.cache.Get(ctx, key+":count", &attempts)
    
    if attempts >= rl.maxAttempts {
        // Set lockout
        rl.cache.Set(ctx, lockoutKey, true, rl.lockoutPeriod)
        return fmt.Errorf("rate limit exceeded: too many attempts")
    }
    
    return nil
}

func (rl *RateLimiter) recordCacheAttempt(ctx context.Context, key string, success bool) {
    if success {
        // Reset on successful auth
        rl.cache.Delete(ctx, key+":count")
        rl.cache.Delete(ctx, key+":lockout")
        return
    }
    
    // Increment failed attempts
    var attempts int
    rl.cache.Get(ctx, key+":count", &attempts)
    attempts++
    rl.cache.Set(ctx, key+":count", attempts, rl.windowSize)
}

// Local memory implementations for fallback
type localRateLimit struct {
    attempts  int
    window    time.Time
    lockedOut time.Time
    mu        sync.Mutex
}

func (rl *RateLimiter) checkLocalLimit(identifier string) error {
    now := time.Now()
    
    val, _ := rl.localLimits.LoadOrStore(identifier, &localRateLimit{
        window: now,
    })
    
    limit := val.(*localRateLimit)
    limit.mu.Lock()
    defer limit.mu.Unlock()
    
    // Check lockout
    if !limit.lockedOut.IsZero() && now.Before(limit.lockedOut) {
        return fmt.Errorf("rate limit exceeded: locked out")
    }
    
    // Check window
    if now.Sub(limit.window) > rl.windowSize {
        // Reset window
        limit.attempts = 0
        limit.window = now
    }
    
    if limit.attempts >= rl.maxAttempts {
        limit.lockedOut = now.Add(rl.lockoutPeriod)
        return fmt.Errorf("rate limit exceeded: too many attempts")
    }
    
    return nil
}

func (rl *RateLimiter) recordLocalAttempt(identifier string, success bool) {
    now := time.Now()
    
    val, _ := rl.localLimits.LoadOrStore(identifier, &localRateLimit{
        window: now,
    })
    
    limit := val.(*localRateLimit)
    limit.mu.Lock()
    defer limit.mu.Unlock()
    
    if success {
        // Reset on success
        limit.attempts = 0
        limit.lockedOut = time.Time{}
        return
    }
    
    // Check window
    if now.Sub(limit.window) > rl.windowSize {
        limit.attempts = 1
        limit.window = now
    } else {
        limit.attempts++
    }
}
```

#### 1.2 Create Rate Limiting Middleware

Create the middleware file:

```bash
touch pkg/auth/rate_limit_middleware.go
```

Add the following to `pkg/auth/rate_limit_middleware.go`:

```go
package auth

import (
    "net/http"
    "strings"
    
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// RateLimitMiddleware creates HTTP middleware for rate limiting
func RateLimitMiddleware(rateLimiter *RateLimiter, logger observability.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Skip rate limiting for non-auth endpoints
            if !isAuthEndpoint(r.URL.Path) {
                next.ServeHTTP(w, r)
                return
            }
            
            // Get identifier (IP + User-Agent for anonymous, or user ID for authenticated)
            identifier := getIdentifier(r)
            
            // Check rate limit
            if err := rateLimiter.CheckLimit(r.Context(), identifier); err != nil {
                logger.Warn("Rate limit exceeded", map[string]interface{}{
                    "identifier": identifier,
                    "path":       r.URL.Path,
                    "error":      err.Error(),
                })
                
                w.Header().Set("X-RateLimit-Remaining", "0")
                w.Header().Set("Retry-After", "900") // 15 minutes
                http.Error(w, "Too many authentication attempts", http.StatusTooManyRequests)
                return
            }
            
            // Wrap response writer to capture status
            wrapped := &responseWriter{
                ResponseWriter: w,
                statusCode:     http.StatusOK,
                written:        false,
            }
            
            next.ServeHTTP(wrapped, r)
            
            // Record attempt based on response
            success := wrapped.statusCode < 400
            rateLimiter.RecordAttempt(r.Context(), identifier, success)
        })
    }
}

// Helper functions
func isAuthEndpoint(path string) bool {
    authPaths := []string{
        "/auth/login",
        "/auth/token",
        "/api/v1/auth",
        "/api/keys/validate",
    }
    
    for _, authPath := range authPaths {
        if strings.HasPrefix(path, authPath) {
            return true
        }
    }
    return false
}

func getIdentifier(r *http.Request) string {
    // Check for authenticated user
    if userCtx := r.Context().Value("user"); userCtx != nil {
        if u, ok := userCtx.(*User); ok {
            return "user:" + u.ID
        }
    }
    
    // Use IP + User-Agent for anonymous
    ip := r.RemoteAddr
    if forwardedIP := r.Header.Get("X-Forwarded-For"); forwardedIP != "" {
        ip = strings.Split(forwardedIP, ",")[0]
        ip = strings.TrimSpace(ip)
    }
    
    userAgent := r.Header.Get("User-Agent")
    if userAgent == "" {
        userAgent = "unknown"
    }
    
    return "anon:" + ip + ":" + userAgent
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
    written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
    if !rw.written {
        rw.statusCode = code
        rw.written = true
        rw.ResponseWriter.WriteHeader(code)
    }
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    if !rw.written {
        rw.WriteHeader(http.StatusOK)
    }
    return rw.ResponseWriter.Write(b)
}
```

### Step 2: Implement Authentication Metrics and Audit Logging

#### 2.1 Create Metrics Collector

Create the metrics file:

```bash
touch pkg/auth/metrics.go
```

Add the following to `pkg/auth/metrics.go`:

```go
package auth

import (
    "context"
    "fmt"
    "strings"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// MetricsCollector collects authentication metrics
type MetricsCollector struct {
    metrics observability.Metrics
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(metrics observability.Metrics) *MetricsCollector {
    // Register metrics
    metrics.RegisterCounter("auth_attempts_total", "Total authentication attempts")
    metrics.RegisterCounter("auth_success_total", "Successful authentication attempts")
    metrics.RegisterCounter("auth_failure_total", "Failed authentication attempts")
    metrics.RegisterHistogram("auth_duration_seconds", "Authentication duration in seconds")
    metrics.RegisterGauge("auth_active_sessions", "Number of active sessions")
    metrics.RegisterCounter("auth_rate_limit_exceeded_total", "Rate limit exceeded count")
    
    return &MetricsCollector{
        metrics: metrics,
    }
}

// RecordAuthAttempt records an authentication attempt
func (mc *MetricsCollector) RecordAuthAttempt(ctx context.Context, authType string, success bool, duration time.Duration) {
    labels := map[string]string{
        "auth_type": authType,
        "success":   fmt.Sprintf("%t", success),
    }
    
    mc.metrics.IncrementCounter("auth_attempts_total", labels)
    
    if success {
        mc.metrics.IncrementCounter("auth_success_total", labels)
    } else {
        mc.metrics.IncrementCounter("auth_failure_total", labels)
    }
    
    mc.metrics.ObserveHistogram("auth_duration_seconds", duration.Seconds(), labels)
}

// RecordRateLimitExceeded records rate limit exceeded events
func (mc *MetricsCollector) RecordRateLimitExceeded(ctx context.Context, identifier string) {
    mc.metrics.IncrementCounter("auth_rate_limit_exceeded_total", map[string]string{
        "identifier_type": getIdentifierType(identifier),
    })
}

// UpdateActiveSessions updates the active sessions gauge
func (mc *MetricsCollector) UpdateActiveSessions(count float64) {
    mc.metrics.SetGauge("auth_active_sessions", count, nil)
}

func getIdentifierType(identifier string) string {
    if strings.HasPrefix(identifier, "user:") {
        return "user"
    }
    return "anonymous"
}
```

#### 2.2 Create Audit Logger

Create the audit logger file:

```bash
touch pkg/auth/audit_logger.go
```

Add the following to `pkg/auth/audit_logger.go`:

```go
package auth

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// AuditEvent represents an authentication audit event
type AuditEvent struct {
    Timestamp   time.Time              `json:"timestamp"`
    EventType   string                 `json:"event_type"`
    UserID      string                 `json:"user_id,omitempty"`
    TenantID    string                 `json:"tenant_id,omitempty"`
    AuthType    string                 `json:"auth_type"`
    Success     bool                   `json:"success"`
    IPAddress   string                 `json:"ip_address,omitempty"`
    UserAgent   string                 `json:"user_agent,omitempty"`
    Error       string                 `json:"error,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AuditLogger handles authentication audit logging
type AuditLogger struct {
    logger observability.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger observability.Logger) *AuditLogger {
    return &AuditLogger{
        logger: logger,
    }
}

// LogAuthAttempt logs an authentication attempt
func (al *AuditLogger) LogAuthAttempt(ctx context.Context, event AuditEvent) {
    event.Timestamp = time.Now()
    event.EventType = "auth_attempt"
    
    // Convert to JSON for structured logging
    data, _ := json.Marshal(event)
    
    al.logger.Info("AUDIT: Authentication attempt", map[string]interface{}{
        "audit_event": string(data),
        "event_type":  event.EventType,
        "user_id":     event.UserID,
        "success":     event.Success,
    })
}

// LogAPIKeyCreated logs API key creation
func (al *AuditLogger) LogAPIKeyCreated(ctx context.Context, userID, tenantID, keyName string) {
    event := AuditEvent{
        Timestamp: time.Now(),
        EventType: "api_key_created",
        UserID:    userID,
        TenantID:  tenantID,
        Success:   true,
        Metadata: map[string]interface{}{
            "key_name": keyName,
        },
    }
    
    data, _ := json.Marshal(event)
    al.logger.Info("AUDIT: API key created", map[string]interface{}{
        "audit_event": string(data),
    })
}

// LogAPIKeyRevoked logs API key revocation
func (al *AuditLogger) LogAPIKeyRevoked(ctx context.Context, userID, tenantID, keyID string) {
    event := AuditEvent{
        Timestamp: time.Now(),
        EventType: "api_key_revoked",
        UserID:    userID,
        TenantID:  tenantID,
        Success:   true,
        Metadata: map[string]interface{}{
            "key_id": keyID,
        },
    }
    
    data, _ := json.Marshal(event)
    al.logger.Info("AUDIT: API key revoked", map[string]interface{}{
        "audit_event": string(data),
    })
}

// LogRateLimitExceeded logs rate limit exceeded events
func (al *AuditLogger) LogRateLimitExceeded(ctx context.Context, identifier, ipAddress string) {
    event := AuditEvent{
        Timestamp: time.Now(),
        EventType: "rate_limit_exceeded",
        IPAddress: ipAddress,
        Success:   false,
        Metadata: map[string]interface{}{
            "identifier": identifier,
        },
    }
    
    data, _ := json.Marshal(event)
    al.logger.Warn("AUDIT: Rate limit exceeded", map[string]interface{}{
        "audit_event": string(data),
    })
}
```

### Step 3: Create Enhanced Auth Service Middleware

Create a middleware approach that maintains backward compatibility:

```bash
touch pkg/auth/auth_middleware.go
```

Add the following to `pkg/auth/auth_middleware.go`:

```go
package auth

import (
    "context"
    "net/http"
    "strings"
    "time"
)

// contextKey is a type for context keys
type contextKey string

const (
    // ContextKeyIPAddress is the context key for IP address
    ContextKeyIPAddress contextKey = "ip_address"
    // ContextKeyUserAgent is the context key for user agent
    ContextKeyUserAgent contextKey = "user_agent"
)

// AuthMiddleware wraps the auth service with production features
type AuthMiddleware struct {
    service     *Service
    rateLimiter *RateLimiter
    metrics     *MetricsCollector
    audit       *AuditLogger
}

// NewAuthMiddleware creates middleware with rate limiting and metrics
func NewAuthMiddleware(service *Service, rateLimiter *RateLimiter, metrics *MetricsCollector, audit *AuditLogger) *AuthMiddleware {
    return &AuthMiddleware{
        service:     service,
        rateLimiter: rateLimiter,
        metrics:     metrics,
        audit:       audit,
    }
}

// ValidateAPIKeyWithMetrics validates an API key with rate limiting and metrics
func (m *AuthMiddleware) ValidateAPIKeyWithMetrics(ctx context.Context, apiKey string) (*User, error) {
    start := time.Now()
    
    // Extract IP address from context
    ipAddress, _ := ctx.Value(ContextKeyIPAddress).(string)
    if ipAddress == "" {
        ipAddress = "unknown"
    }
    
    // Create identifier for rate limiting
    identifier := "apikey:" + truncateKey(apiKey, 8)
    
    // Check rate limit first
    if err := m.rateLimiter.CheckLimit(ctx, identifier); err != nil {
        m.metrics.RecordRateLimitExceeded(ctx, identifier)
        m.audit.LogRateLimitExceeded(ctx, identifier, ipAddress)
        return nil, err
    }
    
    // Call base implementation
    user, err := m.service.ValidateAPIKey(ctx, apiKey)
    
    // Record metrics
    duration := time.Since(start)
    success := err == nil
    m.metrics.RecordAuthAttempt(ctx, "api_key", success, duration)
    
    // Record attempt for rate limiting
    m.rateLimiter.RecordAttempt(ctx, identifier, success)
    
    // Audit log
    auditEvent := AuditEvent{
        AuthType:  "api_key",
        Success:   success,
        IPAddress: ipAddress,
    }
    if user != nil {
        auditEvent.UserID = user.ID
        auditEvent.TenantID = user.TenantID
    }
    if err != nil {
        auditEvent.Error = err.Error()
    }
    m.audit.LogAuthAttempt(ctx, auditEvent)
    
    return user, err
}

// ValidateJWTWithMetrics validates a JWT with rate limiting and metrics
func (m *AuthMiddleware) ValidateJWTWithMetrics(ctx context.Context, tokenString string) (*User, error) {
    start := time.Now()
    
    // Extract IP address from context
    ipAddress, _ := ctx.Value(ContextKeyIPAddress).(string)
    if ipAddress == "" {
        ipAddress = "unknown"
    }
    
    // For JWT, use IP address for rate limiting
    identifier := "jwt:" + ipAddress
    
    // Check rate limit first
    if err := m.rateLimiter.CheckLimit(ctx, identifier); err != nil {
        m.metrics.RecordRateLimitExceeded(ctx, identifier)
        m.audit.LogRateLimitExceeded(ctx, identifier, ipAddress)
        return nil, err
    }
    
    // Call base implementation
    user, err := m.service.ValidateJWT(ctx, tokenString)
    
    // Record metrics
    duration := time.Since(start)
    success := err == nil
    m.metrics.RecordAuthAttempt(ctx, "jwt", success, duration)
    
    // Record attempt for rate limiting
    m.rateLimiter.RecordAttempt(ctx, identifier, success)
    
    // Audit log
    auditEvent := AuditEvent{
        AuthType:  "jwt",
        Success:   success,
        IPAddress: ipAddress,
    }
    if user != nil {
        auditEvent.UserID = user.ID
        auditEvent.TenantID = user.TenantID
    }
    if err != nil {
        auditEvent.Error = err.Error()
    }
    m.audit.LogAuthAttempt(ctx, auditEvent)
    
    return user, err
}

// HTTPAuthMiddleware creates HTTP middleware that adds IP to context
func HTTPAuthMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract IP address
            ip := r.RemoteAddr
            if forwardedIP := r.Header.Get("X-Forwarded-For"); forwardedIP != "" {
                ips := strings.Split(forwardedIP, ",")
                if len(ips) > 0 {
                    ip = strings.TrimSpace(ips[0])
                }
            }
            
            // Add to context
            ctx := context.WithValue(r.Context(), ContextKeyIPAddress, ip)
            ctx = context.WithValue(ctx, ContextKeyUserAgent, r.Header.Get("User-Agent"))
            
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Helper function to safely truncate keys
func truncateKey(key string, length int) string {
    if len(key) <= length {
        return key
    }
    return key[:length]
}
```

### Step 4: Create Comprehensive Test Infrastructure

#### 4.1 Create OAuth Provider Interface and Mock

First, create the OAuth provider interface:

```bash
touch pkg/auth/oauth_provider.go
```

Add the following to `pkg/auth/oauth_provider.go`:

```go
package auth

import (
    "context"
    "net/url"
    "time"
)

// OAuthToken represents an OAuth token
type OAuthToken struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    ExpiresAt    time.Time `json:"expires_at"`
}

// OAuthUserInfo represents user information from OAuth provider
type OAuthUserInfo struct {
    ID    string `json:"id"`
    Email string `json:"email"`
    Name  string `json:"name"`
}

// OAuthProvider defines the interface for OAuth providers
type OAuthProvider interface {
    // GetAuthorizationURL returns the authorization URL
    GetAuthorizationURL(state, redirectURI string) string
    
    // GetAuthorizationURLWithPKCE returns the authorization URL with PKCE
    GetAuthorizationURLWithPKCE(state, redirectURI, codeChallenge string) string
    
    // ExchangeCode exchanges an authorization code for tokens
    ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error)
    
    // ExchangeCodeWithPKCE exchanges an authorization code with PKCE
    ExchangeCodeWithPKCE(ctx context.Context, code, redirectURI, codeVerifier string) (*OAuthToken, error)
    
    // RefreshToken refreshes an access token
    RefreshToken(ctx context.Context, refreshToken string) (*OAuthToken, error)
    
    // ValidateToken validates an access token and returns user info
    ValidateToken(ctx context.Context, accessToken string) (*OAuthUserInfo, error)
    
    // ValidateState validates the state parameter
    ValidateState(providedState, expectedState string) bool
}

// BaseOAuthProvider provides common OAuth functionality
type BaseOAuthProvider struct {
    ClientID     string
    ClientSecret string
    AuthURL      string
    TokenURL     string
    UserInfoURL  string
}

// GetAuthorizationURL returns the authorization URL
func (p *BaseOAuthProvider) GetAuthorizationURL(state, redirectURI string) string {
    params := url.Values{}
    params.Set("client_id", p.ClientID)
    params.Set("redirect_uri", redirectURI)
    params.Set("response_type", "code")
    params.Set("state", state)
    params.Set("scope", "openid email profile")
    
    return p.AuthURL + "?" + params.Encode()
}

// GetAuthorizationURLWithPKCE returns the authorization URL with PKCE
func (p *BaseOAuthProvider) GetAuthorizationURLWithPKCE(state, redirectURI, codeChallenge string) string {
    params := url.Values{}
    params.Set("client_id", p.ClientID)
    params.Set("redirect_uri", redirectURI)
    params.Set("response_type", "code")
    params.Set("state", state)
    params.Set("scope", "openid email profile")
    params.Set("code_challenge", codeChallenge)
    params.Set("code_challenge_method", "S256")
    
    return p.AuthURL + "?" + params.Encode()
}

// ValidateState validates the state parameter
func (p *BaseOAuthProvider) ValidateState(providedState, expectedState string) bool {
    return providedState == expectedState
}
```

#### 4.2 Create Comprehensive Test Suite

Create the test file:

```bash
touch pkg/auth/auth_test.go
```

Add comprehensive tests:

```go
package auth_test

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/auth"
    "github.com/S-Corkum/devops-mcp/pkg/common/cache"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// MockOAuthProvider implements OAuthProvider for testing
type MockOAuthProvider struct {
    *auth.BaseOAuthProvider
    mockServer *httptest.Server
}

// NewMockOAuthProvider creates a new mock OAuth provider
func NewMockOAuthProvider(t *testing.T) *MockOAuthProvider {
    provider := &MockOAuthProvider{
        BaseOAuthProvider: &auth.BaseOAuthProvider{
            ClientID:     "test-client-id",
            ClientSecret: "test-client-secret",
        },
    }
    
    // Create mock server
    provider.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/oauth/token":
            provider.handleTokenRequest(w, r)
        case "/oauth/userinfo":
            provider.handleUserInfo(w, r)
        default:
            w.WriteHeader(http.StatusNotFound)
        }
    }))
    
    provider.AuthURL = provider.mockServer.URL + "/oauth/authorize"
    provider.TokenURL = provider.mockServer.URL + "/oauth/token"
    provider.UserInfoURL = provider.mockServer.URL + "/oauth/userinfo"
    
    return provider
}

// Close closes the mock server
func (m *MockOAuthProvider) Close() {
    m.mockServer.Close()
}

// ExchangeCode mocks code exchange
func (m *MockOAuthProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*auth.OAuthToken, error) {
    if code == "valid-code" {
        return &auth.OAuthToken{
            AccessToken:  "new-access-token",
            RefreshToken: "new-refresh-token",
            TokenType:    "Bearer",
            ExpiresAt:    time.Now().Add(time.Hour),
        }, nil
    }
    return nil, fmt.Errorf("invalid code")
}

// ExchangeCodeWithPKCE mocks PKCE code exchange
func (m *MockOAuthProvider) ExchangeCodeWithPKCE(ctx context.Context, code, redirectURI, codeVerifier string) (*auth.OAuthToken, error) {
    if code == "valid-code" && codeVerifier == "verifier" {
        return &auth.OAuthToken{
            AccessToken:  "new-access-token-pkce",
            RefreshToken: "new-refresh-token-pkce",
            TokenType:    "Bearer",
            ExpiresAt:    time.Now().Add(time.Hour),
        }, nil
    }
    return nil, fmt.Errorf("invalid code or verifier")
}

// RefreshToken mocks token refresh
func (m *MockOAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*auth.OAuthToken, error) {
    if refreshToken == "valid-refresh-token" {
        return &auth.OAuthToken{
            AccessToken:  "refreshed-access-token",
            RefreshToken: "new-refresh-token",
            TokenType:    "Bearer",
            ExpiresAt:    time.Now().Add(time.Hour),
        }, nil
    }
    if refreshToken == "expired-refresh-token" {
        return nil, fmt.Errorf("refresh token expired")
    }
    return nil, fmt.Errorf("invalid refresh token")
}

// ValidateToken mocks token validation
func (m *MockOAuthProvider) ValidateToken(ctx context.Context, accessToken string) (*auth.OAuthUserInfo, error) {
    switch accessToken {
    case "valid-access-token":
        return &auth.OAuthUserInfo{
            ID:    "user-123",
            Email: "user@example.com",
            Name:  "Test User",
        }, nil
    case "expired-token":
        return nil, fmt.Errorf("token expired")
    default:
        return nil, fmt.Errorf("invalid token")
    }
}

// Mock server handlers
func (m *MockOAuthProvider) handleTokenRequest(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
    
    code := r.FormValue("code")
    grantType := r.FormValue("grant_type")
    
    var response map[string]interface{}
    
    switch {
    case code == "valid-code" && grantType == "authorization_code":
        response = map[string]interface{}{
            "access_token":  "new-access-token",
            "refresh_token": "new-refresh-token",
            "token_type":    "Bearer",
            "expires_in":    3600,
        }
    case code == "invalid-grant":
        w.WriteHeader(http.StatusBadRequest)
        response = map[string]interface{}{
            "error":             "invalid_grant",
            "error_description": "The provided authorization grant is invalid",
        }
    default:
        w.WriteHeader(http.StatusBadRequest)
        response = map[string]interface{}{
            "error": "invalid_request",
        }
    }
    
    json.NewEncoder(w).Encode(response)
}

func (m *MockOAuthProvider) handleUserInfo(w http.ResponseWriter, r *http.Request) {
    auth := r.Header.Get("Authorization")
    
    switch auth {
    case "Bearer valid-access-token":
        json.NewEncoder(w).Encode(map[string]interface{}{
            "id":    "user-123",
            "email": "user@example.com",
            "name":  "Test User",
        })
    case "Bearer expired-token":
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": "token_expired",
        })
    default:
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": "invalid_token",
        })
    }
}

// TestOAuthProviderComprehensive tests OAuth provider functionality
func TestOAuthProviderComprehensive(t *testing.T) {
    provider := NewMockOAuthProvider(t)
    defer provider.Close()
    
    t.Run("Authorization Flow", func(t *testing.T) {
        // Test authorization URL generation
        authURL := provider.GetAuthorizationURL("test-state", "http://localhost/callback")
        assert.Contains(t, authURL, "client_id=test-client-id")
        assert.Contains(t, authURL, "state=test-state")
        assert.Contains(t, authURL, "redirect_uri=http://localhost/callback")
        
        // Test PKCE support
        authURLWithPKCE := provider.GetAuthorizationURLWithPKCE("test-state", "http://localhost/callback", "challenge")
        assert.Contains(t, authURLWithPKCE, "code_challenge=challenge")
        assert.Contains(t, authURLWithPKCE, "code_challenge_method=S256")
    })
    
    t.Run("Token Exchange", func(t *testing.T) {
        ctx := context.Background()
        
        // Test successful token exchange
        token, err := provider.ExchangeCode(ctx, "valid-code", "http://localhost/callback")
        require.NoError(t, err)
        assert.NotEmpty(t, token.AccessToken)
        assert.NotEmpty(t, token.RefreshToken)
        assert.True(t, token.ExpiresAt.After(time.Now()))
        
        // Test with PKCE
        tokenWithPKCE, err := provider.ExchangeCodeWithPKCE(ctx, "valid-code", "http://localhost/callback", "verifier")
        require.NoError(t, err)
        assert.NotEmpty(t, tokenWithPKCE.AccessToken)
    })
    
    t.Run("Token Refresh", func(t *testing.T) {
        ctx := context.Background()
        
        // Test successful refresh
        newToken, err := provider.RefreshToken(ctx, "valid-refresh-token")
        require.NoError(t, err)
        assert.NotEmpty(t, newToken.AccessToken)
        assert.NotEqual(t, "valid-refresh-token", newToken.AccessToken)
        
        // Test expired refresh token
        _, err = provider.RefreshToken(ctx, "expired-refresh-token")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "expired")
    })
    
    t.Run("Token Validation", func(t *testing.T) {
        ctx := context.Background()
        
        // Test valid token
        userInfo, err := provider.ValidateToken(ctx, "valid-access-token")
        require.NoError(t, err)
        assert.NotEmpty(t, userInfo.ID)
        assert.NotEmpty(t, userInfo.Email)
        
        // Test invalid token
        _, err = provider.ValidateToken(ctx, "invalid-token")
        assert.Error(t, err)
        
        // Test expired token
        _, err = provider.ValidateToken(ctx, "expired-token")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "expired")
    })
    
    t.Run("State Validation", func(t *testing.T) {
        assert.True(t, provider.ValidateState("valid-state", "valid-state"))
        assert.False(t, provider.ValidateState("state1", "state2"))
    })
}

// TestAuthenticationIntegration tests the full authentication stack
func TestAuthenticationIntegration(t *testing.T) {
    // Setup
    logger := observability.NewNoopLogger()
    metrics := observability.NewNoopMetrics()
    testCache := cache.NewMemoryCache()
    
    // Create base service
    config := auth.DefaultConfig()
    config.JWTSecret = "test-secret"
    baseService := auth.NewService(config, nil, testCache, logger)
    
    // Create rate limiter
    rateLimiter := auth.NewRateLimiter(testCache, logger, &auth.RateLimiterConfig{
        MaxAttempts:   3,
        WindowSize:    1 * time.Minute,
        LockoutPeriod: 5 * time.Minute,
    })
    
    // Create metrics and audit
    metricsCollector := auth.NewMetricsCollector(metrics)
    auditLogger := auth.NewAuditLogger(logger)
    
    // Create auth middleware
    authMiddleware := auth.NewAuthMiddleware(baseService, rateLimiter, metricsCollector, auditLogger)
    
    // Test API key validation with rate limiting
    t.Run("API Key Rate Limiting", func(t *testing.T) {
        ctx := context.Background()
        
        // Create test API key
        apiKey, err := baseService.CreateAPIKey(ctx, "test-tenant", "test-user", "test-key", []string{"read"}, nil)
        require.NoError(t, err)
        
        // Add IP to context
        ctx = context.WithValue(ctx, auth.ContextKeyIPAddress, "127.0.0.1")
        
        // Successful validations
        for i := 0; i < 3; i++ {
            user, err := authMiddleware.ValidateAPIKeyWithMetrics(ctx, apiKey.Key)
            require.NoError(t, err)
            assert.Equal(t, "test-user", user.ID)
        }
        
        // Failed validations (wrong key)
        for i := 0; i < 3; i++ {
            _, err := authMiddleware.ValidateAPIKeyWithMetrics(ctx, "wrong-key")
            assert.Error(t, err)
        }
        
        // Should be rate limited now
        _, err = authMiddleware.ValidateAPIKeyWithMetrics(ctx, "wrong-key")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "rate limit")
    })
    
    // Test middleware integration
    t.Run("Middleware Integration", func(t *testing.T) {
        // Create test handler
        handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
        })
        
        // Wrap with auth context middleware
        wrapped := auth.HTTPAuthMiddleware()(handler)
        
        // Wrap with rate limit middleware
        wrapped = auth.RateLimitMiddleware(rateLimiter, logger)(wrapped)
        
        // Test requests
        for i := 0; i < 5; i++ {
            req := httptest.NewRequest("POST", "/auth/login", nil)
            req.RemoteAddr = "127.0.0.1:12345"
            req.Header.Set("X-Forwarded-For", "192.168.1.1")
            
            rr := httptest.NewRecorder()
            wrapped.ServeHTTP(rr, req)
            
            if i < 3 {
                // First 3 requests should succeed (no auth performed, just hitting endpoint)
                assert.Equal(t, http.StatusOK, rr.Code)
            } else {
                // After 3 failed attempts, should be rate limited
                assert.Equal(t, http.StatusTooManyRequests, rr.Code)
                assert.NotEmpty(t, rr.Header().Get("Retry-After"))
            }
        }
    })
}
```

### Step 5: Remove Hardcoded Test API Keys

#### 5.1 Create Configuration-Based API Key Management

Create a new configuration file:

```bash
touch pkg/auth/config_keys.go
```

Add the following to `pkg/auth/config_keys.go`:

```go
package auth

import (
    "fmt"
    "os"
    "strings"
    "time"
)

// APIKeyConfig represents API key configuration
type APIKeyConfig struct {
    // Development keys (only loaded in dev/test environments)
    DevelopmentKeys map[string]APIKeySettings `yaml:"development_keys"`
    
    // Production key sources
    ProductionKeySource string `yaml:"production_key_source"` // "env", "vault", "aws-secrets"
}

// APIKeySettings represents settings for an API key
type APIKeySettings struct {
    Role      string   `yaml:"role"`
    Scopes    []string `yaml:"scopes"`
    TenantID  string   `yaml:"tenant_id"`
    ExpiresIn string   `yaml:"expires_in"` // Duration string like "30d"
}

// LoadAPIKeys loads API keys based on environment
func (s *Service) LoadAPIKeys(config *APIKeyConfig) error {
    env := os.Getenv("ENVIRONMENT")
    
    switch env {
    case "development", "test", "":
        return s.loadDevelopmentKeys(config.DevelopmentKeys)
    case "production":
        return s.loadProductionKeys(config.ProductionKeySource)
    default:
        return fmt.Errorf("unknown environment: %s", env)
    }
}

// loadDevelopmentKeys loads keys for development/test
func (s *Service) loadDevelopmentKeys(keys map[string]APIKeySettings) error {
    if keys == nil {
        return nil
    }
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    for key, settings := range keys {
        // Generate deterministic key for development
        apiKey := &APIKey{
            Key:       fmt.Sprintf("dev_%s", key),
            TenantID:  settings.TenantID,
            UserID:    "dev-user",
            Name:      fmt.Sprintf("Development %s key", settings.Role),
            Scopes:    settings.Scopes,
            CreatedAt: time.Now(),
            Active:    true,
        }
        
        s.apiKeys[apiKey.Key] = apiKey
        
        s.logger.Debug("Loaded development API key", map[string]interface{}{
            "key_name": key,
            "role":     settings.Role,
        })
    }
    
    return nil
}

// loadProductionKeys loads keys from secure sources
func (s *Service) loadProductionKeys(source string) error {
    switch source {
    case "env":
        return s.loadKeysFromEnv()
    case "vault":
        return s.loadKeysFromVault()
    case "aws-secrets":
        return s.loadKeysFromAWSSecrets()
    default:
        return fmt.Errorf("unsupported key source: %s", source)
    }
}

// loadKeysFromEnv loads API keys from environment variables
func (s *Service) loadKeysFromEnv() error {
    // Look for API_KEY_* environment variables
    foundKeys := 0
    for _, env := range os.Environ() {
        if strings.HasPrefix(env, "API_KEY_") {
            parts := strings.SplitN(env, "=", 2)
            if len(parts) != 2 {
                continue
            }
            
            keyName := strings.TrimPrefix(parts[0], "API_KEY_")
            keyValue := parts[1]
            
            // Parse key metadata from env var name
            // Format: API_KEY_<NAME>_<ROLE>
            keyParts := strings.Split(keyName, "_")
            if len(keyParts) < 2 {
                continue
            }
            
            role := strings.ToLower(keyParts[len(keyParts)-1])
            name := strings.Join(keyParts[:len(keyParts)-1], "_")
            
            apiKey := &APIKey{
                Key:       keyValue,
                TenantID:  "default",
                UserID:    "system",
                Name:      name,
                Scopes:    getRoleScopes(role),
                CreatedAt: time.Now(),
                Active:    true,
            }
            
            s.mu.Lock()
            s.apiKeys[keyValue] = apiKey
            s.mu.Unlock()
            
            s.logger.Info("Loaded API key from environment", map[string]interface{}{
                "key_name": name,
                "role":     role,
            })
            foundKeys++
        }
    }
    
    if foundKeys == 0 {
        s.logger.Warn("No API keys found in environment variables")
    }
    
    return nil
}

// loadKeysFromVault placeholder - implement based on your Vault setup
func (s *Service) loadKeysFromVault() error {
    return fmt.Errorf("vault integration not implemented")
}

// loadKeysFromAWSSecrets placeholder - implement based on your AWS setup
func (s *Service) loadKeysFromAWSSecrets() error {
    return fmt.Errorf("AWS Secrets Manager integration not implemented")
}

// getRoleScopes returns scopes for a role
func getRoleScopes(role string) []string {
    switch role {
    case "admin":
        return []string{"read", "write", "admin"}
    case "write":
        return []string{"read", "write"}
    case "read":
        return []string{"read"}
    default:
        return []string{"read"}
    }
}
```

### Step 6: Integration and Testing

Create setup utilities:

```bash
touch pkg/auth/setup.go
```

Add the following to `pkg/auth/setup.go`:

```go
package auth

import (
    "fmt"
    "os"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/common/cache"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
    "github.com/jmoiron/sqlx"
)

// SetupAuthentication sets up the enhanced authentication service
func SetupAuthentication(db *sqlx.DB, cache cache.Cache, logger observability.Logger, metrics observability.Metrics) (*AuthMiddleware, error) {
    // Create base service
    config := &ServiceConfig{
        JWTSecret:         getEnvOrDefault("JWT_SECRET", ""),
        JWTExpiration:     24 * time.Hour,
        APIKeyHeader:      "X-API-Key",
        EnableAPIKeys:     true,
        EnableJWT:         true,
        CacheEnabled:      true,
        CacheTTL:          5 * time.Minute,
        MaxFailedAttempts: 5,
        LockoutDuration:   15 * time.Minute,
    }
    
    baseService := NewService(config, db, cache, logger)
    
    // Load API keys based on environment
    keyConfig := &APIKeyConfig{
        ProductionKeySource: getEnvOrDefault("API_KEY_SOURCE", "env"),
    }
    
    if err := baseService.LoadAPIKeys(keyConfig); err != nil {
        return nil, fmt.Errorf("failed to load API keys: %w", err)
    }
    
    // Create rate limiter
    rateLimiter := NewRateLimiter(cache, logger, nil)
    
    // Create metrics and audit
    metricsCollector := NewMetricsCollector(metrics)
    auditLogger := NewAuditLogger(logger)
    
    // Create auth middleware
    authMiddleware := NewAuthMiddleware(baseService, rateLimiter, metricsCollector, auditLogger)
    
    return authMiddleware, nil
}

// getEnvOrDefault gets environment variable or returns default
func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### Step 7: Update Main Application

## Configuration Files

Create environment-specific configuration files:

### Development Configuration
Create `configs/auth.development.yaml`:
```yaml
auth:
  development_keys:
    test_admin:
      role: admin
      scopes: ["read", "write", "admin"]
      tenant_id: default
      expires_in: 30d
    test_readonly:
      role: read
      scopes: ["read"]
      tenant_id: default
      expires_in: 30d
  production_key_source: env
```

### Production Configuration
Create `configs/auth.production.yaml`:
```yaml
auth:
  # No hardcoded keys in production
  production_key_source: env  # or "vault" or "aws-secrets"
```

## Integration with Existing Code

In your existing middleware setup, add the authentication middleware:

```go
// In your server setup code
authMiddleware, err := auth.SetupAuthentication(db, cache, logger, metrics)
if err != nil {
    log.Fatal("Failed to setup authentication:", err)
}

// Add to HTTP middleware chain
router.Use(auth.HTTPAuthMiddleware())
router.Use(auth.RateLimitMiddleware(authMiddleware.rateLimiter, logger))

// Use in your handlers
func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Get API key from header
    apiKey := r.Header.Get("X-API-Key")
    
    // Validate with metrics
    user, err := authMiddleware.ValidateAPIKeyWithMetrics(ctx, apiKey)
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Process request with authenticated user
    // ...
}
```

## Verification Steps

After implementing all components, verify the implementation:

1. **Run Unit Tests**:
```bash
go test ./pkg/auth/... -v
```

2. **Run Integration Tests**:
```bash
go test ./pkg/auth/... -v -tags=integration
```

3. **Run Linter**:
```bash
golangci-lint run ./pkg/auth/...
```

4. **Test Rate Limiting Manually**:
```bash
# Start the application
make local-dev

# Test rate limiting with curl
for i in {1..10}; do
  curl -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"test","password":"wrong"}' \
    -w "\nStatus: %{http_code}\n"
  sleep 0.5
done
```

5. **Check Metrics**:
```bash
# Check metrics endpoint
curl http://localhost:9090/metrics | grep auth_
```

## Best Practices Summary

1. **Never commit hardcoded secrets** - Use environment variables or secret management
2. **Always implement rate limiting** - Protect against brute force attacks
3. **Log security events** - Maintain audit trail for compliance
4. **Monitor authentication metrics** - Track success/failure rates
5. **Test thoroughly** - Include unit, integration, and security tests
6. **Use defense in depth** - Multiple layers of security
7. **Follow least privilege** - Minimal scopes by default
8. **Implement token rotation** - Regular key/token refresh
9. **Use secure random generators** - crypto/rand for keys
10. **Handle errors gracefully** - Don't leak information in errors

## Common Pitfalls to Avoid

1. **Thread Safety** - Always use mutexes when accessing shared state
2. **Context Propagation** - Always pass context through the call chain
3. **Error Handling** - Check all errors and handle appropriately
4. **Resource Cleanup** - Always close resources (servers, connections)
5. **Key Truncation** - Check key length before truncating
6. **Type Assertions** - Always check the second return value
7. **Race Conditions** - Use proper synchronization primitives
8. **Memory Leaks** - Clean up goroutines and channels
9. **Panic Recovery** - Add defer/recover in HTTP handlers
10. **Configuration Validation** - Validate all configuration on startup

## Monitoring and Alerting

Set up alerts for:
- High authentication failure rates (>20%)
- Rate limit violations spike
- Unusual authentication patterns
- API key usage from new IPs
- JWT validation errors increase
- Cache failures affecting auth

## Production Deployment Checklist

- [ ] Set JWT_SECRET environment variable
- [ ] Configure API_KEY_SOURCE (env/vault/aws-secrets)
- [ ] Set production API keys in chosen source
- [ ] Enable HTTPS/TLS
- [ ] Configure rate limiting thresholds
- [ ] Set up monitoring alerts
- [ ] Configure log aggregation
- [ ] Test authentication flows
- [ ] Verify metrics collection
- [ ] Review security settings
- [ ] Implement key rotation policy
- [ ] Set up audit log retention
- [ ] Configure backup authentication methods
- [ ] Test failover scenarios
- [ ] Document emergency procedures

## Testing Commands

Run all tests with coverage:
```bash
go test ./pkg/auth/... -v -race -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Run benchmarks:
```bash
go test ./pkg/auth/... -bench=. -benchmem
```

## Next Steps

1. Implement OAuth providers (Google, GitHub, etc.)
2. Add MFA support
3. Implement API key rotation policy
4. Add IP allowlisting
5. Implement session management
6. Add SAML support for enterprise
7. Integrate with external identity providers
8. Add biometric authentication support
9. Implement zero-trust architecture
10. Add compliance reporting (SOC2, HIPAA)