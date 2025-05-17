package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/relationship"
	"github.com/jmoiron/sqlx"
)

// RelationshipRepository implements relationship.Repository
type RelationshipRepository struct {
	db *Database
}

// NewRelationshipRepository creates a new PostgreSQL repository for entity relationships
func NewRelationshipRepository(db *Database) relationship.Repository {
	return &RelationshipRepository{
		db: db,
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

// EnsureRelationshipTables creates the necessary database tables for entity relationships
func (db *Database) EnsureRelationshipTables(ctx context.Context) error {
	// Create entity relationships table
	_, err := db.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mcp.entity_relationships (
			id VARCHAR(255) PRIMARY KEY,
			relationship_type VARCHAR(50) NOT NULL,
			direction VARCHAR(20) NOT NULL,
			source_type VARCHAR(50) NOT NULL,
			source_owner VARCHAR(255) NOT NULL,
			source_repo VARCHAR(255) NOT NULL,
			source_id VARCHAR(255) NOT NULL,
			target_type VARCHAR(50) NOT NULL,
			target_owner VARCHAR(255) NOT NULL,
			target_repo VARCHAR(255) NOT NULL,
			target_id VARCHAR(255) NOT NULL,
			strength FLOAT8 NOT NULL,
			context TEXT,
			metadata JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create entity_relationships table: %w", err)
	}

	// Create indexes for efficient querying
	_, err = db.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_relationships_source_type ON mcp.entity_relationships(source_type);
		CREATE INDEX IF NOT EXISTS idx_relationships_target_type ON mcp.entity_relationships(target_type);
		CREATE INDEX IF NOT EXISTS idx_relationships_source_owner_repo ON mcp.entity_relationships(source_owner, source_repo);
		CREATE INDEX IF NOT EXISTS idx_relationships_target_owner_repo ON mcp.entity_relationships(target_owner, target_repo);
		CREATE INDEX IF NOT EXISTS idx_relationships_relationship_type ON mcp.entity_relationships(relationship_type);
		CREATE INDEX IF NOT EXISTS idx_relationships_direction ON mcp.entity_relationships(direction);
		CREATE INDEX IF NOT EXISTS idx_relationships_source_id ON mcp.entity_relationships(source_id);
		CREATE INDEX IF NOT EXISTS idx_relationships_target_id ON mcp.entity_relationships(target_id);
		CREATE INDEX IF NOT EXISTS idx_relationships_updated_at ON mcp.entity_relationships(updated_at);
	`)
	if err != nil {
		return fmt.Errorf("failed to create relationship indexes: %w", err)
	}

	return nil
}

// CreateRelationship creates a new entity relationship
func (r *RelationshipRepository) CreateRelationship(ctx context.Context, relationship *models.EntityRelationship) error {
	return r.db.Transaction(ctx, func(tx *sqlx.Tx) error {
		return r.createRelationship(ctx, &Tx{tx: tx}, relationship)
	})
}

// createRelationship is the internal implementation to create a relationship within a transaction
func (r *RelationshipRepository) createRelationship(ctx context.Context, tx *Tx, relationship *models.EntityRelationship) error {
	// Serialize metadata to JSON, handling nil/empty cases
	var metadataJSON []byte
	var err error
	if relationship.Metadata == nil || len(relationship.Metadata) == 0 {
		metadataJSON = []byte("{}")
	} else {
		metadataJSON, err = json.Marshal(relationship.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal relationship metadata: %w", err)
		}
		// Ensure valid JSON
		if string(metadataJSON) == "" || string(metadataJSON) == "null" {
			metadataJSON = []byte("{}")
		}
	}

	_, err = tx.tx.ExecContext(ctx, `
		INSERT INTO mcp.entity_relationships (
			id, relationship_type, direction, 
			source_type, source_owner, source_repo, source_id,
			target_type, target_owner, target_repo, target_id,
			strength, context, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		) ON CONFLICT (id) DO NOTHING
	`,
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
		return fmt.Errorf("failed to insert entity relationship: %w", err)
	}

	return nil
}

// UpdateRelationship updates an existing relationship
func (r *RelationshipRepository) UpdateRelationship(ctx context.Context, relationship *models.EntityRelationship) error {
	return r.db.Transaction(ctx, func(tx *sqlx.Tx) error {
		return r.updateRelationship(ctx, &Tx{tx: tx}, relationship)
	})
}

// updateRelationship is the internal implementation to update a relationship within a transaction
func (r *RelationshipRepository) updateRelationship(ctx context.Context, tx *Tx, relationship *models.EntityRelationship) error {
	// Serialize metadata to JSON, handling nil/empty cases
	var metadataJSON []byte
	var err error
	if relationship.Metadata == nil || len(relationship.Metadata) == 0 {
		metadataJSON = []byte("{}")
	} else {
		metadataJSON, err = json.Marshal(relationship.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal relationship metadata: %w", err)
		}
		// Ensure valid JSON
		if string(metadataJSON) == "" || string(metadataJSON) == "null" {
			metadataJSON = []byte("{}")
		}
	}

	// Update the relationship
	now := time.Now()
	result, err := tx.tx.ExecContext(ctx, `
		UPDATE mcp.entity_relationships
		SET 
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
			updated_at = $15
		WHERE id = $1
	`,
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
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to update entity relationship: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if affected == 0 {
		return errors.New("relationship not found")
	}

	// Update the in-memory object
	relationship.UpdatedAt = now

	return nil
}

// DeleteRelationship removes a relationship by ID
func (r *RelationshipRepository) DeleteRelationship(ctx context.Context, relationshipID string) error {
	return r.db.Transaction(ctx, func(tx *sqlx.Tx) error {
		return r.deleteRelationship(ctx, &Tx{tx: tx}, relationshipID)
	})
}

// deleteRelationship is the internal implementation to delete a relationship within a transaction
func (r *RelationshipRepository) deleteRelationship(ctx context.Context, tx *Tx, relationshipID string) error {
	result, err := tx.tx.ExecContext(ctx, `
		DELETE FROM mcp.entity_relationships
		WHERE id = $1
	`, relationshipID)
	if err != nil {
		return fmt.Errorf("failed to delete entity relationship: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if affected == 0 {
		return errors.New("relationship not found")
	}

	return nil
}

// DeleteRelationshipsBetween removes all relationships between two entities
func (r *RelationshipRepository) DeleteRelationshipsBetween(ctx context.Context, source models.EntityID, target models.EntityID) error {
	return r.db.Transaction(ctx, func(tx *sqlx.Tx) error {
		return r.deleteRelationshipsBetween(ctx, &Tx{tx: tx}, source, target)
	})
}

// deleteRelationshipsBetween is the internal implementation to delete relationships between entities within a transaction
func (r *RelationshipRepository) deleteRelationshipsBetween(ctx context.Context, tx *Tx, source models.EntityID, target models.EntityID) error {
	_, err := tx.tx.ExecContext(ctx, `
		DELETE FROM mcp.entity_relationships
		WHERE 
			(source_type = $1 AND source_owner = $2 AND source_repo = $3 AND source_id = $4
			AND target_type = $5 AND target_owner = $6 AND target_repo = $7 AND target_id = $8)
			OR
			(source_type = $5 AND source_owner = $6 AND source_repo = $7 AND source_id = $8
			AND target_type = $1 AND target_owner = $2 AND target_repo = $3 AND target_id = $4)
	`,
		source.Type, source.Owner, source.Repo, source.ID,
		target.Type, target.Owner, target.Repo, target.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete relationships between entities: %w", err)
	}

	return nil
}

// GetRelationship retrieves a relationship by ID
func (r *RelationshipRepository) GetRelationship(ctx context.Context, relationshipID string) (*models.EntityRelationship, error) {
	var relationship *models.EntityRelationship
	err := r.db.Transaction(ctx, func(tx *sqlx.Tx) error {
		var err error
		relationship, err = r.getRelationship(ctx, &Tx{tx: tx}, relationshipID)
		return err
	})
	return relationship, err
}

// getRelationship is the internal implementation to retrieve a relationship within a transaction
func (r *RelationshipRepository) getRelationship(ctx context.Context, tx *Tx, relationshipID string) (*models.EntityRelationship, error) {
	var record EntityRelationshipRecord
	err := tx.tx.GetContext(ctx, &record, `
		SELECT * FROM mcp.entity_relationships
		WHERE id = $1
	`, relationshipID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("relationship not found")
		}
		return nil, fmt.Errorf("failed to get entity relationship: %w", err)
	}

	return r.recordToRelationship(&record)
}

// GetDirectRelationships gets direct relationships for an entity
func (r *RelationshipRepository) GetDirectRelationships(
	ctx context.Context,
	entityID models.EntityID,
	direction string,
	relTypes []models.RelationshipType,
) ([]*models.EntityRelationship, error) {
	var relationships []*models.EntityRelationship
	err := r.db.Transaction(ctx, func(tx *sqlx.Tx) error {
		var err error
		relationships, err = r.getDirectRelationships(ctx, &Tx{tx: tx}, entityID, direction, relTypes)
		return err
	})
	return relationships, err
}

// getDirectRelationships is the internal implementation to get direct relationships within a transaction
func (r *RelationshipRepository) getDirectRelationships(
	ctx context.Context,
	tx *Tx,
	entityID models.EntityID,
	direction string,
	relTypes []models.RelationshipType,
) ([]*models.EntityRelationship, error) {
	query := `
		SELECT * FROM mcp.entity_relationships
		WHERE 
	`
	args := []interface{}{}
	conditions := []string{}

	// Build source entity conditions
	sourceCondition := fmt.Sprintf(
		"(source_type = $%d AND source_owner = $%d AND source_repo = $%d AND source_id = $%d)",
		len(args)+1, len(args)+2, len(args)+3, len(args)+4,
	)
	args = append(args, entityID.Type, entityID.Owner, entityID.Repo, entityID.ID)
	
	// Build target entity conditions
	targetCondition := fmt.Sprintf(
		"(target_type = $%d AND target_owner = $%d AND target_repo = $%d AND target_id = $%d)",
		len(args)+1, len(args)+2, len(args)+3, len(args)+4,
	)
	args = append(args, entityID.Type, entityID.Owner, entityID.Repo, entityID.ID)

	// Apply direction filter
	switch direction {
	case models.DirectionOutgoing:
		conditions = append(conditions, sourceCondition)
	case models.DirectionIncoming:
		conditions = append(conditions, targetCondition)
	case models.DirectionBidirectional:
		conditions = append(conditions, fmt.Sprintf("(%s OR %s)", sourceCondition, targetCondition))
	default:
		// Default to bidirectional if not specified
		conditions = append(conditions, fmt.Sprintf("(%s OR %s)", sourceCondition, targetCondition))
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
	err := tx.tx.SelectContext(ctx, &records, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query entity relationships: %w", err)
	}

	// Convert records to relationships
	relationships := make([]*models.EntityRelationship, 0, len(records))
	for _, record := range records {
		relationship, err := r.recordToRelationship(&record)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, relationship)
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
	var relationships []*models.EntityRelationship
	err := r.db.Transaction(ctx, func(tx *sqlx.Tx) error {
		var err error
		relationships, err = r.getRelationshipsByType(ctx, &Tx{tx: tx}, relType, limit, offset)
		return err
	})
	return relationships, err
}

// getRelationshipsByType is the internal implementation to get relationships by type within a transaction
func (r *RelationshipRepository) getRelationshipsByType(
	ctx context.Context,
	tx *Tx,
	relType models.RelationshipType,
	limit int,
	offset int,
) ([]*models.EntityRelationship, error) {
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
	err := tx.tx.SelectContext(ctx, &records, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query relationships by type: %w", err)
	}

	// Convert records to relationships
	relationships := make([]*models.EntityRelationship, 0, len(records))
	for _, record := range records {
		relationship, err := r.recordToRelationship(&record)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, relationship)
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

	// Parse context if available
	contextMap := make(map[string]any)
	if record.Context != "" {
		// Try to parse as JSON first
		if err := json.Unmarshal([]byte(record.Context), &contextMap); err != nil {
			// If it's not valid JSON, just store it as a single value
			contextMap["value"] = record.Context
		}
	}

	// Create relationship
	relationship := &models.EntityRelationship{
		ID:        record.ID,
		Type:      models.RelationshipType(record.Type),
		Direction: record.Direction,
		Source:    source,
		Target:    target,
		Strength:  record.Strength,
		Context:   contextMap,
		Metadata:  metadata,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}

	return relationship, nil
}
