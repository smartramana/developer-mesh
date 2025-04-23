# DevOps MCP Server Documentation

## Overview

DevOps MCP (Model Context Protocol) Server serves as a bridge between AI agents and DevOps tools, providing:

1. **Unified API for DevOps Tools**: A standardized protocol for AI agents to interact with GitHub and other DevOps platforms
2. **Context Management**: Sophisticated conversation context handling with token management and vector search
3. **Vector Embedding Storage**: Semantic search capabilities using pgvector for enhanced AI memory

## Getting Started

* [Quick Start Guide](quick-start-guide.md) - Get up and running in minutes
* [Installation Guide](installation-guide.md) - Detailed installation instructions
* [Configuration Guide](configuration-guide.md) - Configuration options and examples

## User Guides

* [AI Agent Integration Guide](guides/ai-agent-integration-guide.md) - Complete guide to integrating AI agents
* [GitHub Integration Guide](github-integration-guide.md) - Working with the GitHub adapter
* [Context Management Guide](context-management-guide.md) - Managing conversation contexts effectively
* [Vector Search Guide](guides/vector-search-guide.md) - Using vector embeddings for semantic search

## API Reference

* [API Overview](api-reference.md) - Complete API documentation
* [OpenAPI Specification](swagger/context_api.yaml) - OpenAPI/Swagger specification

## Deployment & Operations

* [Deployment Guide](deployment-guide.md) - Production deployment recommendations
* [AWS Integration](aws/aws-irsa-setup.md) - Working with AWS services
* [Kubernetes Deployment](kubernetes-deployment.md) - Deploying on Kubernetes
* [Monitoring Guide](monitoring-guide.md) - Monitoring and alerting
* [Upgrading Guide](upgrading-guide.md) - How to safely upgrade

## Maintenance & Troubleshooting

* [Database Migrations](database-migrations.md) - Managing database schema changes
* [Troubleshooting Guide](troubleshooting-guide.md) - Common issues and solutions

## Developer Documentation

* [System Architecture](system-architecture.md) - System design and architecture
* [Development Guide](development-guide.md) - Getting started with development
* [Adding New Integrations](adding-new-integrations.md) - How to add new tool adapters
* [Contributing Guide](contributing-guide.md) - How to contribute to the project

## Examples

* [Complete AI Agent Example](examples/complete-ai-agent-example.md) - End-to-end integration example
* [Multi-Model Vector Support](features/multi-model-vector-support.md) - Working with different embedding models
* [S3 Storage Integration](features/s3-storage.md) - Using S3 for context storage

## Support & Community

If you encounter issues not covered in the documentation:

1. Check the [Troubleshooting Guide](troubleshooting-guide.md)
2. Search existing [GitHub Issues](https://github.com/S-Corkum/mcp-server/issues)
3. Create a new issue if needed
