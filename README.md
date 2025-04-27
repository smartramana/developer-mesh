# DevOps MCP Server

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

MCP (Model Context Protocol) Server provides AI agents with a unified API for DevOps tool integrations and context management.

## 🚀 Features

### DevOps Integration
- **Unified API**: Standardized REST API for integrating AI agents with GitHub
- **Tool Operations**: Execute GitHub operations through a consistent interface
- **Event Handling**: Process webhooks from GitHub to keep AI agents informed
- **Tool Discovery**: Dynamically discover available tools and their capabilities

### Context Management
- **Conversation History**: Maintain conversation contexts for AI agents
- **Multi-tiered Storage**: Store contexts efficiently across Redis, PostgreSQL, and S3
- **Context Window Management**: Handle token counting, truncation, and optimization
- **Vector Search**: Find semantically similar content using vector embeddings
- **Session Management**: Track conversations across multiple interactions

### Platform Capabilities
- **Extensible Design**: Modular architecture making it easy to add new tool integrations
- **Resilient Processing**: Built-in retry mechanisms, circuit breakers, and error handling
- **Performance Optimized**: Connection pooling, caching, and concurrency management
- **Comprehensive Authentication**: Secure API access and webhook verification
- **AWS Integration**: Seamless integration with AWS services using IAM Roles for Service Accounts (IRSA)

## 📋 Quick Start

### Prerequisites
- Docker and Docker Compose (for local development)
- GitHub account and personal access token (for GitHub integration)

### Running with Docker Compose

The easiest way to get started is with Docker Compose:

```bash
# Clone the repository
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server

# Create configuration
cp .env.example .env
# Edit .env with your GitHub token and other settings

# Start the services
docker-compose up -d
```

### Verify Installation

```bash
# Check the health endpoint
curl http://localhost:8080/health

# Explore the Swagger UI
open http://localhost:8080/swagger/index.html
```

## 📖 Documentation

For detailed documentation, please see the [Documentation Index](docs/README.md).

### Key Documentation

- [Quick Start Guide](docs/quick-start-guide.md) - Get up and running quickly
- [Installation Guide](docs/installation-guide.md) - Detailed installation instructions
- [Configuration Guide](docs/configuration-guide.md) - Configure the server for your environment
- [AI Agent Integration](docs/guides/ai-agent-integration-guide.md) - Integrate AI agents with the MCP Server
- [API Reference](docs/api-reference.md) - Full API reference documentation
- [System Architecture](docs/system-architecture.md) - Understand the system architecture

## 👩‍💻 For Developers

If you're interested in developing with or contributing to the MCP Server:

- [Development Guide](docs/development-guide.md) - Setup your development environment
- [Adding New Integrations](docs/adding-new-integrations.md) - Add new tool integrations
- [Contributing Guide](CONTRIBUTING.md) - Guidelines for contributing to the project

## 🛠️ Building from Source

```bash
# Clone the repository
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server

# Install dependencies
go mod download

# Build the server
make build
# or
go build -o mcp-server ./cmd/server
```

## 🔒 Security

MCP Server takes security seriously:

- All API endpoints support authentication (JWT or API key)
- Webhook endpoints verify signatures to prevent tampering
- Support for TLS encryption in production environments
- Safety restrictions to prevent destructive operations

Read our [Security Guide](docs/security/production-deployment-security.md) for production deployments.

## 🐞 Troubleshooting

Encountering issues? Check our [Troubleshooting Guide](docs/troubleshooting-guide.md) for solutions to common problems.

## 📊 Monitoring

MCP Server includes built-in monitoring capabilities:

- Prometheus metrics exposed at `/metrics` (public, no authentication required for GET)
- Grafana dashboards for visualizing performance and usage
- Health check endpoint at `/health`

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
