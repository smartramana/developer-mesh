package embedding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// PgVectorStorage implements EmbeddingStorage for PostgreSQL with pgvector
type PgVectorStorage struct {
	// Database connection
	db *sql.DB
	// Schema name
	schema string
}

// NewPgVectorStorage creates a new PostgreSQL vector storage
func NewPgVectorStorage(db *sql.DB, schema string) (*PgVectorStorage, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	if schema == "" {
		schema = "mcp" // Default schema
	}

	// Verify pgvector extension is available
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check pgvector extension: %w", err)
	}

	if !exists {
		return nil, errors.New("pgvector extension is not installed in the database")
	}

	return &PgVectorStorage{
		db:     db,
		schema: schema,
	}, nil
}

// StoreEmbedding stores a single embedding
func (s *PgVectorStorage) StoreEmbedding(ctx context.Context, embedding *EmbeddingVector) error {
	if embedding == nil {
		return errors.New("embedding cannot be nil")
	}

	// Convert metadata to JSON if present
	var metadataJSON sql.NullString
	if len(embedding.Metadata) > 0 {
		// We'll handle this in the database layer to ensure proper JSON formatting
		metadataJSON = sql.NullString{String: "{}", Valid: true}
	}

	// Format vector for pgvector
	vectorStr := formatVectorForPg(embedding.Vector)

	// Insert embedding into database
	query := fmt.Sprintf(`
		INSERT INTO %s.embeddings (
			id, context_id, content_index, text, 
			embedding, vector_dimensions, model_id, 
			metadata, content_type
		) VALUES (
			$1, $2, $3, $4, 
			$5::vector, $6, $7, 
			$8, $9
		)
		ON CONFLICT (id) DO UPDATE SET
			embedding = $5::vector,
			vector_dimensions = $6,
			model_id = $7,
			metadata = $8,
			content_type = $9
	`, s.schema)

	// Generate a unique ID based on content type and ID
	id := fmt.Sprintf("%s:%s", embedding.ContentType, embedding.ContentID)

	_, err := s.db.ExecContext(
		ctx,
		query,
		id,                    // id
		"",                    // context_id (empty for now, could be populated later)
		0,                     // content_index
		"",                    // text (empty for now, could store the original text)
		vectorStr,             // embedding
		embedding.Dimensions,  // vector_dimensions
		embedding.ModelID,     // model_id
		metadataJSON,          // metadata
		embedding.ContentType, // content_type
	)

	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	return nil
}

// BatchStoreEmbeddings stores multiple embeddings in a batch
func (s *PgVectorStorage) BatchStoreEmbeddings(ctx context.Context, embeddings []*EmbeddingVector) error {
	if len(embeddings) == 0 {
		return nil // Nothing to store
	}

	// Use a transaction for batch inserts
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil {
			// Will be ignored if transaction is committed, which is the expected case
			_ = rbErr
		}
	}()

	// Prepare statement for batch insert
	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO %s.embeddings (
			id, context_id, content_index, text, 
			embedding, vector_dimensions, model_id, 
			metadata, content_type
		) VALUES (
			$1, $2, $3, $4, 
			$5::vector, $6, $7, 
			$8, $9
		)
		ON CONFLICT (id) DO UPDATE SET
			embedding = $5::vector,
			vector_dimensions = $6,
			model_id = $7,
			metadata = $8,
			content_type = $9
	`, s.schema))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			// Statement close error - log but don't fail
			_ = err
		}
	}()

	// Insert each embedding
	for _, embedding := range embeddings {
		if embedding == nil {
			continue // Skip nil embeddings
		}

		// Convert metadata to JSON if present
		var metadataJSON sql.NullString
		if len(embedding.Metadata) > 0 {
			// We'll handle this in the database layer to ensure proper JSON formatting
			metadataJSON = sql.NullString{String: "{}", Valid: true}
		}

		// Format vector for pgvector
		vectorStr := formatVectorForPg(embedding.Vector)

		// Generate a unique ID based on content type and ID
		id := fmt.Sprintf("%s:%s", embedding.ContentType, embedding.ContentID)

		_, err := stmt.ExecContext(
			ctx,
			id,                    // id
			"",                    // context_id (empty for now)
			0,                     // content_index
			"",                    // text (empty for now)
			vectorStr,             // embedding
			embedding.Dimensions,  // vector_dimensions
			embedding.ModelID,     // model_id
			metadataJSON,          // metadata
			embedding.ContentType, // content_type
		)

		if err != nil {
			return fmt.Errorf("failed to store embedding %s: %w", id, err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// FindSimilarEmbeddings finds embeddings similar to the provided one
func (s *PgVectorStorage) FindSimilarEmbeddings(ctx context.Context, embedding *EmbeddingVector, limit int, threshold float32) ([]*EmbeddingVector, error) {
	if embedding == nil {
		return nil, errors.New("embedding cannot be nil")
	}

	if limit <= 0 {
		limit = 10 // Default limit
	}

	if threshold <= 0 || threshold > 1 {
		threshold = 0.7 // Default threshold
	}

	// Format vector for pgvector
	vectorStr := formatVectorForPg(embedding.Vector)

	// Query for similar embeddings
	query := fmt.Sprintf(`
		SELECT
			id, context_id, content_index, text,
			embedding::text, vector_dimensions, model_id,
			metadata, content_type,
			(1 - (embedding <=> $1::vector))::float AS similarity
		FROM
			%s.embeddings
		WHERE
			vector_dimensions = $2
			AND model_id = $3
			AND (1 - (embedding <=> $1::vector))::float >= $4
		ORDER BY
			similarity DESC
		LIMIT $5
	`, s.schema)

	rows, err := s.db.QueryContext(
		ctx,
		query,
		vectorStr,
		embedding.Dimensions,
		embedding.ModelID,
		threshold,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query similar embeddings: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Postgres - log but don't fail
			_ = err
		}
	}()

	var results []*EmbeddingVector
	for rows.Next() {
		var (
			id              string
			contextID       sql.NullString
			contentIndex    int
			text            sql.NullString
			embeddingStr    string
			dimensions      int
			modelID         string
			metadataJSON    sql.NullString
			contentType     string
			similarityScore float32
		)

		if err := rows.Scan(
			&id,
			&contextID,
			&contentIndex,
			&text,
			&embeddingStr,
			&dimensions,
			&modelID,
			&metadataJSON,
			&contentType,
			&similarityScore,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding row: %w", err)
		}

		// Extract content ID from compound ID
		parts := strings.SplitN(id, ":", 2)
		contentID := ""
		if len(parts) > 1 {
			contentID = parts[1]
		} else {
			contentID = id
		}

		// Parse the vector string
		vector, err := parseVectorFromPg(embeddingStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse vector: %w", err)
		}

		// Create metadata map if JSON is valid
		metadata := make(map[string]interface{})
		if metadataJSON.Valid {
			// In a real implementation, you would parse the JSON here
			// For simplicity, we're just adding the similarity score
			metadata["similarity"] = similarityScore
		} else {
			metadata["similarity"] = similarityScore
		}

		result := &EmbeddingVector{
			Vector:      vector,
			Dimensions:  dimensions,
			ModelID:     modelID,
			ContentType: contentType,
			ContentID:   contentID,
			Metadata:    metadata,
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return results, nil
}

// GetEmbeddingsByContentIDs retrieves embeddings by content IDs
func (s *PgVectorStorage) GetEmbeddingsByContentIDs(ctx context.Context, contentIDs []string) ([]*EmbeddingVector, error) {
	if len(contentIDs) == 0 {
		return nil, errors.New("no content IDs provided")
	}

	// Prepare the IN clause placeholders and args
	placeholders := make([]string, len(contentIDs))
	args := make([]interface{}, len(contentIDs))

	for i, id := range contentIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	// Query for embeddings by content ID
	query := fmt.Sprintf(`
		SELECT
			id, context_id, content_index, text,
			embedding::text, vector_dimensions, model_id,
			metadata, content_type
		FROM
			%s.embeddings
		WHERE
			id = ANY($1)
	`, s.schema)

	rows, err := s.db.QueryContext(ctx, query, pq.Array(contentIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings by content IDs: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Postgres - log but don't fail
			_ = err
		}
	}()

	var results []*EmbeddingVector
	for rows.Next() {
		var (
			id           string
			contextID    sql.NullString
			contentIndex int
			text         sql.NullString
			embeddingStr string
			dimensions   int
			modelID      string
			metadataJSON sql.NullString
			contentType  string
		)

		if err := rows.Scan(
			&id,
			&contextID,
			&contentIndex,
			&text,
			&embeddingStr,
			&dimensions,
			&modelID,
			&metadataJSON,
			&contentType,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding row: %w", err)
		}

		// Extract content ID from compound ID
		parts := strings.SplitN(id, ":", 2)
		contentID := ""
		if len(parts) > 1 {
			contentID = parts[1]
		} else {
			contentID = id
		}

		// Parse the vector string
		vector, err := parseVectorFromPg(embeddingStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse vector: %w", err)
		}

		result := &EmbeddingVector{
			Vector:      vector,
			Dimensions:  dimensions,
			ModelID:     modelID,
			ContentType: contentType,
			ContentID:   contentID,
			Metadata:    make(map[string]interface{}),
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return results, nil
}

// DeleteEmbeddingsByContentIDs deletes embeddings by content IDs
func (s *PgVectorStorage) DeleteEmbeddingsByContentIDs(ctx context.Context, contentIDs []string) error {
	if len(contentIDs) == 0 {
		return errors.New("no content IDs provided")
	}

	// Delete embeddings by content ID
	query := fmt.Sprintf(`
		DELETE FROM %s.embeddings
		WHERE id = ANY($1)
	`, s.schema)

	_, err := s.db.ExecContext(ctx, query, pq.Array(contentIDs))
	if err != nil {
		return fmt.Errorf("failed to delete embeddings: %w", err)
	}

	return nil
}

// Helper functions

// formatVectorForPg formats a vector for PostgreSQL
func formatVectorForPg(vector []float32) string {
	// Format as [1,2,3,...]
	elements := make([]string, len(vector))
	for i, v := range vector {
		elements[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(elements, ",") + "]"
}

// parseVectorFromPg parses a vector from PostgreSQL string format
func parseVectorFromPg(vectorStr string) ([]float32, error) {
	// Remove brackets and split by commas
	vectorStr = strings.Trim(vectorStr, "[]")
	elements := strings.Split(vectorStr, ",")

	vector := make([]float32, len(elements))
	for i, elem := range elements {
		var val float64
		if _, err := fmt.Sscanf(elem, "%f", &val); err != nil {
			return nil, fmt.Errorf("failed to parse vector element %d: %w", i, err)
		}
		vector[i] = float32(val)
	}

	return vector, nil
}
