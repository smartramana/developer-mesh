// Package models provides backward compatibility for code that still imports
// from github.com/S-Corkum/devops-mcp/internal/models.
//
// Deprecated: This package is being migrated to github.com/S-Corkum/devops-mcp/pkg/models
// as part of the Go workspace migration. Please update your imports to use the new path.
package models

import (
	"github.com/S-Corkum/devops-mcp/pkg/feature"
	newmodels "github.com/S-Corkum/devops-mcp/pkg/models"
)

var (
	// Re-export relationship directions
	DirectionOutgoing      = newmodels.DirectionOutgoing
	DirectionIncoming      = newmodels.DirectionIncoming
	DirectionBidirectional = newmodels.DirectionBidirectional

	// Re-export entity types
	EntityTypeRepository   = newmodels.EntityTypeRepository
	EntityTypeIssue        = newmodels.EntityTypeIssue
	EntityTypePullRequest  = newmodels.EntityTypePullRequest
	EntityTypeCommit       = newmodels.EntityTypeCommit
	EntityTypeFile         = newmodels.EntityTypeFile
	EntityTypeUser         = newmodels.EntityTypeUser
	EntityTypeOrganization = newmodels.EntityTypeOrganization
	EntityTypeDiscussion   = newmodels.EntityTypeDiscussion
	EntityTypeComment      = newmodels.EntityTypeComment
	EntityTypeRelease      = newmodels.EntityTypeRelease
	EntityTypeCodeChunk    = newmodels.EntityTypeCodeChunk

	// Re-export relationship types
	RelationshipTypeReferences  = newmodels.RelationshipTypeReferences
	RelationshipTypeContains    = newmodels.RelationshipTypeContains
	RelationshipTypeCreates     = newmodels.RelationshipTypeCreates
	RelationshipTypeModifies    = newmodels.RelationshipTypeModifies
	RelationshipTypeAssociates  = newmodels.RelationshipTypeAssociates
	RelationshipTypeDependsOn   = newmodels.RelationshipTypeDependsOn
	RelationshipTypeImplements  = newmodels.RelationshipTypeImplements
	RelationshipTypeExtends     = newmodels.RelationshipTypeExtends
	RelationshipTypeReplaces    = newmodels.RelationshipTypeReplaces
	RelationshipTypeComments    = newmodels.RelationshipTypeComments
)

// Type re-exports
type (
	EntityType         = newmodels.EntityType
	RelationshipType   = newmodels.RelationshipType
	EntityID           = newmodels.EntityID
	EntityRelationship = newmodels.EntityRelationship
)

// NewEntityID creates a new EntityID
func NewEntityID(entityType EntityType, owner, repo, id string) EntityID {
	// Log deprecation warning if feature flag is enabled
	if feature.IsEnabled("LOG_DEPRECATION_WARNINGS") {
		// In a real implementation, this would use a logger
		// log.Warn("Deprecated: Using internal/models.NewEntityID - use pkg/models.NewEntityID instead")
	}
	return newmodels.NewEntityID(entityType, owner, repo, id)
}

// EntityIDFromContentMetadata creates an EntityID from storage content metadata
func EntityIDFromContentMetadata(contentType string, owner, repo, contentID string) EntityID {
	return newmodels.EntityIDFromContentMetadata(contentType, owner, repo, contentID)
}

// NewEntityRelationship creates a new relationship between two entities
func NewEntityRelationship(
	relType RelationshipType,
	source EntityID,
	target EntityID,
	direction string,
	strength float64,
) *EntityRelationship {
	return newmodels.NewEntityRelationship(relType, source, target, direction, strength)
}

// GenerateRelationshipID creates a deterministic ID for a relationship
func GenerateRelationshipID(
	relType RelationshipType,
	source EntityID,
	target EntityID,
	direction string,
) string {
	return newmodels.GenerateRelationshipID(relType, source, target, direction)
}
