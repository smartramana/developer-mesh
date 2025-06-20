package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DocumentType represents the type of document
type DocumentType string

const (
	DocumentTypeMarkdown DocumentType = "markdown"
	DocumentTypeJSON     DocumentType = "json"
	DocumentTypeYAML     DocumentType = "yaml"
	DocumentTypeCode     DocumentType = "code"
	DocumentTypeDiagram  DocumentType = "diagram"
	DocumentTypeRunbook  DocumentType = "runbook"
	DocumentTypePlaybook DocumentType = "playbook"
	DocumentTypeTemplate DocumentType = "template"
	DocumentTypeConfig   DocumentType = "config"
)

// DocumentUpdate represents a document update operation
type DocumentUpdate struct {
	ID                 uuid.UUID                  `json:"id" db:"id"`
	DocumentID         uuid.UUID                  `json:"document_id" db:"document_id"`
	Version            int                        `json:"version" db:"version"`
	UpdateType         UpdateType                 `json:"update_type" db:"update_type"`
	Path               string                     `json:"path" db:"path"`
	OldValue           interface{}                `json:"old_value" db:"old_value"`
	NewValue           interface{}                `json:"new_value" db:"new_value"`
	UpdatedBy          string                     `json:"updated_by" db:"updated_by"`
	UpdatedAt          time.Time                  `json:"updated_at" db:"updated_at"`
	Metadata           map[string]interface{}     `json:"metadata" db:"metadata"`
	Checksum           string                     `json:"checksum" db:"checksum"`
	ConflictResolution DocumentConflictResolution `json:"conflict_resolution,omitempty" db:"conflict_resolution"`
}

type UpdateType string

const (
	UpdateTypeInsert  UpdateType = "insert"
	UpdateTypeDelete  UpdateType = "delete"
	UpdateTypeReplace UpdateType = "replace"
	UpdateTypeMove    UpdateType = "move"
	UpdateTypeMerge   UpdateType = "merge"
)

type DocumentConflictResolution struct {
	Strategy      ConflictStrategy `json:"strategy"`
	ResolvedBy    string           `json:"resolved_by"`
	ResolvedAt    time.Time        `json:"resolved_at"`
	OriginalValue interface{}      `json:"original_value,omitempty"`
}

type ConflictStrategy string

const (
	ConflictStrategyLatestWins ConflictStrategy = "latest_wins"
	ConflictStrategyMerge      ConflictStrategy = "merge"
	ConflictStrategyManual     ConflictStrategy = "manual"
	ConflictStrategyCustom     ConflictStrategy = "custom"
)

// WorkspaceMemberRole defines access levels
type WorkspaceMemberRole string

const (
	WorkspaceMemberRoleOwner     WorkspaceMemberRole = "owner"
	WorkspaceMemberRoleAdmin     WorkspaceMemberRole = "admin"
	WorkspaceMemberRoleEditor    WorkspaceMemberRole = "editor"
	WorkspaceMemberRoleCommenter WorkspaceMemberRole = "commenter"
	WorkspaceMemberRoleViewer    WorkspaceMemberRole = "viewer"
	WorkspaceMemberRoleGuest     WorkspaceMemberRole = "guest"
)

// RolePermissions defines what each role can do
var RolePermissions = map[WorkspaceMemberRole][]string{
	WorkspaceMemberRoleOwner:     {"*"}, // All permissions
	WorkspaceMemberRoleAdmin:     {"read", "write", "delete", "invite", "settings"},
	WorkspaceMemberRoleEditor:    {"read", "write", "comment"},
	WorkspaceMemberRoleCommenter: {"read", "comment"},
	WorkspaceMemberRoleViewer:    {"read"},
	WorkspaceMemberRoleGuest:     {"read:public"},
}

// HasPermission checks if a role has a specific permission
func (r WorkspaceMemberRole) HasPermission(permission string) bool {
	perms, exists := RolePermissions[r]
	if !exists {
		return false
	}
	for _, p := range perms {
		if p == "*" || p == permission {
			return true
		}
	}
	return false
}

// CanTransitionTo checks if a role can be changed to another role
func (r WorkspaceMemberRole) CanTransitionTo(target WorkspaceMemberRole) bool {
	// Only owner and admin can change roles
	if r != WorkspaceMemberRoleOwner && r != WorkspaceMemberRoleAdmin {
		return false
	}

	// Owner can change to any role
	if r == WorkspaceMemberRoleOwner {
		return true
	}

	// Admin can't promote to owner
	if r == WorkspaceMemberRoleAdmin && target == WorkspaceMemberRoleOwner {
		return false
	}

	return true
}

// IsHigherThan checks if this role has more privileges than another
func (r WorkspaceMemberRole) IsHigherThan(other WorkspaceMemberRole) bool {
	hierarchy := map[WorkspaceMemberRole]int{
		WorkspaceMemberRoleOwner:     6,
		WorkspaceMemberRoleAdmin:     5,
		WorkspaceMemberRoleEditor:    4,
		WorkspaceMemberRoleCommenter: 3,
		WorkspaceMemberRoleViewer:    2,
		WorkspaceMemberRoleGuest:     1,
	}

	rLevel, rExists := hierarchy[r]
	oLevel, oExists := hierarchy[other]

	if !rExists || !oExists {
		return false
	}

	return rLevel > oLevel
}

// Validate ensures the document type is valid
func (d DocumentType) Validate() error {
	switch d {
	case DocumentTypeMarkdown, DocumentTypeJSON, DocumentTypeYAML,
		DocumentTypeCode, DocumentTypeDiagram, DocumentTypeRunbook,
		DocumentTypePlaybook, DocumentTypeTemplate, DocumentTypeConfig:
		return nil
	default:
		return fmt.Errorf("invalid document type: %s", d)
	}
}

// Validate ensures the update type is valid
func (u UpdateType) Validate() error {
	switch u {
	case UpdateTypeInsert, UpdateTypeDelete, UpdateTypeReplace,
		UpdateTypeMove, UpdateTypeMerge:
		return nil
	default:
		return fmt.Errorf("invalid update type: %s", u)
	}
}

// Validate ensures the conflict strategy is valid
func (c ConflictStrategy) Validate() error {
	switch c {
	case ConflictStrategyLatestWins, ConflictStrategyMerge,
		ConflictStrategyManual, ConflictStrategyCustom:
		return nil
	default:
		return fmt.Errorf("invalid conflict strategy: %s", c)
	}
}

// Validate ensures the workspace member role is valid
func (w WorkspaceMemberRole) Validate() error {
	switch w {
	case WorkspaceMemberRoleOwner, WorkspaceMemberRoleAdmin,
		WorkspaceMemberRoleEditor, WorkspaceMemberRoleCommenter,
		WorkspaceMemberRoleViewer, WorkspaceMemberRoleGuest:
		return nil
	default:
		return fmt.Errorf("invalid workspace member role: %s", w)
	}
}
