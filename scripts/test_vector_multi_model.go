// test_vector_multi_model tests the multi-model vector embedding functionality
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common"
	"github.com/S-Corkum/devops-mcp/pkg/repository/vector"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// modelInfo represents model configuration for testing
type modelInfo struct {
	ID        string
	Dimension int
}

// testModels defines the models to test
var testModels = []modelInfo{
	{ID: "openai.text-embedding-ada-002", Dimension: 1536},
	{ID: "anthropic.claude-2-1", Dimension: 768},
	{ID: "mcp.small-model", Dimension: 384},
}

// config holds the test configuration
type config struct {
	dsn          string
	cleanupAfter bool
	verbose      bool
}

func main() {
	// Parse command line flags
	cfg := parseFlags()

	// Connect to the database
	db, err := connectDB(cfg.dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create vector repository
	repo := vector.NewRepository(db)

	// Run tests
	ctx := context.Background()
	contextID := fmt.Sprintf("test-context-%s", uuid.New().String()[:8])
	
	if cfg.verbose {
		fmt.Printf("Using test context ID: %s\n", contextID)
	}

	// Test each model
	for _, model := range testModels {
		if err := testModel(ctx, repo, contextID, model, cfg.verbose); err != nil {
			log.Fatalf("Failed to test model %s: %v", model.ID, err)
		}
	}

	// Test supported models
	if err := testSupportedModels(ctx, repo, cfg.verbose); err != nil {
		log.Fatalf("Failed to test supported models: %v", err)
	}

	// Clean up test data if requested
	if cfg.cleanupAfter {
		if err := cleanupTestData(ctx, repo, contextID, cfg.verbose); err != nil {
			log.Fatalf("Failed to clean up test data: %v", err)
		}
	}

	fmt.Println("\nAll tests completed successfully!")
}

// parseFlags parses command line flags
func parseFlags() config {
	var cfg config
	
	flag.StringVar(&cfg.dsn, "dsn", "", "Database connection string (defaults to MCP_DATABASE_DSN env var)")
	flag.BoolVar(&cfg.cleanupAfter, "cleanup", true, "Clean up test data after running")
	flag.BoolVar(&cfg.verbose, "verbose", true, "Enable verbose output")
	flag.Parse()

	// Use environment variable if DSN not provided
	if cfg.dsn == "" {
		cfg.dsn = os.Getenv("MCP_DATABASE_DSN")
		if cfg.dsn == "" {
			cfg.dsn = "postgres://postgres:postgres@localhost:5432/mcp?sslmode=disable"
		}
	}

	return cfg
}

// connectDB establishes a database connection
func connectDB(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(15 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// testModel tests storing and retrieving embeddings for a specific model
func testModel(ctx context.Context, repo vector.Repository, contextID string, model modelInfo, verbose bool) error {
	if verbose {
		fmt.Printf("\n=== Testing model: %s (Dimension: %d) ===\n", model.ID, model.Dimension)
	}

	// Generate and store test embeddings
	embeddings := generateTestEmbeddings(contextID, model.ID, model.Dimension, 5)
	
	for i, emb := range embeddings {
		if err := repo.StoreEmbedding(ctx, emb); err != nil {
			return fmt.Errorf("failed to store embedding %d: %w", i, err)
		}
		if verbose {
			fmt.Printf("Stored embedding %d with ID: %s\n", i, emb.ID)
		}
	}

	// Retrieve embeddings for the model
	retrievedEmbeddings, err := repo.GetEmbeddingsByModel(ctx, contextID, model.ID)
	if err != nil {
		return fmt.Errorf("failed to retrieve embeddings: %w", err)
	}
	
	if verbose {
		fmt.Printf("Retrieved %d embeddings for model %s\n", len(retrievedEmbeddings), model.ID)
	}

	// Test vector search if embeddings were retrieved
	if len(retrievedEmbeddings) > 0 {
		if err := testVectorSearch(ctx, repo, retrievedEmbeddings[0], contextID, model.ID, verbose); err != nil {
			return fmt.Errorf("vector search test failed: %w", err)
		}
	}

	return nil
}

// testVectorSearch tests the vector search functionality
func testVectorSearch(ctx context.Context, repo vector.Repository, queryEmb *vector.Embedding, contextID, modelID string, verbose bool) error {
	// Normalize the query vector
	normalizedQuery := common.NormalizeVectorL2(queryEmb.Embedding)

	// Search for similar embeddings
	searchResults, err := repo.SearchEmbeddings(
		ctx,
		normalizedQuery,
		contextID,
		modelID,
		5,
		0.5,
	)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d similar embeddings:\n", len(searchResults))
		for i, result := range searchResults {
			// Calculate similarity score (1 - distance for L2 norm)
			similarity := calculateSimilarity(queryEmb.Embedding, result.Embedding)
			fmt.Printf("  %d. ID: %s, Similarity: %.4f\n", i+1, result.ID, similarity)
		}
	}

	return nil
}

// testSupportedModels tests retrieving the list of supported models
func testSupportedModels(ctx context.Context, repo vector.Repository, verbose bool) error {
	models, err := repo.GetSupportedModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to get supported models: %w", err)
	}

	if verbose {
		fmt.Printf("\n=== Supported Models ===\n")
		for i, model := range models {
			fmt.Printf("%d. %s\n", i+1, model)
		}
	}

	return nil
}

// cleanupTestData removes all test data
func cleanupTestData(ctx context.Context, repo vector.Repository, contextID string, verbose bool) error {
	if verbose {
		fmt.Printf("\nCleaning up test data for context: %s\n", contextID)
	}

	if err := repo.DeleteContextEmbeddings(ctx, contextID); err != nil {
		return fmt.Errorf("failed to delete test embeddings: %w", err)
	}

	if verbose {
		fmt.Println("Test data cleaned up successfully.")
	}

	return nil
}

// generateTestEmbeddings creates random embeddings for testing
func generateTestEmbeddings(contextID, modelID string, dimension, count int) []*vector.Embedding {
	embeddings := make([]*vector.Embedding, count)
	
	// Seed random number generator for reproducibility
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	for i := 0; i < count; i++ {
		// Generate random vector
		vec := make([]float32, dimension)
		for j := 0; j < dimension; j++ {
			vec[j] = rng.Float32()*2 - 1 // Range [-1, 1]
		}
		
		// Normalize vector
		vec = common.NormalizeVectorL2(vec)
		
		// Create embedding with proper metadata
		embeddings[i] = &vector.Embedding{
			ID:           uuid.New().String(),
			ContextID:    contextID,
			ContentIndex: i,
			Text:         fmt.Sprintf("Test embedding %d for model %s", i, modelID),
			Embedding:    vec,
			ModelID:      modelID,
			CreatedAt:    time.Now(),
			Metadata: map[string]interface{}{
				"test":       true,
				"index":      i,
				"dimensions": dimension,
			},
		}
	}
	
	return embeddings
}

// calculateSimilarity calculates cosine similarity between two vectors
func calculateSimilarity(v1, v2 []float32) float64 {
	if len(v1) != len(v2) {
		return 0
	}

	var dotProduct, norm1, norm2 float64
	for i := range v1 {
		dotProduct += float64(v1[i]) * float64(v2[i])
		norm1 += float64(v1[i]) * float64(v1[i])
		norm2 += float64(v2[i]) * float64(v2[i])
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// printJSON prints an object as formatted JSON (utility function)
func printJSON(obj interface{}) error {
	jsonBytes, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(jsonBytes))
	return nil
}