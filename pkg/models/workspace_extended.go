package models

import (
	"time"

	"github.com/google/uuid"
)

// WorkspaceState represents the distributed state of a workspace
type WorkspaceState struct {
	WorkspaceID    uuid.UUID              `json:"workspace_id"`
	Version        int64                  `json:"version"`
	Data           map[string]interface{} `json:"data"`
	VectorClock    map[string]int         `json:"vector_clock"`
	LastModifiedBy string                 `json:"last_modified_by"`
	LastModifiedAt time.Time              `json:"last_modified_at"`
}

// StateOperation represents an operation to modify workspace state
type StateOperation struct {
	Type  string      `json:"type"`  // set, increment, add_to_set, remove_from_set
	Path  string      `json:"path"`  // JSON path to the field
	Value interface{} `json:"value"` // Value to set/add
	Delta int         `json:"delta"` // For increment operations
}

// StateSnapshot represents a snapshot of workspace state
type StateSnapshot struct {
	ID          uuid.UUID              `json:"id"`
	WorkspaceID uuid.UUID              `json:"workspace_id"`
	Version     int64                  `json:"version"`
	State       map[string]interface{} `json:"state"`
	CreatedAt   time.Time              `json:"created_at"`
	CreatedBy   string                 `json:"created_by"`
}

// MemberActivity represents activity of a workspace member
type MemberActivity struct {
	WorkspaceID    uuid.UUID              `json:"workspace_id"`
	AgentID        string                 `json:"agent_id"`
	AgentName      string                 `json:"agent_name"`
	LastActivityAt time.Time              `json:"last_activity_at"`
	ActivityType   string                 `json:"activity_type"`
	ActivityCount  int64                  `json:"activity_count"`
	Details        map[string]interface{} `json:"details"`
}

// MemberPresence represents the presence status of a workspace member
type MemberPresence struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	AgentID     string    `json:"agent_id"`
	AgentName   string    `json:"agent_name"`
	Status      string    `json:"status"` // online, away, busy, offline
	LastSeenAt  time.Time `json:"last_seen_at"`
	Location    string    `json:"location,omitempty"` // Current location in workspace (e.g., document ID)
}

// WorkspaceStats represents statistics for a workspace
type WorkspaceStats struct {
	WorkspaceID      uuid.UUID `json:"workspace_id"`
	TotalMembers     int64     `json:"total_members"`
	ActiveMembers    int64     `json:"active_members"`
	TotalDocuments   int64     `json:"total_documents"`
	TotalOperations  int64     `json:"total_operations"`
	StorageUsedBytes int64     `json:"storage_used_bytes"`
	LastActivityAt   time.Time `json:"last_activity_at"`
}

// ConflictInfo represents information about a detected conflict
type ConflictInfo struct {
	ID            uuid.UUID              `json:"id"`
	DocumentID    uuid.UUID              `json:"document_id"`
	Type          string                 `json:"type"` // concurrent_edit, schema_mismatch, etc.
	LocalVersion  interface{}            `json:"local_version"`
	RemoteVersion interface{}            `json:"remote_version"`
	AffectedPath  string                 `json:"affected_path"`
	DetectedAt    time.Time              `json:"detected_at"`
	Metadata      map[string]interface{} `json:"metadata"`
}
