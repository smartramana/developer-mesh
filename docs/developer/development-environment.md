# Development Environment Setup

This guide provides a comprehensive setup for developing on the DevOps MCP platform with modern tooling and best practices.

## Prerequisites

### Required Tools
- **Go 1.24+** - Required for workspace support
- **Docker 24+** & **Docker Compose v2** - For local services
- **Git 2.40+** - For version control
- **Make** - For build automation

### Recommended Tools
- **VS Code** or **GoLand** - IDE with Go support
- **golangci-lint 1.61+** - For code quality
- **go-task** - Alternative to Make
- **direnv** - For environment management
- **k9s** - For Kubernetes development

## Quick Start (5 minutes)

```bash
# Clone and setup
git clone https://github.com/S-Corkum/devops-mcp.git
cd devops-mcp

# One-command setup
make dev-setup

# Verify everything works
make test-quick
```

## Detailed Setup

### 1. Repository Setup

```bash
# Clone with submodules if any
git clone --recursive https://github.com/S-Corkum/devops-mcp.git
cd devops-mcp

# Setup git hooks
cp scripts/hooks/* .git/hooks/
chmod +x .git/hooks/*
```

### 2. Environment Configuration

```bash
# Copy environment template
cp .env.example .env

# Edit with your settings
vim .env

# Required environment variables
export MCP_ENV=development
export DATABASE_URL=postgres://mcp:mcp@localhost:5432/devops_mcp?sslmode=disable
export REDIS_URL=redis://localhost:6379
export AWS_ENDPOINT=http://localhost:4566  # LocalStack
```

### 3. Go Workspace Setup

```bash
# Verify Go version
go version  # Should be 1.24+

# Initialize workspace
go work sync

# Download dependencies
go mod download -x

# Verify workspace
go work edit -json | jq .
```

### 4. Local Services

```bash
# Start all services
make dev-setup

# Or start individually:
docker-compose -f docker-compose.local.yml up -d postgres
docker-compose -f docker-compose.local.yml up -d redis
docker-compose -f docker-compose.local.yml up -d localstack

# Verify services
make health-check

# View logs
make logs service=postgres
```

### 5. Database Setup

```bash
# Run migrations
make migrate-up

# Seed development data
make seed-dev

# Verify database
make db-status
```

### 6. Build & Run

```bash
# Build all services
make build-all

# Run with hot reload
make dev service=mcp-server
make dev service=rest-api
make dev service=worker

# Or run all with Docker Compose
make up
```

## IDE Setup

### VS Code Configuration

```json
// .vscode/settings.json
{
    "go.useLanguageServer": true,
    "go.lintTool": "golangci-lint",
    "go.lintFlags": ["--fast"],
    "go.testFlags": ["-v", "-race"],
    "go.testTimeout": "30s",
    "go.buildTags": "integration",
    "gopls": {
        "experimentalWorkspaceModule": true,
        "ui.semanticTokens": true,
        "ui.completion.usePlaceholders": true
    },
    "files.associations": {
        "*.go.tmpl": "go",
        "go.work": "go.work"
    }
}
```

### Essential Extensions

#### VS Code
- **golang.go** - Official Go support
- **ms-vscode-remote.remote-containers** - Dev containers
- **ms-azuretools.vscode-docker** - Docker support
- **mtxr.sqltools** - Database explorer
- **humao.rest-client** - API testing
- **redhat.vscode-yaml** - YAML support
- **eamodio.gitlens** - Git superpowers

#### GoLand
- Built-in Go support
- Database tools
- HTTP client
- Docker integration

## Project Structure

```
devops-mcp/
├── apps/                    # Application modules
│   ├── mcp-server/         # MCP protocol server
│   │   ├── cmd/            # Entry points
│   │   ├── internal/       # Private packages
│   │   └── go.mod         # Module definition
│   ├── rest-api/          # REST API service
│   └── worker/            # Async worker service
├── pkg/                    # Shared packages
│   ├── adapters/          # Tool adapters
│   ├── models/            # Domain models
│   └── observability/     # Logging, metrics, tracing
├── configs/               # Configuration files
├── scripts/              # Development scripts
├── test/                 # E2E tests
└── go.work              # Workspace definition
```

### Working with Modules

```bash
# Add new dependency to specific module
cd apps/mcp-server
go get github.com/some/package

# Run tests for specific module
cd apps/rest-api
go test -v ./...

# Update all modules
go work sync

# Add new module to workspace
go work use ./apps/new-service
```

## Development Workflow

### 1. Feature Development

```bash
# Create feature branch
git checkout -b feature/my-feature

# Make changes with TDD
# 1. Write failing test
vim internal/feature_test.go

# 2. Run test (should fail)
go test ./internal -run TestFeature

# 3. Implement feature
vim internal/feature.go

# 4. Run test (should pass)
go test ./internal -run TestFeature

# 5. Run all tests
make test

# 6. Lint and format
make lint
make fmt
```

### 2. Debugging

```bash
# Run with debug logging
LOG_LEVEL=debug go run ./cmd/server

# Use delve debugger
dlv debug ./cmd/server

# Attach to running process
dlv attach $(pgrep mcp-server)

# Debug tests
dlv test ./internal/... -- -test.run TestSpecific
```

### 3. Testing Pyramid

```bash
# Unit tests (fast, isolated)
go test ./internal/...

# Integration tests (with dependencies)
go test ./... -tags=integration

# E2E tests (full stack)
cd test/functional
go test ./...

# Benchmarks
go test -bench=. -benchmem ./...

# Fuzzing
go test -fuzz=FuzzParser -fuzztime=10s ./internal/parser
```

## Code Standards

### Style Guide

```go
// Package names: lowercase, no underscores
package adapters

// Interface names: end with -er when possible
type Adapter interface {
    Execute(ctx context.Context, req Request) (Response, error)
}

// Struct initialization: use field names
config := &Config{
    URL:     "https://api.example.com",
    Timeout: 30 * time.Second,
}

// Error handling: wrap with context
if err != nil {
    return fmt.Errorf("execute request: %w", err)
}

// Context usage: always first parameter
func (s *Service) Process(ctx context.Context, data []byte) error {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    // Process...
}
```

### Testing Standards

```go
// Table-driven tests
func TestAdapter_Execute(t *testing.T) {
    tests := []struct {
        name    string
        input   Request
        want    Response
        wantErr bool
    }{
        {
            name:  "valid request",
            input: Request{ID: "123"},
            want:  Response{Status: "success"},
        },
        {
            name:    "invalid request",
            input:   Request{},
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Parallel safe
            t.Parallel()
            
            // Test implementation
            got, err := adapter.Execute(context.Background(), tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Execute() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Local Development Tips

### Hot Reload

```bash
# Install air for hot reload
go install github.com/cosmtrek/air@latest

# Run with hot reload
air -c .air.toml
```

### Database Management

```bash
# Connect to local database
make db-shell

# Run SQL migrations
make migrate-up

# Rollback last migration
make migrate-down

# Reset database
make db-reset
```

### Performance Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Trace execution
go test -trace=trace.out
go tool trace trace.out
```

### Troubleshooting Common Issues

```bash
# Module issues
go clean -modcache
go mod download

# Workspace issues
go work sync
go work use -r .

# Docker issues
docker system prune -a
make clean-docker

# Port conflicts
lsof -i :8080
kill -9 $(lsof -t -i:8080)
```

## Security Considerations

### Secret Management

```bash
# Never commit secrets
echo '.env' >> .gitignore

# Use environment variables
export GITHUB_TOKEN=$(pass show github/token)

# Or use direnv
echo 'export GITHUB_TOKEN="..."' >> .envrc
direnv allow
```

### Dependency Security

```bash
# Check for vulnerabilities
go list -json -m all | nancy sleuth

# Update dependencies safely
go get -u ./...
go mod tidy
go test ./...
```

## Next Steps

1. **Run Tests**: `make test` to ensure everything works
2. **Explore Code**: Start with `cmd/server/main.go`
3. **Read Docs**: Check [architecture docs](../architecture/)
4. **Contribute**: See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

*Questions? Join our [Discord](https://discord.gg/devops-mcp) or open an [issue](https://github.com/S-Corkum/devops-mcp/issues)*
