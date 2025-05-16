package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/S-Corkum/devops-mcp/internal/chunking"
	"github.com/S-Corkum/devops-mcp/internal/chunking/parsers"
	"github.com/S-Corkum/devops-mcp/internal/config"
	commonConfig "github.com/S-Corkum/devops-mcp/internal/common/config"
	"github.com/S-Corkum/devops-mcp/internal/core"
	"github.com/S-Corkum/devops-mcp/internal/embedding"
	"github.com/S-Corkum/devops-mcp/internal/observability"
	"github.com/S-Corkum/devops-mcp/internal/storage"
)

const (
	defaultConfigPath = "config.yaml"
	defaultBatchSize  = 10
	defaultConcurrency = 4
)

var (
	configPath    = flag.String("config", defaultConfigPath, "Path to config file")
	repoOwner     = flag.String("owner", "", "Repository owner")
	repoName      = flag.String("repo", "", "Repository name")
	filePath      = flag.String("file", "", "File path (for embedding code)")
	issueNumbers  = flag.String("issues", "", "Comma-separated issue numbers (for embedding issues)")
	modelName     = flag.String("model", "text-embedding-3-small", "Embedding model name")
	apiKey        = flag.String("api-key", "", "API key for the embedding model")
	batchSize     = flag.Int("batch-size", defaultBatchSize, "Batch size for processing")
	concurrency   = flag.Int("concurrency", defaultConcurrency, "Number of concurrent workers")
	includeComments = flag.Bool("include-comments", true, "Whether to include comments in code embeddings")
	command       = flag.String("command", "embed", "Command to execute (embed, search)")
	query         = flag.String("query", "", "Query for searching similar content")
	limit         = flag.Int("limit", 10, "Limit for search results")
	threshold     = flag.Float64("threshold", 0.7, "Similarity threshold for search results")
)

// mockGitHubContentManager is a simplified implementation of GitHubContentManager for CLI usage
type mockGitHubContentManager struct {
	metricsClient observability.MetricsClient
}

// GetContent retrieves file content (mock implementation for CLI)
func (m *mockGitHubContentManager) GetContent(
	ctx context.Context, 
	owner string, 
	repo string, 
	contentType storage.ContentType, 
	contentID string,
) ([]byte, *storage.ContentMetadata, error) {
	startTime := time.Now()
	m.metricsClient.RecordOperation("github_content_manager", "get_content", true, time.Since(startTime).Seconds(), nil)
	return []byte("mock content"), &storage.ContentMetadata{}, nil
}

// StoreContent stores GitHub content (mock implementation for CLI)
func (m *mockGitHubContentManager) StoreContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
	data []byte,
	metadata map[string]interface{},
) (*storage.ContentMetadata, error) {
	startTime := time.Now()
	m.metricsClient.RecordOperation("github_content_manager", "store_content", true, time.Since(startTime).Seconds(), nil)
	return &storage.ContentMetadata{}, nil
}

// ListContent lists GitHub content (mock implementation for CLI)
func (m *mockGitHubContentManager) ListContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	limit int,
) ([]*storage.ContentMetadata, error) {
	return []*storage.ContentMetadata{}, nil
}

// DeleteContent deletes GitHub content (mock implementation for CLI)
func (m *mockGitHubContentManager) DeleteContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
) error {
	return nil
}

// GetContentByChecksum gets content by checksum (mock implementation for CLI)
func (m *mockGitHubContentManager) GetContentByChecksum(
	ctx context.Context,
	checksum string,
) ([]byte, *storage.ContentMetadata, error) {
	return []byte{}, &storage.ContentMetadata{}, nil
}

func main() {
	flag.Parse()

	// Validate flags
	if *repoOwner == "" || *repoName == "" {
		log.Fatal("Repository owner and name are required")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Create a minimal default config when loading fails
		cfg = &config.Config{
			Database: commonConfig.GetDefaultDatabaseConfig(),
		}
		log.Printf("Warning: Could not load config from %s, using defaults", *configPath)
	}

	// Connect to the database
	db, err := connectToDatabase(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// If API key is not specified in flags, try to get from environment variable
	if *apiKey == "" {
		*apiKey = os.Getenv("OPENAI_API_KEY")
		if *apiKey == "" {
			log.Fatal("API key is required via -api-key flag or OPENAI_API_KEY environment variable")
		}
	}

	// Create the chunking service
	chunkingService := createChunkingService()

	// Create a mock GitHub content manager that satisfies the interface
	// This is a workaround for embedding CLI which doesn't need full functionality
	// No need for metrics client in CLI tool
	
	// Create a mock content provider that implements the GitHubContentProvider interface
	// The embedding pipeline will work with this mock without needing a real GitHub content manager
	contentProvider := embedding.NewMockGitHubContentProvider()

	// Create the embedding factory config
	factoryConfig := &embedding.EmbeddingFactoryConfig{
		ModelType:       embedding.ModelTypeOpenAI,
		ModelName:       *modelName,
		ModelAPIKey:     *apiKey,
		ModelDimensions: 1536, // For OpenAI's text-embedding-3-small
		DatabaseConnection: db,
		DatabaseSchema:  "mcp",
		Concurrency:     *concurrency,
		BatchSize:       *batchSize,
		IncludeComments: *includeComments,
		EnrichMetadata:  true,
	}

	// Create the embedding factory
	factory, err := embedding.NewEmbeddingFactory(factoryConfig)
	if err != nil {
		log.Fatalf("Failed to create embedding factory: %v", err)
	}

	// Use the mock content provider directly as it already implements the GitHubContentProvider interface
	// contentProvider is already defined above

	// Create the embedding pipeline
	pipeline, err := factory.CreateEmbeddingPipeline(chunkingService, contentProvider)
	if err != nil {
		log.Fatalf("Failed to create embedding pipeline: %v", err)
	}

	// Create the embedding manager
	manager, err := core.NewEmbeddingManager(db, chunkingService, pipeline)
	if err != nil {
		log.Fatalf("Failed to create embedding manager: %v", err)
	}

	// Execute the specified command
	ctx := context.Background()
	switch *command {
	case "embed":
		err = runEmbedCommand(ctx, manager)
	case "search":
		err = runSearchCommand(ctx, manager)
	default:
		err = fmt.Errorf("unknown command: %s", *command)
	}

	if err != nil {
		log.Fatalf("Failed to execute command: %v", err)
	}

	log.Println("Completed successfully!")
}

func runEmbedCommand(ctx context.Context, manager *core.EmbeddingManager) error {
	// Check if we're embedding a file or issues
	if *filePath != "" {
		// Get file content from GitHub
		content, err := os.ReadFile(*filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Generate embeddings for the file
		err = manager.CreateEmbeddingsFromCodeFile(ctx, *repoOwner, *repoName, *filePath, content)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings for file: %w", err)
		}

		log.Printf("Successfully generated embeddings for file: %s", *filePath)
		return nil
	}

	if *issueNumbers != "" {
		// Parse issue numbers
		issueNums, err := parseIssueNumbers(*issueNumbers)
		if err != nil {
			return fmt.Errorf("failed to parse issue numbers: %w", err)
		}

		// Generate embeddings for issues
		err = manager.CreateEmbeddingsFromIssues(ctx, *repoOwner, *repoName, issueNums)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings for issues: %w", err)
		}

		log.Printf("Successfully generated embeddings for %d issues", len(issueNums))
		return nil
	}

	return fmt.Errorf("either -file or -issues must be specified")
}

func runSearchCommand(ctx context.Context, manager *core.EmbeddingManager) error {
	if *query == "" {
		return fmt.Errorf("query is required for search command")
	}

	// Search for similar content
	results, err := manager.SearchSimilarContent(
		ctx,
		*query,
		core.ModelTypeOpenAI,
		*modelName,
		*limit,
		float32(*threshold),
	)
	if err != nil {
		return fmt.Errorf("failed to search for similar content: %w", err)
	}

	// Print results
	fmt.Printf("Found %d similar items:\n\n", len(results))
	for i, result := range results {
		fmt.Printf("%d. ID: %s\n", i+1, result["id"])
		fmt.Printf("   Type: %s\n", result["content_type"])
		fmt.Printf("   Similarity: %.4f\n", result["similarity"])
		
		if text, ok := result["text"].(string); ok && text != "" {
			previewText := text
			if len(previewText) > 100 {
				previewText = previewText[:100] + "..."
			}
			fmt.Printf("   Preview: %s\n", previewText)
		}
		
		fmt.Println()
	}

	return nil
}

func parseIssueNumbers(input string) ([]int, error) {
	if input == "" {
		return nil, nil
	}

	parts := strings.Split(input, ",")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		num, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid issue number: %s", part)
		}
		result = append(result, num)
	}

	return result, nil
}

func connectToDatabase(dbConfig commonConfig.DatabaseConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Database,
		dbConfig.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	db.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func createChunkingService() *chunking.ChunkingService {
	service := chunking.NewChunkingService()
	
	// Register parsers for supported languages
	service.RegisterParser(parsers.NewGoParser())
	service.RegisterParser(parsers.NewJavaScriptParser())
	service.RegisterParser(parsers.NewJavaParser())
	service.RegisterParser(parsers.NewShellParser())
	// Add other parsers as needed
	
	return service
}
