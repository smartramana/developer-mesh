# Repository Package

> **Purpose**: Data access layer with repository pattern implementation for the DevOps MCP platform
> **Status**: Modular Architecture with Subpackages
> **Dependencies**: PostgreSQL, sqlx, database/sql

## Overview

The repository package provides a data access layer using the repository pattern. It has been organized into modular subpackages for different entity types while maintaining backward compatibility through adapters and type aliases.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Repository Package                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Factory ──► Subpackages ──► Database                      │
│     │            │                                          │
│     │            ├── agent/    (Agent operations)          │
│     │            ├── model/    (Model operations)          │
│     │            ├── vector/   (Embedding operations)      │
│     │            ├── search/   (Search operations)         │
│     │            └── postgres/ (Base repository)           │
│     │                                                       │
│     └──► Legacy Adapters (Backward compatibility)          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Package Structure

- `pkg/repository/` - Root package with factory and adapters
  - `factory.go` - Repository factory for creating instances
  - `interfaces.go` - Type aliases for backward compatibility
  - `*_bridge.go` - Adapter implementations
  
- Subpackages:
  - `agent/` - Agent entity repository
  - `model/` - Model entity repository  
  - `vector/` - Vector embedding repository
  - `search/` - Search functionality
  - `postgres/` - Base PostgreSQL repository
  - `interfaces/` - Shared interfaces

Each subpackage contains:
- `interfaces.go` - Repository interface definition
- `repository.go` - PostgreSQL implementation
- `mock.go` - Mock implementation for testing

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

### Direct Repository Access (V2)

For new code, use the V2 methods that return the actual subpackage types:

```go
// Get V2 repositories (recommended for new code)
agentRepoV2 := factory.GetAgentRepositoryV2()  // Returns agent.Repository
modelRepoV2 := factory.GetModelRepositoryV2()  // Returns model.Repository
vectorRepoV2 := factory.GetVectorRepositoryV2() // Returns vector.Repository
searchRepo := factory.GetSearchRepository()     // Returns search.Repository
```

### Direct Instantiation

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

## Repository Interfaces

### Agent Repository

```go
type AgentRepository interface {
    // CRUD operations
    CreateAgent(ctx context.Context, agent *models.Agent) error
    GetAgent(ctx context.Context, agentID string) (*models.Agent, error)
    UpdateAgent(ctx context.Context, agent *models.Agent) error
    DeleteAgent(ctx context.Context, agentID string) error
    
    // List operations
    ListAgents(ctx context.Context, tenantID uuid.UUID) ([]*models.Agent, error)
    ListAgentsByStatus(ctx context.Context, tenantID uuid.UUID, status string) ([]*models.Agent, error)
    
    // Capability operations
    GetAgentsByCapability(ctx context.Context, tenantID uuid.UUID, capability string) ([]*models.Agent, error)
}
```

### Model Repository

```go
type ModelRepository interface {
    // CRUD operations
    CreateModel(ctx context.Context, model *models.Model) error
    GetModel(ctx context.Context, modelID string) (*models.Model, error)
    UpdateModel(ctx context.Context, model *models.Model) error
    DeleteModel(ctx context.Context, modelID string) error
    
    // List operations
    ListModels(ctx context.Context, tenantID uuid.UUID) ([]*models.Model, error)
    ListEnabledModels(ctx context.Context, tenantID uuid.UUID) ([]*models.Model, error)
}
```

### Vector Repository

```go
type VectorAPIRepository interface {
    // Store a vector embedding
    StoreEmbedding(ctx context.Context, embedding *Embedding) error
    
    // Search embeddings with various filter options
    SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, 
                     modelID string, limit int, similarityThreshold float64) ([]*Embedding, error)
    
    // Context operations
    GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)
    DeleteContextEmbeddings(ctx context.Context, contextID string) error
    
    // Model operations
    GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error)
    DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error
    GetSupportedModels(ctx context.Context) ([]string, error)
}
```

### Search Repository

```go
type SearchRepository interface {
    // Search with options
    Search(ctx context.Context, query string, options SearchOptions) (*SearchResults, error)
    
    // Type-specific searches
    SearchAgents(ctx context.Context, query string, tenantID uuid.UUID) ([]*models.Agent, error)
    SearchModels(ctx context.Context, query string, tenantID uuid.UUID) ([]*models.Model, error)
    SearchTasks(ctx context.Context, query string, tenantID uuid.UUID) ([]*models.Task, error)
}

```

## Design Patterns

### Repository Pattern

Each repository follows the repository pattern for data access:

```go
// Generic repository interface
type Repository[T any] interface {
    Create(ctx context.Context, entity T) error
    Get(ctx context.Context, id string) (T, error)
    Update(ctx context.Context, entity T) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, opts ...interface{}) ([]T, error)
}
```

### Adapter Pattern for Backward Compatibility

Legacy adapters maintain compatibility with existing API code:

```go
// Legacy adapter bridges old and new interfaces
type LegacyAgentAdapter struct {
    repo agent.Repository
}

// API expects CreateAgent, adapter calls Create
func (a *LegacyAgentAdapter) CreateAgent(ctx context.Context, agent *models.Agent) error {
    return a.repo.Create(ctx, agent)
}
```

### Factory Pattern

The factory provides a centralized way to create repositories:

```go
factory := repository.NewFactory(db)

// Legacy methods for backward compatibility
agentRepo := factory.GetAgentRepository()

// V2 methods for new code
agentRepoV2 := factory.GetAgentRepositoryV2()
```

## Testing

### Using Mock Repositories

Each subpackage provides mock implementations:

```go
import (
    "testing"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
    "github.com/S-Corkum/devops-mcp/pkg/repository/agent"
)

func TestWithFactory(t *testing.T) {
    // Factory returns mocks when created with nil DB
    factory := repository.NewFactory(nil)
    
    agentRepo := factory.GetAgentRepositoryV2()
    // agentRepo is now a mock implementation
}

func TestDirectMock(t *testing.T) {
    // Create mock directly
    mockRepo := agent.NewMockRepository()
    
    // Add test data
    testAgent := &models.Agent{ID: "test-123"}
    mockRepo.Create(ctx, testAgent)
    
    // Test operations
    agent, err := mockRepo.Get(ctx, "test-123")
    assert.NoError(t, err)
    assert.Equal(t, testAgent.ID, agent.ID)
}
```

### Mock Features

- In-memory storage
- Full interface compliance
- Thread-safe operations
- Configurable behavior

## Additional Repositories

### PostgreSQL Base Repository

The `postgres/` subpackage provides:

```go
// BaseRepository provides common database operations
type BaseRepository struct {
    db *sqlx.DB
}

// Transaction support
func (r *BaseRepository) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error

// Common query builders
func (r *BaseRepository) BuildInsertQuery(table string, fields []string) string
func (r *BaseRepository) BuildUpdateQuery(table string, fields []string) string
```

### Document Repository

For document operations:

```go
type DocumentRepository interface {
    // Document CRUD
    CreateDocument(ctx context.Context, doc *models.Document) error
    GetDocument(ctx context.Context, id uuid.UUID) (*models.Document, error)
    UpdateDocument(ctx context.Context, doc *models.Document) error
    DeleteDocument(ctx context.Context, id uuid.UUID) error
    
    // Workspace operations
    ListWorkspaceDocuments(ctx context.Context, workspaceID uuid.UUID) ([]*models.Document, error)
}
```

### Task Repository

For task management:

```go
type TaskRepository interface {
    // Task CRUD
    CreateTask(ctx context.Context, task *models.Task) error
    GetTask(ctx context.Context, id uuid.UUID) (*models.Task, error)
    UpdateTask(ctx context.Context, task *models.Task) error
    DeleteTask(ctx context.Context, id uuid.UUID) error
    
    // Queue operations
    GetPendingTasks(ctx context.Context, limit int) ([]*models.Task, error)
    ClaimTask(ctx context.Context, taskID uuid.UUID, agentID string) error
}
```

## Implementation Status

**Implemented:**
- Modular subpackage architecture
- Factory pattern for repository creation
- Legacy adapters for backward compatibility
- Mock implementations for all repositories
- PostgreSQL implementations
- Type aliases for smooth migration

**Features:**
- Thread-safe operations
- Context support
- Transaction support (in postgres subpackage)
- Comprehensive error handling
- pgvector support for embeddings

## Best Practices

1. **Use Factory**: Create repositories through the factory for consistency
2. **V2 for New Code**: Use V2 methods that return actual subpackage types
3. **Context**: Always pass context for cancellation support
4. **Transactions**: Use postgres.BaseRepository for transaction support
5. **Testing**: Use mock repositories for unit tests

---

Package Version: 2.0.0 (Modular Architecture)
Last Updated: 2024-01-23
