// Package relationship provides a compatibility layer for the pkg/models/relationship package.
// This package is part of the Go Workspace migration to ensure backward compatibility
// with code still importing the old pkg/relationship package path.
package relationship

import (
	"time"
)

// Relationship represents a relationship between two entities
type Relationship struct {
	ID           string          `json:"id" db:"id"`
	SourceID     string          `json:"source_id" db:"source_id"`
	SourceType   EntityType      `json:"source_type" db:"source_type"`
	TargetID     string          `json:"target_id" db:"target_id"`
	TargetType   EntityType      `json:"target_type" db:"target_type"`
	Type         RelationshipType `json:"type" db:"type"`
	CreatedAt    int64           `json:"created_at" db:"created_at"`
	CreatedBy    string          `json:"created_by" db:"created_by"`
	TenantID     string          `json:"tenant_id" db:"tenant_id"`
	Metadata     interface{}     `json:"metadata" db:"metadata"`
}

// RelationshipType represents the type of a relationship
type RelationshipType string

// EntityType represents the type of an entity
type EntityType string

// Filter is used to filter relationships
type Filter struct {
	SourceID   string
	TargetID   string
	SourceType EntityType
	TargetType EntityType
	Type       RelationshipType
	TenantID   string
}

// Constants
const (
	// Entity types
	EntityTypeAgent   EntityType = "agent"
	EntityTypeModel   EntityType = "model"
	EntityTypeContext EntityType = "context"
	EntityTypeMemory  EntityType = "memory"
	EntityTypeUser    EntityType = "user"
	EntityTypeUnknown EntityType = "unknown"
	
	// Relationship types
	RelationshipTypeCreatedBy      RelationshipType = "created_by"
	RelationshipTypeHas            RelationshipType = "has"
	RelationshipTypeContains       RelationshipType = "contains"
	RelationshipTypeAssociatedWith RelationshipType = "associated_with"
	RelationshipTypeParentOf       RelationshipType = "parent_of"
	RelationshipTypeChildOf        RelationshipType = "child_of"
	RelationshipTypeUnknown        RelationshipType = "unknown"
)

// Function implementations for backward compatibility

// NewRelationship creates a new relationship
func NewRelationship(sourceID string, sourceType EntityType, targetID string, targetType EntityType, relType RelationshipType, tenantID string, metadata interface{}) *Relationship {
	return &Relationship{
		SourceID:   sourceID,
		SourceType: sourceType,
		TargetID:   targetID,
		TargetType: targetType,
		Type:       relType,
		TenantID:   tenantID,
		Metadata:   metadata,
		CreatedAt:  time.Now().Unix(),
	}
}

// FilterFromSourceID creates a filter for relationships with the given source ID
func FilterFromSourceID(sourceID string) *Filter {
	return &Filter{
		SourceID: sourceID,
	}
}

// FilterFromTargetID creates a filter for relationships with the given target ID
func FilterFromTargetID(targetID string) *Filter {
	return &Filter{
		TargetID: targetID,
	}
}

// FilterFromEntityIDs creates a filter for relationships with the given source and target IDs
func FilterFromEntityIDs(sourceID, targetID string) *Filter {
	return &Filter{
		SourceID: sourceID,
		TargetID: targetID,
	}
}

// FilterFromType creates a filter for relationships with the given type
func FilterFromType(relType RelationshipType) *Filter {
	return &Filter{
		Type: relType,
	}
}

// ParseEntityType parses an entity type from a string
func ParseEntityType(entityType string) EntityType {
	switch entityType {
	case string(EntityTypeAgent):
		return EntityTypeAgent
	case string(EntityTypeModel):
		return EntityTypeModel
	case string(EntityTypeContext):
		return EntityTypeContext
	case string(EntityTypeMemory):
		return EntityTypeMemory
	case string(EntityTypeUser):
		return EntityTypeUser
	default:
		return EntityTypeUnknown
	}
}

// ParseRelationshipType parses a relationship type from a string
func ParseRelationshipType(relType string) RelationshipType {
	switch relType {
	case string(RelationshipTypeCreatedBy):
		return RelationshipTypeCreatedBy
	case string(RelationshipTypeHas):
		return RelationshipTypeHas
	case string(RelationshipTypeContains):
		return RelationshipTypeContains
	case string(RelationshipTypeAssociatedWith):
		return RelationshipTypeAssociatedWith
	case string(RelationshipTypeParentOf):
		return RelationshipTypeParentOf
	case string(RelationshipTypeChildOf):
		return RelationshipTypeChildOf
	default:
		return RelationshipTypeUnknown
	}
}
