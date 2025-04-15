package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/internal/api"
	"github.com/S-Corkum/mcp-server/internal/cache"
	"github.com/S-Corkum/mcp-server/internal/core"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/metrics"
	"github.com/S-Corkum/mcp-server/internal/storage"
	"github.com/spf13/viper"
)

// Config holds the complete application configuration
type Config struct {
	API      api.Config        `mapstructure:"api"`
	Cache    cache.RedisConfig `mapstructure:"cache"`
	Database database.Config   `mapstructure:"database"`
	Engine   core.Config       `mapstructure:"engine"`
	Metrics  metrics.Config    `mapstructure:"metrics"`
	Storage  StorageConfig     `mapstructure:"storage"`
}

// StorageConfig holds configuration for different storage providers
type StorageConfig struct {
	Type             string           `mapstructure:"type"`
	S3               storage.S3Config `mapstructure:"s3"`
	ContextStorage   ContextStorage   `mapstructure:"context_storage"`
}

// ContextStorage configuration for context storage
type ContextStorage struct {
	Provider         string           `mapstructure:"provider"` // "s3", "database", "filesystem"
	S3PathPrefix     string           `mapstructure:"s3_path_prefix"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	// Initialize configuration
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read from config file
	configFile := os.Getenv("MCP_CONFIG_FILE")
	if configFile == "" {
		configFile = "configs/config.yaml"
	}

	v.SetConfigFile(configFile)

	// Read from environment variables prefixed with MCP_
	v.SetEnvPrefix("MCP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config
	if err := v.ReadInConfig(); err != nil {
		// Config file is not required if environment variables are set
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal config
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// API defaults
	v.SetDefault("api.listen_address", ":8080")
	v.SetDefault("api.read_timeout", 30*time.Second)
	v.SetDefault("api.write_timeout", 30*time.Second)
	v.SetDefault("api.idle_timeout", 90*time.Second)
	v.SetDefault("api.base_path", "/api/v1")
	v.SetDefault("api.enable_cors", true)
	v.SetDefault("api.cors_origins", []string{"http://localhost:3000"})
	v.SetDefault("api.log_requests", true)
	
	// TLS defaults (empty means no TLS)
	v.SetDefault("api.tls_cert_file", "")
	v.SetDefault("api.tls_key_file", "")

	// API rate limiting defaults
	v.SetDefault("api.rate_limit.enabled", true)
	v.SetDefault("api.rate_limit.limit", 100)
	v.SetDefault("api.rate_limit.burst", 150)
	v.SetDefault("api.rate_limit.expiration", 1*time.Hour)
	
	// Auth defaults
	v.SetDefault("api.auth.require_auth", true)
	v.SetDefault("api.auth.jwt_expiration", 24*time.Hour)
	v.SetDefault("api.auth.token_renewal_threshold", 1*time.Hour)

	// Database defaults
	v.SetDefault("database.driver", "postgres")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 5*time.Minute)

	// Cache defaults
	v.SetDefault("cache.type", "redis")
	v.SetDefault("cache.address", "localhost:6379")
	v.SetDefault("cache.max_retries", 3)
	v.SetDefault("cache.dial_timeout", 5)
	v.SetDefault("cache.read_timeout", 3)
	v.SetDefault("cache.write_timeout", 3)
	v.SetDefault("cache.pool_size", 10)
	v.SetDefault("cache.min_idle_conns", 2)
	v.SetDefault("cache.pool_timeout", 4)

	// Engine defaults
	v.SetDefault("engine.event_buffer_size", 1000)
	v.SetDefault("engine.concurrency_limit", 5)
	v.SetDefault("engine.event_timeout", 30*time.Second)

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.type", "prometheus")
	v.SetDefault("metrics.push_interval", 10*time.Second)
	
	// Storage defaults
	v.SetDefault("storage.type", "local")
	
	// S3 defaults
	v.SetDefault("storage.s3.region", "us-west-2")
	v.SetDefault("storage.s3.upload_part_size", 5*1024*1024) // 5MB
	v.SetDefault("storage.s3.download_part_size", 5*1024*1024) // 5MB
	v.SetDefault("storage.s3.concurrency", 5)
	v.SetDefault("storage.s3.request_timeout", 30*time.Second)
	
	// Context storage defaults
	v.SetDefault("storage.context_storage.provider", "database") // Default to database
	v.SetDefault("storage.context_storage.s3_path_prefix", "contexts")
}
