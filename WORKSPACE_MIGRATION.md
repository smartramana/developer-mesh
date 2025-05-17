# Go Workspace Migration Guide

This document outlines the steps to migrate the current monolithic repository structure to a Go Workspace-based monorepo according to industry best practices.

## Migration Overview

We've set up the basic structure for the Go Workspace monorepo, including:

1. A `go.work` file to define the workspace
2. App-specific modules in the `apps/` directory (MCP server, REST API, worker, mockserver)
3. Shared package modules in the `pkg/` directory (including migrations as a shared package)
4. Makefiles for each application and the root directory
5. Updated Dockerfiles for each application
6. A Docker Compose file for local development

## Migration Status

### Completed

- ✅ Initial `go.work` file created 
- ✅ Application module structure set up (apps/mcp-server, apps/rest-api, apps/worker, apps/mockserver)
- ✅ Shared package modules organized (pkg/common, pkg/database, pkg/embedding, etc.)
- ✅ Worker application dependency issues fixed
- ✅ Mockserver application dependencies resolved
- ✅ Removed problematic observability dependency from applications

### In Progress

- 🔄 Resolving import path conflicts in MCP server
- 🔄 Resolving import path conflicts in REST API
- 🔄 Updating internal references to use the new package structure

### Pending

- ⏳ Run tests for each application to verify functionality
- ⏳ Cleanup deprecated code that's been migrated
- ⏳ Update CI/CD pipelines for the new monorepo structure

## Migration Steps

### 1. Code Structure Organization

We've organized the code into the following structure:

#### Applications (apps/)

- **MCP Server** (apps/mcp-server/)
  - Core application for handling GitHub webhooks and interactions
  - Uses shared packages for common functionality

- **REST API** (apps/rest-api/)
  - Dedicated API for embeddings and vector search
  - Separated from MCP server for independent scaling

- **Worker** (apps/worker/)
  - Background processor for handling asynchronous tasks
  - Consumes messages from SQS and other queue systems

- **Mockserver** (apps/mockserver/)
  - Service for simulating external APIs during testing and development

#### Shared Packages (pkg/)

- **Common** (pkg/common/)
  - Configuration management
  - Logging utilities
  - Metrics and monitoring

- **Models** (pkg/models/)
  - Shared data structures used across applications

- **MCP** (pkg/mcp/)
  - Core MCP protocol implementation
  - Tool definitions and handlers

- **Database** (pkg/database/)
  - Database access layer
  - Query builders and connection management

- **Embedding** (pkg/embedding/)
  - Embedding generation services
  - Vector search implementations

- **Chunking** (pkg/chunking/)
  - Code chunking strategies
  - AST parsing for semantic chunking

- **Storage** (pkg/storage/)
  - S3 and other storage providers
  - Content-addressable storage

- **Migrations** (pkg/migrations/)
  - Shared database migration scripts
  - Migration utilities for database schema upgrades

### 2. Import Path Updates

The most critical part of the migration is updating import paths to reflect the new module structure. We've made the following key changes:

#### Old Structure (Monolithic)

```go
// Direct imports from internal packages
import "github.com/S-Corkum/devops-mcp/internal/embedding"
import "github.com/S-Corkum/devops-mcp/internal/storage"
import "github.com/S-Corkum/devops-mcp/internal/observability"
```

#### New Structure (Workspaces)

```go
// Imports from shared packages
import "github.com/S-Corkum/devops-mcp/pkg/embedding"
import "github.com/S-Corkum/devops-mcp/pkg/storage"
import "github.com/S-Corkum/devops-mcp/pkg/common/logging"

// Imports from application-specific internal packages
import "github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/api"
```

A useful command to find files that still need updates:

```bash
find . -name "*.go" -type f -exec grep -l "github.com/S-Corkum/devops-mcp/internal" {} \;
```

### 3. Current Challenges

#### Dependency Management

During the migration, we've encountered and resolved several challenges:

1. **OpenTelemetry Compatibility Issues**: 
   - The `internal/observability` package had dependency issues with conflicting versions
   - Solution: Temporarily removed OpenTelemetry dependencies and switched to standard logging
   - Future: Will reintegrate OpenTelemetry with proper version management

2. **Ambiguous Import Resolution**:
   - Go was finding packages in both monolithic and workspace modules
   - Solution: Updated go.mod files to explicitly manage dependencies
   - Removed ambiguous imports with proper replace directives

3. **AWS SDK Version Conflicts**:
   - Multiple AWS SDK package versions caused conflicts
   - Solution: Standardized AWS SDK imports and versions

4. **PostgreSQL JSON Issues**:
   - Known issue with empty string handling for JSON fields
   - Tracking for future fix in the database layer

### 4. Next Steps

#### Immediate Actions

1. **Verify Application Functionality**
   ```bash
   # Run the worker application tests
   cd apps/worker && go test ./...
   
   # Build and run the worker application
   cd apps/worker && go build ./cmd/worker && ./worker
   
   # Test each application similarly
   cd apps/mcp-server && go test ./...
   cd apps/rest-api && go test ./...
   ```

2. **Fix Remaining Import Conflicts**
   ```bash
   # Find remaining files with old import paths
   find . -name "*.go" -type f -exec grep -l "github.com/S-Corkum/devops-mcp/internal" {} \;
   
   # Update those files to use the new package structure
   # Example: sed -i 's|github.com/S-Corkum/devops-mcp/internal/embedding|github.com/S-Corkum/devops-mcp/pkg/embedding|g' filename.go
   ```

3. **Clean Up Old Code Structure**
   ```bash
   # Only after verifying all tests pass and functionality is preserved
   # This is a dangerous operation - make sure you have a backup or proper version control
   # rm -rf internal/
   ```
   
#### Long-term Improvements

1. **Documentation**: Create comprehensive documentation for the new structure
2. **CI/CD**: Update build and deployment pipelines
3. **Testing**: Add integration tests across modules
4. **Metrics**: Reintegrate observability with proper versioning
	// Application-specific defaults
	switch appName {
	case "mcp-server":
		v.SetDefault("server.port", 8080)
		v.SetDefault("server.host", "0.0.0.0")
	case "worker":
		v.SetDefault("worker.interval", "1m")
		v.SetDefault("worker.concurrency", 5)
	}
}
```

### 5. Update the Database Access

Create a shared database package:

#### Database Connection (pkg/database/postgres/connection.go)

```go
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
)

// Config represents database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	MaxConns int
}

// NewConfigFromViper creates a database config from viper
func NewConfigFromViper(v *viper.Viper) Config {
	return Config{
		Host:     v.GetString("database.host"),
		Port:     v.GetInt("database.port"),
		User:     v.GetString("database.user"),
		Password: v.GetString("database.password"),
		Database: v.GetString("database.name"),
		SSLMode:  v.GetString("database.sslmode"),
		MaxConns: v.GetInt("database.max_conns"),
	}
}

// Connect establishes a connection to the database
func Connect(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.Database, config.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("error parsing connection string: %w", err)
	}

	poolConfig.MaxConns = int32(config.MaxConns)
	
	// Set reasonable defaults
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
```

### 6. Test the Migration

After moving code and updating imports, use the workspace Makefile to build and test:

```bash
# Build all applications
make -f Makefile.workspace build

# Run tests
make -f Makefile.workspace test

# Run the MCP server directly
make -f Makefile.workspace run-mcp-server
```

### 7. Update CI/CD Pipeline

Update your CI/CD pipeline configuration to use the workspace structure. For example:

```yaml
# Example GitHub Actions workflow
name: Build and Test

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
          
      - name: Build all applications
        run: make -f Makefile.workspace build
        
      - name: Test all modules
        run: make -f Makefile.workspace test
        
      - name: Build Docker images
        run: make -f Makefile.workspace docker
```

## Specialized Components Migration

### Embedding System

Based on your existing memories about the embedding system, you'll need to:

1. Move the AWS Bedrock integration code to `pkg/embedding/bedrock`
2. Move the vector search API code to `pkg/embedding/search`
3. Update dependencies between modules

### Code Chunking

For the intelligent code chunking system:

1. Move the language-specific parsers to `pkg/chunking/parsers`
2. Move the chunking service to `pkg/chunking/service`
3. Update imports and dependencies

### S3 Storage

For the S3-based content-addressable storage:

1. Move the S3 client implementation to `pkg/storage/s3`
2. Move the context storage to `pkg/storage/context`
3. Implement the repository-centric structure with content-addressable storage

## Final Steps

1. Once all code is migrated and tested, rename `Makefile.workspace` to `Makefile`
2. Rename `docker-compose.workspace.yml` to `docker-compose.yml`
3. Remove the old monolithic `go.mod` and `go.sum` files
4. Update documentation to reflect the new structure

## Resolving Known Issues

Based on the memory about PostgreSQL rejecting updates with "invalid input syntax for type json", ensure that:

1. Empty strings are properly handled as valid JSON objects (`'{}'` or `null`) in the database code
2. Update the metadata handling in `pkg/storage` to fix this issue
