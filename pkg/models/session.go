package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// SessionStatus represents the state of a session
type SessionStatus string

const (
	SessionStatusActive     SessionStatus = "active"
	SessionStatusIdle       SessionStatus = "idle"
	SessionStatusExpired    SessionStatus = "expired"
	SessionStatusTerminated SessionStatus = "terminated"
)

// Validate checks if the session status is valid
func (s SessionStatus) Validate() error {
	switch s {
	case SessionStatusActive, SessionStatusIdle, SessionStatusExpired, SessionStatusTerminated:
		return nil
	default:
		return fmt.Errorf("invalid session status: %s", s)
	}
}

// ClientType represents the type of client
type ClientType string

const (
	ClientTypeClaudeCode ClientType = "claude-code"
	ClientTypeIDE        ClientType = "ide"
	ClientTypeAgent      ClientType = "agent"
	ClientTypeCLI        ClientType = "cli"
)

// Validate checks if the client type is valid
func (c ClientType) Validate() error {
	switch c {
	case ClientTypeClaudeCode, ClientTypeIDE, ClientTypeAgent, ClientTypeCLI:
		return nil
	default:
		return fmt.Errorf("invalid client type: %s", c)
	}
}

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
	ClientName    *string     `json:"client_name,omitempty" db:"client_name"`
	ClientType    *ClientType `json:"client_type,omitempty" db:"client_type"`
	ClientVersion *string     `json:"client_version,omitempty" db:"client_version"`

	// Session state
	Status        SessionStatus `json:"status" db:"status"`
	Initialized   bool          `json:"initialized" db:"initialized"`
	CoreSessionID *string       `json:"core_session_id,omitempty" db:"core_session_id"`

	// Passthrough auth (stored encrypted)
	PassthroughAuth          *PassthroughAuthBundle `json:"passthrough_auth,omitempty" db:"-"`
	PassthroughAuthEncrypted *string                `json:"-" db:"passthrough_auth_encrypted"`

	// Metadata
	ConnectionMetadata json.RawMessage `json:"connection_metadata,omitempty" db:"connection_metadata"`
	ContextID          *uuid.UUID      `json:"context_id,omitempty" db:"context_id"`

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

// IsActive returns true if the session is active
func (s *EdgeMCPSession) IsActive() bool {
	return s.Status == SessionStatusActive
}

// IsExpired checks if the session has expired
func (s *EdgeMCPSession) IsExpired() bool {
	if s.Status == SessionStatusExpired {
		return true
	}
	if s.ExpiresAt != nil && time.Now().After(*s.ExpiresAt) {
		return true
	}
	return false
}

// ShouldRefresh checks if the session should be refreshed (80% of lifetime)
func (s *EdgeMCPSession) ShouldRefresh() bool {
	if s.ExpiresAt == nil || !s.IsActive() {
		return false
	}

	totalLifetime := s.ExpiresAt.Sub(s.CreatedAt)
	elapsed := time.Since(s.CreatedAt)

	// Refresh if 80% of lifetime has passed
	return elapsed > (totalLifetime * 80 / 100)
}

// GetSessionAge returns the age of the session
func (s *EdgeMCPSession) GetSessionAge() time.Duration {
	if s.TerminatedAt != nil {
		return s.TerminatedAt.Sub(s.CreatedAt)
	}
	return time.Since(s.CreatedAt)
}

// GetIdleTime returns the time since last activity
func (s *EdgeMCPSession) GetIdleTime() time.Duration {
	return time.Since(s.LastActivityAt)
}

// Validate performs validation on the session
func (s *EdgeMCPSession) Validate() error {
	if s.SessionID == "" {
		return errors.New("session_id is required")
	}
	if s.TenantID == uuid.Nil {
		return errors.New("tenant_id is required")
	}
	if s.EdgeMCPID == "" {
		return errors.New("edge_mcp_id is required")
	}
	if err := s.Status.Validate(); err != nil {
		return errors.Wrap(err, "invalid status")
	}
	if s.ClientType != nil {
		if err := s.ClientType.Validate(); err != nil {
			return errors.Wrap(err, "invalid client_type")
		}
	}
	return nil
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

// IsSuccess returns true if the execution was successful
func (e *SessionToolExecution) IsSuccess() bool {
	return e.Error == nil || *e.Error == ""
}

// GetDuration returns the duration as a time.Duration
func (e *SessionToolExecution) GetDuration() time.Duration {
	if e.DurationMs == nil {
		return 0
	}
	return time.Duration(*e.DurationMs) * time.Millisecond
}

// Validate performs validation on the tool execution
func (e *SessionToolExecution) Validate() error {
	if e.SessionID == uuid.Nil {
		return errors.New("session_id is required")
	}
	if e.ToolName == "" {
		return errors.New("tool_name is required")
	}
	return nil
}

// ConnectionMetadata contains client connection information
type ConnectionMetadata struct {
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	Protocol   string                 `json:"protocol,omitempty"`
	TLSVersion string                 `json:"tls_version,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
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

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, cm)
	case string:
		return json.Unmarshal([]byte(v), cm)
	default:
		return errors.New("cannot scan non-byte/string value into ConnectionMetadata")
	}
}

// SessionMetrics contains aggregated metrics for sessions
type SessionMetrics struct {
	TenantID            uuid.UUID `json:"tenant_id" db:"tenant_id"`
	ActiveSessions      int       `json:"active_sessions" db:"active_sessions"`
	TotalSessions       int       `json:"total_sessions" db:"total_sessions"`
	TotalToolExecutions int       `json:"total_tool_executions" db:"total_tool_executions"`
	TotalTokensUsed     int       `json:"total_tokens_used" db:"total_tokens_used"`
	AvgSessionMinutes   float64   `json:"avg_session_duration_minutes" db:"avg_session_duration_minutes"`
}

// CreateSessionRequest represents a request to create a new session
type CreateSessionRequest struct {
	SessionID       string                 `json:"session_id,omitempty"`
	EdgeMCPID       string                 `json:"edge_mcp_id" validate:"required"`
	TenantID        uuid.UUID              `json:"tenant_id" validate:"required"`
	UserID          *uuid.UUID             `json:"user_id,omitempty"`
	ClientName      string                 `json:"client_name,omitempty"`
	ClientType      ClientType             `json:"client_type,omitempty"`
	ClientVersion   string                 `json:"client_version,omitempty"`
	PassthroughAuth *PassthroughAuthBundle `json:"passthrough_auth,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	TTL             int                    `json:"ttl,omitempty"` // Session TTL in seconds
}

// SessionToolExecutionRequest represents a request to record a tool execution in a session
type SessionToolExecutionRequest struct {
	ToolName   string          `json:"tool_name" validate:"required"`
	ToolID     *uuid.UUID      `json:"tool_id,omitempty"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      *string         `json:"error,omitempty"`
	DurationMs int             `json:"duration_ms,omitempty"`
	TokensUsed int             `json:"tokens_used,omitempty"`
}

// SessionFilter represents filters for querying sessions
type SessionFilter struct {
	TenantID   *uuid.UUID     `json:"tenant_id,omitempty"`
	UserID     *uuid.UUID     `json:"user_id,omitempty"`
	EdgeMCPID  *string        `json:"edge_mcp_id,omitempty"`
	Status     *SessionStatus `json:"status,omitempty"`
	ClientType *ClientType    `json:"client_type,omitempty"`
	ActiveOnly bool           `json:"active_only,omitempty"`
	Since      *time.Time     `json:"since,omitempty"`
	Until      *time.Time     `json:"until,omitempty"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
	OrderBy    string         `json:"order_by,omitempty"`
	OrderDesc  bool           `json:"order_desc,omitempty"`
}

// SetDefaults sets default values for the filter
func (f *SessionFilter) SetDefaults() {
	if f.Limit == 0 {
		f.Limit = 100
	}
	if f.OrderBy == "" {
		f.OrderBy = "created_at"
		f.OrderDesc = true
	}
}

// SessionResponse represents a session response with additional computed fields
type SessionResponse struct {
	*EdgeMCPSession
	SessionAge    string `json:"session_age,omitempty"`
	IdleTime      string `json:"idle_time,omitempty"`
	IsExpired     bool   `json:"is_expired"`
	ShouldRefresh bool   `json:"should_refresh"`
}

// NewSessionResponse creates a new session response from a session
func NewSessionResponse(session *EdgeMCPSession) *SessionResponse {
	if session == nil {
		return nil
	}

	return &SessionResponse{
		EdgeMCPSession: session,
		SessionAge:     session.GetSessionAge().String(),
		IdleTime:       session.GetIdleTime().String(),
		IsExpired:      session.IsExpired(),
		ShouldRefresh:  session.ShouldRefresh(),
	}
}
