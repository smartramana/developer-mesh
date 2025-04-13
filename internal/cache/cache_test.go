package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMiniRedis creates a test Redis server using miniredis
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, string) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	return mr, mr.Addr()
}

// TestItem is a test struct for marshal/unmarshal operations
type TestItem struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestNewRedisCache(t *testing.T) {
	mr, addr := setupMiniRedis(t)
	defer mr.Close()

	t.Run("Successful Connection", func(t *testing.T) {
		config := RedisConfig{
			Address:  addr,
			Password: "",
			Database: 0,
		}
		
		cache, err := NewRedisCache(config)
		require.NoError(t, err)
		require.NotNil(t, cache)
		assert.NotNil(t, cache.client)
		
		// Clean up
		err = cache.Close()
		assert.NoError(t, err)
	})

	t.Run("With Password", func(t *testing.T) {
		mr.RequireAuth("testpassword")
		
		// Should fail without password
		config := RedisConfig{
			Address:  addr,
			Password: "",
			Database: 0,
		}
		
		cache, err := NewRedisCache(config)
		assert.Error(t, err)
		
		// Should succeed with password
		config.Password = "testpassword"
		cache, err = NewRedisCache(config)
		require.NoError(t, err)
		require.NotNil(t, cache)
		
		// Clean up
		err = cache.Close()
		assert.NoError(t, err)
		
		// Reset auth for subsequent tests
		mr.RequireAuth("")
	})

	t.Run("With Database Index", func(t *testing.T) {
		config := RedisConfig{
			Address:  addr,
			Password: "",
			Database: 1,
		}
		
		cache, err := NewRedisCache(config)
		require.NoError(t, err)
		require.NotNil(t, cache)
		
		// Clean up
		err = cache.Close()
		assert.NoError(t, err)
	})

	t.Run("Invalid Address", func(t *testing.T) {
		config := RedisConfig{
			Address:  "invalid:6379",
			Password: "",
			Database: 0,
		}
		
		cache, err := NewRedisCache(config)
		assert.Error(t, err)
		assert.Nil(t, cache)
	})
}

func TestCacheOperations(t *testing.T) {
	mr, addr := setupMiniRedis(t)
	defer mr.Close()

	config := RedisConfig{
		Address:  addr,
		Password: "",
		Database: 0,
	}
	
	cache, err := NewRedisCache(config)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	t.Run("Set and Get", func(t *testing.T) {
		key := "test:key"
		value := TestItem{ID: 1, Name: "test", Value: 42}
		
		// Set the value
		err := cache.Set(ctx, key, value, 1*time.Hour)
		assert.NoError(t, err)
		
		// Get the value
		var result TestItem
		err = cache.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("Get Non-Existent Key", func(t *testing.T) {
		key := "non:existent:key"
		
		var result TestItem
		err := cache.Get(ctx, key, &result)
		assert.Error(t, err)
		assert.True(t, err == ErrNotFound)
	})

	t.Run("Exists", func(t *testing.T) {
		key := "exists:test:key"
		value := TestItem{ID: 2, Name: "exists test", Value: 100}
		
		// Key should not exist initially
		exists, err := cache.Exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists)
		
		// Set the value
		err = cache.Set(ctx, key, value, 1*time.Hour)
		assert.NoError(t, err)
		
		// Key should now exist
		exists, err = cache.Exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Delete", func(t *testing.T) {
		key := "delete:test:key"
		value := TestItem{ID: 3, Name: "delete test", Value: 200}
		
		// Set the value
		err := cache.Set(ctx, key, value, 1*time.Hour)
		assert.NoError(t, err)
		
		// Verify it exists
		exists, err := cache.Exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)
		
		// Delete the key
		err = cache.Delete(ctx, key)
		assert.NoError(t, err)
		
		// Verify it no longer exists
		exists, err = cache.Exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Flush", func(t *testing.T) {
		// Set multiple keys
		err := cache.Set(ctx, "flush:key1", TestItem{ID: 4}, 1*time.Hour)
		assert.NoError(t, err)
		err = cache.Set(ctx, "flush:key2", TestItem{ID: 5}, 1*time.Hour)
		assert.NoError(t, err)
		
		// Verify keys exist
		exists, err := cache.Exists(ctx, "flush:key1")
		assert.NoError(t, err)
		assert.True(t, exists)
		
		// Flush the cache
		err = cache.Flush(ctx)
		assert.NoError(t, err)
		
		// Verify keys no longer exist
		exists, err = cache.Exists(ctx, "flush:key1")
		assert.NoError(t, err)
		assert.False(t, exists)
		
		exists, err = cache.Exists(ctx, "flush:key2")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Expiration", func(t *testing.T) {
		key := "expiring:key"
		value := TestItem{ID: 6, Name: "expiring", Value: 300}
		
		// Set with a short expiration
		err := cache.Set(ctx, key, value, 100*time.Millisecond)
		assert.NoError(t, err)
		
		// Key should exist initially
		exists, err := cache.Exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)
		
		// Manually set expiry in miniredis
		mr.FastForward(200 * time.Millisecond)
		
		// Key should no longer exist
		exists, err = cache.Exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}
