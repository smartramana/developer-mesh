package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Workspace represents a collaborative space for agents
type Workspace struct {
	ID             uuid.UUID           `json:"id" db:"id"`
	TenantID       uuid.UUID           `json:"tenant_id" db:"tenant_id"`
	Name           string              `json:"name" db:"name"`
	Type           string              `json:"type" db:"type"`
	OwnerID        string              `json:"owner_id" db:"owner_id"`
	Description    string              `json:"description,omitempty" db:"description"`
	Configuration  JSONMap             `json:"configuration" db:"configuration"`
	Visibility     WorkspaceVisibility `json:"visibility" db:"visibility"`
	State          JSONMap             `json:"state" db:"state"`
	StateVersion   int64               `json:"state_version" db:"state_version"`
	LastActivityAt time.Time           `json:"last_activity_at" db:"last_activity_at"`
	LockedBy       *string             `json:"locked_by,omitempty" db:"locked_by"`
	LockedAt       *time.Time          `json:"locked_at,omitempty" db:"locked_at"`
	LockExpiresAt  *time.Time          `json:"lock_expires_at,omitempty" db:"lock_expires_at"`
	CreatedAt      time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at" db:"updated_at"`
	DeletedAt      *time.Time          `json:"deleted_at,omitempty" db:"deleted_at"`

	// New production fields for Phase 3
	IsPublic bool              `json:"is_public" db:"is_public"`
	Settings WorkspaceSettings `json:"settings" db:"settings"`
	Tags     pq.StringArray    `json:"tags" db:"tags"`
	Metadata JSONMap           `json:"metadata" db:"metadata"`

	// Additional fields for production
	Owner    string          `json:"owner" db:"owner"`
	Status   WorkspaceStatus `json:"status" db:"status"`
	Features pq.StringArray  `json:"features" db:"features"`
	Limits   WorkspaceLimits `json:"limits" db:"limits"`
	Stats    *WorkspaceStats `json:"stats,omitempty" db:"-"` // Computed field

	// Runtime data
	Members   []*WorkspaceMember `json:"members,omitempty" db:"-"`
	Documents []*SharedDocument  `json:"documents,omitempty" db:"-"`
}

// WorkspaceVisibility defines who can access a workspace
type WorkspaceVisibility string

const (
	WorkspaceVisibilityPrivate WorkspaceVisibility = "private"
	WorkspaceVisibilityTeam    WorkspaceVisibility = "team"
	WorkspaceVisibilityPublic  WorkspaceVisibility = "public"
)

// WorkspaceMember represents an agent's membership in a workspace
type WorkspaceMember struct {
	WorkspaceID uuid.UUID  `json:"workspace_id" db:"workspace_id"`
	AgentID     string     `json:"agent_id" db:"agent_id"`
	TenantID    uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	Role        MemberRole `json:"role" db:"role"`
	Permissions JSONMap    `json:"permissions" db:"permissions"`
	JoinedAt    time.Time  `json:"joined_at" db:"joined_at"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty" db:"last_seen_at"`

	// Runtime data
	Agent interface{} `json:"agent,omitempty" db:"-"` // Agent details if loaded
}

// MemberRole defines the role of a member in a workspace
type MemberRole string

const (
	MemberRoleOwner  MemberRole = "owner"
	MemberRoleAdmin  MemberRole = "admin"
	MemberRoleMember MemberRole = "member"
	MemberRoleViewer MemberRole = "viewer"
)

// WorkspaceActivity represents an activity event in a workspace
type WorkspaceActivity struct {
	ID           uuid.UUID              `json:"id" db:"id"`
	WorkspaceID  uuid.UUID              `json:"workspace_id" db:"workspace_id"`
	AgentID      string                 `json:"agent_id" db:"agent_id"`
	ActivityType string                 `json:"activity_type" db:"activity_type"`
	Description  string                 `json:"description" db:"description"`
	Details      map[string]interface{} `json:"details,omitempty" db:"details"`
	Timestamp    time.Time              `json:"timestamp" db:"timestamp"`
}

// Helper methods

// IsLocked returns true if the workspace is currently locked
func (w *Workspace) IsLocked() bool {
	if w.LockedBy == nil || w.LockExpiresAt == nil {
		return false
	}
	return time.Now().Before(*w.LockExpiresAt)
}

// CanEdit returns true if the member has edit permissions
func (m *WorkspaceMember) CanEdit() bool {
	switch m.Role {
	case MemberRoleOwner, MemberRoleAdmin, MemberRoleMember:
		return true
	default:
		return false
	}
}

// CanManage returns true if the member has management permissions
func (m *WorkspaceMember) CanManage() bool {
	switch m.Role {
	case MemberRoleOwner, MemberRoleAdmin:
		return true
	default:
		return false
	}
}

// IsActive returns true if the workspace is active
func (w *Workspace) IsActive() bool {
	return w.Status == WorkspaceStatusActive && w.DeletedAt == nil
}

// HasFeature checks if the workspace has a specific feature enabled
func (w *Workspace) HasFeature(feature string) bool {
	for _, f := range w.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// HasTag checks if the workspace has a specific tag
func (w *Workspace) HasTag(tag string) bool {
	for _, t := range w.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// SetDefaultValues sets default values for a new workspace
func (w *Workspace) SetDefaultValues() {
	if w.Status == "" {
		w.Status = WorkspaceStatusActive
	}
	if w.Visibility == "" {
		w.Visibility = WorkspaceVisibilityPrivate
	}
	// Initialize settings if empty (check a specific field)
	if w.Settings.Notifications.DigestFrequency == "" {
		w.Settings = GetDefaultWorkspaceSettings()
	}
	// Initialize limits if empty (check a specific field)
	if w.Limits.MaxMembers == 0 {
		w.Limits = GetDefaultWorkspaceLimits()
	}
	if w.Configuration == nil {
		w.Configuration = make(JSONMap)
	}
	if w.State == nil {
		w.State = make(JSONMap)
	}
	if w.Metadata == nil {
		w.Metadata = make(JSONMap)
	}
	if w.Tags == nil {
		w.Tags = pq.StringArray{}
	}
	if w.Features == nil {
		w.Features = pq.StringArray{}
	}
}

// Validate validates the workspace fields
func (w *Workspace) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("workspace name is required")
	}
	if w.TenantID == uuid.Nil {
		return fmt.Errorf("tenant ID is required")
	}
	if !w.Status.IsValid() {
		return fmt.Errorf("invalid workspace status: %s", w.Status)
	}
	return nil
}
