package database

import (
	"context"
	"fmt"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/common"
	commonConfig "github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/jmoiron/sqlx"
	
	// Import pkg versions for migration
	pkgDb "github.com/S-Corkum/devops-mcp/pkg/database"
	pkgObs "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// VectorDatabase provides specialized database operations for vector data
type VectorDatabase struct {
	db          *sqlx.DB
	vectorDB    *sqlx.DB
	logger      *observability.Logger
	config      *commonConfig.DatabaseVectorConfig
	initialized bool
	lock        sync.RWMutex
	
	// Reference to pkg implementation
	pkgVectorDB *pkgDb.VectorDatabase
}

// NewVectorDatabase creates a new vector database
func NewVectorDatabase(db *sqlx.DB, cfg interface{}, logger *observability.Logger) (*VectorDatabase, error) {
	if logger == nil {
		logger = observability.NewLogger("vector_database")
	}
	
	// Use the main database connection pool by default
	vectorDB := db
	
	// Try to extract vector config from the provided config
	var vectorConfig *commonConfig.DatabaseVectorConfig
	
	// If the config is a Config type from config package
	if config, ok := cfg.(*config.Config); ok && config != nil {
		vectorConfig = &config.Database.Vector
	} else if dbConfig, ok := cfg.(*commonConfig.DatabaseConfig); ok && dbConfig != nil {
		vectorConfig = &dbConfig.Vector
	} else if vConfig, ok := cfg.(*commonConfig.DatabaseVectorConfig); ok && vConfig != nil {
		vectorConfig = vConfig
	} else {
		// Create a default config if none provided
		vectorConfig = &commonConfig.DatabaseVectorConfig{
			Enabled:         true,
			Dimensions:      1536,
			SimilarityMetric: "cosine",
		}
	}
	
	// Create an adapter for the pkg/observability.Logger interface
	pkgLogger := &obsLoggerAdapter{internal: logger}
	
	// Create the pkg implementation
	pkgVectorDB, err := pkgDb.NewVectorDatabase(db, cfg, pkgLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create pkg vector database: %w", err)
	}
	
	// Create the internal VectorDatabase that delegates to pkg implementation
	vectorDatabase := &VectorDatabase{
		db:          db,
		vectorDB:    vectorDB,
		logger:      logger,
		config:      vectorConfig,
		initialized: false,
		pkgVectorDB: pkgVectorDB,
	}
	
	return vectorDatabase, nil
}

// Initialize ensures the vector database is properly set up
func (vdb *VectorDatabase) Initialize(ctx context.Context) error {
	vdb.lock.Lock()
	defer vdb.lock.Unlock()
	
	if vdb.initialized {
		return nil
	}
	
	// Use the pkg implementation
	err := vdb.pkgVectorDB.Initialize(ctx)
	if err != nil {
		return err
	}
	
	vdb.initialized = true
	return nil
}

// Transaction runs a vector database operation in a transaction
func (vdb *VectorDatabase) Transaction(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	if err := vdb.Initialize(ctx); err != nil {
		return err
	}
	
	// Use the pkg implementation
	return vdb.pkgVectorDB.Transaction(ctx, fn)
}

// Close closes the vector database connection pool
func (vdb *VectorDatabase) Close() error {
	vdb.lock.Lock()
	defer vdb.lock.Unlock()
	
	// Use the pkg implementation
	err := vdb.pkgVectorDB.Close()
	vdb.initialized = false
	return err
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
	
	// Use the pkg implementation
	return vdb.pkgVectorDB.CheckVectorDimensions(ctx)
}

// NormalizeVector applies normalization to a vector based on the chosen similarity metric
func NormalizeVector(vector []float32, method string) ([]float32, error) {
	// Use the pkg implementation
	return pkgDb.NormalizeVector(vector, method)
}

// CreateVector creates a new vector from float32 array
func (vdb *VectorDatabase) CreateVector(ctx context.Context, vector []float32) (string, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return "", err
	}
	
	// Use the pkg implementation
	return vdb.pkgVectorDB.CreateVector(ctx, vector)
}

// formatFloatArray formats a float slice as a comma-separated string
// This is kept for backward compatibility but the actual implementation uses the pkg version
func formatFloatArray(arr []float64) string {
	if len(arr) == 0 {
		return ""
	}
	
	result := fmt.Sprintf("%f", arr[0])
	for i := 1; i < len(arr); i++ {
		result += fmt.Sprintf(",%f", arr[i])
	}
	
	return result
}

// CalculateSimilarity calculates the similarity between two vectors
func (vdb *VectorDatabase) CalculateSimilarity(ctx context.Context, vector1, vector2 []float32, method string) (float64, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return 0, err
	}
	
	// Use the pkg implementation
	return vdb.pkgVectorDB.CalculateSimilarity(ctx, vector1, vector2, method)
}
