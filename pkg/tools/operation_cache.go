package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/redis/go-redis/v9"
)

// OperationCache provides multi-level caching for resolved operations
// This is completely tool-agnostic and works with ANY API
type OperationCache struct {
	redis  *redis.Client
	logger observability.Logger

	// In-memory L1 cache for hot paths
	memCache map[string]*CachedOperation
	memTTL   time.Duration
}

// NewOperationCache creates a new operation cache
func NewOperationCache(redis *redis.Client, logger observability.Logger) *OperationCache {
	return &OperationCache{
		redis:    redis,
		logger:   logger,
		memCache: make(map[string]*CachedOperation),
		memTTL:   5 * time.Minute, // Short TTL for memory cache
	}
}

// CachedOperation represents a cached operation resolution
type CachedOperation struct {
	OperationID   string    `json:"operation_id"`
	Path          string    `json:"path"`
	Method        string    `json:"method"`
	ResolvedAt    time.Time `json:"resolved_at"`
	ResolutionMs  int64     `json:"resolution_ms"`
	ContextHash   string    `json:"context_hash"`
	Score         int       `json:"score"`
	HitCount      int       `json:"hit_count"`
	LastHit       time.Time `json:"last_hit"`
	ResourceScope string    `json:"resource_scope,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
}

// GetResolved retrieves a cached resolution if available
func (c *OperationCache) GetResolved(
	ctx context.Context,
	toolID string,
	action string,
	context map[string]interface{},
) (*CachedOperation, error) {
	// Generate cache key
	cacheKey := c.generateCacheKey(toolID, action, context)

	// Check L1 memory cache first
	if cached := c.getFromMemory(cacheKey); cached != nil {
		c.logger.Debug("Operation cache hit (memory)", map[string]interface{}{
			"tool_id":      toolID,
			"action":       action,
			"operation_id": cached.OperationID,
			"hit_count":    cached.HitCount,
		})
		return cached, nil
	}

	// Check L2 Redis cache
	if c.redis != nil {
		cached, err := c.getFromRedis(ctx, cacheKey)
		if err == nil && cached != nil {
			// Promote to memory cache
			c.setInMemory(cacheKey, cached)

			c.logger.Debug("Operation cache hit (Redis)", map[string]interface{}{
				"tool_id":      toolID,
				"action":       action,
				"operation_id": cached.OperationID,
				"hit_count":    cached.HitCount,
			})
			return cached, nil
		}
	}

	return nil, nil
}

// SetResolved caches a successful operation resolution
func (c *OperationCache) SetResolved(
	ctx context.Context,
	toolID string,
	action string,
	context map[string]interface{},
	operation *ResolvedOperation,
	score int,
	resolutionMs int64,
) error {
	// Generate cache key
	cacheKey := c.generateCacheKey(toolID, action, context)
	contextHash := c.hashContext(context)

	// Create cached entry
	cached := &CachedOperation{
		OperationID:  operation.OperationID,
		Path:         operation.Path,
		Method:       operation.Method,
		ResolvedAt:   time.Now(),
		ResolutionMs: resolutionMs,
		ContextHash:  contextHash,
		Score:        score,
		HitCount:     0,
		LastHit:      time.Now(),
		Tags:         operation.Tags,
	}

	// Extract resource scope if present
	if scope, ok := context["__resource_type"].(string); ok {
		cached.ResourceScope = scope
	}

	// Set in both caches
	c.setInMemory(cacheKey, cached)

	if c.redis != nil {
		if err := c.setInRedis(ctx, cacheKey, cached); err != nil {
			c.logger.Warn("Failed to cache operation in Redis", map[string]interface{}{
				"error":     err.Error(),
				"cache_key": cacheKey,
			})
		}
	}

	c.logger.Debug("Cached operation resolution", map[string]interface{}{
		"tool_id":      toolID,
		"action":       action,
		"operation_id": cached.OperationID,
		"cache_key":    cacheKey,
		"score":        score,
	})

	return nil
}

// InvalidateToolCache invalidates all cached operations for a tool
func (c *OperationCache) InvalidateToolCache(ctx context.Context, toolID string) error {
	pattern := fmt.Sprintf("op_cache:%s:*", toolID)

	// Clear from memory
	for key := range c.memCache {
		if strings.HasPrefix(key, pattern) {
			delete(c.memCache, key)
		}
	}

	// Clear from Redis
	if c.redis != nil {
		iter := c.redis.Scan(ctx, 0, pattern, 0).Iterator()
		var keys []string
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		if len(keys) > 0 {
			if err := c.redis.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("failed to invalidate Redis cache: %w", err)
			}
		}
	}

	c.logger.Info("Invalidated tool cache", map[string]interface{}{
		"tool_id": toolID,
		"pattern": pattern,
	})

	return nil
}

// generateCacheKey creates a unique cache key for an operation resolution
func (c *OperationCache) generateCacheKey(toolID string, action string, context map[string]interface{}) string {
	// Create a deterministic context fingerprint
	contextHash := c.hashContext(context)

	// Include resource scope in key if present
	resourceScope := ""
	if scope, ok := context["__resource_type"].(string); ok {
		resourceScope = scope
	}

	if resourceScope != "" {
		return fmt.Sprintf("op_cache:%s:%s:%s:%s", toolID, resourceScope, action, contextHash)
	}

	return fmt.Sprintf("op_cache:%s:%s:%s", toolID, action, contextHash)
}

// hashContext creates a deterministic hash of the context
func (c *OperationCache) hashContext(context map[string]interface{}) string {
	// Extract and sort parameter names (excluding internal ones)
	var params []string
	for key := range context {
		if !strings.HasPrefix(key, "__") {
			params = append(params, key)
		}
	}
	sort.Strings(params)

	// Create a fingerprint based on parameter presence
	// We don't include values to allow some flexibility
	fingerprint := strings.Join(params, ",")

	// Add critical parameter values that affect resolution
	criticalParams := []string{"owner", "repo", "org", "user", "id", "name"}
	var criticalValues []string
	for _, param := range criticalParams {
		if value, exists := context[param]; exists {
			// Only include string values for determinism
			if strVal, ok := value.(string); ok {
				criticalValues = append(criticalValues, fmt.Sprintf("%s=%s", param, strVal))
			}
		}
	}

	if len(criticalValues) > 0 {
		fingerprint += ":" + strings.Join(criticalValues, ",")
	}

	// Hash the fingerprint
	hash := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter keys
}

// getFromMemory retrieves from memory cache
func (c *OperationCache) getFromMemory(key string) *CachedOperation {
	if cached, exists := c.memCache[key]; exists {
		// Check TTL
		if time.Since(cached.ResolvedAt) < c.memTTL {
			cached.HitCount++
			cached.LastHit = time.Now()
			return cached
		}
		// Expired, remove from cache
		delete(c.memCache, key)
	}
	return nil
}

// setInMemory stores in memory cache
func (c *OperationCache) setInMemory(key string, operation *CachedOperation) {
	// Simple size limit - keep last 1000 entries
	if len(c.memCache) > 1000 {
		// Remove oldest entry (simple strategy)
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.memCache {
			if oldestKey == "" || v.ResolvedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.ResolvedAt
			}
		}
		delete(c.memCache, oldestKey)
	}

	c.memCache[key] = operation
}

// getFromRedis retrieves from Redis cache
func (c *OperationCache) getFromRedis(ctx context.Context, key string) (*CachedOperation, error) {
	data, err := c.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, err
	}

	var cached CachedOperation
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached operation: %w", err)
	}

	// Update hit statistics
	cached.HitCount++
	cached.LastHit = time.Now()

	// Update in Redis (async)
	go func() {
		if data, err := json.Marshal(cached); err == nil {
			c.redis.Set(context.Background(), key, data, 1*time.Hour)
		}
	}()

	return &cached, nil
}

// setInRedis stores in Redis cache
func (c *OperationCache) setInRedis(ctx context.Context, key string, operation *CachedOperation) error {
	data, err := json.Marshal(operation)
	if err != nil {
		return fmt.Errorf("failed to marshal operation: %w", err)
	}

	// Set with TTL based on score and success
	ttl := c.calculateTTL(operation)

	if err := c.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set in Redis: %w", err)
	}

	return nil
}

// calculateTTL determines cache TTL based on operation characteristics
func (c *OperationCache) calculateTTL(operation *CachedOperation) time.Duration {
	// Base TTL
	ttl := 1 * time.Hour

	// Higher score = longer TTL (more confident resolution)
	if operation.Score > 500 {
		ttl = 24 * time.Hour
	} else if operation.Score > 200 {
		ttl = 6 * time.Hour
	} else if operation.Score > 100 {
		ttl = 2 * time.Hour
	}

	// Frequently hit items get longer TTL
	if operation.HitCount > 10 {
		ttl = ttl * 2
	}

	// Cap at 48 hours
	if ttl > 48*time.Hour {
		ttl = 48 * time.Hour
	}

	return ttl
}

// GetCacheStats returns cache statistics
func (c *OperationCache) GetCacheStats(ctx context.Context) map[string]interface{} {
	stats := map[string]interface{}{
		"memory_entries": len(c.memCache),
		"memory_ttl":     c.memTTL.String(),
	}

	// Calculate memory cache hit rate
	totalHits := 0
	for _, cached := range c.memCache {
		totalHits += cached.HitCount
	}
	stats["memory_total_hits"] = totalHits

	// Redis stats if available
	if c.redis != nil {
		// Count Redis entries
		pattern := "op_cache:*"
		iter := c.redis.Scan(ctx, 0, pattern, 0).Iterator()
		count := 0
		for iter.Next(ctx) {
			count++
		}
		stats["redis_entries"] = count
	}

	return stats
}

// CleanupExpired removes expired entries from caches
func (c *OperationCache) CleanupExpired(ctx context.Context) error {
	// Clean memory cache
	now := time.Now()
	for key, cached := range c.memCache {
		if now.Sub(cached.ResolvedAt) > c.memTTL {
			delete(c.memCache, key)
		}
	}

	// Redis handles expiration automatically via TTL

	c.logger.Debug("Cleaned up expired cache entries", map[string]interface{}{
		"remaining_memory_entries": len(c.memCache),
	})

	return nil
}
