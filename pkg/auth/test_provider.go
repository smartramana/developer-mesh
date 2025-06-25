package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// TestProvider implements the Authorizer interface for test environments
// It provides JWT-based authentication with rate limiting and audit logging
type TestProvider struct {
	logger      observability.Logger
	tracer      observability.StartSpanFunc
	jwtSecret   []byte
	rateLimiter *rate.Limiter
	auditLog    *testAuditLogger
	enabled     bool
	mu          sync.RWMutex

	// Track issued tokens for revocation
	issuedTokens map[string]tokenInfo
}

type tokenInfo struct {
	userID    uuid.UUID
	tenantID  uuid.UUID
	role      string
	scopes    []string
	expiresAt time.Time
	revoked   bool
}

type testAuditLogger struct {
	logger observability.Logger
	mu     sync.Mutex
}

// NewTestProvider creates a new test authentication provider
func NewTestProvider(logger observability.Logger, tracer observability.StartSpanFunc) (*TestProvider, error) {
	// Ensure we're in test mode
	if os.Getenv("MCP_TEST_MODE") != "true" {
		return nil, errors.New("test provider can only be used in test mode")
	}

	// Ensure test auth is explicitly enabled
	if os.Getenv("TEST_AUTH_ENABLED") != "true" {
		return nil, errors.New("test auth must be explicitly enabled")
	}

	// Generate a secure test JWT secret
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, errors.Wrap(err, "failed to generate JWT secret")
	}

	provider := &TestProvider{
		logger:       logger,
		tracer:       tracer,
		jwtSecret:    secret,
		rateLimiter:  rate.NewLimiter(rate.Limit(1000.0/60.0), 100), // 1000 requests/minute with burst of 100
		auditLog:     &testAuditLogger{logger: logger},
		enabled:      true,
		issuedTokens: make(map[string]tokenInfo),
	}

	logger.Info("Test authentication provider initialized", map[string]interface{}{
		"rate_limit": "1000/minute",
		"jwt_expiry": "1h",
		"mode":       "test",
	})

	return provider, nil
}

// Authorize implements the Authorizer interface
func (tp *TestProvider) Authorize(ctx context.Context, permission Permission) Decision {
	if tp.tracer != nil {
		_, span := tp.tracer(ctx, "TestProvider.Authorize")
		defer span.End()
	}

	// Rate limiting - use Allow() for immediate non-blocking check
	if !tp.rateLimiter.Allow() {
		tp.auditLog.logFailure(permission.Resource, permission.Action, "rate_limit_exceeded")
		return Decision{Allowed: false, Reason: "rate limit exceeded"}
	}

	// Check if provider is enabled
	tp.mu.RLock()
	if !tp.enabled {
		tp.mu.RUnlock()
		tp.auditLog.logFailure(permission.Resource, permission.Action, "provider_disabled")
		return Decision{Allowed: false, Reason: "test provider is disabled"}
	}
	tp.mu.RUnlock()

	// In test mode, we allow all actions but still log them
	tp.auditLog.logSuccess(permission.Resource, permission.Action)

	return Decision{Allowed: true, Reason: "test mode allows all"}
}

// CheckPermission implements the Authorizer interface
func (tp *TestProvider) CheckPermission(ctx context.Context, resource, action string) bool {
	if tp.tracer != nil {
		_, span := tp.tracer(ctx, "TestProvider.CheckPermission")
		defer span.End()
	}

	// Rate limiting - use Allow() for immediate non-blocking check
	if !tp.rateLimiter.Allow() {
		tp.auditLog.logFailure(resource, action, "rate_limit_exceeded")
		return false
	}

	// Check if provider is enabled
	tp.mu.RLock()
	if !tp.enabled {
		tp.mu.RUnlock()
		tp.auditLog.logFailure(resource, action, "provider_disabled")
		return false
	}
	tp.mu.RUnlock()

	// In test mode, we allow all permissions but still log them
	tp.auditLog.logSuccess(resource, action)

	return true
}

// Additional helper methods for test scenarios

// GetUserRole returns the role for a user (test helper)
func (tp *TestProvider) GetUserRole(ctx context.Context, userID, tenantID uuid.UUID) (string, error) {
	if tp.tracer != nil {
		_, span := tp.tracer(ctx, "TestProvider.GetUserRole")
		defer span.End()
	}

	// In test mode, return a default role
	return "test_user", nil
}

// ListUserPermissions returns permissions for a user (test helper)
func (tp *TestProvider) ListUserPermissions(ctx context.Context, userID, tenantID uuid.UUID) ([]string, error) {
	if tp.tracer != nil {
		_, span := tp.tracer(ctx, "TestProvider.ListUserPermissions")
		defer span.End()
	}

	// In test mode, return common test permissions
	return []string{
		"read:*",
		"write:*",
		"test:*",
	}, nil
}

// Close closes the test provider
func (tp *TestProvider) Close() error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	tp.enabled = false
	tp.logger.Info("Test provider closed", nil)

	return nil
}

// GenerateTestToken generates a JWT token for testing
func (tp *TestProvider) GenerateTestToken(userID, tenantID uuid.UUID, role string, scopes []string) (string, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if !tp.enabled {
		return "", errors.New("test provider is disabled")
	}

	// Create token ID
	tokenID := uuid.New().String()

	// Create claims
	claims := jwt.MapClaims{
		"jti":       tokenID,
		"sub":       userID.String(),
		"tenant_id": tenantID.String(),
		"role":      role,
		"scopes":    scopes,
		"iat":       time.Now().Unix(),
		"exp":       time.Now().Add(time.Hour).Unix(), // 1 hour expiry
		"test_mode": true,
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(tp.jwtSecret)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign token")
	}

	// Track issued token
	tp.issuedTokens[tokenID] = tokenInfo{
		userID:    userID,
		tenantID:  tenantID,
		role:      role,
		scopes:    scopes,
		expiresAt: time.Now().Add(time.Hour),
		revoked:   false,
	}

	tp.logger.Info("Test token generated", map[string]interface{}{
		"token_id":  tokenID,
		"user_id":   userID,
		"tenant_id": tenantID,
		"role":      role,
		"expires":   time.Hour,
	})

	return tokenString, nil
}

// ValidateTestToken validates a test JWT token
func (tp *TestProvider) ValidateTestToken(tokenString string) (*Claims, error) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	if !tp.enabled {
		return nil, errors.New("test provider is disabled")
	}

	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tp.jwtSecret, nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "invalid token")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Check if it's a test token
	if testMode, ok := claims["test_mode"].(bool); !ok || !testMode {
		return nil, errors.New("not a test token")
	}

	// Check if token is revoked
	tokenID, _ := claims["jti"].(string)
	if info, exists := tp.issuedTokens[tokenID]; exists && info.revoked {
		return nil, errors.New("token has been revoked")
	}

	// Parse UUIDs
	userID, err := uuid.Parse(claims["sub"].(string))
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	tenantID, err := uuid.Parse(claims["tenant_id"].(string))
	if err != nil {
		return nil, errors.New("invalid tenant ID")
	}

	// Extract scopes
	var scopes []string
	if scopeList, ok := claims["scopes"].([]interface{}); ok {
		for _, s := range scopeList {
			if scope, ok := s.(string); ok {
				scopes = append(scopes, scope)
			}
		}
	}

	return &Claims{
		UserID:   userID.String(),
		TenantID: tenantID.String(),
		Scopes:   scopes,
	}, nil
}

// RevokeToken revokes a test token
func (tp *TestProvider) RevokeToken(tokenID string) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if info, exists := tp.issuedTokens[tokenID]; exists {
		info.revoked = true
		tp.issuedTokens[tokenID] = info
		tp.logger.Info("Test token revoked", map[string]interface{}{
			"token_id": tokenID,
		})
	}

	return nil
}

// CleanupExpiredTokens removes expired tokens from tracking
func (tp *TestProvider) CleanupExpiredTokens() {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	now := time.Now()
	for tokenID, info := range tp.issuedTokens {
		if now.After(info.expiresAt) {
			delete(tp.issuedTokens, tokenID)
		}
	}
}

// Audit logging methods
func (al *testAuditLogger) logSuccess(resource, action string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.logger.Info("Test auth audit", map[string]interface{}{
		"event":     "auth_success",
		"resource":  resource,
		"action":    action,
		"timestamp": time.Now().UTC(),
	})
}

func (al *testAuditLogger) logFailure(resource, action, reason string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.logger.Warn("Test auth audit", map[string]interface{}{
		"event":     "auth_failure",
		"resource":  resource,
		"action":    action,
		"reason":    reason,
		"timestamp": time.Now().UTC(),
	})
}

// GenerateTestAPIKey generates a secure test API key
func GenerateTestAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a default if random generation fails
		return "test-default-api-key"
	}
	return "test-" + hex.EncodeToString(b)
}
