# Multi-Model Vector Support

This document describes the enhanced support for multiple LLM models with different embedding vector dimensions in the MCP Server.

## Overview

The MCP Server now supports storing and searching vector embeddings from multiple LLM models with different dimensions. This is achieved through:

1. Dynamic dimension tracking in the database
2. Model-specific vector storage and indexing
3. Optimized connection pooling for vector operations
4. Vector normalization and preprocessing

These enhancements ensure that:
- Embeddings from different models (e.g., OpenAI, Anthropic, Cohere) can be stored in the same database
- Searches only compare embeddings from the same model and with the same dimensions
- Vector operations are performed efficiently with dedicated connection pools

## Architecture

The enhanced vector support architecture consists of:

1. **Database Schema**: Optimized for storing embeddings with different dimensions
2. **Embedding Repository**: Handles storing and retrieving embeddings
3. **Vector Database**: Specialized database operations for vector data
4. **Vector API**: HTTP endpoints for vector operations
5. **Vector Utilities**: Normalization and similarity calculations

### Database Schema

The database schema has been updated to:
- Store the vector dimensions and model ID with each embedding
- Create specialized indices for common dimension sizes (384, 768, 1536)
- Implement model-aware similarity search

### Connection Pooling

The system now supports dedicated connection pools for vector operations, which:
- Isolates vector operations from regular CRUD operations
- Optimizes connection settings for vector searches
- Improves overall system performance

## API Endpoints

The following API endpoints are available for vector operations:

### Store Embedding

```
POST /api/v1/vectors/store
```

**Request Body**:
```json
{
  "context_id": "context-123",
  "content_index": 2,
  "text": "The content that was embedded",
  "embedding": [0.1, 0.2, 0.3, ...],
  "model_id": "amazon.titan-embed-text-v1"
}
```

**Response**:
```json
{
  "id": "embedding-456",
  "context_id": "context-123",
  "content_index": 2,
  "text": "The content that was embedded",
  "model_id": "amazon.titan-embed-text-v1"
}
```

### Search Embeddings

```
POST /api/v1/vectors/search
```

**Request Body**:
```json
{
  "context_id": "context-123",
  "query_embedding": [0.1, 0.2, 0.3, ...],
  "model_id": "amazon.titan-embed-text-v1",
  "limit": 5,
  "threshold": 0.7
}
```

**Response**:
```json
{
  "context_id": "context-123",
  "model_id": "amazon.titan-embed-text-v1",
  "results": [
    {
      "id": "embedding-456",
      "context_id": "context-123",
      "content_index": 2,
      "text": "The content that was embedded",
      "model_id": "amazon.titan-embed-text-v1",
      "similarity": 0.92
    }
  ]
}
```

### Get Embeddings by Model

```
GET /api/v1/vectors/context/:contextID/model/:modelID
```

**Response**:
```json
{
  "context_id": "context-123",
  "model_id": "amazon.titan-embed-text-v1",
  "embeddings": [
    {
      "id": "embedding-456",
      "context_id": "context-123",
      "content_index": 2,
      "text": "The content that was embedded",
      "model_id": "amazon.titan-embed-text-v1"
    }
  ]
}
```

### Delete Model Embeddings

```
DELETE /api/v1/vectors/context/:contextID/model/:modelID
```

**Response**: HTTP 200 OK

### Get Supported Models

```
GET /api/v1/vectors/models
```

**Response**:
```json
{
  "models": [
    "amazon.titan-embed-text-v1",
    "anthropic.claude-2-1",
    "openai.text-embedding-ada-002"
  ]
}
```

## Configuration

The following configuration options are available for vector operations:

```yaml
database:
  vector:
    enabled: true
    index_type: "ivfflat"  # Options: "ivfflat", "hnsw"
    lists: 100  # Number of lists for ivfflat index
    probes: 10  # Number of probes for search
    
    # Separate connection pool for vector operations
    pool:
      enabled: true
      max_open_conns: 25
      max_idle_conns: 5
      conn_max_lifetime: 10m
```

## Usage Examples

### Storing Embeddings from Different Models

```go
// Store OpenAI embedding
openaiEmbedding := &repository.Embedding{
    ContextID:    "context-123",
    ContentIndex: 0,
    Text:         "This is a test for OpenAI",
    Embedding:    openaiEmbeddingVector, // 1536 dimensions
    ModelID:      "openai.text-embedding-ada-002",
}
embeddingRepo.StoreEmbedding(ctx, openaiEmbedding)

// Store Anthropic embedding
anthropicEmbedding := &repository.Embedding{
    ContextID:    "context-123",
    ContentIndex: 1,
    Text:         "This is a test for Anthropic",
    Embedding:    anthropicEmbeddingVector, // 768 dimensions
    ModelID:      "anthropic.claude-2-1",
}
embeddingRepo.StoreEmbedding(ctx, anthropicEmbedding)
```

### Searching with Model-Specific Queries

```go
// Search using OpenAI embeddings
openaiResults, err := embeddingRepo.SearchEmbeddings(
    ctx,
    openaiQueryVector,
    "context-123",
    "openai.text-embedding-ada-002",
    5,
    0.7,
)

// Search using Anthropic embeddings
anthropicResults, err := embeddingRepo.SearchEmbeddings(
    ctx,
    anthropicQueryVector,
    "context-123",
    "anthropic.claude-2-1",
    5,
    0.7,
)
```

## Best Practices

When working with multiple embedding models:

1. **Consistent Model Usage**: Always use the same model for embedding and searching within a specific application context
2. **Vector Normalization**: Normalize vectors based on the similarity metric used by your model
3. **Database Indices**: Create appropriate indices for the vector dimensions you use
4. **Connection Pool Tuning**: Adjust connection pool settings based on your workload
5. **Model Tracking**: Track which models are used for which contexts to avoid confusion

## Troubleshooting

### Common Issues

1. **Mismatched Dimensions**: Ensure the query vector has the same dimensions as the stored vectors for the specified model
2. **Incorrect Model ID**: Verify that the model ID matches exactly between storage and search operations
3. **Performance Issues**: Check connection pool settings and database indices

### Checking Database Status

You can check the status of your vector database using the following SQL:

```sql
-- Check available models
SELECT DISTINCT model_id FROM mcp.embeddings;

-- Check dimensions by model
SELECT model_id, vector_dimensions, COUNT(*) 
FROM mcp.embeddings 
GROUP BY model_id, vector_dimensions;

-- Check indices
SELECT indexname, indexdef 
FROM pg_indexes 
WHERE tablename = 'embeddings';
```

## References

1. PostgreSQL pgvector documentation: [https://github.com/pgvector/pgvector](https://github.com/pgvector/pgvector)
2. Vector similarity measures: [https://en.wikipedia.org/wiki/Cosine_similarity](https://en.wikipedia.org/wiki/Cosine_similarity)
