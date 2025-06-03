package core

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// GeneralDatabaseAdapter provides a wrapper around pkg/database.Database with the same interface as internal/database.Database
// This allows for incremental migration from internal/database to pkg/database
// Deprecated: This adapter is part of the migration strategy and will be removed when migration is complete.
// Use pkg/database.Database directly instead.
type GeneralDatabaseAdapter struct {
	// Deprecated: Field is unused as part of the migration strategy and will be removed in future versions
	db     *sqlx.DB
	// Deprecated: Field is unused as part of the migration strategy and will be removed in future versions
	pkgDB  *database.Database
	// Deprecated: Field is unused as part of the migration strategy and will be removed in future versions
	config config.DatabaseConfig
	// Deprecated: Field is unused as part of the migration strategy and will be removed in future versions
	logger *observability.Logger
}

// NewDatabaseFromPkg creates a new database.Database instance that wraps a pkg/database.Database
func NewDatabaseFromPkg(ctx context.Context, cfg config.DatabaseConfig, logger observability.Logger) (*database.Database, error) {
	// We don't need a logger adapter since we're using the same interface

	// Convert config to database.Config format
	pkgConfig := database.Config{
		Driver:       cfg.Driver,
		Host:         cfg.Host,
		Port:         cfg.Port,
		Database:     cfg.Database,
		Username:     cfg.Username,
		Password:     cfg.Password,
		SSLMode:      cfg.SSLMode,
		MaxOpenConns: cfg.MaxOpenConns,
		MaxIdleConns: cfg.MaxIdleConns,
		UseAWS:       false, // Will be set below based on flags
		UseIAM:       cfg.UseIAM,
	}

	// UseAWS and UseIAM are already set in the config above

	// Create the pkg database instance
	pkgDatabase, err := database.NewDatabase(ctx, pkgConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pkg database: %w", err)
	}

	return pkgDatabase, nil
}

// We're now using a consistent Logger interface across packages,
// so we don't need a logger adapter
