# Database Package

This package provides a standardized database access layer for the devops-mcp workspace. It replaces the deprecated `internal/database` package as part of the Go workspace migration.

## Features

- PostgreSQL database connection management with connection pooling
- AWS RDS IAM authentication support
- Vector database support with pgvector
- Repository implementations for various entity types
- Automatic database migration handling
- Comprehensive transaction support

## Migration Guide

### Migrating from internal/database

If you're migrating from the deprecated `internal/database` package, follow these steps:

1. **Update imports**: Change all imports from `github.com/S-Corkum/devops-mcp/internal/database` to `github.com/S-Corkum/devops-mcp/pkg/database`

2. **Configuration**: The `Config` struct remains compatible:
   ```go
   // Old
   cfg := &internal_database.Config{
       Host: "localhost",
       Port: 5432,
       ...
   }
   
   // New
   cfg := &database.Config{
       Host: "localhost",
       Port: 5432,
       ...
   }
   ```

3. **Database Initialization**: Use the same initialization pattern:
   ```go
   // Old
   db, err := internal_database.NewDatabase(ctx, cfg)
   
   // New
   db, err := database.NewDatabase(ctx, cfg)
   ```

4. **Repository Interfaces**: All repository interfaces maintain the same method signatures, so your existing code should work without changes other than the import path.

5. **Vector Database**: The vector database implementation has the same interface but with improved performance and error handling.

## Configuration Options

### Basic Configuration

```go
cfg := &database.Config{
    Driver:          "postgres",  // Only PostgreSQL is supported
    Host:            "localhost", // Database host
    Port:            5432,        // Database port
    Database:        "mydb",      // Database name
    Username:        "user",      // Database user
    Password:        "pass",      // Database password (omit if using IAM)
    MaxOpenConns:    25,          // Max open connections
    MaxIdleConns:    5,           // Max idle connections
    ConnMaxLifetime: 5 * time.Minute, // Connection max lifetime
}
```

### AWS RDS with IAM Authentication

```go
cfg := &database.Config{
    // Basic connection info
    Driver:   "postgres",
    Host:     "mydb.cluster-xyz.region.rds.amazonaws.com",
    Port:     5432,
    Database: "mydb",
    Username: "admin",
    
    // AWS RDS specific configuration
    RDSConfig: &database.RDSConfig{
        UseIAMAuth: true,
        AuthConfig: database.AuthConfig{
            Region: "us-west-2",
        },
    },
}
```

### Vector Database Support

```go
cfg := &database.Config{
    // Basic connection info
    // ...
    
    // Vector database configuration
    Vector: database.VectorConfig{
        Enabled:         true,       // Enable vector database
        Dimensions:      1536,       // Vector dimensions
        IndexType:       "hnsw",     // Index type (ivfflat, hnsw)
        Distance:        "cosine",   // Distance metric (cosine, l2, ip)
        CreateExtension: true,       // Automatically create pgvector extension
    },
}
```

## Repository Usage Examples

### Relationship Repository

```go
// Get repository instance
relationshipRepo := db.RelationshipRepository()

// Create a relationship
relationship := &models.EntityRelationship{
    Type:      models.RelationshipTypeReferences,
    Direction: models.DirectionOutgoing,
    Source:    sourceEntityID,
    Target:    targetEntityID,
    Strength:  0.95,
}
id, err := relationshipRepo.CreateRelationship(ctx, relationship)

// Get relationships
relationships, err := relationshipRepo.GetRelationshipsBySource(ctx, sourceEntityID)
```

### Vector Repository

```go
// Get repository instance
vectorRepo := db.VectorRepository()

// Store an embedding
embedding := &models.Vector{
    ID:        "vec123",
    TenantID:  "tenant1",
    Content:   "Sample text content",
    Embedding: []float32{0.1, 0.2, 0.3, ...},
    Metadata:  map[string]interface{}{"source": "document1"},
}
err := vectorRepo.StoreEmbedding(ctx, embedding)

// Search for similar vectors
results, err := vectorRepo.SearchSimilar(ctx, queryVector, 5, 0.7)
```
