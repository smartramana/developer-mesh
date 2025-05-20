package embedding

import (
	"context"
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/models/relationship"
)

// RelationshipContextEnricher enhances embedding vectors with relationship context
type RelationshipContextEnricher struct {
	relationshipService relationship.Service
	maxRelationships    int
	includeDirection    string
	contextDepth        int
}

// NewRelationshipContextEnricher creates a new enricher for enhancing embeddings with relationship context
func NewRelationshipContextEnricher(service relationship.Service) *RelationshipContextEnricher {
	return &RelationshipContextEnricher{
		relationshipService: service,
		maxRelationships:    10,         // Default max relationships to include
		includeDirection:    "outgoing", // Default to outgoing relationships
		contextDepth:        1,          // Default to direct relationships only
	}
}

// WithMaxRelationships sets the maximum number of relationships to include in context
func (e *RelationshipContextEnricher) WithMaxRelationships(max int) *RelationshipContextEnricher {
	if max > 0 {
		e.maxRelationships = max
	}
	return e
}

// WithDirection sets the relationship direction to include
func (e *RelationshipContextEnricher) WithDirection(direction string) *RelationshipContextEnricher {
	if direction == models.DirectionOutgoing || 
	   direction == models.DirectionIncoming || 
	   direction == models.DirectionBidirectional {
		e.includeDirection = direction
	}
	return e
}

// WithContextDepth sets the depth of relationships to include (1=direct, 2+=indirect)
func (e *RelationshipContextEnricher) WithContextDepth(depth int) *RelationshipContextEnricher {
	if depth > 0 && depth <= 3 {
		e.contextDepth = depth
	}
	return e
}

// EnrichEmbeddingMetadata adds relationship context to embedding metadata
func (e *RelationshipContextEnricher) EnrichEmbeddingMetadata(
	ctx context.Context,
	contentType string,
	contentID string,
	owner string,
	repo string,
	metadata map[string]interface{},
) (map[string]interface{}, error) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Convert content metadata to entity ID
	entityID := models.EntityIDFromContentMetadata(contentType, owner, repo, contentID)

	// Get relationships for the entity
	var relationships []*models.EntityRelationship
	var err error

	if e.contextDepth == 1 {
		// Get direct relationships only
		relationships, err = e.relationshipService.GetDirectRelationships(
			ctx,
			entityID,
			e.includeDirection,
			nil, // All relationship types
		)
	} else {
		// Get full relationship graph to the specified depth
		relationships, err = e.relationshipService.GetRelationshipGraph(
			ctx,
			entityID,
			e.contextDepth,
		)
	}

	if err != nil {
		return metadata, fmt.Errorf("failed to get relationships for entity: %w", err)
	}

	// Limit the number of relationships to include
	if len(relationships) > e.maxRelationships {
		relationships = relationships[:e.maxRelationships]
	}

	// Create relationship context for metadata
	relContext := make(map[string]interface{})
	
	// Track related entities of each type
	relatedEntities := make(map[string][]string)
	
	// Track relationship types
	relTypes := make(map[string]int)

	// Process relationships
	for _, rel := range relationships {
		// Record relationship type
		relTypeName := string(rel.Type)
		relTypes[relTypeName] = relTypes[relTypeName] + 1

		// Process the "other" entity in the relationship (not the source entity)
		var relatedEntity models.EntityID

		// Determine which entity is the "other" one in the relationship
		if rel.Source.Type == entityID.Type &&
			rel.Source.Owner == entityID.Owner &&
			rel.Source.Repo == entityID.Repo &&
			rel.Source.ID == entityID.ID {
			// Current entity is the source, so related entity is the target
			relatedEntity = rel.Target
		} else if rel.Target.Type == entityID.Type &&
			rel.Target.Owner == entityID.Owner &&
			rel.Target.Repo == entityID.Repo &&
			rel.Target.ID == entityID.ID {
			// Current entity is the target, so related entity is the source
			relatedEntity = rel.Source
		} else {
			// This is an indirect relationship, could be included in multi-depth traversals
			// Skip for now to keep metadata focused
			continue
		}

		// Create a simple identifier for the related entity
		entityKey := fmt.Sprintf("%s/%s/%s/%s", 
			relatedEntity.Type, 
			relatedEntity.Owner, 
			relatedEntity.Repo, 
			relatedEntity.ID)

		// Add to related entities map by type
		entityType := string(relatedEntity.Type)
		relatedEntities[entityType] = append(relatedEntities[entityType], entityKey)
	}

	// Add relationship summary to metadata
	relContext["rel_types"] = relTypes
	relContext["rel_entities"] = relatedEntities
	relContext["rel_count"] = len(relationships)

	// Add context to metadata
	metadata["relationships"] = relContext

	// If the entity has relationships, generate a textual context description
	if len(relationships) > 0 {
		metadata["relationship_context"] = e.generateRelationshipContextText(relationships, entityID)
	}

	return metadata, nil
}

// EnrichEmbeddingText adds relationship context to the text for embedding
func (e *RelationshipContextEnricher) EnrichEmbeddingText(
	ctx context.Context,
	contentType string,
	contentID string,
	owner string,
	repo string,
	originalText string,
) (string, error) {
	// Convert content metadata to entity ID
	entityID := models.EntityIDFromContentMetadata(contentType, owner, repo, contentID)

	// Get direct relationships for the entity
	relationships, err := e.relationshipService.GetDirectRelationships(
		ctx,
		entityID,
		models.DirectionBidirectional, // Get both directions for text enrichment
		nil, // All relationship types
	)
	if err != nil {
		return originalText, fmt.Errorf("failed to get relationships for entity: %w", err)
	}

	// If no relationships, return original text
	if len(relationships) == 0 {
		return originalText, nil
	}

	// Limit relationships to include
	if len(relationships) > e.maxRelationships {
		relationships = relationships[:e.maxRelationships]
	}

	// Generate relationship context text
	relationshipContext := e.generateRelationshipContextText(relationships, entityID)
	
	// Combine with original text
	enrichedText := fmt.Sprintf("%s\n\nRelationship Context:\n%s", originalText, relationshipContext)
	
	return enrichedText, nil
}

// generateRelationshipContextText creates a textual description of entity relationships
func (e *RelationshipContextEnricher) generateRelationshipContextText(
	relationships []*models.EntityRelationship,
	entityID models.EntityID,
) string {
	var contextBuilder strings.Builder
	
	// Group relationships by type
	relsByType := make(map[models.RelationshipType][]*models.EntityRelationship)
	for _, rel := range relationships {
		relsByType[rel.Type] = append(relsByType[rel.Type], rel)
	}
	
	// Generate context for each relationship type
	for relType, rels := range relsByType {
		// Determine verb based on relationship type
		var verb string
		switch relType {
		case models.RelationshipTypeReferences:
			verb = "references"
		case models.RelationshipTypeContains:
			verb = "contains"
		case models.RelationshipTypeCreates:
			verb = "creates"
		case models.RelationshipTypeModifies:
			verb = "modifies"
		case models.RelationshipTypeDependsOn:
			verb = "depends on"
		case models.RelationshipTypeImplements:
			verb = "implements"
		case models.RelationshipTypeExtends:
			verb = "extends"
		case models.RelationshipTypeReplaces:
			verb = "replaces"
		case models.RelationshipTypeComments:
			verb = "comments on"
		default:
			verb = "is associated with"
		}
		
		// Create text description for each relationship of this type
		for _, rel := range rels {
			// Determine if this entity is source or target
			isSource := rel.Source.Type == entityID.Type &&
				rel.Source.Owner == entityID.Owner &&
				rel.Source.Repo == entityID.Repo &&
				rel.Source.ID == entityID.ID
				
			var otherEntity models.EntityID
			if isSource {
				otherEntity = rel.Target
			} else {
				otherEntity = rel.Source
			}
			
			// Format the relationship text
			if isSource {
				contextBuilder.WriteString(fmt.Sprintf("This %s %s %s %s/%s in repo %s/%s.\n",
					entityID.Type, verb, otherEntity.Type, otherEntity.Repo, otherEntity.ID,
					otherEntity.Owner, otherEntity.Repo))
			} else {
				// Reverse the relationship description
				contextBuilder.WriteString(fmt.Sprintf("This %s is %sd by %s %s/%s in repo %s/%s.\n",
					entityID.Type, verb, otherEntity.Type, otherEntity.Repo, otherEntity.ID,
					otherEntity.Owner, otherEntity.Repo))
			}
			
			// Add context if available
			if rel.Context != "" {
				contextBuilder.WriteString(fmt.Sprintf("  Context: %s\n", rel.Context))
			}
		}
	}
	
	return contextBuilder.String()
}
