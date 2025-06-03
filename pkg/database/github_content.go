package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/storage"
	"github.com/jmoiron/sqlx"
)

// GitHubContentMetadata represents a database record for GitHub content metadata
type GitHubContentMetadata struct {
	ID          string    `db:"id"`
	Owner       string    `db:"owner"`
	Repo        string    `db:"repo"`
	ContentType string    `db:"content_type"`
	ContentID   string    `db:"content_id"`
	Checksum    string    `db:"checksum"`
	URI         string    `db:"uri"`
	Size        int64     `db:"size"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	ExpiresAt   time.Time `db:"expires_at"`
	Metadata    []byte    `db:"metadata"`
}

// StoreGitHubContent stores GitHub content metadata in the database
func (db *Database) StoreGitHubContent(ctx context.Context, metadata *storage.ContentMetadata) error {
	return db.Transaction(ctx, func(tx *sqlx.Tx) error {
		return db.storeGitHubContent(ctx, &Tx{tx: tx}, metadata)
	})
}

// storeGitHubContent is the internal implementation to store GitHub content metadata within a transaction
func (db *Database) storeGitHubContent(ctx context.Context, tx *Tx, metadata *storage.ContentMetadata) error {
	// Serialize metadata to JSON, handling nil/empty cases
	var metadataJSON []byte
	var err error
	if metadata.Metadata == nil || len(metadata.Metadata) == 0 {
		metadataJSON = []byte("{}")
	} else {
		metadataJSON, err = json.Marshal(metadata.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		// Final check to avoid empty string being sent to PostgreSQL
		if string(metadataJSON) == "" || string(metadataJSON) == "null" {
			metadataJSON = []byte("{}")
		}
	}

	// Generate a unique ID for this metadata record
	id := fmt.Sprintf("gh-%s-%s-%s-%s",
		metadata.Owner,
		metadata.Repo,
		metadata.ContentType,
		metadata.ContentID)

	// Handle nullable time.Time for expires_at
	var expiresAt sql.NullTime
	if metadata.ExpiresAt != nil {
		expiresAt.Valid = true
		expiresAt.Time = *metadata.ExpiresAt
	}

	// Insert or update metadata record
	_, err = tx.tx.ExecContext(ctx, `
		INSERT INTO mcp.github_content_metadata (
			id, owner, repo, content_type, content_id, checksum, uri, 
			size, created_at, updated_at, expires_at, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) ON CONFLICT (id) DO UPDATE SET
			checksum = EXCLUDED.checksum,
			uri = EXCLUDED.uri,
			size = EXCLUDED.size,
			updated_at = EXCLUDED.updated_at,
			expires_at = EXCLUDED.expires_at,
			metadata = EXCLUDED.metadata
	`,
		id,
		metadata.Owner,
		metadata.Repo,
		string(metadata.ContentType),
		metadata.ContentID,
		metadata.Checksum,
		metadata.URI,
		metadata.Size,
		metadata.CreatedAt,
		metadata.UpdatedAt,
		expiresAt,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert GitHub content metadata: %w", err)
	}

	return nil
}

// GetGitHubContent retrieves GitHub content metadata from the database
func (db *Database) GetGitHubContent(ctx context.Context, owner, repo, contentType, contentID string) (*storage.ContentMetadata, error) {
	var metadata *storage.ContentMetadata

	err := db.Transaction(ctx, func(tx *sqlx.Tx) error {
		var err error
		metadata, err = db.getGitHubContent(ctx, &Tx{tx: tx}, owner, repo, contentType, contentID)
		return err
	})

	return metadata, err
}

// getGitHubContent is the internal implementation to retrieve GitHub content metadata within a transaction
func (db *Database) getGitHubContent(ctx context.Context, tx *Tx, owner, repo, contentType, contentID string) (*storage.ContentMetadata, error) {
	var (
		id          string
		ownerVal    string
		repoVal     string
		typeVal     string
		idVal       string
		checksum    string
		uri         string
		size        int64
		createdAt   time.Time
		updatedAt   time.Time
		expiresAt   sql.NullTime
		metadataRaw []byte
	)

	err := tx.tx.QueryRowContext(ctx, `
		SELECT id, owner, repo, content_type, content_id, checksum, uri, 
			   size, created_at, updated_at, expires_at, metadata
		FROM mcp.github_content_metadata
		WHERE owner = $1 AND repo = $2 AND content_type = $3 AND content_id = $4
	`, owner, repo, contentType, contentID).Scan(
		&id,
		&ownerVal,
		&repoVal,
		&typeVal,
		&idVal,
		&checksum,
		&uri,
		&size,
		&createdAt,
		&updatedAt,
		&expiresAt,
		&metadataRaw,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("GitHub content metadata not found")
		}
		return nil, fmt.Errorf("failed to get GitHub content metadata: %w", err)
	}

	// Parse metadata
	var metadataMap map[string]any
	if len(metadataRaw) > 0 {
		err = json.Unmarshal(metadataRaw, &metadataMap)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Create metadata object
	result := &storage.ContentMetadata{
		Owner:       ownerVal,
		Repo:        repoVal,
		ContentType: storage.ContentType(typeVal),
		ContentID:   idVal,
		Checksum:    checksum,
		URI:         uri,
		Size:        size,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		Metadata:    metadataMap,
	}

	if expiresAt.Valid {
		result.ExpiresAt = &expiresAt.Time
	}

	return result, nil
}

// DeleteGitHubContent deletes GitHub content metadata from the database
func (db *Database) DeleteGitHubContent(ctx context.Context, owner, repo, contentType, contentID string) error {
	return db.Transaction(ctx, func(tx *sqlx.Tx) error {
		return db.deleteGitHubContent(ctx, &Tx{tx: tx}, owner, repo, contentType, contentID)
	})
}

// deleteGitHubContent is the internal implementation to delete GitHub content metadata within a transaction
func (db *Database) deleteGitHubContent(ctx context.Context, tx *Tx, owner, repo, contentType, contentID string) error {
	_, err := tx.tx.ExecContext(ctx, `
		DELETE FROM mcp.github_content_metadata
		WHERE owner = $1 AND repo = $2 AND content_type = $3 AND content_id = $4
	`, owner, repo, contentType, contentID)

	if err != nil {
		return fmt.Errorf("failed to delete GitHub content metadata: %w", err)
	}

	return nil
}

// ListGitHubContent lists GitHub content metadata for a repository
func (db *Database) ListGitHubContent(ctx context.Context, owner, repo, contentType string, limit int) ([]*storage.ContentMetadata, error) {
	var metadataList []*storage.ContentMetadata

	err := db.Transaction(ctx, func(tx *sqlx.Tx) error {
		var err error
		metadataList, err = db.listGitHubContent(ctx, &Tx{tx: tx}, owner, repo, contentType, limit)
		return err
	})

	return metadataList, err
}

// listGitHubContent is the internal implementation to list GitHub content metadata within a transaction
func (db *Database) listGitHubContent(ctx context.Context, tx *Tx, owner, repo, contentType string, limit int) ([]*storage.ContentMetadata, error) {
	query := `
		SELECT id, owner, repo, content_type, content_id, checksum, uri, 
			   size, created_at, updated_at, expires_at, metadata
		FROM mcp.github_content_metadata
		WHERE owner = $1 AND repo = $2
	`

	args := []any{owner, repo}
	argIndex := 3

	if contentType != "" {
		query += fmt.Sprintf(" AND content_type = $%d", argIndex)
		args = append(args, contentType)
		argIndex++
	}

	query += " ORDER BY updated_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
	}

	rows, err := tx.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query GitHub content metadata: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// GitHub content - log but don't fail
			_ = err
		}
	}()

	var results []*storage.ContentMetadata

	for rows.Next() {
		var (
			id          string
			ownerVal    string
			repoVal     string
			typeVal     string
			idVal       string
			checksum    string
			uri         string
			size        int64
			createdAt   time.Time
			updatedAt   time.Time
			expiresAt   sql.NullTime
			metadataRaw []byte
		)

		err := rows.Scan(
			&id,
			&ownerVal,
			&repoVal,
			&typeVal,
			&idVal,
			&checksum,
			&uri,
			&size,
			&createdAt,
			&updatedAt,
			&expiresAt,
			&metadataRaw,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan GitHub content metadata: %w", err)
		}

		// Parse metadata
		var metadataMap map[string]any
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &metadataMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		// Create metadata object
		metadata := &storage.ContentMetadata{
			Owner:       ownerVal,
			Repo:        repoVal,
			ContentType: storage.ContentType(typeVal),
			ContentID:   idVal,
			Checksum:    checksum,
			URI:         uri,
			Size:        size,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
			Metadata:    metadataMap,
		}

		if expiresAt.Valid {
			metadata.ExpiresAt = &expiresAt.Time
		}

		results = append(results, metadata)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over GitHub content metadata: %w", err)
	}

	return results, nil
}

// GetGitHubContentByChecksum retrieves GitHub content metadata from the database using checksum
func (db *Database) GetGitHubContentByChecksum(ctx context.Context, checksum string) (*storage.ContentMetadata, error) {
	var metadata *storage.ContentMetadata

	err := db.Transaction(ctx, func(tx *sqlx.Tx) error {
		var err error
		metadata, err = db.getGitHubContentByChecksum(ctx, &Tx{tx: tx}, checksum)
		return err
	})

	return metadata, err
}

// getGitHubContentByChecksum is the internal implementation to retrieve GitHub content metadata by checksum within a transaction
func (db *Database) getGitHubContentByChecksum(ctx context.Context, tx *Tx, checksum string) (*storage.ContentMetadata, error) {
	var (
		id          string
		ownerVal    string
		repoVal     string
		typeVal     string
		idVal       string
		checksumVal string
		uri         string
		size        int64
		createdAt   time.Time
		updatedAt   time.Time
		expiresAt   sql.NullTime
		metadataRaw []byte
	)

	err := tx.tx.QueryRowContext(ctx, `
		SELECT id, owner, repo, content_type, content_id, checksum, uri, 
		       size, created_at, updated_at, expires_at, metadata
		FROM mcp.github_content_metadata
		WHERE checksum = $1
		LIMIT 1
	`, checksum).Scan(
		&id,
		&ownerVal,
		&repoVal,
		&typeVal,
		&idVal,
		&checksumVal,
		&uri,
		&size,
		&createdAt,
		&updatedAt,
		&expiresAt,
		&metadataRaw,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil without error when no content is found
		}
		return nil, fmt.Errorf("failed to get GitHub content metadata by checksum: %w", err)
	}

	// Parse metadata
	var metadataMap map[string]any
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &metadataMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Create metadata object
	metadata := &storage.ContentMetadata{
		Owner:       ownerVal,
		Repo:        repoVal,
		ContentType: storage.ContentType(typeVal),
		ContentID:   idVal,
		Checksum:    checksumVal,
		URI:         uri,
		Size:        size,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		Metadata:    metadataMap,
	}

	if expiresAt.Valid {
		metadata.ExpiresAt = &expiresAt.Time
	}

	return metadata, nil
}

// Initialize database tables for GitHub content metadata
func (db *Database) ensureGitHubContentTables(ctx context.Context) error {
	// Create GitHub content metadata table if it doesn't exist
	_, err := db.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mcp.github_content_metadata (
			id VARCHAR(255) PRIMARY KEY,
			owner VARCHAR(255) NOT NULL,
			repo VARCHAR(255) NOT NULL,
			content_type VARCHAR(50) NOT NULL,
			content_id VARCHAR(255) NOT NULL,
			checksum VARCHAR(64) NOT NULL,
			uri VARCHAR(1024) NOT NULL,
			size BIGINT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE,
			metadata JSONB NOT NULL DEFAULT '{}'
		)
	`)

	if err != nil {
		return fmt.Errorf("failed to create github_content_metadata table: %w", err)
	}

	// Create indexes for efficient querying
	_, err = db.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_github_content_owner_repo ON mcp.github_content_metadata(owner, repo);
		CREATE INDEX IF NOT EXISTS idx_github_content_type ON mcp.github_content_metadata(content_type);
		CREATE INDEX IF NOT EXISTS idx_github_content_checksum ON mcp.github_content_metadata(checksum);
		CREATE INDEX IF NOT EXISTS idx_github_content_updated_at ON mcp.github_content_metadata(updated_at);
	`)

	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
