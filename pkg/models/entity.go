package models

import (
	"fmt"
	"time"
)

// EntityType represents the type of entity
type EntityType string

// Common entity types
const (
	EntityTypeRepository EntityType = "REPOSITORY"
	EntityTypeIssue      EntityType = "ISSUE"
	EntityTypePullRequest EntityType = "PULL_REQUEST"
	EntityTypeCommit     EntityType = "COMMIT"
	EntityTypeFile       EntityType = "FILE"
	EntityTypeUser       EntityType = "USER"
	EntityTypeComment    EntityType = "COMMENT"
)

// EntityID represents a unique identifier for an entity
type EntityID struct {
	Type  EntityType `json:"type"`
	Owner string     `json:"owner"`
	Repo  string     `json:"repo"`
	ID    string     `json:"id"`
}

// String returns a string representation of the entity ID
func (e EntityID) String() string {
	return fmt.Sprintf("%s:%s/%s:%s", e.Type, e.Owner, e.Repo, e.ID)
}

// NewEntityID creates a new entity ID with the given components
func NewEntityID(entityType EntityType, owner string, repo string, id string) EntityID {
	return EntityID{
		Type:  entityType,
		Owner: owner,
		Repo:  repo,
		ID:    id,
	}
}

// EntityIDFromContentMetadata creates an EntityID from content metadata
func EntityIDFromContentMetadata(contentType string, owner string, repo string, contentID string) EntityID {
	// Map the content type to an entity type
	var entityType EntityType
	switch contentType {
	case "repository":
		entityType = EntityTypeRepository
	case "issue":
		entityType = EntityTypeIssue
	case "pull_request":
		entityType = EntityTypePullRequest
	case "commit":
		entityType = EntityTypeCommit
	case "file":
		entityType = EntityTypeFile
	case "user":
		entityType = EntityTypeUser
	case "comment":
		entityType = EntityTypeComment
	default:
		// Use a default or the content type as is
		entityType = EntityType(contentType)
	}
	
	return EntityID{
		Type:  entityType,
		Owner: owner,
		Repo:  repo,
		ID:    contentID,
	}
}

// RelationshipType defines the type of relationship between entities
type RelationshipType string

// Common relationship types
const (
	RelationshipTypeOwns       RelationshipType = "OWNS"
	RelationshipTypeContains   RelationshipType = "CONTAINS"
	RelationshipTypeReferences RelationshipType = "REFERENCES"
	RelationshipTypeDependsOn  RelationshipType = "DEPENDS_ON"
	RelationshipTypeAssociates RelationshipType = "ASSOCIATES"
	RelationshipTypeCreates    RelationshipType = "CREATES"
	RelationshipTypeModifies   RelationshipType = "MODIFIES"
	RelationshipTypeImplements RelationshipType = "IMPLEMENTS"
	RelationshipTypeExtends    RelationshipType = "EXTENDS"
	RelationshipTypeReplaces   RelationshipType = "REPLACES"
	RelationshipTypeComments   RelationshipType = "COMMENTS"
)

// Direction constants for relationships
const (
	DirectionOutgoing    string = "OUTGOING"
	DirectionIncoming    string = "INCOMING"
	DirectionBidirectional string = "BIDIRECTIONAL"
)

// EntityRelationship represents a relationship between two entities
type EntityRelationship struct {
	ID          string           `json:"id"`
	Source      EntityID         `json:"source"`
	Target      EntityID         `json:"target"`
	Type        RelationshipType `json:"type"`
	Properties  map[string]any   `json:"properties,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Direction   string           `json:"direction,omitempty"`
	Metadata    map[string]any   `json:"metadata,omitempty"`
	Context     map[string]any   `json:"context,omitempty"`
	Strength    float64          `json:"strength,omitempty"`
}

// NewEntityRelationship creates a new entity relationship
func NewEntityRelationship(relType RelationshipType, source EntityID, target EntityID, direction string, strength float64) *EntityRelationship {
	now := time.Now()
	id := GenerateRelationshipID(source, target, relType)
	return &EntityRelationship{
		ID:         id,
		Source:     source,
		Target:     target,
		Type:       relType,
		Direction:  direction,
		Strength:   strength,
		Properties: make(map[string]any),
		Metadata:   make(map[string]any),
		Context:    make(map[string]any),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// WithContext adds context to the relationship
// It can be called with either a key-value pair or a complete map
func (r *EntityRelationship) WithContext(keyOrMap interface{}, value ...interface{}) *EntityRelationship {
	if r.Context == nil {
		r.Context = make(map[string]any)
	}
	
	// Check if we're adding a complete map
	if m, ok := keyOrMap.(map[string]interface{}); ok && len(value) == 0 {
		for k, v := range m {
			r.Context[k] = v
		}
		return r
	}
	
	// Otherwise treat it as a key-value pair
	if key, ok := keyOrMap.(string); ok && len(value) > 0 {
		r.Context[key] = value[0]
	}
	return r
}

// WithMetadata adds metadata to the relationship
// It can be called with either a key-value pair or a complete map
func (r *EntityRelationship) WithMetadata(keyOrMap interface{}, value ...interface{}) *EntityRelationship {
	if r.Metadata == nil {
		r.Metadata = make(map[string]any)
	}
	
	// Check if we're adding a complete map
	if m, ok := keyOrMap.(map[string]interface{}); ok && len(value) == 0 {
		for k, v := range m {
			r.Metadata[k] = v
		}
		return r
	}
	
	// Otherwise treat it as a key-value pair
	if key, ok := keyOrMap.(string); ok && len(value) > 0 {
		r.Metadata[key] = value[0]
	}
	return r
}

// GenerateRelationshipID generates a unique ID for a relationship
// Takes either (source, target, relType) or (relType, source, target, direction) - the latter ignores direction
func GenerateRelationshipID(arg1 interface{}, arg2 interface{}, arg3 interface{}, arg4 ...interface{}) string {
	var source EntityID
	var target EntityID
	var relType RelationshipType

	// Handle the original signature: (source, target, relType)
	if src, ok := arg1.(EntityID); ok {
		if tgt, ok := arg2.(EntityID); ok {
			if rt, ok := arg3.(RelationshipType); ok {
				source = src
				target = tgt
				relType = rt
			}
		}
	}

	// Handle alternative signature: (relType, source, target, direction)
	if rt, ok := arg1.(RelationshipType); ok {
		if src, ok := arg2.(EntityID); ok {
			if tgt, ok := arg3.(EntityID); ok {
				source = src
				target = tgt
				relType = rt
			}
		}
	}

	return fmt.Sprintf("%s-%s-%s", source, relType, target)
}
