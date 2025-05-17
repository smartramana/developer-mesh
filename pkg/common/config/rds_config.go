package config

import (
	"time"
)

// RDSConfig holds AWS RDS configuration
type RDSConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	Database          string        `mapstructure:"database"`
	Username          string        `mapstructure:"username"`
	Password          string        `mapstructure:"password"`
	UseIAMAuth        bool          `mapstructure:"use_iam_auth"`
	TokenExpiration   int           `mapstructure:"token_expiration"`
	MaxOpenConns      int           `mapstructure:"max_open_conns"`
	MaxIdleConns      int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `mapstructure:"conn_max_lifetime"`
	EnablePooling     bool          `mapstructure:"enable_pooling"`
	MinPoolSize       int           `mapstructure:"min_pool_size"`
	MaxPoolSize       int           `mapstructure:"max_pool_size"`
	ConnectionTimeout int           `mapstructure:"connection_timeout"`
	AuthConfig        struct {
		Region     string `mapstructure:"region"`
		Endpoint   string `mapstructure:"endpoint"`
		AssumeRole string `mapstructure:"assume_role"`
	} `mapstructure:"auth"`
}

// GetDefaultRDSConfig returns default RDS configuration
func GetDefaultRDSConfig() *RDSConfig {
	return &RDSConfig{
		Host:              "localhost",
		Port:              5432,
		Database:          "mcp",
		Username:          "postgres",
		Password:          "",
		UseIAMAuth:        false,
		TokenExpiration:   900,
		MaxOpenConns:      25,
		MaxIdleConns:      5,
		ConnMaxLifetime:   5 * time.Minute,
		EnablePooling:     false,
		MinPoolSize:       5,
		MaxPoolSize:       25,
		ConnectionTimeout: 10,
		AuthConfig: struct {
			Region     string `mapstructure:"region"`
			Endpoint   string `mapstructure:"endpoint"`
			AssumeRole string `mapstructure:"assume_role"`
		}{
			Region:     "us-west-2",
			Endpoint:   "",
			AssumeRole: "",
		},
	}
}
