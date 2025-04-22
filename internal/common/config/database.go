package config

import (
	"time"
)

// RDSConfig is a placeholder to avoid circular imports
type RDSConfig struct {
	AuthConfig        struct {
		Region    string `mapstructure:"region"`
		Endpoint  string `mapstructure:"endpoint"`
		AssumeRole string `mapstructure:"assume_role"`
	} `mapstructure:"auth"`
	Host              string     `mapstructure:"host"`
	Port              int        `mapstructure:"port"`
	Database          string     `mapstructure:"database"`
	Username          string     `mapstructure:"username"`
	Password          string     `mapstructure:"password"`
	UseIAMAuth        bool       `mapstructure:"use_iam_auth"`
	TokenExpiration   int        `mapstructure:"token_expiration"`
	MaxOpenConns      int        `mapstructure:"max_open_conns"`
	MaxIdleConns      int        `mapstructure:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `mapstructure:"conn_max_lifetime"`
	EnablePooling     bool       `mapstructure:"enable_pooling"`
	MinPoolSize       int        `mapstructure:"min_pool_size"`
	MaxPoolSize       int        `mapstructure:"max_pool_size"`
	ConnectionTimeout int        `mapstructure:"connection_timeout"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver"`
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`

	// Additional configuration for different database types
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"ssl_mode"`
	
	// AWS RDS specific configuration
	UseAWS    *bool      `mapstructure:"use_aws"` // Pointer to allow nil (unspecified) value
	UseIAM    *bool      `mapstructure:"use_iam"` // Pointer to allow nil (unspecified) value
	RDSConfig *RDSConfig `mapstructure:"rds"`
	
	// Vector database configuration
	Vector DatabaseVectorConfig `mapstructure:"vector"`
}

// DatabaseVectorConfig defines the configuration for vector database operations
type DatabaseVectorConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Dimensions      int    `mapstructure:"dimensions"`
	SimilarityMetric string `mapstructure:"similarity_metric"`
	IndexType       string `mapstructure:"index_type"`
	
	// Pool configuration for vector operations
	Pool struct {
		Enabled         bool          `mapstructure:"enabled"`
		MaxOpenConns    int           `mapstructure:"max_open_conns"`
		MaxIdleConns    int           `mapstructure:"max_idle_conns"`
		ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	} `mapstructure:"pool"`
	
	// Query configuration
	Query struct {
		MaxResults    int     `mapstructure:"max_results"`
		MinScore      float64 `mapstructure:"min_score"`
		BoostRecent   bool    `mapstructure:"boost_recent"`
		BoostFactor   float64 `mapstructure:"boost_factor"`
		BoostWindow   string  `mapstructure:"boost_window"`
		ExactMatch    bool    `mapstructure:"exact_match"`
		EnableFilters bool    `mapstructure:"enable_filters"`
	} `mapstructure:"query"`
}

// GetDefaultDatabaseConfig returns the default database configuration
func GetDefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Vector: DatabaseVectorConfig{
			Enabled:         true,
			Dimensions:      1536,
			SimilarityMetric: "cosine",
		},
	}
}
