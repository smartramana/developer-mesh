// Package database is deprecated and will be removed in a future version.
// Use github.com/S-Corkum/devops-mcp/pkg/database instead.
//
// Deprecated: This package is part of the Go workspace migration and is being phased out.
// All functionality has been migrated to pkg/database with feature parity.
//
// Migration Guide:
// 1. Import github.com/S-Corkum/devops-mcp/pkg/database instead of internal/database
// 2. Use pkg/database.Config instead of internal/database.Config
// 3. Database client initialization is backward compatible:
//    - Replace NewDatabase() with database.NewDatabase()
//    - Config struct fields have the same names and semantics
// 4. Repository interfaces remain the same but are now in pkg/database
//
// If you encounter any migration issues, please refer to the documentation
// or contact the DevOps team for assistance.
package database

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
	log.Printf("[DEPRECATED] The package 'github.com/S-Corkum/devops-mcp/internal/database' is deprecated and will be removed in a future version.")
	log.Printf("[DEPRECATED] Called from %s (%s:%d)", caller, file, line)
	log.Printf("[DEPRECATED] Please use 'github.com/S-Corkum/devops-mcp/pkg/database' instead.")
	
	// Don't panic, just warn - to maintain backward compatibility during migration
	fmt.Println("WARNING: Using deprecated internal/database package. Please migrate to pkg/database.")
}
