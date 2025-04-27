# Vector Search Functionality

The MCP Server now supports vector-based semantic search for context management using the PostgreSQL `pg_vector` extension. This feature enables AI agents to find semantically similar context items based on meaning rather than just keywords.

## Overview

Vector search provides several advantages over traditional text search:

1. **Semantic Understanding**: Find similar concepts even when different words are used
2. **Contextual Relevance**: Better understand the meaning behind queries
3. **Language Agnostic**: Work effectively across multiple languages
4. **Fuzzy Matching**: Handle typos and variations more effectively

## Architecture

The MCP Server implements a hybrid approach to vector functionality:

- **MCP Server Responsibilities**:
  - Storing vector embeddings in the database
  - Providing efficient vector similarity search
  - Maintaining the relationship between embeddings and contexts
  - Handling vector indexing for performance

- **Agent Responsibilities**:
  - Generating embeddings using appropriate models
  - Deciding which context items to embed
  - Interpreting vector search results
  - Determining how to use the retrieved similar contexts

This division of responsibilities allows the MCP Server to focus on efficient storage and retrieval while giving agents the flexibility to use any embedding model that suits their needs.

## Implementation Details

The vector search functionality is implemented using:

1. **PostgreSQL pg_vector Extension**: Provides vector data types and similarity search functions
2. **Vector Repository**: Manages embedding storage and retrieval
3. **API Endpoints**: Allow agents to store and search embeddings
4. **Database Indexing**: Optimized indexing for fast similarity search

## API Endpoints

The MCP Server provides the following endpoints for vector operations:

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
  "limit": 5,
  "threshold": 0.7
}
```

**Response**:
```json
{
  "embeddings": [
    {
      "id": "embedding-456",
      "context_id": "context-123",
      "content_index": 2,
      "text": "The content that was embedded",
      "model_id": "amazon.titan-embed-text-v1",
      "similarity": 0.92
    },
    ...
  ]
}
```

### Get Context Embeddings

```
GET /api/v1/vectors/context/:context_id
```

**Response**:
```json
{
  "embeddings": [
    {
      "id": "embedding-456",
      "context_id": "context-123",
      "content_index": 2,
      "text": "The content that was embedded",
      "model_id": "amazon.titan-embed-text-v1"
    },
    ...
  ]
}
```

### Delete Context Embeddings

```
DELETE /api/v1/vectors/context/:context_id
```

**Response**: HTTP 200 OK

## Example Usage

Here's a typical workflow for using vector search with the MCP Server:

1. Agent creates a context in MCP server
2. Agent communicates with an LLM service (like Amazon Bedrock)
3. Agent generates embeddings for context items using an embedding model
4. Agent stores these embeddings in the MCP server
5. When a new query arrives, agent generates an embedding for the query
6. Agent uses MCP's vector search to find semantically similar context items
7. Agent retrieves the most relevant context items to enhance its response

For a complete example, see `examples/vector_search.go` in the repository.

## Generating Embeddings

The MCP Server does not generate embeddings directly. Instead, agents are responsible for generating embeddings using their preferred models. The example code demonstrates using Amazon Bedrock's Titan Text Embeddings model, but any embedding model can be used as long as it produces a vector of floating-point numbers.

Popular embedding models include:

- Amazon Titan Text Embeddings
- OpenAI Embeddings
- Cohere Embeddings
- Sentence Transformers (local)

## Performance Considerations

Vector search performance depends on several factors:

1. **Vector Size**: Smaller vectors (e.g., 384 dimensions) are more efficient than larger ones (e.g., 1536 dimensions)
2. **Database Indexing**: The pg_vector extension supports various indexing methods (IVF, HNSW, etc.)
3. **Database Configuration**: Properly configured PostgreSQL instances with sufficient memory
4. **Search Parameters**: Using appropriate limits and thresholds for searches

For large-scale deployments, consider performance tuning options in the PostgreSQL configuration.

## Configuration

The vector search functionality requires the PostgreSQL pg_vector extension to be installed. This is automatically handled in the Docker Compose setup.

No additional configuration is required in the MCP Server configuration file.

## Limitations

The current vector search implementation has the following limitations:

1. **Model Agnostic**: The MCP Server does not validate that vectors from the same model are used for comparison
2. **Fixed Distance Metric**: Currently uses cosine similarity only (not Euclidean or other distances)
3. **No Clustering or Preprocessing**: No automatic vector normalization or clustering
4. **Single Database**: No support for dedicated vector databases like Pinecone, Milvus, etc.

These limitations may be addressed in future releases.

## Best Practices

When working with vector search, consider the following best practices:

1. **Consistent Models**: Use the same embedding model for both storing and querying
2. **Normalize Vectors**: Some models require vector normalization before comparison
3. **Appropriate Vector Size**: Balance dimensionality with performance needs
4. **Selective Embedding**: Embed only relevant content to reduce storage requirements
5. **Thoughtful Thresholds**: Adjust similarity thresholds based on your application needs

## Troubleshooting

Common issues and their solutions:

1. **Poor Search Results**:
   - Check that the same embedding model is used for both storing and querying
   - Verify that vectors are normalized if required by the model
   - Adjust similarity thresholds

2. **Slow Performance**:
   - Ensure proper indexing in PostgreSQL
   - Consider reducing vector dimensions
   - Optimize database configuration

3. **Database Errors**:
   - Verify that the pg_vector extension is installed
   - Check PostgreSQL version compatibility