# Contributing to DevOps MCP Server

Thank you for your interest in contributing to the DevOps MCP Server! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Release Process](#release-process)
- [Communication](#communication)

## Code of Conduct

We expect all contributors to adhere to our Code of Conduct. Please be respectful and considerate of other contributors. Harassment or offensive behavior of any kind will not be tolerated.

## Getting Started

### Prerequisites

- Go 1.20 or higher
- Docker and Docker Compose for local testing
- Git and Make
- An IDE with Go support (VSCode, GoLand, etc.)

### Setup Development Environment

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR-USERNAME/mcp-server.git
   cd mcp-server
   ```

3. Add the upstream repository as a remote:
   ```bash
   git remote add upstream https://github.com/S-Corkum/mcp-server.git
   ```

4. Create a branch for your work:
   ```bash
   git checkout -b feature/your-feature-name
   ```

5. Set up local dependencies:
   ```bash
   # Install dependencies
   go mod download
   
   # Start required services (PostgreSQL, Redis) with Docker Compose
   docker-compose up -d postgres redis
   
   # Copy and edit configuration
   cp configs/config.yaml.template configs/config.yaml
   # Edit config.yaml for your environment
   ```

## Development Workflow

1. Make your changes, following our coding standards
2. Add tests for new functionality
3. Run tests locally to ensure they pass
4. Update documentation for any user-facing changes
5. Commit your changes with a clear message
6. Push to your fork and create a pull request

### Running the Server Locally

```bash
# Build and run the server
make build
./mcp-server

# Or use the convenience command for local development
make local-dev
```

### Using Mock Mode for Development

For development without requiring actual GitHub integration:

```bash
# Set GITHUB_MOCK_RESPONSES=true in your .env file
# Or use the mock server
make mockserver-build
make local-dev
```

## Pull Request Process

1. Ensure your code meets our coding standards and passes all tests
2. Update documentation if necessary
3. Include a descriptive PR title and detailed description
4. Reference any related issues using GitHub's keywords (Fixes #123, Closes #456)
5. Wait for CI checks to pass
6. Address review feedback promptly
7. Once approved, your PR will be merged by a maintainer

## Coding Standards

We follow standard Go coding conventions:

- Use `gofmt` or `goimports` to format your code
- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Add comments for exported functions, types, and packages
- Write meaningful variable and function names
- Keep functions focused and reasonably sized
- Handle errors appropriately

### Project Structure

```
mcp-server/
├── cmd/                    # Command-line applications
├── configs/                # Configuration files
├── docs/                   # Documentation
├── internal/               # Internal packages (not importable)
│   ├── adapters/           # External system adapters
│   ├── api/                # API server
│   ├── core/               # Core engine
│   └── ...                 # Other internal packages
├── pkg/                    # Public packages (importable)
├── scripts/                # Build and deployment scripts
└── test/                   # Test files and fixtures
```

## Testing Guidelines

- Write unit tests for all new functionality
- Write integration tests for API endpoints and adapter functionality
- Run tests before submitting a PR:
  ```bash
  # Run unit tests
  go test ./...
  
  # Run integration tests
  go test -tags=integration ./...
  
  # Run tests with coverage
  go test -cover ./...
  ```

## Documentation

Good documentation is crucial for the MCP Server. When making changes:

1. Update any affected documentation in the `/docs` directory
2. Add godoc-style comments to exported types and functions
3. Include examples for non-trivial functionality
4. Update the API reference if you modify API endpoints
5. Add new diagrams or update existing ones if needed

## Release Process

Our release process follows these steps:

1. Create a release branch: `release/vX.Y.Z`
2. Update version information
3. Prepare release notes
4. Create and push a tag: `git tag vX.Y.Z`
5. Create a GitHub release with release notes

## Communication

- GitHub Issues: For bug reports, feature requests, and discussions
- Pull Requests: For code reviews and implementation discussions

## Adding New Integrations

If you want to add a new tool integration, see our [Adding New Integrations](docs/adding-new-integrations.md) guide for detailed instructions.

## Thank You!

Thank you for contributing to the DevOps MCP Server! Your efforts help improve the project for everyone.
