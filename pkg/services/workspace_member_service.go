package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// WorkspaceMemberRole represents the role of a member in a workspace
type WorkspaceMemberRole string

const (
	WorkspaceMemberRoleOwner  WorkspaceMemberRole = "owner"
	WorkspaceMemberRoleAdmin  WorkspaceMemberRole = "admin"
	WorkspaceMemberRoleMember WorkspaceMemberRole = "member"
	WorkspaceMemberRoleViewer WorkspaceMemberRole = "viewer"
)

// WorkspaceMember represents a member of a workspace with their role
type WorkspaceMember struct {
	WorkspaceID uuid.UUID              `json:"workspace_id" db:"workspace_id"`
	AgentID     string                 `json:"agent_id" db:"agent_id"`
	Role        WorkspaceMemberRole    `json:"role" db:"role"`
	Permissions map[string]interface{} `json:"permissions" db:"permissions"`
	JoinedAt    time.Time              `json:"joined_at" db:"joined_at"`
	JoinedBy    string                 `json:"joined_by" db:"joined_by"`
	LastActive  time.Time              `json:"last_active" db:"last_active"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
}

// WorkspaceActivity represents an activity log entry
type WorkspaceActivity struct {
	ID           uuid.UUID              `json:"id" db:"id"`
	WorkspaceID  uuid.UUID              `json:"workspace_id" db:"workspace_id"`
	AgentID      string                 `json:"agent_id" db:"agent_id"`
	ActivityType string                 `json:"activity_type" db:"activity_type"`
	ResourceType string                 `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID   string                 `json:"resource_id,omitempty" db:"resource_id"`
	Action       string                 `json:"action" db:"action"`
	Metadata     map[string]interface{} `json:"metadata" db:"metadata"`
	IPAddress    string                 `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    string                 `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
}

// WorkspaceMemberService handles workspace member management
type WorkspaceMemberService interface {
	// Member management
	AddMember(ctx context.Context, workspaceID uuid.UUID, agentID string, role WorkspaceMemberRole) error
	UpdateMemberRole(ctx context.Context, workspaceID uuid.UUID, agentID string, newRole WorkspaceMemberRole) error
	RemoveMember(ctx context.Context, workspaceID uuid.UUID, agentID string) error
	GetMember(ctx context.Context, workspaceID uuid.UUID, agentID string) (*WorkspaceMember, error)
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*WorkspaceMember, error)

	// Role-based access control
	CheckPermission(ctx context.Context, workspaceID uuid.UUID, agentID string, permission string) (bool, error)
	GetMemberRole(ctx context.Context, workspaceID uuid.UUID, agentID string) (WorkspaceMemberRole, error)

	// Activity tracking
	LogActivity(ctx context.Context, activity *WorkspaceActivity) error
	GetActivities(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*WorkspaceActivity, error)

	// Workspace quotas
	CheckMemberQuota(ctx context.Context, workspaceID uuid.UUID) error
	UpdateLastActive(ctx context.Context, workspaceID uuid.UUID, agentID string) error
}

// workspaceMemberService implements WorkspaceMemberService
type workspaceMemberService struct {
	config         ServiceConfig
	txManager      repository.TransactionManager
	eventPublisher events.Publisher
	authorizer     auth.Authorizer
}

// NewWorkspaceMemberService creates a new workspace member service
func NewWorkspaceMemberService(
	config ServiceConfig,
	txManager repository.TransactionManager,
	eventPublisher events.Publisher,
	authorizer auth.Authorizer,
) WorkspaceMemberService {
	return &workspaceMemberService{
		config:         config,
		txManager:      txManager,
		eventPublisher: eventPublisher,
		authorizer:     authorizer,
	}
}

// AddMember adds a new member to a workspace
func (s *workspaceMemberService) AddMember(ctx context.Context, workspaceID uuid.UUID, agentID string, role WorkspaceMemberRole) error {
	ctx, span := s.config.Tracer(ctx, "WorkspaceMemberService.AddMember")
	defer span.End()

	// Get current user
	currentAgentID := auth.GetAgentID(ctx)

	// Check authorization
	decision := s.authorizer.Authorize(ctx, auth.Permission{
		Resource: "workspace:" + workspaceID.String(),
		Action:   "member.add",
	})
	if !decision.Allowed {
		return errors.New("authorization failed: " + decision.Reason)
	}

	return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Check member quota
		if err := s.checkMemberQuotaInTx(ctx, tx, workspaceID); err != nil {
			return err
		}

		// Check if member already exists
		existingMember, err := s.getMemberInTx(ctx, tx, workspaceID, agentID)
		if err == nil && existingMember != nil {
			return errors.New("member already exists in workspace")
		}

		// Add member
		query := `
			INSERT INTO workspace_members (
				workspace_id, agent_id, role, permissions, joined_at, joined_by, last_active, metadata
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8
			)
		`

		now := time.Now()
		_, err = tx.ExecContext(ctx, query,
			workspaceID, agentID, role, models.JSONMap{}, now, currentAgentID, now, models.JSONMap{},
		)
		if err != nil {
			return errors.Wrap(err, "failed to add member")
		}

		// Log activity
		if err := s.logActivityInTx(ctx, tx, &WorkspaceActivity{
			ID:           uuid.New(),
			WorkspaceID:  workspaceID,
			AgentID:      currentAgentID,
			ActivityType: "member.added",
			ResourceType: "agent",
			ResourceID:   agentID,
			Action:       "add",
			Metadata: map[string]interface{}{
				"role":     role,
				"added_by": currentAgentID,
			},
			CreatedAt: now,
		}); err != nil {
			s.config.Logger.Error("Failed to log activity", map[string]interface{}{
				"error": err.Error(),
			})
		}

		// Publish event
		event := &events.WorkspaceMemberAddedEvent{
			BaseEvent: events.BaseEvent{
				ID:        uuid.New().String(),
				Type:      "workspace.member.added",
				Timestamp: now,
				TenantID:  "", // Would need to fetch from workspace
				AgentID:   currentAgentID,
			},
			WorkspaceID: workspaceID.String(),
			MemberID:    agentID,
			Role:        string(role),
			AddedBy:     currentAgentID,
		}

		if err := s.eventPublisher.Publish(ctx, event); err != nil {
			s.config.Logger.Error("Failed to publish member added event", map[string]interface{}{
				"error": err.Error(),
			})
		}

		// Update metrics
		s.config.Metrics.IncrementCounter("workspace_members_added", 1)

		return nil
	})
}

// UpdateMemberRole updates a member's role in a workspace
func (s *workspaceMemberService) UpdateMemberRole(ctx context.Context, workspaceID uuid.UUID, agentID string, newRole WorkspaceMemberRole) error {
	ctx, span := s.config.Tracer(ctx, "WorkspaceMemberService.UpdateMemberRole")
	defer span.End()

	currentAgentID := auth.GetAgentID(ctx)

	// Check authorization
	decision := s.authorizer.Authorize(ctx, auth.Permission{
		Resource: "workspace:" + workspaceID.String(),
		Action:   "member.update",
	})
	if !decision.Allowed {
		return errors.New("authorization failed: " + decision.Reason)
	}

	return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Get current member
		member, err := s.getMemberInTx(ctx, tx, workspaceID, agentID)
		if err != nil {
			return errors.Wrap(err, "failed to get member")
		}

		oldRole := member.Role

		// Prevent demoting the last owner
		if oldRole == WorkspaceMemberRoleOwner && newRole != WorkspaceMemberRoleOwner {
			ownerCount, err := s.getOwnerCountInTx(ctx, tx, workspaceID)
			if err != nil {
				return errors.Wrap(err, "failed to count owners")
			}
			if ownerCount <= 1 {
				return errors.New("cannot demote the last owner")
			}
		}

		// Update role
		query := `
			UPDATE workspace_members 
			SET role = $1, metadata = jsonb_set(metadata, '{previous_role}', to_jsonb($2::text))
			WHERE workspace_id = $3 AND agent_id = $4
		`

		_, err = tx.ExecContext(ctx, query, newRole, oldRole, workspaceID, agentID)
		if err != nil {
			return errors.Wrap(err, "failed to update member role")
		}

		// Log activity
		if err := s.logActivityInTx(ctx, tx, &WorkspaceActivity{
			ID:           uuid.New(),
			WorkspaceID:  workspaceID,
			AgentID:      currentAgentID,
			ActivityType: "member.role_changed",
			ResourceType: "agent",
			ResourceID:   agentID,
			Action:       "update",
			Metadata: map[string]interface{}{
				"old_role":   oldRole,
				"new_role":   newRole,
				"updated_by": currentAgentID,
			},
			CreatedAt: time.Now(),
		}); err != nil {
			s.config.Logger.Error("Failed to log activity", map[string]interface{}{
				"error": err.Error(),
			})
		}

		// Update metrics
		s.config.Metrics.IncrementCounter("workspace_member_role_changes", 1)

		return nil
	})
}

// RemoveMember removes a member from a workspace
func (s *workspaceMemberService) RemoveMember(ctx context.Context, workspaceID uuid.UUID, agentID string) error {
	ctx, span := s.config.Tracer(ctx, "WorkspaceMemberService.RemoveMember")
	defer span.End()

	currentAgentID := auth.GetAgentID(ctx)

	// Check authorization
	decision := s.authorizer.Authorize(ctx, auth.Permission{
		Resource: "workspace:" + workspaceID.String(),
		Action:   "member.remove",
	})
	if !decision.Allowed {
		return errors.New("authorization failed: " + decision.Reason)
	}

	return s.txManager.WithTransaction(ctx, func(ctx context.Context, tx database.Transaction) error {
		// Get member to check role
		member, err := s.getMemberInTx(ctx, tx, workspaceID, agentID)
		if err != nil {
			return errors.Wrap(err, "failed to get member")
		}

		// Prevent removing the last owner
		if member.Role == WorkspaceMemberRoleOwner {
			ownerCount, err := s.getOwnerCountInTx(ctx, tx, workspaceID)
			if err != nil {
				return errors.Wrap(err, "failed to count owners")
			}
			if ownerCount <= 1 {
				return errors.New("cannot remove the last owner")
			}
		}

		// Remove member
		query := `DELETE FROM workspace_members WHERE workspace_id = $1 AND agent_id = $2`
		result, err := tx.ExecContext(ctx, query, workspaceID, agentID)
		if err != nil {
			return errors.Wrap(err, "failed to remove member")
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return errors.New("member not found")
		}

		// Log activity
		if err := s.logActivityInTx(ctx, tx, &WorkspaceActivity{
			ID:           uuid.New(),
			WorkspaceID:  workspaceID,
			AgentID:      currentAgentID,
			ActivityType: "member.removed",
			ResourceType: "agent",
			ResourceID:   agentID,
			Action:       "remove",
			Metadata: map[string]interface{}{
				"removed_role": member.Role,
				"removed_by":   currentAgentID,
			},
			CreatedAt: time.Now(),
		}); err != nil {
			s.config.Logger.Error("Failed to log activity", map[string]interface{}{
				"error": err.Error(),
			})
		}

		// Update metrics
		s.config.Metrics.IncrementCounter("workspace_members_removed", 1)

		return nil
	})
}

// GetMember retrieves a workspace member
func (s *workspaceMemberService) GetMember(ctx context.Context, workspaceID uuid.UUID, agentID string) (*WorkspaceMember, error) {
	ctx, span := s.config.Tracer(ctx, "WorkspaceMemberService.GetMember")
	defer span.End()

	// Check authorization
	decision := s.authorizer.Authorize(ctx, auth.Permission{
		Resource: "workspace:" + workspaceID.String(),
		Action:   "member.read",
	})
	if !decision.Allowed {
		return nil, errors.New("authorization failed: " + decision.Reason)
	}

	return s.getMemberInTx(ctx, nil, workspaceID, agentID)
}

// ListMembers lists all members of a workspace
func (s *workspaceMemberService) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*WorkspaceMember, error) {
	ctx, span := s.config.Tracer(ctx, "WorkspaceMemberService.ListMembers")
	defer span.End()

	// Check authorization
	decision := s.authorizer.Authorize(ctx, auth.Permission{
		Resource: "workspace:" + workspaceID.String(),
		Action:   "member.list",
	})
	if !decision.Allowed {
		return nil, errors.New("authorization failed: " + decision.Reason)
	}

	var members []*WorkspaceMember
	// This would need proper database access through repository
	// For now, returning empty slice
	return members, nil
}

// CheckPermission checks if an agent has a specific permission in a workspace
func (s *workspaceMemberService) CheckPermission(ctx context.Context, workspaceID uuid.UUID, agentID string, permission string) (bool, error) {
	ctx, span := s.config.Tracer(ctx, "WorkspaceMemberService.CheckPermission")
	defer span.End()

	member, err := s.getMemberInTx(ctx, nil, workspaceID, agentID)
	if err != nil {
		return false, err
	}

	// Check role-based permissions
	switch member.Role {
	case WorkspaceMemberRoleOwner:
		return true, nil // Owners have all permissions
	case WorkspaceMemberRoleAdmin:
		// Admins have most permissions except ownership transfer
		if permission == "workspace.ownership.transfer" {
			return false, nil
		}
		return true, nil
	case WorkspaceMemberRoleMember:
		// Members can read and write
		if permission == "workspace.read" || permission == "workspace.write" {
			return true, nil
		}
	case WorkspaceMemberRoleViewer:
		// Viewers can only read
		if permission == "workspace.read" {
			return true, nil
		}
	}

	// Check custom permissions
	if perms, ok := member.Permissions[permission]; ok {
		if allowed, ok := perms.(bool); ok {
			return allowed, nil
		}
	}

	return false, nil
}

// GetMemberRole gets a member's role in a workspace
func (s *workspaceMemberService) GetMemberRole(ctx context.Context, workspaceID uuid.UUID, agentID string) (WorkspaceMemberRole, error) {
	member, err := s.getMemberInTx(ctx, nil, workspaceID, agentID)
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

// LogActivity logs an activity in a workspace
func (s *workspaceMemberService) LogActivity(ctx context.Context, activity *WorkspaceActivity) error {
	ctx, span := s.config.Tracer(ctx, "WorkspaceMemberService.LogActivity")
	defer span.End()

	return s.logActivityInTx(ctx, nil, activity)
}

// GetActivities retrieves recent activities for a workspace
func (s *workspaceMemberService) GetActivities(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*WorkspaceActivity, error) {
	ctx, span := s.config.Tracer(ctx, "WorkspaceMemberService.GetActivities")
	defer span.End()

	// Check authorization
	decision := s.authorizer.Authorize(ctx, auth.Permission{
		Resource: "workspace:" + workspaceID.String(),
		Action:   "activity.read",
	})
	if !decision.Allowed {
		return nil, errors.New("authorization failed: " + decision.Reason)
	}

	query := `
		SELECT * FROM workspace_activities 
		WHERE workspace_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2
	`

	var activities []*WorkspaceActivity
	// This would need proper database access
	// For now, returning empty slice
	_ = query
	_ = limit
	return activities, nil
}

// CheckMemberQuota checks if the workspace has reached its member quota
func (s *workspaceMemberService) CheckMemberQuota(ctx context.Context, workspaceID uuid.UUID) error {
	return s.checkMemberQuotaInTx(ctx, nil, workspaceID)
}

// UpdateLastActive updates the last active timestamp for a member
func (s *workspaceMemberService) UpdateLastActive(ctx context.Context, workspaceID uuid.UUID, agentID string) error {
	query := `
		UPDATE workspace_members 
		SET last_active = $1 
		WHERE workspace_id = $2 AND agent_id = $3
	`

	// This would need proper database access
	_ = query

	return nil
}

// Helper methods

func (s *workspaceMemberService) getMemberInTx(ctx context.Context, tx database.Transaction, workspaceID uuid.UUID, agentID string) (*WorkspaceMember, error) {
	// This would need proper database access through repository
	// For now, returning a mock error
	return nil, errors.New("member not found")
}

func (s *workspaceMemberService) getOwnerCountInTx(ctx context.Context, tx database.Transaction, workspaceID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM workspace_members 
		WHERE workspace_id = $1 AND role = 'owner'
	`

	var count int
	// This would need proper database access
	_ = query
	return count, nil
}

func (s *workspaceMemberService) checkMemberQuotaInTx(ctx context.Context, tx database.Transaction, workspaceID uuid.UUID) error {
	// Get workspace quota and current member count
	query := `
		SELECT w.max_members, COUNT(m.agent_id) as current_members
		FROM workspaces w
		LEFT JOIN workspace_members m ON w.id = m.workspace_id
		WHERE w.id = $1
		GROUP BY w.max_members
	`

	// This would need proper database access
	_ = query

	// For now, assume quota is not exceeded
	return nil
}

func (s *workspaceMemberService) logActivityInTx(ctx context.Context, tx database.Transaction, activity *WorkspaceActivity) error {
	query := `
		INSERT INTO workspace_activities (
			id, workspace_id, agent_id, activity_type, resource_type, resource_id, 
			action, metadata, ip_address, user_agent, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	// Get request metadata if available
	ipAddress := ""
	userAgent := ""
	if reqInfo, ok := ctx.Value("request_info").(map[string]string); ok {
		ipAddress = reqInfo["ip_address"]
		userAgent = reqInfo["user_agent"]
	}

	activity.IPAddress = ipAddress
	activity.UserAgent = userAgent

	// This would need proper database access
	_ = query

	return nil
}
