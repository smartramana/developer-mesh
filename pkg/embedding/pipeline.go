package embedding

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

const (
	// Default batch size for processing chunks
	defaultBatchSize = 10

	// Content types
	ContentTypeCodeChunk  = "code_chunk"
	ContentTypeIssue      = "issue"
	ContentTypeComment    = "comment"
	ContentTypeDiscussion = "discussion"

	// Metadata keys
	MetadataKeyRepositoryOwner = "repository_owner"
	MetadataKeyRepositoryName  = "repository_name"
	MetadataKeyLanguage        = "language"
	MetadataKeyChunkType       = "chunk_type"
	MetadataKeySourceFile      = "source_file"
	MetadataKeyCreatedAt       = "created_at"
	MetadataKeyContentType     = "content_type"
)

// EmbeddingPipelineConfig holds configuration for the embedding pipeline
type EmbeddingPipelineConfig struct {
	// Number of goroutines to use for parallel processing
	Concurrency int

	// Batch size for processing
	BatchSize int

	// Whether to include code comments in embeddings
	IncludeComments bool

	// Whether to enrich embeddings with metadata
	EnrichMetadata bool
}

// DefaultEmbeddingPipelineConfig returns the default embedding pipeline configuration
func DefaultEmbeddingPipelineConfig() *EmbeddingPipelineConfig {
	return &EmbeddingPipelineConfig{
		Concurrency:     4,
		BatchSize:       defaultBatchSize,
		IncludeComments: true,
		EnrichMetadata:  true,
	}
}

// GitHubContentProvider defines the interface for accessing GitHub content
type GitHubContentProvider interface {
	// GetContent retrieves file content from GitHub
	GetContent(ctx context.Context, owner, repo, path string) ([]byte, error)

	// GetIssue retrieves issue details from GitHub
	GetIssue(ctx context.Context, owner, repo string, issueNumber int) (*GitHubIssueData, error)

	// GetIssueComments retrieves issue comments from GitHub
	GetIssueComments(ctx context.Context, owner, repo string, issueNumber int) ([]*GitHubCommentData, error)
}

// GitHubIssue represents a GitHub issue
type GitHubIssue struct {
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GitHubComment represents a GitHub comment
type GitHubComment struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ChunkingInterface defines the interface for chunking services
type ChunkingInterface interface {
	// Here we define the minimum methods needed from the chunking service
	// These methods should match what we actually use in the pipeline
	ChunkCode(ctx context.Context, content string, path string) ([]*chunking.CodeChunk, error)
}

// DefaultEmbeddingPipeline implements EmbeddingPipeline for processing different content types
type DefaultEmbeddingPipeline struct {
	// Embedding service for generating embeddings
	embeddingService EmbeddingService

	// Storage for persisting embeddings
	storage EmbeddingStorage

	// Chunking service for code chunking - uses the interface instead of concrete type
	chunkingService ChunkingInterface

	// GitHub content provider for accessing GitHub content
	contentProvider GitHubContentProvider

	// Configuration
	config *EmbeddingPipelineConfig
}

// NewEmbeddingPipeline creates a new embedding pipeline
func NewEmbeddingPipeline(
	embeddingService EmbeddingService,
	storage EmbeddingStorage,
	chunkingService *chunking.ChunkingService,
	contentProvider GitHubContentProvider,
	config *EmbeddingPipelineConfig,
) (*DefaultEmbeddingPipeline, error) {
	if embeddingService == nil {
		return nil, errors.New("embedding service is required")
	}

	if storage == nil {
		return nil, errors.New("embedding storage is required")
	}

	if chunkingService == nil {
		return nil, errors.New("chunking service is required")
	}

	if config == nil {
		config = DefaultEmbeddingPipelineConfig()
	}

	return &DefaultEmbeddingPipeline{
		embeddingService: embeddingService,
		storage:          storage,
		chunkingService:  chunkingService,
		contentProvider:  contentProvider,
		config:           config,
	}, nil
}

// ProcessContent processes a single content item to generate and store embeddings
func (p *DefaultEmbeddingPipeline) ProcessContent(ctx context.Context, content string, contentType string, contentID string) error {
	// Generate embedding
	embedding, err := p.embeddingService.GenerateEmbedding(ctx, content, contentType, contentID)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Store embedding
	if err := p.storage.StoreEmbedding(ctx, embedding); err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	return nil
}

// BatchProcessContent processes multiple content items in a batch
func (p *DefaultEmbeddingPipeline) BatchProcessContent(ctx context.Context, contents []string, contentType string, contentIDs []string) error {
	if len(contents) != len(contentIDs) {
		return errors.New("number of contents must match number of content IDs")
	}

	if len(contents) == 0 {
		return nil // Nothing to process
	}

	// Process in batches
	for i := 0; i < len(contents); i += p.config.BatchSize {
		end := i + p.config.BatchSize
		if end > len(contents) {
			end = len(contents)
		}

		batchContents := contents[i:end]
		batchIDs := contentIDs[i:end]

		// Generate embeddings for the batch
		embeddings, err := p.embeddingService.BatchGenerateEmbeddings(ctx, batchContents, contentType, batchIDs)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings for batch %d-%d: %w", i, end, err)
		}

		// Store embeddings
		if err := p.storage.BatchStoreEmbeddings(ctx, embeddings); err != nil {
			return fmt.Errorf("failed to store embeddings for batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// ProcessCodeChunks processes code chunks to generate and store embeddings
func (p *DefaultEmbeddingPipeline) ProcessCodeChunks(ctx context.Context, contentType string, contentID string, chunkIDs []string) error {
	if p.contentProvider == nil {
		return errors.New("GitHub content provider is required for processing code chunks")
	}

	// Extract repository information from content ID
	parts := strings.Split(contentID, "/")
	if len(parts) < 3 {
		return fmt.Errorf("invalid content ID format: %s", contentID)
	}

	owner := parts[0]
	repo := parts[1]
	path := strings.Join(parts[2:], "/")

	// Get file content from GitHub
	fileContent, err := p.contentProvider.GetContent(ctx, owner, repo, path)
	if err != nil {
		return fmt.Errorf("failed to get file content: %w", err)
	}

	// Chunk the code
	chunks, err := p.chunkingService.ChunkCode(ctx, string(fileContent), path)
	if err != nil {
		return fmt.Errorf("failed to chunk code: %w", err)
	}

	// Process each chunk in parallel
	var wg sync.WaitGroup
	errors := make(chan error, len(chunks))
	semaphore := make(chan struct{}, p.config.Concurrency)

	for _, chunk := range chunks {
		// Skip comments if configured to do so
		if !p.config.IncludeComments && chunk.Type == chunking.ChunkTypeComment {
			continue
		}

		wg.Add(1)
		go func(chunk *chunking.CodeChunk) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create chunk content ID
			chunkContentID := fmt.Sprintf("%s:%s", contentID, chunk.ID)

			// Enrich metadata if configured
			metadata := make(map[string]interface{})
			if p.config.EnrichMetadata {
				metadata[MetadataKeyRepositoryOwner] = owner
				metadata[MetadataKeyRepositoryName] = repo
				metadata[MetadataKeyLanguage] = string(chunk.Language)
				metadata[MetadataKeyChunkType] = string(chunk.Type)
				metadata[MetadataKeySourceFile] = path
				metadata[MetadataKeyCreatedAt] = time.Now().UTC().Format(time.RFC3339)
				metadata[MetadataKeyContentType] = ContentTypeCodeChunk

				// Add custom metadata from the chunk
				for k, v := range chunk.Metadata {
					metadata[k] = v
				}
			}

			// Generate embedding
			embedding, err := p.embeddingService.GenerateEmbedding(ctx, chunk.Content, ContentTypeCodeChunk, chunkContentID)
			if err != nil {
				errors <- fmt.Errorf("failed to generate embedding for chunk %s: %w", chunk.ID, err)
				return
			}

			// Add metadata
			embedding.Metadata = metadata

			// Store embedding
			if err := p.storage.StoreEmbedding(ctx, embedding); err != nil {
				errors <- fmt.Errorf("failed to store embedding for chunk %s: %w", chunk.ID, err)
				return
			}

		}(chunk)
	}

	// Wait for all chunks to be processed
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

// ProcessIssues processes GitHub issues to generate and store embeddings
func (p *DefaultEmbeddingPipeline) ProcessIssues(ctx context.Context, ownerRepo string, issueNumbers []int) error {
	if p.contentProvider == nil {
		return errors.New("GitHub content provider is required for processing issues")
	}

	// Extract owner and repo
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid owner/repo format: %s", ownerRepo)
	}

	owner := parts[0]
	repo := parts[1]

	// Process each issue in batches
	for i := 0; i < len(issueNumbers); i += p.config.BatchSize {
		end := i + p.config.BatchSize
		if end > len(issueNumbers) {
			end = len(issueNumbers)
		}

		batchIssues := issueNumbers[i:end]

		// Process each issue in the batch
		var wg sync.WaitGroup
		errors := make(chan error, len(batchIssues))
		semaphore := make(chan struct{}, p.config.Concurrency)

		for _, issueNumber := range batchIssues {
			wg.Add(1)
			go func(issueNum int) {
				defer wg.Done()

				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				// Get issue details
				issue, err := p.contentProvider.GetIssue(ctx, owner, repo, issueNum)
				if err != nil {
					errors <- fmt.Errorf("failed to get issue #%d: %w", issueNum, err)
					return
				}

				// Process issue body
				issueID := fmt.Sprintf("%s/%s/issues/%d", owner, repo, issueNum)

				// Create content text combining title and body
				contentText := fmt.Sprintf("Title: %s\n\nBody: %s", issue.Title, issue.Body)

				// Generate embedding for issue
				embedding, err := p.embeddingService.GenerateEmbedding(ctx, contentText, ContentTypeIssue, issueID)
				if err != nil {
					errors <- fmt.Errorf("failed to generate embedding for issue #%d: %w", issueNum, err)
					return
				}

				// Enrich metadata if configured
				if p.config.EnrichMetadata {
					embedding.Metadata = map[string]interface{}{
						MetadataKeyRepositoryOwner: owner,
						MetadataKeyRepositoryName:  repo,
						MetadataKeyContentType:     ContentTypeIssue,
						MetadataKeyCreatedAt:       time.Now().UTC().Format(time.RFC3339),
						"issue_number":             issueNum,
						"issue_title":              issue.Title,
						"issue_state":              issue.State,
						"issue_created_at":         issue.CreatedAt,
						"issue_updated_at":         issue.UpdatedAt,
					}
				}

				// Store embedding
				if err := p.storage.StoreEmbedding(ctx, embedding); err != nil {
					errors <- fmt.Errorf("failed to store embedding for issue #%d: %w", issueNum, err)
					return
				}

				// Get comments for the issue
				comments, err := p.contentProvider.GetIssueComments(ctx, owner, repo, issueNum)
				if err != nil {
					log.Printf("Warning: failed to get comments for issue #%d: %v", issueNum, err)
					// Continue processing other aspects
				} else {
					// Process each comment
					for _, comment := range comments {
						commentID := fmt.Sprintf("%s/comment/%d", issueID, comment.ID)

						// Generate embedding for comment
						commentEmbedding, err := p.embeddingService.GenerateEmbedding(ctx, comment.Body, ContentTypeComment, commentID)
						if err != nil {
							errors <- fmt.Errorf("failed to generate embedding for comment #%d: %w", comment.ID, err)
							continue
						}

						// Enrich metadata if configured
						if p.config.EnrichMetadata {
							commentEmbedding.Metadata = map[string]interface{}{
								MetadataKeyRepositoryOwner: owner,
								MetadataKeyRepositoryName:  repo,
								MetadataKeyContentType:     ContentTypeComment,
								MetadataKeyCreatedAt:       time.Now().UTC().Format(time.RFC3339),
								"issue_number":             issueNum,
								"comment_id":               comment.ID,
								"comment_created_at":       comment.CreatedAt,
								"comment_updated_at":       comment.UpdatedAt,
								"user_login":               comment.User.Login,
							}
						}

						// Store embedding
						if err := p.storage.StoreEmbedding(ctx, commentEmbedding); err != nil {
							errors <- fmt.Errorf("failed to store embedding for comment #%d: %w", comment.ID, err)
							continue
						}
					}
				}

			}(issueNumber)
		}

		// Wait for all issues in the batch to be processed
		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ProcessDiscussions processes GitHub discussions to generate and store embeddings
func (p *DefaultEmbeddingPipeline) ProcessDiscussions(ctx context.Context, ownerRepo string, discussionIDs []string) error {
	if p.contentProvider == nil {
		return errors.New("GitHub content provider is required for processing discussions")
	}

	// Extract owner and repo
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid owner/repo format: %s", ownerRepo)
	}

	owner := parts[0]
	repo := parts[1]

	// Process each discussion in batches
	for i := 0; i < len(discussionIDs); i += p.config.BatchSize {
		end := i + p.config.BatchSize
		if end > len(discussionIDs) {
			end = len(discussionIDs)
		}

		batchDiscussions := discussionIDs[i:end]

		// Process each discussion in the batch
		var wg sync.WaitGroup
		errors := make(chan error, len(batchDiscussions))
		semaphore := make(chan struct{}, p.config.Concurrency)

		for _, discussionID := range batchDiscussions {
			wg.Add(1)
			go func(discID string) {
				defer wg.Done()

				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				// Get discussion details - this is a placeholder as we don't have the actual implementation
				// In a real implementation, you would call the appropriate method on contentManager
				// For now, we'll just log a warning
				log.Printf("Warning: GitHub Discussions API support is not implemented")

				// Create a dummy discussion content for demonstration
				discussionContent := fmt.Sprintf("Discussion ID: %s\nThis is a placeholder for discussion content.", discID)
				fullDiscussionID := fmt.Sprintf("%s/%s/discussions/%s", owner, repo, discID)

				// Generate embedding for discussion
				embedding, err := p.embeddingService.GenerateEmbedding(ctx, discussionContent, ContentTypeDiscussion, fullDiscussionID)
				if err != nil {
					errors <- fmt.Errorf("failed to generate embedding for discussion %s: %w", discID, err)
					return
				}

				// Enrich metadata if configured
				if p.config.EnrichMetadata {
					embedding.Metadata = map[string]interface{}{
						MetadataKeyRepositoryOwner: owner,
						MetadataKeyRepositoryName:  repo,
						MetadataKeyContentType:     ContentTypeDiscussion,
						MetadataKeyCreatedAt:       time.Now().UTC().Format(time.RFC3339),
						"discussion_id":            discID,
					}
				}

				// Store embedding
				if err := p.storage.StoreEmbedding(ctx, embedding); err != nil {
					errors <- fmt.Errorf("failed to store embedding for discussion %s: %w", discID, err)
					return
				}

			}(discussionID)
		}

		// Wait for all discussions in the batch to be processed
		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			if err != nil {
				return err
			}
		}
	}

	return nil
}
