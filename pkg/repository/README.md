# Repository Package Migration Guide

## Overview

This package has been migrated from a flat structure to a more modular approach using subpackages. The migration resolves issues with multiple redeclarations of interfaces and types while maintaining backward compatibility with existing API code.

## Repository Structure

The repository package now consists of:

- `pkg/repository/` - Root package containing factory methods and adapter interfaces
  - `pkg/repository/agent/` - Subpackage for agent entity operations
  - `pkg/repository/model/` - Subpackage for model entity operations
  - `pkg/repository/vector/` - Subpackage for vector embedding operations

Each subpackage follows a consistent structure:
- `interfaces.go` - Defines the core interfaces and types
- `repository.go` - Contains the implementation of the interfaces
- `mock.go` - Provides mock implementations for testing

## Usage Examples

### Using the Factory

```go
import (
    "database/sql"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
)

// Create a factory with an existing database connection
db, _ := sql.Open("postgres", "connection_string")
factory := repository.NewFactory(db)

// Access repositories via the factory
agentRepo := factory.GetAgentRepository()
modelRepo := factory.GetModelRepository()
vectorRepo := factory.GetVectorRepository()

// Use the repositories
agent, err := agentRepo.Get(ctx, "agent-id")
```

### Direct Repository Access

For code that needs direct access to the underlying implementations:

```go
import (
    "github.com/jmoiron/sqlx"
    "github.com/S-Corkum/devops-mcp/pkg/repository/agent"
    "github.com/S-Corkum/devops-mcp/pkg/repository/model"
    "github.com/S-Corkum/devops-mcp/pkg/repository/vector"
)

// Create database connection
db := sqlx.MustConnect("postgres", "connection_string")

// Create repositories directly
agentRepo := agent.NewRepository(db)
modelRepo := model.NewRepository(db)
vectorRepo := vector.NewRepository(db)
```

## Key Features and Implementation Details

### VectorAPIRepository Interface

The Vector Repository supports the following operations:

```go
type VectorAPIRepository interface {
    // Store a vector embedding
    StoreEmbedding(ctx context.Context, embedding *Embedding) error
    
    // Search embeddings with various filter options
    SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*Embedding, error)
    
    // Legacy search method for backward compatibility
    SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error)
    
    // Get all embeddings for a context
    GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)
    
    // Delete all embeddings for a context
    DeleteContextEmbeddings(ctx context.Context, contextID string) error
    
    // Get all embeddings for a specific model in a context
    GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error)
    
    // Get list of unique model IDs with stored embeddings
    GetSupportedModels(ctx context.Context) ([]string, error)
    
    // Delete all embeddings for a specific model in a context
    DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error
}
```

### DeleteModelEmbeddings Implementation

The `DeleteModelEmbeddings` method has been implemented across all repository layers to provide fine-grained control over vector embeddings storage:

```go
// SQL implementation in EmbeddingRepositoryImpl
func (r *EmbeddingRepositoryImpl) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
    query := `DELETE FROM embeddings WHERE context_id = $1 AND model_id = $2`
    _, err := r.db.ExecContext(ctx, query, contextID, modelID)
    return err
}

// In-memory implementation in MockRepository
func (m *MockVectorRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
    // Filter out embeddings for the specified model
    filtered := make([]*repository.Embedding, 0)
    for _, emb := range m.embeddings[contextID] {
        if emb.ModelID != modelID {
            filtered = append(filtered, emb)
        }
    }
    m.embeddings[contextID] = filtered
    return nil
}
```

## Migration Design Patterns

### Adapter Pattern

We've implemented the adapter pattern to reconcile the API expectations with our refactored repository interfaces. For example:

- API code expects `CreateAgent`, `ListAgents`, etc.
- Core repository uses `Create`, `List`, etc.

The adapters bridge this gap:

```go
// API expectation
agentRepo.CreateAgent(ctx, agent)

// Adapter implementation
func (a *LegacyAgentAdapter) CreateAgent(ctx context.Context, agent *models.Agent) error {
    return a.Create(ctx, agent)
}

// Repository implementation
func (r *RepositoryImpl) Create(ctx context.Context, agent *models.Agent) error {
    // Implementation details
}
```

### Multi-layer Adapters

For the Vector API repository, we use multiple layers of adaptation:

1. **Direct Repository Implementation**: `pkg/repository/vector/repository.go`
2. **Bridge Adapter**: `pkg/repository/vector_bridge.go` - Links pkg interfaces to implementation
3. **API Adapter**: `apps/rest-api/internal/adapters/repository_shim.go` - Adapts between API and repository types

### Type Aliases

When necessary, we've used type aliases to maintain compatibility:

```go
// In vector_bridge.go
type Embedding = vector.Embedding
```

## Testing

Each subpackage includes a mock implementation for testing purposes:

```go
import (
    "testing"
    "github.com/S-Corkum/devops-mcp/pkg/repository/agent"
)

func TestAgentOperations(t *testing.T) {
    // Use the mock repository for testing
    repo := agent.NewMockRepository()
    
    // Test against the mock
    agent, err := repo.Get(ctx, "test-id")
}
```
