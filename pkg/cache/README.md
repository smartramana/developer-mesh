# Cache Package

> **Purpose**: Multi-level caching infrastructure for the DevOps MCP platform
> **Status**: Production Ready
> **Dependencies**: Redis (ElastiCache), LRU in-memory cache, distributed cache coordination

## Overview

The cache package provides a sophisticated multi-level caching strategy that balances performance, consistency, and cost. It implements a hierarchical cache with L1 (in-memory), L2 (Redis/ElastiCache), and optional L3 (S3) tiers.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Cache Architecture                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Request ──► L1 Cache ──► L2 Cache ──► L3 Cache ──► Origin │
│              (Memory)     (Redis)      (S3)       (DB/API)  │
│                 │            │           │                   │
│                 └────────────┴───────────┘                   │
│                        Write-Through                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Cache Interface

```go
// Cache defines the standard cache operations
type Cache interface {
    // Get retrieves a value from cache
    Get(ctx context.Context, key string) (interface{}, error)
    
    // Set stores a value in cache with TTL
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    
    // Delete removes a value from cache
    Delete(ctx context.Context, key string) error
    
    // Clear removes all values from cache
    Clear(ctx context.Context) error
    
    // Stats returns cache statistics
    Stats(ctx context.Context) (*CacheStats, error)
}
```

### 2. Multi-Level Cache

```go
// MultiLevelCache implements hierarchical caching
type MultiLevelCache struct {
    l1    *MemoryCache    // Fast, limited size
    l2    *RedisCache     // Shared, larger capacity
    l3    *S3Cache        // Cold storage for large objects
    stats *CacheStats
}

// Example usage
cache := NewMultiLevelCache(
    WithL1(NewMemoryCache(1000)),              // 1000 items max
    WithL2(NewRedisCache(redisClient)),        // ElastiCache
    WithL3(NewS3Cache(s3Client, "cache-bucket")), // Optional
)
```

### 3. Cache Key Strategies

```go
// KeyBuilder helps construct consistent cache keys
type KeyBuilder struct {
    namespace string
    version   string
}

// Example key patterns
kb := NewKeyBuilder("mcp", "v1")
key := kb.Build("context", contextID)           // mcp:v1:context:123
key := kb.BuildWithTags("embedding", modelID, text) // mcp:v1:embedding:titan-v2:hash(text)
```

## Features

### L1: In-Memory Cache

- **Technology**: LRU eviction with size limits
- **Capacity**: Configurable (default: 10,000 items)
- **TTL**: Item-level expiration
- **Use Cases**: Hot data, frequent access patterns

```go
// Memory cache with custom configuration
l1 := NewMemoryCache(
    WithMaxSize(10000),
    WithDefaultTTL(5*time.Minute),
    WithEvictionCallback(func(key string, value interface{}) {
        logger.Debug("evicted from L1", "key", key)
    }),
)
```

### L2: Redis Cache (ElastiCache)

- **Technology**: Redis 7.0+ with cluster mode
- **Features**: Pub/Sub for invalidation, atomic operations
- **Consistency**: Strong consistency with distributed locks
- **Use Cases**: Shared cache across services

```go
// Redis cache with ElastiCache configuration
l2 := NewRedisCache(
    WithRedisClient(redisClient),
    WithKeyPrefix("mcp"),
    WithSerializer(&JSONSerializer{}),
    WithCompression(true),
)
```

### L3: S3 Cache (Optional)

- **Technology**: S3 with lifecycle policies
- **Features**: Compression, encryption at rest
- **Use Cases**: Large objects, cold data, backups

```go
// S3 cache for large objects
l3 := NewS3Cache(
    WithS3Client(s3Client),
    WithBucket("mcp-cache"),
    WithPrefix("cache/"),
    WithCompression(true),
    WithEncryption(true),
)
```

## Cache Patterns

### 1. Cache-Aside Pattern

```go
func GetContext(ctx context.Context, id string) (*Context, error) {
    // Try cache first
    cached, err := cache.Get(ctx, "context:"+id)
    if err == nil {
        return cached.(*Context), nil
    }
    
    // Load from database
    context, err := db.GetContext(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Store in cache
    cache.Set(ctx, "context:"+id, context, 1*time.Hour)
    
    return context, nil
}
```

### 2. Write-Through Pattern

```go
func SaveContext(ctx context.Context, context *Context) error {
    // Save to database
    if err := db.SaveContext(ctx, context); err != nil {
        return err
    }
    
    // Update cache
    return cache.Set(ctx, "context:"+context.ID, context, 1*time.Hour)
}
```

### 3. Refresh-Ahead Pattern

```go
func RefreshPopularContexts(ctx context.Context) error {
    contexts, err := db.GetPopularContexts(ctx, 100)
    if err != nil {
        return err
    }
    
    for _, context := range contexts {
        // Refresh cache before expiration
        cache.Set(ctx, "context:"+context.ID, context, 2*time.Hour)
    }
    
    return nil
}
```

## Specialized Caches

### Embedding Cache

```go
// EmbeddingCache optimizes vector storage
type EmbeddingCache struct {
    cache     Cache
    dimension int
}

func (e *EmbeddingCache) GetEmbedding(ctx context.Context, text string, model string) ([]float32, error) {
    key := fmt.Sprintf("embedding:%s:%s", model, hash(text))
    
    cached, err := e.cache.Get(ctx, key)
    if err == nil {
        return cached.([]float32), nil
    }
    
    // Generate embedding
    embedding, err := generateEmbedding(ctx, text, model)
    if err != nil {
        return nil, err
    }
    
    // Cache with model-specific TTL
    ttl := e.getTTLForModel(model)
    e.cache.Set(ctx, key, embedding, ttl)
    
    return embedding, nil
}
```

### Query Result Cache

```go
// QueryCache caches database query results
type QueryCache struct {
    cache Cache
    db    *sql.DB
}

func (q *QueryCache) Query(ctx context.Context, query string, args ...interface{}) (*Result, error) {
    key := q.buildKey(query, args)
    
    // Check cache
    cached, err := q.cache.Get(ctx, key)
    if err == nil {
        return cached.(*Result), nil
    }
    
    // Execute query
    result, err := q.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    
    // Cache based on query pattern
    ttl := q.getTTLForQuery(query)
    q.cache.Set(ctx, key, result, ttl)
    
    return result, nil
}
```

## Cache Invalidation

### 1. TTL-Based Invalidation

```go
// Set with automatic expiration
cache.Set(ctx, key, value, 5*time.Minute)
```

### 2. Event-Based Invalidation

```go
// Subscribe to invalidation events
sub := redis.Subscribe(ctx, "cache:invalidate")
go func() {
    for msg := range sub.Channel() {
        var event InvalidationEvent
        json.Unmarshal([]byte(msg.Payload), &event)
        
        // Invalidate across all levels
        cache.Delete(ctx, event.Key)
    }
}()

// Publish invalidation
event := InvalidationEvent{Key: "context:123", Reason: "updated"}
redis.Publish(ctx, "cache:invalidate", event)
```

### 3. Tag-Based Invalidation

```go
// Tag-based cache management
cache.SetWithTags(ctx, key, value, ttl, "user:123", "project:456")

// Invalidate all entries for a user
cache.InvalidateByTag(ctx, "user:123")
```

## Performance Optimization

### 1. Batch Operations

```go
// Batch get for efficiency
keys := []string{"key1", "key2", "key3"}
values, err := cache.MGet(ctx, keys...)

// Batch set
items := map[string]interface{}{
    "key1": value1,
    "key2": value2,
}
cache.MSet(ctx, items, ttl)
```

### 2. Pipeline Support

```go
// Redis pipeline for multiple operations
pipe := cache.Pipeline()
pipe.Get(ctx, "key1")
pipe.Set(ctx, "key2", value2, ttl)
pipe.Delete(ctx, "key3")
results, err := pipe.Exec(ctx)
```

### 3. Compression

```go
// Enable compression for large values
cache := NewMultiLevelCache(
    WithCompression(CompressionConfig{
        Enabled:   true,
        Threshold: 1024, // Compress values > 1KB
        Level:     gzip.BestSpeed,
    }),
)
```

## Monitoring & Metrics

### Cache Statistics

```go
stats, err := cache.Stats(ctx)
// Returns:
// - Hit rate by level
// - Miss rate
// - Eviction count
// - Average latency
// - Memory usage
```

### Prometheus Metrics

```go
var (
    cacheHits = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_cache_hits_total",
            Help: "Total number of cache hits",
        },
        []string{"level", "cache_type"},
    )
    
    cacheMisses = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_cache_misses_total",
            Help: "Total number of cache misses",
        },
        []string{"level", "cache_type"},
    )
    
    cacheLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_cache_operation_duration_seconds",
            Help:    "Cache operation duration",
            Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15),
        },
        []string{"operation", "level"},
    )
)
```

## Configuration

### Environment Variables

```bash
# L1 Cache
CACHE_L1_ENABLED=true
CACHE_L1_MAX_SIZE=10000
CACHE_L1_DEFAULT_TTL=5m

# L2 Cache (Redis)
CACHE_L2_ENABLED=true
REDIS_ADDR=127.0.0.1:6379  # Via SSH tunnel
REDIS_PASSWORD=
REDIS_DB=0
CACHE_L2_DEFAULT_TTL=1h

# L3 Cache (S3)
CACHE_L3_ENABLED=false
CACHE_L3_BUCKET=mcp-cache
CACHE_L3_PREFIX=cache/
CACHE_L3_DEFAULT_TTL=24h
```

### Configuration File

```yaml
cache:
  l1:
    enabled: true
    max_size: 10000
    default_ttl: 5m
    eviction_policy: lru
    
  l2:
    enabled: true
    redis:
      addr: "127.0.0.1:6379"
      password: ""
      db: 0
      pool_size: 100
    default_ttl: 1h
    compression:
      enabled: true
      threshold: 1024
      
  l3:
    enabled: false
    s3:
      bucket: "mcp-cache"
      prefix: "cache/"
    default_ttl: 24h
    encryption: true
```

## Error Handling

```go
// Graceful degradation
value, err := cache.Get(ctx, key)
if err != nil {
    if errors.Is(err, ErrCacheMiss) {
        // Normal cache miss
        return loadFromOrigin(ctx, key)
    }
    
    if errors.Is(err, ErrCacheUnavailable) {
        // L2 cache down, try L1 only
        logger.Warn("L2 cache unavailable, falling back to L1")
        return l1Cache.Get(ctx, key)
    }
    
    // Log error but continue
    logger.Error("cache error", "error", err)
    return loadFromOrigin(ctx, key)
}
```

## Best Practices

1. **Key Design**: Use hierarchical, versioned keys
2. **TTL Strategy**: Balance freshness vs performance
3. **Size Limits**: Monitor memory usage, implement eviction
4. **Compression**: Enable for values > 1KB
5. **Monitoring**: Track hit rates, adjust cache sizes
6. **Fallback**: Always have origin fallback
7. **Warming**: Pre-warm cache for critical data
8. **Consistency**: Use distributed locks for updates

## Testing

```go
// Example cache test
func TestMultiLevelCache(t *testing.T) {
    // Create test cache with mock backends
    cache := NewMultiLevelCache(
        WithL1(NewMemoryCache(100)),
        WithL2(NewMockRedisCache()),
    )
    
    // Test cache operations
    ctx := context.Background()
    key := "test:key"
    value := "test-value"
    
    // Set value
    err := cache.Set(ctx, key, value, 1*time.Minute)
    assert.NoError(t, err)
    
    // Get value (should hit L1)
    cached, err := cache.Get(ctx, key)
    assert.NoError(t, err)
    assert.Equal(t, value, cached)
    
    // Verify metrics
    stats, _ := cache.Stats(ctx)
    assert.Equal(t, 1, stats.L1Hits)
    assert.Equal(t, 0, stats.L2Hits)
}
```

## Common Issues

1. **Cache Stampede**: Use distributed locks for cache refresh
2. **Memory Leaks**: Monitor L1 cache size, implement proper eviction
3. **Stale Data**: Implement proper invalidation strategies
4. **Network Latency**: Use connection pooling, pipeline operations
5. **Serialization**: Choose efficient serializers (MessagePack vs JSON)

---

Package Version: 1.0.0
Last Updated: 2024-01-10