// Package cache provides stub implementations for cache-related functionality
package cache

import (
	"context"
	"time"
)

// Cache interface defines the operations for a caching system
type Cache interface {
	// Get retrieves data from the cache
	Get(ctx context.Context, key string, value interface{}) error
	// Set stores data in the cache
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	// Delete removes data from the cache
	Delete(ctx context.Context, key string) error
	// Exists checks if a key exists in the cache
	Exists(ctx context.Context, key string) (bool, error)
	// Flush clears all data from the cache
	Flush(ctx context.Context) error
	// Close closes the cache connection
	Close() error
}

// Error represents a cache-related error
type Error struct {
	Message string
}

// Error implements the error interface
func (e Error) Error() string {
	return e.Message
}

// Note: RedisConfig is now fully defined in init.go to prevent redeclaration errors

// stubCache is a simple stub implementation of the Cache interface
type stubCache struct {}

func (s *stubCache) Get(ctx context.Context, key string, value interface{}) error {
	return nil // Stub implementation
}

func (s *stubCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil // Stub implementation
}

func (s *stubCache) Delete(ctx context.Context, key string) error {
	return nil // Stub implementation
}

func (s *stubCache) Close() error {
	return nil // Stub implementation
}

func (s *stubCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil // Stub implementation
}

func (s *stubCache) Flush(ctx context.Context) error {
	return nil // Stub implementation
}

// Note: NewCache is now fully implemented in init.go to prevent redeclaration errors

// ConvertFromCommonRedisConfig converts a common/cache.RedisConfig to our RedisConfig
// Kept for compatibility with external packages that might use this function
func ConvertFromCommonRedisConfig(commonConfig interface{}) RedisConfig {
	// Create a default config with sensible values
	return RedisConfig{
		Type:         "redis",
		Address:      "localhost:6379", // Default
		DialTimeout:  time.Second * 5,
		ReadTimeout:  time.Second * 3,
		WriteTimeout: time.Second * 3,
		PoolSize:     10,
		MinIdleConns: 2,
		PoolTimeout:  3, // seconds
		MaxRetries:   3,
	}
}
