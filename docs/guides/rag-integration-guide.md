# RAG + MCP Integration Guide

> **Purpose**: Step-by-step guide for integrating RAG systems with DevOps MCP
> **Audience**: Developers implementing RAG features using MCP
> **Prerequisites**: Basic understanding of RAG, Go programming, AWS services

## Overview

This guide provides practical instructions for integrating Retrieval-Augmented Generation (RAG) systems with the DevOps MCP platform. You'll learn how to leverage MCP's embedding pipeline, vector storage, and multi-agent orchestration to build powerful RAG applications.

## Quick Start

### 1. Basic RAG Setup

```go
package main

import (
    "context"
    "github.com/S-Corkum/devops-mcp/pkg/embedding"
    "github.com/S-Corkum/devops-mcp/pkg/services"
)

func main() {
    // Initialize embedding service
    embeddingService := services.NewEmbeddingService(
        db,           // PostgreSQL with pgvector
        bedrockClient,// AWS Bedrock client
        logger,
        tracer,
    )
    
    // Create RAG pipeline
    ragPipeline := NewRAGPipeline(embeddingService)
    
    // Process query
    response, err := ragPipeline.Query(ctx, "How do I implement caching?")
}
```

### 2. Environment Setup

```bash
# Required AWS services
export AWS_REGION=us-east-1
export S3_BUCKET=your-rag-bucket
export BEDROCK_ENABLED=true

# ElastiCache for vector caching
export REDIS_ADDR=127.0.0.1:6379
export USE_SSH_TUNNEL_FOR_REDIS=true

# Start services
make dev-native
```

## Core Components Integration

### 1. Embedding Pipeline Configuration

```go
// Configure the embedding pipeline for RAG
type RAGPipeline struct {
    embedding    *embedding.Pipeline
    vectorStore  *embedding.PostgresVectorStore
    chunker      chunking.Chunker
    cache        cache.Cache
}

func NewRAGPipeline(config RAGConfig) (*RAGPipeline, error) {
    // Initialize embedding pipeline
    pipelineConfig := &embedding.EmbeddingPipelineConfig{
        Concurrency:     10,
        BatchSize:       50,
        IncludeComments: true,
        EnrichMetadata:  true,
    }
    
    pipeline := embedding.NewPipeline(
        config.EmbeddingService,
        config.ChunkingService,
        config.VectorStore,
        pipelineConfig,
    )
    
    return &RAGPipeline{
        embedding:   pipeline,
        vectorStore: config.VectorStore,
        chunker:     config.Chunker,
        cache:       config.Cache,
    }, nil
}
```

### 2. Document Processing

```go
// Process documents for RAG
func (r *RAGPipeline) ProcessDocuments(ctx context.Context, docs []Document) error {
    for _, doc := range docs {
        // 1. Chunk the document
        chunks, err := r.chunker.Chunk(ctx, doc.Content, chunking.ChunkOptions{
            MaxChunkSize:     1000,
            ChunkOverlap:     200,
            PreserveContext:  true,
            SentenceBoundary: true,
        })
        if err != nil {
            return fmt.Errorf("chunking failed: %w", err)
        }
        
        // 2. Generate embeddings
        embeddings, err := r.embedding.ProcessChunks(ctx, chunks)
        if err != nil {
            return fmt.Errorf("embedding failed: %w", err)
        }
        
        // 3. Store in vector database
        for i, chunk := range chunks {
            vector := &embedding.Vector{
                ID:        uuid.New().String(),
                Content:   chunk.Content,
                Embedding: embeddings[i],
                Metadata: map[string]interface{}{
                    "document_id":   doc.ID,
                    "chunk_index":   i,
                    "document_type": doc.Type,
                    "created_at":    time.Now(),
                },
            }
            
            if err := r.vectorStore.StoreVector(ctx, vector); err != nil {
                return fmt.Errorf("vector storage failed: %w", err)
            }
        }
    }
    
    return nil
}
```

### 3. Query Processing

```go
// RAG query processing
func (r *RAGPipeline) Query(ctx context.Context, query string) (*RAGResponse, error) {
    // 1. Check cache
    cacheKey := fmt.Sprintf("rag:query:%s", hash(query))
    if cached, err := r.cache.Get(ctx, cacheKey); err == nil {
        return cached.(*RAGResponse), nil
    }
    
    // 2. Generate query embedding
    queryEmbedding, err := r.embedding.GenerateEmbedding(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query embedding failed: %w", err)
    }
    
    // 3. Vector search with filters
    searchOptions := &embedding.SearchOptions{
        TopK:            10,
        MinScore:        0.7,
        IncludeMetadata: true,
        Filters: map[string]interface{}{
            "document_type": []string{"code", "documentation"},
        },
    }
    
    results, err := r.vectorStore.Search(ctx, queryEmbedding, searchOptions)
    if err != nil {
        return nil, fmt.Errorf("vector search failed: %w", err)
    }
    
    // 4. Build context from results
    context := r.buildContext(results)
    
    // 5. Generate response using MCP agent
    response, err := r.generateResponse(ctx, query, context)
    if err != nil {
        return nil, fmt.Errorf("response generation failed: %w", err)
    }
    
    // 6. Cache result
    r.cache.Set(ctx, cacheKey, response, 1*time.Hour)
    
    return response, nil
}
```

## Multi-Agent RAG Integration

### 1. Agent Registration

```go
// Register specialized RAG agents
func RegisterRAGAgents(mcp *MCPServer) error {
    // Code analysis agent
    codeAgent := &Agent{
        ID:   "rag-code-agent",
        Type: "code-analysis",
        Capabilities: AgentCapabilities{
            ModelType:       "gpt-4",
            Specializations: []string{"code", "syntax", "algorithms"},
            EmbeddingModels: []string{"text-embedding-ada-002"},
        },
    }
    
    // Documentation agent
    docsAgent := &Agent{
        ID:   "rag-docs-agent",
        Type: "documentation",
        Capabilities: AgentCapabilities{
            ModelType:       "claude-2",
            Specializations: []string{"documentation", "tutorials", "guides"},
            EmbeddingModels: []string{"titan-embed-text-v1"},
        },
    }
    
    // Register agents
    mcp.RegisterAgent(codeAgent)
    mcp.RegisterAgent(docsAgent)
    
    return nil
}
```

### 2. Collaborative RAG Processing

```go
// Multi-agent collaborative RAG
type CollaborativeRAG struct {
    mcp         *MCPServer
    pipeline    *RAGPipeline
    coordinator *AgentCoordinator
}

func (c *CollaborativeRAG) ProcessQuery(ctx context.Context, query Query) (*Response, error) {
    // 1. Analyze query to determine required agents
    analysis := c.analyzeQuery(query)
    
    // 2. Create RAG task
    task := &RAGTask{
        ID:       uuid.New().String(),
        Query:    query.Text,
        Type:     analysis.Type,
        Agents:   analysis.RequiredAgents,
        Strategy: "collaborative",
    }
    
    // 3. Distribute to agents
    subtasks := c.createSubtasks(task, analysis)
    results := make(chan AgentResult, len(subtasks))
    
    for _, subtask := range subtasks {
        agent := c.mcp.GetAgent(subtask.AgentID)
        go func(st Subtask) {
            result := agent.ProcessRAG(ctx, st)
            results <- result
        }(subtask)
    }
    
    // 4. Collect and merge results
    var agentResults []AgentResult
    for i := 0; i < len(subtasks); i++ {
        agentResults = append(agentResults, <-results)
    }
    
    // 5. Synthesize final response
    return c.synthesizeResponse(agentResults)
}
```

## AWS Bedrock Integration

### 1. Multi-Model Embedding

```go
// Configure Bedrock for multi-model embeddings
type BedrockRAGConfig struct {
    Models []BedrockModel
    Router *ModelRouter
}

func NewBedrockRAG(config BedrockRAGConfig) *BedrockRAG {
    // Initialize Bedrock providers
    providers := make(map[string]embedding.Provider)
    
    for _, model := range config.Models {
        provider := embedding.NewBedrockProvider(embedding.BedrockConfig{
            Model:     model.ID,
            Region:    model.Region,
            Dimension: model.Dimension,
        })
        providers[model.ID] = provider
    }
    
    return &BedrockRAG{
        providers: providers,
        router:    config.Router,
    }
}

// Route to appropriate model based on content
func (b *BedrockRAG) GenerateEmbedding(ctx context.Context, content string) ([]float32, error) {
    // Select model based on content type
    model := b.router.SelectModel(content)
    provider := b.providers[model]
    
    return provider.GenerateEmbedding(ctx, content)
}
```

### 2. Cost-Optimized Processing

```go
// Cost-aware RAG processing
type CostOptimizedRAG struct {
    budget      *Budget
    metrics     *CostMetrics
    strategies  []OptimizationStrategy
}

func (c *CostOptimizedRAG) Process(ctx context.Context, request Request) (*Response, error) {
    // 1. Estimate cost
    estimate := c.estimateCost(request)
    
    // 2. Select strategy based on budget
    strategy := c.selectStrategy(estimate, c.budget.Remaining())
    
    // 3. Apply optimizations
    optimized := strategy.Optimize(request)
    
    // 4. Process with cost tracking
    start := time.Now()
    response, err := c.processOptimized(ctx, optimized)
    
    // 5. Track actual cost
    c.metrics.Track(CostMetric{
        RequestID: request.ID,
        Model:     optimized.Model,
        Tokens:    response.TokensUsed,
        Cost:      calculateCost(response),
        Duration:  time.Since(start),
    })
    
    return response, err
}
```

## Vector Storage with pgvector

### 1. Schema Setup

```sql
-- Create vectors table with pgvector
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE rag_vectors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content TEXT NOT NULL,
    embedding vector(1536), -- Adjust dimension based on model
    metadata JSONB,
    document_id UUID,
    chunk_index INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes for performance
    INDEX idx_rag_vectors_document (document_id),
    INDEX idx_rag_vectors_created (created_at DESC)
);

-- Create index for vector similarity search
CREATE INDEX idx_rag_vectors_embedding ON rag_vectors 
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);
```

### 2. Vector Operations

```go
// pgvector operations for RAG
type PgVectorStore struct {
    db     *sqlx.DB
    logger observability.Logger
}

// Similarity search with metadata filtering
func (p *PgVectorStore) SearchWithFilters(
    ctx context.Context, 
    embedding []float32,
    filters map[string]interface{},
    limit int,
) ([]SearchResult, error) {
    query := `
        SELECT 
            id,
            content,
            metadata,
            1 - (embedding <=> $1::vector) as similarity
        FROM rag_vectors
        WHERE 1=1
    `
    
    args := []interface{}{pgvector.NewVector(embedding)}
    argCount := 1
    
    // Add metadata filters
    for key, value := range filters {
        argCount++
        query += fmt.Sprintf(" AND metadata->>'%s' = $%d", key, argCount)
        args = append(args, value)
    }
    
    query += fmt.Sprintf(`
        ORDER BY embedding <=> $1::vector
        LIMIT $%d
    `, argCount+1)
    
    args = append(args, limit)
    
    var results []SearchResult
    err := p.db.SelectContext(ctx, &results, query, args...)
    
    return results, err
}
```

## Caching Strategy

### 1. Multi-Level Cache

```go
// Multi-level caching for RAG
type RAGCache struct {
    l1 *MemoryCache      // In-memory cache
    l2 *RedisCache       // Redis cache
    l3 *S3Cache          // S3 for large embeddings
}

func (c *RAGCache) Get(ctx context.Context, key string) (interface{}, error) {
    // Check L1 (memory)
    if val, found := c.l1.Get(key); found {
        return val, nil
    }
    
    // Check L2 (Redis)
    val, err := c.l2.Get(ctx, key)
    if err == nil {
        c.l1.Set(key, val, 5*time.Minute) // Promote to L1
        return val, nil
    }
    
    // Check L3 (S3)
    val, err = c.l3.Get(ctx, key)
    if err == nil {
        c.l2.Set(ctx, key, val, 1*time.Hour) // Promote to L2
        c.l1.Set(key, val, 5*time.Minute)    // Promote to L1
        return val, nil
    }
    
    return nil, ErrCacheMiss
}
```

### 2. Embedding Cache

```go
// Cache embeddings to reduce API calls
type EmbeddingCache struct {
    cache cache.Cache
    ttl   time.Duration
}

func (e *EmbeddingCache) GetOrGenerate(
    ctx context.Context,
    content string,
    generator func(string) ([]float32, error),
) ([]float32, error) {
    // Generate cache key
    key := fmt.Sprintf("embedding:%s", hash(content))
    
    // Check cache
    if cached, err := e.cache.Get(ctx, key); err == nil {
        return cached.([]float32), nil
    }
    
    // Generate new embedding
    embedding, err := generator(content)
    if err != nil {
        return nil, err
    }
    
    // Cache result
    e.cache.Set(ctx, key, embedding, e.ttl)
    
    return embedding, nil
}
```

## WebSocket Real-Time RAG

### 1. Streaming Responses

```go
// Stream RAG responses via WebSocket
type StreamingRAG struct {
    ws       *websocket.Conn
    pipeline *RAGPipeline
}

func (s *StreamingRAG) StreamQuery(ctx context.Context, query string) error {
    // 1. Send acknowledgment
    s.ws.WriteJSON(Message{
        Type: "rag.query.received",
        Data: map[string]string{"query": query},
    })
    
    // 2. Stream retrieval progress
    retrievalChan := make(chan RetrievalUpdate)
    go s.pipeline.RetrieveWithProgress(ctx, query, retrievalChan)
    
    for update := range retrievalChan {
        s.ws.WriteJSON(Message{
            Type: "rag.retrieval.progress",
            Data: update,
        })
    }
    
    // 3. Stream generation
    responseChan := make(chan string)
    go s.pipeline.GenerateStreaming(ctx, query, responseChan)
    
    for chunk := range responseChan {
        s.ws.WriteJSON(Message{
            Type: "rag.response.chunk",
            Data: map[string]string{"content": chunk},
        })
    }
    
    // 4. Send completion
    s.ws.WriteJSON(Message{
        Type: "rag.query.complete",
    })
    
    return nil
}
```

### 2. Collaborative RAG Sessions

```go
// Multi-user collaborative RAG
type CollaborativeSession struct {
    id       string
    users    map[string]*User
    ragState *RAGState
    crdt     *CRDT
}

func (c *CollaborativeSession) HandleMessage(userID string, msg Message) error {
    switch msg.Type {
    case "rag.context.add":
        // User adds context
        c.crdt.AddContext(userID, msg.Data.(Context))
        c.broadcastUpdate("context.added", msg.Data)
        
    case "rag.query.refine":
        // Collaborative query refinement
        refinement := msg.Data.(QueryRefinement)
        c.ragState.ApplyRefinement(refinement)
        c.broadcastUpdate("query.refined", refinement)
        
    case "rag.source.verify":
        // Collaborative source verification
        verification := msg.Data.(SourceVerification)
        c.ragState.VerifySource(verification)
        c.broadcastUpdate("source.verified", verification)
    }
    
    return nil
}
```

## Performance Optimization

### 1. Batch Processing

```go
// Batch embeddings for efficiency
func (r *RAGPipeline) BatchProcess(ctx context.Context, documents []Document) error {
    batches := r.createBatches(documents, 100) // 100 docs per batch
    
    var wg sync.WaitGroup
    errors := make(chan error, len(batches))
    
    for _, batch := range batches {
        wg.Add(1)
        go func(b []Document) {
            defer wg.Done()
            if err := r.processBatch(ctx, b); err != nil {
                errors <- err
            }
        }(batch)
    }
    
    wg.Wait()
    close(errors)
    
    // Check for errors
    for err := range errors {
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### 2. Precomputed Embeddings

```go
// Precompute common embeddings
type PrecomputedEmbeddings struct {
    store   *S3Store
    index   map[string]string // content hash -> S3 key
}

func (p *PrecomputedEmbeddings) Initialize(ctx context.Context) error {
    // Load common queries/documents
    commonContent := p.loadCommonContent()
    
    // Generate and store embeddings
    for _, content := range commonContent {
        embedding, err := p.generateEmbedding(ctx, content)
        if err != nil {
            continue
        }
        
        key := fmt.Sprintf("embeddings/precomputed/%s", hash(content))
        p.store.Put(ctx, key, embedding)
        p.index[hash(content)] = key
    }
    
    return nil
}
```

## Monitoring and Debugging

### 1. RAG Metrics

```go
// Track RAG performance metrics
var (
    ragQueryLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "rag_query_latency_seconds",
            Help: "RAG query processing latency",
        },
        []string{"query_type", "model"},
    )
    
    ragRetrievalRelevance = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "rag_retrieval_relevance_score",
            Help: "Relevance score of retrieved documents",
        },
        []string{"query_type"},
    )
    
    ragCacheHitRate = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rag_cache_hits_total",
            Help: "Number of RAG cache hits",
        },
        []string{"cache_level"},
    )
)
```

### 2. Distributed Tracing

```go
// Trace RAG operations
func (r *RAGPipeline) QueryWithTracing(ctx context.Context, query string) (*Response, error) {
    ctx, span := r.tracer.Start(ctx, "rag.query",
        trace.WithAttributes(
            attribute.String("query.text", query),
            attribute.Int("query.length", len(query)),
        ),
    )
    defer span.End()
    
    // Trace embedding generation
    embCtx, embSpan := r.tracer.Start(ctx, "rag.embedding.generate")
    embedding, err := r.generateEmbedding(embCtx, query)
    embSpan.End()
    
    if err != nil {
        span.RecordError(err)
        return nil, err
    }
    
    // Trace vector search
    searchCtx, searchSpan := r.tracer.Start(ctx, "rag.vector.search")
    results, err := r.search(searchCtx, embedding)
    searchSpan.SetAttributes(
        attribute.Int("results.count", len(results)),
    )
    searchSpan.End()
    
    // Continue with response generation...
}
```

## Testing RAG Integration

### 1. Integration Tests

```go
func TestRAGPipeline(t *testing.T) {
    // Setup test environment
    pipeline := setupTestPipeline(t)
    
    // Test document processing
    t.Run("ProcessDocuments", func(t *testing.T) {
        docs := []Document{
            {ID: "1", Content: "Test content", Type: "code"},
        }
        
        err := pipeline.ProcessDocuments(context.Background(), docs)
        assert.NoError(t, err)
        
        // Verify embeddings stored
        vectors, err := pipeline.vectorStore.List(context.Background())
        assert.NoError(t, err)
        assert.Len(t, vectors, 1)
    })
    
    // Test query processing
    t.Run("Query", func(t *testing.T) {
        response, err := pipeline.Query(context.Background(), "test query")
        assert.NoError(t, err)
        assert.NotNil(t, response)
        assert.NotEmpty(t, response.Content)
    })
}
```

### 2. Benchmark Tests

```go
func BenchmarkRAGQuery(b *testing.B) {
    pipeline := setupBenchmarkPipeline(b)
    queries := generateTestQueries(100)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            query := queries[i%len(queries)]
            _, err := pipeline.Query(context.Background(), query)
            if err != nil {
                b.Fatal(err)
            }
            i++
        }
    })
}
```

## Common Patterns

### 1. Hybrid Search

```go
// Combine vector and keyword search
func (r *RAGPipeline) HybridSearch(ctx context.Context, query string) ([]Result, error) {
    // Vector search
    vectorResults := r.vectorSearch(ctx, query)
    
    // Keyword search
    keywordResults := r.keywordSearch(ctx, query)
    
    // Merge and rerank
    merged := r.mergeResults(vectorResults, keywordResults)
    reranked := r.rerank(merged, query)
    
    return reranked, nil
}
```

### 2. Context Window Management

```go
// Manage context window for LLMs
func (r *RAGPipeline) OptimizeContext(results []Result, maxTokens int) string {
    // Sort by relevance
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })
    
    // Build context within token limit
    var context strings.Builder
    tokenCount := 0
    
    for _, result := range results {
        tokens := r.countTokens(result.Content)
        if tokenCount+tokens > maxTokens {
            break
        }
        
        context.WriteString(result.Content)
        context.WriteString("\n\n")
        tokenCount += tokens
    }
    
    return context.String()
}
```

## Troubleshooting

### Common Issues

1. **Slow Embedding Generation**
   - Enable caching at multiple levels
   - Use batch processing
   - Consider precomputing common embeddings

2. **Poor Retrieval Quality**
   - Tune chunk size and overlap
   - Implement hybrid search
   - Add metadata filtering

3. **High Costs**
   - Implement cost tracking
   - Use tiered model selection
   - Cache aggressively

4. **WebSocket Connection Issues**
   - Implement reconnection logic
   - Use connection pooling
   - Add heartbeat monitoring

## Next Steps

1. Review [RAG Patterns](./rag-patterns.md) for advanced implementations
2. Explore [AI Agent Orchestration](./ai-agent-orchestration.md) for multi-agent RAG
3. See [Performance Tuning Guide](./performance-tuning-guide.md) for optimization
4. Check [Cost Optimization Guide](./cost-optimization-guide.md) for managing expenses

## Resources

- [pgvector Documentation](https://github.com/pgvector/pgvector)
- [AWS Bedrock Embeddings](https://docs.aws.amazon.com/bedrock/latest/userguide/embeddings.html)
- [Chunking Strategies](https://www.pinecone.io/learn/chunking-strategies/)
- [RAG Evaluation Metrics](https://arxiv.org/abs/2309.01431)