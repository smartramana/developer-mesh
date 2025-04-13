# MCP Server Documentation

## Overview

The MCP (Multi-Cloud Platform) Server is a centralized platform for integrating, monitoring, and managing DevOps tools across your organization. It provides a unified API and event system for various development tools and platforms, enabling you to connect different services and respond to events in real-time.

## Documentation Index

### Getting Started
- [Installation Guide](installation-guide.md) - How to install and set up the MCP Server
- [Quick Start Guide](quick-start-guide.md) - Getting up and running quickly
- [Configuration Guide](configuration-guide.md) - Detailed configuration options

### Architecture
- [System Architecture](system-architecture.md) - Overall system design and components
- [Core Components](core-components.md) - Detailed description of the core components
- [Event System](event-system.md) - How the event system works
- [API Reference](api-reference.md) - API endpoints and usage

### Integrations
- [Integration Overview](integration-overview.md) - Summary of all supported integrations
- [GitHub Integration](github-integration.md) - Connecting to GitHub
- [Harness Integration](harness-integration.md) - Connecting to Harness
- [SonarQube Integration](sonarqube-integration.md) - Connecting to SonarQube
- [JFrog Artifactory Integration](artifactory-integration.md) - Connecting to JFrog Artifactory
- [JFrog Xray Integration](xray-integration.md) - Connecting to JFrog Xray

### Development
- [Development Guide](development-guide.md) - Guide for developers working on the MCP Server
- [Adding New Integrations](adding-new-integrations.md) - How to add support for new tools
- [Performance Optimizations](performance-optimizations.md) - Performance tuning strategies

### Operations
- [Deployment Guide](deployment-guide.md) - Production deployment recommendations
- [Monitoring Guide](monitoring-guide.md) - How to monitor the MCP Server
- [Troubleshooting Guide](troubleshooting-guide.md) - Common issues and solutions

## Project Status

MCP Server is currently in active development. The core engine, API server, and adapter system are operational, with initial integrations for:

1. GitHub
2. Harness
3. SonarQube
4. JFrog Artifactory
5. JFrog Xray

Additional integrations and features are planned for future releases.