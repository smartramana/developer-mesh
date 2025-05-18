# Development Environment Setup

This guide explains how to set up a development environment for working on the DevOps MCP project.

## Prerequisites

- Go 1.19 or later
- Docker and Docker Compose
- Git
- Your preferred code editor (VS Code recommended)

## Setting Up Your Dev Environment

### 1. Clone the Repository

```bash
git clone https://github.com/S-Corkum/devops-mcp.git
cd devops-mcp
```

### 2. Initialize the Go Workspace

The project uses Go's workspace feature to manage multiple modules:

```bash
# Verify the workspace configuration
make check-workspace

# Sync dependencies
make sync
```

### 3. Start the Development Services

Start the required services using Docker Compose:

```bash
make dev-setup
```

This will start PostgreSQL with pgvector extension, Redis, LocalStack, and other required services.

### 4. Building the Applications

Build all applications:

```bash
# Build everything
make build

# Build a specific application
make build-mcp-server
make build-rest-api
make build-worker
```

### 5. Running the Applications

Run the applications individually for development:

```bash
# Run MCP Server
make run-mcp-server

# Run REST API
make run-rest-api

# Run Worker
make run-worker
```

Alternatively, use the Docker Compose setup:

```bash
make docker-compose-up
```

## Recommended VS Code Extensions

For an optimal development experience with VS Code, install the following extensions:

- **Go**: Official Go extension
- **Go Test Explorer**: For managing Go tests
- **Docker**: For Docker integration
- **PostgreSQL**: For database connectivity
- **REST Client**: For testing API endpoints
- **YAML**: For Docker Compose and configuration files

## Go Workspace Organization

The project is organized as a Go workspace with multiple modules:

- `apps/mcp-server`: MCP server implementation
- `apps/rest-api`: REST API implementation
- `apps/worker`: Worker service implementation
- `pkg/`: Shared packages used across modules

To switch between modules in development:

```bash
cd apps/rest-api  # Change to the REST API module
go test ./...     # Run tests in that module
```

## Understanding Adapters

The project makes extensive use of the adapter pattern to bridge between different interfaces. When developing new features:

1. Identify which interfaces need to be implemented
2. Determine if an adapter is needed
3. Implement the adapter using the established patterns
4. Write tests that validate the adapter behavior

For more details, see the [Adapter Pattern](../architecture/adapter-pattern.md) documentation.

## Code Style and Standards

The project follows standard Go code style conventions:

- Use `gofmt` or `goimports` to format code
- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Keep functions small and focused
- Document exported functions and types
- Write unit tests for new functionality

## Working with Vector Embedding Data

When developing with vector embeddings:

1. Ensure PostgreSQL has the pgvector extension installed
2. Use the repository interfaces for storing and retrieving embeddings
3. Test vector searches with appropriate similarity thresholds

## Debugging Tips

See the [Debugging Guide](debugging-guide.md) for detailed information on debugging issues in development.
