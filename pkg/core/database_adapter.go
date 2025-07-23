package core

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/storage"
	"github.com/jmoiron/sqlx"
)

// DatabaseAdapter provides compatibility between internal/database and pkg/database
// This adapter allows us to migrate code incrementally without breaking existing functionality
type DatabaseAdapter struct {
	db     *sqlx.DB
	pkgDB  *database.Database
	logger observability.Logger
}

// NewDatabaseAdapter creates a new adapter that wraps a pkg/database.Database instance
// but exposes it with the same interface as internal/database.Database
func NewDatabaseAdapter(db *sqlx.DB, logger observability.Logger) (*DatabaseAdapter, error) {
	// Use existing connection through the constructor that takes an sqlx.DB
	pkgDatabase := database.NewDatabaseWithConnection(db)

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
	// Use the pkg database implementation directly
	return a.pkgDB.StoreGitHubContent(ctx, metadata)
}

// GetGitHubContent retrieves GitHub content metadata from the database
func (a *DatabaseAdapter) GetGitHubContent(ctx context.Context, owner, repo, contentType, contentID string) (*storage.ContentMetadata, error) {
	// Use the pkg database implementation directly
	return a.pkgDB.GetGitHubContent(ctx, owner, repo, contentType, contentID)
}

// GetGitHubContentByChecksum retrieves GitHub content metadata by checksum
func (a *DatabaseAdapter) GetGitHubContentByChecksum(ctx context.Context, checksum string) (*storage.ContentMetadata, error) {
	// Use the pkg database implementation directly
	return a.pkgDB.GetGitHubContentByChecksum(ctx, checksum)
}

// DeleteGitHubContent deletes GitHub content metadata from the database
func (a *DatabaseAdapter) DeleteGitHubContent(ctx context.Context, owner, repo, contentType, contentID string) error {
	// Use the pkg database implementation directly
	return a.pkgDB.DeleteGitHubContent(ctx, owner, repo, contentType, contentID)
}

// ListGitHubContent lists GitHub content metadata from the database
func (a *DatabaseAdapter) ListGitHubContent(ctx context.Context, owner, repo, contentType string, limit int) ([]*storage.ContentMetadata, error) {
	// Use the pkg database implementation directly
	return a.pkgDB.ListGitHubContent(ctx, owner, repo, contentType, limit)
}
