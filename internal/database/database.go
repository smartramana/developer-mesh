package database

import (
	"context"

	"github.com/jmoiron/sqlx"
)

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

// Repository interface for data access
type Repository interface {
	FindByID(ctx context.Context, id string) (interface{}, error)
	FindAll(ctx context.Context, options QueryOptions) (interface{}, error)
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
