# Go Workspace Structure

## Overview

The DevOps MCP project uses Go's workspace feature to organize the codebase into multiple modules. This structure enables better separation of concerns, cleaner dependencies, and improved maintainability while allowing shared code between components.

## Workspace Organization

The project is structured as follows:

```
devops-mcp/
├── go.work                # Workspace definition file
├── apps/                  # Application modules
│   ├── mcp-server/        # MCP server module
│   │   ├── go.mod         # Module definition
│   │   ├── cmd/           # Command entry points
│   │   ├── internal/      # Internal implementation
│   │   └── configs/       # Configuration files
│   ├── rest-api/          # REST API service
│   │   ├── go.mod
│   │   ├── cmd/
│   │   └── internal/
│   └── worker/            # Asynchronous worker service
│       ├── go.mod
│       └── cmd/
├── pkg/                   # Shared packages
│   ├── go.mod             # Shared module definition
│   ├── adapters/          # Interface adapters
│   ├── config/            # Configuration management
│   ├── repository/        # Data access layer
│   └── ...
└── docker-compose.local.yml # Local development setup
```

## Module Dependencies

The workspace is configured so that:

1. `apps/mcp-server`, `apps/rest-api`, and `apps/worker` can import packages from `pkg/`
2. Each application module is independent and doesn't import from other applications
3. The `pkg/` module contains shared interfaces, models, and utilities

## Managing the Workspace

### Initializing the Workspace

```bash
go work init
go work use ./apps/mcp-server ./apps/rest-api ./apps/worker ./pkg
```

### Syncing Dependencies

To ensure all module dependencies are in sync:

```bash
go work sync
```

Or use the Makefile shortcut:

```bash
make sync
```

## Adapter Pattern in the Workspace

The workspace structure is designed to work seamlessly with the adapter pattern:

1. Core interfaces are defined in `pkg/repository/`
2. Implementation-specific repositories are in their respective app modules
3. Adapters bridge between different interface expectations:
   - `apps/rest-api/internal/adapters/` contains adapters for REST API
   - `apps/mcp-server/internal/adapters/` contains adapters for MCP Server

For more details on the adapter pattern implementation, see [Adapter Pattern](adapter-pattern.md).

## Benefits of the Workspace Structure

1. **Modular Development**: Work on one component without affecting others
2. **Clear Boundaries**: Each module has well-defined responsibilities
3. **Independent Versioning**: Modules can evolve at different rates
4. **Reduced Build Times**: Only rebuild affected modules
5. **Simplified Testing**: Test modules in isolation
6. **Easier Onboarding**: New developers can focus on specific modules

## Common Operations

### Building Specific Modules

```bash
# Build MCP Server
cd apps/mcp-server && go build ./cmd/server

# Build REST API
cd apps/rest-api && go build ./cmd/server

# Build Worker
cd apps/worker && go build ./cmd/worker
```

### Running Tests

```bash
# Test a specific module
cd apps/rest-api && go test ./...

# Test all modules (from workspace root)
make test
```

### Adding a New Module

1. Create a new directory in `apps/` or `pkg/`
2. Initialize a new module:
   ```bash
   cd apps/new-module
   go mod init github.com/S-Corkum/devops-mcp/apps/new-module
   ```
3. Add it to the workspace:
   ```bash
   cd ../..
   go work use ./apps/new-module
   ```

## Best Practices

1. **Interface Definitions**: Define interfaces in `pkg/` to ensure consistency
2. **Implementation Independence**: Keep application-specific implementations in their respective modules
3. **Dependency Direction**: Prefer `apps/` modules depending on `pkg/`, not vice versa
4. **Adapter Usage**: Use adapters to bridge interface differences between modules
5. **Standardized Testing**: Follow consistent testing patterns across modules
6. **Configuration Management**: Use similar configuration approaches in all modules
