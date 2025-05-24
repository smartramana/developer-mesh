// Package config provides database configuration structures for the MCP server
package config

import (
	"time"
)

// DatabaseVectorPoolConfig defines connection pool settings for database vector operations
type DatabaseVectorPoolConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// DatabaseVectorConfig defines the configuration for database vector operations
type DatabaseVectorConfig struct {
	Enabled         bool                    `yaml:"enabled"`
	IndexType       string                  `yaml:"index_type"`
	Lists           int                     `yaml:"lists"`
	Probes          int                     `yaml:"probes"`
	Dimensions      int                     `yaml:"dimensions"`
	SimilarityMetric string                 `yaml:"similarity_metric"`
	Pool            DatabaseVectorPoolConfig `yaml:"pool"`
}
