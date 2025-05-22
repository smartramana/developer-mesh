package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// RelationshipRepository implements relationship operations
type RelationshipRepository struct {
	db       *sqlx.DB
	vectorDB *database.VectorDatabase
	logger   observability.Logger
}

// NewRelationshipRepository creates a new repository for entity relationships
func NewRelationshipRepository(db *sqlx.DB) RelationshipRepositoryInterface {
	// Create a logger that implements the observability.Logger interface
	logger := observability.NewStandardLogger("relationship_repository")

	// Initialize the vector database for any vector operations
	vectorDB, err := database.NewVectorDatabase(db, nil, logger)
	if err != nil {
		logger.Error("Failed to create vector database", map[string]interface{}{"error": err})
		// We still create the repository, but operations using vectorDB will fail
	}

	return &RelationshipRepository{
		db:       db,
		vectorDB: vectorDB,
		logger:   logger,
	}
}

// EntityRelationshipRecord represents the database record for an entity relationship
type EntityRelationshipRecord struct {
	ID         string    `db:"id"`
	Type       string    `db:"relationship_type"`
	Direction  string    `db:"direction"`
	SourceType string    `db:"source_type"`
	SourceOwner string   `db:"source_owner"`
	SourceRepo string    `db:"source_repo"`
	SourceID   string    `db:"source_id"`
	TargetType string    `db:"target_type"`
	TargetOwner string   `db:"target_owner"`
	TargetRepo string    `db:"target_repo"`
	TargetID   string    `db:"target_id"`
	Strength   float64   `db:"strength"`
	Context    string    `db:"context"`
	Metadata   []byte    `db:"metadata"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// CreateRelationship creates a new entity relationship
func (r *RelationshipRepository) CreateRelationship(ctx context.Context, relationship *models.EntityRelationship) error {
	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Use transaction to create the relationship
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Serialize metadata to JSON, handling nil/empty cases
		var metadataJSON []byte
		var err error

		if relationship.Metadata != nil {
			metadataJSON, err = json.Marshal(relationship.Metadata)
			if err != nil {
				return fmt.Errorf("failed to serialize relationship metadata: %w", err)
			}
		} else {
			metadataJSON = []byte("{}")
		}

		// Set timestamps if not provided
		now := time.Now()
		if relationship.CreatedAt.IsZero() {
			relationship.CreatedAt = now
		}
		if relationship.UpdatedAt.IsZero() {
			relationship.UpdatedAt = now
		}

		// Insert the relationship
		query := `
			INSERT INTO mcp.entity_relationships (
				id, relationship_type, direction, 
				source_type, source_owner, source_repo, source_id,
				target_type, target_owner, target_repo, target_id,
				strength, context, metadata, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
			)
			ON CONFLICT (id) DO UPDATE SET
				relationship_type = $2,
				direction = $3,
				source_type = $4,
				source_owner = $5,
				source_repo = $6,
				source_id = $7,
				target_type = $8,
				target_owner = $9,
				target_repo = $10,
				target_id = $11,
				strength = $12,
				context = $13,
				metadata = $14,
				updated_at = $16
		`

		_, err = tx.ExecContext(ctx, query,
			relationship.ID,
			relationship.Type,
			relationship.Direction,
			relationship.Source.Type,
			relationship.Source.Owner,
			relationship.Source.Repo,
			relationship.Source.ID,
			relationship.Target.Type,
			relationship.Target.Owner,
			relationship.Target.Repo,
			relationship.Target.ID,
			relationship.Strength,
			relationship.Context,
			metadataJSON,
			relationship.CreatedAt,
			relationship.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to create entity relationship: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// UpdateRelationship updates an existing relationship
func (r *RelationshipRepository) UpdateRelationship(ctx context.Context, relationship *models.EntityRelationship) error {
	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Use transaction to update the relationship
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Check if the relationship exists
		var exists bool
		err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM mcp.entity_relationships WHERE id = $1)", relationship.ID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check if relationship exists: %w", err)
		}
		if !exists {
			return fmt.Errorf("relationship not found: %s", relationship.ID)
		}

		// Serialize metadata to JSON, handling nil/empty cases
		var metadataJSON []byte
		if relationship.Metadata != nil {
			metadataJSON, err = json.Marshal(relationship.Metadata)
			if err != nil {
				return fmt.Errorf("failed to serialize relationship metadata: %w", err)
			}
		} else {
			metadataJSON = []byte("{}")
		}

		// Set updated timestamp
		relationship.UpdatedAt = time.Now()

		// Update the relationship
		query := `
			UPDATE mcp.entity_relationships SET
				relationship_type = $1,
				direction = $2,
				source_type = $3,
				source_owner = $4,
				source_repo = $5,
				source_id = $6,
				target_type = $7,
				target_owner = $8,
				target_repo = $9,
				target_id = $10,
				strength = $11,
				context = $12,
				metadata = $13,
				updated_at = $14
			WHERE id = $15
		`

		_, err = tx.ExecContext(ctx, query,
			relationship.Type,
			relationship.Direction,
			relationship.Source.Type,
			relationship.Source.Owner,
			relationship.Source.Repo,
			relationship.Source.ID,
			relationship.Target.Type,
			relationship.Target.Owner,
			relationship.Target.Repo,
			relationship.Target.ID,
			relationship.Strength,
			relationship.Context,
			metadataJSON,
			relationship.UpdatedAt,
			relationship.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to update entity relationship: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// DeleteRelationship removes a relationship by ID
func (r *RelationshipRepository) DeleteRelationship(ctx context.Context, relationshipID string) error {
	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Use transaction to delete the relationship
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		query := "DELETE FROM mcp.entity_relationships WHERE id = $1"
		result, err := tx.ExecContext(ctx, query, relationshipID)
		if err != nil {
			return fmt.Errorf("failed to delete relationship: %w", err)
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}

		if rows == 0 {
			return fmt.Errorf("relationship not found: %s", relationshipID)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// DeleteRelationshipsBetween removes all relationships between two entities
func (r *RelationshipRepository) DeleteRelationshipsBetween(ctx context.Context, source models.EntityID, target models.EntityID) error {
	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Use transaction to delete the relationships
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		query := `
			DELETE FROM mcp.entity_relationships 
			WHERE 
				(source_type = $1 AND source_owner = $2 AND source_repo = $3 AND source_id = $4 AND
				 target_type = $5 AND target_owner = $6 AND target_repo = $7 AND target_id = $8)
				OR
				(source_type = $5 AND source_owner = $6 AND source_repo = $7 AND source_id = $8 AND
				 target_type = $1 AND target_owner = $2 AND target_repo = $3 AND target_id = $4)
		`

		_, err := tx.ExecContext(ctx, query,
			source.Type, source.Owner, source.Repo, source.ID,
			target.Type, target.Owner, target.Repo, target.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to delete relationships between entities: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// GetRelationship retrieves a relationship by ID
func (r *RelationshipRepository) GetRelationship(ctx context.Context, relationshipID string) (*models.EntityRelationship, error) {
	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	var relationship *models.EntityRelationship
	// Use transaction to get the relationship
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		query := "SELECT * FROM mcp.entity_relationships WHERE id = $1"
		
		var record EntityRelationshipRecord
		err := tx.GetContext(ctx, &record, query, relationshipID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil // Not found, will return nil relationship
			}
			return fmt.Errorf("failed to get relationship: %w", err)
		}

		var err2 error
		relationship, err2 = r.recordToRelationship(&record)
		if err2 != nil {
			return err2
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return relationship, nil
}

// GetDirectRelationships gets direct relationships for an entity
func (r *RelationshipRepository) GetDirectRelationships(
	ctx context.Context,
	entityID models.EntityID,
	direction string,
	relTypes []models.RelationshipType,
) ([]*models.EntityRelationship, error) {
	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	var relationships []*models.EntityRelationship
	// Use transaction to get the relationships
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Base query
		query := "SELECT * FROM mcp.entity_relationships WHERE "
		
		// Build conditions based on direction
		conditions := []string{}
		args := []interface{}{}

		// Handle different directions
		if direction == models.DirectionOutgoing || direction == models.DirectionBidirectional {
			sourceCondition := "(source_type = $1 AND source_owner = $2 AND source_repo = $3 AND source_id = $4)"
			args = append(args, entityID.Type, entityID.Owner, entityID.Repo, entityID.ID)
			conditions = append(conditions, sourceCondition)
		}

		if direction == models.DirectionIncoming || direction == models.DirectionBidirectional {
			targetCondition := fmt.Sprintf("(target_type = $%d AND target_owner = $%d AND target_repo = $%d AND target_id = $%d)",
				len(args)+1, len(args)+2, len(args)+3, len(args)+4)
			args = append(args, entityID.Type, entityID.Owner, entityID.Repo, entityID.ID)
			conditions = append(conditions, targetCondition)
		}

		// Apply relationship type filter
		if len(relTypes) > 0 {
			typeConditions := []string{}
			for _, relType := range relTypes {
				typeConditions = append(
					typeConditions,
					fmt.Sprintf("relationship_type = $%d", len(args)+1),
				)
				args = append(args, relType)
			}
			conditions = append(conditions, fmt.Sprintf("(%s)", strings.Join(typeConditions, " OR ")))
		}

		// Combine all conditions
		query += strings.Join(conditions, " AND ")
		query += " ORDER BY updated_at DESC"

		var records []EntityRelationshipRecord
		err := tx.SelectContext(ctx, &records, query, args...)
		if err != nil {
			return fmt.Errorf("failed to query entity relationships: %w", err)
		}

		// Convert records to relationships
		relationships = make([]*models.EntityRelationship, 0, len(records))
		for _, record := range records {
			relationship, err := r.recordToRelationship(&record)
			if err != nil {
				return err
			}
			relationships = append(relationships, relationship)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return relationships, nil
}

// GetRelationshipsByType gets relationships of a specific type
func (r *RelationshipRepository) GetRelationshipsByType(
	ctx context.Context,
	relType models.RelationshipType,
	limit int,
	offset int,
) ([]*models.EntityRelationship, error) {
	// Make sure the vector database is initialized
	if r.vectorDB == nil {
		return nil, errors.New("vector database not initialized")
	}

	// Initialize the vector database
	if err := r.vectorDB.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize vector database: %w", err)
	}

	var relationships []*models.EntityRelationship
	// Use transaction to get the relationships
	err := r.vectorDB.Transaction(ctx, func(tx *sqlx.Tx) error {
		query := `
			SELECT * FROM mcp.entity_relationships
			WHERE relationship_type = $1
			ORDER BY updated_at DESC
		`
		args := []interface{}{relType}

		// Apply limit and offset
		if limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
			args = append(args, limit)
		}
		
		if offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", len(args)+1)
			args = append(args, offset)
		}

		var records []EntityRelationshipRecord
		err := tx.SelectContext(ctx, &records, query, args...)
		if err != nil {
			return fmt.Errorf("failed to query relationships by type: %w", err)
		}

		// Convert records to relationships
		relationships = make([]*models.EntityRelationship, 0, len(records))
		for _, record := range records {
			relationship, err := r.recordToRelationship(&record)
			if err != nil {
				return err
			}
			relationships = append(relationships, relationship)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return relationships, nil
}

// recordToRelationship converts a database record to an entity relationship
func (r *RelationshipRepository) recordToRelationship(record *EntityRelationshipRecord) (*models.EntityRelationship, error) {
	// Create source entity ID
	source := models.EntityID{
		Type:  models.EntityType(record.SourceType),
		Owner: record.SourceOwner,
		Repo:  record.SourceRepo,
		ID:    record.SourceID,
	}

	// Create target entity ID
	target := models.EntityID{
		Type:  models.EntityType(record.TargetType),
		Owner: record.TargetOwner,
		Repo:  record.TargetRepo,
		ID:    record.TargetID,
	}

	// Parse metadata
	var metadata map[string]interface{}
	if len(record.Metadata) > 0 {
		err := json.Unmarshal(record.Metadata, &metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal relationship metadata: %w", err)
		}
	} else {
		metadata = make(map[string]interface{})
	}

	// Create relationship
	relationship := &models.EntityRelationship{
		ID:        record.ID,
		Type:      models.RelationshipType(record.Type),
		Direction: record.Direction,
		Source:    source,
		Target:    target,
		Strength:  record.Strength,
		Context:   record.Context,
		Metadata:  metadata,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}

	return relationship, nil
}
