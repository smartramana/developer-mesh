package config

import (
	"time"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver           string               `yaml:"driver"`
	DSN              string               `yaml:"dsn"`
	Host             string               `yaml:"host"`
	Port             int                  `yaml:"port"`
	Username         string               `yaml:"username"`
	Password         string               `yaml:"password"`
	Database         string               `yaml:"database"`
	SSLMode          string               `yaml:"ssl_mode"`
	MaxOpenConns     int                  `yaml:"max_open_conns"`
	MaxIdleConns     int                  `yaml:"max_idle_conns"`
	ConnMaxLifetime  time.Duration        `yaml:"conn_max_lifetime"`
	Vector           DatabaseVectorConfig `yaml:"vector"`
}

// DatabaseVectorConfig holds configuration for vector database operations
type DatabaseVectorConfig struct {
	Enabled    bool                    `yaml:"enabled"`
	IndexType  string                  `yaml:"index_type"`
	Lists      int                     `yaml:"lists"`
	Probes     int                     `yaml:"probes"`
	Pool       DatabaseVectorPoolConfig `yaml:"pool"`
}

// DatabaseVectorPoolConfig holds configuration for the vector database connection pool
type DatabaseVectorPoolConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// GetDefaultDatabaseConfig returns default database configuration
func GetDefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Vector: DatabaseVectorConfig{
			Enabled:   true,
			IndexType: "ivfflat",
			Lists:     100,
			Probes:    10,
			Pool: DatabaseVectorPoolConfig{
				Enabled:         false,
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 10 * time.Minute,
			},
		},
	}
}
