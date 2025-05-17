package repository

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// AgentRepository defines operations for managing agent entities
type AgentRepository interface {
	// Core repository interface methods
	Create(ctx context.Context, agent *models.Agent) error
	Get(ctx context.Context, id string) (*models.Agent, error)
	List(ctx context.Context, filter map[string]interface{}) ([]*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	Delete(ctx context.Context, id string) error
	
	// API-specific method names that the agent_api.go expects
	CreateAgent(ctx context.Context, agent *models.Agent) error
	GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error)
	ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error)
	UpdateAgent(ctx context.Context, agent *models.Agent) error
	DeleteAgent(ctx context.Context, id string) error
}

// ModelRepository defines operations for managing model entities
type ModelRepository interface {
	// Core repository interface methods
	Create(ctx context.Context, model *models.Model) error
	Get(ctx context.Context, id string) (*models.Model, error)
	List(ctx context.Context, filter map[string]interface{}) ([]*models.Model, error)
	Update(ctx context.Context, model *models.Model) error
	Delete(ctx context.Context, id string) error
	
	// API-specific method names that the model_api.go expects
	CreateModel(ctx context.Context, model *models.Model) error
	GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error)
	ListModels(ctx context.Context, tenantID string) ([]*models.Model, error)
	UpdateModel(ctx context.Context, model *models.Model) error
	DeleteModel(ctx context.Context, id string) error
}

// VectorRepository defines operations for managing vector embeddings
type VectorRepository interface {
	StoreVectors(ctx context.Context, vectors []*models.Vector) error
	FindSimilar(ctx context.Context, vector []float32, limit int, filter map[string]interface{}) ([]*models.Vector, error)
	DeleteVectors(ctx context.Context, ids []string) error
	GetVector(ctx context.Context, id string) (*models.Vector, error)
}

// VectorAPIRepository defines the interface expected by the vector API code
type VectorAPIRepository interface {
	StoreEmbedding(ctx context.Context, embedding *Embedding) error
	SearchEmbeddings(ctx context.Context, queryEmbedding []float32, contextID string, modelID string, limit int, threshold float64) ([]*Embedding, error)
	SearchEmbeddings_Legacy(ctx context.Context, queryEmbedding []float32, contextID string, limit int) ([]*Embedding, error)
	GetEmbedding(ctx context.Context, id string) (*Embedding, error)
	DeleteEmbedding(ctx context.Context, id string) error
	
	// Additional methods needed by vector_handlers.go
	GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)
	DeleteContextEmbeddings(ctx context.Context, contextID string) error
	GetSupportedModels(ctx context.Context) ([]string, error)
	GetEmbeddingsByModel(ctx context.Context, tenantID string, modelID string) ([]*Embedding, error)
	DeleteModelEmbeddings(ctx context.Context, tenantID string, modelID string) error
}
