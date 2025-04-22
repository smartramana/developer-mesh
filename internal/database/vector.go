package database

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/S-Corkum/mcp-server/internal/common"
	commonConfig "github.com/S-Corkum/mcp-server/internal/common/config"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/jmoiron/sqlx"
)

// VectorDatabase provides specialized database operations for vector data
type VectorDatabase struct {
	db          *sqlx.DB
	vectorDB    *sqlx.DB
	logger      *observability.Logger
	config      *commonConfig.DatabaseVectorConfig
	initialized bool
	lock        sync.RWMutex
}

// NewVectorDatabase creates a new vector database
func NewVectorDatabase(db *sqlx.DB, cfg interface{}, logger *observability.Logger) (*VectorDatabase, error) {
	if logger == nil {
		logger = observability.NewLogger("vector_database")
	}
	
	// Create a dedicated connection pool for vector operations if enabled
	var vectorDB *sqlx.DB
	
	// Use the main database connection pool by default
	vectorDB = db
	
	// Create a default config if none provided
	vectorConfig := &commonConfig.DatabaseVectorConfig{
		Enabled:         true,
		Dimensions:      1536,
		SimilarityMetric: "cosine",
	}
	
	return &VectorDatabase{
		db:          db,
		vectorDB:    vectorDB,
		logger:      logger,
		config:      vectorConfig,
		initialized: false,
	}, nil
}

// Initialize ensures the vector database is properly set up
func (vdb *VectorDatabase) Initialize(ctx context.Context) error {
	vdb.lock.Lock()
	defer vdb.lock.Unlock()
	
	if vdb.initialized {
		return nil
	}
	
	// Check if pgvector extension is installed
	var extExists bool
	err := vdb.vectorDB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_extension WHERE extname = 'vector'
		)
	`).Scan(&extExists)
	
	if err != nil {
		return fmt.Errorf("failed to check if pgvector extension exists: %w", err)
	}
	
	if !extExists {
		return fmt.Errorf("pgvector extension is not installed")
	}
	
	// Check if the embeddings table exists
	var tableExists bool
	err = vdb.vectorDB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.tables 
			WHERE table_schema = 'mcp' AND table_name = 'embeddings'
		)
	`).Scan(&tableExists)
	
	if err != nil {
		return fmt.Errorf("failed to check if embeddings table exists: %w", err)
	}
	
	if !tableExists {
		vdb.logger.Warn("Embeddings table does not exist; migrations may need to be run", nil)
		return fmt.Errorf("embeddings table does not exist")
	}
	
	vdb.initialized = true
	vdb.logger.Info("Vector database initialized successfully", nil)
	
	return nil
}

// Transaction runs a vector database operation in a transaction
func (vdb *VectorDatabase) Transaction(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	// Use the vector-specific connection pool
	tx, err := vdb.vectorDB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	// Execute the operation
	if err := fn(tx); err != nil {
		// Roll back on error
		if rbErr := tx.Rollback(); rbErr != nil {
			vdb.logger.Error("Failed to roll back transaction", map[string]interface{}{
				"error": rbErr.Error(),
			})
		}
		return err
	}
	
	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// Close closes the vector database connection pool
func (vdb *VectorDatabase) Close() error {
	vdb.lock.Lock()
	defer vdb.lock.Unlock()
	
	// Only close the dedicated pool if it's different from the main pool
	if vdb.vectorDB != vdb.db {
		return vdb.vectorDB.Close()
	}
	
	return nil
}

// GetVectorSearchConfig returns the current vector search configuration
func (vdb *VectorDatabase) GetVectorSearchConfig() *commonConfig.DatabaseVectorConfig {
	vdb.lock.RLock()
	defer vdb.lock.RUnlock()
	
	return vdb.config
}

// CheckVectorDimensions returns the available vector dimensions in the database
func (vdb *VectorDatabase) CheckVectorDimensions(ctx context.Context) ([]int, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return nil, err
	}
	
	// Query distinct dimensions
	rows, err := vdb.vectorDB.QueryContext(ctx, `
		SELECT DISTINCT vector_dimensions
		FROM mcp.embeddings
		ORDER BY vector_dimensions
	`)
	
	if err != nil {
		return nil, fmt.Errorf("failed to query vector dimensions: %w", err)
	}
	defer rows.Close()
	
	// Process results
	var dimensions []int
	
	for rows.Next() {
		var dim int
		if err := rows.Scan(&dim); err != nil {
			return nil, fmt.Errorf("failed to scan dimension: %w", err)
		}
		dimensions = append(dimensions, dim)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over dimensions: %w", err)
	}
	
	return dimensions, nil
}

// NormalizeVector applies normalization to a vector based on the chosen similarity metric
func NormalizeVector(vector []float32, method string) ([]float32, error) {
	if len(vector) == 0 {
		return vector, nil
	}
	
	switch method {
	case "cosine":
		// Cosine similarity requires L2 normalization
		return common.NormalizeVectorL2(vector), nil
	case "dot":
		// Dot product doesn't require normalization
		return vector, nil
	case "euclidean":
		// Euclidean distance doesn't require normalization
		return vector, nil
	default:
		return nil, fmt.Errorf("unsupported normalization method: %s", method)
	}
}
