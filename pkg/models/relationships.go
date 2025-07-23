// Package models provides common data models used across the developer-mesh workspace.
// It contains definitions for entity relationships and connection models that
// represent relationships between various GitHub entities.
//
// This package is part of the Go workspace migration and provides a standardized
// location for all shared model definitions.
package models

import (
	"fmt"
	"time"
)

// EntityType represents the type of GitHub entity
type EntityType string

// RelationshipType represents the type of relationship between entities
type RelationshipType string

// Relationship directions
const (
	// DirectionOutgoing represents a relationship from source to target
	DirectionOutgoing = "outgoing"
	// DirectionIncoming represents a relationship from target to source
	DirectionIncoming = "incoming"
	// DirectionBidirectional represents a bidirectional relationship
	DirectionBidirectional = "bidirectional"
)

// Entity types
const (
	// EntityTypeRepository represents a GitHub repository
	EntityTypeRepository EntityType = "repository"
	// EntityTypeIssue represents a GitHub issue
	EntityTypeIssue EntityType = "issue"
	// EntityTypePullRequest represents a GitHub pull request
	EntityTypePullRequest EntityType = "pull_request"
	// EntityTypeCommit represents a GitHub commit
	EntityTypeCommit EntityType = "commit"
	// EntityTypeFile represents a file in a GitHub repository
	EntityTypeFile EntityType = "file"
	// EntityTypeUser represents a GitHub user
	EntityTypeUser EntityType = "user"
	// EntityTypeOrganization represents a GitHub organization
	EntityTypeOrganization EntityType = "organization"
	// EntityTypeDiscussion represents a GitHub discussion
	EntityTypeDiscussion EntityType = "discussion"
	// EntityTypeComment represents a comment on an issue, PR, or discussion
	EntityTypeComment EntityType = "comment"
	// EntityTypeRelease represents a GitHub release
	EntityTypeRelease EntityType = "release"
	// EntityTypeCodeChunk represents a semantic code chunk
	EntityTypeCodeChunk EntityType = "code_chunk"
)

// Relationship types
const (
	// RelationshipTypeReferences indicates an entity references another entity
	RelationshipTypeReferences RelationshipType = "references"
	// RelationshipTypeContains indicates an entity contains another entity
	RelationshipTypeContains RelationshipType = "contains"
	// RelationshipTypeCreates indicates an entity created another entity
	RelationshipTypeCreates RelationshipType = "creates"
	// RelationshipTypeModifies indicates an entity modified another entity
	RelationshipTypeModifies RelationshipType = "modifies"
	// RelationshipTypeAssociates indicates a general association between entities
	RelationshipTypeAssociates RelationshipType = "associates"
	// RelationshipTypeDependsOn indicates an entity depends on another entity
	RelationshipTypeDependsOn RelationshipType = "depends_on"
	// RelationshipTypeImplements indicates an entity implements another entity
	RelationshipTypeImplements RelationshipType = "implements"
	// RelationshipTypeExtends indicates an entity extends another entity
	RelationshipTypeExtends RelationshipType = "extends"
	// RelationshipTypeReplaces indicates an entity replaces another entity
	RelationshipTypeReplaces RelationshipType = "replaces"
	// RelationshipTypeComments indicates an entity is a comment on another entity
	RelationshipTypeComments RelationshipType = "comments"
)

// EntityID represents a unique identifier for an entity
type EntityID struct {
	// Type of the entity
	Type EntityType `json:"type"`

	// Owner of the entity (GitHub username or organization)
	Owner string `json:"owner"`

	// Repository name
	Repo string `json:"repo"`

	// Identifier of the entity (issue number, PR number, commit hash, etc.)
	ID string `json:"id"`

	// Additional qualifiers for uniquely identifying the entity
	Qualifiers map[string]string `json:"qualifiers,omitempty"`
}

// EntityRelationship represents a relationship between two GitHub entities
type EntityRelationship struct {
	// ID is a unique identifier for this relationship
	ID string `json:"id"`

	// Type of relationship
	Type RelationshipType `json:"type"`

	// Direction of the relationship (outgoing, incoming, bidirectional)
	Direction string `json:"direction"`

	// Source entity
	Source EntityID `json:"source"`

	// Target entity
	Target EntityID `json:"target"`

	// Strength of the relationship (0.0 to 1.0)
	Strength float64 `json:"strength"`

	// Context provides additional information about the relationship
	Context string `json:"context,omitempty"`

	// Metadata contains additional structured data about the relationship
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is the timestamp when the relationship was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the timestamp when the relationship was last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// NewEntityID creates a new EntityID
func NewEntityID(entityType EntityType, owner, repo, id string) EntityID {
	return EntityID{
		Type:  entityType,
		Owner: owner,
		Repo:  repo,
		ID:    id,
	}
}

// WithQualifiers adds qualifiers to an EntityID
func (e EntityID) WithQualifiers(qualifiers map[string]string) EntityID {
	e.Qualifiers = qualifiers
	return e
}

// EntityIDFromContentMetadata creates an EntityID from storage content metadata
func EntityIDFromContentMetadata(contentType string, owner, repo, contentID string) EntityID {
	var entityType EntityType

	// Map storage content type to entity type
	switch contentType {
	case "issue":
		entityType = EntityTypeIssue
	case "pull_request":
		entityType = EntityTypePullRequest
	case "commit":
		entityType = EntityTypeCommit
	case "file":
		entityType = EntityTypeFile
	case "release":
		entityType = EntityTypeRelease
	case "code_chunk":
		entityType = EntityTypeCodeChunk
	case "comment":
		entityType = EntityTypeComment
	case "repository":
		entityType = EntityTypeRepository
	case "user":
		entityType = EntityTypeUser
	case "organization":
		entityType = EntityTypeOrganization
	case "discussion":
		entityType = EntityTypeDiscussion
	default:
		// Default fallback
		entityType = EntityType(contentType)
	}

	return NewEntityID(entityType, owner, repo, contentID)
}

// NewEntityRelationship creates a new relationship between two entities
func NewEntityRelationship(
	relType RelationshipType,
	source EntityID,
	target EntityID,
	direction string,
	strength float64,
) *EntityRelationship {
	now := time.Now()

	// Validate direction
	if direction != DirectionOutgoing &&
		direction != DirectionIncoming &&
		direction != DirectionBidirectional {
		direction = DirectionOutgoing
	}

	// Generate a deterministic ID for the relationship
	id := GenerateRelationshipID(relType, source, target, direction)

	return &EntityRelationship{
		ID:        id,
		Type:      relType,
		Source:    source,
		Target:    target,
		Direction: direction,
		Strength:  strength,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]any),
	}
}

// WithContext adds context to a relationship
func (r *EntityRelationship) WithContext(context string) *EntityRelationship {
	r.Context = context
	return r
}

// WithMetadata adds metadata to a relationship
func (r *EntityRelationship) WithMetadata(metadata map[string]any) *EntityRelationship {
	if r.Metadata == nil {
		r.Metadata = make(map[string]any)
	}

	for k, v := range metadata {
		r.Metadata[k] = v
	}

	return r
}

// GenerateRelationshipID creates a deterministic ID for a relationship
func GenerateRelationshipID(
	relType RelationshipType,
	source EntityID,
	target EntityID,
	direction string,
) string {
	// Format: {source.Type}:{source.Owner}/{source.Repo}/{source.ID}-{relType}:{direction}-{target.Type}:{target.Owner}/{target.Repo}/{target.ID}
	return fmt.Sprintf("%s:%s/%s/%s-%s:%s-%s:%s/%s/%s",
		source.Type, source.Owner, source.Repo, source.ID,
		relType, direction,
		target.Type, target.Owner, target.Repo, target.ID)
}
