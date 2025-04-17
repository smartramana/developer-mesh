package database

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/mcp-server/internal/aws"
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
	
	// AWS RDS specific configuration
	UseAWS   *bool          `mapstructure:"use_aws"` // Pointer to allow nil (unspecified) value
	UseIAM   *bool          `mapstructure:"use_iam"` // Pointer to allow nil (unspecified) value
	RDSConfig *aws.RDSConfig `mapstructure:"rds"`
}

// Database represents the database access layer
type Database struct {
	db         *sqlx.DB
	config     Config
	statements map[string]*sqlx.Stmt
	rdsClient  *aws.RDSClient
}

// NewDatabase creates a new database connection
func NewDatabase(ctx context.Context, cfg Config) (*Database, error) {
	var dsn string
	var err error
	var rdsClient *aws.RDSClient
	
	// Default to AWS RDS with IAM authentication unless explicitly disabled
	useAWS := true
	useIAM := true
	
	if cfg.UseAWS != nil {
		useAWS = *cfg.UseAWS
	}
	
	if cfg.UseIAM != nil {
		useIAM = *cfg.UseIAM
	}
	
	// If we're using AWS RDS with IAM authentication (the default and recommended approach)
	if useAWS && useIAM && cfg.RDSConfig != nil {
		// Initialize the RDS client
		rdsClient, err = aws.NewRDSClient(ctx, *cfg.RDSConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize RDS client: %w", err)
		}
		
		// Get DSN with IAM authentication
		dsn, err = rdsClient.BuildPostgresConnectionString(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to build PostgreSQL connection string with IAM auth: %w", err)
		}
	} else if cfg.DSN != "" {
		// Use provided DSN (fallback method)
		log.Println("Warning: Using explicit DSN for database connection instead of IAM authentication")
		dsn = cfg.DSN
	} else {
		// Build DSN from individual components (least recommended option)
		log.Println("Warning: Building database connection string from components instead of using IAM authentication")
		
		sslMode := cfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		
		// Check that password is not empty when not using IAM auth
		if !useIAM && cfg.Password == "" {
			return nil, fmt.Errorf("password is required when not using IAM authentication")
		}
		
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, sslMode)
	}
	
	// Connect to the database
	db, err := sqlx.ConnectContext(ctx, cfg.Driver, dsn)
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
		rdsClient:  rdsClient,
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

// RefreshConnection refreshes the database connection (especially for IAM auth)
func (d *Database) RefreshConnection(ctx context.Context) error {
	// If we're using AWS RDS with IAM authentication, we need to refresh the token
	if d.config.UseAWS && d.config.UseIAM && d.rdsClient != nil {
		// Get fresh DSN with a new IAM authentication token
		dsn, err := d.rdsClient.BuildPostgresConnectionString(ctx)
		if err != nil {
			return fmt.Errorf("failed to build PostgreSQL connection string: %w", err)
		}
		
		// Close existing connection
		if err := d.Close(); err != nil {
			return fmt.Errorf("failed to close existing database connection: %w", err)
		}
		
		// Create new connection
		db, err := sqlx.ConnectContext(ctx, d.config.Driver, dsn)
		if err != nil {
			return fmt.Errorf("failed to create new database connection: %w", err)
		}
		
		// Configure connection pool
		db.SetMaxOpenConns(d.config.MaxOpenConns)
		db.SetMaxIdleConns(d.config.MaxIdleConns)
		db.SetConnMaxLifetime(d.config.ConnMaxLifetime)
		
		// Update database instance
		d.db = db
		
		// Prepare statements
		if err := d.prepareStatements(ctx); err != nil {
			db.Close()
			return fmt.Errorf("failed to prepare statements: %w", err)
		}
	}
	
	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	// Close all prepared statements
	for _, stmt := range d.statements {
		stmt.Close()
	}

	// Clear statements map
	d.statements = make(map[string]*sqlx.Stmt)

	// Close database connection
	return d.db.Close()
}

// Ping checks if the database connection is alive
func (d *Database) Ping() error {
	return d.db.Ping()
}

// GetDB returns the underlying sqlx.DB instance
func (d *Database) GetDB() *sqlx.DB {
	return d.db
}
