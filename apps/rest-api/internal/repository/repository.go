// Package repository provides interfaces for the REST API
// These interfaces are isolated from the pkg/repository package
// to prevent circular dependencies and redeclaration issues
package repository

import (
	"context"

	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/types"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Embedding is an alias for types.Embedding
type Embedding = types.Embedding

// No conversion functions needed - we are keeping our interfaces isolated

// AgentRepository defines the interface expected by the agent API
type AgentRepository interface {
	// CreateAgent creates a new agent
	CreateAgent(ctx context.Context, agent *models.Agent) error

	// GetAgentByID retrieves an agent by ID and tenant ID
	GetAgentByID(ctx context.Context, tenantID, id string) (*models.Agent, error)

	// ListAgents retrieves all agents for a given tenant
	ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error)

	// UpdateAgent updates an existing agent
	UpdateAgent(ctx context.Context, agent *models.Agent) error

	// DeleteAgent deletes an agent by ID
	DeleteAgent(ctx context.Context, id string) error
}

// ModelRepository defines the interface expected by the model API
type ModelRepository interface {
	// CreateModel creates a new model
	CreateModel(ctx context.Context, model *models.Model) error

	// GetModelByID retrieves a model by ID and tenant ID
	GetModelByID(ctx context.Context, tenantID, id string) (*models.Model, error)

	// ListModels retrieves all models for a given tenant
	ListModels(ctx context.Context, tenantID string) ([]*models.Model, error)

	// UpdateModel updates an existing model
	UpdateModel(ctx context.Context, model *models.Model) error

	// DeleteModel deletes a model by ID
	DeleteModel(ctx context.Context, id string) error

	// SearchModels searches for models by query string (tenant-scoped)
	SearchModels(ctx context.Context, tenantID, query string, limit, offset int) ([]*models.Model, error)
}

// VectorAPIRepository defines the interface expected by the vector API
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

	// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
	DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error
}

// SearchOptions is an alias for types.SearchOptions
type SearchOptions = types.SearchOptions

// SearchResult is an alias for types.SearchResult
type SearchResult = types.SearchResult

// SearchResults is an alias for types.SearchResults
type SearchResults = types.SearchResults

// SearchRepository defines the interface for search operations
type SearchRepository interface {
	// SearchByContentID searches for embeddings by content ID
	SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error)

	// SearchByText searches for embeddings by text
	SearchByText(ctx context.Context, text string, options *SearchOptions) (*SearchResults, error)

	// SearchByEmbedding searches for embeddings by embedding vector
	SearchByEmbedding(ctx context.Context, embedding []float32, options *SearchOptions) (*SearchResults, error)
}
