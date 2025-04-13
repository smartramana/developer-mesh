package database

import (
	"context"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/models"
	"github.com/jmoiron/sqlx"
)

// Config holds database configuration
type Config struct {
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
}

// Database represents the database access layer
type Database struct {
	db         *sqlx.DB
	config     Config
	statements map[string]*sqlx.Stmt
}

// NewDatabase creates a new database connection
func NewDatabase(ctx context.Context, cfg Config) (*Database, error) {
	db, err := sqlx.ConnectContext(ctx, cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Create database instance
	database := &Database{
		db:         db,
		config:     cfg,
		statements: make(map[string]*sqlx.Stmt),
	}

	// Prepare statements
	if err := database.prepareStatements(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return database, nil
}

// prepareStatements prepares common SQL statements for reuse
func (d *Database) prepareStatements(ctx context.Context) error {
	// Prepare common SQL statements for better performance
	// This is a placeholder implementation - add actual statements as needed
	queries := map[string]string{
		"get_event":       "SELECT * FROM mcp.events WHERE id = $1",
		"insert_event":    "INSERT INTO mcp.events (source, type, data, timestamp) VALUES ($1, $2, $3, $4) RETURNING id",
		"get_context":     "SELECT * FROM mcp.contexts WHERE id = $1",
		"insert_context":  "INSERT INTO mcp.contexts (name, description, data) VALUES ($1, $2, $3) RETURNING id",
		"get_integration": "SELECT * FROM mcp.integrations WHERE id = $1",
	}

	for name, query := range queries {
		stmt, err := d.db.PreparexContext(ctx, query)
		if err != nil {
			return err
		}
		d.statements[name] = stmt
	}

	return nil
}

// Repository interface for data access
type Repository interface {
	FindByID(ctx context.Context, id string) (interface{}, error)
	FindAll(ctx context.Context, options models.QueryOptions) (interface{}, error)
	Create(ctx context.Context, entity interface{}) error
	Update(ctx context.Context, entity interface{}) error
	Delete(ctx context.Context, id string) error
}

// Transaction executes a function within a database transaction
func (d *Database) Transaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // Re-throw panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Close closes the database connection
func (d *Database) Close() error {
	// Close all prepared statements
	for _, stmt := range d.statements {
		stmt.Close()
	}

	// Close database connection
	return d.db.Close()
}

// Ping checks if the database connection is alive
func (d *Database) Ping() error {
	return d.db.Ping()
}
