package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

var (
	dsn      = flag.String("dsn", "", "Database connection string")
	up       = flag.Bool("up", false, "Run all pending migrations")
	down     = flag.Bool("down", false, "Rollback the last migration")
	reset    = flag.Bool("reset", false, "Rollback all migrations")
	version  = flag.Bool("version", false, "Show current migration version")
	validate = flag.Bool("validate", false, "Validate migrations without running them")
	create   = flag.Bool("create", false, "Create a new migration file")
	name     = flag.String("name", "", "Name for the new migration")
	steps    = flag.Int("steps", 0, "Number of migrations to run (0 = all)")
	force    = flag.Int("force", -1, "Force database to specific version")
)

const migrationDir = "../../migrations/sql"

func main() {
	flag.Parse()

	if *create {
		if *name == "" {
			log.Fatal("Please provide a name for the migration with -name")
		}
		createMigration(*name)
		return
	}

	if *dsn == "" {
		log.Fatal("Please provide a database connection string with -dsn")
	}

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	if err := ensureMigrationTable(db); err != nil {
		log.Fatal("Failed to create migration table:", err)
	}

	switch {
	case *up:
		runMigrations(db, "up", *steps)
	case *down:
		runMigrations(db, "down", 1)
	case *reset:
		runMigrations(db, "down", 0)
	case *version:
		showVersion(db)
	case *validate:
		validateMigrations()
	case *force >= 0:
		forceVersion(db, *force)
	default:
		flag.Usage()
	}
}

func ensureMigrationTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func getCurrentVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	return version, err
}

func getMigrationFiles(direction string) ([]string, error) {
	pattern := fmt.Sprintf("*.%s.sql", direction)
	files, err := filepath.Glob(filepath.Join(migrationDir, pattern))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if direction == "down" {
		// Reverse for down migrations
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}
	return files, nil
}

func extractVersion(filename string) int {
	base := filepath.Base(filename)
	parts := strings.Split(base, "_")
	if len(parts) > 0 {
		var v int
		fmt.Sscanf(parts[0], "%d", &v)
		return v
	}
	return 0
}

func runMigrations(db *sql.DB, direction string, limit int) {
	currentVersion, err := getCurrentVersion(db)
	if err != nil {
		log.Fatal("Failed to get current version:", err)
	}

	files, err := getMigrationFiles(direction)
	if err != nil {
		log.Fatal("Failed to get migration files:", err)
	}

	count := 0
	for _, file := range files {
		fileVersion := extractVersion(file)

		if direction == "up" && fileVersion <= currentVersion {
			continue
		}
		if direction == "down" && fileVersion > currentVersion {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatal("Failed to read migration file:", err)
		}

		tx, err := db.Begin()
		if err != nil {
			log.Fatal("Failed to begin transaction:", err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			log.Fatalf("Failed to execute migration %s: %v", file, err)
		}

		if direction == "up" {
			if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", fileVersion); err != nil {
				tx.Rollback()
				log.Fatal("Failed to record migration:", err)
			}
		} else {
			if _, err := tx.Exec("DELETE FROM schema_migrations WHERE version = $1", fileVersion); err != nil {
				tx.Rollback()
				log.Fatal("Failed to remove migration record:", err)
			}
		}

		if err := tx.Commit(); err != nil {
			log.Fatal("Failed to commit transaction:", err)
		}

		fmt.Printf("Applied migration: %s\n", filepath.Base(file))
		count++

		if limit > 0 && count >= limit {
			break
		}
	}

	fmt.Printf("Applied %d migrations\n", count)
}

func showVersion(db *sql.DB) {
	version, err := getCurrentVersion(db)
	if err != nil {
		log.Fatal("Failed to get current version:", err)
	}
	fmt.Printf("Current migration version: %d\n", version)
}

func validateMigrations() {
	upFiles, err := getMigrationFiles("up")
	if err != nil {
		log.Fatal("Failed to get up migrations:", err)
	}

	downFiles, err := getMigrationFiles("down")
	if err != nil {
		log.Fatal("Failed to get down migrations:", err)
	}

	// Check that each up has a corresponding down
	upVersions := make(map[int]string)
	for _, f := range upFiles {
		v := extractVersion(f)
		upVersions[v] = f
	}

	downVersions := make(map[int]string)
	for _, f := range downFiles {
		v := extractVersion(f)
		downVersions[v] = f
	}

	valid := true
	for v, f := range upVersions {
		if _, ok := downVersions[v]; !ok {
			fmt.Printf("Missing down migration for: %s\n", filepath.Base(f))
			valid = false
		}
	}

	for v, f := range downVersions {
		if _, ok := upVersions[v]; !ok {
			fmt.Printf("Missing up migration for: %s\n", filepath.Base(f))
			valid = false
		}
	}

	if valid {
		fmt.Println("All migrations are valid")
	}
}

func createMigration(name string) {
	// Find the next version number
	files, _ := filepath.Glob(filepath.Join(migrationDir, "*.sql"))
	maxVersion := 0
	for _, f := range files {
		if v := extractVersion(f); v > maxVersion {
			maxVersion = v
		}
	}

	nextVersion := maxVersion + 1
	upFile := filepath.Join(migrationDir, fmt.Sprintf("%03d_%s.up.sql", nextVersion, name))
	downFile := filepath.Join(migrationDir, fmt.Sprintf("%03d_%s.down.sql", nextVersion, name))

	// Create up migration
	if err := os.WriteFile(upFile, []byte("-- Write your UP migration here\n"), 0644); err != nil {
		log.Fatal("Failed to create up migration:", err)
	}

	// Create down migration
	if err := os.WriteFile(downFile, []byte("-- Write your DOWN migration here\n"), 0644); err != nil {
		log.Fatal("Failed to create down migration:", err)
	}

	fmt.Printf("Created migrations:\n  %s\n  %s\n", upFile, downFile)
}

func forceVersion(db *sql.DB, version int) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal("Failed to begin transaction:", err)
	}

	// Clear existing migrations
	if _, err := tx.Exec("DELETE FROM schema_migrations"); err != nil {
		tx.Rollback()
		log.Fatal("Failed to clear migrations:", err)
	}

	// Set to specific version
	if version > 0 {
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback()
			log.Fatal("Failed to set version:", err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatal("Failed to commit transaction:", err)
	}

	fmt.Printf("Forced database to version: %d\n", version)
}
