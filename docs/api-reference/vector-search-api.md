# Vector Search API Reference

This document provides detailed information about the Vector Search API endpoints available in DevOps MCP.

## API Overview

The Vector Search API enables storing and searching for vector embeddings using semantic similarity. It uses PostgreSQL with pgvector extension for fast and efficient vector operations.

### Base URL

```
http://localhost:8081/api/v1
```

## Authentication

All requests require an API key provided in the `Authorization` header:

```
Authorization: Bearer your-api-key
```

## Endpoints

### Store Embedding

Stores a new vector embedding in the system.

**Endpoint:** `POST /vectors`

**Request Body:**
```json
{
  "id": "emb-123",
  "context_id": "context-123",
  "content_index": 1,
  "text": "Example text content",
  "embedding": [0.1, 0.2, 0.3, ...],
  "model_id": "text-embedding-ada-002",
  "metadata": {
    "source": "document1",
    "page": 5
  }
}
```

**Response:**
```json
{
  "status": "success",
  "message": "Embedding stored successfully"
}
```

**Status Codes:**
- `200 OK`: Embedding stored successfully
- `400 Bad Request`: Invalid request body
- `500 Internal Server Error`: Server error

### Search Embeddings

Searches for embeddings similar to the provided query vector.

**Endpoint:** `POST /vectors/search`

**Request Body:**
```json
{
  "context_id": "context-123",
  "model_id": "text-embedding-ada-002",
  "query_embedding": [0.1, 0.2, 0.3, ...],
  "limit": 5,
  "similarity_threshold": 0.7
}
```

**Response:**
```json
{
  "embeddings": [
    {
      "ID": "emb-1",
      "ContextID": "context-123",
      "ContentIndex": 1,
      "Text": "Example content 1",
      "Embedding": null,
      "ModelID": "text-embedding-ada-002",
      "Metadata": {
        "similarity": 0.95,
        "source": "document1"
      }
    },
    {
      "ID": "emb-2",
      "ContextID": "context-123",
      "ContentIndex": 2,
      "Text": "Example content 2",
      "Embedding": null,
      "ModelID": "text-embedding-ada-002",
      "Metadata": {
        "similarity": 0.85,
        "source": "document1"
      }
    }
  ]
}
```

**Important Notes:**
- The `Embedding` field in responses is typically `null` to reduce response size
- Similarity scores are included in the `Metadata` as a `similarity` field
- Response field names use capitalized format (e.g., `ID`, `ContextID`, `ModelID`)

**Status Codes:**
- `200 OK`: Search completed successfully
- `400 Bad Request`: Invalid request body
- `500 Internal Server Error`: Server error

### Get Context Embeddings

Retrieves all embeddings for a specific context.

**Endpoint:** `GET /vectors/context/:context_id`

**Parameters:**
- `context_id` (path): ID of the context to retrieve embeddings for

**Response:**
```json
{
  "embeddings": [
    {
      "ID": "emb-1",
      "ContextID": "context-123",
      "ContentIndex": 1,
      "Text": "Example content 1",
      "Embedding": null,
      "ModelID": "text-embedding-ada-002",
      "Metadata": {
        "source": "document1"
      }
    }
  ]
}
```

**Status Codes:**
- `200 OK`: Retrieved successfully
- `404 Not Found`: Context not found
- `500 Internal Server Error`: Server error

### Delete Context Embeddings

Deletes all embeddings for a specific context.

**Endpoint:** `DELETE /vectors/context/:context_id`

**Parameters:**
- `context_id` (path): ID of the context to delete embeddings for

**Response:**
```json
{
  "status": "success",
  "message": "Embeddings deleted successfully"
}
```

**Status Codes:**
- `200 OK`: Deleted successfully
- `404 Not Found`: Context not found
- `500 Internal Server Error`: Server error

### Get Model Embeddings

Retrieves embeddings for a specific model within a context.

**Endpoint:** `GET /vectors/context/:context_id/model/:model_id`

**Parameters:**
- `context_id` (path): Context ID
- `model_id` (path): Model ID

**Response:**
Same format as Get Context Embeddings, filtered by model ID.

### Delete Model Embeddings

Deletes embeddings for a specific model within a context.

**Endpoint:** `DELETE /vectors/context/:context_id/model/:model_id`

**Parameters:**
- `context_id` (path): Context ID
- `model_id` (path): Model ID

**Response:**
Same format as Delete Context Embeddings.

## Data Models

### Vector Embedding

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique identifier for the embedding |
| ContextID | string | Context the embedding belongs to |
| ContentIndex | integer | Position/index within the context |
| Text | string | Text content represented by the embedding |
| Embedding | float32[] | Vector representation of the content |
| ModelID | string | ID of the model that created the embedding |
| Metadata | object | Additional data associated with the embedding |

## Error Handling

Errors are returned in a standard format:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "Invalid embedding vector format"
  }
}
```

## Implementation Notes

### JSON Field Capitalization

When processing response JSON, be aware that field names use capitalized format (e.g., `ID`, `ModelID`) rather than lowercase or snake_case. This is important when deserializing responses.

### Similarity Score

The similarity score is included in the `Metadata` field of each embedding in search results. Higher scores indicate greater similarity to the query vector.

### Vector Dimensions

The system supports various vector dimensions, but all vectors within the same model ID should have consistent dimensions.
