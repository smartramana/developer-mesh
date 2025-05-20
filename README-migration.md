# Migration Guide: internal to pkg

## Current Status

We are in the process of migrating all code from `internal/` directories to more reusable `pkg/` directories following Go best practices. This document provides guidance on the migration process.

## Migration Strategy

Following our previous successful migrations, we're using a phased approach:

1. **Phase 1**: Create minimal, self-contained adapters that implement the same interfaces
2. **Phase 2**: Update consumer code incrementally to use the adapters
3. **Phase 3**: Gradually migrate to direct pkg implementations after ensuring compatibility

## Database Migration Status

The `internal/database` package is currently being migrated to `pkg/database` using the adapter pattern for backward compatibility:

- Created `pkg/database/adapters/vector_adapter.go` with a simplified implementation
- Updated `internal/api/server_vector.go` to use the adapter
- Minimized dependencies to avoid complex module dependency issues

## How to Use the Adapters

When updating a component that uses `internal/database`, you have two options:

### Option 1: Use the LegacyVectorAdapter during transition

```go
import "github.com/S-Corkum/devops-mcp/pkg/database/adapters"

// Create an adapter to work with the simplified Logger interface
adapterLogger := &loggerAdapter{logger: yourLogger}

// Create the adapter directly
adapter, err := adapters.NewLegacyVectorAdapter(db, config, adapterLogger) 
if err != nil {
    return err
}

// Use the adapter where you would use VectorDatabase
// The adapter implements the same interface
```

### Option 2: Directly migrate to pkg/database (Preferred for new code)

```go
import "github.com/S-Corkum/devops-mcp/pkg/database"

// Use the pkg implementation directly
vectorDB, err := database.NewVectorDatabase(db, config, logger)
if err != nil {
    return err
}

// Use vectorDB methods directly
```

## Module Dependencies

During this transition phase, we're taking care to avoid complex module dependency chains by:

1. Using simplified adapter implementations with minimal dependencies
2. Adding appropriate replace directives in go.mod files
3. Gradually updating modules one at a time

## Testing Strategy

Each component should be tested after migration:

1. Unit tests for the adapter implementations
2. Integration tests to verify API behavior remains the same
3. End-to-end tests to ensure overall system functionality

## Timeline

The database migration is targeted for completion by June 15, 2025.
