package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// ErrNotFound is returned when a key is not found in the cache
var ErrNotFound = errors.New("key not found in cache")

// Cache interface defines caching operations
type Cache interface {
	Get(ctx context.Context, key string, value any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Flush(ctx context.Context) error
	Close() error
}

// RedisCache implements Cache using Redis
type RedisCache struct {
	client *redis.Client
	config RedisConfig
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(cfg RedisConfig) (*RedisCache, error) {
	options := &redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.Database,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		PoolTimeout:  time.Duration(cfg.PoolTimeout) * time.Second,
	}

	// Configure TLS if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig, err := cfg.TLS.BuildTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
		if tlsConfig != nil {
			options.TLSConfig = tlsConfig
		}
	}

	client := redis.NewClient(options)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{
		client: client,
		config: cfg,
	}, nil
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string, value any) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrNotFound
		}
		return err
	}

	return json.Unmarshal(data, value)
}

// Set stores a value in cache with TTL
func (c *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a value from cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists checks if a key exists in cache
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// Flush clears all values in cache
func (c *RedisCache) Flush(ctx context.Context) error {
	return c.client.FlushAll(ctx).Err()
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// MetricsRecorder interface for optional metrics
type MetricsRecorder interface {
	RecordCacheHit(key string)
	RecordCacheMiss(key string)
	RecordCacheError(key string, err error)
}

// CacheWarmer interface for cache warming strategies
type CacheWarmer interface {
	// WarmCache pre-populates the cache with frequently accessed data
	WarmCache(ctx context.Context) error
	// GetWarmupKeys returns keys that should be warmed
	GetWarmupKeys() []string
}

// WarmableCacheImpl adds warming capabilities to any cache
type WarmableCacheImpl struct {
	Cache
	warmer CacheWarmer
	logger interface { // Simple logger interface to avoid import
		Info(msg string, fields map[string]any)
		Error(msg string, fields map[string]any)
	}
}

// NewWarmableCache wraps a cache with warming capabilities
func NewWarmableCache(cache Cache, warmer CacheWarmer, logger interface {
	Info(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
}) *WarmableCacheImpl {
	return &WarmableCacheImpl{
		Cache:  cache,
		warmer: warmer,
		logger: logger,
	}
}

// StartWarmup initiates cache warming in the background
func (w *WarmableCacheImpl) StartWarmup(ctx context.Context) {
	go func() {
		if err := w.warmer.WarmCache(ctx); err != nil {
			w.logger.Error("cache warmup failed", map[string]any{"error": err.Error()})
		} else {
			w.logger.Info("cache warmup completed", map[string]any{"keys": len(w.warmer.GetWarmupKeys())})
		}
	}()
}
