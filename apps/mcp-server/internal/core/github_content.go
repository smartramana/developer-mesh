package core

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/models/relationship"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/storage"
)

// GitHubContentManager manages GitHub content storage and retrieval
type GitHubContentManager struct {
	db                  *database.Database
	storageManager      *storage.GitHubContentStorage
	logger              observability.Logger
	metricsClient       observability.MetricsClient
	relationshipManager *GitHubRelationshipManager
}

// NewGitHubContentManager creates a new GitHub content manager
func NewGitHubContentManager(
	db *database.Database,
	s3Client *storage.S3Client,
	metricsClient observability.MetricsClient,
	relationshipService relationship.Service,
) (*GitHubContentManager, error) {
	// Create storage manager
	storageManager := storage.NewGitHubContentStorage(s3Client)

	// Create logger
	logger := observability.NewLogger("github-content-manager")

	// Create content manager instance
	manager := &GitHubContentManager{
		db:             db,
		storageManager: storageManager,
		logger:         logger,
		metricsClient:  metricsClient,
	}

	// Create relationship manager if service is provided
	if relationshipService != nil {
		manager.relationshipManager = NewGitHubRelationshipManager(relationshipService, manager)
	}

	return manager, nil
}

// StoreContent stores GitHub content in S3 and indexes it in the database
func (m *GitHubContentManager) StoreContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
	data []byte,
	metadata map[string]interface{},
) (*storage.ContentMetadata, error) {
	startTime := time.Now()
	success := false
	defer func() {
		// Record metrics
		duration := time.Since(startTime).Seconds()
		m.metricsClient.RecordOperation(
			"github_content_manager",
			"store_content",
			success,
			duration,
			map[string]string{
				"owner":        owner,
				"repo":         repo,
				"content_type": string(contentType),
			},
		)
	}()

	// Store content in S3
	contentMetadata, err := m.storageManager.StoreContent(
		ctx, owner, repo, contentType, contentID, data, metadata,
	)
	if err != nil {
		m.logger.Error("Failed to store content in S3", map[string]interface{}{
			"error":        err.Error(),
			"owner":        owner,
			"repo":         repo,
			"content_type": contentType,
			"content_id":   contentID,
		})
		return nil, fmt.Errorf("failed to store content in S3: %w", err)
	}

	// Index metadata in database
	err = m.db.StoreGitHubContent(ctx, contentMetadata)
	if err != nil {
		m.logger.Error("Failed to store content metadata in database", map[string]interface{}{
			"error":        err.Error(),
			"owner":        owner,
			"repo":         repo,
			"content_type": contentType,
			"content_id":   contentID,
		})
		return nil, fmt.Errorf("failed to store content metadata in database: %w", err)
	}

	// Process relationships if relationship manager is available
	if m.relationshipManager != nil {
		err = m.relationshipManager.ProcessContentRelationships(ctx, contentMetadata, data)
		if err != nil {
			// Log but don't fail the operation
			m.logger.Warn("Failed to process content relationships", map[string]interface{}{
				"error":        err.Error(),
				"owner":        owner,
				"repo":         repo,
				"content_type": contentType,
				"content_id":   contentID,
			})
		}
	}

	m.logger.Info("Stored GitHub content", map[string]interface{}{
		"owner":        owner,
		"repo":         repo,
		"content_type": contentType,
		"content_id":   contentID,
		"checksum":     contentMetadata.Checksum,
		"uri":          contentMetadata.URI,
		"size":         contentMetadata.Size,
	})

	success = true
	return contentMetadata, nil
}

// GetContent retrieves GitHub content by owner, repo, type, and ID
func (m *GitHubContentManager) GetContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
) ([]byte, *storage.ContentMetadata, error) {
	startTime := time.Now()
	success := false
	defer func() {
		// Record metrics
		duration := time.Since(startTime).Seconds()
		m.metricsClient.RecordOperation(
			"github_content_manager",
			"get_content",
			success,
			duration,
			map[string]string{
				"owner":        owner,
				"repo":         repo,
				"content_type": string(contentType),
			},
		)
	}()

	// Check database first for metadata
	metadata, err := m.db.GetGitHubContent(ctx, owner, repo, string(contentType), contentID)
	if err == nil && metadata != nil {
		// Get content by URI from S3
		content, _, err := m.storageManager.GetContentByURI(ctx, metadata.URI)
		if err != nil {
			m.logger.Error("Failed to get content from S3 by URI", map[string]interface{}{
				"error": err.Error(),
				"uri":   metadata.URI,
			})
			return nil, metadata, fmt.Errorf("failed to get content from S3: %w", err)
		}

		// Process relationships if relationship manager is available and content was retrieved
		if m.relationshipManager != nil && len(content) > 0 {
			err = m.relationshipManager.ProcessContentRelationships(ctx, metadata, content)
			if err != nil {
				// Log but don't fail the operation
				m.logger.Warn("Failed to process content relationships during retrieval", map[string]interface{}{
					"error":        err.Error(),
					"owner":        metadata.Owner,
					"repo":         metadata.Repo,
					"content_type": metadata.ContentType,
					"content_id":   metadata.ContentID,
				})
			}
		}

		m.logger.Info("Retrieved GitHub content from database reference", map[string]interface{}{
			"owner":        owner,
			"repo":         repo,
			"content_type": contentType,
			"content_id":   contentID,
			"uri":          metadata.URI,
		})

		success = true
		return content, metadata, nil
	}

	// If not found in database, try direct retrieval from S3
	content, metadata, err := m.storageManager.GetContent(ctx, owner, repo, contentType, contentID)
	if err != nil {
		m.logger.Error("Failed to get content from S3", map[string]interface{}{
			"error":        err.Error(),
			"owner":        owner,
			"repo":         repo,
			"content_type": contentType,
			"content_id":   contentID,
		})
		return nil, nil, fmt.Errorf("failed to get content from S3: %w", err)
	}

	// Store metadata in database for future queries
	if metadata != nil {
		err = m.db.StoreGitHubContent(ctx, metadata)
		if err != nil {
			// Log but don't fail
			m.logger.Warn("Failed to store content metadata in database after retrieval", map[string]interface{}{
				"error":        err.Error(),
				"owner":        owner,
				"repo":         repo,
				"content_type": contentType,
				"content_id":   contentID,
			})
		}
	}

	m.logger.Info("Retrieved GitHub content directly from S3", map[string]interface{}{
		"owner":        owner,
		"repo":         repo,
		"content_type": contentType,
		"content_id":   contentID,
	})

	success = true
	return content, metadata, nil
}

// GetContentByChecksum retrieves GitHub content by its checksum
func (m *GitHubContentManager) GetContentByChecksum(
	ctx context.Context,
	checksum string,
) ([]byte, *storage.ContentMetadata, error) {
	startTime := time.Now()
	success := false
	defer func() {
		// Record metrics
		duration := time.Since(startTime).Seconds()
		m.metricsClient.RecordOperation(
			"github_content_manager",
			"get_content_by_checksum",
			success,
			duration,
			map[string]string{
				"checksum": checksum,
			},
		)
	}()

	// Generate the content-addressable key
	contentKey := fmt.Sprintf("%s/%s", "content", checksum)
	uri := fmt.Sprintf("s3://%s/%s", m.storageManager.GetS3Client().GetBucketName(), contentKey)

	// Get content from S3
	content, metadata, err := m.storageManager.GetContentByURI(ctx, uri)
	if err != nil {
		m.logger.Error("Failed to get content from S3 by checksum", map[string]interface{}{
			"error":    err.Error(),
			"checksum": checksum,
			"uri":      uri,
		})
		return nil, nil, fmt.Errorf("failed to get content from S3 by checksum: %w", err)
	}

	m.logger.Info("Retrieved GitHub content by checksum", map[string]interface{}{
		"checksum": checksum,
		"uri":      uri,
	})

	success = true
	return content, metadata, nil
}

// DeleteContent deletes GitHub content
func (m *GitHubContentManager) DeleteContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
) error {
	startTime := time.Now()
	success := false
	defer func() {
		// Record metrics
		duration := time.Since(startTime).Seconds()
		m.metricsClient.RecordOperation(
			"github_content_manager",
			"delete_content",
			success,
			duration,
			map[string]string{
				"owner":        owner,
				"repo":         repo,
				"content_type": string(contentType),
			},
		)
	}()

	// Delete from database first
	err := m.db.DeleteGitHubContent(ctx, owner, repo, string(contentType), contentID)
	if err != nil {
		m.logger.Error("Failed to delete content metadata from database", map[string]interface{}{
			"error":        err.Error(),
			"owner":        owner,
			"repo":         repo,
			"content_type": contentType,
			"content_id":   contentID,
		})
		// Continue anyway to try deleting from S3
	}

	// Delete content references from S3
	err = m.storageManager.DeleteContent(ctx, owner, repo, contentType, contentID)
	if err != nil {
		m.logger.Error("Failed to delete content from S3", map[string]interface{}{
			"error":        err.Error(),
			"owner":        owner,
			"repo":         repo,
			"content_type": contentType,
			"content_id":   contentID,
		})
		return fmt.Errorf("failed to delete content from S3: %w", err)
	}

	m.logger.Info("Deleted GitHub content", map[string]interface{}{
		"owner":        owner,
		"repo":         repo,
		"content_type": contentType,
		"content_id":   contentID,
	})

	success = true
	return nil
}

// ListContent lists GitHub content for a repository
func (m *GitHubContentManager) ListContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	limit int,
) ([]*storage.ContentMetadata, error) {
	startTime := time.Now()
	success := false
	defer func() {
		// Record metrics
		duration := time.Since(startTime).Seconds()
		m.metricsClient.RecordOperation(
			"github_content_manager",
			"list_content",
			success,
			duration,
			map[string]string{
				"owner":        owner,
				"repo":         repo,
				"content_type": string(contentType),
			},
		)
	}()

	// First try to get metadata from database (faster)
	metadata, err := m.db.ListGitHubContent(ctx, owner, repo, string(contentType), limit)
	if err == nil && len(metadata) > 0 {
		m.logger.Info("Listed GitHub content from database", map[string]interface{}{
			"owner":        owner,
			"repo":         repo,
			"content_type": contentType,
			"count":        len(metadata),
		})

		success = true
		return metadata, nil
	}

	// If not found or error, try S3 directly
	metadata, err = m.storageManager.ListContent(ctx, owner, repo, contentType)
	if err != nil {
		m.logger.Error("Failed to list content from S3", map[string]interface{}{
			"error":        err.Error(),
			"owner":        owner,
			"repo":         repo,
			"content_type": contentType,
		})
		return nil, fmt.Errorf("failed to list content from S3: %w", err)
	}

	// If limit is provided, truncate the result
	if limit > 0 && len(metadata) > limit {
		metadata = metadata[:limit]
	}

	// Index metadata in database for future queries
	for _, meta := range metadata {
		err = m.db.StoreGitHubContent(ctx, meta)
		if err != nil {
			// Log but don't fail
			m.logger.Warn("Failed to store content metadata in database after listing", map[string]interface{}{
				"error":        err.Error(),
				"owner":        owner,
				"repo":         repo,
				"content_type": contentType,
				"content_id":   meta.ContentID,
			})
		}
	}

	m.logger.Info("Listed GitHub content from S3", map[string]interface{}{
		"owner":        owner,
		"repo":         repo,
		"content_type": contentType,
		"count":        len(metadata),
	})

	success = true
	return metadata, nil
}
