package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	s3client "github.com/S-Corkum/mcp-server/internal/storage"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// S3ContextStorage implements context storage using AWS S3
type S3ContextStorage struct {
	s3Client   *s3client.S3Client
	bucketName string
	prefix     string
}

// NewS3ContextStorage creates a new S3 context storage provider
func NewS3ContextStorage(s3Client *s3client.S3Client, prefix string) *S3ContextStorage {
	return &S3ContextStorage{
		s3Client:   s3Client,
		bucketName: s3Client.GetBucketName(),
		prefix:     prefix,
	}
}

// StoreContext stores a context in S3
func (s *S3ContextStorage) StoreContext(ctx context.Context, contextData *mcp.Context) error {
	// Generate key for the context
	key := s.generateContextKey(contextData.ID)
	
	// Serialize context data to JSON
	jsonData, err := json.Marshal(contextData)
	if err != nil {
		return fmt.Errorf("failed to serialize context data: %w", err)
	}
	
	// Upload to S3
	err = s.s3Client.UploadFile(ctx, key, jsonData, "application/json")
	if err != nil {
		return fmt.Errorf("failed to upload context to S3: %w", err)
	}
	
	return nil
}

// GetContext retrieves a context from S3
func (s *S3ContextStorage) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	// Generate key for the context
	key := s.generateContextKey(contextID)
	
	// Download from S3
	data, err := s.s3Client.DownloadFile(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to download context from S3: %w", err)
	}
	
	// Deserialize context data
	var contextData mcp.Context
	err = json.Unmarshal(data, &contextData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize context data: %w", err)
	}
	
	return &contextData, nil
}

// DeleteContext deletes a context from S3
func (s *S3ContextStorage) DeleteContext(ctx context.Context, contextID string) error {
	// Generate key for the context
	key := s.generateContextKey(contextID)
	
	// Delete from S3
	err := s.s3Client.DeleteFile(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete context from S3: %w", err)
	}
	
	return nil
}

// ListContexts lists contexts from S3
func (s *S3ContextStorage) ListContexts(ctx context.Context, agentID string, sessionID string) ([]*mcp.Context, error) {
	// Generate prefix for listing
	listPrefix := s.prefix
	if agentID != "" {
		listPrefix = fmt.Sprintf("%s/agent/%s", s.prefix, agentID)
	}
	
	// List objects from S3
	keys, err := s.s3Client.ListFiles(ctx, listPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts from S3: %w", err)
	}
	
	// Retrieve each context
	var contexts []*mcp.Context
	for _, key := range keys {
		// Extract context ID from key
		contextID := s.extractContextID(key)
		if contextID == "" {
			continue
		}
		
		// Get the context
		contextData, err := s.GetContext(ctx, contextID)
		if err != nil {
			log.Printf("Warning: failed to get context %s: %v", contextID, err)
			continue
		}
		
		// Filter by session ID if provided
		if sessionID != "" && contextData.SessionID != sessionID {
			continue
		}
		
		contexts = append(contexts, contextData)
	}
	
	return contexts, nil
}

// generateContextKey generates an S3 key for a context
func (s *S3ContextStorage) generateContextKey(contextID string) string {
	return fmt.Sprintf("%s/%s.json", s.prefix, contextID)
}

// extractContextID extracts the context ID from an S3 key
func (s *S3ContextStorage) extractContextID(key string) string {
	// Implementation depends on the key format
	// For simplicity, we'll assume the key format is prefix/contextID.json
	// In a real implementation, we'd use proper path handling
	
	// This is a simplified example
	filename := key[len(s.prefix)+1:]
	if len(filename) > 5 && filename[len(filename)-5:] == ".json" {
		return filename[:len(filename)-5]
	}
	return ""
}
