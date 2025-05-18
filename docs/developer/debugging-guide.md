# Debugging Guide

This guide provides tips and strategies for debugging common issues in the DevOps MCP project.

## Setup for Debugging

### Environment Setup

1. **Local Docker Environment**:
   ```bash
   # Start with debug logs enabled
   DEBUG=true make dev-setup
   ```

2. **Viewing Logs**:
   ```bash
   # View logs from all services
   make docker-compose-logs
   
   # View logs from a specific service
   make docker-compose-logs service=rest-api
   ```

3. **Database Access**:
   ```bash
   # Access the PostgreSQL database
   make db-shell
   
   # Check vector tables
   make psql-vector-tables
   ```

## Common Issues and Solutions

### 1. Adapter Pattern Issues

#### Symptoms:
- Type mismatch errors
- Method not implemented errors
- "Interface doesn't match expected signature" errors

#### Debugging Steps:
1. Check if all interface methods are properly implemented
2. Verify that type conversions are correct
3. Examine JSON field capitalization (common source of errors)

#### Solution Example:
```go
// INCORRECT - lowercase JSON field names
var resp struct {
    Embeddings []struct {
        ID string `json:"id"` // Wrong: uses lowercase
    } `json:"embeddings"`
}

// CORRECT - capitalized JSON field names
var resp struct {
    Embeddings []struct {
        ID string `json:"ID"` // Correct: uses proper capitalization
    } `json:"embeddings"`
}
```

### 2. Mock Testing Issues

#### Symptoms:
- Test failures with "argument has wrong type" errors
- Interface implementation mismatches in tests

#### Debugging Steps:
1. Use `mock.Anything` for type alias parameters:
   ```go
   // More flexible approach that works with type aliases
   mockRepo.On("SearchEmbeddings", mock.Anything, mock.Anything, mock.Anything,
       mock.Anything, mock.Anything, mock.Anything).Return(mockResults, nil)
   ```

2. Check if mock expectations match actual method calls

### 3. Vector Search Issues

#### Symptoms:
- Missing similarity scores in search results
- Empty search results despite existing data

#### Debugging Steps:
1. Verify vector dimensions match between query and stored embeddings
2. Check if metadata contains expected fields
3. Ensure similarity scores are properly included in response metadata
4. Inspect SQL queries with PostgreSQL logs enabled

#### Solution:
```go
// Use the vector_api_repository directly to test a search
results, err := repo.SearchEmbeddings(
    context.Background(),
    queryVector,
    contextID,
    modelID,
    10,  // limit
    0.7, // similarity threshold
)

// Check similarity values in results
for _, embed := range results {
    fmt.Printf("ID: %s, Similarity: %v\n", 
        embed.ID, embed.Metadata["similarity"])
}
```

### 4. Docker Compose Issues

#### Symptoms:
- Services fail to start
- Connection refused errors

#### Debugging Steps:
1. Check container status:
   ```bash
   docker-compose -f docker-compose.local.yml ps
   ```
2. Check container logs:
   ```bash
   docker-compose -f docker-compose.local.yml logs -f [service_name]
   ```
3. Verify health checks are passing:
   ```bash
   docker inspect --format "{{.State.Health.Status}}" [container_id]
   ```

## Using Delve Debugger

For Go code debugging, Delve is recommended:

```bash
# Install Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug a specific application
cd apps/rest-api
dlv debug ./cmd/server/main.go

# Set breakpoints
(dlv) break api.VectorAPI.searchEmbeddings
```

## Testing Specific Components

### Testing the REST API

```bash
# Make requests to the REST API
curl -v -X POST http://localhost:8081/api/v1/vectors \
  -H "Content-Type: application/json" \
  -d '{
    "context_id": "test-context",
    "model_id": "test-model",
    "content_index": 1,
    "text": "Example content",
    "embedding": [0.1, 0.2, 0.3],
    "metadata": {"key": "value"}
  }'
```

### Testing Vector Search

```bash
# Search vectors
curl -v -X POST http://localhost:8081/api/v1/vectors/search \
  -H "Content-Type: application/json" \
  -d '{
    "context_id": "test-context",
    "model_id": "test-model",
    "query_embedding": [0.1, 0.2, 0.3],
    "limit": 5,
    "similarity_threshold": 0.7
  }'
```

## Reporting Issues

When reporting issues:

1. Include the exact error message
2. Note which component is failing (MCP Server, REST API, Worker)
3. Include relevant logs
4. Describe the steps to reproduce the issue
5. Mention your environment details (OS, Docker version, etc.)
