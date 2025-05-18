# DevOps MCP (Model Context Protocol)

![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)
![Go](https://img.shields.io/badge/go-1.19+-00ADD8.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

> Connect AI agents to DevOps tools through a unified, standardized API

## Overview

DevOps MCP (Model Context Protocol) provides a standardized interface for AI agents to interact with DevOps tools, manage context data, and perform vector-based semantic search. This platform bridges the gap between LLM-based AI systems and external tools like GitHub.

![MCP Architecture Overview](docs/assets/images/architecture-overview.png)

## Key Features

- **Unified Tool API**: Standardized REST API for multiple DevOps tools
- **Context Management**: Store and retrieve conversation contexts efficiently
- **Vector Search**: Semantic search capabilities with pgvector integration
- **Adapter Pattern**: Clean interface separation through Go workspace architecture
- **Asynchronous Processing**: Worker-based event processing system

## Quick Start

```bash
# Clone the repository
git clone https://github.com/S-Corkum/devops-mcp.git
cd devops-mcp

# Start local development environment
make dev-setup

# Test the API
curl http://localhost:8080/health
```

## Documentation

For comprehensive documentation, please visit our [Documentation Index](docs/README.md):

- [Getting Started Guide](docs/getting-started/README.md) - Start here for new users
- [Architecture Overview](docs/architecture/README.md) - System design and components
- [Developer Guide](docs/developer/README.md) - For developers contributing to the project
- [API Reference](docs/api-reference/README.md) - Detailed API endpoints and schemas
- [Operations Guide](docs/operations/README.md) - Deployment and maintenance information
- [Troubleshooting](docs/troubleshooting/README.md) - Solutions to common issues

## Repository Structure

This project uses a Go workspace with multiple modules:

```
devops-mcp/
├── apps/                   # Application modules
│   ├── mcp-server/         # Main MCP server module
│   ├── rest-api/           # REST API service
│   └── worker/             # Asynchronous worker service
├── pkg/                    # Shared library code
│   ├── adapters/           # Interface adapters
│   ├── config/             # Configuration management
│   ├── repository/         # Data access layer
│   └── ...
├── docs/                   # Documentation
└── docker-compose.local.yml # Local development setup
```

## Requirements

- Go 1.19+
- Docker and Docker Compose
- PostgreSQL with pgvector extension
- Redis

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please see our [Contributing Guide](docs/contributing/README.md) for details on how to get involved.
