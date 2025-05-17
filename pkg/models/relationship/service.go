package relationship

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Service defines the interface for managing entity relationships
type Service interface {
	// CreateRelationship creates a new relationship between entities
	CreateRelationship(ctx context.Context, relationship *models.EntityRelationship) error
	
	// CreateBidirectionalRelationship creates bidirectional relationships between entities
	CreateBidirectionalRelationship(
		ctx context.Context, 
		relType models.RelationshipType,
		source models.EntityID,
		target models.EntityID,
		strength float64,
		metadata map[string]interface{},
	) error
	
	// DeleteRelationship removes a relationship by ID
	DeleteRelationship(ctx context.Context, relationshipID string) error
	
	// DeleteRelationshipsBetween removes all relationships between two entities
	DeleteRelationshipsBetween(
		ctx context.Context,
		source models.EntityID,
		target models.EntityID,
	) error
	
	// GetRelationship retrieves a relationship by ID
	GetRelationship(ctx context.Context, relationshipID string) (*models.EntityRelationship, error)
	
	// GetDirectRelationships gets direct relationships for an entity
	GetDirectRelationships(
		ctx context.Context, 
		entityID models.EntityID,
		direction string,
		relTypes []models.RelationshipType,
	) ([]*models.EntityRelationship, error)
	
	// GetRelatedEntities gets entities related to the specified entity
	GetRelatedEntities(
		ctx context.Context,
		entityID models.EntityID,
		relTypes []models.RelationshipType,
		maxDepth int,
	) ([]models.EntityID, error)
	
	// GetRelationshipGraph gets the relationship graph for an entity up to a specified depth
	GetRelationshipGraph(
		ctx context.Context,
		entityID models.EntityID,
		maxDepth int,
	) ([]*models.EntityRelationship, error)
}

// NewService creates a new relationship service with the specified repository
func NewService(repo Repository) Service {
	return &serviceImpl{
		repo: repo,
	}
}

// serviceImpl implements the Service interface
type serviceImpl struct {
	repo Repository
}

// CreateRelationship creates a new relationship between entities
func (s *serviceImpl) CreateRelationship(ctx context.Context, relationship *models.EntityRelationship) error {
	// Check if relationship already exists
	existing, err := s.GetRelationship(ctx, relationship.ID)
	if err == nil && existing != nil {
		// Relationship exists, update instead
		return s.repo.UpdateRelationship(ctx, relationship)
	}
	
	// Create new relationship
	return s.repo.CreateRelationship(ctx, relationship)
}

// CreateBidirectionalRelationship creates bidirectional relationships between entities
func (s *serviceImpl) CreateBidirectionalRelationship(
	ctx context.Context,
	relType models.RelationshipType,
	source models.EntityID,
	target models.EntityID,
	strength float64,
	metadata map[string]interface{},
) error {
	// Create primary relationship (bidirectional)
	relationship := models.NewEntityRelationship(
		relType,
		source,
		target,
		models.DirectionBidirectional,
		strength,
	)
	
	if metadata != nil {
		relationship.WithMetadata(metadata)
	}
	
	return s.CreateRelationship(ctx, relationship)
}

// DeleteRelationship removes a relationship by ID
func (s *serviceImpl) DeleteRelationship(ctx context.Context, relationshipID string) error {
	return s.repo.DeleteRelationship(ctx, relationshipID)
}

// DeleteRelationshipsBetween removes all relationships between two entities
func (s *serviceImpl) DeleteRelationshipsBetween(
	ctx context.Context,
	source models.EntityID,
	target models.EntityID,
) error {
	return s.repo.DeleteRelationshipsBetween(ctx, source, target)
}

// GetRelationship retrieves a relationship by ID
func (s *serviceImpl) GetRelationship(ctx context.Context, relationshipID string) (*models.EntityRelationship, error) {
	return s.repo.GetRelationship(ctx, relationshipID)
}

// GetDirectRelationships gets direct relationships for an entity
func (s *serviceImpl) GetDirectRelationships(
	ctx context.Context,
	entityID models.EntityID,
	direction string,
	relTypes []models.RelationshipType,
) ([]*models.EntityRelationship, error) {
	return s.repo.GetDirectRelationships(ctx, entityID, direction, relTypes)
}

// GetRelatedEntities gets entities related to the specified entity
func (s *serviceImpl) GetRelatedEntities(
	ctx context.Context,
	entityID models.EntityID,
	relTypes []models.RelationshipType,
	maxDepth int,
) ([]models.EntityID, error) {
	// Limit max depth to prevent potential performance issues
	if maxDepth > 5 {
		maxDepth = 5
	}
	
	// Get relationships through traversal
	relationships, err := s.GetRelationshipGraph(ctx, entityID, maxDepth)
	if err != nil {
		return nil, err
	}
	
	// Track visited entities to avoid duplicates
	visited := make(map[string]bool)
	
	// Extract distinct entities from relationships
	var entities []models.EntityID
	
	// Start with source entity
	sourceKey := models.GenerateRelationshipID(models.RelationshipTypeAssociates, entityID, entityID, models.DirectionOutgoing)
	visited[sourceKey] = true
	
	for _, rel := range relationships {
		// Check if relationship type is of interest
		if relTypes != nil && len(relTypes) > 0 {
			typeMatched := false
			for _, wantedType := range relTypes {
				if rel.Type == wantedType {
					typeMatched = true
					break
				}
			}
			if !typeMatched {
				continue
			}
		}
		
		// Process source entity
		sourceKey := models.GenerateRelationshipID(models.RelationshipTypeAssociates, rel.Source, rel.Source, models.DirectionOutgoing)
		if !visited[sourceKey] {
			visited[sourceKey] = true
			entities = append(entities, rel.Source)
		}
		
		// Process target entity
		targetKey := models.GenerateRelationshipID(models.RelationshipTypeAssociates, rel.Target, rel.Target, models.DirectionOutgoing)
		if !visited[targetKey] {
			visited[targetKey] = true
			entities = append(entities, rel.Target)
		}
	}
	
	return entities, nil
}

// GetRelationshipGraph gets the relationship graph for an entity up to a specified depth
func (s *serviceImpl) GetRelationshipGraph(
	ctx context.Context,
	entityID models.EntityID,
	maxDepth int,
) ([]*models.EntityRelationship, error) {
	// Limit max depth to prevent potential performance issues
	if maxDepth > 5 {
		maxDepth = 5
	}
	
	// Initialize results
	var results []*models.EntityRelationship
	
	// Track visited entities to prevent cycles
	visited := make(map[string]bool)
	
	// Start graph traversal from the entity
	err := s.traverseRelationships(ctx, entityID, 0, maxDepth, visited, &results)
	if err != nil {
		return nil, err
	}
	
	return results, nil
}

// traverseRelationships recursively traverses the relationship graph
func (s *serviceImpl) traverseRelationships(
	ctx context.Context,
	entityID models.EntityID,
	currentDepth int,
	maxDepth int,
	visited map[string]bool,
	results *[]*models.EntityRelationship,
) error {
	// Check if we've reached the maximum depth
	if currentDepth >= maxDepth {
		return nil
	}
	
	// Generate a key for this entity to track visitation
	entityKey := models.GenerateRelationshipID(models.RelationshipTypeAssociates, entityID, entityID, models.DirectionOutgoing)
	
	// Skip if already visited
	if visited[entityKey] {
		return nil
	}
	
	// Mark as visited
	visited[entityKey] = true
	
	// Get all relationships for this entity
	relationships, err := s.repo.GetDirectRelationships(ctx, entityID, models.DirectionBidirectional, nil)
	if err != nil {
		return err
	}
	
	// Add relationships to results
	for _, rel := range relationships {
		*results = append(*results, rel)
		
		// Recursively traverse target entity
		if !visited[models.GenerateRelationshipID(models.RelationshipTypeAssociates, rel.Target, rel.Target, models.DirectionOutgoing)] {
			err := s.traverseRelationships(ctx, rel.Target, currentDepth+1, maxDepth, visited, results)
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}
