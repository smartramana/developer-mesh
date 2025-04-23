# DevOps MCP Server Documentation

## 🌟 Overview

DevOps MCP (Model Context Protocol) Server serves as a bridge between AI agents and DevOps tools, providing:

1. **Unified API for DevOps Tools**: A standardized protocol for AI agents to interact with GitHub.
2. **Context Management**: Helps AI agents maintain conversation history and track interactions.
3. **Vector Embedding Storage**: Enables semantic search within conversation contexts.

## 📚 Documentation Index

### Getting Started
- [Quick Start Guide](quick-start-guide.md) - Get up and running in minutes
- [Installation Guide](installation-guide.md) - Detailed installation instructions
- [Configuration Guide](configuration-guide.md) - Configuration options and examples
- [Upgrading Guide](upgrading-guide.md) - Guide for upgrading between versions

### User Guides
- [AI Agent Integration Guide](guides/ai-agent-integration.md) - Integrate AI agents with MCP Server
- [GitHub Integration Guide](github-integration-guide.md) - Using the GitHub integration
- [Context Management Guide](context-management-guide.md) - Managing conversation contexts
- [Vector Search Guide](features/vector-search.md) - Using vector embeddings for semantic search
- [Multi-Model Vector Support](features/multi-model-vector-support.md) - Working with different embedding models
- [S3 Storage](features/s3-storage.md) - Using S3 for context storage

### API Reference
- [API Overview](api-reference.md) - Complete API documentation
- [OpenAPI Specification](swagger/context_api.yaml) - OpenAPI 3.0 specification

### Deployment & Operations
- [Deployment Guide](deployment-guide.md) - Production deployment recommendations
- [AWS IRSA Setup](aws/aws-irsa-setup.md) - Setting up IAM Roles for Service Accounts
- [Monitoring Guide](monitoring-guide.md) - Monitoring and alerting
- [Database Migrations](database-migrations.md) - Managing database schema changes
- [Troubleshooting Guide](troubleshooting-guide.md) - Common issues and solutions
- [Security Best Practices](security/production-deployment-security.md) - Security recommendations

### Development
- [System Architecture](system-architecture.md) - System design and architecture
- [Development Guide](development-guide.md) - Guide for developers
- [Adding New Integrations](adding-new-integrations.md) - How to add new tool integrations
- [Contributing Guide](../CONTRIBUTING.md) - How to contribute to the project
- [Testing Guide](testing-guide.md) - Guide for testing the server
- [Integration Testing](integration-testing-guide.md) - Guide for integration testing
- [Event System](event-system.md) - Understanding the event system
- [Core Components](core-components.md) - Breakdown of core components

### Architecture Diagrams
- [Context Management Architecture](diagrams/context-management-architecture.md) - Context management design

### Use Cases
- [Leveraging MCP Server](use-cases/leveraging-mcp-server.md) - Example use cases and scenarios

## ✅ Best Practices

- Follow our [Best Practices](BEST_PRACTICES.md) for using the MCP Server effectively.
- For developers, ensure you adhere to the guidelines in the [Contributing Guide](../CONTRIBUTING.md).

## 📋 Version Information

This documentation covers DevOps MCP Server version 1.0.x. Make sure you're using the documentation that matches your installed version.

## 🔄 Staying Updated

The DevOps MCP Server is actively developed. Be sure to check the [CHANGELOG.md](../CHANGELOG.md) for updates and new features.

## 🤝 Getting Help

If you encounter issues not covered in the documentation:

1. Check the [Troubleshooting Guide](troubleshooting-guide.md)
2. Search existing [GitHub Issues](https://github.com/S-Corkum/mcp-server/issues)
3. Create a new issue if needed
