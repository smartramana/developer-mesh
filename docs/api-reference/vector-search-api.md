# Vector Search API Reference

Comprehensive API documentation for DevOps MCP's production-ready vector search capabilities.

## Overview

The Vector Search API provides high-performance semantic search with:

- **🚀 Multi-Model Support**: OpenAI, Anthropic, Voyage, and open-source models
- **⚡ Sub-100ms Latency**: Optimized pgvector with HNSW indexing
- **🎯 99.9% Uptime**: Production-tested with automatic failover
- **🔒 Enterprise Security**: OAuth 2.0, API keys, and row-level security
- **📊 Advanced Features**: Hybrid search, metadata filtering, batch operations

## Base URLs

```yaml
Production: https://api.devops-mcp.com/v1
Staging:    https://staging-api.devops-mcp.com/v1
Local:      http://localhost:8081/api/v1
```

## Authentication

### API Key Authentication

```bash
curl -H "Authorization: Bearer $API_KEY" \
     https://api.devops-mcp.com/v1/vectors
```

### OAuth 2.0

```bash
curl -X POST https://auth.devops-mcp.com/oauth/token \
  -d "grant_type=client_credentials" \
  -d "client_id=$CLIENT_ID" \
  -d "client_secret=$CLIENT_SECRET"
```

### Request Signing (HMAC)

```bash
# Generate signature
SIGNATURE=$(echo -n "$REQUEST_BODY" | \
  openssl dgst -sha256 -hmac "$SECRET_KEY" -binary | \
  base64)

curl -H "X-Signature: $SIGNATURE" \
     -H "X-Timestamp: $(date +%s)" \
     https://api.devops-mcp.com/v1/vectors
```

## Core Endpoints

### 1. Create Embedding

Store a vector embedding with automatic indexing.

**Endpoint:** `POST /vectors`

**Headers:**
```http
Authorization: Bearer <token>
Content-Type: application/json
X-Request-ID: <uuid>  # Optional, for tracing
X-Idempotency-Key: <key>  # Optional, for idempotency
```

**Request Body:**
```json
{
  "id": "emb-123",  // Optional, auto-generated if not provided
  "context_id": "ctx-456",
  "content_index": 1,
  "text": "The quick brown fox jumps over the lazy dog",
  "embedding": [0.1, 0.2, 0.3, ...],  // Optional if using auto-embedding
  "model_id": "text-embedding-3-small",
  "metadata": {
    "source": "document.pdf",
    "page": 5,
    "language": "en",
    "timestamp": "2024-01-20T10:30:00Z",
    "custom_field": "any_value"
  }
}
```

**Response (201 Created):**
```json
{
  "id": "emb-123",
  "context_id": "ctx-456",
  "created_at": "2024-01-20T10:30:00Z",
  "model_id": "text-embedding-3-small",
  "dimensions": 1536,
  "metadata": {
    "source": "document.pdf",
    "page": 5
  },
  "_links": {
    "self": "/vectors/emb-123",
    "context": "/contexts/ctx-456",
    "search": "/vectors/search?context_id=ctx-456"
  }
}
```

**Error Response (400 Bad Request):**
```json
{
  "error": {
    "code": "INVALID_DIMENSIONS",
    "message": "Embedding dimensions (1024) do not match model requirements (1536)",
    "details": {
      "provided": 1024,
      "expected": 1536,
      "model_id": "text-embedding-3-small"
    }
  },
  "request_id": "req-789"
}
```

**Status Codes:**
- `201 Created`: Embedding created successfully
- `400 Bad Request`: Invalid input (dimensions mismatch, missing fields)
- `409 Conflict`: Duplicate ID (when using idempotency key)
- `413 Payload Too Large`: Text or metadata exceeds limits
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

### 2. Search Embeddings

Perform semantic search with advanced filtering and ranking.

**Endpoint:** `POST /vectors/search`

**Request Body:**
```json
{
  // Required fields
  "query": "How to implement OAuth 2.0?",  // Text query (auto-embedded)
  // OR
  "query_embedding": [0.1, 0.2, 0.3, ...],  // Pre-computed embedding
  
  // Filtering options
  "context_ids": ["ctx-123", "ctx-456"],  // Search specific contexts
  "model_id": "text-embedding-3-small",   // Model consistency
  
  // Search parameters
  "limit": 10,                            // Max results (default: 10, max: 100)
  "offset": 0,                            // Pagination offset
  "similarity_threshold": 0.75,           // Min similarity (0.0-1.0)
  
  // Advanced options
  "search_mode": "hybrid",                // "semantic", "hybrid", "keyword"
  "rerank": true,                         // Apply ML reranking
  "include_embeddings": false,            // Include vectors in response
  
  // Metadata filters
  "filters": {
    "source": {"$in": ["docs", "code"]},
    "language": "en",
    "timestamp": {"$gte": "2024-01-01"},
    "custom.tag": {"$contains": "oauth"}
  },
  
  // Boost certain results
  "boost": {
    "source": {
      "docs": 1.5,      // Boost docs by 50%
      "code": 1.2       // Boost code by 20%
    }
  }
}
```

**Response (200 OK):**
```json
{
  "results": [
    {
      "id": "emb-789",
      "score": 0.94,  // Combined score (similarity + boosts)
      "similarity": 0.89,  // Raw cosine similarity
      "text": "OAuth 2.0 is an authorization framework that enables...",
      "context_id": "ctx-123",
      "content_index": 42,
      "metadata": {
        "source": "docs",
        "title": "Authentication Guide",
        "url": "https://docs.example.com/auth#oauth2"
      },
      "highlights": [
        {
          "field": "text",
          "snippet": "...implement <em>OAuth 2.0</em> authorization...",
          "positions": [[10, 19]]
        }
      ],
      "explanation": {
        "base_score": 0.89,
        "boost_applied": 1.5,
        "final_score": 0.94,
        "matched_filters": ["source", "custom.tag"]
      }
    }
  ],
  "metadata": {
    "total_results": 156,
    "returned": 10,
    "query_time_ms": 45,
    "index_used": "hnsw_cosine",
    "model_id": "text-embedding-3-small"
  },
  "facets": {
    "source": {
      "docs": 89,
      "code": 45,
      "issues": 22
    },
    "language": {
      "en": 134,
      "es": 12,
      "fr": 10
    }
  },
  "_links": {
    "next": "/vectors/search?offset=10&limit=10",
    "self": "/vectors/search"
  }
}
```

**Search Modes:**
- `semantic`: Pure vector similarity search
- `hybrid`: Combines vector search with keyword matching
- `keyword`: Traditional text search with vector boost

**Filter Operators:**
- `$eq`: Exact match
- `$ne`: Not equal
- `$in`: In array
- `$nin`: Not in array
- `$gt`, `$gte`, `$lt`, `$lte`: Numeric comparisons
- `$contains`: Substring match
- `$regex`: Regular expression
- `$exists`: Field exists

### 3. Batch Operations

Efficiently process multiple embeddings in a single request.

**Endpoint:** `POST /vectors/batch`

**Request Body:**
```json
{
  "operations": [
    {
      "operation": "create",
      "data": {
        "context_id": "ctx-123",
        "text": "First document chunk",
        "metadata": {"chunk": 1}
      }
    },
    {
      "operation": "update",
      "id": "emb-456",
      "data": {
        "metadata": {"reviewed": true}
      }
    },
    {
      "operation": "delete",
      "id": "emb-789"
    }
  ],
  "options": {
    "atomic": true,  // All or nothing
    "continue_on_error": false,
    "parallel": true
  }
}
```

**Response (207 Multi-Status):**
```json
{
  "results": [
    {
      "index": 0,
      "status": 201,
      "data": {"id": "emb-new-1", "created": true}
    },
    {
      "index": 1,
      "status": 200,
      "data": {"id": "emb-456", "updated": true}
    },
    {
      "index": 2,
      "status": 404,
      "error": {"code": "NOT_FOUND", "message": "Embedding emb-789 not found"}
    }
  ],
  "summary": {
    "total": 3,
    "successful": 2,
    "failed": 1
  }
}
```

### 4. Get Embedding Details

Retrieve a specific embedding with full details.

**Endpoint:** `GET /vectors/:id`

**Query Parameters:**
- `include_embedding`: Return the actual vector (default: false)
- `include_context`: Include context details (default: false)

**Response (200 OK):**
```json
{
  "id": "emb-123",
  "context_id": "ctx-456",
  "content_index": 1,
  "text": "Example content",
  "embedding": [0.1, 0.2, ...],  // Only if include_embedding=true
  "model_id": "text-embedding-3-small",
  "dimensions": 1536,
  "metadata": {
    "source": "document.pdf",
    "page": 5
  },
  "created_at": "2024-01-20T10:30:00Z",
  "updated_at": "2024-01-20T10:30:00Z",
  "context": {  // Only if include_context=true
    "id": "ctx-456",
    "name": "Project Documentation",
    "created_at": "2024-01-15T08:00:00Z"
  },
  "_links": {
    "self": "/vectors/emb-123",
    "context": "/contexts/ctx-456",
    "similar": "/vectors/emb-123/similar"
  }
}
```

### 5. Update Embedding

Update metadata or re-embed content.

**Endpoint:** `PATCH /vectors/:id`

**Request Body:**
```json
{
  "text": "Updated content text",  // Optional, triggers re-embedding
  "metadata": {
    "reviewed": true,
    "reviewer": "john.doe",
    "review_date": "2024-01-21"
  },
  "merge_metadata": true  // Merge with existing metadata (default: false)
}
```

### 6. Delete Operations

#### Delete Single Embedding
**Endpoint:** `DELETE /vectors/:id`

#### Delete by Context
**Endpoint:** `DELETE /vectors?context_id=ctx-123`

#### Delete by Query
**Endpoint:** `POST /vectors/delete`
```json
{
  "filters": {
    "context_id": "ctx-123",
    "metadata.source": "old_docs",
    "created_at": {"$lt": "2024-01-01"}
  },
  "dry_run": true  // Preview what would be deleted
}
```

### 7. Find Similar

Find embeddings similar to an existing one.

**Endpoint:** `GET /vectors/:id/similar`

**Query Parameters:**
- `limit`: Number of results (default: 10)
- `threshold`: Minimum similarity (default: 0.7)
- `exclude_same_context`: Exclude results from same context

**Response:**
```json
{
  "source": {
    "id": "emb-123",
    "text": "OAuth 2.0 implementation guide"
  },
  "similar": [
    {
      "id": "emb-456",
      "similarity": 0.92,
      "text": "OpenID Connect authentication tutorial",
      "context_id": "ctx-789",
      "metadata": {
        "source": "tutorials"
      }
    }
  ]
}
```

## Advanced Features

### Auto-Embedding

Automatically generate embeddings from text.

**Endpoint:** `POST /vectors/auto-embed`

```json
{
  "texts": [
    "First chunk of text",
    "Second chunk of text"
  ],
  "context_id": "ctx-123",
  "model_id": "text-embedding-3-small",  // Optional, uses default
  "chunking": {
    "enabled": true,
    "max_chunk_size": 1000,
    "overlap": 200,
    "language": "en"
  }
}
```

### Hybrid Search

Combine vector and keyword search.

```json
{
  "query": "OAuth 2.0 implementation",
  "search_mode": "hybrid",
  "vector_weight": 0.7,  // 70% vector, 30% keyword
  "keyword_fields": ["text", "metadata.title"],
  "fuzziness": "AUTO"
}
```

### Cross-Model Search

Search across different embedding models.

```json
{
  "query": "authentication methods",
  "model_ids": [
    "text-embedding-3-small",
    "text-embedding-3-large",
    "voyage-2"
  ],
  "normalize_scores": true
}
```

## Rate Limits

| Tier | Requests/Second | Requests/Day | Batch Size |
|------|----------------|--------------|------------|
| Free | 10 | 10,000 | 10 |
| Starter | 50 | 100,000 | 100 |
| Pro | 200 | 1,000,000 | 500 |
| Enterprise | Custom | Unlimited | 1000 |

**Rate Limit Headers:**
```http
X-RateLimit-Limit: 50
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1642521600
Retry-After: 60
```

## Error Codes

| Code | Description | Resolution |
|------|-------------|------------|
| `INVALID_DIMENSIONS` | Vector dimensions mismatch | Check model requirements |
| `CONTEXT_NOT_FOUND` | Context ID doesn't exist | Verify context ID |
| `MODEL_NOT_SUPPORTED` | Unknown model ID | Use supported models |
| `RATE_LIMIT_EXCEEDED` | Too many requests | Wait or upgrade tier |
| `PAYLOAD_TOO_LARGE` | Request exceeds size limit | Reduce batch size |
| `INVALID_FILTER` | Malformed filter syntax | Check filter documentation |
| `TIMEOUT` | Request took too long | Reduce query complexity |

## SDK Examples

### Python
```python
from devops_mcp import VectorClient

client = VectorClient(
    api_key="your-api-key",
    base_url="https://api.devops-mcp.com/v1"
)

# Create embedding
result = await client.create_embedding(
    text="OAuth 2.0 implementation guide",
    context_id="ctx-123",
    metadata={"source": "docs"}
)

# Search
results = await client.search(
    query="How to implement OAuth?",
    context_ids=["ctx-123"],
    limit=5
)
```

### Go
```go
import "github.com/S-Corkum/devops-mcp/pkg/client"

client := client.NewVectorClient(
    client.WithAPIKey("your-api-key"),
    client.WithBaseURL("https://api.devops-mcp.com/v1"),
)

// Create embedding
result, err := client.CreateEmbedding(ctx, &CreateEmbeddingRequest{
    Text:      "OAuth 2.0 implementation guide",
    ContextID: "ctx-123",
    Metadata:  map[string]interface{}{"source": "docs"},
})

// Search
results, err := client.Search(ctx, &SearchRequest{
    Query:      "How to implement OAuth?",
    ContextIDs: []string{"ctx-123"},
    Limit:      5,
})
```

### JavaScript/TypeScript
```typescript
import { VectorClient } from '@devops-mcp/client';

const client = new VectorClient({
  apiKey: 'your-api-key',
  baseURL: 'https://api.devops-mcp.com/v1'
});

// Create embedding
const result = await client.createEmbedding({
  text: 'OAuth 2.0 implementation guide',
  contextId: 'ctx-123',
  metadata: { source: 'docs' }
});

// Search
const results = await client.search({
  query: 'How to implement OAuth?',
  contextIds: ['ctx-123'],
  limit: 5
});
```

## Webhooks

Receive real-time notifications for vector events.

### Configure Webhook
```json
POST /webhooks
{
  "url": "https://your-app.com/webhooks/vectors",
  "events": ["vector.created", "vector.deleted", "search.performed"],
  "secret": "your-webhook-secret"
}
```

### Webhook Payload
```json
{
  "event": "vector.created",
  "timestamp": "2024-01-20T10:30:00Z",
  "data": {
    "id": "emb-123",
    "context_id": "ctx-456",
    "model_id": "text-embedding-3-small"
  },
  "signature": "sha256=..."
}
```

## Best Practices

1. **Batch Operations**: Use batch endpoints for bulk operations
2. **Caching**: Cache frequently accessed embeddings
3. **Model Consistency**: Use same model within a context
4. **Metadata Design**: Plan metadata schema for efficient filtering
5. **Error Handling**: Implement exponential backoff for retries
6. **Security**: Rotate API keys regularly
7. **Monitoring**: Track usage via provided metrics

---

*For support, visit [docs.devops-mcp.com](https://docs.devops-mcp.com) or contact support@devops-mcp.com*
