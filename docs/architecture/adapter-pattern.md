# Adapter Pattern Implementation

## Overview

The DevOps MCP project uses the adapter pattern to bridge interface incompatibilities between different layers of the application, particularly during the Go workspace migration. This document describes how adapters are implemented and their role in the system architecture.

## Purpose of Adapters

In the MCP system, adapters serve several critical purposes:

1. **Interface Compatibility**: Bridge between different interface expectations in the API and repository layers
2. **Type Conversion**: Handle bidirectional conversion between different data models
3. **Backward Compatibility**: Allow gradual migration without breaking existing code
4. **Separation of Concerns**: Maintain clean boundaries between system layers

## Adapter Implementation

### Interface Adaptation

The system uses adapters to bridge between different interface expectations:

- **API Layer** expects methods like `CreateAgent`, `ListAgents` with specific types
- **Repository Layer** uses generic methods like `Create`, `List` with different types

```go
// API expects this method signature
func (a *ServerEmbeddingAdapter) StoreEmbedding(ctx context.Context, vector *models.Vector) error {
    // Conversion and delegation logic
    return a.repo.StoreEmbedding(ctx, repoEmbedding)
}
```

### Type Conversion

Adapters handle bidirectional conversion between:

- `models.Vector` (used by API code)
- `repository.Embedding` (used by repository code)

Key conversions include:

- Field name mapping (`tenant_id` ↔ `context_id`, `content` ↔ `text`)
- Metadata handling (extracting `content_index` and `model_id` from metadata)
- Proper JSON field capitalization (API uses capitalized field names)
- Similarity score handling within metadata

Example conversion:

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

## Mock Implementation for Testing

When testing adapter implementations, we use a flexible approach to handle type aliases and ensure proper test coverage:

```go
// Use mock.Anything for parameter matching to avoid issues with type aliases
mockRepo.On("StoreEmbedding", mock.Anything, mock.Anything).Return(nil)

// When comparing JSON responses, ensure proper field capitalization
var resp struct {
    Embeddings []struct {
        ID string `json:"ID"` // Note: Uses capitalized "ID" not lowercase "id"
        ModelID string `json:"ModelID"` // Uses "ModelID" not "model_id"
        Metadata map[string]interface{} `json:"Metadata"`
    } `json:"embeddings"`
}
```

This flexible mocking approach prevents type compatibility issues when working with the system's type aliases.

## Benefits of the Adapter Pattern

1. **Separation of Concerns**: Clean boundary between API and repository layers
2. **Interface Evolution**: Can evolve either interface independently
3. **Simplified Testing**: Mock implementations can focus on the appropriate interface
4. **Avoid Import Cycles**: Prevents circular dependencies between packages
5. **Backward Compatibility**: Maintains compatibility with existing code

## Common Gotchas and Solutions

1. **JSON Field Capitalization**: Ensure field names match exactly in JSON struct tags (e.g., `ID` vs `id`)
2. **Type Alias Compatibility**: Use `mock.Anything` in tests to avoid type alias comparison issues
3. **Complete Delegation**: Ensure all methods in the target interface are implemented by the adapter
4. **Proper Error Handling**: Adapters should preserve and wrap errors appropriately

## Example Implementations

- `ServerEmbeddingAdapter` in `apps/rest-api/internal/adapters/embedding_adapter.go`
- `PkgVectorAPIAdapter` in `apps/rest-api/internal/adapters/repository_adapter.go`
