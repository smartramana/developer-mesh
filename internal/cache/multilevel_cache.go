package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	lru "github.com/hashicorp/golang-lru/v2"
)

// MultiLevelCache implements multi-level caching with in-memory and Redis caches
type MultiLevelCache struct {
	// L1 cache (in-memory)
	l1Cache *lru.Cache[string, []byte]
	
	// L2 cache (Redis)
	l2Cache Cache
	
	// Configuration
	ttl        time.Duration
	metricsClient observability.MetricsClient
	
	// Prefetch queue
	prefetchQueue   chan prefetchRequest
	prefetchWorkers int
	prefetchMutex   sync.Mutex
}

// prefetchRequest represents a request to prefetch related data
type prefetchRequest struct {
	key      string
	context  *mcp.Context
	metadata map[string]interface{}
}

// MultiLevelCacheConfig holds configuration for the multi-level cache
type MultiLevelCacheConfig struct {
	L1MaxSize       int           `mapstructure:"l1_max_size"`
	DefaultTTL      time.Duration `mapstructure:"default_ttl"`
	PrefetchWorkers int           `mapstructure:"prefetch_workers"`
	PrefetchQueueSize int         `mapstructure:"prefetch_queue_size"`
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache(l2Cache Cache, config MultiLevelCacheConfig) (*MultiLevelCache, error) {
	// Apply defaults
	if config.L1MaxSize <= 0 {
		config.L1MaxSize = 1000 // Default to 1000 entries
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 15 * time.Minute // Default to 15 minutes
	}
	if config.PrefetchWorkers <= 0 {
		config.PrefetchWorkers = 2 // Default to 2 workers
	}
	if config.PrefetchQueueSize <= 0 {
		config.PrefetchQueueSize = 100 // Default to 100 entries
	}
	
	// Create L1 cache
	l1Cache, err := lru.New[string, []byte](config.L1MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 cache: %w", err)
	}
	
	// Create multi-level cache
	mlc := &MultiLevelCache{
		l1Cache:       l1Cache,
		l2Cache:       l2Cache,
		ttl:           config.DefaultTTL,
		metricsClient: observability.NewMetricsClient(),
		prefetchQueue: make(chan prefetchRequest, config.PrefetchQueueSize),
		prefetchWorkers: config.PrefetchWorkers,
	}
	
	// Start prefetch workers
	for i := 0; i < config.PrefetchWorkers; i++ {
		go mlc.prefetchWorker()
	}
	
	return mlc, nil
}

// prefetchWorker processes prefetch requests
func (c *MultiLevelCache) prefetchWorker() {
	for req := range c.prefetchQueue {
		// Skip if already in L1 cache to avoid unnecessary work
		if _, ok := c.l1Cache.Get(req.key); ok {
			continue
		}
		
		// Get from L2 cache
		_, _ = c.GetContext(context.Background(), req.key)
		
		// Don't block the worker if there's an error
	}
}

// queuePrefetch adds a prefetch request to the queue
func (c *MultiLevelCache) queuePrefetch(key string, context *mcp.Context, metadata map[string]interface{}) {
	// Skip if prefetch queue is full
	select {
	case c.prefetchQueue <- prefetchRequest{key: key, context: context, metadata: metadata}:
		// Request added to queue
	default:
		// Queue is full, skip prefetch
	}
}

// Set stores a value in the cache
func (c *MultiLevelCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	startTime := time.Now()
	
	// Marshal value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	
	// Add to L1 cache
	c.l1Cache.Add(key, data)
	
	// Add to L2 cache
	if ttl <= 0 {
		ttl = c.ttl
	}
	err = c.l2Cache.Set(ctx, key, data, ttl)
	
	// Record metrics
	duration := time.Since(startTime)
	c.metricsClient.RecordCacheOperation("set", true, duration.Seconds())
	
	return err
}

// Get retrieves a value from the cache
func (c *MultiLevelCache) Get(ctx context.Context, key string, value interface{}) (bool, error) {
	startTime := time.Now()
	
	// Try L1 cache first
	if data, ok := c.l1Cache.Get(key); ok {
		// Unmarshal value
		err := json.Unmarshal(data, value)
		if err != nil {
			return false, fmt.Errorf("failed to unmarshal value from L1 cache: %w", err)
		}
		
		// Record metrics
		duration := time.Since(startTime)
		c.metricsClient.RecordCacheOperation("get", true, duration.Seconds())
		
		return true, nil
	}
	
	// Try L2 cache
	var data []byte
	err := c.l2Cache.Get(ctx, key, &data)
	if err != nil {
		// Record metrics
		duration := time.Since(startTime)
		c.metricsClient.RecordCacheOperation("get", false, duration.Seconds())
		
		// If it's a not found error, return false with no error
		if err == ErrNotFound {
			return false, nil
		}
		
		return false, err
	}
	
	// Add to L1 cache
	c.l1Cache.Add(key, data)
	
	// Unmarshal value
	err = json.Unmarshal(data, value)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal value from L2 cache: %w", err)
	}
	
	// Record metrics
	duration := time.Since(startTime)
	c.metricsClient.RecordCacheOperation("get", true, duration.Seconds())
	
	return true, nil
}

// Delete removes a value from the cache
func (c *MultiLevelCache) Delete(ctx context.Context, key string) error {
	startTime := time.Now()
	
	// Remove from L1 cache
	c.l1Cache.Remove(key)
	
	// Remove from L2 cache
	err := c.l2Cache.Delete(ctx, key)
	
	// Record metrics
	duration := time.Since(startTime)
	c.metricsClient.RecordCacheOperation("delete", true, duration.Seconds())
	
	return err
}

// Close closes the cache
func (c *MultiLevelCache) Close() error {
	// Close prefetch queue
	close(c.prefetchQueue)
	
	// Close L2 cache
	return c.l2Cache.Close()
}

// SetContext stores a context in the cache
func (c *MultiLevelCache) SetContext(ctx context.Context, contextID string, context *mcp.Context) error {
	key := fmt.Sprintf("context:%s", contextID)
	return c.Set(ctx, key, context, c.ttl)
}

// GetContext retrieves a context from the cache
func (c *MultiLevelCache) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	key := fmt.Sprintf("context:%s", contextID)
	var context mcp.Context
	found, err := c.Get(ctx, key, &context)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	
	// Prefetch related contexts
	c.prefetchRelatedContexts(context)
	
	return &context, nil
}

// DeleteContext removes a context from the cache
func (c *MultiLevelCache) DeleteContext(ctx context.Context, contextID string) error {
	key := fmt.Sprintf("context:%s", contextID)
	return c.Delete(ctx, key)
}

// prefetchRelatedContexts prefetches related contexts
func (c *MultiLevelCache) prefetchRelatedContexts(context mcp.Context) {
	// Prefetch contexts from the same agent
	if context.AgentID != "" {
		key := fmt.Sprintf("contexts:agent:%s", context.AgentID)
		c.queuePrefetch(key, &context, map[string]interface{}{
			"type":     "agent_contexts",
			"agent_id": context.AgentID,
		})
	}
	
	// Prefetch contexts from the same session
	if context.SessionID != "" {
		key := fmt.Sprintf("contexts:session:%s", context.SessionID)
		c.queuePrefetch(key, &context, map[string]interface{}{
			"type":       "session_contexts",
			"session_id": context.SessionID,
		})
	}
	
	// Prefetch contexts from the same model
	if context.ModelID != "" {
		key := fmt.Sprintf("contexts:model:%s", context.ModelID)
		c.queuePrefetch(key, &context, map[string]interface{}{
			"type":     "model_contexts",
			"model_id": context.ModelID,
		})
	}
}
