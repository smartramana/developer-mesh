package proxies

import (
	"context"
	"time"

	// Using pkg/database and adapting to the internal database models instead of importing them directly
	pkgdb "github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// Embedding represents a vector embedding in the internal database model format
// This struct mirrors the internal/database.Embedding struct to avoid importing it directly
type Embedding struct {
	ID          string                 `json:"id" db:"id"`
	Vector      []float32              `json:"vector" db:"vector"`
	Dimensions  int                    `json:"dimensions" db:"dimensions"`
	ModelID     string                 `json:"model_id" db:"model_id"`
	ContentType string                 `json:"content_type" db:"content_type"`
	ContentID   string                 `json:"content_id" db:"content_id"`
	Namespace   string                 `json:"namespace" db:"namespace"`
	ContextID   string                 `json:"context_id" db:"context_id"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
	Similarity  float64                `json:"similarity" db:"similarity"`
}

// VectorDatabaseAdapter provides compatibility between internal/database.VectorDatabase and pkg/database.VectorDatabase
// This adapter allows us to migrate code incrementally without breaking existing functionality
type VectorDatabaseAdapter struct {
	db        *sqlx.DB
	pkgVector *pkgdb.VectorDatabase
	logger    observability.Logger
}

// NewVectorDatabaseAdapter creates a new adapter that wraps a pkg/database.VectorDatabase instance
// but exposes it with the same interface as internal/database.VectorDatabase
func NewVectorDatabaseAdapter(db *sqlx.DB, cfg interface{}, logger observability.Logger) (*VectorDatabaseAdapter, error) {
	// Create the new pkg/database.VectorDatabase
	pkgVector, err := pkgdb.NewVectorDatabase(db, nil, logger)
	if err != nil {
		return nil, err
	}

	return &VectorDatabaseAdapter{
		db:        db,
		pkgVector: pkgVector,
		logger:    logger,
	}, nil
}

// Initialize initializes the vector database tables and indexes
func (a *VectorDatabaseAdapter) Initialize(ctx context.Context) error {
	return a.pkgVector.Initialize(ctx)
}

// EnsureSchema ensures the vector database schema exists
func (a *VectorDatabaseAdapter) EnsureSchema(ctx context.Context) error {
	return a.pkgVector.EnsureSchema(ctx)
}

// Transaction initiates a transaction and passes control to the provided function
func (a *VectorDatabaseAdapter) Transaction(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	return a.pkgVector.Transaction(ctx, fn)
}

// StoreEmbedding stores an embedding vector in the database
func (a *VectorDatabaseAdapter) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	// Convert from internal to pkg embedding format
	pkgEmbedding := &pkgdb.Embedding{
		ID:          embedding.ID,
		Vector:      embedding.Vector,
		Dimensions:  embedding.Dimensions,
		ModelID:     embedding.ModelID,
		ContentType: embedding.ContentType,
		ContentID:   embedding.ContentID,
		Namespace:   embedding.Namespace,
		ContextID:   embedding.ContextID,
		CreatedAt:   embedding.CreatedAt,
		UpdatedAt:   embedding.UpdatedAt,
		Metadata:    embedding.Metadata,
	}

	// Use transaction to store embedding
	return a.pkgVector.Transaction(ctx, func(tx *sqlx.Tx) error {
		return a.pkgVector.StoreEmbedding(ctx, tx, pkgEmbedding)
	})
}

// SearchEmbeddings searches for similar embeddings in the vector database
func (a *VectorDatabaseAdapter) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*Embedding, error) {
	var results []*Embedding

	// Use transaction for read operations
	err := a.pkgVector.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Call the pkg implementation
		pkgResults, err := a.pkgVector.SearchEmbeddings(ctx, tx, queryVector, contextID, modelID, limit, similarityThreshold)
		if err != nil {
			return err
		}

		// Convert from pkg to internal embedding format
		results = make([]*Embedding, len(pkgResults))
		for i, pkgEmb := range pkgResults {
			results[i] = &Embedding{
				ID:          pkgEmb.ID,
				Vector:      pkgEmb.Vector,
				Dimensions:  pkgEmb.Dimensions,
				ModelID:     pkgEmb.ModelID,
				ContentType: pkgEmb.ContentType,
				ContentID:   pkgEmb.ContentID,
				Namespace:   pkgEmb.Namespace,
				ContextID:   pkgEmb.ContextID,
				CreatedAt:   pkgEmb.CreatedAt,
				UpdatedAt:   pkgEmb.UpdatedAt,
				Metadata:    pkgEmb.Metadata,
				Similarity:  pkgEmb.Similarity, // This field is filled during search
			}
		}
		return nil
	})

	return results, err
}

// GetEmbeddingByID retrieves an embedding by ID
func (a *VectorDatabaseAdapter) GetEmbeddingByID(ctx context.Context, id string) (*Embedding, error) {
	var result *Embedding

	// Use transaction for read operations
	err := a.pkgVector.Transaction(ctx, func(tx *sqlx.Tx) error {
		// Call the pkg implementation
		pkgEmb, err := a.pkgVector.GetEmbeddingByID(ctx, tx, id)
		if err != nil {
			return err
		}

		// Convert from pkg to internal embedding format
		result = &Embedding{
			ID:          pkgEmb.ID,
			Vector:      pkgEmb.Vector,
			Dimensions:  pkgEmb.Dimensions,
			ModelID:     pkgEmb.ModelID,
			ContentType: pkgEmb.ContentType,
			ContentID:   pkgEmb.ContentID,
			Namespace:   pkgEmb.Namespace,
			ContextID:   pkgEmb.ContextID,
			CreatedAt:   pkgEmb.CreatedAt,
			UpdatedAt:   pkgEmb.UpdatedAt,
			Metadata:    pkgEmb.Metadata,
		}
		return nil
	})

	return result, err
}

// DeleteEmbedding deletes an embedding from the database
func (a *VectorDatabaseAdapter) DeleteEmbedding(ctx context.Context, id string) error {
	// Use transaction for write operations
	return a.pkgVector.Transaction(ctx, func(tx *sqlx.Tx) error {
		return a.pkgVector.DeleteEmbedding(ctx, tx, id)
	})
}

// BatchDeleteEmbeddings deletes multiple embeddings matching criteria
func (a *VectorDatabaseAdapter) BatchDeleteEmbeddings(ctx context.Context, contentType, contentID, contextID string) (int, error) {
	var count int

	// Use transaction for write operations
	err := a.pkgVector.Transaction(ctx, func(tx *sqlx.Tx) error {
		var err error
		count, err = a.pkgVector.BatchDeleteEmbeddings(ctx, tx, contentType, contentID, contextID)
		return err
	})

	return count, err
}

// Close closes the vector database connection
func (a *VectorDatabaseAdapter) Close() error {
	// No explicit close needed for pkg/database.VectorDatabase
	return nil
}
