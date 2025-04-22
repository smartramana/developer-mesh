package migration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CreateMigration creates a new migration file with the given name
func CreateMigration(dir, name string) error {
	if dir == "" {
		return errors.New("migration directory cannot be empty")
	}
	
	if name == "" {
		return errors.New("migration name cannot be empty")
	}
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create migration directory: %w", err)
	}
	
	// Find the latest migration version
	version, err := getNextVersion(dir)
	if err != nil {
		return fmt.Errorf("failed to determine next migration version: %w", err)
	}
	
	// Format version as 3-digit padded number
	versionStr := fmt.Sprintf("%03d", version)
	
	// Create migration file names
	upFileName := fmt.Sprintf("%s_%s.up.sql", versionStr, strings.ToLower(name))
	downFileName := fmt.Sprintf("%s_%s.down.sql", versionStr, strings.ToLower(name))
	
	// Create up migration file
	upFilePath := filepath.Join(dir, upFileName)
	if err := createFile(upFilePath, getUpTemplate(name)); err != nil {
		return fmt.Errorf("failed to create up migration file: %w", err)
	}
	
	// Create down migration file
	downFilePath := filepath.Join(dir, downFileName)
	if err := createFile(downFilePath, getDownTemplate(name)); err != nil {
		return fmt.Errorf("failed to create down migration file: %w", err)
	}
	
	fmt.Printf("Created migration files:\n  %s\n  %s\n", upFilePath, downFilePath)
	return nil
}

// getNextVersion determines the next migration version by scanning the directory
func getNextVersion(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// If directory doesn't exist, start with version 1
			return 1, nil
		}
		return 0, err
	}
	
	maxVersion := 0
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		// Extract version from file name (format: NNN_name.up.sql or NNN_name.down.sql)
		parts := strings.Split(file.Name(), "_")
		if len(parts) < 2 {
			continue
		}
		
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		
		if version > maxVersion {
			maxVersion = version
		}
	}
	
	// Next version is maxVersion + 1, or 1 if no migrations exist
	return maxVersion + 1, nil
}

// createFile creates a new file with the given content
func createFile(path, content string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	_, err = file.WriteString(content)
	return err
}

// getUpTemplate returns a template for up migration
func getUpTemplate(name string) string {
	timestamp := time.Now().Format(time.RFC3339)
	return fmt.Sprintf(`-- Migration: %s
-- Created at: %s
-- Description: Add your migration description here

-- Add your migration SQL here
`, name, timestamp)
}

// getDownTemplate returns a template for down migration
func getDownTemplate(name string) string {
	timestamp := time.Now().Format(time.RFC3339)
	return fmt.Sprintf(`-- Migration: %s (down)
-- Created at: %s
-- Description: This migration reverts the changes made in the up migration

-- Add your migration SQL here
`, name, timestamp)
}
