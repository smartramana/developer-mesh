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
- ✅ Resolved import path conflicts in MCP server
- ✅ Resolved import path conflicts in REST API
- ✅ Updated internal references to use the new package structure
- ✅ Implemented adapter pattern for vector API to solve interface compatibility issues
- ✅ Successfully tested vector API with the new adapter implementation

### In Progress

- 🔄 Run tests for each application to verify functionality
- 🔄 Resolving dependency issues between migrated packages
- 🔄 Implementing adapter pattern for agent API to solve interface compatibility issues

### Pending

- ⏳ Cleanup deprecated code that's been migrated
- ⏳ Update CI/CD pipelines for the new monorepo structure

### Additional Migration Challenges

During the migration process, we've encountered and partially addressed the following challenges:

1. **Import Cycles**: We have circular dependencies between modules that need to be resolved by restructuring the code.
   - Issue: Some packages in `pkg/common/config` are importing packages that are now part of `pkg/common`
   - ✅ Fixed circular dependencies in `pkg/common/config` by updating import paths
   - ✅ Fixed AWS imports in `pkg/common/cache` to point to the new location
   - ✅ Updated observability imports to use the new package structure
   - ❌ Still need to resolve remaining circular dependencies in other packages

2. **Missing Modules**: Some packages referenced in the code don't exist in the new structure yet.
   - ✅ Created necessary directory structure for missing packages
   - ✅ Migrated essential code from internal/ to pkg/ directories
   - ❌ Need to complete migration of remaining internal packages

3. **Inconsistent Package Structure**: The package structure in the new pkg/ directory doesn't fully match the imports.
   - ✅ Updated package declarations to match their new locations
   - ✅ Fixed import paths in key packages
   - ❌ Need to ensure consistent naming and structure across all packages

4. **Module Definition Issues**: The go.mod files for each package need proper configuration.
   - ✅ Created go.mod files for missing packages
   - ❌ Need to properly define dependencies and version constraints
   - ❌ Need to address version compatibility issues with external dependencies

### Next Steps for Resolution

1. **Dependency Restructuring**:
   - Complete a full dependency graph analysis of all packages to map remaining circular references
   - Reorganize internal structure of packages to eliminate import cycles by:
     - Moving interfaces to dedicated packages (e.g., `pkg/common/interfaces`)
     - Using dependency injection patterns for cross-package dependencies
     - Implementing the Interface Segregation Principle to minimize dependencies
   - Update remaining import paths, especially in:
     - `pkg/mcp` packages that reference incorrect common package paths
     - Database-related packages that have circular references
     - The worker and REST API application packages

2. **Adapter Pattern Implementation**:
   - ✅ Implemented for Vector API (details below)
   - 🔄 In progress for Agent API
   - Future implementation for Model API and other components with interface compatibility issues

### Adapter Pattern Strategy

To resolve interface incompatibilities between API code and repository implementations, we've successfully implemented the adapter pattern. This approach allows us to maintain backward compatibility while transitioning to the new package structure.

#### Vector API Adapter Implementation

The adapter pattern bridges the gap between the Vector API's expected interface and the repository implementation:

```go
// Adapter implementation
type ServerEmbeddingAdapter struct {
    repo repository.VectorAPIRepository
}

func NewServerEmbeddingAdapter(repo repository.VectorAPIRepository) *ServerEmbeddingAdapter {
    return &ServerEmbeddingAdapter{repo: repo}
}

// API expects this method signature with models.Vector
func (a *ServerEmbeddingAdapter) StoreEmbedding(ctx context.Context, vector *models.Vector) error {
    // Convert from models.Vector to repository.Embedding
    repoEmbedding := &repository.Embedding{
        ID:           vector.ID,
        ContextID:    vector.TenantID,
        ContentIndex: extractContentIndex(vector.Metadata),
        Text:         vector.Content,
        Embedding:    vector.Embedding,
        ModelID:      extractModelID(vector.Metadata),
    }
    
    // Delegate to the repository implementation
    return a.repo.StoreEmbedding(ctx, repoEmbedding)
}
```

#### Key Conversion Points

- Field name mapping (`tenant_id` ↔ `context_id`, `content` ↔ `text`)
- Metadata handling (extracting `content_index` and `model_id` from metadata)
- Method signature adaptations to match interface expectations

#### Benefits

1. Maintains backward compatibility with existing API code
2. Allows for clear separation between repository and API layers
3. Enables proper type safety with explicit conversions
4. Improves maintainability by isolating interface differences

#### Testing Success

We've successfully created isolated tests for the Vector API with the adapter pattern, verifying all operations work correctly with proper type conversion.

2. **Module Configuration**:
   - Add `replace` directives in each module's go.mod file to point to local modules during development:
     ```go
     replace github.com/S-Corkum/devops-mcp/pkg/common => ../../pkg/common
     ```
   - Use `go mod edit` to ensure consistent versioning across all modules
   - Update the workspace-level Makefile to handle module sync operations

3. **Systematic Testing Plan**:
   - Start testing with fundamental packages that have no dependencies
   - Work upward through the dependency tree, fixing issues at each level
   - For each module, ensure both unit tests and integration tests pass
   - Introduce proper mocking for cross-module dependencies
   - Create a full test matrix for all applications in the CI/CD pipeline

4. **Package Structure Consistency**:
   - Standardize package naming across the codebase
   - Ensure consistent structure for configuration files
   - Implement consistent error handling across packages
   - Create a package guidelines document for future development

5. **Migration Completion Checklist**:
   - [ ] All import paths updated to use the new package structure
   - [ ] All circular dependencies resolved
   - [ ] All applications build successfully
   - [ ] All tests pass with the new workspace structure
   - [ ] CI/CD pipelines updated for the workspace structure
   - [ ] Dockerfiles updated to use the workspace structure
   - [ ] Documentation updated to reflect the new package organization

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

### 4. Workspace Setup and Configuration

To implement the Go workspace for the monorepo, follow these steps:

#### Create the go.work file

Create a `go.work` file in the root directory with the following content:

```go
go 1.24.2

use (
	./apps/mcp-server
	./apps/rest-api
	./apps/worker
	./apps/mockserver
	./pkg/common
	./pkg/database
	./pkg/embedding
	./pkg/chunking
	./pkg/storage
	./pkg/migrations
	./pkg/models
	./pkg/mcp
)
```

#### Create Module Files for Each Package

For each application and shared package, create a `go.mod` file that defines its module path and dependencies.

Example for an application module (apps/mcp-server/go.mod):

```go
module github.com/S-Corkum/devops-mcp/apps/mcp-server

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp/pkg/common v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/database v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/mcp v0.0.0
	github.com/aws/aws-sdk-go-v2 v1.25.0
	github.com/google/go-github/v57 v57.0.0
	github.com/gorilla/mux v1.8.1
	github.com/spf13/viper v1.18.2
)

replace (
	github.com/S-Corkum/devops-mcp/pkg/common => ../../pkg/common
	github.com/S-Corkum/devops-mcp/pkg/database => ../../pkg/database
	github.com/S-Corkum/devops-mcp/pkg/mcp => ../../pkg/mcp
)
```

Example for a shared package module (pkg/common/go.mod):

```go
module github.com/S-Corkum/devops-mcp/pkg/common

go 1.24.2

require (
	github.com/aws/aws-sdk-go-v2/config v1.26.3
	github.com/spf13/viper v1.18.2
)
```

#### Create Workspace Makefile

Create a `Makefile.workspace` in the root directory to build and test all modules:

```makefile
# Workspace Makefile for building and testing all modules

.PHONY: build test clean docker run-mcp-server run-rest-api run-worker

build:
	@echo "Building all applications..."
	@cd apps/mcp-server && go build ./cmd/mcp-server
	@cd apps/rest-api && go build ./cmd/rest-api
	@cd apps/worker && go build ./cmd/worker
	@cd apps/mockserver && go build ./cmd/mockserver

test:
	@echo "Running tests for all packages..."
	@cd apps/mcp-server && go test ./...
	@cd apps/rest-api && go test ./...
	@cd apps/worker && go test ./...
	@cd apps/mockserver && go test ./...
	@cd pkg/common && go test ./...
	@cd pkg/database && go test ./...
	@cd pkg/embedding && go test ./...
	@cd pkg/chunking && go test ./...
	@cd pkg/storage && go test ./...
	@cd pkg/migrations && go test ./...
	@cd pkg/models && go test ./...
	@cd pkg/mcp && go test ./...

clean:
	@echo "Cleaning build artifacts..."
	@find . -name "*.exe" -delete
	@find . -type f -name "mcp-server" -delete
	@find . -type f -name "rest-api" -delete
	@find . -type f -name "worker" -delete
	@find . -type f -name "mockserver" -delete

docker:
	@echo "Building Docker images..."
	docker build -t mcp-server:latest -f apps/mcp-server/Dockerfile .
	docker build -t rest-api:latest -f apps/rest-api/Dockerfile .
	docker build -t worker:latest -f apps/worker/Dockerfile .
	docker build -t mockserver:latest -f apps/mockserver/Dockerfile .

run-mcp-server:
	@cd apps/mcp-server && go run ./cmd/mcp-server

run-rest-api:
	@cd apps/rest-api && go run ./cmd/rest-api

run-worker:
	@cd apps/worker && go run ./cmd/worker
```

### 5. Code Migration Process

Follow these steps to migrate each component from the monolithic structure to the workspace structure:

#### Moving Source Files

1. **Identify code to migrate**: Use the grep command to find related files:
   ```bash
   find . -name "*.go" -type f -exec grep -l "github.com/S-Corkum/devops-mcp/internal/embedding" {} \;
   ```

2. **Copy files** to their new locations according to the structure outlined in Step 1

3. **Update package declarations** in each file to match their new module path

4. **Fix imports** to use the new workspace module paths

#### Dependency Resolution

1. After moving files, run `go mod tidy` in each module directory to update dependencies

2. Use the `replace` directive in `go.mod` files for local development:
   ```go
   replace github.com/S-Corkum/devops-mcp/pkg/common => ../../pkg/common
   ```

3. Install Go 1.24.2 or later which is required for workspace support:
   ```bash
   go install golang.org/dl/go1.24.2@latest
   go1.24.2 download
   ```

#### Docker Configuration Updates

Update Dockerfiles for each application to use the new workspace structure:

```dockerfile
# Example Dockerfile for MCP Server
FROM golang:1.24.2 as builder

WORKDIR /app

# Copy go.work and all go.mod/go.sum files
COPY go.work go.work.sum* ./
COPY apps/mcp-server/go.mod apps/mcp-server/go.sum* apps/mcp-server/
COPY pkg/*/go.mod pkg/*/go.sum* ./pkg/

# Copy source code
COPY apps/mcp-server ./apps/mcp-server
COPY pkg ./pkg

# Build the application
RUN cd apps/mcp-server && go build -o /app/mcp-server ./cmd/mcp-server

# Runtime image
FROM debian:bullseye-slim

WORKDIR /app

COPY --from=builder /app/mcp-server /app/

CMD ["/app/mcp-server"]
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
