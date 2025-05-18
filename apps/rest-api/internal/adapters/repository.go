// Package adapters provides compatibility adapters for the repository interfaces
package adapters

import (
	"context"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Embedding represents a vector embedding stored in the database
type Embedding struct {
	ID           string
	ContextID    string
	ContentIndex int
	Text         string
	Embedding    []float32
	ModelID      string
	CreatedAt    time.Time
	Metadata     map[string]interface{}
}

// AgentRepository defines the interface for agent operations
type AgentRepository interface {
	// Create creates a new agent
	Create(ctx context.Context, agent *models.Agent) error
	
	// Get retrieves an agent by ID
	Get(ctx context.Context, id string) (*models.Agent, error)
	
	// List retrieves agents based on filter criteria
	List(ctx context.Context, filter map[string]interface{}) ([]*models.Agent, error)
	
	// Update updates an existing agent
	Update(ctx context.Context, agent *models.Agent) error
	
	// Delete deletes an agent by ID
	Delete(ctx context.Context, id string) error
}

// ModelRepository defines the interface for model operations
type ModelRepository interface {
	// Create creates a new model
	Create(ctx context.Context, model *models.Model) error
	
	// Get retrieves a model by ID
	Get(ctx context.Context, id string) (*models.Model, error)
	
	// List retrieves models based on filter criteria
	List(ctx context.Context, filter map[string]interface{}) ([]*models.Model, error)
	
	// Update updates an existing model
	Update(ctx context.Context, model *models.Model) error
	
	// Delete deletes a model by ID
	Delete(ctx context.Context, id string) error
}

// VectorAPIRepository defines the interface for vector operations
type VectorAPIRepository interface {
	// StoreEmbedding stores a vector embedding
	StoreEmbedding(ctx context.Context, embedding *Embedding) error
	
	// SearchEmbeddings performs a vector search with various filter options
	SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*Embedding, error)
	
	// SearchEmbeddings_Legacy performs a legacy vector search
	SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error)
	
	// GetContextEmbeddings retrieves all embeddings for a context
	GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)
	
	// DeleteContextEmbeddings deletes all embeddings for a context
	DeleteContextEmbeddings(ctx context.Context, contextID string) error
	
	// GetEmbeddingsByModel retrieves all embeddings for a context and model
	GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error)
	
	// GetSupportedModels returns a list of models with embeddings
	GetSupportedModels(ctx context.Context) ([]string, error)
}

// SearchRepository defines the interface for search operations
type SearchRepository interface {
	// SearchByText performs a vector search using text
	SearchByText(ctx context.Context, query string, options *SearchOptions) (*SearchResults, error)
	
	// SearchByVector performs a vector search using a pre-computed vector
	SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error)
	
	// SearchByContentID performs a "more like this" search
	SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error)
	
	// GetSupportedModels returns a list of models with embeddings
	GetSupportedModels(ctx context.Context) ([]string, error)
	
	// GetSearchStats retrieves statistics about the search index
	GetSearchStats(ctx context.Context) (map[string]interface{}, error)
}

// SearchOptions defines options for search operations
type SearchOptions struct {
	Limit         int
	Offset        int
	MinSimilarity float32
	Filters       []SearchFilter
	Sorts         []SearchSort
	ContentTypes  []string
	WeightFactors map[string]float32
}

// SearchFilter defines a filter for search operations
type SearchFilter struct {
	Field    string
	Operator string
	Value    interface{}
}

// SearchSort defines a sort order for search operations
type SearchSort struct {
	Field     string
	Direction string
}

// SearchResults contains results from a search operation
type SearchResults struct {
	Results []*SearchResult
	Total   int
	HasMore bool
}

// SearchResult represents a single result item from a search
type SearchResult struct {
	ID          string
	Score       float32
	Distance    float32
	Content     string
	Type        string
	Metadata    map[string]interface{}
	ContentHash string
}
