package chunking

import (
	"context"
	"encoding/json"
	"fmt"
	
	"github.com/S-Corkum/devops-mcp/pkg/storage"
)

// GitHubContentStorageInterface defines the interface for GitHub content storage operations
type GitHubContentStorageInterface interface {
	// StoreContent stores content in storage
	StoreContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string, data []byte, metadata map[string]interface{}) (*storage.ContentMetadata, error)
	
	// GetContent retrieves content from storage
	GetContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string) ([]byte, *storage.ContentMetadata, error)
	
	// GetContentByURI retrieves content by URI
	GetContentByURI(ctx context.Context, uri string) ([]byte, *storage.ContentMetadata, error)
	
	// ListContent lists content of a given type
	ListContent(ctx context.Context, owner string, repo string, contentType storage.ContentType) ([]*storage.ContentMetadata, error)
	
	// DeleteContent deletes content from storage
	DeleteContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string) error
	
	// GetS3Client returns the S3 client
	GetS3Client() storage.S3ClientInterface
}

// ChunkingManager handles storage and retrieval of code chunks
type ChunkingManager struct {
	chunkingService *ChunkingService
	contentStorage  GitHubContentStorageInterface
}

// NewChunkingManager creates a new ChunkingManager
func NewChunkingManager(chunkingService *ChunkingService, contentStorage GitHubContentStorageInterface) *ChunkingManager {
	return &ChunkingManager{
		chunkingService: chunkingService,
		contentStorage:  contentStorage,
	}
}

// ChunkAndStoreFile chunks a file and stores the chunks in the content storage
func (m *ChunkingManager) ChunkAndStoreFile(
	ctx context.Context,
	owner string,
	repo string,
	code []byte,
	filename string,
	fileMetadata map[string]interface{},
) ([]*CodeChunk, error) {
	// Chunk the code
	chunks, err := m.chunkingService.ChunkCode(ctx, string(code), filename)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk code: %w", err)
	}
	
	// Store chunks
	for _, chunk := range chunks {
		// Convert chunk to JSON
		chunkData, err := json.Marshal(chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal chunk: %w", err)
		}
		
		// Create metadata for storage
		metadata := map[string]interface{}{
			"chunk_id":   chunk.ID,
			"chunk_type": string(chunk.Type),
			"language":   string(chunk.Language),
			"path":       chunk.Path,
			"name":       chunk.Name,
			"start_line": chunk.StartLine,
			"end_line":   chunk.EndLine,
		}
		
		// Add file metadata
		if fileMetadata != nil {
			for k, v := range fileMetadata {
				metadata[k] = v
			}
		}
		
		// Add chunk specific metadata
		if chunk.Metadata != nil {
			for k, v := range chunk.Metadata {
				metadata["chunk_"+k] = v
			}
		}
		
		// Add parent and dependencies information
		if chunk.ParentID != "" {
			metadata["parent_id"] = chunk.ParentID
		}
		
		if len(chunk.Dependencies) > 0 {
			metadata["dependencies"] = chunk.Dependencies
		}
		
		// Store the chunk using content-addressable storage
		contentID := fmt.Sprintf("%s_%s_%s", chunk.ID, chunk.Type, chunk.Name)
		_, err = m.contentStorage.StoreContent(
			ctx,
			owner,
			repo,
			storage.ContentTypeFile, // Use file content type for code chunks
			contentID,
			chunkData,
			metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to store chunk: %w", err)
		}
	}
	
	return chunks, nil
}

// GetChunk retrieves a chunk by ID
func (m *ChunkingManager) GetChunk(
	ctx context.Context,
	owner string,
	repo string,
	chunkID string,
) (*CodeChunk, error) {
	// List all chunks for the repo
	chunks, err := m.ListChunks(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	
	// Find the chunk with the matching ID
	for _, chunk := range chunks {
		if chunk.ID == chunkID {
			return chunk, nil
		}
	}
	
	return nil, fmt.Errorf("chunk not found: %s", chunkID)
}

// ListChunks lists all chunks for a repository
func (m *ChunkingManager) ListChunks(
	ctx context.Context,
	owner string,
	repo string,
) ([]*CodeChunk, error) {
	// Get all file content from the content storage
	contentMetadata, err := m.contentStorage.ListContent(ctx, owner, repo, storage.ContentTypeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to list content: %w", err)
	}
	
	var chunks []*CodeChunk
	
	for _, metadata := range contentMetadata {
		// Check if this is a code chunk by looking for chunk_id in metadata
		_, ok := metadata.GetMetadata()["chunk_id"]
		if !ok {
			continue
		}
		
		// Get the chunk data
		chunkData, _, err := m.contentStorage.GetContent(
			ctx,
			owner,
			repo,
			storage.ContentTypeFile,
			metadata.ContentID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get chunk data: %w", err)
		}
		
		// Unmarshal the chunk
		var chunk CodeChunk
		err = json.Unmarshal(chunkData, &chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal chunk: %w", err)
		}
		
		chunks = append(chunks, &chunk)
	}
	
	return chunks, nil
}

// GetChunksByType gets chunks of a specific type
func (m *ChunkingManager) GetChunksByType(
	ctx context.Context,
	owner string,
	repo string,
	chunkType ChunkType,
) ([]*CodeChunk, error) {
	// Get all chunks
	allChunks, err := m.ListChunks(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	
	// Filter by type
	var filteredChunks []*CodeChunk
	for _, chunk := range allChunks {
		if chunk.Type == chunkType {
			filteredChunks = append(filteredChunks, chunk)
		}
	}
	
	return filteredChunks, nil
}

// GetChunksByLanguage gets chunks of a specific language
func (m *ChunkingManager) GetChunksByLanguage(
	ctx context.Context,
	owner string,
	repo string,
	language Language,
) ([]*CodeChunk, error) {
	// Get all chunks
	allChunks, err := m.ListChunks(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	
	// Filter by language
	var filteredChunks []*CodeChunk
	for _, chunk := range allChunks {
		if chunk.Language == language {
			filteredChunks = append(filteredChunks, chunk)
		}
	}
	
	return filteredChunks, nil
}

// GetRelatedChunks gets chunks related to a specific chunk (dependencies, parent, children)
func (m *ChunkingManager) GetRelatedChunks(
	ctx context.Context,
	owner string,
	repo string,
	chunkID string,
) ([]*CodeChunk, error) {
	// Get the specified chunk
	chunk, err := m.GetChunk(ctx, owner, repo, chunkID)
	if err != nil {
		return nil, err
	}
	
	// Get all chunks to find related ones
	allChunks, err := m.ListChunks(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	
	// Find related chunks
	var relatedChunks []*CodeChunk
	relatedIDs := make(map[string]bool)
	
	// Add dependencies
	for _, depID := range chunk.Dependencies {
		relatedIDs[depID] = true
	}
	
	// Add parent
	if chunk.ParentID != "" {
		relatedIDs[chunk.ParentID] = true
	}
	
	// Add children (chunks where this chunk is the parent)
	for _, otherChunk := range allChunks {
		if otherChunk.ParentID == chunk.ID {
			relatedIDs[otherChunk.ID] = true
		}
		
		// Add chunks that depend on this chunk
		for _, depID := range otherChunk.Dependencies {
			if depID == chunk.ID {
				relatedIDs[otherChunk.ID] = true
				break
			}
		}
	}
	
	// Get all related chunks
	for _, otherChunk := range allChunks {
		if otherChunk.ID == chunk.ID {
			continue // Skip the original chunk
		}
		
		if relatedIDs[otherChunk.ID] {
			relatedChunks = append(relatedChunks, otherChunk)
		}
	}
	
	return relatedChunks, nil
}
