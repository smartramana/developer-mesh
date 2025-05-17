// Package relationship provides a compatibility layer for code that imports
// github.com/S-Corkum/devops-mcp/pkg/relationship. This package re-exports all
// types and functions from github.com/S-Corkum/devops-mcp/pkg/models/relationship.
package relationship

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	modelsrel "github.com/S-Corkum/devops-mcp/pkg/models/relationship"
)

// Repository defines the interface for relationship data persistence
type Repository = modelsrel.Repository

// Service is a wrapper around a Repository that provides relationship management
type Service struct {
	repo Repository
}

// NewService creates a new relationship service
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateRelationship creates a new entity relationship
func (s *Service) CreateRelationship(ctx context.Context, relationship *models.EntityRelationship) error {
	return s.repo.CreateRelationship(ctx, relationship)
}

// UpdateRelationship updates an existing relationship
func (s *Service) UpdateRelationship(ctx context.Context, relationship *models.EntityRelationship) error {
	return s.repo.UpdateRelationship(ctx, relationship)
}

// DeleteRelationship removes a relationship by ID
func (s *Service) DeleteRelationship(ctx context.Context, relationshipID string) error {
	return s.repo.DeleteRelationship(ctx, relationshipID)
}

// DeleteRelationshipsBetween removes all relationships between two entities
func (s *Service) DeleteRelationshipsBetween(ctx context.Context, source models.EntityID, target models.EntityID) error {
	return s.repo.DeleteRelationshipsBetween(ctx, source, target)
}

// GetRelationship retrieves a relationship by ID
func (s *Service) GetRelationship(ctx context.Context, relationshipID string) (*models.EntityRelationship, error) {
	return s.repo.GetRelationship(ctx, relationshipID)
}

// GetDirectRelationships gets direct relationships for an entity
func (s *Service) GetDirectRelationships(
	ctx context.Context,
	entityID models.EntityID,
	direction string,
	relTypes []models.RelationshipType,
) ([]*models.EntityRelationship, error) {
	return s.repo.GetDirectRelationships(ctx, entityID, direction, relTypes)
}

// GetRelationshipsByType gets relationships of a specific type
func (s *Service) GetRelationshipsByType(
	ctx context.Context,
	relType models.RelationshipType,
	limit int,
	offset int,
) ([]*models.EntityRelationship, error) {
	return s.repo.GetRelationshipsByType(ctx, relType, limit, offset)
}

// GetRelationshipGraph gets the relationship graph for an entity up to a specified depth
func (s *Service) GetRelationshipGraph(
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
func (s *Service) traverseRelationships(
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
	entityKey := fmt.Sprintf("%s:%s/%s:%s", entityID.Type, entityID.Owner, entityID.Repo, entityID.ID)
	
	// Skip if already visited
	if visited[entityKey] {
		return nil
	}
	
	// Mark as visited
	visited[entityKey] = true
	
	// Get all relationships for this entity
	relationships, err := s.GetDirectRelationships(ctx, entityID, models.DirectionBidirectional, nil)
	if err != nil {
		return err
	}
	
	// Add relationships to results
	for _, rel := range relationships {
		*results = append(*results, rel)
		
		// Determine the other entity (not the source)
		var nextEntity models.EntityID
		if rel.Source.String() == entityID.String() {
			nextEntity = rel.Target
		} else {
			nextEntity = rel.Source
		}
		
		// Recursively traverse the next entity
		nextKey := fmt.Sprintf("%s:%s/%s:%s", nextEntity.Type, nextEntity.Owner, nextEntity.Repo, nextEntity.ID)
		if !visited[nextKey] {
			err := s.traverseRelationships(ctx, nextEntity, currentDepth+1, maxDepth, visited, results)
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}
