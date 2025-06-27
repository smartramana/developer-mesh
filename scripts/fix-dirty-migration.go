package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	password := os.Getenv("DATABASE_PASSWORD")
	if password == "" {
		log.Fatal("DATABASE_PASSWORD environment variable not set")
	}

	// Connect to the database
	connStr := fmt.Sprintf("host=localhost port=5432 user=dbadmin password=%s dbname=devops_mcp_dev sslmode=require", password)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Check the migration state
	var version int
	var dirty bool
	err = db.QueryRow("SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version, &dirty)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No migrations found in schema_migrations table")
			return
		}
		log.Fatal("Failed to query migration state:", err)
	}

	log.Printf("Current migration state - Version: %d, Dirty: %v\n", version, dirty)

	if dirty {
		// Fix the dirty state
		log.Println("Fixing dirty migration state...")

		// Force the migration version to clean state
		_, err = db.Exec("DELETE FROM schema_migrations WHERE version = $1", version)
		if err != nil {
			log.Fatal("Failed to delete dirty migration:", err)
		}

		// Insert clean version (version-1 so it will retry the failed migration)
		newVersion := version - 1
		_, err = db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)", newVersion)
		if err != nil {
			log.Fatal("Failed to insert clean migration state:", err)
		}

		log.Printf("Reset migration state to version %d (clean) to retry migration %d\n", newVersion, version)
	} else {
		log.Println("Migration is not dirty, no action needed")
	}
}
