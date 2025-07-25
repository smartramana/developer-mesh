# OpenAPI/Swagger Documentation

This directory contains the comprehensive OpenAPI 3.0 specification for the Developer Mesh Platform API. The documentation is structured to support scalability as new tools and integrations are added.

## Structure

```
docs/swagger/
├── openapi.yaml              # Main OpenAPI specification with references
├── common/                   # Shared components
│   ├── schemas.yaml         # Common data models
│   ├── parameters.yaml      # Reusable parameters
│   └── responses.yaml       # Standard HTTP responses
├── core/                    # Core API specifications
│   ├── agents.yaml         # Agent management endpoints
│   ├── auth.yaml           # Authentication endpoints
│   ├── collaborations.yaml # Collaboration features
│   ├── contexts.yaml       # Context management
│   ├── embeddings_v2.yaml  # Embedding operations
│   ├── health.yaml         # Health and monitoring
│   ├── metrics.yaml        # Metrics endpoints
│   ├── models.yaml         # Model configuration
│   ├── monitoring.yaml     # Monitoring endpoints
│   ├── relationships.yaml  # Entity relationships
│   ├── search.yaml         # Semantic search
│   ├── tasks.yaml          # Task management
│   ├── tools.yaml          # Generic tool endpoints
│   ├── vectors.yaml        # Vector operations
│   ├── webhooks.yaml       # Webhook endpoints
│   └── workflows.yaml      # Workflow orchestration
└── tools/                   # Tool-specific specifications
    └── github/             # GitHub integration
        └── api.yaml       # GitHub endpoints (ORPHANED - not implemented)
```

**Note**: GitHub, Harness, and SonarQube tools are implemented through the generic tool endpoints (`/api/v1/tools/{tool}/actions/{action}`). The tool-specific endpoints documented in `tools/github/api.yaml` are not implemented.

## Design Principles

### 1. Modularity
- Each tool has its own directory under `tools/`
- Common components are reused via `$ref` references
- Tool-specific schemas are isolated from core schemas

### 2. Scalability
- New tools can be added by creating a new directory under `tools/`
- Each tool defines its own endpoints, schemas, and examples
- The main `openapi.yaml` imports tool specifications dynamically

### 3. Consistency
- All tools follow the same API patterns
- Common responses and error formats are standardized
- Authentication and rate limiting are handled uniformly

### 4. MCP Protocol Compliance
- Context management follows MCP specification
- Tool execution returns MCP-compliant responses
- Error handling matches MCP protocol requirements

## How Tools Work

Tools in the Developer Mesh Platform are accessed through generic endpoints:

- `GET /api/v1/tools` - List all available tools
- `GET /api/v1/tools/{tool}` - Get tool details (e.g., `/api/v1/tools/github`)
- `GET /api/v1/tools/{tool}/actions` - List available actions for a tool
- `GET /api/v1/tools/{tool}/actions/{action}` - Get action details
- `POST /api/v1/tools/{tool}/actions/{action}` - Execute a tool action
- `POST /api/v1/tools/{tool}/queries` - Query tool data

Currently implemented tools:
- **github**: Repository management, issues, pull requests
- **harness**: CI/CD pipeline operations
- **sonarqube**: Code quality and security scanning

Tool-specific endpoint patterns (like `/tools/github/{tool_name}`) are not implemented.

## Viewing Documentation

### Local Development

1. **Using Make command:**
```bash
make swagger-serve
# Opens at http://localhost:8082/
```

2. **Using Python:**
```bash
cd docs/swagger
python3 -m http.server 8080
# Opens at http://localhost:8080/
```

3. **Using the API servers:**
- MCP Server: http://localhost:8080/swagger/index.html
- REST API: http://localhost:8081/swagger/index.html

### Online Tools

1. **Swagger Editor:**
   - Visit https://editor.swagger.io/
   - Import `openapi.yaml`

2. **ReDoc:**
   - Use the `/redoc` endpoint on either server
   - Or visit https://redocly.github.io/redoc/

## Validation

Validate the OpenAPI specification:

```bash
# Install OpenAPI validator
npm install -g @apidevtools/swagger-cli

# Validate
swagger-cli validate docs/swagger/openapi.yaml
```

**Note**: The validator may report warnings about external file references. These are expected when using modular file structure with `$ref` references.

## Code Generation

Generate client SDKs or server stubs:

```bash
# Install OpenAPI Generator
brew install openapi-generator

# Generate Go client
openapi-generator generate -i docs/swagger/openapi.yaml -g go -o sdk/go

# Generate Python client
openapi-generator generate -i docs/swagger/openapi.yaml -g python -o sdk/python
```

## Best Practices

1. **Use References**: Don't duplicate schemas, use `$ref` to reference common components
2. **Provide Examples**: Include request/response examples for all endpoints
3. **Document Errors**: Define all possible error responses with examples
4. **Version APIs**: Use path versioning (`/api/v1/`) for backward compatibility
5. **Security First**: Always define security requirements for endpoints
6. **Test with Mock Data**: Use example data that reflects real-world usage

## Tool-Specific Guidelines

### GitHub Integration
- Follow GitHub API conventions for resource names
- Use GitHub's standard error codes
- Include webhook event schemas

### CI/CD Tools (Harness, GitLab)
- Define pipeline schemas clearly
- Include status enumerations
- Document async operations
- Support both trigger and monitoring endpoints

### Security Tools (SonarQube, Xray)
- Define vulnerability severity levels
- Include remediation guidance schemas
- Document scan result formats

### Artifact Repositories (Artifactory)
- Follow repository naming conventions
- Include artifact metadata schemas
- Document search query formats

## Current State

As of the latest review:
- All core API endpoints are properly documented
- Authentication is handled through API keys and JWT tokens (no OAuth2 login endpoint)
- Context endpoints use `/contexts` path (not `/mcp/context`)
- Tools are accessed through generic endpoints, not tool-specific paths
- The `tools/github/api.yaml` file documents endpoints that are not implemented

## Maintenance

1. **Keep specs in sync**: Update OpenAPI specs when implementing changes
2. **Version control**: Track all changes in Git
3. **Review process**: Include API documentation in code reviews
4. **Breaking changes**: Document in CHANGELOG and migration guides
5. **Deprecation**: Use OpenAPI deprecation markers with sunset dates
6. **Regular audits**: Periodically verify that documentation matches implementation