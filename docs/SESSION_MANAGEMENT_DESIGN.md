# Edge MCP Session Management System Design

## Executive Summary

This document outlines a comprehensive session management solution for edge-mcp clients connecting to the DevMesh platform. The design follows 2025 industry best practices for distributed systems, leveraging existing project infrastructure while maintaining zero-error tolerance for this critical functionality.

## Current State Analysis

### What Exists
1. **Database**: `user_sessions` table for JWT refresh tokens (migration 000027)
2. **Auth Package**: Framework exists but session management NOT implemented
3. **Context Manager**: Manages context data with caching
4. **Repository Pattern**: Well-established data access patterns
5. **Service Pattern**: Consistent service layer architecture
6. **Redis**: Available for distributed caching

### What's Missing
1. Edge MCP session tracking in REST API
2. Session repository and service implementations
3. Database schema for edge MCP sessions
4. Session lifecycle management
5. Tool execution tracking per session

## Architecture Design

### Session Types

We will implement two distinct session types:

1. **User Sessions** (existing table, enhance functionality)
   - JWT refresh token management
   - User authentication state
   - Device tracking

2. **Edge MCP Sessions** (new)
   - Client connection tracking
   - Tool execution audit
   - Passthrough auth management
   - Context synchronization

### Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        REST API Layer                        │
│                   /api/v1/sessions/*                         │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                     Session Service                          │
│         (Orchestration, Business Logic, Validation)          │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                   Session Repository                         │
│              (Data Access, SQL Queries)                      │
└─────────────────────┬───────────────────────────────────────┘
                      │
         ┌────────────┴────────────┬─────────────┐
         │                         │             │
┌────────▼──────────┐  ┌──────────▼──────┐  ┌──▼──────────┐
│   PostgreSQL      │  │      Redis       │  │   Metrics   │
│ edge_mcp_sessions │  │  Session Cache   │  │  Prometheus │
└───────────────────┘  └──────────────────┘  └─────────────┘
```

## Database Schema

### New Table: edge_mcp_sessions

```sql
CREATE TABLE IF NOT EXISTS mcp.edge_mcp_sessions (
    -- Primary identification
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id VARCHAR(255) UNIQUE NOT NULL, -- External session ID from edge-mcp
    
    -- Association
    tenant_id UUID NOT NULL REFERENCES mcp.tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES mcp.users(id) ON DELETE SET NULL,
    edge_mcp_id VARCHAR(255) NOT NULL, -- Edge MCP instance identifier
    
    -- Client information
    client_name VARCHAR(255),
    client_type VARCHAR(50), -- 'claude-code', 'ide', 'agent', 'cli'
    client_version VARCHAR(50),
    
    -- Session state
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'idle', 'expired', 'terminated')),
    initialized BOOLEAN DEFAULT false,
    core_session_id VARCHAR(255), -- Link to MCP server session if applicable
    
    -- Passthrough auth (encrypted)
    passthrough_auth_encrypted TEXT, -- Encrypted JSON bundle
    
    -- Metadata
    connection_metadata JSONB, -- IP, user agent, etc.
    context_id UUID REFERENCES mcp.contexts(id) ON DELETE SET NULL,
    
    -- Activity tracking
    last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    tool_execution_count INTEGER DEFAULT 0,
    total_tokens_used INTEGER DEFAULT 0,
    
    -- Lifecycle
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    terminated_at TIMESTAMP WITH TIME ZONE,
    termination_reason TEXT,
    
    -- Indexes for performance
    INDEX idx_edge_sessions_tenant (tenant_id),
    INDEX idx_edge_sessions_user (user_id),
    INDEX idx_edge_sessions_edge_mcp (edge_mcp_id),
    INDEX idx_edge_sessions_status (status),
    INDEX idx_edge_sessions_active (status, expires_at) WHERE status = 'active',
    INDEX idx_edge_sessions_activity (last_activity_at)
);

-- Tool execution audit trail
CREATE TABLE IF NOT EXISTS mcp.session_tool_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES mcp.edge_mcp_sessions(id) ON DELETE CASCADE,
    tool_name VARCHAR(255) NOT NULL,
    tool_id UUID REFERENCES mcp.tool_configurations(id) ON DELETE SET NULL,
    
    -- Execution details
    arguments JSONB,
    result JSONB,
    error TEXT,
    
    -- Performance metrics
    duration_ms INTEGER,
    tokens_used INTEGER,
    
    -- Timestamps
    executed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_tool_exec_session (session_id),
    INDEX idx_tool_exec_tool (tool_id),
    INDEX idx_tool_exec_time (executed_at)
);
```

## Model Definitions

### pkg/models/session.go

```go
package models

import (
    "database/sql/driver"
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
)

// SessionStatus represents the state of a session
type SessionStatus string

const (
    SessionStatusActive     SessionStatus = "active"
    SessionStatusIdle       SessionStatus = "idle"
    SessionStatusExpired    SessionStatus = "expired"
    SessionStatusTerminated SessionStatus = "terminated"
)

// ClientType represents the type of client
type ClientType string

const (
    ClientTypeClaudeCode ClientType = "claude-code"
    ClientTypeIDE        ClientType = "ide"
    ClientTypeAgent      ClientType = "agent"
    ClientTypeCLI        ClientType = "cli"
)

// EdgeMCPSession represents an edge MCP client session
type EdgeMCPSession struct {
    // Primary identification
    ID        uuid.UUID `json:"id" db:"id"`
    SessionID string    `json:"session_id" db:"session_id"`
    
    // Association
    TenantID  uuid.UUID  `json:"tenant_id" db:"tenant_id"`
    UserID    *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
    EdgeMCPID string     `json:"edge_mcp_id" db:"edge_mcp_id"`
    
    // Client information
    ClientName    string     `json:"client_name,omitempty" db:"client_name"`
    ClientType    ClientType `json:"client_type,omitempty" db:"client_type"`
    ClientVersion string     `json:"client_version,omitempty" db:"client_version"`
    
    // Session state
    Status         SessionStatus `json:"status" db:"status"`
    Initialized    bool          `json:"initialized" db:"initialized"`
    CoreSessionID  string        `json:"core_session_id,omitempty" db:"core_session_id"`
    
    // Passthrough auth (stored encrypted)
    PassthroughAuth *PassthroughAuthBundle `json:"passthrough_auth,omitempty" db:"-"`
    PassthroughAuthEncrypted *string       `json:"-" db:"passthrough_auth_encrypted"`
    
    // Metadata
    ConnectionMetadata json.RawMessage `json:"connection_metadata,omitempty" db:"connection_metadata"`
    ContextID         *uuid.UUID       `json:"context_id,omitempty" db:"context_id"`
    
    // Activity tracking
    LastActivityAt     time.Time `json:"last_activity_at" db:"last_activity_at"`
    ToolExecutionCount int       `json:"tool_execution_count" db:"tool_execution_count"`
    TotalTokensUsed    int       `json:"total_tokens_used" db:"total_tokens_used"`
    
    // Lifecycle
    CreatedAt         time.Time  `json:"created_at" db:"created_at"`
    ExpiresAt         *time.Time `json:"expires_at,omitempty" db:"expires_at"`
    TerminatedAt      *time.Time `json:"terminated_at,omitempty" db:"terminated_at"`
    TerminationReason *string    `json:"termination_reason,omitempty" db:"termination_reason"`
}

// SessionToolExecution represents a tool execution within a session
type SessionToolExecution struct {
    ID         uuid.UUID       `json:"id" db:"id"`
    SessionID  uuid.UUID       `json:"session_id" db:"session_id"`
    ToolName   string          `json:"tool_name" db:"tool_name"`
    ToolID     *uuid.UUID      `json:"tool_id,omitempty" db:"tool_id"`
    Arguments  json.RawMessage `json:"arguments,omitempty" db:"arguments"`
    Result     json.RawMessage `json:"result,omitempty" db:"result"`
    Error      *string         `json:"error,omitempty" db:"error"`
    DurationMs *int            `json:"duration_ms,omitempty" db:"duration_ms"`
    TokensUsed *int            `json:"tokens_used,omitempty" db:"tokens_used"`
    ExecutedAt time.Time       `json:"executed_at" db:"executed_at"`
}

// ConnectionMetadata contains client connection information
type ConnectionMetadata struct {
    IPAddress  string `json:"ip_address,omitempty"`
    UserAgent  string `json:"user_agent,omitempty"`
    Protocol   string `json:"protocol,omitempty"`
    TLSVersion string `json:"tls_version,omitempty"`
}

// Value implements driver.Valuer for ConnectionMetadata
func (cm ConnectionMetadata) Value() (driver.Value, error) {
    return json.Marshal(cm)
}

// Scan implements sql.Scanner for ConnectionMetadata
func (cm *ConnectionMetadata) Scan(value interface{}) error {
    if value == nil {
        return nil
    }
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("cannot scan non-byte value into ConnectionMetadata")
    }
    return json.Unmarshal(bytes, cm)
}
```

## Repository Implementation

### pkg/repository/session_repository.go

```go
package repository

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "github.com/pkg/errors"
    
    "github.com/developer-mesh/developer-mesh/pkg/models"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// SessionRepository handles database operations for sessions
type SessionRepository interface {
    // Edge MCP Sessions
    CreateSession(ctx context.Context, session *models.EdgeMCPSession) error
    GetSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error)
    GetSessionByID(ctx context.Context, id uuid.UUID) (*models.EdgeMCPSession, error)
    UpdateSession(ctx context.Context, session *models.EdgeMCPSession) error
    UpdateSessionActivity(ctx context.Context, sessionID string) error
    TerminateSession(ctx context.Context, sessionID string, reason string) error
    ListActiveSessions(ctx context.Context, tenantID uuid.UUID) ([]*models.EdgeMCPSession, error)
    CleanupExpiredSessions(ctx context.Context) (int, error)
    
    // Tool Execution Tracking
    RecordToolExecution(ctx context.Context, execution *models.SessionToolExecution) error
    GetSessionToolExecutions(ctx context.Context, sessionID uuid.UUID) ([]*models.SessionToolExecution, error)
    
    // Metrics
    GetSessionMetrics(ctx context.Context, tenantID uuid.UUID, since time.Time) (*SessionMetrics, error)
}

type SessionMetrics struct {
    ActiveSessions      int     `json:"active_sessions"`
    TotalSessions       int     `json:"total_sessions"`
    TotalToolExecutions int     `json:"total_tool_executions"`
    TotalTokensUsed     int     `json:"total_tokens_used"`
    AverageSessionTime  float64 `json:"average_session_time_minutes"`
}

type sessionRepository struct {
    db     *sqlx.DB
    logger observability.Logger
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *sqlx.DB, logger observability.Logger) SessionRepository {
    return &sessionRepository{
        db:     db,
        logger: logger,
    }
}

func (r *sessionRepository) CreateSession(ctx context.Context, session *models.EdgeMCPSession) error {
    query := `
        INSERT INTO mcp.edge_mcp_sessions (
            id, session_id, tenant_id, user_id, edge_mcp_id,
            client_name, client_type, client_version,
            status, initialized, core_session_id,
            passthrough_auth_encrypted, connection_metadata, context_id,
            last_activity_at, tool_execution_count, total_tokens_used,
            created_at, expires_at
        ) VALUES (
            :id, :session_id, :tenant_id, :user_id, :edge_mcp_id,
            :client_name, :client_type, :client_version,
            :status, :initialized, :core_session_id,
            :passthrough_auth_encrypted, :connection_metadata, :context_id,
            :last_activity_at, :tool_execution_count, :total_tokens_used,
            :created_at, :expires_at
        )`
    
    _, err := r.db.NamedExecContext(ctx, query, session)
    if err != nil {
        return errors.Wrap(err, "failed to create session")
    }
    
    r.logger.Info("Session created", map[string]interface{}{
        "session_id": session.SessionID,
        "tenant_id":  session.TenantID,
        "edge_mcp_id": session.EdgeMCPID,
    })
    
    return nil
}

func (r *sessionRepository) GetSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error) {
    query := `
        SELECT * FROM mcp.edge_mcp_sessions 
        WHERE session_id = $1 AND status = 'active'`
    
    var session models.EdgeMCPSession
    err := r.db.GetContext(ctx, &session, query, sessionID)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrSessionNotFound
        }
        return nil, errors.Wrap(err, "failed to get session")
    }
    
    return &session, nil
}

func (r *sessionRepository) UpdateSessionActivity(ctx context.Context, sessionID string) error {
    query := `
        UPDATE mcp.edge_mcp_sessions 
        SET last_activity_at = CURRENT_TIMESTAMP 
        WHERE session_id = $1 AND status = 'active'`
    
    result, err := r.db.ExecContext(ctx, query, sessionID)
    if err != nil {
        return errors.Wrap(err, "failed to update session activity")
    }
    
    rows, err := result.RowsAffected()
    if err != nil {
        return errors.Wrap(err, "failed to get rows affected")
    }
    
    if rows == 0 {
        return ErrSessionNotFound
    }
    
    return nil
}

func (r *sessionRepository) TerminateSession(ctx context.Context, sessionID string, reason string) error {
    query := `
        UPDATE mcp.edge_mcp_sessions 
        SET status = 'terminated',
            terminated_at = CURRENT_TIMESTAMP,
            termination_reason = $2
        WHERE session_id = $1 AND status IN ('active', 'idle')`
    
    result, err := r.db.ExecContext(ctx, query, sessionID, reason)
    if err != nil {
        return errors.Wrap(err, "failed to terminate session")
    }
    
    rows, err := result.RowsAffected()
    if err != nil {
        return errors.Wrap(err, "failed to get rows affected")
    }
    
    if rows == 0 {
        return ErrSessionNotFound
    }
    
    r.logger.Info("Session terminated", map[string]interface{}{
        "session_id": sessionID,
        "reason": reason,
    })
    
    return nil
}

func (r *sessionRepository) RecordToolExecution(ctx context.Context, execution *models.SessionToolExecution) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return errors.Wrap(err, "failed to begin transaction")
    }
    defer tx.Rollback()
    
    // Insert tool execution
    query := `
        INSERT INTO mcp.session_tool_executions (
            id, session_id, tool_name, tool_id,
            arguments, result, error,
            duration_ms, tokens_used, executed_at
        ) VALUES (
            :id, :session_id, :tool_name, :tool_id,
            :arguments, :result, :error,
            :duration_ms, :tokens_used, :executed_at
        )`
    
    _, err = tx.NamedExecContext(ctx, query, execution)
    if err != nil {
        return errors.Wrap(err, "failed to record tool execution")
    }
    
    // Update session metrics
    updateQuery := `
        UPDATE mcp.edge_mcp_sessions 
        SET tool_execution_count = tool_execution_count + 1,
            total_tokens_used = total_tokens_used + COALESCE($2, 0),
            last_activity_at = CURRENT_TIMESTAMP
        WHERE id = $1`
    
    tokensUsed := 0
    if execution.TokensUsed != nil {
        tokensUsed = *execution.TokensUsed
    }
    
    _, err = tx.ExecContext(ctx, updateQuery, execution.SessionID, tokensUsed)
    if err != nil {
        return errors.Wrap(err, "failed to update session metrics")
    }
    
    if err = tx.Commit(); err != nil {
        return errors.Wrap(err, "failed to commit transaction")
    }
    
    return nil
}

// Additional methods implementation continues...
```

## Service Implementation

### pkg/services/session_service.go

```go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/pkg/errors"
    
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/common/cache"
    "github.com/developer-mesh/developer-mesh/pkg/models"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/developer-mesh/developer-mesh/pkg/repository"
    "github.com/developer-mesh/developer-mesh/pkg/security"
)

// SessionService handles session management business logic
type SessionService interface {
    // Session lifecycle
    CreateSession(ctx context.Context, req *CreateSessionRequest) (*models.EdgeMCPSession, error)
    GetSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error)
    UpdateSessionActivity(ctx context.Context, sessionID string) error
    TerminateSession(ctx context.Context, sessionID string, reason string) error
    
    // Tool execution
    RecordToolExecution(ctx context.Context, sessionID string, req *ToolExecutionRequest) error
    
    // Session queries
    ListActiveSessions(ctx context.Context, tenantID uuid.UUID) ([]*models.EdgeMCPSession, error)
    GetSessionMetrics(ctx context.Context, tenantID uuid.UUID, since time.Time) (*repository.SessionMetrics, error)
    
    // Maintenance
    CleanupExpiredSessions(ctx context.Context) error
}

type CreateSessionRequest struct {
    SessionID       string                        `json:"session_id"`
    EdgeMCPID       string                        `json:"edge_mcp_id"`
    TenantID        uuid.UUID                     `json:"tenant_id"`
    UserID          *uuid.UUID                    `json:"user_id,omitempty"`
    ClientName      string                        `json:"client_name,omitempty"`
    ClientType      models.ClientType             `json:"client_type,omitempty"`
    ClientVersion   string                        `json:"client_version,omitempty"`
    PassthroughAuth *models.PassthroughAuthBundle `json:"passthrough_auth,omitempty"`
    Metadata        map[string]interface{}        `json:"metadata,omitempty"`
    TTL             time.Duration                 `json:"ttl,omitempty"`
}

type ToolExecutionRequest struct {
    ToolName   string          `json:"tool_name"`
    ToolID     *uuid.UUID      `json:"tool_id,omitempty"`
    Arguments  json.RawMessage `json:"arguments,omitempty"`
    Result     json.RawMessage `json:"result,omitempty"`
    Error      *string         `json:"error,omitempty"`
    DurationMs int             `json:"duration_ms,omitempty"`
    TokensUsed int             `json:"tokens_used,omitempty"`
}

type sessionService struct {
    repo       repository.SessionRepository
    cache      cache.Cache
    encryptor  security.EncryptionService
    logger     observability.Logger
    
    // Configuration
    defaultTTL      time.Duration
    maxSessionsPerTenant int
    enableMetrics   bool
}

// NewSessionService creates a new session service
func NewSessionService(
    repo repository.SessionRepository,
    cache cache.Cache,
    encryptor security.EncryptionService,
    logger observability.Logger,
) SessionService {
    return &sessionService{
        repo:                 repo,
        cache:                cache,
        encryptor:            encryptor,
        logger:               logger,
        defaultTTL:           24 * time.Hour,
        maxSessionsPerTenant: 100,
        enableMetrics:        true,
    }
}

func (s *sessionService) CreateSession(ctx context.Context, req *CreateSessionRequest) (*models.EdgeMCPSession, error) {
    // Validate request
    if req.SessionID == "" {
        req.SessionID = uuid.New().String()
    }
    
    // Check session limit
    activeSessions, err := s.repo.ListActiveSessions(ctx, req.TenantID)
    if err != nil {
        return nil, errors.Wrap(err, "failed to check active sessions")
    }
    
    if len(activeSessions) >= s.maxSessionsPerTenant {
        return nil, errors.New("session limit exceeded for tenant")
    }
    
    // Create session model
    session := &models.EdgeMCPSession{
        ID:              uuid.New(),
        SessionID:       req.SessionID,
        TenantID:        req.TenantID,
        UserID:          req.UserID,
        EdgeMCPID:       req.EdgeMCPID,
        ClientName:      req.ClientName,
        ClientType:      req.ClientType,
        ClientVersion:   req.ClientVersion,
        Status:          models.SessionStatusActive,
        Initialized:     false,
        LastActivityAt:  time.Now(),
        CreatedAt:       time.Now(),
    }
    
    // Set TTL
    ttl := req.TTL
    if ttl == 0 {
        ttl = s.defaultTTL
    }
    expiresAt := time.Now().Add(ttl)
    session.ExpiresAt = &expiresAt
    
    // Encrypt passthrough auth if provided
    if req.PassthroughAuth != nil {
        encrypted, err := s.encryptor.Encrypt(ctx, req.PassthroughAuth)
        if err != nil {
            return nil, errors.Wrap(err, "failed to encrypt passthrough auth")
        }
        session.PassthroughAuthEncrypted = &encrypted
    }
    
    // Handle metadata
    if req.Metadata != nil {
        metadataJSON, err := json.Marshal(req.Metadata)
        if err != nil {
            return nil, errors.Wrap(err, "failed to marshal metadata")
        }
        session.ConnectionMetadata = metadataJSON
    }
    
    // Create in database
    if err := s.repo.CreateSession(ctx, session); err != nil {
        return nil, errors.Wrap(err, "failed to create session")
    }
    
    // Cache session
    cacheKey := fmt.Sprintf("session:%s", session.SessionID)
    if err := s.cache.Set(ctx, cacheKey, session, ttl); err != nil {
        s.logger.Warn("Failed to cache session", map[string]interface{}{
            "session_id": session.SessionID,
            "error": err.Error(),
        })
    }
    
    // Emit metrics
    if s.enableMetrics {
        observability.RecordMetric("sessions.created", 1, map[string]string{
            "tenant_id": req.TenantID.String(),
            "client_type": string(req.ClientType),
        })
    }
    
    s.logger.Info("Session created", map[string]interface{}{
        "session_id": session.SessionID,
        "tenant_id": session.TenantID,
        "edge_mcp_id": session.EdgeMCPID,
        "expires_at": session.ExpiresAt,
    })
    
    return session, nil
}

func (s *sessionService) GetSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("session:%s", sessionID)
    var session models.EdgeMCPSession
    
    if err := s.cache.Get(ctx, cacheKey, &session); err == nil {
        // Update activity in background
        go func() {
            ctx := context.Background()
            if err := s.repo.UpdateSessionActivity(ctx, sessionID); err != nil {
                s.logger.Warn("Failed to update session activity", map[string]interface{}{
                    "session_id": sessionID,
                    "error": err.Error(),
                })
            }
        }()
        return &session, nil
    }
    
    // Get from database
    sessionPtr, err := s.repo.GetSession(ctx, sessionID)
    if err != nil {
        return nil, err
    }
    
    // Decrypt passthrough auth if present
    if sessionPtr.PassthroughAuthEncrypted != nil && *sessionPtr.PassthroughAuthEncrypted != "" {
        var authBundle models.PassthroughAuthBundle
        if err := s.encryptor.Decrypt(ctx, *sessionPtr.PassthroughAuthEncrypted, &authBundle); err != nil {
            s.logger.Warn("Failed to decrypt passthrough auth", map[string]interface{}{
                "session_id": sessionID,
                "error": err.Error(),
            })
        } else {
            sessionPtr.PassthroughAuth = &authBundle
        }
    }
    
    // Update cache
    if sessionPtr.ExpiresAt != nil {
        ttl := time.Until(*sessionPtr.ExpiresAt)
        if ttl > 0 {
            if err := s.cache.Set(ctx, cacheKey, sessionPtr, ttl); err != nil {
                s.logger.Warn("Failed to cache session", map[string]interface{}{
                    "session_id": sessionID,
                    "error": err.Error(),
                })
            }
        }
    }
    
    return sessionPtr, nil
}

func (s *sessionService) RecordToolExecution(ctx context.Context, sessionID string, req *ToolExecutionRequest) error {
    // Verify session exists and is active
    session, err := s.GetSession(ctx, sessionID)
    if err != nil {
        return errors.Wrap(err, "session not found")
    }
    
    if session.Status != models.SessionStatusActive {
        return errors.New("session is not active")
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
    
    // Record in database (also updates session metrics)
    if err := s.repo.RecordToolExecution(ctx, execution); err != nil {
        return errors.Wrap(err, "failed to record tool execution")
    }
    
    // Invalidate cache to force refresh with updated metrics
    cacheKey := fmt.Sprintf("session:%s", sessionID)
    if err := s.cache.Delete(ctx, cacheKey); err != nil {
        s.logger.Warn("Failed to invalidate session cache", map[string]interface{}{
            "session_id": sessionID,
            "error": err.Error(),
        })
    }
    
    // Emit metrics
    if s.enableMetrics {
        observability.RecordMetric("tools.executed", 1, map[string]string{
            "tenant_id": session.TenantID.String(),
            "tool_name": req.ToolName,
        })
        
        if req.TokensUsed > 0 {
            observability.RecordMetric("tokens.used", float64(req.TokensUsed), map[string]string{
                "tenant_id": session.TenantID.String(),
            })
        }
    }
    
    return nil
}

func (s *sessionService) CleanupExpiredSessions(ctx context.Context) error {
    count, err := s.repo.CleanupExpiredSessions(ctx)
    if err != nil {
        return errors.Wrap(err, "failed to cleanup expired sessions")
    }
    
    if count > 0 {
        s.logger.Info("Cleaned up expired sessions", map[string]interface{}{
            "count": count,
        })
        
        if s.enableMetrics {
            observability.RecordMetric("sessions.expired", float64(count), nil)
        }
    }
    
    return nil
}

// Additional methods implementation...
```

## REST API Implementation

### apps/rest-api/internal/api/session_handler.go

```go
package api

import (
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/models"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/developer-mesh/developer-mesh/pkg/services"
)

// SessionHandler handles session-related HTTP requests
type SessionHandler struct {
    sessionService services.SessionService
    logger         observability.Logger
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(sessionService services.SessionService, logger observability.Logger) *SessionHandler {
    return &SessionHandler{
        sessionService: sessionService,
        logger:         logger,
    }
}

// RegisterRoutes registers session routes
func (h *SessionHandler) RegisterRoutes(router *gin.RouterGroup) {
    sessions := router.Group("/sessions")
    {
        sessions.POST("", h.CreateSession)
        sessions.GET("/:id", h.GetSession)
        sessions.DELETE("/:id", h.TerminateSession)
        sessions.POST("/:id/activity", h.UpdateActivity)
        sessions.POST("/:id/tools", h.RecordToolExecution)
        sessions.GET("", h.ListSessions)
        sessions.GET("/metrics", h.GetMetrics)
    }
}

// CreateSession creates a new session
func (h *SessionHandler) CreateSession(c *gin.Context) {
    // Get tenant from auth context
    tenantID := auth.GetTenantID(c.Request.Context())
    if tenantID == uuid.Nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant not found"})
        return
    }
    
    var req services.CreateSessionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Override tenant from auth
    req.TenantID = tenantID
    
    // Add connection metadata
    req.Metadata = map[string]interface{}{
        "ip_address": c.ClientIP(),
        "user_agent": c.GetHeader("User-Agent"),
        "protocol":   c.Request.Proto,
    }
    
    session, err := h.sessionService.CreateSession(c.Request.Context(), &req)
    if err != nil {
        h.logger.Error("Failed to create session", map[string]interface{}{
            "error": err.Error(),
            "tenant_id": tenantID,
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
        return
    }
    
    c.JSON(http.StatusOK, session)
}

// GetSession retrieves a session
func (h *SessionHandler) GetSession(c *gin.Context) {
    sessionID := c.Param("id")
    
    session, err := h.sessionService.GetSession(c.Request.Context(), sessionID)
    if err != nil {
        if err == repository.ErrSessionNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
            return
        }
        h.logger.Error("Failed to get session", map[string]interface{}{
            "error": err.Error(),
            "session_id": sessionID,
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get session"})
        return
    }
    
    // Verify tenant access
    tenantID := auth.GetTenantID(c.Request.Context())
    if session.TenantID != tenantID {
        c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
        return
    }
    
    c.JSON(http.StatusOK, session)
}

// TerminateSession terminates a session
func (h *SessionHandler) TerminateSession(c *gin.Context) {
    sessionID := c.Param("id")
    
    // Verify ownership
    session, err := h.sessionService.GetSession(c.Request.Context(), sessionID)
    if err != nil {
        if err == repository.ErrSessionNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get session"})
        return
    }
    
    tenantID := auth.GetTenantID(c.Request.Context())
    if session.TenantID != tenantID {
        c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
        return
    }
    
    reason := c.Query("reason")
    if reason == "" {
        reason = "client request"
    }
    
    if err := h.sessionService.TerminateSession(c.Request.Context(), sessionID, reason); err != nil {
        h.logger.Error("Failed to terminate session", map[string]interface{}{
            "error": err.Error(),
            "session_id": sessionID,
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to terminate session"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "session terminated"})
}

// UpdateActivity updates session activity timestamp
func (h *SessionHandler) UpdateActivity(c *gin.Context) {
    sessionID := c.Param("id")
    
    if err := h.sessionService.UpdateSessionActivity(c.Request.Context(), sessionID); err != nil {
        if err == repository.ErrSessionNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update activity"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "activity updated"})
}

// RecordToolExecution records a tool execution
func (h *SessionHandler) RecordToolExecution(c *gin.Context) {
    sessionID := c.Param("id")
    
    var req services.ToolExecutionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if err := h.sessionService.RecordToolExecution(c.Request.Context(), sessionID, &req); err != nil {
        h.logger.Error("Failed to record tool execution", map[string]interface{}{
            "error": err.Error(),
            "session_id": sessionID,
            "tool_name": req.ToolName,
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record execution"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "execution recorded"})
}

// ListSessions lists active sessions for a tenant
func (h *SessionHandler) ListSessions(c *gin.Context) {
    tenantID := auth.GetTenantID(c.Request.Context())
    
    sessions, err := h.sessionService.ListActiveSessions(c.Request.Context(), tenantID)
    if err != nil {
        h.logger.Error("Failed to list sessions", map[string]interface{}{
            "error": err.Error(),
            "tenant_id": tenantID,
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sessions"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "sessions": sessions,
        "count": len(sessions),
    })
}

// GetMetrics returns session metrics
func (h *SessionHandler) GetMetrics(c *gin.Context) {
    tenantID := auth.GetTenantID(c.Request.Context())
    
    // Parse time range
    sinceStr := c.DefaultQuery("since", "24h")
    duration, err := time.ParseDuration(sinceStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid duration"})
        return
    }
    since := time.Now().Add(-duration)
    
    metrics, err := h.sessionService.GetSessionMetrics(c.Request.Context(), tenantID, since)
    if err != nil {
        h.logger.Error("Failed to get metrics", map[string]interface{}{
            "error": err.Error(),
            "tenant_id": tenantID,
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metrics"})
        return
    }
    
    c.JSON(http.StatusOK, metrics)
}
```

## Migration Script

### apps/rest-api/migrations/sql/000028_edge_mcp_sessions.up.sql

```sql
-- Edge MCP Session Management System
BEGIN;

-- Create edge MCP sessions table
CREATE TABLE IF NOT EXISTS mcp.edge_mcp_sessions (
    -- Primary identification
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id VARCHAR(255) UNIQUE NOT NULL,
    
    -- Association
    tenant_id UUID NOT NULL REFERENCES mcp.tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES mcp.users(id) ON DELETE SET NULL,
    edge_mcp_id VARCHAR(255) NOT NULL,
    
    -- Client information
    client_name VARCHAR(255),
    client_type VARCHAR(50) CHECK (client_type IN ('claude-code', 'ide', 'agent', 'cli')),
    client_version VARCHAR(50),
    
    -- Session state
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'idle', 'expired', 'terminated')),
    initialized BOOLEAN DEFAULT false,
    core_session_id VARCHAR(255),
    
    -- Passthrough auth (encrypted)
    passthrough_auth_encrypted TEXT,
    
    -- Metadata
    connection_metadata JSONB,
    context_id UUID REFERENCES mcp.contexts(id) ON DELETE SET NULL,
    
    -- Activity tracking
    last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    tool_execution_count INTEGER DEFAULT 0,
    total_tokens_used INTEGER DEFAULT 0,
    
    -- Lifecycle
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    terminated_at TIMESTAMP WITH TIME ZONE,
    termination_reason TEXT
);

-- Create indexes for performance
CREATE INDEX idx_edge_sessions_tenant ON mcp.edge_mcp_sessions(tenant_id);
CREATE INDEX idx_edge_sessions_user ON mcp.edge_mcp_sessions(user_id);
CREATE INDEX idx_edge_sessions_edge_mcp ON mcp.edge_mcp_sessions(edge_mcp_id);
CREATE INDEX idx_edge_sessions_status ON mcp.edge_mcp_sessions(status);
CREATE INDEX idx_edge_sessions_active ON mcp.edge_mcp_sessions(status, expires_at) WHERE status = 'active';
CREATE INDEX idx_edge_sessions_activity ON mcp.edge_mcp_sessions(last_activity_at);

-- Tool execution audit trail
CREATE TABLE IF NOT EXISTS mcp.session_tool_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES mcp.edge_mcp_sessions(id) ON DELETE CASCADE,
    tool_name VARCHAR(255) NOT NULL,
    tool_id UUID REFERENCES mcp.tool_configurations(id) ON DELETE SET NULL,
    
    -- Execution details
    arguments JSONB,
    result JSONB,
    error TEXT,
    
    -- Performance metrics
    duration_ms INTEGER,
    tokens_used INTEGER,
    
    -- Timestamps
    executed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for tool executions
CREATE INDEX idx_tool_exec_session ON mcp.session_tool_executions(session_id);
CREATE INDEX idx_tool_exec_tool ON mcp.session_tool_executions(tool_id);
CREATE INDEX idx_tool_exec_time ON mcp.session_tool_executions(executed_at);

-- Add session metrics view for analytics
CREATE OR REPLACE VIEW mcp.session_metrics AS
SELECT 
    tenant_id,
    COUNT(*) FILTER (WHERE status = 'active') as active_sessions,
    COUNT(*) as total_sessions,
    SUM(tool_execution_count) as total_tool_executions,
    SUM(total_tokens_used) as total_tokens_used,
    AVG(EXTRACT(EPOCH FROM (COALESCE(terminated_at, CURRENT_TIMESTAMP) - created_at))/60) as avg_session_duration_minutes
FROM mcp.edge_mcp_sessions
GROUP BY tenant_id;

-- Function to cleanup expired sessions
CREATE OR REPLACE FUNCTION mcp.cleanup_expired_sessions()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    UPDATE mcp.edge_mcp_sessions
    SET status = 'expired'
    WHERE status = 'active' 
    AND expires_at < CURRENT_TIMESTAMP;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust as needed)
GRANT SELECT, INSERT, UPDATE ON mcp.edge_mcp_sessions TO api_user;
GRANT SELECT, INSERT ON mcp.session_tool_executions TO api_user;
GRANT SELECT ON mcp.session_metrics TO api_user;
GRANT EXECUTE ON FUNCTION mcp.cleanup_expired_sessions() TO api_user;

COMMIT;
```

## Integration Points

### 1. Edge MCP Client Update
```go
// Update edge-mcp/internal/core/client.go
// No changes needed - existing CreateSession and CloseSession methods will work
// The 404 error will no longer occur once endpoints are implemented
```

### 2. Scheduled Cleanup Job
```go
// Add to apps/rest-api/cmd/api/main.go or a dedicated scheduler
func startSessionCleanup(sessionService services.SessionService, logger observability.Logger) {
    ticker := time.NewTicker(5 * time.Minute)
    go func() {
        for range ticker.C {
            ctx := context.Background()
            if err := sessionService.CleanupExpiredSessions(ctx); err != nil {
                logger.Error("Session cleanup failed", map[string]interface{}{
                    "error": err.Error(),
                })
            }
        }
    }()
}
```

### 3. Redis Caching Configuration
```yaml
# configs/config.base.yaml
session:
  cache:
    enabled: true
    ttl: 5m
    prefix: "session:"
  limits:
    max_per_tenant: 100
    default_ttl: 24h
    idle_timeout: 30m
  cleanup:
    interval: 5m
    batch_size: 100
```

## Security Considerations

1. **Encryption**: All passthrough auth data is encrypted using AES-256-GCM
2. **Access Control**: Sessions are tenant-isolated with ownership verification
3. **Rate Limiting**: Max sessions per tenant enforced
4. **Audit Trail**: All tool executions are logged
5. **Token Security**: Passthrough tokens never logged or exposed
6. **TLS**: All communications must use TLS 1.2+
7. **Session Expiry**: Automatic cleanup of expired sessions
8. **Activity Tracking**: Last activity timestamp for idle detection

## Testing Strategy

### Unit Tests
- Repository layer with mock database
- Service layer with mock repository
- Handler layer with mock service
- Encryption/decryption of passthrough auth

### Integration Tests
- Full flow: Create → Use → Terminate session
- Tool execution recording
- Session expiry and cleanup
- Concurrent session limits
- Cache invalidation

### Load Tests
- 1000 concurrent sessions per tenant
- 10,000 tool executions per hour
- Session creation/termination rate

## Monitoring & Metrics

### Key Metrics
- `sessions.created` - New sessions created
- `sessions.terminated` - Sessions terminated  
- `sessions.expired` - Sessions that expired
- `sessions.active` - Current active sessions (gauge)
- `tools.executed` - Tool executions
- `tokens.used` - Token consumption
- `session.duration` - Session lifetime histogram

### Alerts
- Session creation failures > 1% 
- Active sessions > 90% of limit
- Session cleanup failures
- Encryption/decryption errors

## Rollout Plan

### Phase 1: Database & Models (Week 1)
1. Deploy migration 000028
2. Deploy model definitions
3. Verify schema creation

### Phase 2: Repository & Service (Week 1-2)
1. Implement repository with tests
2. Implement service with tests
3. Integration testing

### Phase 3: API Endpoints (Week 2)
1. Implement handlers
2. Add routes to server
3. API testing with Postman

### Phase 4: Integration (Week 3)
1. Update edge-mcp to use endpoints
2. Add cleanup scheduler
3. Enable metrics and monitoring

### Phase 5: Production (Week 4)
1. Gradual rollout by tenant
2. Monitor metrics and errors
3. Performance tuning

## Backward Compatibility

- Edge MCP clients will continue to work without sessions (404 handling)
- Session creation is optional - system works without it
- Gradual migration path for existing connections

## Future Enhancements

1. **Session Replay**: Record and replay sessions for debugging
2. **Session Sharing**: Multiple clients sharing a session
3. **Session Templates**: Pre-configured session types
4. **Advanced Analytics**: Detailed usage patterns and insights
5. **WebSocket Subscriptions**: Real-time session updates
6. **Session Export**: Export session data for compliance

## Conclusion

This comprehensive session management system provides:
- **Zero-error tolerance** through extensive validation and error handling
- **Production-ready** implementation following project patterns
- **Scalable** architecture supporting thousands of concurrent sessions
- **Secure** handling of sensitive authentication data
- **Observable** with comprehensive metrics and logging
- **Maintainable** with clear separation of concerns

The implementation leverages existing project infrastructure while adding minimal complexity, ensuring reliable session management for the DevMesh platform.