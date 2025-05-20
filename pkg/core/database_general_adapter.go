package core

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/config"
	internalDb "github.com/S-Corkum/devops-mcp/pkg/database"
	pkgDb "github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	pkgObs "github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// GeneralDatabaseAdapter provides a wrapper around pkg/database.Database with the same interface as internal/database.Database
// This allows for incremental migration from internal/database to pkg/database
type GeneralDatabaseAdapter struct {
	db        *sqlx.DB
	pkgDB     *pkgDb.Database
	config    config.DatabaseConfig
	logger    *observability.Logger
}

// NewDatabaseFromPkg creates a new internal/database.Database instance that wraps a pkg/database.Database
func NewDatabaseFromPkg(ctx context.Context, cfg config.DatabaseConfig, logger *observability.Logger) (*internalDb.Database, error) {
	// Create an adapter for the pkg/observability.Logger interface
	pkgLogger := &obsLoggerAdapter{internal: logger}

	// Convert config to pkg format
	pkgConfig := pkgDb.Config{
		Driver:               cfg.Driver,
		Host:                 cfg.Host,
		Port:                 cfg.Port,
		Username:             cfg.Username,
		Password:             cfg.Password,
		Database:             cfg.Database,
		SSLMode:              cfg.SSLMode,
		DSN:                  cfg.DSN,
		MaxOpenConns:         cfg.MaxOpenConns,
		MaxIdleConns:         cfg.MaxIdleConns,
		ConnMaxLifetime:      cfg.ConnMaxLifetime,
		UseAWS:               cfg.UseAWS,
		UseIAM:               cfg.UseIAM,
		AutoMigrate:          cfg.AutoMigrate,
		MigrationsPath:       cfg.MigrationsPath,
		FailOnMigrationError: cfg.FailOnMigrationError,
	}

	// Handle RDS config conversion if needed
	if cfg.RDSConfig != nil {
		pkgConfig.RDSConfig = &pkgDb.RDSConfig{
			Host:              cfg.RDSConfig.Host,
			Port:              cfg.RDSConfig.Port,
			Database:          cfg.RDSConfig.Database,
			Username:          cfg.RDSConfig.Username,
			Password:          cfg.RDSConfig.Password,
			UseIAMAuth:        cfg.RDSConfig.UseIAMAuth,
			TokenExpiration:   cfg.RDSConfig.TokenExpiration,
			MaxOpenConns:      cfg.RDSConfig.MaxOpenConns,
			MaxIdleConns:      cfg.RDSConfig.MaxIdleConns,
			ConnMaxLifetime:   cfg.RDSConfig.ConnMaxLifetime,
			EnablePooling:     cfg.RDSConfig.EnablePooling,
			MinPoolSize:       cfg.RDSConfig.MinPoolSize,
			MaxPoolSize:       cfg.RDSConfig.MaxPoolSize,
			ConnectionTimeout: cfg.RDSConfig.ConnectionTimeout,
		}

		if cfg.RDSConfig.AuthConfig != nil {
			pkgConfig.RDSConfig.AuthConfig = &pkgDb.AuthConfig{
				Region:    cfg.RDSConfig.AuthConfig.Region,
				Endpoint:  cfg.RDSConfig.AuthConfig.Endpoint,
				AssumeRole: cfg.RDSConfig.AuthConfig.AssumeRole,
			}
		}
	}

	// Create the pkg database instance
	pkgDatabase, err := pkgDb.NewDatabase(ctx, pkgConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pkg database: %w", err)
	}

	// Create a thin internal database instance that uses the underlying sqlx.DB connection
	internalDatabase := internalDb.NewDatabaseWithConnection(pkgDatabase.GetDB())

	return internalDatabase, nil
}

// obsLoggerAdapter adapts an internal/observability.Logger to the pkg/observability.Logger interface
type obsLoggerAdapter struct {
	internal *observability.Logger
}

// Debug logs a debug message
func (a *obsLoggerAdapter) Debug(msg string, keyvals map[string]interface{}) {
	a.internal.Debug(msg, keyvals)
}

// Info logs an info message
func (a *obsLoggerAdapter) Info(msg string, keyvals map[string]interface{}) {
	a.internal.Info(msg, keyvals)
}

// Warn logs a warning message
func (a *obsLoggerAdapter) Warn(msg string, keyvals map[string]interface{}) {
	a.internal.Warn(msg, keyvals)
}

// Error logs an error message
func (a *obsLoggerAdapter) Error(msg string, keyvals map[string]interface{}) {
	a.internal.Error(msg, keyvals)
}

// WithFields returns a new logger with the given fields
func (a *obsLoggerAdapter) WithFields(keyvals map[string]interface{}) pkgObs.Logger {
	// Create a new logger with the fields
	internalLogger := a.internal.WithFields(keyvals)
	return &obsLoggerAdapter{internal: internalLogger}
}

// WithPrefix returns a new logger with the given prefix
func (a *obsLoggerAdapter) WithPrefix(prefix string) pkgObs.Logger {
	// Create a new logger with the prefix
	internalLogger := a.internal.WithPrefix(prefix)
	return &obsLoggerAdapter{internal: internalLogger}
}
