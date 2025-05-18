# Quick Start Guide

This guide will help you quickly get started with the DevOps MCP platform for local development and testing.

## Prerequisites

Before you begin, ensure you have the following installed on your system:

- [Docker](https://www.docker.com/get-started) and Docker Compose
- [Go](https://golang.org/doc/install) 1.19 or later
- [Git](https://git-scm.com/downloads)

## Clone the Repository

```bash
git clone https://github.com/S-Corkum/devops-mcp.git
cd devops-mcp
```

## Start the Development Environment

The easiest way to get started is using our `docker-compose.local.yml` file and the Makefile:

```bash
# Start all services using Docker Compose
make dev-setup

# Check the status of your services
make docker-compose-logs
```

This will start the following services:
- MCP Server (port 8080)
- REST API (port 8081)
- Worker service
- PostgreSQL with pgvector extension
- Redis
- LocalStack for SQS emulation

## Verify the Installation

Check if the services are running properly:

```bash
# Check the health endpoint
curl http://localhost:8080/health

# Check the REST API
curl http://localhost:8081/health
```

## Run Tests

You can run the test suite to make sure everything is working correctly:

```bash
# Run all tests
make test

# Run tests for a specific component
make test-rest-api
```

## Next Steps

Now that you have a running MCP development environment, you can:

- Explore the API Reference to understand available endpoints
- Set up your Development Environment for coding
- Learn about the System Architecture

## Stopping the Environment

When you're done, you can stop all services with:

```bash
make docker-compose-down
```
