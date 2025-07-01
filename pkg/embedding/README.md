# Embedding Package

## Overview

The `embedding` package provides a sophisticated, production-ready system for generating, storing, and searching vector embeddings in the DevOps MCP platform. It supports multiple AI providers, intelligent routing, cost optimization, and fault tolerance with enterprise-grade features for multi-agent orchestration.

## Architecture

```
embedding/
├── providers/          # Embedding provider implementations
├── bedrock.go         # AWS Bedrock integration
├── openai.go          # OpenAI integration
├── service_v2.go      # Enhanced embedding service
├── pipeline.go        # Content processing pipeline
├── router.go          # Intelligent provider routing
├── repository.go      # pgvector storage layer
├── search.go          # Vector search capabilities
├── circuit_breaker.go # Fault tolerance
└── dimension_adapter.go # Cross-model compatibility
```

## Key Features

- **Multi-Provider Support**: OpenAI, AWS Bedrock, Anthropic, Google, Voyage
- **Smart Routing**: Automatic provider selection based on cost/quality/speed
- **Dimension Normalization**: Handle different embedding dimensions seamlessly
- **Fault Tolerance**: Circuit breakers prevent cascade failures
- **Cost Optimization**: Track and optimize embedding generation costs
- **Batch Processing**: Efficient batch operations with progress tracking
- **Vector Search**: Advanced similarity search with metadata filtering
- **Multi-Agent Support**: Different strategies per agent
- **Caching**: Reduce costs with intelligent caching

## Provider Support

### Supported Models

| Provider | Model | Dimensions | Cost/1K Tokens | Use Case |
|----------|-------|------------|----------------|----------|
| **OpenAI** | text-embedding-3-small | 1536 | $0.02 | General purpose |
| | text-embedding-3-large | 3072 | $0.13 | High quality |
| | text-embedding-ada-002 | 1536 | $0.10 | Legacy |
| **AWS Bedrock** | amazon.titan-embed-text-v1 | 1536 | $0.10 | AWS native |
| | amazon.titan-embed-text-v2 | 1024 | $0.02 | Cost optimized |
| | cohere.embed-english-v3 | 1024 | $0.10 | English text |
| | cohere.embed-multilingual-v3 | 1024 | $0.10 | Multi-language |
| **Anthropic** | claude-synthetic | 1536 | Variable | Via Claude |
| **Google** | textembedding-gecko | 768 | $0.05 | Small & fast |
| **Voyage** | voyage-02 | 1024 | $0.10 | Specialized |

### Provider Interface

All providers implement this interface:

```go
type Provider interface {
    // Generate single embedding
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    
    // Batch generate embeddings
    BatchGenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
    
    // Get supported models
    GetSupportedModels() []ModelInfo
    
    // Health check
    HealthCheck(ctx context.Context) error
    
    // Get metadata
    GetProviderInfo() ProviderInfo
}
```

## Usage Examples

### Basic Embedding Generation

```go
// Initialize service
service, err := embedding.NewServiceV2(
    repo,
    cache,
    logger,
    tracer,
)

// Generate embedding
result, err := service.GenerateEmbedding(ctx, &GenerateRequest{
    Text:    "Implement user authentication",
    Model:   "text-embedding-3-small",
    AgentID: agentID,
})

// Access embedding
vector := result.Embedding // []float32
```

### Batch Processing

```go
// Batch generate with progress tracking
results, err := service.BatchGenerateEmbeddings(ctx, &BatchRequest{
    Texts: []string{
        "Fix memory leak in worker",
        "Optimize database queries",
        "Add unit tests for API",
    },
    Model: "amazon.titan-embed-text-v1",
    Options: BatchOptions{
        Concurrency: 10,
        OnProgress: func(processed, total int) {
            log.Printf("Progress: %d/%d", processed, total)
        },
    },
})
```

### Smart Routing

```go
// Configure routing strategy
router := embedding.NewSmartRouter(
    providers,
    embedding.RouterConfig{
        Strategy: embedding.StrategyBalanced,
        CostLimit: 0.50, // Max $0.50 per request
        QualityThreshold: 0.8,
    },
)

// Router automatically selects best provider
provider := router.SelectProvider(ctx, &SelectionCriteria{
    TaskType: "code_analysis",
    AgentCapabilities: []string{"premium"},
    TextLength: len(text),
})
```

### Vector Search

```go
// Search similar embeddings
results, err := service.SearchEmbeddings(ctx, &SearchRequest{
    Query: "authentication bug",
    Model: "text-embedding-3-small",
    TopK: 10,
    Filters: map[string]interface{}{
        "type": "issue",
        "status": "open",
    },
    MinSimilarity: 0.7,
})

// Process results
for _, result := range results.Results {
    fmt.Printf("Score: %.3f, ID: %s\n", 
        result.Score, 
        result.ContextID,
    )
}
```

### Cross-Model Search

```go
// Search across embeddings from different models
results, err := service.CrossModelSearch(ctx, &CrossModelRequest{
    Query: "performance optimization",
    Models: []string{
        "text-embedding-3-small",
        "amazon.titan-embed-text-v1",
    },
    TopK: 20,
    Strategy: embedding.MergeStrategyWeighted,
})
```

## Pipeline Processing

The embedding pipeline processes different content types:

```go
// Initialize pipeline
pipeline := embedding.NewPipeline(
    embeddingService,
    chunkingService,
    githubAdapter,
    logger,
)

// Process GitHub issue
err := pipeline.ProcessIssue(ctx, &IssueRequest{
    Owner: "S-Corkum",
    Repo: "devops-mcp",
    Number: 123,
})

// Process code file
err := pipeline.ProcessCode(ctx, &CodeRequest{
    FilePath: "pkg/services/agent.go",
    Language: "go",
    ChunkSize: 500,
})
```

## Dimension Adaptation

Handle different embedding dimensions:

```go
// Initialize adapter
adapter := embedding.NewDimensionAdapter()

// Adapt embeddings to target dimension
adapted, err := adapter.AdaptEmbedding(
    sourceEmbedding, // 1536 dimensions
    1024,           // target dimensions
    "text-embedding-3-small", // source model
    "amazon.titan-embed-text-v1", // target model
)

// Check adaptation quality
quality := adapter.GetAdaptationQuality(
    sourceModel,
    targetModel,
)
```

## Circuit Breaker

Fault tolerance for provider failures:

```go
// Wrap provider with circuit breaker
protected := embedding.NewCircuitBreaker(
    provider,
    embedding.CircuitConfig{
        FailureThreshold: 5,
        SuccessThreshold: 2,
        Timeout: 30 * time.Second,
        ResetTimeout: 60 * time.Second,
    },
)

// Circuit breaker states
// Closed -> Open (after failures)
// Open -> Half-Open (after reset timeout)
// Half-Open -> Closed (after successes)
```

## Cost Management

Track and optimize embedding costs:

```go
// Get cost estimate
estimate := service.EstimateCost(&CostRequest{
    Texts: texts,
    Model: "text-embedding-3-large",
})

// Set cost limits
service.SetCostLimit(10.0) // $10 limit

// Get usage report
report := service.GetUsageReport(ctx, &UsageRequest{
    StartTime: time.Now().Add(-24 * time.Hour),
    EndTime: time.Now(),
    GroupBy: "model",
})
```

## Repository Operations

Store and retrieve embeddings:

```go
// Insert embedding
err := repo.InsertEmbedding(ctx, &Embedding{
    ID: uuid.New(),
    ContextID: contextID,
    Model: "text-embedding-3-small",
    Vector: vector,
    Metadata: map[string]interface{}{
        "type": "code",
        "language": "go",
    },
})

// Get embeddings by context
embeddings, err := repo.GetEmbeddingsByContext(ctx, contextID)

// Bulk operations
err := repo.BulkInsertEmbeddings(ctx, embeddings)
```

## Configuration

### Environment Variables

```bash
# OpenAI
OPENAI_API_KEY=sk-...
OPENAI_ORG_ID=org-...

# AWS Bedrock
AWS_REGION=us-east-1
BEDROCK_ENABLED=true

# Cost controls
EMBEDDING_COST_LIMIT=10.0
EMBEDDING_CACHE_TTL=3600

# Performance
EMBEDDING_BATCH_SIZE=100
EMBEDDING_CONCURRENCY=10
```

### Service Configuration

```go
config := embedding.ServiceConfig{
    // Provider settings
    DefaultModel: "text-embedding-3-small",
    EnabledProviders: []string{"openai", "bedrock"},
    
    // Cost controls
    MaxCostPerRequest: 0.50,
    DailyCostLimit: 100.0,
    
    // Performance
    BatchSize: 100,
    Concurrency: 10,
    CacheTTL: time.Hour,
    
    // Retry policy
    MaxRetries: 3,
    RetryDelay: time.Second,
}

service := embedding.NewServiceV2WithConfig(config)
```

## Routing Strategies

Different strategies for provider selection:

```go
// Quality First - Highest quality embeddings
router.SetStrategy(embedding.StrategyQuality)

// Cost Optimized - Lowest cost provider
router.SetStrategy(embedding.StrategyCost)

// Speed Optimized - Fastest response time
router.SetStrategy(embedding.StrategySpeed)

// Balanced - Balance all factors
router.SetStrategy(embedding.StrategyBalanced)

// Custom strategy
router.SetCustomStrategy(func(providers []Provider, criteria Criteria) Provider {
    // Custom selection logic
    return selectedProvider
})
```

## Metrics and Monitoring

Built-in metrics collection:

```go
// Metrics automatically collected:
- embedding_generation_total (counter)
- embedding_generation_duration_seconds (histogram)
- embedding_cost_dollars (counter)
- embedding_cache_hit_rate (gauge)
- embedding_provider_errors_total (counter)
- embedding_dimension_adaptations_total (counter)
- circuit_breaker_state (gauge)
```

## Error Handling

Comprehensive error types:

```go
// Check error types
switch err := err.(type) {
case *embedding.ProviderError:
    // Provider-specific error
    if err.Retryable {
        // Can retry
    }
case *embedding.DimensionError:
    // Dimension mismatch
case *embedding.CostLimitError:
    // Cost limit exceeded
case *embedding.RateLimitError:
    // Rate limited
    time.Sleep(err.RetryAfter)
}
```

## Best Practices

### 1. Use Batch Processing

```go
// Good: Batch multiple texts
results, err := service.BatchGenerateEmbeddings(ctx, texts)

// Avoid: Individual calls in loop
for _, text := range texts {
    result, err := service.GenerateEmbedding(ctx, text)
}
```

### 2. Enable Caching

```go
// Cache frequently used embeddings
service.EnableCache(cache.NewRedisCache(redis))
```

### 3. Monitor Costs

```go
// Set cost alerts
service.OnCostThreshold(5.0, func(usage float64) {
    alert.Send("Embedding cost at $%.2f", usage)
})
```

### 4. Handle Dimension Differences

```go
// Always check dimensions when switching models
if sourceModel != targetModel {
    adapted, err := adapter.AdaptEmbedding(...)
}
```

## Testing

### Unit Tests

```go
// Use mock providers
mockProvider := embedding.NewMockProvider()
mockProvider.On("GenerateEmbedding", text).Return(vector, nil)

// Test with mock
service := embedding.NewServiceV2(repo, cache, logger, tracer)
service.AddProvider("mock", mockProvider)
```

### Integration Tests

```go
// Test with real providers
func TestRealProviders(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Test each provider
    for _, provider := range providers {
        t.Run(provider.Name(), func(t *testing.T) {
            // Test embedding generation
        })
    }
}
```

## Performance Considerations

- **Batch Size**: Optimal batch size is 50-100 texts
- **Caching**: Can reduce costs by 60-80% for repeated content
- **Dimension Adaptation**: ~5% quality loss when reducing dimensions
- **Circuit Breaker**: Prevents cascade failures, adds <1ms overhead
- **pgvector Indexing**: Use IVFFLAT index for large datasets

## Future Enhancements

- [ ] Support for multimodal embeddings (text + image)
- [ ] Custom fine-tuned models
- [ ] Embedding compression techniques
- [ ] Real-time embedding updates
- [ ] Federated search across regions
- [ ] GPU acceleration for local models

## References

- [OpenAI Embeddings Guide](https://platform.openai.com/docs/guides/embeddings)
- [AWS Bedrock Documentation](https://docs.aws.amazon.com/bedrock/)
- [pgvector Documentation](https://github.com/pgvector/pgvector)
- [Vector Search Best Practices](../docs/guides/vector-search.md)