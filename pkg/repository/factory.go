// Package repository provides factory methods for creating repositories
package repository

import (
	"database/sql"
	
	"github.com/S-Corkum/devops-mcp/pkg/repository/agent"
	"github.com/S-Corkum/devops-mcp/pkg/repository/model"
	"github.com/S-Corkum/devops-mcp/pkg/repository/search"
	"github.com/S-Corkum/devops-mcp/pkg/repository/vector"
	"github.com/jmoiron/sqlx"
)

// Factory provides a unified interface for creating repositories
type Factory struct {
	db *sqlx.DB
}

// NewFactory creates a new repository factory
func NewFactory(db interface{}) *Factory {
	var sqlxDB *sqlx.DB
	
	switch typedDB := db.(type) {
	case *sqlx.DB:
		sqlxDB = typedDB
	case *sql.DB:
		sqlxDB = sqlx.NewDb(typedDB, "postgres")
	default:
		// Return factory with nil DB for testing scenarios
		return &Factory{db: nil}
	}
	
	return &Factory{db: sqlxDB}
}

// LegacyNewAgentRepository creates a new agent repository implementation
// This maintains compatibility with existing code
func LegacyNewAgentRepository(db *sql.DB) AgentRepository {
	return NewLegacyAgentAdapter(db)
}

// GetAgentRepository returns an agent repository from the factory
func (f *Factory) GetAgentRepository() AgentRepository {
	return NewLegacyAgentAdapter(f.db)
}

// LegacyNewModelRepository creates a new model repository implementation
// This maintains compatibility with existing code
func LegacyNewModelRepository(db *sql.DB) ModelRepository {
	return NewLegacyModelAdapter(db)
}

// GetModelRepository returns a model repository from the factory
func (f *Factory) GetModelRepository() ModelRepository {
	return NewLegacyModelAdapter(f.db)
}

// GetVectorRepository returns a vector repository from the factory
func (f *Factory) GetVectorRepository() VectorAPIRepository {
	// We're using the adapter defined in vector_bridge.go, not directly from the vector package
	// This avoids compatibility issues during the transition
	return NewEmbeddingAdapter(f.db)
}

// GetAgentRepositoryV2 returns the new agent repository implementation from the agent subpackage
func (f *Factory) GetAgentRepositoryV2() agent.Repository {
	if f.db == nil {
		return agent.NewMockRepository()
	}
	return agent.NewRepository(f.db)
}

// GetModelRepositoryV2 returns the new model repository implementation from the model subpackage
func (f *Factory) GetModelRepositoryV2() model.Repository {
	if f.db == nil {
		return model.NewMockRepository()
	}
	return model.NewRepository(f.db)
}

// GetVectorRepositoryV2 returns the new vector repository implementation from the vector subpackage
func (f *Factory) GetVectorRepositoryV2() vector.Repository {
	if f.db == nil {
		return vector.NewMockRepository()
	}
	
	// Since f.db is already a *sqlx.DB, we can pass it directly
	return vector.NewRepository(f.db)
}

// GetSearchRepository returns a search repository from the factory
func (f *Factory) GetSearchRepository() SearchRepository {
	if f.db == nil {
		return search.NewMockRepository()
	}
	
	// Since f.db is already a *sqlx.DB, we can pass it directly
	return search.NewRepository(f.db)
}

// NOTE: NewEmbeddingAdapter (previously NewEmbeddingRepository) is defined in vector_bridge.go
