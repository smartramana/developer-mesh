package models

import (
	"time"

	"github.com/google/uuid"
)

// DocumentVersion represents a specific version of a document
type DocumentVersion struct {
	ID         uuid.UUID `json:"id" db:"id"`
	DocumentID uuid.UUID `json:"document_id" db:"document_id"`
	Version    int       `json:"version" db:"version"`
	Content    string    `json:"content" db:"content"`
	CreatedBy  string    `json:"created_by" db:"created_by"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	Metadata   JSONMap   `json:"metadata" db:"metadata"`
}

// DocumentDiff represents the difference between two document versions
type DocumentDiff struct {
	FromVersion int       `json:"from_version"`
	ToVersion   int       `json:"to_version"`
	Changes     []Change  `json:"changes"`
	CreatedAt   time.Time `json:"created_at"`
}

// Change represents a single change in a document
type Change struct {
	Type     string `json:"type"` // "add", "remove", "modify"
	Path     string `json:"path"`
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// DocumentStats represents statistics about a document
type DocumentStats struct {
	DocumentID    uuid.UUID `json:"document_id"`
	TotalVersions int       `json:"total_versions"`
	TotalEdits    int       `json:"total_edits"`
	UniqueEditors int       `json:"unique_editors"`
	LastEditedAt  time.Time `json:"last_edited_at"`
	LastEditedBy  string    `json:"last_edited_by"`
	ContentLength int       `json:"content_length"`
	CreatedAt     time.Time `json:"created_at"`
}

// WorkspaceCollaborator represents a collaborator in a workspace
type WorkspaceCollaborator struct {
	ID          uuid.UUID `json:"id" db:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id" db:"workspace_id"`
	AgentID     string    `json:"agent_id" db:"agent_id"`
	Role        string    `json:"role" db:"role"` // "owner", "editor", "viewer"
	Permissions []string  `json:"permissions" db:"permissions"`
	AddedAt     time.Time `json:"added_at" db:"added_at"`
	AddedBy     string    `json:"added_by" db:"added_by"`
}
