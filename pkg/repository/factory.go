package repository

import (
	"database/sql"
)

// NewAgentRepository creates a new agent repository implementation
func NewAgentRepository(db *sql.DB) AgentRepository {
	// Ensure the adapter implements the interface fully
	var _ AgentRepository = (*agentRepositoryAdapter)(nil)
	return &agentRepositoryAdapter{
		db: db,
	}
}

// NewModelRepository creates a new model repository implementation
func NewModelRepository(db *sql.DB) ModelRepository {
	// Ensure the adapter implements the interface fully
	var _ ModelRepository = (*modelRepositoryAdapter)(nil)
	return &modelRepositoryAdapter{
		db: db,
	}
}

// NOTE: NewEmbeddingRepository is now defined in embedding_adapter.go

// agentRepositoryAdapter adapts between the API expectations and the repository interface
type agentRepositoryAdapter struct {
	db *sql.DB
}

// modelRepositoryAdapter adapts between the API expectations and the repository interface
type modelRepositoryAdapter struct {
	db *sql.DB
}
