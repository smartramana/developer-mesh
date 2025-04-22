package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/S-Corkum/mcp-server/internal/common"
	"github.com/S-Corkum/mcp-server/internal/repository"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Models to test
var models = []struct {
	ID        string
	Dimension int
}{
	{ID: "openai.text-embedding-ada-002", Dimension: 1536},
	{ID: "anthropic.claude-2-1", Dimension: 768},
	{ID: "mcp.small-model", Dimension: 384},
}

func main() {
	// Database connection string (can be set via environment variable)
	dsn := os.Getenv("MCP_DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/mcp?sslmode=disable"
	}

	// Connect to the database
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Set connection pool parameters
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(15 * time.Minute)

	// Create embedding repository
	repo := repository.NewEmbeddingRepository(db)

	// Create a test context ID
	contextID := fmt.Sprintf("test-context-%d", time.Now().Unix())
	fmt.Printf("Using test context ID: %s\n", contextID)

	// Test storing and retrieving embeddings for each model
	for _, model := range models {
		fmt.Printf("\n=== Testing model: %s (Dimension: %d) ===\n", model.ID, model.Dimension)

		// Generate and store test embeddings
		embeddings := generateTestEmbeddings(contextID, model.ID, model.Dimension, 5)
		for i, emb := range embeddings {
			err := repo.StoreEmbedding(context.Background(), emb)
			if err != nil {
				log.Fatalf("Failed to store embedding %d for model %s: %v", i, model.ID, err)
			}
			fmt.Printf("Stored embedding %d with ID: %s\n", i, emb.ID)
		}

		// Retrieve all embeddings for the model
		retrievedEmbeddings, err := repo.GetEmbeddingsByModel(context.Background(), contextID, model.ID)
		if err != nil {
			log.Fatalf("Failed to retrieve embeddings for model %s: %v", model.ID, err)
		}
		fmt.Printf("Retrieved %d embeddings for model %s\n", len(retrievedEmbeddings), model.ID)

		// Test vector search
		if len(retrievedEmbeddings) > 0 {
			// Use the first embedding as the query
			queryEmbedding := retrievedEmbeddings[0].Embedding

			// Normalize the query vector for better search results
			normalizedQuery := common.NormalizeVectorL2(queryEmbedding)

			// Search for similar embeddings
			searchResults, err := repo.SearchEmbeddings(
				context.Background(),
				normalizedQuery,
				contextID,
				model.ID,
				5,
				0.5,
			)
			if err != nil {
				log.Fatalf("Failed to search embeddings for model %s: %v", model.ID, err)
			}

			fmt.Printf("Found %d similar embeddings:\n", len(searchResults))
			for i, result := range searchResults {
				fmt.Printf("  %d. ID: %s, Similarity: %.4f\n", i+1, result.ID, result.Similarity)
			}
		}
	}

	// Check which models are available
	models, err := repo.GetSupportedModels(context.Background())
	if err != nil {
		log.Fatalf("Failed to get supported models: %v", err)
	}

	fmt.Printf("\n=== Supported Models ===\n")
	for i, model := range models {
		fmt.Printf("%d. %s\n", i+1, model)
	}

	// Clean up test data
	fmt.Printf("\nCleaning up test data...\n")
	err = repo.DeleteContextEmbeddings(context.Background(), contextID)
	if err != nil {
		log.Fatalf("Failed to delete test embeddings: %v", err)
	}
	fmt.Printf("Test data cleaned up successfully.\n")
}

// generateTestEmbeddings generates random embeddings for testing
func generateTestEmbeddings(contextID, modelID string, dimension, count int) []*repository.Embedding {
	embeddings := make([]*repository.Embedding, count)
	
	for i := 0; i < count; i++ {
		// Generate random vector
		vector := make([]float32, dimension)
		for j := 0; j < dimension; j++ {
			vector[j] = rand.Float32()
		}
		
		// Normalize vector
		vector = common.NormalizeVectorL2(vector)
		
		// Create embedding
		embeddings[i] = &repository.Embedding{
			ContextID:        contextID,
			ContentIndex:     i,
			Text:             fmt.Sprintf("Test embedding %d for model %s", i, modelID),
			Embedding:        vector,
			VectorDimensions: dimension,
			ModelID:          modelID,
		}
	}
	
	return embeddings
}

// printJSON prints an object as JSON
func printJSON(obj interface{}) {
	jsonBytes, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
}
