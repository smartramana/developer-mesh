# Go Workspace Structure

## Overview

DevOps MCP uses Go 1.24's workspace feature to organize the codebase as a monorepo with multiple modules. This architecture provides strong module boundaries while enabling code sharing, making it ideal for microservices development.

## Why Go Workspaces?

Go workspaces solve several challenges in multi-module projects:

- **Local Development**: Work on multiple modules simultaneously without replace directives
- **Dependency Management**: Shared dependencies are managed consistently
- **Code Sharing**: Common packages can be imported across modules
- **Build Optimization**: Only affected modules need rebuilding
- **Version Independence**: Modules can have different dependency versions when needed

## Project Structure

```
devops-mcp/
├── go.work                    # Workspace definition
├── go.work.sum               # Workspace checksums
├── Makefile                  # Build automation
├── apps/                     # Application modules
│   ├── mcp-server/          # MCP protocol server
│   │   ├── go.mod           # Module: mcp-server
│   │   ├── cmd/server/      # Entry point
│   │   └── internal/        # Private packages
│   │       ├── adapters/    # External adapters
│   │       ├── api/         # API handlers
│   │       ├── config/      # Configuration
│   │       └── core/        # Business logic
│   ├── rest-api/            # REST API service
│   │   ├── go.mod           # Module: rest-api
│   │   ├── cmd/api/         # Entry point
│   │   └── internal/        # Private packages
│   └── worker/              # Event processor
│       ├── go.mod           # Module: worker
│       ├── cmd/worker/      # Entry point
│       └── internal/        # Private packages
└── pkg/                     # Shared packages
    ├── adapters/            # Common adapters
    ├── common/              # Utilities
    ├── database/            # DB abstractions
    ├── embedding/           # Vector operations
    ├── models/              # Domain models
    ├── observability/       # Logging/metrics
    └── repository/          # Data patterns
```

## Workspace Configuration

### go.work File

```go
go 1.24

use (
    ./apps/mcp-server
    ./apps/rest-api
    ./apps/worker
    ./pkg
)
```

### Module Names

Post-refactor, modules use simple names instead of full GitHub paths:

```go
// apps/mcp-server/go.mod
module mcp-server

// apps/rest-api/go.mod
module rest-api

// apps/worker/go.mod
module worker
```

This prevents module resolution conflicts in workspace mode.

## Dependency Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ mcp-server  │     │  rest-api   │     │   worker    │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                     │
       └───────────────────┴─────────────────────┘
                           │
                     ┌─────▼─────┐
                     │    pkg    │
                     └───────────┘
```

Rules:
1. Applications (`apps/*`) can import from `pkg`
2. Applications cannot import from each other
3. `pkg` cannot import from applications
4. External dependencies are managed per module

## Working with the Workspace

### Initial Setup

```bash
# Clone repository
git clone https://github.com/S-Corkum/devops-mcp.git
cd devops-mcp

# Sync workspace dependencies
go work sync

# Or use Makefile
make sync
```

### Building Modules

```bash
# Build all applications
make build

# Build specific application
make build-mcp-server
make build-rest-api
make build-worker

# Build from module directory
cd apps/mcp-server
go build -o mcp-server ./cmd/server
```

### Testing

```bash
# Test all modules
make test

# Test specific module
make test-mcp-server

# Test with coverage
make test-coverage

# Integration tests
make test-integration
```

### Adding Dependencies

```bash
# Add to specific module
cd apps/rest-api
go get github.com/some/package

# Sync workspace
cd ../..
go work sync
```

## Module Organization

### Application Modules (`apps/*`)

Each application follows a consistent structure:

```
apps/service-name/
├── go.mod              # Module definition
├── cmd/                # Entry points
│   └── server/         # Main binary
│       └── main.go
├── internal/           # Private packages
│   ├── adapters/       # External integrations
│   ├── api/            # API layer
│   ├── config/         # Configuration
│   └── core/           # Business logic
└── tests/              # Integration tests
```

### Shared Packages (`pkg/*`)

Shared packages are organized by functionality:

```
pkg/
├── adapters/           # Interface adapters
│   ├── github/        # GitHub integration
│   ├── events/        # Event bus
│   └── resilience/    # Circuit breakers
├── common/            # Common utilities
│   ├── config/        # Config structs
│   ├── errors/        # Error types
│   └── utils/         # Helpers
├── models/            # Domain models
│   ├── context.go     # Context entity
│   ├── agent.go       # Agent entity
│   └── ...
└── repository/        # Data access
    ├── interfaces.go  # Repository contracts
    └── ...
```

## Best Practices

### 1. Module Independence

Keep modules loosely coupled:
- Define interfaces in `pkg`
- Implement in application modules
- Use dependency injection

### 2. Internal Packages

Use `internal/` for private code:
```go
// Only accessible within the module
apps/mcp-server/internal/core/engine.go

// Cannot import from another module
apps/rest-api/internal/adapters/
```

### 3. Shared Types

Place shared types in `pkg/models`:
```go
// pkg/models/context.go
type Context struct {
    ID        string
    Name      string
    CreatedAt time.Time
}
```

### 4. Interface Definitions

Define interfaces where they're used:
```go
// pkg/repository/interfaces.go
type Repository[T any] interface {
    Create(ctx context.Context, entity T) (T, error)
    Get(ctx context.Context, id string) (T, error)
}
```

### 5. Configuration

Standardize configuration across modules:
```go
// Use viper for consistent config loading
cfg, err := config.Load()
```

## Common Patterns

### Adapter Pattern

Bridge interface differences:

```go
// pkg expects one interface
type PkgRepository interface {
    Store(ctx context.Context, item Item) error
}

// app has different interface
type AppRepository interface {
    Save(ctx context.Context, item Item) error
}

// Adapter bridges the gap
type RepositoryAdapter struct {
    app AppRepository
}

func (a *RepositoryAdapter) Store(ctx context.Context, item Item) error {
    return a.app.Save(ctx, item)
}
```

### Factory Pattern

Create module-specific implementations:

```go
// pkg/adapters/factory.go
func NewAdapter(cfg Config) (Adapter, error) {
    switch cfg.Type {
    case "github":
        return github.NewAdapter(cfg)
    case "gitlab":
        return gitlab.NewAdapter(cfg)
    default:
        return nil, errors.New("unknown adapter type")
    }
}
```

## Migration Notes

When migrating from a single module:

1. **Create Module Structure**: Set up `apps/` directories
2. **Move Code**: Relocate packages to appropriate modules
3. **Update Imports**: Change import paths
4. **Add go.mod**: Initialize modules with simple names
5. **Update go.work**: Include all modules
6. **Test Thoroughly**: Ensure no circular dependencies

## Troubleshooting

### Module Not Found

```bash
# Ensure module is in workspace
go work use ./apps/new-module

# Sync dependencies
go work sync
```

### Import Conflicts

```bash
# Use simple module names
module mcp-server  # Good
module github.com/user/mcp-server  # Can cause conflicts
```

### Circular Dependencies

```bash
# Check dependency graph
go mod graph | grep circular

# Refactor to use interfaces
```

## Future Considerations

1. **Module Versioning**: Consider semantic versioning for pkg
2. **Module Publishing**: Potentially publish pkg as separate module
3. **Workspace Tooling**: Leverage Go 1.24+ workspace improvements
4. **Build Optimization**: Use module-aware build caching

## References

- [Go Workspaces Documentation](https://go.dev/doc/tutorial/workspaces)
- [Go Modules Reference](https://go.dev/ref/mod)
- [Adapter Pattern](adapter-pattern.md)
- [System Architecture](system-overview.md)