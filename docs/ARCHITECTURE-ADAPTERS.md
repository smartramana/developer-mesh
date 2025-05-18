# Repository and Adapter Pattern Architecture

This document outlines the architecture of the repository layer in the devops-mcp project, specifically focusing on the adapter pattern implementation used to bridge different interface requirements.

## Repository Structure

The repository layer is organized into several key components:

### 1. Core Repository Interfaces

Located in `pkg/repository/interfaces.go`, these define the primary contract that all repository implementations must fulfill:

```go
type VectorAPIRepository interface {
    StoreEmbedding(ctx context.Context, embedding *Embedding) error
    SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*Embedding, error)
    SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error)
    GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)
    DeleteContextEmbeddings(ctx context.Context, contextID string) error
    GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error)
    GetSupportedModels(ctx context.Context) ([]string, error)
    DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error
}
```

### 2. Implementation-Specific Repositories

These implement the core interfaces for different storage backends:

- SQL-based implementation: `pkg/repository/vector/repository.go`
- In-memory implementation for testing: `pkg/repository/vector/mock.go`
- Direct adapter for in-memory usage: `apps/rest-api/internal/adapters/direct_vector_adapter.go`

### 3. Shared Type Definitions

Located in `apps/rest-api/internal/types/models.go`, these provide common type definitions used across both repository and API layers:

```go
type Embedding struct {
    ID           string                 `json:"id" db:"id"`
    ContextID    string                 `json:"context_id" db:"context_id"`
    ContentIndex int                    `json:"content_index" db:"content_index"`
    Text         string                 `json:"text" db:"text"`
    Embedding    []float32              `json:"embedding" db:"embedding"`
    ModelID      string                 `json:"model_id" db:"model_id"`
    Metadata     map[string]interface{} `json:"metadata" db:"metadata"`
}
```

## Adapter Pattern Implementation

The adapter pattern is used to bridge different interface expectations and enable backwards compatibility during the migration from monolithic to workspace-based structure.

### 1. Interface Adaptation Challenge

The API code expects specific method names and signatures that are different from our repository interfaces:

- API expects: Methods like `CreateAgent`, `ListAgents` with specific types
- Repository uses: Generic methods like `Create`, `List` with different types

### 2. Adapter Implementation

The adapter pattern is implemented in two main places:

#### A. Package Level Adapter (`pkg/repository/vector_bridge.go`)

This adapter enables package-level compatibility:

```go
// embeddingRepositoryAdapter adapts VectorAPIRepository to previous interface
type embeddingRepositoryAdapter struct {
    repo VectorAPIRepository
}

func (a *embeddingRepositoryAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
    return a.repo.DeleteModelEmbeddings(ctx, contextID, modelID)
}
```

#### B. API Level Adapter (`apps/rest-api/internal/adapters/repository_shim.go`)

This adapter translates between API expectations and internal repository implementations:

```go
// PkgVectorAPIAdapter implements the internal repository interface using a pkg repository
type PkgVectorAPIAdapter struct {
    internal repository.VectorAPIRepository
}

// DeleteModelEmbeddings implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
    return a.internal.DeleteModelEmbeddings(ctx, contextID, modelID)
}
```

### 3. Type Conversion

The adapter handles bidirectional type conversion between:
- `models.Vector` (used by API code)
- `repository.Embedding` (used by repository code)

Key conversions include:
- Field name mapping (`tenant_id` ↔ `context_id`, `content` ↔ `text`)
- Metadata handling (extracting `content_index` and `model_id` from metadata)

Example:
```go
// Converting API model to repository model
repoEmbedding := &repository.Embedding{
    ID:           vector.ID,
    ContextID:    vector.TenantID,  // Field name conversion
    ContentIndex: extractContentIndex(vector.Metadata),
    Text:         vector.Content,   // Field name conversion
    Embedding:    vector.Embedding,
    ModelID:      extractModelID(vector.Metadata),
}
```

## Benefits of the Adapter Pattern

1. **Separation of Concerns**: Clean boundary between API and repository layers
2. **Interface Evolution**: Can evolve either interface independently
3. **Simplified Testing**: Mock implementations can focus on the appropriate interface
4. **Avoid Import Cycles**: Prevents circular dependencies between packages
5. **Backward Compatibility**: Maintains compatibility with existing code

## Testing the Adapter Pattern

Tests for the adapter pattern focus on verifying correct delegation and type conversion:

1. **Direct Tests**: Test the adapter methods directly
2. **Integration Tests**: Test the full data flow through the adapter chain
3. **Edge Cases**: Test error handling and special cases

## Future Improvements

1. Consider automating type conversion using reflection for simpler maintenance
2. Implement more comprehensive validation in the adapter layer
3. Add metrics and logging specific to the adapter operations
