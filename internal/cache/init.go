package cache

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a key is not found in the cache
var ErrNotFound = errors.New("key not found in cache")

// RedisConfig holds configuration for Redis
type RedisConfig struct {
	Address      string
	Password     string
	Database     int
	MaxRetries   int
	DialTimeout  int
	ReadTimeout  int
	WriteTimeout int
	PoolSize     int
	MinIdleConns int
	PoolTimeout  int
}

// NewCache creates a new cache based on the configuration
func NewCache(ctx context.Context, cfg interface{}) (Cache, error) {
	switch config := cfg.(type) {
	case RedisConfig:
		return NewRedisCache(config)
	default:
		return nil, fmt.Errorf("unsupported cache type: %T", cfg)
	}
}
