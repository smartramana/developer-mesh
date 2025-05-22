package embedding

import (
	"context"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/pkg/repository/vector"
	"github.com/S-Corkum/devops-mcp/pkg/tests/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddingChunkingIntegration(t *testing.T) {
	helper := integration.NewTestHelper(t)
	
	// Create observability components
	logger := observability.NewLogger()
	require.NotNil(t, logger)
	
	// Create mock vector repository for testing
	vectorRepo := vector.NewMockVectorRepository()
	require.NotNil(t, vectorRepo)
	
	t.Run("Content chunking and embedding pipeline integration", func(t *testing.T) {
		// Create chunker
		chunker := chunking.NewChunker(&chunking.ChunkerConfig{
			ChunkSize:    1000,
			ChunkOverlap: 200,
		})
		require.NotNil(t, chunker)
		
		// Create embedding service (using mock for tests)
		embeddingService, err := embedding.NewMockEmbeddingService(logger)
		require.NoError(t, err)
		require.NotNil(t, embeddingService)
		
		// Create test content
		testContent := "This is a test document for integration testing of the embedding and chunking pipeline. " +
			"It needs to be long enough to generate multiple chunks with the configured chunk size. " +
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
			"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. " +
			"Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. " +
			"Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. " +
			"This should be sufficient to generate multiple chunks with meaningful content for testing."
		
		// Process content through full pipeline
		ctx := context.Background()
		
		// 1. Chunk the content
		chunks, err := chunker.ChunkText(testContent)
		require.NoError(t, err)
		require.NotEmpty(t, chunks)
		
		// Verify we have chunks
		assert.True(t, len(chunks) > 0, "Should generate at least one chunk")
		
		// 2. Generate embeddings for each chunk
		var vectors []*models.Vector
		for i, chunk := range chunks {
			// Generate embedding for chunk
			embedding, err := embeddingService.GenerateEmbedding(ctx, chunk.Text)
			require.NoError(t, err)
			require.NotNil(t, embedding)
			require.True(t, len(embedding) > 0, "Embedding should not be empty")
			
			// Create vector object
			vector := &models.Vector{
				ID:        "test-vector-" + string(rune(i+'0')),
				Text:      chunk.Text,
				Embedding: embedding,
				Metadata: map[string]interface{}{
					"source":  "integration-test",
					"chunk":   i,
					"content": "test document",
				},
			}
			
			vectors = append(vectors, vector)
		}
		
		// 3. Store vectors in repository
		for _, v := range vectors {
			err = vectorRepo.StoreVector(ctx, v)
			require.NoError(t, err)
		}
		
		// 4. Perform a similarity search to verify end-to-end functionality
		queryEmbedding, err := embeddingService.GenerateEmbedding(ctx, "test document")
		require.NoError(t, err)
		
		// Execute search
		searchParams := &vector.SearchParams{
			Embedding:  queryEmbedding,
			Limit:      5,
			MinScore:   0.7,
			Collection: "default",
		}
		
		// The mock repository should return our test vectors
		results, err := vectorRepo.SimilaritySearch(ctx, searchParams)
		require.NoError(t, err)
		require.NotEmpty(t, results)
		
		// Verify at least one result was returned
		assert.True(t, len(results) > 0, "Should return at least one search result")
		
		// Check that the first result has text and score
		assert.NotEmpty(t, results[0].Text, "Search result should have text")
		assert.True(t, results[0].Score > 0, "Search result should have positive score")
	})
}
