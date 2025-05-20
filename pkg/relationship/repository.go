package relationship

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Repository defines the interface for relationship data persistence
type Repository interface {
	// CreateRelationship creates a new entity relationship
	CreateRelationship(ctx context.Context, relationship *models.EntityRelationship) error
	
	// UpdateRelationship updates an existing relationship
	UpdateRelationship(ctx context.Context, relationship *models.EntityRelationship) error
	
	// DeleteRelationship removes a relationship by ID
	DeleteRelationship(ctx context.Context, relationshipID string) error
	
	// DeleteRelationshipsBetween removes all relationships between two entities
	DeleteRelationshipsBetween(ctx context.Context, source models.EntityID, target models.EntityID) error
	
	// GetRelationship retrieves a relationship by ID
	GetRelationship(ctx context.Context, relationshipID string) (*models.EntityRelationship, error)
	
	// GetDirectRelationships gets direct relationships for an entity
	GetDirectRelationships(
		ctx context.Context,
		entityID models.EntityID,
		direction string,
		relTypes []models.RelationshipType,
	) ([]*models.EntityRelationship, error)
	
	// GetRelationshipsByType gets relationships of a specific type
	GetRelationshipsByType(
		ctx context.Context,
		relType models.RelationshipType,
		limit int,
		offset int,
	) ([]*models.EntityRelationship, error)
}
