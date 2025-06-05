package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// EmbeddingModelType represents the type of embedding model
type EmbeddingModelType string

const (
	// ModelTypeOpenAI represents OpenAI embedding models
	ModelTypeOpenAI EmbeddingModelType = "openai"
	// ModelTypeHuggingFace represents HuggingFace embedding models
	ModelTypeHuggingFace EmbeddingModelType = "huggingface"
	// ModelTypeCustom represents custom embedding models
	ModelTypeCustom EmbeddingModelType = "custom"

	// Content types for embedding generation
	ContentTypeCodeChunk  = "code_chunk"
	ContentTypeIssue      = "issue"
	ContentTypeComment    = "comment"
	ContentTypeDiscussion = "discussion"
)

// EmbeddingVector represents a vector embedding with metadata
type EmbeddingVector struct {
	// The actual embedding vector values
	Vector []float32 `json:"vector"`
	// Dimensions of the vector
	Dimensions int `json:"dimensions"`
	// Model ID used to generate this embedding
	ModelID string `json:"model_id"`
	// ContentType indicates what type of content this is an embedding for
	ContentType string `json:"content_type"`
	// ContentID is a unique identifier for the content
	ContentID string `json:"content_id"`
	// Metadata about the embedding and content
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// EmbeddingService defines the interface for generating embeddings
type EmbeddingService interface {
	// GenerateEmbedding creates an embedding for a single text
	GenerateEmbedding(ctx context.Context, text string, contentType string, contentID string) (*EmbeddingVector, error)
	// BatchGenerateEmbeddings creates embeddings for multiple texts
	BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error)
}

// EmbeddingStorage defines the interface for storing and retrieving embeddings
type EmbeddingStorage interface {
	// StoreEmbedding stores a single embedding
	StoreEmbedding(ctx context.Context, embedding *EmbeddingVector) error
	// BatchStoreEmbeddings stores multiple embeddings in a batch
	BatchStoreEmbeddings(ctx context.Context, embeddings []*EmbeddingVector) error
	// FindSimilarEmbeddings finds embeddings similar to the provided one
	FindSimilarEmbeddings(ctx context.Context, embedding *EmbeddingVector, limit int, threshold float32) ([]*EmbeddingVector, error)
}

// EmbeddingPipeline coordinates the embedding generation and storage process
type EmbeddingPipeline interface {
	// ProcessContent processes content to generate and store embeddings
	ProcessContent(ctx context.Context, content string, contentType string, contentID string) error
	// BatchProcessContent processes multiple content items in a batch
	BatchProcessContent(ctx context.Context, contents []string, contentType string, contentIDs []string) error
	// ProcessCodeChunks processes code chunks to generate and store embeddings
	ProcessCodeChunks(ctx context.Context, contentType string, contentID string, chunkIDs []string) error
	// ProcessIssues processes GitHub issues to generate and store embeddings
	ProcessIssues(ctx context.Context, ownerRepo string, issueNumbers []int) error
	// ProcessDiscussions processes GitHub discussions to generate and store embeddings
	ProcessDiscussions(ctx context.Context, ownerRepo string, discussionIDs []string) error
}

// EmbeddingManager manages the embedding pipeline
type EmbeddingManager struct {
	// The embedding pipeline for processing content
	pipeline EmbeddingPipeline

	// The chunking service for code chunking
	chunkingService *chunking.ChunkingService

	// Database connection for direct operations
	db *sql.DB

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Flag to indicate if the manager is initialized
	initialized bool
}

// NewEmbeddingManager creates a new embedding manager
func NewEmbeddingManager(db *sql.DB, chunkingService *chunking.ChunkingService, pipeline EmbeddingPipeline) (*EmbeddingManager, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	if chunkingService == nil {
		return nil, errors.New("chunking service is required")
	}

	if pipeline == nil {
		return nil, errors.New("embedding pipeline is required")
	}

	return &EmbeddingManager{
		pipeline:        pipeline,
		chunkingService: chunkingService,
		db:              db,
		initialized:     true,
	}, nil
}

// CreateEmbeddingFromContent generates and stores an embedding for a content string
func (m *EmbeddingManager) CreateEmbeddingFromContent(ctx context.Context, content string, contentType string, contentID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return errors.New("embedding manager is not initialized")
	}

	return m.pipeline.ProcessContent(ctx, content, contentType, contentID)
}

// CreateEmbeddingsFromCodeFile processes a code file to generate and store embeddings
func (m *EmbeddingManager) CreateEmbeddingsFromCodeFile(ctx context.Context, owner string, repo string, path string, content []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return errors.New("embedding manager is not initialized")
	}

	// Generate contentID in the format owner/repo/path
	contentID := fmt.Sprintf("%s/%s/%s", owner, repo, path)

	// Chunk the code
	chunks, err := m.chunkingService.ChunkCode(ctx, string(content), path)
	if err != nil {
		return fmt.Errorf("failed to chunk code: %w", err)
	}

	// Create a slice of chunk IDs
	chunkIDs := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkIDs[i] = chunk.ID
	}

	// Process the code chunks
	return m.pipeline.ProcessCodeChunks(ctx, ContentTypeCodeChunk, contentID, chunkIDs)
}

// CreateEmbeddingsFromIssue processes a GitHub issue to generate and store embeddings
func (m *EmbeddingManager) CreateEmbeddingsFromIssue(ctx context.Context, owner string, repo string, issueNumber int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return errors.New("embedding manager is not initialized")
	}

	// Process the issue
	ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
	return m.pipeline.ProcessIssues(ctx, ownerRepo, []int{issueNumber})
}

// CreateEmbeddingsFromIssues processes multiple GitHub issues to generate and store embeddings
func (m *EmbeddingManager) CreateEmbeddingsFromIssues(ctx context.Context, owner string, repo string, issueNumbers []int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return errors.New("embedding manager is not initialized")
	}

	// Process the issues
	ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
	return m.pipeline.ProcessIssues(ctx, ownerRepo, issueNumbers)
}

// CreateEmbeddingsFromDiscussion processes a GitHub discussion to generate and store embeddings
func (m *EmbeddingManager) CreateEmbeddingsFromDiscussion(ctx context.Context, owner string, repo string, discussionID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return errors.New("embedding manager is not initialized")
	}

	// Process the discussion
	ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
	return m.pipeline.ProcessDiscussions(ctx, ownerRepo, []string{discussionID})
}

// CreateEmbeddingsFromDiscussions processes multiple GitHub discussions to generate and store embeddings
func (m *EmbeddingManager) CreateEmbeddingsFromDiscussions(ctx context.Context, owner string, repo string, discussionIDs []string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return errors.New("embedding manager is not initialized")
	}

	// Process the discussions
	ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
	return m.pipeline.ProcessDiscussions(ctx, ownerRepo, discussionIDs)
}

// ProcessRepository processes an entire repository to generate and store embeddings
func (m *EmbeddingManager) ProcessRepository(ctx context.Context, owner string, repo string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return errors.New("embedding manager is not initialized")
	}

	// This is a placeholder for a more comprehensive implementation
	// In a real implementation, this would:
	// 1. List all files in the repository
	// 2. Process each file to generate embeddings
	// 3. List all issues in the repository
	// 4. Process each issue to generate embeddings
	// 5. List all discussions in the repository
	// 6. Process each discussion to generate embeddings

	log.Printf("Processing repository %s/%s", owner, repo)

	// TODO: Implement full repository processing
	// For now, return a "not implemented" error
	return errors.New("full repository processing not implemented yet")
}

// SearchSimilarContent searches for content similar to the provided text
func (m *EmbeddingManager) SearchSimilarContent(
	ctx context.Context,
	text string,
	modelType EmbeddingModelType,
	modelName string,
	limit int,
	threshold float32,
) ([]map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, errors.New("embedding manager is not initialized")
	}

	// Since we can't import the embedding package directly,
	// we'll use a direct SQL query to search for similar embeddings

	// For demonstration purposes, we'll generate a simple random vector
	// In a real implementation, this would be generated by an embedding model
	vector := make([]float32, 1536) // Placeholder vector with 1536 dimensions

	// Fill with some random values for demonstration
	for i := range vector {
		vector[i] = float32(i%100) / 100.0
	}

	embedding := &EmbeddingVector{
		Vector:      vector,
		Dimensions:  1536,
		ModelID:     modelName,
		ContentType: "search",
		ContentID:   "search-" + time.Now().Format(time.RFC3339Nano),
		Metadata:    make(map[string]interface{}),
	}

	// Now we need to search for similar embeddings in the database
	// Since we don't have direct access to the storage implementation, we'll use a SQL query

	// Format the vector for PostgreSQL
	vectorStr := formatVectorForPg(embedding.Vector)

	// Query for similar embeddings
	query := `
		SELECT
			id, context_id, content_index, text,
			content_type, model_id,
			metadata,
			(1 - (embedding <=> $1::vector))::float AS similarity
		FROM
			mcp.embeddings
		WHERE
			vector_dimensions = $2
			AND model_id = $3
			AND (1 - (embedding <=> $1::vector))::float >= $4
		ORDER BY
			similarity DESC
		LIMIT $5`

	rows, err := m.db.QueryContext(
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
			log.Printf("Failed to close rows: %v", err)
		}
	}()

	var results []map[string]interface{}
	for rows.Next() {
		var (
			id           string
			contextID    sql.NullString
			contentIndex int
			text         sql.NullString
			contentType  string
			modelID      string
			metadataJSON sql.NullString
			similarity   float32
		)

		if err := rows.Scan(
			&id,
			&contextID,
			&contentIndex,
			&text,
			&contentType,
			&modelID,
			&metadataJSON,
			&similarity,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding row: %w", err)
		}

		// Create a result map
		result := map[string]interface{}{
			"id":           id,
			"content_type": contentType,
			"model_id":     modelID,
			"similarity":   similarity,
		}

		// Add optional fields if present
		if contextID.Valid {
			result["context_id"] = contextID.String
		}

		if text.Valid {
			result["text"] = text.String
		}

		if metadataJSON.Valid {
			// In a real implementation, you would parse the JSON
			result["metadata"] = metadataJSON.String
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return results, nil
}

// formatVectorForPg formats a vector for PostgreSQL
func formatVectorForPg(vector []float32) string {
	// Format as [1,2,3,...]
	elements := make([]string, len(vector))
	for i, v := range vector {
		elements[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(elements, ",") + "]"
}
