// Package main provides tools for analyzing and migrating Go imports
// in the devops-mcp workspace.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options for the import analyzer
type Options struct {
	baseDir      string
	targetImport string
	analysisMode string
	verbose      bool
}

// ImportDetails tracks information about imports
type ImportDetails struct {
	Path      string
	Count     int
	Files     []string
	AliasUsed bool
}

func main() {
	// Parse command line flags
	opts := parseFlags()

	// Create an import map to store our findings
	importMap := make(map[string]*ImportDetails)

	// Walk the directory tree and analyze imports
	err := filepath.Walk(opts.baseDir, func(path string, info os.FileInfo, err error) error {
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

		// Parse the file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			log.Printf("Error parsing %s: %v", path, err)
			return nil
		}

		// Process imports based on analysis mode
		switch opts.analysisMode {
		case "all":
			processAllImports(path, node, importMap)
		case "target":
			if opts.targetImport == "" {
				log.Fatal("Target import is required for 'target' analysis mode")
			}
			processTargetImport(path, node, opts.targetImport, importMap)
		case "legacy":
			processLegacyImports(path, node, importMap)
		default:
			log.Fatalf("Unknown analysis mode: %s", opts.analysisMode)
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory: %v", err)
	}

	// Print results
	printResults(importMap, opts)
}

// parseFlags parses command line flags and returns options
func parseFlags() Options {
	baseDir := flag.String("dir", ".", "Base directory to start analysis")
	targetImport := flag.String("import", "", "Target import to analyze (for 'target' mode)")
	analysisMode := flag.String("mode", "all", "Analysis mode: 'all', 'target', or 'legacy'")
	verbose := flag.Bool("verbose", false, "Verbose output including file paths")

	flag.Parse()

	return Options{
		baseDir:      *baseDir,
		targetImport: *targetImport,
		analysisMode: *analysisMode,
		verbose:      *verbose,
	}
}

// processAllImports processes all imports in a file
func processAllImports(filePath string, node *ast.File, importMap map[string]*ImportDetails) {
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		
		// Only track our own imports
		if !strings.HasPrefix(importPath, "github.com/S-Corkum/devops-mcp") {
			continue
		}
		
		details, exists := importMap[importPath]
		if !exists {
			details = &ImportDetails{
				Path:  importPath,
				Count: 0,
				Files: []string{},
			}
			importMap[importPath] = details
		}

		details.Count++
		details.Files = append(details.Files, filePath)
		
		// Check if the import uses an alias
		if imp.Name != nil {
			details.AliasUsed = true
		}
	}
}

// processTargetImport processes a specific target import
func processTargetImport(filePath string, node *ast.File, targetImport string, importMap map[string]*ImportDetails) {
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		
		if importPath == targetImport {
			details, exists := importMap[importPath]
			if !exists {
				details = &ImportDetails{
					Path:  importPath,
					Count: 0,
					Files: []string{},
				}
				importMap[importPath] = details
			}

			details.Count++
			details.Files = append(details.Files, filePath)
			
			if imp.Name != nil {
				details.AliasUsed = true
			}
		}
	}
}

// processLegacyImports processes imports that follow a legacy pattern
func processLegacyImports(filePath string, node *ast.File, importMap map[string]*ImportDetails) {
	legacyPatterns := []string{
		"github.com/S-Corkum/devops-mcp/pkg/util",
		"github.com/S-Corkum/devops-mcp/pkg/common/aws",
		"github.com/S-Corkum/devops-mcp/pkg/models/relationship",
		"github.com/S-Corkum/devops-mcp/pkg/observability",
	}

	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		
		for _, pattern := range legacyPatterns {
			if strings.HasPrefix(importPath, pattern) {
				details, exists := importMap[importPath]
				if !exists {
					details = &ImportDetails{
						Path:  importPath,
						Count: 0,
						Files: []string{},
					}
					importMap[importPath] = details
				}

				details.Count++
				details.Files = append(details.Files, filePath)
				
				if imp.Name != nil {
					details.AliasUsed = true
				}
				
				break
			}
		}
	}
}

// printResults prints the analysis results
func printResults(importMap map[string]*ImportDetails, opts Options) {
	// Extract imports for sorting
	imports := make([]string, 0, len(importMap))
	for imp := range importMap {
		imports = append(imports, imp)
	}
	
	// Sort by count (descending) then by path
	sort.Slice(imports, func(i, j int) bool {
		if importMap[imports[i]].Count == importMap[imports[j]].Count {
			return imports[i] < imports[j]
		}
		return importMap[imports[i]].Count > importMap[imports[j]].Count
	})

	// Print header
	fmt.Println("Import Analysis Results")
	fmt.Println("======================")
	
	// Print summary
	fmt.Printf("Found %d unique imports\n\n", len(imports))
	
	// Print import details
	for _, imp := range imports {
		details := importMap[imp]
		fmt.Printf("%s: %d usages", details.Path, details.Count)
		
		if details.AliasUsed {
			fmt.Print(" (uses alias)")
		}
		
		fmt.Println()
		
		if opts.verbose && len(details.Files) > 0 {
			fmt.Println("  Files:")
			for _, file := range details.Files {
				fmt.Printf("    %s\n", file)
			}
		}
	}
}
