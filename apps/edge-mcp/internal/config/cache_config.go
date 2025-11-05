package config

import (
	"os"
	"strconv"
	"time"
)

// CacheConfig represents cache configuration for Edge MCP
type CacheConfig struct {
	// L1 Memory Cache (always enabled)
	L1MaxItems int           `yaml:"l1_max_items" json:"l1_max_items"`
	L1TTL      time.Duration `yaml:"l1_ttl" json:"l1_ttl"`

	// L2 Redis Cache (optional)
	RedisEnabled        bool          `yaml:"redis_enabled" json:"redis_enabled"`
	RedisAddr           string        `yaml:"redis_addr" json:"redis_addr"`           // host:port format (e.g., localhost:6379)
	RedisPassword       string        `yaml:"redis_password" json:"redis_password"`   // Optional password
	RedisDB             int           `yaml:"redis_db" json:"redis_db"`               // Database number (default: 0)
	RedisConnectTimeout time.Duration `yaml:"redis_connect_timeout" json:"redis_connect_timeout"`
	RedisFallbackMode   bool          `yaml:"redis_fallback_mode" json:"redis_fallback_mode"`
	L2TTL               time.Duration `yaml:"l2_ttl" json:"l2_ttl"`

	// Compression
	EnableCompression    bool `yaml:"enable_compression" json:"enable_compression"`
	CompressionThreshold int  `yaml:"compression_threshold" json:"compression_threshold"`
}

// LoadCacheConfig loads cache configuration from environment variables
func LoadCacheConfig() *CacheConfig {
	return &CacheConfig{
		// L1 defaults
		L1MaxItems: getEnvInt("EDGE_MCP_L1_MAX_ITEMS", 10000),
		L1TTL:      getEnvDuration("EDGE_MCP_L1_TTL", 5*time.Minute),

		// L2 defaults - Using standard Redis env vars like all other services
		RedisEnabled:        getEnvBool("EDGE_MCP_REDIS_ENABLED", false),
		RedisAddr:           getEnvString("REDIS_ADDR", "localhost:6379"),
		RedisPassword:       getEnvString("REDIS_PASSWORD", ""),
		RedisDB:             getEnvInt("EDGE_MCP_REDIS_DB", 0),
		RedisConnectTimeout: getEnvDuration("EDGE_MCP_REDIS_CONNECT_TIMEOUT", 5*time.Second),
		RedisFallbackMode:   getEnvBool("EDGE_MCP_REDIS_FALLBACK_MODE", true),
		L2TTL:               getEnvDuration("EDGE_MCP_L2_TTL", 1*time.Hour),

		// Compression defaults
		EnableCompression:    getEnvBool("EDGE_MCP_ENABLE_COMPRESSION", true),
		CompressionThreshold: getEnvInt("EDGE_MCP_COMPRESSION_THRESHOLD", 1024),
	}
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		L1MaxItems:           10000,
		L1TTL:                5 * time.Minute,
		RedisEnabled:         false,
		RedisAddr:            "localhost:6379",
		RedisPassword:        "",
		RedisDB:              0,
		RedisConnectTimeout:  5 * time.Second,
		RedisFallbackMode:    true,
		L2TTL:                1 * time.Hour,
		EnableCompression:    true,
		CompressionThreshold: 1024,
	}
}

// Helper functions for environment variable parsing

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		// Handle common yes/no strings explicitly
		switch value {
		case "yes", "Yes", "YES", "y", "Y":
			return true
		case "no", "No", "NO", "n", "N":
			return false
		}
		// Fall back to ParseBool for standard boolean strings (true, false, 1, 0, etc.)
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
