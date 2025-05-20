// Package storage is deprecated and will be removed in a future version.
// Use github.com/S-Corkum/devops-mcp/pkg/storage instead.
//
// Deprecated: This package is part of the Go workspace migration and is being phased out.
// All functionality has been migrated to pkg/storage with feature parity.
//
// Migration Guide:
// 1. Import github.com/S-Corkum/devops-mcp/pkg/storage instead of internal/storage
// 2. All interfaces and functions remain the same
// 3. No code changes should be needed other than updating import paths
//
// If you encounter any migration issues, please refer to the documentation
// or contact the DevOps team for assistance.
package storage

import (
	"fmt"
	"log"
	"runtime"
)

func init() {
	// Get caller information to provide more useful deprecation warnings
	pc, file, line, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(pc).Name()
	
	// Log a deprecation warning
	log.Printf("[DEPRECATED] The package 'github.com/S-Corkum/devops-mcp/internal/storage' is deprecated and will be removed in a future version.")
	log.Printf("[DEPRECATED] Called from %s (%s:%d)", caller, file, line)
	log.Printf("[DEPRECATED] Please use 'github.com/S-Corkum/devops-mcp/pkg/storage' instead.")
	
	// Don't panic, just warn - to maintain backward compatibility during migration
	fmt.Println("WARNING: Using deprecated internal/storage package. Please migrate to pkg/storage.")
}
