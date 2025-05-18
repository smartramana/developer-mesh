// Package repository provides backward compatibility for search operations
package repository

// No imports needed

/*
This file has been deprecated in favor of the search package.
Types and interfaces have been moved to:
- github.com/S-Corkum/devops-mcp/pkg/repository/search/interfaces.go

The factory methods for creating repository instances are now:
- GetSearchRepository() in factory.go

For direct access to the search package, use:
- github.com/S-Corkum/devops-mcp/pkg/repository/search
*/

// Legacy constructors may be added here in the future if needed
// for backward compatibility with old code that imports this package directly

