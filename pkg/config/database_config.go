package config

import (
	"time"

	commonconfig "github.com/S-Corkum/devops-mcp/pkg/common/config"
)

// Import database config types from common/config
// This prevents type redeclaration errors during compilation

// GetDefaultDatabaseConfig returns default database configuration
func GetDefaultDatabaseConfig() DatabaseConfig {
	// Create a default config using the common config package
	vector := commonconfig.DatabaseVectorConfig{
		Enabled:   true,
		IndexType: "ivfflat",
		Lists:     100,
		Probes:    10,
		Pool: commonconfig.DatabaseVectorPoolConfig{
			Enabled:         false,
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 10 * time.Minute,
		},
	}

	return commonconfig.DatabaseConfig{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Vector:          vector,
	}
}
