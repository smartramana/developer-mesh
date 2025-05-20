// Package main provides tools for migrating imports
// in the devops-mcp workspace.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// PackageMapping defines a mapping from old import path to new
type PackageMapping struct {
	OldPath string
	NewPath string
}

// Options for the import migrator
type Options struct {
	BaseDir   string
	DryRun    bool
	Verbose   bool
	MappingID string
}

// Define standard mappings
var standardMappings = map[string][]PackageMapping{
	"util": {
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/util",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/common/util",
		},
	},
	"aws": {
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/common/aws",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/common/aws",
		},
	},
	"relationship": {
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/models/relationship",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/models/relationship",
		},
	},
	"config": {
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/config",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/config",
		},
	},
	"metrics": {
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
		},
	},
	"direct": {
		// Direct internal to pkg migrations
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/adapters",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/adapters",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/api",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/api",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/common/aws",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/common/aws",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/cache",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/cache",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/chunking",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/chunking",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/common",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/common",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/config",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/config",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/core",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/core",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/database",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/database",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/embedding",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/embedding",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/events",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/events",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/interfaces",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/interfaces",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/models",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/models",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/queue",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/queue",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/models/relationship",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/models/relationship",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/repository",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/repository",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/resilience",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/resilience",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/safety",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/safety",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/storage",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/storage",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/util",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/util",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/util",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/util",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/worker",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/worker",
		},
	},
	"all": {
		// Include all mappings here
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/util",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/common/util",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/common/aws",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/common/aws",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/models/relationship",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/models/relationship",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/config",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/config",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
		},
		// Direct internal to pkg migrations
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/adapters",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/adapters",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/api",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/api",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/common/aws",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/common/aws",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/cache",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/cache",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/chunking",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/chunking",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/common",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/common",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/config",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/config",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/core",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/core",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/database",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/database",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/embedding",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/embedding",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/events",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/events",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/interfaces",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/interfaces",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/models",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/models",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/observability",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/queue",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/queue",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/models/relationship",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/models/relationship",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/repository",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/repository",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/resilience",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/resilience",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/safety",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/safety",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/storage",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/storage",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/util",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/util",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/util",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/util",
		},
		{
			OldPath: "github.com/S-Corkum/devops-mcp/pkg/worker",
			NewPath: "github.com/S-Corkum/devops-mcp/pkg/worker",
		},
	},
}

func main() {
	// Parse command line flags
	opts := parseFlags()

	// Get the appropriate mapping
	mappings, exists := standardMappings[opts.MappingID]
	if !exists {
		log.Fatalf("Mapping ID '%s' not found. Available mappings: %v", 
			opts.MappingID, getAvailableMappings())
	}

	fmt.Printf("Migrating imports in %s\n", opts.BaseDir)
	fmt.Printf("Using mapping: %s\n", opts.MappingID)
	if opts.DryRun {
		fmt.Println("Dry run mode: no files will be modified")
	}

	// Track statistics
	stats := struct {
		FilesProcessed int
		FilesModified  int
		ImportsUpdated int
	}{}

	// Walk the directory tree and migrate imports
	err := filepath.Walk(opts.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") || info.IsDir() {
			return nil
		}

		// Skip vendor directory
		if strings.Contains(path, "/vendor/") {
			return nil
		}

		// Process the file
		modified, updates, err := processFile(path, mappings, opts.DryRun)
		if err != nil {
			log.Printf("Error processing %s: %v", path, err)
			return nil
		}

		// Update statistics
		stats.FilesProcessed++
		if modified {
			stats.FilesModified++
			stats.ImportsUpdated += updates
			if opts.Verbose {
				fmt.Printf("Updated %d imports in %s\n", updates, path)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory: %v", err)
	}

	// Print summary
	fmt.Println("\nMigration Summary:")
	fmt.Printf("Files processed: %d\n", stats.FilesProcessed)
	fmt.Printf("Files modified: %d\n", stats.FilesModified)
	fmt.Printf("Imports updated: %d\n", stats.ImportsUpdated)
}

// parseFlags parses command line flags and returns options
func parseFlags() Options {
	baseDir := flag.String("dir", ".", "Base directory to start migration")
	dryRun := flag.Bool("dry-run", true, "Dry run mode (don't modify files)")
	verbose := flag.Bool("verbose", false, "Verbose output")
	mappingID := flag.String("mapping", "all", "Mapping ID to use")

	flag.Parse()

	return Options{
		BaseDir:   *baseDir,
		DryRun:    *dryRun,
		Verbose:   *verbose,
		MappingID: *mappingID,
	}
}

// getAvailableMappings returns a list of available mapping IDs
func getAvailableMappings() []string {
	result := make([]string, 0, len(standardMappings))
	for id := range standardMappings {
		result = append(result, id)
	}
	return result
}

// processFile processes a single file and updates imports
func processFile(filePath string, mappings []PackageMapping, dryRun bool) (bool, int, error) {
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return false, 0, fmt.Errorf("parse error: %w", err)
	}

	// Track if the file was modified
	modified := false
	updateCount := 0

	// Update imports
	for i, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		
		// Check if this import path should be updated
		for _, mapping := range mappings {
			if importPath == mapping.OldPath || strings.HasPrefix(importPath, mapping.OldPath+"/") {
				// Calculate the new import path
				newPath := strings.Replace(importPath, mapping.OldPath, mapping.NewPath, 1)
				
				// Update the import path
				node.Imports[i].Path.Value = "\"" + newPath + "\""
				modified = true
				updateCount++
				break
			}
		}
	}

	// If the file was modified and this is not a dry run, write it back
	if modified && !dryRun {
		buf := bytes.Buffer{}
		err = format.Node(&buf, fset, node)
		if err != nil {
			return false, 0, fmt.Errorf("format error: %w", err)
		}

		err = ioutil.WriteFile(filePath, buf.Bytes(), 0644)
		if err != nil {
			return false, 0, fmt.Errorf("write error: %w", err)
		}
	}

	return modified, updateCount, nil
}
