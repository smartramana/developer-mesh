package core

import (
	"context"

	internaldb "github.com/S-Corkum/devops-mcp/pkg/database"
	pkgdb "github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/storage"
	"github.com/jmoiron/sqlx"
)

// DatabaseAdapter provides compatibility between internal/database and pkg/database
// This adapter allows us to migrate code incrementally without breaking existing functionality
type DatabaseAdapter struct {
	db     *sqlx.DB
	pkgDB  *pkgdb.Database
	logger observability.Logger
}

// NewDatabaseAdapter creates a new adapter that wraps a pkg/database.Database instance
// but exposes it with the same interface as internal/database.Database
func NewDatabaseAdapter(db *sqlx.DB, logger observability.Logger) (*DatabaseAdapter, error) {
	pkgDatabase, err := pkgdb.NewDatabase(db, nil, logger)
	if err != nil {
		return nil, err
	}

	return &DatabaseAdapter{
		db:     db,
		pkgDB:  pkgDatabase,
		logger: logger,
	}, nil
}

// GetDB returns the underlying sqlx.DB instance
func (a *DatabaseAdapter) GetDB() *sqlx.DB {
	return a.db
}

// StoreGitHubContent stores GitHub content metadata in the database
// This method adapts the pkg/database implementation to match the internal/database interface
func (a *DatabaseAdapter) StoreGitHubContent(ctx context.Context, metadata *storage.ContentMetadata) error {
	// Create a transaction
	tx, err := a.pkgDB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Convert to the pkg content storage format
	// This is a simplified adaptation - in a real implementation, you'd map all fields
	pkgMetadata := pkgdb.ContentMetadata{
		Owner:       metadata.Owner,
		Repo:        metadata.Repo,
		ContentType: metadata.ContentType,
		ContentID:   metadata.ContentID,
		URI:         metadata.URI,
		Checksum:    metadata.Checksum,
		Size:        metadata.Size,
		CreatedAt:   metadata.CreatedAt,
		UpdatedAt:   metadata.UpdatedAt,
		Metadata:    metadata.Metadata,
	}

	// Use the pkg database implementation
	err = a.pkgDB.ContentStorage.StoreContent(ctx, tx, &pkgMetadata)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetGitHubContent retrieves GitHub content metadata from the database
func (a *DatabaseAdapter) GetGitHubContent(ctx context.Context, owner, repo, contentType, contentID string) (*storage.ContentMetadata, error) {
	// Create a read-only transaction
	tx, err := a.pkgDB.BeginTx(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Use the pkg database implementation
	pkgMetadata, err := a.pkgDB.ContentStorage.GetContent(ctx, tx, owner, repo, contentType, contentID)
	if err != nil {
		return nil, err
	}

	// Convert back to internal storage format
	metadata := &storage.ContentMetadata{
		Owner:       pkgMetadata.Owner,
		Repo:        pkgMetadata.Repo,
		ContentType: pkgMetadata.ContentType,
		ContentID:   pkgMetadata.ContentID,
		URI:         pkgMetadata.URI,
		Checksum:    pkgMetadata.Checksum,
		Size:        pkgMetadata.Size,
		CreatedAt:   pkgMetadata.CreatedAt,
		UpdatedAt:   pkgMetadata.UpdatedAt,
		Metadata:    pkgMetadata.Metadata,
	}

	return metadata, nil
}

// GetGitHubContentByChecksum retrieves GitHub content metadata by checksum
func (a *DatabaseAdapter) GetGitHubContentByChecksum(ctx context.Context, checksum string) (*storage.ContentMetadata, error) {
	// Create a read-only transaction
	tx, err := a.pkgDB.BeginTx(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Use the pkg database implementation
	pkgMetadata, err := a.pkgDB.ContentStorage.GetContentByChecksum(ctx, tx, checksum)
	if err != nil {
		return nil, err
	}

	// Convert back to internal storage format
	metadata := &storage.ContentMetadata{
		Owner:       pkgMetadata.Owner,
		Repo:        pkgMetadata.Repo,
		ContentType: pkgMetadata.ContentType,
		ContentID:   pkgMetadata.ContentID,
		URI:         pkgMetadata.URI,
		Checksum:    pkgMetadata.Checksum,
		Size:        pkgMetadata.Size,
		CreatedAt:   pkgMetadata.CreatedAt,
		UpdatedAt:   pkgMetadata.UpdatedAt,
		Metadata:    pkgMetadata.Metadata,
	}

	return metadata, nil
}

// DeleteGitHubContent deletes GitHub content metadata from the database
func (a *DatabaseAdapter) DeleteGitHubContent(ctx context.Context, owner, repo, contentType, contentID string) error {
	// Create a transaction
	tx, err := a.pkgDB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Use the pkg database implementation
	err = a.pkgDB.ContentStorage.DeleteContent(ctx, tx, owner, repo, contentType, contentID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ListGitHubContent lists GitHub content metadata from the database
func (a *DatabaseAdapter) ListGitHubContent(ctx context.Context, owner, repo, contentType string, limit int) ([]*storage.ContentMetadata, error) {
	// Create a read-only transaction
	tx, err := a.pkgDB.BeginTx(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Use the pkg database implementation
	pkgMetadataList, err := a.pkgDB.ContentStorage.ListContent(ctx, tx, owner, repo, contentType, limit)
	if err != nil {
		return nil, err
	}

	// Convert back to internal storage format
	metadataList := make([]*storage.ContentMetadata, len(pkgMetadataList))
	for i, pkgMetadata := range pkgMetadataList {
		metadataList[i] = &storage.ContentMetadata{
			Owner:       pkgMetadata.Owner,
			Repo:        pkgMetadata.Repo,
			ContentType: pkgMetadata.ContentType,
			ContentID:   pkgMetadata.ContentID,
			URI:         pkgMetadata.URI,
			Checksum:    pkgMetadata.Checksum,
			Size:        pkgMetadata.Size,
			CreatedAt:   pkgMetadata.CreatedAt,
			UpdatedAt:   pkgMetadata.UpdatedAt,
			Metadata:    pkgMetadata.Metadata,
		}
	}

	return metadataList, nil
}
