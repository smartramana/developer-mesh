package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ContentType represents the type of GitHub content being stored
type ContentType string

const (
	// Content types
	ContentTypeIssue       ContentType = "issue"
	ContentTypePullRequest ContentType = "pull_request"
	ContentTypeCommit      ContentType = "commit"
	ContentTypeFile        ContentType = "file"
	ContentTypeReference   ContentType = "reference"
	ContentTypeRelease     ContentType = "release"
	ContentTypeWebhook     ContentType = "webhook"
	
	// Storage paths
	repoPathPrefix   = "repositories"
	contentPathPrefix = "content"
)

// ContentMetadata holds metadata about stored GitHub content
type ContentMetadata struct {
	Owner       string                 `json:"owner"`
	Repo        string                 `json:"repo"`
	ContentType ContentType            `json:"content_type"`
	ContentID   string                 `json:"content_id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Size        int64                  `json:"size"`
	Checksum    string                 `json:"checksum"`
	URI         string                 `json:"uri"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// GitHubContentStorage manages storage of GitHub content in S3
type GitHubContentStorage struct {
	s3Client   *S3Client
	bucketName string
}

// NewGitHubContentStorage creates a new GitHub content storage manager
func NewGitHubContentStorage(s3Client *S3Client) *GitHubContentStorage {
	return &GitHubContentStorage{
		s3Client:   s3Client,
		bucketName: s3Client.GetBucketName(),
	}
}

// GetS3Client returns the underlying S3 client
func (s *GitHubContentStorage) GetS3Client() *S3Client {
	return s.s3Client
}

// StoreContent stores GitHub content in S3 using content-addressable storage
// It returns the ContentMetadata including the URI for the stored content
func (s *GitHubContentStorage) StoreContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType ContentType,
	contentID string,
	data []byte,
	metadata map[string]interface{},
) (*ContentMetadata, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("content data cannot be empty")
	}

	// Calculate SHA-256 hash of the content
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])
	
	// Store content by hash in the content-addressable store
	contentKey := fmt.Sprintf("%s/%s", contentPathPrefix, checksum)
	
	// Store content by repository path for organization
	repoKey := fmt.Sprintf("%s/%s/%s/%s/%s", 
		repoPathPrefix, owner, repo, contentType, contentID)
	
	// Create metadata
	now := time.Now()
	contentMetadata := &ContentMetadata{
		Owner:       owner,
		Repo:        repo,
		ContentType: contentType,
		ContentID:   contentID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Size:        int64(len(data)),
		Checksum:    checksum,
		URI:         fmt.Sprintf("s3://%s/%s", s.bucketName, contentKey),
		Metadata:    metadata,
	}

	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(contentMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content metadata: %w", err)
	}

	// Store content in the content-addressable path
	// Only store if it doesn't already exist (deduplicate by hash)
	exists, err := s.contentExists(ctx, checksum)
	if err != nil {
		return nil, fmt.Errorf("failed to check if content exists: %w", err)
	}
	
	if !exists {
		err = s.s3Client.UploadFile(ctx, contentKey, data, "application/octet-stream")
		if err != nil {
			return nil, fmt.Errorf("failed to upload content to S3: %w", err)
		}
	}

	// Store metadata in the repository path
	err = s.s3Client.UploadFile(ctx, repoKey, metadataJSON, "application/json")
	if err != nil {
		return nil, fmt.Errorf("failed to upload metadata to S3: %w", err)
	}

	// Store reference from repo path to content hash
	refData := []byte(contentKey)
	err = s.s3Client.UploadFile(ctx, repoKey+".ref", refData, "text/plain")
	if err != nil {
		return nil, fmt.Errorf("failed to upload reference to S3: %w", err)
	}

	return contentMetadata, nil
}

// contentExists checks if content with the given hash already exists
func (s *GitHubContentStorage) contentExists(ctx context.Context, checksum string) (bool, error) {
	contentKey := fmt.Sprintf("%s/%s", contentPathPrefix, checksum)
	
	// List objects with this prefix to check if any exist
	keys, err := s.s3Client.ListFiles(ctx, contentKey)
	if err != nil {
		return false, err
	}
	
	return len(keys) > 0, nil
}

// GetContentByURI retrieves content from S3 by URI
func (s *GitHubContentStorage) GetContentByURI(ctx context.Context, uri string) ([]byte, *ContentMetadata, error) {
	// Parse URI to extract key
	prefix := fmt.Sprintf("s3://%s/", s.bucketName)
	if !strings.HasPrefix(uri, prefix) {
		return nil, nil, fmt.Errorf("invalid URI format: %s", uri)
	}
	
	key := uri[len(prefix):]
	
	// Download content
	data, err := s.s3Client.DownloadFile(ctx, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download content from S3: %w", err)
	}
	
	// For content-addressable paths, retrieve the metadata
	if strings.HasPrefix(key, contentPathPrefix+"/") {
		checksum := strings.TrimPrefix(key, contentPathPrefix+"/")
		metadata, err := s.getContentMetadataByChecksum(ctx, checksum)
		if err != nil {
			return data, nil, fmt.Errorf("content found but metadata not found: %w", err)
		}
		return data, metadata, nil
	}
	
	return data, nil, nil
}

// GetContent retrieves GitHub content from S3
func (s *GitHubContentStorage) GetContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType ContentType,
	contentID string,
) ([]byte, *ContentMetadata, error) {
	// Generate repository path key
	repoKey := fmt.Sprintf("%s/%s/%s/%s/%s", 
		repoPathPrefix, owner, repo, contentType, contentID)
	
	// Get reference file first
	refData, err := s.s3Client.DownloadFile(ctx, repoKey+".ref")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download content reference: %w", err)
	}
	
	// Reference contains the content path
	contentKey := string(refData)
	
	// Download the actual content
	content, err := s.s3Client.DownloadFile(ctx, contentKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download content from S3: %w", err)
	}
	
	// Get metadata
	metadataContent, err := s.s3Client.DownloadFile(ctx, repoKey)
	if err != nil {
		return content, nil, fmt.Errorf("content found but metadata not found: %w", err)
	}
	
	var metadata ContentMetadata
	err = json.Unmarshal(metadataContent, &metadata)
	if err != nil {
		return content, nil, fmt.Errorf("failed to unmarshal content metadata: %w", err)
	}
	
	return content, &metadata, nil
}

// getContentMetadataByChecksum searches for metadata of content with the given checksum
func (s *GitHubContentStorage) getContentMetadataByChecksum(ctx context.Context, checksum string) (*ContentMetadata, error) {
	// List all repository objects
	repoObjects, err := s.s3Client.ListFiles(ctx, repoPathPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list repository objects: %w", err)
	}
	
	// Check each metadata file 
	for _, key := range repoObjects {
		// Skip reference files
		if strings.HasSuffix(key, ".ref") {
			continue
		}
		
		metadataContent, err := s.s3Client.DownloadFile(ctx, key)
		if err != nil {
			continue // Skip files that can't be downloaded
		}
		
		var metadata ContentMetadata
		err = json.Unmarshal(metadataContent, &metadata)
		if err != nil {
			continue // Skip files with invalid JSON
		}
		
		if metadata.Checksum == checksum {
			return &metadata, nil
		}
	}
	
	return nil, fmt.Errorf("metadata not found for checksum: %s", checksum)
}

// DeleteContent deletes GitHub content from S3
func (s *GitHubContentStorage) DeleteContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType ContentType,
	contentID string,
) error {
	// Generate repository path key
	repoKey := fmt.Sprintf("%s/%s/%s/%s/%s", 
		repoPathPrefix, owner, repo, contentType, contentID)
	
	// Get reference file first to find the content path
	refData, err := s.s3Client.DownloadFile(ctx, repoKey+".ref")
	if err != nil {
		return fmt.Errorf("failed to download content reference: %w", err)
	}
	
	// Log the content path being referenced (helpful for debugging)
	contentPath := string(refData)
	if contentPath == "" {
		return fmt.Errorf("invalid empty reference data")
	}
	
	// Delete the reference
	err = s.s3Client.DeleteFile(ctx, repoKey+".ref")
	if err != nil {
		return fmt.Errorf("failed to delete reference: %w", err)
	}
	
	// Delete the metadata
	err = s.s3Client.DeleteFile(ctx, repoKey)
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}
	
	// Note: We intentionally do not delete the content from the content-addressable store
	// as it may be referenced by other repositories. Content should be managed by lifecycle policies.
	
	return nil
}

// ListContent lists GitHub content for a repository
func (s *GitHubContentStorage) ListContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType ContentType,
) ([]*ContentMetadata, error) {
	// Generate prefix for listing
	prefix := fmt.Sprintf("%s/%s/%s/%s/", 
		repoPathPrefix, owner, repo, contentType)
	
	// List objects with this prefix
	keys, err := s.s3Client.ListFiles(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list content: %w", err)
	}
	
	var results []*ContentMetadata
	
	for _, key := range keys {
		// Skip reference files
		if strings.HasSuffix(key, ".ref") {
			continue
		}
		
		// Download metadata
		metadataContent, err := s.s3Client.DownloadFile(ctx, key)
		if err != nil {
			continue // Skip files that can't be downloaded
		}
		
		var metadata ContentMetadata
		err = json.Unmarshal(metadataContent, &metadata)
		if err != nil {
			continue // Skip files with invalid JSON
		}
		
		results = append(results, &metadata)
	}
	
	return results, nil
}

// CalculateContentHash calculates the SHA-256 hash for the given content
func CalculateContentHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CalculateContentHashFromReader calculates the SHA-256 hash for content from a reader
func CalculateContentHashFromReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
