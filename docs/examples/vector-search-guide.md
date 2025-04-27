# Vector Search Guide

This guide provides comprehensive information on using the MCP Server's vector search capabilities for semantic search within conversation contexts.

## Table of Contents

- [Introduction](#introduction)
- [How Vector Search Works](#how-vector-search-works)
- [Multi-Model Support](#multi-model-support)
- [API Endpoints](#api-endpoints)
- [Implementation Examples](#implementation-examples)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Introduction

MCP Server includes powerful vector search capabilities that allow AI agents to find semantically similar content within conversation contexts. This enables more intelligent retrieval of relevant information and enhances the AI's ability to maintain context over long conversations.

Key features include:

- Storage for vector embeddings from multiple embedding models
- Efficient similarity search using PostgreSQL's pgvector extension
- Configurable similarity thresholds and result limits
- Multiple models support (OpenAI, Anthropic, custom embedding models)

## How Vector Search Works

Vector search works by representing text as high-dimensional vectors (embeddings) and finding vectors that are close to each other in the vector space. The process involves:

1. **Embedding Generation**: Convert text to vector embeddings (typically done by the AI agent)
2. **Embedding Storage**: Store the embeddings in the MCP Server
3. **Similarity Search**: Find stored embeddings that are similar to a query embedding
4. **Content Retrieval**: Retrieve the original text associated with the similar embeddings

MCP Server uses PostgreSQL's pgvector extension to efficiently store and search vector embeddings.

## Multi-Model Support

MCP Server supports embeddings from different models and with different dimensions:

- OpenAI embeddings (1536 dimensions)
- Anthropic embeddings (various dimensions)
- Custom model embeddings

The vector storage system tracks the model ID and dimensions for each embedding, allowing you to:

- Store embeddings from different models for the same context
- Search only within embeddings from a specific model
- Compare compatibility between embeddings

## API Endpoints

MCP Server provides the following endpoints for vector operations:

### Store an Embedding

```http
POST /api/v1/vectors/store
```

Request body:
```json
{
  "context_id": "ctx_123",
  "content_index": 0,
  "text": "Hello AI assistant!",
  "embedding": [0.1, 0.2, 0.3, ...],
  "model_id": "text-embedding-ada-002"
}
```

### Search Embeddings

```http
POST /api/v1/vectors/search
```

Request body:
```json
{
  "context_id": "ctx_123",
  "query_embedding": [0.1, 0.2, 0.3, ...],
  "limit": 5,
  "model_id": "text-embedding-ada-002",
  "similarity_threshold": 0.7
}
```

### Get Context Embeddings

```http
GET /api/v1/vectors/context/{context_id}
```

### Delete Context Embeddings

```http
DELETE /api/v1/vectors/context/{context_id}
```

### Get Supported Models

```http
GET /api/v1/vectors/models
```

### Get Model Embeddings

```http
GET /api/v1/vectors/context/{context_id}/model/{model_id}
```

### Delete Model Embeddings

```http
DELETE /api/v1/vectors/context/{context_id}/model/{model_id}
```

For complete details on request/response formats, see the [API Reference](../api-reference.md).

## Implementation Examples

### Storing Embeddings

```python
import requests
import numpy as np
import openai

# Generate an embedding using OpenAI
response = openai.embeddings.create(
    model="text-embedding-ada-002",
    input="Hello, how can I help you with GitHub today?"
)
embedding = response.data[0].embedding

# Store the embedding in MCP Server
response = requests.post(
    "http://localhost:8080/api/v1/vectors/store",
    headers={
        "Content-Type": "application/json",
        "Authorization": "Bearer YOUR_API_KEY"
    },
    json={
        "context_id": "ctx_123",
        "content_index": 0,
        "text": "Hello, how can I help you with GitHub today?",
        "embedding": embedding,
        "model_id": "text-embedding-ada-002"
    }
)

print(f"Response: {response.status_code}")
print(response.json())
```

### Searching Embeddings

```python
import requests
import numpy as np
import openai

# Generate a query embedding
response = openai.embeddings.create(
    model="text-embedding-ada-002",
    input="I need help with GitHub"
)
query_embedding = response.data[0].embedding

# Search for similar content
response = requests.post(
    "http://localhost:8080/api/v1/vectors/search",
    headers={
        "Content-Type": "application/json",
        "Authorization": "Bearer YOUR_API_KEY"
    },
    json={
        "context_id": "ctx_123",
        "query_embedding": query_embedding,
        "limit": 5,
        "model_id": "text-embedding-ada-002",
        "similarity_threshold": 0.7
    }
)

# Process search results
results = response.json()
for item in results.get("embeddings", []):
    print(f"Text: {item['text']}")
    print(f"Similarity: {item['similarity']}")
    print("---")
```

## Best Practices

### Embedding Generation

1. **Use consistent models** - Use the same embedding model for both storage and search
2. **Store the model ID** - Always include the model ID when storing embeddings
3. **Normalize embeddings** - Some models require normalization for optimal similarity search

### Search Optimization

1. **Set appropriate similarity thresholds** - Typically between 0.7 and 0.85 for good results
2. **Limit result count** - Limit search results to a reasonable number (5-10)
3. **Index key concepts** - For large contexts, focus on embedding important information
4. **Combine with text search** - Use both vector and text search for comprehensive results

### Performance Considerations

1. **Batch embedding operations** - Generate and store embeddings in batches when possible
2. **Monitor embedding count** - Large numbers of embeddings can impact performance
3. **Use appropriate vector dimensions** - Higher dimensions aren't always better (more precise but slower)
4. **Clean up unused embeddings** - Delete embeddings for expired or deleted contexts

## Troubleshooting

### Common Issues

1. **Low similarity scores** - Check that you're using the same embedding model for storage and search
2. **Missing results** - Verify that the similarity threshold isn't too high
3. **Slow search performance** - Consider optimizing indexes or limiting the search scope
4. **Inconsistent results** - Ensure consistent preprocessing of text before embedding
5. **Dimension mismatch errors** - Verify that the query embedding has the same dimensions as the stored embeddings

For more troubleshooting help, see the [Troubleshooting Guide](../troubleshooting-guide.md).
