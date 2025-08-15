package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Common errors
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session has expired")
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
	ListSessions(ctx context.Context, filter *models.SessionFilter) ([]*models.EdgeMCPSession, error)
	CleanupExpiredSessions(ctx context.Context) (int, error)

	// Tool Execution Tracking
	RecordToolExecution(ctx context.Context, execution *models.SessionToolExecution) error
	GetSessionToolExecutions(ctx context.Context, sessionID uuid.UUID, limit int) ([]*models.SessionToolExecution, error)

	// Metrics
	GetSessionMetrics(ctx context.Context, tenantID uuid.UUID, since time.Time) (*models.SessionMetrics, error)
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
		"session_id":  session.SessionID,
		"tenant_id":   session.TenantID,
		"edge_mcp_id": session.EdgeMCPID,
	})

	return nil
}

func (r *sessionRepository) GetSession(ctx context.Context, sessionID string) (*models.EdgeMCPSession, error) {
	query := `
		SELECT 
			id, session_id, tenant_id, user_id, edge_mcp_id,
			client_name, client_type, client_version,
			status, initialized, core_session_id,
			passthrough_auth_encrypted, connection_metadata, context_id,
			last_activity_at, tool_execution_count, total_tokens_used,
			created_at, expires_at, terminated_at, termination_reason
		FROM mcp.edge_mcp_sessions 
		WHERE session_id = $1`

	var session models.EdgeMCPSession
	err := r.db.GetContext(ctx, &session, query, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, errors.Wrap(err, "failed to get session")
	}

	// Check if session is expired
	if session.IsExpired() && session.Status != models.SessionStatusExpired {
		// Update status to expired
		updateQuery := `UPDATE mcp.edge_mcp_sessions SET status = 'expired' WHERE session_id = $1`
		if _, err := r.db.ExecContext(ctx, updateQuery, sessionID); err != nil {
			r.logger.Warn("Failed to update expired session status", map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			})
		}
		session.Status = models.SessionStatusExpired
	}

	return &session, nil
}

func (r *sessionRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (*models.EdgeMCPSession, error) {
	query := `
		SELECT 
			id, session_id, tenant_id, user_id, edge_mcp_id,
			client_name, client_type, client_version,
			status, initialized, core_session_id,
			passthrough_auth_encrypted, connection_metadata, context_id,
			last_activity_at, tool_execution_count, total_tokens_used,
			created_at, expires_at, terminated_at, termination_reason
		FROM mcp.edge_mcp_sessions 
		WHERE id = $1`

	var session models.EdgeMCPSession
	err := r.db.GetContext(ctx, &session, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, errors.Wrap(err, "failed to get session by ID")
	}

	return &session, nil
}

func (r *sessionRepository) UpdateSession(ctx context.Context, session *models.EdgeMCPSession) error {
	query := `
		UPDATE mcp.edge_mcp_sessions SET
			status = :status,
			initialized = :initialized,
			core_session_id = :core_session_id,
			passthrough_auth_encrypted = :passthrough_auth_encrypted,
			connection_metadata = :connection_metadata,
			context_id = :context_id,
			last_activity_at = :last_activity_at,
			tool_execution_count = :tool_execution_count,
			total_tokens_used = :total_tokens_used,
			expires_at = :expires_at,
			terminated_at = :terminated_at,
			termination_reason = :termination_reason
		WHERE session_id = :session_id`

	result, err := r.db.NamedExecContext(ctx, query, session)
	if err != nil {
		return errors.Wrap(err, "failed to update session")
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
		// Check if session exists
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM mcp.edge_mcp_sessions WHERE session_id = $1)`
		if err := r.db.GetContext(ctx, &exists, checkQuery, sessionID); err != nil {
			return errors.Wrap(err, "failed to check session existence")
		}
		if !exists {
			return ErrSessionNotFound
		}
		// Session exists but is not active
		return ErrSessionExpired
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
		"reason":     reason,
	})

	return nil
}

func (r *sessionRepository) ListActiveSessions(ctx context.Context, tenantID uuid.UUID) ([]*models.EdgeMCPSession, error) {
	query := `
		SELECT 
			id, session_id, tenant_id, user_id, edge_mcp_id,
			client_name, client_type, client_version,
			status, initialized, core_session_id,
			passthrough_auth_encrypted, connection_metadata, context_id,
			last_activity_at, tool_execution_count, total_tokens_used,
			created_at, expires_at, terminated_at, termination_reason
		FROM mcp.edge_mcp_sessions 
		WHERE tenant_id = $1 AND status = 'active'
		ORDER BY created_at DESC`

	var sessions []*models.EdgeMCPSession
	err := r.db.SelectContext(ctx, &sessions, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list active sessions")
	}

	return sessions, nil
}

func (r *sessionRepository) ListSessions(ctx context.Context, filter *models.SessionFilter) ([]*models.EdgeMCPSession, error) {
	// Build dynamic query based on filter
	query := `
		SELECT 
			id, session_id, tenant_id, user_id, edge_mcp_id,
			client_name, client_type, client_version,
			status, initialized, core_session_id,
			passthrough_auth_encrypted, connection_metadata, context_id,
			last_activity_at, tool_execution_count, total_tokens_used,
			created_at, expires_at, terminated_at, termination_reason
		FROM mcp.edge_mcp_sessions 
		WHERE 1=1`

	args := []interface{}{}
	argCount := 0

	// Apply filters
	if filter.TenantID != nil {
		argCount++
		query += fmt.Sprintf(" AND tenant_id = $%d", argCount)
		args = append(args, *filter.TenantID)
	}

	if filter.UserID != nil {
		argCount++
		query += fmt.Sprintf(" AND user_id = $%d", argCount)
		args = append(args, *filter.UserID)
	}

	if filter.EdgeMCPID != nil {
		argCount++
		query += fmt.Sprintf(" AND edge_mcp_id = $%d", argCount)
		args = append(args, *filter.EdgeMCPID)
	}

	if filter.Status != nil {
		argCount++
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *filter.Status)
	}

	if filter.ClientType != nil {
		argCount++
		query += fmt.Sprintf(" AND client_type = $%d", argCount)
		args = append(args, *filter.ClientType)
	}

	if filter.ActiveOnly {
		query += " AND status = 'active'"
	}

	if filter.Since != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *filter.Since)
	}

	if filter.Until != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *filter.Until)
	}

	// Apply ordering
	if filter.OrderBy != "" {
		query += fmt.Sprintf(" ORDER BY %s", filter.OrderBy)
		if filter.OrderDesc {
			query += " DESC"
		}
	} else {
		query += " ORDER BY created_at DESC"
	}

	// Apply limit and offset
	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	var sessions []*models.EdgeMCPSession
	err := r.db.SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list sessions")
	}

	return sessions, nil
}

func (r *sessionRepository) CleanupExpiredSessions(ctx context.Context) (int, error) {
	// Use the stored function
	query := `SELECT mcp.cleanup_expired_sessions()`

	var count int
	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, errors.Wrap(err, "failed to cleanup expired sessions")
	}

	if count > 0 {
		r.logger.Info("Cleaned up expired sessions", map[string]interface{}{
			"count": count,
		})
	}

	return count, nil
}

func (r *sessionRepository) RecordToolExecution(ctx context.Context, execution *models.SessionToolExecution) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

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

func (r *sessionRepository) GetSessionToolExecutions(ctx context.Context, sessionID uuid.UUID, limit int) ([]*models.SessionToolExecution, error) {
	query := `
		SELECT 
			id, session_id, tool_name, tool_id,
			arguments, result, error,
			duration_ms, tokens_used, executed_at
		FROM mcp.session_tool_executions
		WHERE session_id = $1
		ORDER BY executed_at DESC`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var executions []*models.SessionToolExecution
	err := r.db.SelectContext(ctx, &executions, query, sessionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session tool executions")
	}

	return executions, nil
}

func (r *sessionRepository) GetSessionMetrics(ctx context.Context, tenantID uuid.UUID, since time.Time) (*models.SessionMetrics, error) {
	query := `
		SELECT 
			$1::uuid as tenant_id,
			COUNT(*) FILTER (WHERE status = 'active') as active_sessions,
			COUNT(*) as total_sessions,
			COALESCE(SUM(tool_execution_count), 0) as total_tool_executions,
			COALESCE(SUM(total_tokens_used), 0) as total_tokens_used,
			COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(terminated_at, CURRENT_TIMESTAMP) - created_at))/60), 0) as avg_session_duration_minutes
		FROM mcp.edge_mcp_sessions
		WHERE tenant_id = $1 AND created_at >= $2`

	var metrics models.SessionMetrics
	err := r.db.GetContext(ctx, &metrics, query, tenantID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session metrics")
	}

	return &metrics, nil
}
