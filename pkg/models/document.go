package models

import (
	"time"

	"github.com/google/uuid"
)

// SharedDocument represents a collaborative document
type SharedDocument struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	WorkspaceID   uuid.UUID  `json:"workspace_id" db:"workspace_id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	Type          string     `json:"type" db:"type"`
	Title         string     `json:"title" db:"title"`
	Content       string     `json:"content" db:"content"`
	ContentType   string     `json:"content_type" db:"content_type"`
	Version       int64      `json:"version" db:"version"`
	CreatedBy     string     `json:"created_by" db:"created_by"`
	Metadata      JSONMap    `json:"metadata" db:"metadata"`
	LockedBy      *string    `json:"locked_by,omitempty" db:"locked_by"`
	LockedAt      *time.Time `json:"locked_at,omitempty" db:"locked_at"`
	LockExpiresAt *time.Time `json:"lock_expires_at,omitempty" db:"lock_expires_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`

	// Runtime data
	Operations    []*DocumentOperation `json:"operations,omitempty" db:"-"`
	Collaborators []string             `json:"collaborators,omitempty" db:"-"`
}

// DocumentOperation represents a CRDT operation on a document
type DocumentOperation struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	DocumentID        uuid.UUID  `json:"document_id" db:"document_id"`
	TenantID          uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	AgentID           string     `json:"agent_id" db:"agent_id"`
	OperationType     string     `json:"operation_type" db:"operation_type"`
	OperationData     JSONMap    `json:"operation_data" db:"operation_data"`
	VectorClock       JSONMap    `json:"vector_clock" db:"vector_clock"`
	SequenceNumber    int64      `json:"sequence_number" db:"sequence_number"`
	Timestamp         time.Time  `json:"timestamp" db:"timestamp"`
	ParentOperationID *uuid.UUID `json:"parent_operation_id,omitempty" db:"parent_operation_id"`
	IsApplied         bool       `json:"is_applied" db:"is_applied"`
}

// DocumentLock represents lock information for a document
type DocumentLock struct {
	DocumentID    uuid.UUID `json:"document_id"`
	LockedBy      string    `json:"locked_by"`
	LockedAt      time.Time `json:"locked_at"`
	LockExpiresAt time.Time `json:"lock_expires_at"`
	LockType      string    `json:"lock_type"` // "exclusive" or "shared"
}

// DocumentSnapshot represents a point-in-time snapshot of a document
type DocumentSnapshot struct {
	ID          uuid.UUID `json:"id" db:"id"`
	DocumentID  uuid.UUID `json:"document_id" db:"document_id"`
	Version     int64     `json:"version" db:"version"`
	Content     string    `json:"content" db:"content"`
	VectorClock JSONMap   `json:"vector_clock" db:"vector_clock"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	CreatedBy   string    `json:"created_by" db:"created_by"`
}

// ConflictResolution represents a resolved conflict in document operations
type ConflictResolution struct {
	ID                 uuid.UUID  `json:"id" db:"id"`
	TenantID           uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	ResourceType       string     `json:"resource_type" db:"resource_type"`
	ResourceID         uuid.UUID  `json:"resource_id" db:"resource_id"`
	ConflictType       string     `json:"conflict_type" db:"conflict_type"`
	Description        string     `json:"description,omitempty" db:"description"`
	ResolutionStrategy string     `json:"resolution_strategy,omitempty" db:"resolution_strategy"`
	Details            JSONMap    `json:"details" db:"details"`
	ResolvedBy         *string    `json:"resolved_by,omitempty" db:"resolved_by"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
}

// CollaborationMetrics represents metrics for document collaboration
type CollaborationMetrics struct {
	DocumentID          uuid.UUID        `json:"document_id"`
	Period              time.Duration    `json:"period"`
	UniqueCollaborators int              `json:"unique_collaborators"`
	TotalOperations     int64            `json:"total_operations"`
	OperationsByType    map[string]int64 `json:"operations_by_type"`
	ConflictCount       int64            `json:"conflict_count"`
	AverageResponseTime time.Duration    `json:"average_response_time"`
	PeakConcurrency     int              `json:"peak_concurrency"`
}

// Helper methods

// IsLocked returns true if the document is currently locked
func (d *SharedDocument) IsLocked() bool {
	if d.LockedBy == nil || d.LockExpiresAt == nil {
		return false
	}
	return time.Now().Before(*d.LockExpiresAt)
}

// CanEdit returns true if the agent can edit the document
func (d *SharedDocument) CanEdit(agentID string) bool {
	// If locked, only lock owner can edit
	if d.IsLocked() && d.LockedBy != nil && *d.LockedBy != agentID {
		return false
	}
	return true
}

// IsConflict checks if this operation conflicts with another
func (op *DocumentOperation) IsConflict(other *DocumentOperation) bool {
	// Simple conflict detection - operations on same position
	if op.OperationType == other.OperationType {
		return false
	}

	// Check if operations affect same region
	// This is simplified - real implementation would check operation data
	return true
}
