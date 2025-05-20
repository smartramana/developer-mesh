// Package database provides database access functionality for the MCP system.
package database

import (
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/config"
)

// Config is an alias for the common DatabaseConfig, extended with RDS-specific fields
type Config struct {
	config.DatabaseConfig

	// Direct access to common fields needed by application code
	Driver           string        `yaml:"driver" mapstructure:"driver"`
	MaxOpenConns     int           `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxIdleConns     int           `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	ConnMaxLifetime  time.Duration `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`

	// UseAWS determines whether to use AWS services for database connections
	UseAWS *bool `yaml:"use_aws" mapstructure:"use_aws"`

	// UseIAM determines whether to use IAM authentication for database connections
	UseIAM *bool `yaml:"use_iam" mapstructure:"use_iam"`

	// RDSConfig contains RDS-specific configuration
	RDSConfig *config.RDSConfig `yaml:"rds_config" mapstructure:"rds_config"`

	// AutoMigrate determines whether to automatically run database migrations
	AutoMigrate *bool `yaml:"auto_migrate" mapstructure:"auto_migrate"`

	// FailOnMigrationError determines whether to fail if migration errors occur
	FailOnMigrationError *bool `yaml:"fail_on_migration_error" mapstructure:"fail_on_migration_error"`

	// MigrationsPath specifies the path to the database migration files
	MigrationsPath string `yaml:"migrations_path" mapstructure:"migrations_path"`
}

// RDSConfig contains AWS RDS configuration
type RDSConfig = config.RDSConfig

// GetDefaultConfig returns a default database configuration
func GetDefaultConfig() Config {
	defaultDbConfig := config.GetDefaultDatabaseConfig()
	useAWS := true
	useIAM := true
	autoMigrate := true

	return Config{
		DatabaseConfig: defaultDbConfig,
		// Direct access fields
		Driver:          defaultDbConfig.Driver,
		MaxOpenConns:    defaultDbConfig.MaxOpenConns,
		MaxIdleConns:    defaultDbConfig.MaxIdleConns,
		ConnMaxLifetime: defaultDbConfig.ConnMaxLifetime,
		// Other fields
		UseAWS:         &useAWS,
		UseIAM:         &useIAM,
		AutoMigrate:    &autoMigrate,
		MigrationsPath: "migrations",
		RDSConfig:      &config.RDSConfig{},
	}
}

// FromDatabaseConfig converts a config.DatabaseConfig to a database.Config
// This is needed when applications use the common config package but need to pass
// values to functions expecting the database.Config type
func FromDatabaseConfig(dbConfig config.DatabaseConfig) Config {
	useAWS := false
	useIAM := false
	autoMigrate := false
	
	return Config{
		DatabaseConfig: dbConfig,
		// Copy all the fields directly for direct access
		Driver:          dbConfig.Driver,
		MaxOpenConns:    dbConfig.MaxOpenConns,
		MaxIdleConns:    dbConfig.MaxIdleConns,
		ConnMaxLifetime: dbConfig.ConnMaxLifetime,
		// Set the pointer fields with default values
		UseAWS:         &useAWS,
		UseIAM:         &useIAM,
		AutoMigrate:    &autoMigrate,
		MigrationsPath: "migrations",
	}
}
