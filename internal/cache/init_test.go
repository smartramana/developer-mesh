package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	mr, addr := setupMiniRedis(t)
	defer mr.Close()

	t.Run("Redis Cache", func(t *testing.T) {
		redisConfig := RedisConfig{
			Address:       addr,
			Password:      "",
			Database:      0,
			DialTimeout:   5 * time.Second,
			ReadTimeout:   3 * time.Second,
			WriteTimeout:  3 * time.Second,
		}
		
		ctx := context.Background()
		cache, err := NewCache(ctx, redisConfig)
		require.NoError(t, err)
		require.NotNil(t, cache)
		
		// Verify it's a Redis cache
		_, ok := cache.(*RedisCache)
		assert.True(t, ok, "Expected RedisCache implementation")
		
		// Clean up
		err = cache.Close()
		assert.NoError(t, err)
	})

	t.Run("Unsupported Cache Type", func(t *testing.T) {
		// Use a struct that doesn't match any supported cache types
		type UnsupportedConfig struct {
			Type string
		}
		unsupportedConfig := UnsupportedConfig{
			Type: "unsupported",
		}
		
		ctx := context.Background()
		cache, err := NewCache(ctx, unsupportedConfig)
		assert.Error(t, err)
		assert.Nil(t, cache)
		assert.Contains(t, err.Error(), "unsupported cache type")
	})
}
