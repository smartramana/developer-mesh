package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/S-Corkum/mcp-server/internal/database/migration"
	_ "github.com/lib/pq"
	"github.com/jmoiron/sqlx"
)

const defaultMigrationsPath = "migrations/sql"

var (
	// Command flags
	createFlag    = flag.Bool("create", false, "Create a new migration")
	upFlag        = flag.Bool("up", false, "Run migrations up")
	downFlag      = flag.Bool("down", false, "Roll back the last migration")
	resetFlag     = flag.Bool("reset", false, "Roll back all migrations")
	versionFlag   = flag.Bool("version", false, "Show current migration version")
	validateFlag  = flag.Bool("validate", false, "Validate migrations without applying them")
	forceFlag     = flag.Int("force", -1, "Force migration version")
	
	// Global flags
	dsn           = flag.String("dsn", "", "Database connection string")
	migrationsDir = flag.String("dir", defaultMigrationsPath, "Migrations directory")
	migrationName = flag.String("name", "", "Migration name (used with -create)")
	steps         = flag.Int("steps", 0, "Number of migrations to apply (0 = all)")
	timeout       = flag.Duration("timeout", 1*time.Minute, "Migration timeout")
	driver        = flag.String("driver", "postgres", "Database driver")
)

func main() {
	flag.Parse()
	
	// Validate flags
	if *createFlag && *migrationName == "" {
		fmt.Println("Error: -name is required when using -create")
		flag.Usage()
		os.Exit(1)
	}
	
	// Create migration (doesn't require database connection)
	if *createFlag {
		if err := migration.CreateMigration(*migrationsDir, *migrationName); err != nil {
			log.Fatalf("Failed to create migration: %v", err)
		}
		return
	}
	
	// For all other commands, we need a database connection
	if *dsn == "" {
		fmt.Println("Error: -dsn is required for all operations except -create")
		flag.Usage()
		os.Exit(1)
	}
	
	// Create database connection
	db, err := sql.Open(*driver, *dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// Check connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	
	// Create sqlx database
	sqlxDB := sqlx.NewDb(db, *driver)
	
	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received termination signal, canceling operations...")
		cancel()
	}()
	
	// Create migration manager
	manager, err := migration.NewManager(sqlxDB, migration.Config{
		MigrationsPath:   *migrationsDir,
		MigrationTimeout: *timeout,
		Steps:            *steps,
	}, *driver)
	if err != nil {
		log.Fatalf("Failed to create migration manager: %v", err)
	}
	defer manager.Close()
	
	if err := manager.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize migration manager: %v", err)
	}
	
	// Run the requested command
	if *versionFlag {
		version, dirty, err := manager.GetVersion()
		if err != nil {
			log.Fatalf("Failed to get migration version: %v", err)
		}
		fmt.Printf("Current migration version: %d (dirty: %t)\n", version, dirty)
		return
	}
	
	if *validateFlag {
		fmt.Println("Validating migrations...")
		if err := manager.ValidateMigrations(ctx); err != nil {
			log.Fatalf("Migration validation failed: %v", err)
		}
		fmt.Println("Migrations are valid")
		return
	}
	
	if *forceFlag >= 0 {
		fmt.Printf("Forcing migration version to %d...\n", *forceFlag)
		if err := manager.ForceVersion(uint(*forceFlag)); err != nil {
			log.Fatalf("Failed to force version: %v", err)
		}
		fmt.Printf("Migration version forced to %d\n", *forceFlag)
		return
	}
	
	if *upFlag {
		fmt.Println("Running migrations...")
		startTime := time.Now()
		if err := manager.RunMigrations(ctx); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Printf("Migrations completed in %s\n", time.Since(startTime))
		return
	}
	
	if *downFlag {
		fmt.Println("Rolling back last migration...")
		if err := manager.Rollback(ctx); err != nil {
			log.Fatalf("Failed to roll back migration: %v", err)
		}
		fmt.Println("Rollback completed")
		return
	}
	
	if *resetFlag {
		fmt.Println("Rolling back all migrations...")
		if err := manager.RollbackAll(ctx); err != nil {
			log.Fatalf("Failed to reset migrations: %v", err)
		}
		fmt.Println("All migrations have been rolled back")
		return
	}
	
	// If no command is specified, show usage
	flag.Usage()
}
