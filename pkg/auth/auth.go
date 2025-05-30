// Package auth provides centralized authentication and authorization for the DevOps MCP platform
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"database/sql"
	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// Common errors
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrNoAPIKey           = errors.New("no API key provided")
	ErrInvalidAPIKey      = errors.New("invalid API key")
	ErrInsufficientScope  = errors.New("insufficient scope")
)

// Type represents the type of authentication
type Type string

const (
	TypeAPIKey Type = "api_key"
	TypeJWT    Type = "jwt"
	TypeNone   Type = "none"
)

// Claims represents JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserID   string   `json:"user_id"`
	TenantID string   `json:"tenant_id"`
	Scopes   []string `json:"scopes,omitempty"`
	Email    string   `json:"email,omitempty"`
}

// APIKey represents an API key
type APIKey struct {
	Key       string     `db:"key"`
	TenantID  string     `db:"tenant_id"`
	UserID    string     `db:"user_id"`
	Name      string     `db:"name"`
	Scopes    []string   `db:"scopes"`
	ExpiresAt *time.Time `db:"expires_at"`
	CreatedAt time.Time  `db:"created_at"`
	LastUsed  *time.Time `db:"last_used"`
	Active    bool       `db:"active"`
}

// User represents an authenticated user
type User struct {
	ID       string   `json:"id"`
	TenantID string   `json:"tenant_id"`
	Email    string   `json:"email,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
	AuthType Type     `json:"auth_type"`
}

// ServiceConfig represents auth configuration
type ServiceConfig struct {
	JWTSecret         string
	JWTExpiration     time.Duration
	APIKeyHeader      string
	EnableAPIKeys     bool
	EnableJWT         bool
	CacheEnabled      bool
	CacheTTL          time.Duration
	MaxFailedAttempts int
	LockoutDuration   time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() *ServiceConfig {
	return &ServiceConfig{
		JWTExpiration:     24 * time.Hour,
		APIKeyHeader:      "X-API-Key",
		EnableAPIKeys:     true,
		EnableJWT:         true,
		CacheEnabled:      true,
		CacheTTL:          5 * time.Minute,
		MaxFailedAttempts: 5,
		LockoutDuration:   15 * time.Minute,
	}
}

// Service provides authentication services
type Service struct {
	config *ServiceConfig
	db     *sqlx.DB
	cache  cache.Cache
	logger observability.Logger

	// In-memory storage for development/testing
	apiKeys map[string]*APIKey
	mu      sync.RWMutex
}

// NewService creates a new auth service
func NewService(config *ServiceConfig, db *sqlx.DB, cache cache.Cache, logger observability.Logger) *Service {
	if config == nil {
		config = DefaultConfig()
	}

	return &Service{
		config:  config,
		db:      db,
		cache:   cache,
		logger:  logger,
		apiKeys: make(map[string]*APIKey),
	}
}

// ValidateAPIKey validates an API key and returns the associated user
func (s *Service) ValidateAPIKey(ctx context.Context, apiKey string) (*User, error) {
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}

	// Check cache first if enabled
	if s.config.CacheEnabled && s.cache != nil {
		cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
		var cached string
		if err := s.cache.Get(ctx, cacheKey, &cached); err == nil && cached != "" {
			// Parse cached user data
			// In production, unmarshal JSON from cache
			// For now, return a simple user
			return &User{
				ID:       "cached-user",
				TenantID: "cached-tenant",
				AuthType: TypeAPIKey,
			}, nil
		}
	}

	// Check in-memory storage (for development)
	s.mu.RLock()
	key, exists := s.apiKeys[apiKey]
	s.mu.RUnlock()

	if exists && key.Active {
		// Check expiration
		if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
			return nil, ErrInvalidAPIKey
		}

		user := &User{
			ID:       key.UserID,
			TenantID: key.TenantID,
			Scopes:   key.Scopes,
			AuthType: TypeAPIKey,
		}

		// Update last used timestamp asynchronously
		go func() {
			now := time.Now()
			s.mu.Lock()
			if k, ok := s.apiKeys[apiKey]; ok {
				k.LastUsed = &now
			}
			s.mu.Unlock()
		}()

		// Cache the result
		if s.config.CacheEnabled && s.cache != nil {
			cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
			// In production, marshal user to JSON
			s.cache.Set(ctx, cacheKey, "cached", s.config.CacheTTL)
		}

		return user, nil
	}

	// Check database if available
	if s.db != nil {
		var dbKey APIKey
		query := `
			SELECT key, tenant_id, user_id, name, scopes, expires_at, created_at, last_used, active
			FROM api_keys
			WHERE key = $1 AND active = true
		`
		err := s.db.GetContext(ctx, &dbKey, query, apiKey)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrInvalidAPIKey
			}
			return nil, fmt.Errorf("database error: %w", err)
		}

		// Check expiration
		if dbKey.ExpiresAt != nil && time.Now().After(*dbKey.ExpiresAt) {
			return nil, ErrInvalidAPIKey
		}

		user := &User{
			ID:       dbKey.UserID,
			TenantID: dbKey.TenantID,
			Scopes:   dbKey.Scopes,
			AuthType: TypeAPIKey,
		}

		// Update last used timestamp
		updateQuery := `UPDATE api_keys SET last_used = $1 WHERE key = $2`
		s.db.ExecContext(ctx, updateQuery, time.Now(), apiKey)

		// Cache the result
		if s.config.CacheEnabled && s.cache != nil {
			cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
			s.cache.Set(ctx, cacheKey, "cached", s.config.CacheTTL)
		}

		return user, nil
	}

	return nil, ErrInvalidAPIKey
}

// ValidateJWT validates a JWT token and returns the associated user
func (s *Service) ValidateJWT(ctx context.Context, tokenString string) (*User, error) {
	if tokenString == "" || s.config.JWTSecret == "" {
		return nil, ErrInvalidToken
	}

	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	// Validate claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Check expiration (jwt library handles this, but we can add custom logic)
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrTokenExpired
	}

	// Create user from claims
	user := &User{
		ID:       claims.UserID,
		TenantID: claims.TenantID,
		Email:    claims.Email,
		Scopes:   claims.Scopes,
		AuthType: TypeJWT,
	}

	return user, nil
}

// GenerateJWT generates a new JWT token for a user
func (s *Service) GenerateJWT(ctx context.Context, user *User) (string, error) {
	if s.config.JWTSecret == "" {
		return "", errors.New("JWT secret not configured")
	}

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.JWTExpiration)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        generateID(), // You would implement this
		},
		UserID:   user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Scopes:   user.Scopes,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// CreateAPIKey creates a new API key
func (s *Service) CreateAPIKey(ctx context.Context, tenantID, userID, name string, scopes []string, expiresAt *time.Time) (*APIKey, error) {
	// Generate a secure random key
	keyStr := generateAPIKey() // You would implement this

	apiKey := &APIKey{
		Key:       keyStr,
		TenantID:  tenantID,
		UserID:    userID,
		Name:      name,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		Active:    true,
	}

	// Store in memory (for development)
	s.mu.Lock()
	s.apiKeys[keyStr] = apiKey
	s.mu.Unlock()

	// Store in database if available
	if s.db != nil {
		query := `
			INSERT INTO api_keys (key, tenant_id, user_id, name, scopes, expires_at, created_at, active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
		_, err := s.db.ExecContext(ctx, query,
			apiKey.Key, apiKey.TenantID, apiKey.UserID, apiKey.Name,
			apiKey.Scopes, apiKey.ExpiresAt, apiKey.CreatedAt, apiKey.Active,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create API key: %w", err)
		}
	}

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (s *Service) RevokeAPIKey(ctx context.Context, apiKey string) error {
	// Remove from memory
	s.mu.Lock()
	delete(s.apiKeys, apiKey)
	s.mu.Unlock()

	// Update in database if available
	if s.db != nil {
		query := `UPDATE api_keys SET active = false WHERE key = $1`
		_, err := s.db.ExecContext(ctx, query, apiKey)
		if err != nil {
			return fmt.Errorf("failed to revoke API key: %w", err)
		}
	}

	// Remove from cache
	if s.config.CacheEnabled && s.cache != nil {
		cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
		s.cache.Delete(ctx, cacheKey)
	}

	return nil
}

// AuthorizeScopes checks if a user has the required scopes
func (s *Service) AuthorizeScopes(user *User, requiredScopes []string) error {
	if len(requiredScopes) == 0 {
		return nil // No scopes required
	}

	userScopeMap := make(map[string]bool)
	for _, scope := range user.Scopes {
		userScopeMap[scope] = true
	}

	for _, required := range requiredScopes {
		if !userScopeMap[required] {
			return ErrInsufficientScope
		}
	}

	return nil
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares a password with its hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Helper functions (implement these based on your needs)
func generateID() string {
	// Implement UUID generation
	return fmt.Sprintf("id_%d", time.Now().UnixNano())
}

func generateAPIKey() string {
	// Implement secure random key generation
	// In production, use crypto/rand
	return fmt.Sprintf("mcp_%s_%d", strings.ReplaceAll(time.Now().Format("20060102"), "-", ""), time.Now().UnixNano())
}

// InitializeDefaultAPIKeys initializes default API keys from configuration
func (s *Service) InitializeDefaultAPIKeys(keys map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, role := range keys {
		// Parse role to determine scopes
		var scopes []string
		switch role {
		case "admin":
			scopes = []string{"read", "write", "admin"}
		case "write":
			scopes = []string{"read", "write"}
		case "read":
			scopes = []string{"read"}
		default:
			scopes = []string{"read"}
		}

		s.apiKeys[key] = &APIKey{
			Key:       key,
			TenantID:  "default",
			UserID:    "system",
			Name:      fmt.Sprintf("Default %s key", role),
			Scopes:    scopes,
			CreatedAt: time.Now(),
			Active:    true,
		}

		s.logger.Info("Initialized default API key", map[string]interface{}{
			"key_suffix": key[len(key)-4:], // Log only last 4 chars for security
			"role":       role,
			"scopes":     scopes,
		})
	}
}

// AddAPIKey adds an API key to the service at runtime (thread-safe)
func (s *Service) AddAPIKey(key string, settings APIKeySettings) error {
    // Validation
    if key == "" {
        return fmt.Errorf("API key cannot be empty")
    }
    if len(key) < 16 {
        return fmt.Errorf("API key too short (minimum 16 characters)")
    }
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Create API key object
    apiKey := &APIKey{
        Key:       key,
        TenantID:  settings.TenantID,
        UserID:    "system",
        Name:      fmt.Sprintf("%s API key", settings.Role),
        Scopes:    settings.Scopes,
        Active:    true,
        CreatedAt: time.Now(),
    }
    
    // Apply defaults
    if apiKey.TenantID == "" {
        apiKey.TenantID = "default"
    }
    if len(apiKey.Scopes) == 0 {
        apiKey.Scopes = []string{"read"} // Minimum scope
    }
    
    // Handle expiration
    if settings.ExpiresIn != "" {
        duration, err := time.ParseDuration(settings.ExpiresIn)
        if err != nil {
            return fmt.Errorf("invalid expiration duration %q: %w", settings.ExpiresIn, err)
        }
        if duration < 0 {
            return fmt.Errorf("expiration duration cannot be negative")
        }
        expiresAt := time.Now().Add(duration)
        apiKey.ExpiresAt = &expiresAt
    }
    
    // Store in memory
    s.apiKeys[key] = apiKey
    
    // Persist to database if available
    if s.db != nil {
        if err := s.persistAPIKey(context.Background(), apiKey); err != nil {
            // Log but don't fail - memory storage sufficient for operation
            s.logger.Warn("Failed to persist API key", map[string]interface{}{
                "key_suffix": lastN(key, 4),
                "error":      err.Error(),
            })
        }
    }
    
    s.logger.Info("API key added", map[string]interface{}{
        "key_suffix": lastN(key, 4),
        "role":       settings.Role,
        "scopes":     settings.Scopes,
        "tenant_id":  apiKey.TenantID,
    })
    
    return nil
}

// persistAPIKey saves to database with upsert semantics
func (s *Service) persistAPIKey(ctx context.Context, apiKey *APIKey) error {
    query := `
        INSERT INTO api_keys (
            key, tenant_id, user_id, name, scopes, 
            expires_at, created_at, active
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (key) DO UPDATE SET
            scopes = EXCLUDED.scopes,
            expires_at = EXCLUDED.expires_at,
            active = EXCLUDED.active,
            updated_at = NOW()
    `
    
    _, err := s.db.ExecContext(ctx, query,
        apiKey.Key,
        apiKey.TenantID,
        apiKey.UserID,
        apiKey.Name,
        apiKey.Scopes,
        apiKey.ExpiresAt,
        apiKey.CreatedAt,
        apiKey.Active,
    )
    
    return err
}
