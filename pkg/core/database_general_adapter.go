package core

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/common/aws"
	commonconfig "github.com/S-Corkum/devops-mcp/pkg/common/config"
	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// GeneralDatabaseAdapter provides a wrapper around pkg/database.Database with the same interface as internal/database.Database
// This allows for incremental migration from internal/database to pkg/database
type GeneralDatabaseAdapter struct {
	db        *sqlx.DB
	pkgDB     *database.Database
	config    config.DatabaseConfig
	logger    *observability.Logger
}

// NewDatabaseFromPkg creates a new database.Database instance that wraps a pkg/database.Database
func NewDatabaseFromPkg(ctx context.Context, cfg config.DatabaseConfig, logger observability.Logger) (*database.Database, error) {
	// We don't need a logger adapter since we're using the same interface

	// Convert config to pkg format using the FromDatabaseConfig function
	// This correctly handles the DatabaseConfig embedding and additional fields
	pkgConfig := database.FromDatabaseConfig(cfg)
	
	// Set additional fields that might not be in the standard config
	useAWS := false
	useIAM := false
	autoMigrate := false
	
	// Set these explicitly since they may not be in the standard config
	pkgConfig.UseAWS = &useAWS
	pkgConfig.UseIAM = &useIAM
	pkgConfig.AutoMigrate = &autoMigrate

	// Create a minimal RDS config
	// Note: In the actual implementation, you would need to check if the application 
	// requires specific RDS configuration and add it here
	pkgConfig.RDSConfig = &commonconfig.RDSConfig{
		Host:            cfg.Host,
		Port:            cfg.Port,
		Database:        cfg.Database,
		Username:        cfg.Username,
		Password:        cfg.Password,
		UseIAMAuth:      false,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	}

	// Add minimal auth config
	pkgConfig.RDSConfig.AuthConfig = aws.AuthConfig{
		Region: "us-west-2", // Default region
	}

	// Create the pkg database instance
	pkgDatabase, err := database.NewDatabase(ctx, pkgConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pkg database: %w", err)
	}

	return pkgDatabase, nil
}

// We're now using a consistent Logger interface across packages,
// so we don't need a logger adapter
