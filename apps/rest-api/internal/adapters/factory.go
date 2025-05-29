// Package adapters provides compatibility adapters for the repository interfaces
package adapters

import (
	corerepo "github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/jmoiron/sqlx"
	"rest-api/internal/repository"
)

// RepositoryFactory creates repository instances with the appropriate adapters
type RepositoryFactory struct {
	db *sqlx.DB
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(db *sqlx.DB) *RepositoryFactory {
	return &RepositoryFactory{
		db: db,
	}
}

// NewAgentRepository creates a new AgentRepository with the appropriate adapter
func (f *RepositoryFactory) NewAgentRepository() repository.AgentRepository {
	// Create the core repository implementation
	// Note: Handling the DB type difference between sqlx.DB and sql.DB
	coreRepo := corerepo.NewAgentRepositoryAdapter(f.db.DB)

	// Wrap it with the adapter
	return NewAgentAdapter(coreRepo)
}

// NewModelRepository creates a new ModelRepository with the appropriate adapter
func (f *RepositoryFactory) NewModelRepository() repository.ModelRepository {
	// Create the core repository implementation
	// Note: Handling the DB type difference between sqlx.DB and sql.DB
	coreRepo := corerepo.NewModelRepository(f.db.DB)

	// Wrap it with the adapter
	return NewModelAdapter(coreRepo)
}

// NewVectorRepository creates a new VectorAPIRepository with the appropriate adapter
func (f *RepositoryFactory) NewVectorRepository() repository.VectorAPIRepository {
	// Create the core repository implementation
	// Note: Handling the DB type difference between sqlx.DB and sql.DB
	coreRepo := corerepo.NewEmbeddingAdapter(f.db.DB)

	// Wrap it with the adapter
	return NewServerEmbeddingAdapter(coreRepo)
}

// NewSearchRepository creates a new SearchRepository using a mock implementation
// for initial development and testing
func (f *RepositoryFactory) NewSearchRepository() repository.SearchRepository {
	// Use mock implementation for search repository to avoid dependency issues
	// This will be replaced with a proper implementation in the future
	return NewMockSearchAdapter()
}
