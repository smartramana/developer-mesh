# MCP Server Development Guide

This guide provides information for developers who want to contribute to or extend the MCP Server.

## Development Environment Setup

### Prerequisites

- Go 1.20 or higher
- Docker and Docker Compose
- Git
- Make (optional, but recommended)
- An IDE with Go support (VS Code, GoLand, etc.)

### Setting Up the Development Environment

1. Clone the repository:

```bash
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server
```

2. Install Go dependencies:

```bash
go mod download
go mod tidy
```

3. Setup configuration:

```bash
cp configs/config.yaml.template configs/config.yaml
```

4. Edit the configuration for your development environment.

5. Start the local development environment:

```bash
# Using make commands
make local-dev-setup

# Or start dependencies manually
docker-compose up -d postgres redis
```

### Directory Structure

The MCP Server follows a standard Go project layout:

```
mcp-server/
├── cmd/                    # Command-line applications
│   ├── mockserver/        # Mock server entry point
│   └── server/            # MCP Server entry point
├── configs/                # Configuration files
│   ├── config.yaml        # Main configuration file
│   └── config.yaml.template  # Configuration template
├── docs/                   # Documentation
├── internal/               # Internal packages (not importable)
│   ├── adapters/          # External system adapters
│   │   ├── github/        # GitHub adapter
│   │   ├── harness/       # Harness adapter
│   │   ├── sonarqube/     # SonarQube adapter
│   │   ├── artifactory/   # Artifactory adapter
│   │   ├── xray/          # Xray adapter
│   │   └── factory/       # Adapter factory
│   ├── api/               # API server
│   ├── cache/             # Cache implementation
│   ├── config/            # Configuration handling
│   ├── core/              # Core engine
│   ├── database/          # Database implementation
│   └── metrics/           # Metrics collection
├── pkg/                    # Public packages (importable)
│   ├── models/            # Data models
│   └── mcp/               # MCP protocol definition
├── scripts/               # Build and deployment scripts
├── test/                  # Test files and fixtures
├── docker-compose.yml     # Docker Compose configuration
├── Dockerfile             # Main Dockerfile
├── Dockerfile.mockserver  # Mock server Dockerfile
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── LICENSE                # License file
├── Makefile               # Makefile for common tasks
└── README.md              # Project README
```

## Building the Server

### Building with Go

To build the MCP Server using Go:

```bash
# Using make
make build

# Or using Go directly
go build -o mcp-server ./cmd/server
```

### Building the Mock Server

To build the mock server:

```bash
# Using make
make mockserver-build

# Or using Go directly
go build -o mockserver ./cmd/mockserver
```

### Building with Docker

To build the MCP Server using Docker:

```bash
# Using make
make docker-build

# Or using Docker directly
docker build -t mcp-server .
```

## Running Tests

### Running Unit Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Running Integration Tests

Integration tests require dependencies like PostgreSQL and Redis:

```bash
# Start dependencies
docker-compose up -d postgres redis

# Run integration tests
go test -tags=integration ./...
```

## Making Changes

### Code Style Guidelines

The MCP Server follows standard Go code style guidelines:

- Use `gofmt` or `goimports` to format code
- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use meaningful variable and function names
- Write godoc-style comments for exported functions and types
- Keep functions focused and reasonably sized

### Creating a New Feature

To create a new feature:

1. Create a new branch from `main`:

```bash
git checkout -b feature/your-feature-name
```

2. Implement your changes, following the project structure and guidelines.

3. Add tests for your feature.

4. Run existing tests to ensure nothing breaks:

```bash
go test ./...
```

5. Commit your changes with a descriptive message.

6. Submit a pull request.

### Fixing a Bug

To fix a bug:

1. Create a new branch from `main`:

```bash
git checkout -b fix/bug-description
```

2. Implement the fix.

3. Add a test that confirms the bug is fixed.

4. Run existing tests:

```bash
go test ./...
```

5. Commit your changes with a descriptive message that references the issue.

6. Submit a pull request.

## Adding a New Adapter

The MCP Server is designed to be extensible. To add a new adapter for a new external system:

1. Create a new package in `internal/adapters/yourservice`.

2. Create a configuration struct for your adapter:

```go
// Config holds configuration for the YourService adapter
type Config struct {
    APIToken       string        `mapstructure:"api_token"`
    WebhookSecret  string        `mapstructure:"webhook_secret"`
    RequestTimeout time.Duration `mapstructure:"request_timeout"`
    MaxRetries     int           `mapstructure:"max_retries"`
    RetryDelay     time.Duration `mapstructure:"retry_delay"`
    MockResponses  bool          `mapstructure:"mock_responses"`
    MockURL        string        `mapstructure:"mock_url"`
}
```

3. Implement the Adapter interface:

```go
// Adapter implements the YourService integration
type Adapter struct {
    client       *yourservice.Client
    config       Config
    subscribers  map[string][]func(interface{})
    subscriberMu sync.RWMutex
    httpClient   *http.Client
    baseAdapter  adapters.BaseAdapter
}

// NewAdapter creates a new YourService adapter
func NewAdapter(cfg Config) (*Adapter, error) {
    // Implementation...
}

// Initialize initializes the adapter
func (a *Adapter) Initialize(ctx context.Context, config interface{}) error {
    // Implementation...
}

// GetData retrieves data from YourService
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
    // Implementation...
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
    // Implementation...
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
    // Implementation...
}

// HandleWebhook processes YourService webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
    // Implementation...
}

// Subscribe adds a callback for a specific event type
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
    // Implementation...
}
```

4. Create a webhook handler in `internal/api/webhooks.go`:

```go
// yourServiceWebhookHandler handles webhooks from YourService
func (s *Server) yourServiceWebhookHandler(c *gin.Context) {
    // Implementation...
}
```

5. Add the adapter configuration to the engine configuration in `internal/config/config.go`:

```go
// Update the EngineConfig struct
type EngineConfig struct {
    // Existing fields...
    YourService YourServiceConfig `mapstructure:"yourservice"`
}
```

6. Update the Core Engine to initialize your adapter in `internal/core/engine.go`:

```go
// Update the initializeAdapters method
func (e *Engine) initializeAdapters() error {
    // Existing code...

    // Initialize YourService adapter if configured
    if e.config.YourService.APIToken != "" {
        if e.adapters["yourservice"], err = yourservice.NewAdapter(e.config.YourService); err != nil {
            return err
        }
        if err = e.setupYourServiceEventHandlers(e.adapters["yourservice"].(*yourservice.Adapter)); err != nil {
            return err
        }
    }

    return nil
}

// Add a setupYourServiceEventHandlers method
func (e *Engine) setupYourServiceEventHandlers(adapter *yourservice.Adapter) error {
    // Implementation...
}

// Add a processYourServiceEvent method
func (e *Engine) processYourServiceEvent(ctx context.Context, event mcp.Event) {
    // Implementation...
}
```

7. Update the API server to register your webhook endpoint in `internal/api/server.go`:

```go
// Update the setupRoutes method
func (s *Server) setupRoutes() {
    // Existing code...

    // Setup YourService webhook endpoint if enabled
    if s.config.Webhooks.YourService.Enabled {
        path := "/yourservice"
        if s.config.Webhooks.YourService.Path != "" {
            path = s.config.Webhooks.YourService.Path
        }
        webhook.POST(path, s.yourServiceWebhookHandler)
    }
}
```

8. Add tests for your adapter in `internal/adapters/yourservice/yourservice_test.go`.

9. Update the mock server to include your service in `cmd/mockserver/main.go`:

```go
// Add a mock handler for your service
http.HandleFunc("/mock-yourservice/", func(w http.ResponseWriter, r *http.Request) {
    log.Printf("Mock YourService request: %s %s", r.Method, r.URL.Path)
    w.Header().Set("Content-Type", "application/json")
    response := map[string]interface{}{
        "success": true,
        "message": "Mock YourService response",
        "timestamp": time.Now().Format(time.RFC3339),
    }
    json.NewEncoder(w).Encode(response)
})
```

10. Update documentation to include your new adapter.

## Debugging

### Debugging with Logger

The MCP Server uses the standard Go logger. You can add debug logging:

```go
log.Printf("Debug: variable=%v", someVariable)
```

For more structured logging, consider using a more advanced logging library.

### Debugging with Metrics

Use metrics to track important values and operations:

```go
// Record an API call
metricsClient.RecordAPICall("yourservice", "operation", startTime)

// Record an event
metricsClient.RecordEvent("yourservice", "event_type")
```

### Debugging with Tests

Write focused tests to debug specific functionality:

```go
func TestYourFunction(t *testing.T) {
    result := YourFunction(input)
    if result != expected {
        t.Errorf("YourFunction(%v) = %v, want %v", input, result, expected)
    }
}
```

## Continuous Integration

The MCP Server uses CI workflows for:

- Running tests on pull requests
- Building Docker images
- Running code quality checks
- Generating documentation

## Release Process

To create a release:

1. Update version information in the codebase
2. Create a release branch: `release/v1.0.0`
3. Run all tests and ensure they pass
4. Build the Docker image with the version tag
5. Tag the commit: `git tag v1.0.0`
6. Push the tag: `git push origin v1.0.0`
7. Create a GitHub release with release notes

## Development Tools

### Useful Make Commands

The Makefile includes several useful commands:

- `make build`: Build the MCP Server
- `make mockserver-build`: Build the mock server
- `make test`: Run all tests
- `make clean`: Clean build artifacts
- `make local-dev`: Run the MCP Server with mock server
- `make docker-build`: Build the Docker image
- `make docker-compose-up`: Start all services with Docker Compose
- `make init-config`: Initialize the configuration file

### Recommended VS Code Extensions

If you're using VS Code, these extensions are recommended:

- Go (official Go extension)
- Go Test Explorer
- Go Coverage
- Docker
- YAML
- GitLens

## Documentation Guidelines

When adding new features or making changes, update the documentation:

1. Update the relevant markdown files in the `docs` directory
2. Add or update code comments for exported functions and types
3. Keep the README updated with key information
4. Include examples for complex functionality