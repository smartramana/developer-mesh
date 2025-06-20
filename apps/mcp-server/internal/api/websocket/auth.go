package websocket

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

// AuthenticatedMessage adds authentication to messages
type AuthenticatedMessage struct {
	Message   *ws.Message
	Signature string    // HMAC signature
	Timestamp time.Time // Anti-replay
}

// SessionKey is used for HMAC signatures per connection
type SessionKey struct {
	Key       []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager manages session keys for connections
type SessionManager struct {
	keys map[string]*SessionKey
	mu   sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		keys: make(map[string]*SessionKey),
	}
}

// GenerateSessionKey generates a new session key for a connection
func (sm *SessionManager) GenerateSessionKey(connectionID string) (*SessionKey, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate random key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(time.Now().UnixNano() & 0xFF)
	}

	sessionKey := &SessionKey{
		Key:       key,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	sm.keys[connectionID] = sessionKey
	return sessionKey, nil
}

// GetSessionKey retrieves a session key for a connection
func (sm *SessionManager) GetSessionKey(connectionID string) (*SessionKey, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key, ok := sm.keys[connectionID]
	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(key.ExpiresAt) {
		return nil, false
	}

	return key, true
}

// RemoveSessionKey removes a session key
func (sm *SessionManager) RemoveSessionKey(connectionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.keys, connectionID)
}

// ValidateConnection performs initial authentication
func (s *Server) ValidateConnection(token string) (*auth.Claims, error) {
	// Use the auth service to validate JWT
	user, err := s.auth.ValidateJWT(context.Background(), token)
	if err != nil {
		return nil, err
	}

	// Convert User to Claims
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: user.ID,
		},
		TenantID: user.TenantID,
		UserID:   user.ID,
		Scopes:   user.Scopes,
	}

	return claims, nil
}

// getSigningKey retrieves the JWT signing key
// TODO: Uncomment when JWT validation is implemented
// func (s *Server) getSigningKey() []byte {
//     // In production, this would come from secure configuration
//     // For now, use a placeholder
//     return []byte("your-secret-key")
// }

// SignMessage creates HMAC signature for a message
func (c *Connection) SignMessage(msg []byte) string {
	sessionKey, ok := c.hub.sessionManager.GetSessionKey(c.ID)
	if !ok {
		return ""
	}

	h := hmac.New(sha256.New, sessionKey.Key)
	h.Write(msg)
	h.Write([]byte(c.ID))
	if _, err := fmt.Fprintf(h, "%d", time.Now().Unix()); err != nil {
		// Hash write error is unlikely but log it
		c.hub.logger.Debug("Error writing to hash", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// VerifyMessage validates HMAC signature
func (c *Connection) VerifyMessage(msg []byte, signature string) error {
	expected := c.SignMessage(msg)

	// Constant-time comparison
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("invalid signature")
	}

	return nil
}

// RateLimiter is already defined in connection.go, but let's add configuration
type RateLimiterConfig struct {
	Rate    float64 // Requests per second
	Burst   float64 // Burst capacity
	PerIP   bool    // Enable per-IP rate limiting
	PerUser bool    // Enable per-user rate limiting
}

// DefaultRateLimiterConfig returns default rate limiter configuration
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		Rate:    1000.0 / 60.0, // 1000 per minute
		Burst:   100,
		PerIP:   true,
		PerUser: true,
	}
}

// IPRateLimiter manages rate limiting per IP address
type IPRateLimiter struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
	config   *RateLimiterConfig
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(config *RateLimiterConfig) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*RateLimiter),
		config:   config,
	}
}

// Allow checks if a request from an IP is allowed
func (r *IPRateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	limiter, ok := r.limiters[ip]
	if !ok {
		limiter = NewRateLimiter(r.config.Rate, r.config.Burst)
		r.limiters[ip] = limiter
	}
	r.mu.Unlock()

	return limiter.Allow()
}

// Cleanup removes old rate limiters
func (r *IPRateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// In production, implement cleanup of inactive IPs
	// For now, just clear if too many entries
	if len(r.limiters) > 10000 {
		r.limiters = make(map[string]*RateLimiter)
	}
}

// AntiReplayCache prevents replay attacks
type AntiReplayCache struct {
	seen map[string]time.Time
	mu   sync.RWMutex
	ttl  time.Duration
}

// NewAntiReplayCache creates a new anti-replay cache
func NewAntiReplayCache(ttl time.Duration) *AntiReplayCache {
	cache := &AntiReplayCache{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Check verifies if a message ID has been seen
func (c *AntiReplayCache) Check(messageID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.seen[messageID]; ok {
		return false // Already seen, reject
	}

	c.seen[messageID] = time.Now()
	return true
}

// cleanup removes expired entries
func (c *AntiReplayCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for id, timestamp := range c.seen {
			if now.Sub(timestamp) > c.ttl {
				delete(c.seen, id)
			}
		}
		c.mu.Unlock()
	}
}

// Add these to the Server struct (would be added in server.go)
// TODO: Uncomment when implementing enhanced auth features
// type ServerWithAuth struct {
//     *Server
//     sessionManager  *SessionManager
//     ipRateLimiter  *IPRateLimiter
//     antiReplayCache *AntiReplayCache
// }

// ValidateMessageAuth validates authentication for a single message
func (s *Server) ValidateMessageAuth(conn *Connection, msg *AuthenticatedMessage) error {
	// Check timestamp (prevent old messages)
	if time.Since(msg.Timestamp) > 5*time.Minute {
		s.metricsCollector.RecordError("auth")
		return errors.New("message too old")
	}

	// Check anti-replay
	if s.antiReplayCache != nil && !s.antiReplayCache.Check(msg.Message.ID) {
		s.metricsCollector.RecordError("auth")
		return errors.New("duplicate message")
	}

	// Verify signature if HMAC is enabled
	if s.config.Security.HMACSignatures && msg.Signature != "" {
		msgBytes, err := json.Marshal(msg.Message)
		if err != nil {
			return err
		}

		if err := conn.VerifyMessage(msgBytes, msg.Signature); err != nil {
			return err
		}
	}

	return nil
}

// Security configuration
type SecurityConfig struct {
	RequireAuth    bool     // Require authentication
	HMACSignatures bool     // Enable HMAC signatures
	AllowedOrigins []string // CORS allowed origins
	MaxFrameSize   int64    // Maximum WebSocket frame size
	EnableTLS      bool     // Require TLS
	MinTLSVersion  string   // Minimum TLS version
}
