package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/security"
)

// SessionService handles edge MCP session management
type SessionService interface {
	// Session lifecycle
	CreateSession(ctx context.Context, req *models.CreateSessionRequest) (*models.EdgeMCPSession, error)
	GetSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error)
	RefreshSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error)
	TerminateSession(ctx context.Context, sessionID string, reason string) error
	ValidateSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error)

	// Session queries
	ListActiveSessions(ctx context.Context, tenantID uuid.UUID) ([]*models.EdgeMCPSession, error)
	ListSessions(ctx context.Context, filter *models.SessionFilter) ([]*models.EdgeMCPSession, error)
	GetSessionMetrics(ctx context.Context, tenantID uuid.UUID, since time.Time) (*models.SessionMetrics, error)

	// Tool execution
	RecordToolExecution(ctx context.Context, sessionID string, req *models.SessionToolExecutionRequest) error
	GetSessionToolExecutions(ctx context.Context, sessionID string, limit int) ([]*models.SessionToolExecution, error)

	// Maintenance
	CleanupExpiredSessions(ctx context.Context) (int, error)
	UpdateSessionActivity(ctx context.Context, sessionID string) error
}

// sessionService implementation
type sessionService struct {
	repo       repository.SessionRepository
	cache      *redis.Client
	encryption *security.EncryptionService
	logger     observability.Logger
	metrics    observability.MetricsClient

	// Configuration
	defaultTTL           time.Duration
	maxSessionsPerTenant int
	idleTimeout          time.Duration
}

// SessionServiceConfig holds configuration for the session service
type SessionServiceConfig struct {
	Repository  repository.SessionRepository
	Cache       *redis.Client
	Encryption  *security.EncryptionService
	Logger      observability.Logger
	Metrics     observability.MetricsClient
	DefaultTTL  time.Duration
	MaxSessions int
	IdleTimeout time.Duration
}

// NewSessionService creates a new session service
func NewSessionService(config SessionServiceConfig) SessionService {
	// Set defaults
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 24 * time.Hour
	}
	if config.MaxSessions == 0 {
		config.MaxSessions = 100
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 30 * time.Minute
	}

	return &sessionService{
		repo:                 config.Repository,
		cache:                config.Cache,
		encryption:           config.Encryption,
		logger:               config.Logger,
		metrics:              config.Metrics,
		defaultTTL:           config.DefaultTTL,
		maxSessionsPerTenant: config.MaxSessions,
		idleTimeout:          config.IdleTimeout,
	}
}

// CreateSession creates a new edge MCP session
func (s *sessionService) CreateSession(ctx context.Context, req *models.CreateSessionRequest) (*models.EdgeMCPSession, error) {
	startTime := time.Now()
	defer func() {
		s.recordMetric("session.create.duration", time.Since(startTime), nil)
	}()

	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		s.recordMetric("session.create.error", 1, map[string]string{"error": "validation"})
		return nil, errors.Wrap(err, "invalid session request")
	}

	// Check tenant session limit
	activeSessions, err := s.repo.ListActiveSessions(ctx, req.TenantID)
	if err != nil {
		s.recordMetric("session.create.error", 1, map[string]string{"error": "list_active"})
		return nil, errors.Wrap(err, "failed to check active sessions")
	}

	if len(activeSessions) >= s.maxSessionsPerTenant {
		s.recordMetric("session.create.error", 1, map[string]string{"error": "limit_exceeded"})
		return nil, fmt.Errorf("session limit exceeded for tenant: %d/%d", len(activeSessions), s.maxSessionsPerTenant)
	}

	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = s.generateSessionID()
	}

	// Calculate expiry
	ttl := s.defaultTTL
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}
	expiresAt := time.Now().Add(ttl)

	// Encrypt passthrough auth if provided
	var encryptedAuth *string
	if req.PassthroughAuth != nil {
		authJSON, err := json.Marshal(req.PassthroughAuth)
		if err != nil {
			s.recordMetric("session.create.error", 1, map[string]string{"error": "auth_marshal"})
			return nil, errors.Wrap(err, "failed to marshal passthrough auth")
		}

		encrypted, err := s.encryption.EncryptCredential(string(authJSON), req.TenantID.String())
		if err != nil {
			s.recordMetric("session.create.error", 1, map[string]string{"error": "auth_encrypt"})
			return nil, errors.Wrap(err, "failed to encrypt passthrough auth")
		}

		encryptedStr := base64.StdEncoding.EncodeToString(encrypted)
		encryptedAuth = &encryptedStr
	}

	// Create connection metadata
	var metadata json.RawMessage
	if req.Metadata != nil {
		metadataBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			s.recordMetric("session.create.error", 1, map[string]string{"error": "metadata_marshal"})
			return nil, errors.Wrap(err, "failed to marshal metadata")
		}
		metadata = metadataBytes
	}

	// Create session
	session := &models.EdgeMCPSession{
		ID:                       uuid.New(),
		SessionID:                sessionID,
		TenantID:                 req.TenantID,
		UserID:                   req.UserID,
		EdgeMCPID:                req.EdgeMCPID,
		ClientName:               &req.ClientName,
		ClientType:               &req.ClientType,
		ClientVersion:            &req.ClientVersion,
		Status:                   models.SessionStatusActive,
		Initialized:              false,
		PassthroughAuthEncrypted: encryptedAuth,
		ConnectionMetadata:       metadata,
		LastActivityAt:           time.Now(),
		ToolExecutionCount:       0,
		TotalTokensUsed:          0,
		CreatedAt:                time.Now(),
		ExpiresAt:                &expiresAt,
	}

	// Store in database
	if err := s.repo.CreateSession(ctx, session); err != nil {
		s.recordMetric("session.create.error", 1, map[string]string{"error": "database"})
		return nil, errors.Wrap(err, "failed to create session")
	}

	// Cache session if Redis available
	if s.cache != nil {
		s.cacheSession(ctx, session)
	}

	// Log and metrics
	s.logger.Info("Session created", map[string]interface{}{
		"session_id":  session.SessionID,
		"tenant_id":   session.TenantID,
		"edge_mcp_id": session.EdgeMCPID,
		"client_type": req.ClientType,
		"expires_at":  expiresAt,
	})

	s.recordMetric("session.create.success", 1, map[string]string{
		"client_type": string(req.ClientType),
	})

	// Decrypt passthrough auth for response
	session.PassthroughAuth = req.PassthroughAuth

	return session, nil
}

// GetSession retrieves a session by ID
func (s *sessionService) GetSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error) {
	startTime := time.Now()
	defer func() {
		s.recordMetric("session.get.duration", time.Since(startTime), nil)
	}()

	// Try cache first
	if s.cache != nil {
		if session := s.getCachedSession(ctx, sessionID); session != nil {
			s.recordMetric("session.get.cache_hit", 1, nil)
			return session, nil
		}
	}

	// Get from database
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		if err == repository.ErrSessionNotFound {
			s.recordMetric("session.get.not_found", 1, nil)
			return nil, err
		}
		s.recordMetric("session.get.error", 1, nil)
		return nil, errors.Wrap(err, "failed to get session")
	}

	// Check if expired
	if session.IsExpired() {
		s.recordMetric("session.get.expired", 1, nil)
		return nil, repository.ErrSessionExpired
	}

	// Decrypt passthrough auth if present
	if session.PassthroughAuthEncrypted != nil {
		if err := s.decryptPassthroughAuth(ctx, session); err != nil {
			s.logger.Warn("Failed to decrypt passthrough auth", map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			})
		}
	}

	// Cache for next time
	if s.cache != nil {
		s.cacheSession(ctx, session)
	}

	return session, nil
}

// RefreshSession extends the session expiry
func (s *sessionService) RefreshSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error) {
	// Get current session
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Check if session can be refreshed
	if session.Status != models.SessionStatusActive {
		return nil, fmt.Errorf("cannot refresh non-active session")
	}

	// Extend expiry
	newExpiry := time.Now().Add(s.defaultTTL)
	session.ExpiresAt = &newExpiry
	session.LastActivityAt = time.Now()

	// Update in database
	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, errors.Wrap(err, "failed to refresh session")
	}

	// Update cache
	if s.cache != nil {
		s.cacheSession(ctx, session)
	}

	s.logger.Info("Session refreshed", map[string]interface{}{
		"session_id": sessionID,
		"new_expiry": newExpiry,
	})

	s.recordMetric("session.refresh", 1, nil)

	return session, nil
}

// TerminateSession terminates an active session
func (s *sessionService) TerminateSession(ctx context.Context, sessionID string, reason string) error {
	// Terminate in database
	if err := s.repo.TerminateSession(ctx, sessionID, reason); err != nil {
		if err == repository.ErrSessionNotFound {
			return err
		}
		return errors.Wrap(err, "failed to terminate session")
	}

	// Remove from cache
	if s.cache != nil {
		s.invalidateCache(ctx, sessionID)
	}

	s.logger.Info("Session terminated", map[string]interface{}{
		"session_id": sessionID,
		"reason":     reason,
	})

	s.recordMetric("session.terminate", 1, map[string]string{"reason": reason})

	return nil
}

// ValidateSession validates and returns session if valid
func (s *sessionService) ValidateSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error) {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Check status
	if session.Status != models.SessionStatusActive {
		return nil, fmt.Errorf("session is not active: %s", session.Status)
	}

	// Check expiry
	if session.IsExpired() {
		// Mark as expired
		_ = s.repo.UpdateSession(ctx, &models.EdgeMCPSession{
			SessionID: sessionID,
			Status:    models.SessionStatusExpired,
		})
		return nil, repository.ErrSessionExpired
	}

	// Check idle timeout
	if time.Since(session.LastActivityAt) > s.idleTimeout {
		// Mark as idle
		session.Status = models.SessionStatusIdle
		_ = s.repo.UpdateSession(ctx, session)
	}

	return session, nil
}

// ListActiveSessions lists all active sessions for a tenant
func (s *sessionService) ListActiveSessions(ctx context.Context, tenantID uuid.UUID) ([]*models.EdgeMCPSession, error) {
	sessions, err := s.repo.ListActiveSessions(ctx, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list active sessions")
	}

	// Decrypt passthrough auth for each session
	for _, session := range sessions {
		if session.PassthroughAuthEncrypted != nil {
			if err := s.decryptPassthroughAuth(ctx, session); err != nil {
				s.logger.Warn("Failed to decrypt passthrough auth", map[string]interface{}{
					"session_id": session.SessionID,
					"error":      err.Error(),
				})
			}
		}
	}

	return sessions, nil
}

// ListSessions lists sessions with filtering
func (s *sessionService) ListSessions(ctx context.Context, filter *models.SessionFilter) ([]*models.EdgeMCPSession, error) {
	// Apply defaults
	filter.SetDefaults()

	sessions, err := s.repo.ListSessions(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list sessions")
	}

	// Decrypt passthrough auth for each session
	for _, session := range sessions {
		if session.PassthroughAuthEncrypted != nil {
			if err := s.decryptPassthroughAuth(ctx, session); err != nil {
				s.logger.Warn("Failed to decrypt passthrough auth", map[string]interface{}{
					"session_id": session.SessionID,
					"error":      err.Error(),
				})
			}
		}
	}

	return sessions, nil
}

// GetSessionMetrics retrieves aggregated session metrics
func (s *sessionService) GetSessionMetrics(ctx context.Context, tenantID uuid.UUID, since time.Time) (*models.SessionMetrics, error) {
	metrics, err := s.repo.GetSessionMetrics(ctx, tenantID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session metrics")
	}

	return metrics, nil
}

// RecordToolExecution records a tool execution in the session
func (s *sessionService) RecordToolExecution(ctx context.Context, sessionID string, req *models.SessionToolExecutionRequest) error {
	// Get session to validate
	session, err := s.ValidateSession(ctx, sessionID)
	if err != nil {
		return errors.Wrap(err, "invalid session")
	}

	// Create execution record
	execution := &models.SessionToolExecution{
		ID:         uuid.New(),
		SessionID:  session.ID,
		ToolName:   req.ToolName,
		ToolID:     req.ToolID,
		Arguments:  req.Arguments,
		Result:     req.Result,
		Error:      req.Error,
		DurationMs: &req.DurationMs,
		TokensUsed: &req.TokensUsed,
		ExecutedAt: time.Now(),
	}

	// Validate
	if err := execution.Validate(); err != nil {
		return errors.Wrap(err, "invalid tool execution")
	}

	// Record in database
	if err := s.repo.RecordToolExecution(ctx, execution); err != nil {
		return errors.Wrap(err, "failed to record tool execution")
	}

	// Update activity timestamp
	_ = s.UpdateSessionActivity(ctx, sessionID)

	// Emit metrics
	s.recordMetric("tool_execution.recorded", 1, map[string]string{
		"tool_name": req.ToolName,
		"success":   fmt.Sprintf("%t", req.Error == nil || *req.Error == ""),
	})

	if req.DurationMs > 0 {
		s.recordMetric("tool_execution.duration", float64(req.DurationMs), map[string]string{
			"tool_name": req.ToolName,
		})
	}

	if req.TokensUsed > 0 {
		s.recordMetric("tool_execution.tokens", float64(req.TokensUsed), map[string]string{
			"tool_name": req.ToolName,
		})
	}

	return nil
}

// GetSessionToolExecutions retrieves tool executions for a session
func (s *sessionService) GetSessionToolExecutions(ctx context.Context, sessionID string, limit int) ([]*models.SessionToolExecution, error) {
	// Get session to get internal ID
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	executions, err := s.repo.GetSessionToolExecutions(ctx, session.ID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tool executions")
	}

	return executions, nil
}

// CleanupExpiredSessions marks expired sessions as expired
func (s *sessionService) CleanupExpiredSessions(ctx context.Context) (int, error) {
	count, err := s.repo.CleanupExpiredSessions(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to cleanup expired sessions")
	}

	if count > 0 {
		s.logger.Info("Cleaned up expired sessions", map[string]interface{}{
			"count": count,
		})
		s.recordMetric("session.cleanup", float64(count), nil)
	}

	return count, nil
}

// UpdateSessionActivity updates the last activity timestamp
func (s *sessionService) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	if err := s.repo.UpdateSessionActivity(ctx, sessionID); err != nil {
		if err == repository.ErrSessionNotFound || err == repository.ErrSessionExpired {
			return err
		}
		return errors.Wrap(err, "failed to update session activity")
	}

	// Update cache if present
	if s.cache != nil {
		if session := s.getCachedSession(ctx, sessionID); session != nil {
			session.LastActivityAt = time.Now()
			s.cacheSession(ctx, session)
		}
	}

	return nil
}

// Helper methods

func (s *sessionService) validateCreateRequest(req *models.CreateSessionRequest) error {
	if req.EdgeMCPID == "" {
		return errors.New("edge_mcp_id is required")
	}
	if req.TenantID == uuid.Nil {
		return errors.New("tenant_id is required")
	}
	if req.ClientType != "" {
		if err := req.ClientType.Validate(); err != nil {
			return errors.Wrap(err, "invalid client_type")
		}
	}
	if req.PassthroughAuth != nil {
		if err := req.PassthroughAuth.Validate(); err != nil {
			return errors.Wrap(err, "invalid passthrough_auth")
		}
	}
	return nil
}

func (s *sessionService) generateSessionID() string {
	// Generate cryptographically secure random session ID
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to UUID if random fails
		return "ses_" + uuid.New().String()
	}
	return "ses_" + base64.URLEncoding.EncodeToString(b)[:43] // Remove padding
}

func (s *sessionService) cacheSession(ctx context.Context, session *models.EdgeMCPSession) {
	if s.cache == nil {
		return
	}

	// Serialize session
	data, err := json.Marshal(session)
	if err != nil {
		s.logger.Warn("Failed to marshal session for cache", map[string]interface{}{
			"session_id": session.SessionID,
			"error":      err.Error(),
		})
		return
	}

	// Calculate TTL
	ttl := 5 * time.Minute // Default cache TTL
	if session.ExpiresAt != nil {
		remainingTime := time.Until(*session.ExpiresAt)
		if remainingTime < ttl {
			ttl = remainingTime
		}
	}

	// Store in cache
	key := fmt.Sprintf("session:%s", session.SessionID)
	if err := s.cache.Set(ctx, key, data, ttl).Err(); err != nil {
		s.logger.Warn("Failed to cache session", map[string]interface{}{
			"session_id": session.SessionID,
			"error":      err.Error(),
		})
	}
}

func (s *sessionService) getCachedSession(ctx context.Context, sessionID string) *models.EdgeMCPSession {
	if s.cache == nil {
		return nil
	}

	key := fmt.Sprintf("session:%s", sessionID)
	data, err := s.cache.Get(ctx, key).Result()
	if err != nil {
		return nil
	}

	var session models.EdgeMCPSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		s.logger.Warn("Failed to unmarshal cached session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return nil
	}

	return &session
}

func (s *sessionService) invalidateCache(ctx context.Context, sessionID string) {
	if s.cache == nil {
		return
	}

	key := fmt.Sprintf("session:%s", sessionID)
	if err := s.cache.Del(ctx, key).Err(); err != nil {
		s.logger.Warn("Failed to invalidate cached session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
	}
}

func (s *sessionService) decryptPassthroughAuth(ctx context.Context, session *models.EdgeMCPSession) error {
	if session.PassthroughAuthEncrypted == nil || *session.PassthroughAuthEncrypted == "" {
		return nil
	}

	// Decode from base64
	encrypted, err := base64.StdEncoding.DecodeString(*session.PassthroughAuthEncrypted)
	if err != nil {
		return errors.Wrap(err, "failed to decode encrypted auth")
	}

	// Decrypt
	decrypted, err := s.encryption.DecryptCredential(encrypted, session.TenantID.String())
	if err != nil {
		return errors.Wrap(err, "failed to decrypt auth")
	}

	// Unmarshal
	var auth models.PassthroughAuthBundle
	if err := json.Unmarshal([]byte(decrypted), &auth); err != nil {
		return errors.Wrap(err, "failed to unmarshal auth")
	}

	session.PassthroughAuth = &auth

	return nil
}

func (s *sessionService) recordMetric(name string, value interface{}, labels map[string]string) {
	if s.metrics == nil {
		return
	}

	// Add common labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["service"] = "session"

	// Record based on type
	switch v := value.(type) {
	case float64:
		s.metrics.RecordHistogram(name, v, labels)
	case time.Duration:
		s.metrics.RecordHistogram(name, v.Seconds(), labels)
	case int:
		s.metrics.IncrementCounterWithLabels(name, float64(v), labels)
	}
}
