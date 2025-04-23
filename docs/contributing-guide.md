# Contributing to DevOps MCP Server

Thank you for your interest in contributing to the DevOps MCP Server! This guide will help you get started with contributions to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Environment](#development-environment)
- [Contribution Workflow](#contribution-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation Guidelines](#documentation-guidelines)
- [Issue Reporting](#issue-reporting)
- [Pull Request Process](#pull-request-process)
- [Community](#community)

## Code of Conduct

This project adheres to a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

## Getting Started

### Prerequisites

- [Go](https://golang.org/dl/) 1.20 or higher
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [Git](https://git-scm.com/downloads)
- A code editor such as [VS Code](https://code.visualstudio.com/) or [GoLand](https://www.jetbrains.com/go/)

### Understanding the Project

Before making contributions, please take some time to understand the project:

1. Read the [README.md](../README.md) for an overview
2. Review the [System Architecture](system-architecture.md) documentation
3. Explore the [API Reference](api-reference.md)
4. Run the [Quick Start Guide](quick-start-guide.md) to get familiar with the system

## Development Environment

### Setting Up Your Local Development Environment

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR-USERNAME/mcp-server.git
   cd mcp-server
   ```

3. Add the original repository as an upstream remote:
   ```bash
   git remote add upstream https://github.com/S-Corkum/mcp-server.git
   ```

4. Install dependencies:
   ```bash
   go mod download
   go mod tidy
   ```

5. Set up your configuration:
   ```bash
   cp configs/config.yaml.template configs/config.yaml
   # Edit config.yaml with your local settings
   ```

6. Start the dependencies with Docker Compose:
   ```bash
   docker-compose up -d postgres redis
   ```

7. Build and run the server:
   ```bash
   go build -o mcp-server ./cmd/server
   ./mcp-server
   ```

### Project Structure

The MCP Server follows a standard Go project layout:

```
mcp-server/
├── cmd/                    # Command-line applications
│   ├── mockserver/        # Mock server entry point
│   ├── migrate/           # Database migration tool
│   └── server/            # MCP Server entry point
├── configs/                # Configuration files
├── docs/                   # Documentation
├── internal/               # Internal packages (not importable)
│   ├── adapters/          # External system adapters
│   ├── api/               # API server
│   ├── cache/             # Cache implementation
│   ├── config/            # Configuration handling
│   ├── core/              # Core engine
│   ├── database/          # Database implementation
│   └── repository/        # Data repositories
├── pkg/                    # Public packages (importable)
│   ├── client/            # Client library
│   ├── models/            # Data models
│   └── mcp/               # MCP protocol definition
├── migrations/            # Database migration files
├── scripts/               # Build and deployment scripts
├── test/                  # Test files and fixtures
└── kubernetes/            # Kubernetes deployment files
```

## Contribution Workflow

### Finding Issues to Work On

1. Check the [GitHub Issues](https://github.com/S-Corkum/mcp-server/issues) for open tasks
2. Look for issues labeled `good first issue` if you're new to the project
3. Comment on an issue to express your interest before starting work

### Working on Issues

1. Create a new branch for your work:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

2. Make your changes, following the [coding standards](#coding-standards)
3. Write or update tests to cover your changes
4. Ensure all tests pass with `go test ./...`
5. Commit your changes with descriptive commit messages
6. Push your branch to your fork
7. Submit a pull request to the main repository

## Coding Standards

### Go Code Guidelines

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use [`gofmt`](https://golang.org/cmd/gofmt/) or [`goimports`](https://godoc.org/golang.org/x/tools/cmd/goimports) to format your code
- Provide meaningful variable and function names
- Write [godoc-style comments](https://blog.golang.org/godoc-documenting-go-code) for all exported functions, types, and variables
- Avoid unnecessary dependencies
- Handle errors explicitly

### Code Organization

- Keep functions focused and reasonably sized
- Group related functionality in the same package
- Follow the [single responsibility principle](https://en.wikipedia.org/wiki/Single-responsibility_principle)
- Use interfaces to abstract dependencies
- Prefer composition over inheritance

## Testing Guidelines

### Writing Tests

- Write unit tests for all new code
- Aim for high test coverage, especially in critical paths
- Use table-driven tests for multiple test cases
- Use mocks for external dependencies
- Name test functions clearly: `TestFunctionName_TestCase`

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Integration Tests

Integration tests require the full environment:

```bash
# Start dependencies
docker-compose up -d

# Run integration tests
go test -tags=integration ./...
```

## Documentation Guidelines

### Code Documentation

- Add godoc comments for all exported functions, types, and constants
- Use inline comments to explain complex logic
- Update existing documentation when changing functionality

### Project Documentation

When updating the documentation:

- Use clear, concise language
- Provide examples where possible
- Use proper Markdown formatting
- Organize content logically
- Keep the documentation up-to-date with code changes

## Issue Reporting

### Reporting Bugs

When reporting bugs, please include:

1. A clear, descriptive title
2. Steps to reproduce the issue
3. Expected behavior
4. Actual behavior
5. Environment details (OS, Go version, etc.)
6. Logs or error messages
7. Screenshots if applicable

### Suggesting Enhancements

When suggesting enhancements:

1. Describe the current behavior
2. Explain the desired behavior
3. Provide a use case explaining why this enhancement would be useful
4. If possible, outline how this could be implemented

## Pull Request Process

1. Ensure your code follows the [coding standards](#coding-standards)
2. Update the documentation with details of changes
3. Add or update tests to cover your changes
4. Ensure all tests pass
5. Update the README.md if necessary
6. Submit your pull request with a clear title and description
7. Reference any related issues with "Fixes #123" or "Relates to #123"

### PR Review Process

1. Maintainers will review your PR
2. Address any requested changes
3. Once approved, a maintainer will merge your PR

## Community

- Join discussions in GitHub Issues
- Follow the project on GitHub
- Respect all community members

Thank you for contributing to the DevOps MCP Server project!
