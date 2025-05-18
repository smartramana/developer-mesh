# MCP-Server: Model Context Protocol Server

![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.19+-00ADD8.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

MCP-Server is a powerful Go-based application that implements the Model Context Protocol, enabling AI systems to communicate with external tools, APIs, and data sources through a standardized interface.

## Table of Contents

1. [Architecture](#architecture)
2. [Core Components](#core-components)
3. [API Structure](#api-structure)
4. [Proxy System & Adapter Pattern](#proxy-system--adapter-pattern)
5. [Configuration](#configuration)
6. [Deployment](#deployment)
7. [Security](#security)
8. [API Endpoints](#api-endpoints)
9. [Extending the Application](#extending-the-application)
10. [Troubleshooting](#troubleshooting)

## Architecture

MCP-Server follows a clean, layered architecture:

```
┌─────────────────────────────────────────────────────────┐
│                     API Layer                           │
│  ┌─────────────┐ ┌──────────────┐  ┌────────────────┐   │
│  │  REST API   │ │  MCP API     │  │  Tool API      │   │
│  └─────────────┘ └──────────────┘  └────────────────┘   │
├─────────────────────────────────────────────────────────┤
│                    Proxy Layer                          │
│  ┌─────────────┐ ┌──────────────┐  ┌────────────────┐   │
│  │ AgentProxy  │ │ SearchProxy  │  │ ModelProxy     │   │
│  └─────────────┘ └──────────────┘  └────────────────┘   │
├─────────────────────────────────────────────────────────┤
│                   Core Engine                           │
├─────────────────────────────────────────────────────────┤
│                External Dependencies                    │
│  ┌─────────────┐ ┌──────────────┐  ┌────────────────┐   │
│  │  Database   │ │  AWS Services│  │ Vector Storage │   │
│  └─────────────┘ └──────────────┘  └────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

The server utilizes a modular design with clear separation of concerns between components.

## Core Components

### 1. Main Entry Point (`cmd/server/main.go`)

This is the application's entry point, responsible for:
- Loading and validating configuration
- Initializing core services (database, logging, metrics)
- Setting up AWS credentials when using IAM authentication
- Starting the HTTP/HTTPS server
- Handling graceful shutdown

### 2. Server Component (`internal/api/server.go`)

The server component manages:
- Gin router initialization and configuration
- Middleware setup (compression, caching, rate limiting)
- API versioning
- Route registration
- Health checks and monitoring
- TLS configuration

### 3. Core Engine (`internal/core`)

The engine is the central processing unit that:
- Coordinates between different subsystems
- Manages application state
- Provides service health checks
- Delivers core business logic

## API Structure

### API Layers

The MCP-Server exposes several API layers:

#### 1. MCP API (`internal/api/mcp_api.go`)

Provides endpoints for context management:
- Creating, retrieving, updating, and deleting contexts
- Listing contexts
- Searching within contexts
- Generating context summaries

#### 2. Tool API (`internal/api/tool_api.go`)

Handles DevOps tool operations:
- Executing tool actions
- Querying tool data
- Listing available tools and actions
- Provides detailed information about tools and their capabilities

Supported tools include:
- GitHub (repository management, issues, PRs)
- Harness (CI/CD pipelines)
- SonarQube (code quality)
- Artifactory (artifact management)
- Xray (security scanning)

#### 3. Search API

Enables sophisticated search capabilities:
- Text-based search
- Vector-based search
- "More like this" content search based on IDs
- Configurable filtering and sorting

## Proxy System & Adapter Pattern

MCP-Server employs the adapter pattern to resolve interface incompatibility issues, particularly for API implementations that expect specific method names.

### Adapter Implementation Pattern

The proxy system is implemented in `internal/api/proxies/` and includes:

#### Agent Proxy (`agent_proxy.go`)
- Adapts repository.AgentRepository interface to REST API client
- Translates between internal models and API formats
- Handles method name discrepancies (e.g., `Create` vs `CreateAgent`)

#### Search Proxy (`search_proxy.go`)
- Implements repository.SearchRepository interface
- Transforms search options between systems
- Uses reflection for safe field access to handle version differences
- Converts between embedding.SearchResults and repository.SearchResults

#### Model Proxy (`model_proxy.go`)
- Adapts repository.ModelRepository to REST API
- Provides consistent interface regardless of backend implementation

This pattern provides several benefits:
1. Maintains backward compatibility with existing API code
2. Enables clear separation between repository and API layers
3. Provides proper type safety with explicit conversions
4. Improves maintainability by isolating interface differences

Example adapter method:
```go
// ServerEmbeddingAdapter implements the API's expected interface
// while internally delegating to the repository implementation
type ServerEmbeddingAdapter struct {
    repo repository.VectorAPIRepository
}

// API expects this method signature
func (a *ServerEmbeddingAdapter) StoreEmbedding(ctx context.Context, vector *models.Vector) error {
    // Convert from models.Vector to repository.Embedding
    repoEmbedding := &repository.Embedding{
        ID:           vector.ID,
        ContextID:    vector.TenantID,    // Field name mapping
        ContentIndex: extractContentIndex(vector.Metadata),
        Text:         vector.Content,     // Field name mapping
        Embedding:    vector.Embedding,
        ModelID:      extractModelID(vector.Metadata),
    }
    
    // Delegate to the repository implementation
    return a.repo.StoreEmbedding(ctx, repoEmbedding)
}
```

## Configuration

MCP-Server uses a comprehensive configuration system that supports:
- Environment-specific settings
- Infrastructure configuration (database, AWS)
- API settings (timeouts, rate limits)
- Security options (TLS, webhooks)
- Observability settings (logging, metrics)

### Configuration Validation

The server performs validation at startup to ensure:
- Database is properly configured
- API timeouts are reasonable
- Webhook security is applied when enabled
- Critical settings are present and valid

### AWS Integration

The server supports advanced AWS integration:
- IAM authentication for RDS
- IRSA (IAM Roles for Service Accounts) detection
- Secure token generation for database access

## Deployment

### Prerequisites

- Go 1.19 or later
- PostgreSQL database
- Vector DB (optional)
- AWS credentials (if using AWS services)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/S-Corkum/devops-mcp.git

# Change to app directory
cd devops-mcp/apps/mcp-server

# Build the server
make build

# Or use Go directly
go build -o mcp-server ./cmd/server
```

### Running in Docker

The repository includes a Dockerfile for containerized deployment:

```bash
# Build Docker image
docker build -t mcp-server .

# Run container
docker run -p 8080:8080 \
  -e DATABASE_HOST=postgres \
  -e DATABASE_PORT=5432 \
  -e DATABASE_NAME=mcp \
  -e DATABASE_USER=postgres \
  -e DATABASE_PASSWORD=your_password \
  mcp-server
```

### Environment Variables

Key environment variables include:

| Variable | Description | Default |
|----------|-------------|---------|
| PORT | Server port | 8080 |
| ENVIRONMENT | Environment (dev/staging/prod) | dev |
| DATABASE_HOST | Database hostname | localhost |
| DATABASE_PORT | Database port | 5432 |
| DATABASE_NAME | Database name | mcp |
| DATABASE_USER | Database username | postgres |
| DATABASE_PASSWORD | Database password | - |
| AWS_RDS_USE_IAM_AUTH | Use IAM auth for RDS | false |
| API_RATE_LIMIT_ENABLED | Enable rate limiting | false |
| API_ENABLE_CORS | Enable CORS | true (in dev) |

## Security

MCP-Server includes numerous security features:

### Authentication

- API key authentication
- JWT token support
- AWS IAM integration

### Request Protection

- Rate limiting to prevent abuse
- Input validation on all endpoints
- Proper error handling to avoid information leakage

### Transport Security

- TLS/HTTPS support
- Modern cipher configuration
- HTTP security headers

### Webhook Security

- Secret verification
- IP validation
- Event filtering

### Tool Action Safety

All tool actions are carefully designed with safety in mind:
- Read-only operations are prioritized
- Destructive operations are limited or disabled
- Clear documentation of safety restrictions

## API Endpoints

### Core Endpoints

- `GET /health` - Server health check
- `GET /metrics` - Prometheus metrics
- `GET /swagger/*` - API documentation

### API v1 Endpoints

- `GET /api/v1/` - API version info

#### MCP Endpoints

- `POST /api/v1/mcp/context` - Create context
- `GET /api/v1/mcp/context/:id` - Get context by ID
- `PUT /api/v1/mcp/context/:id` - Update context
- `DELETE /api/v1/mcp/context/:id` - Delete context
- `GET /api/v1/mcp/contexts` - List contexts
- `POST /api/v1/mcp/context/:id/search` - Search in context
- `GET /api/v1/mcp/context/:id/summary` - Get context summary

#### Tool Endpoints

- `GET /api/v1/tools` - List available tools
- `GET /api/v1/tools/:tool` - Get tool details
- `GET /api/v1/tools/:tool/actions` - List tool actions
- `GET /api/v1/tools/:tool/actions/:action` - Get action details
- `POST /api/v1/tools/:tool/actions/:action` - Execute action
- `POST /api/v1/tools/:tool/queries` - Query tool data

## Extending the Application

### Adding New Tools

To add a new tool:
1. Create a new tool client in `pkg/client/`
2. Add tool actions to the ToolAPI handler
3. Implement necessary adapters in `internal/api/proxies/`
4. Update authorization rules for new actions

### Supporting New Search Providers

1. Update the SearchAPIProxy to support new provider
2. Add conversion logic for provider-specific result formats
3. Register the provider in the search service initialization

### Custom Middleware

The server supports custom middleware registration for:
- Authentication
- Logging
- Metrics
- Request transformation

## Troubleshooting

### Common Issues

1. **Database Connection Errors**
   - Verify database credentials and connection string
   - Check network connectivity and firewall rules
   - For IAM auth, verify proper role assignment

2. **API Authentication Failures**
   - Verify API keys are properly configured
   - Check JWT token expiration and signature
   - Review authorization headers

3. **Proxy Failures**
   - Check REST API client configuration
   - Verify endpoint URLs and credentials
   - Review adapter implementation for interface mismatches

### Logging

MCP-Server uses structured logging with multiple levels:
- ERROR - Critical errors requiring immediate attention
- WARN - Potential issues that don't stop operation
- INFO - Normal operational information
- DEBUG - Detailed troubleshooting information

### Monitoring

- Health endpoint (`/health`) provides component-level health status
- Prometheus metrics available at `/metrics`
- Structured logs can be parsed by log aggregators

## License

MIT License - See LICENSE file for details.

## Contributing

Contributions are welcome! Please see our contribution guidelines for details.
