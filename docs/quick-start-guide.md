# MCP Server Quick Start Guide

This guide will help you get started with MCP Server quickly, using the mock server for local development.

## Prerequisites

- Docker and Docker Compose installed
- Git installed
- Basic familiarity with DevOps tools (GitHub, Harness, SonarQube, Artifactory, Xray)

## Step 1: Clone the Repository

```bash
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server
```

## Step 2: Start with Docker Compose

The quickest way to get started is using Docker Compose, which will set up all required components:

```bash
docker-compose up -d
```

This will start:
- MCP Server on port 8080
- Mock server on port 8081 (for simulating external services)
- PostgreSQL database
- Redis cache
- Prometheus for metrics
- Grafana for dashboards

**Note**: The default configuration uses container hostnames like "postgres" and "redis" for database and cache connections. If you plan to run the server locally outside of Docker, you'll need to update these in `configs/config.yaml` to use "localhost" instead.

## Step 3: Verify Installation

Check if all services are running:

```bash
docker-compose ps
```

Verify that the MCP Server is healthy:

```bash
curl http://localhost:8080/health
```

You should see a response indicating that all components are healthy.

## Step 4: Access the Dashboard

Open Grafana in your browser:

```
http://localhost:3000
```

Log in with the default credentials:
- Username: admin
- Password: admin

Navigate to the MCP Server dashboard to monitor the system.

## Step 5: Verify Webhook Endpoints

The MCP Server provides webhook endpoints for various services. Currently, webhook requests require proper signature validation. To view the available webhook endpoints:

```bash
# List the server's registered routes
docker-compose exec mcp-server wget -qO- http://localhost:8080/health
```

You should see that the following components are available:
- GitHub
- Harness
- SonarQube
- Artifactory
- Xray

To view the logs when webhooks are received:

```bash
docker-compose logs mcp-server
```

Note: When configuring actual webhooks from external services, make sure to use the correct webhook secrets as defined in your configuration.

## Step 6: Use the API

You can interact with the MCP Server API:

```bash
# Check the server health status
curl http://localhost:8080/health

# Create an MCP context (requires JWT authentication)
curl -X POST \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/mcp/context
```

Note: The API endpoints require proper authentication with JWT tokens. For testing purposes, you can use the health endpoint to verify the server is running properly.

## Next Steps

1. Configure real external services in `configs/config.yaml`
2. Set up webhook endpoints in your external services to point to your MCP Server
3. Develop custom integrations or workflows
4. Explore the [API Reference](api-reference.md) for all available endpoints
5. Read the [Configuration Guide](configuration-guide.md) for detailed configuration options

## Development Setup

If you want to develop with the MCP Server:

```bash
# Build the server and mock server
make build
make mockserver-build

# Set up PostgreSQL and Redis in Docker, then run both servers for local development
make local-dev-setup
```

Before running `local-dev-setup`, ensure that your `configs/config.yaml` is properly configured:

```yaml
# Database Configuration
database:
  host: "localhost"
  # ...
  dsn: "postgres://postgres:postgres@localhost:5432/mcp?sslmode=disable"

# Cache Configuration
cache:
  address: "localhost:6379"
  # ...

# GitHub Configuration (ensure mock mode is enabled for local development)
github:
  api_token: "${GITHUB_API_TOKEN}"
  webhook_secret: "${GITHUB_WEBHOOK_SECRET}"
  # ... other settings ...
  mock_responses: ${GITHUB_MOCK_RESPONSES:-false}
  mock_url: "${GITHUB_MOCK_URL:-http://mockserver:8081/mock-github}"
```

Make sure to set the appropriate environment variables or update the config file directly:

```bash
export GITHUB_MOCK_RESPONSES=true
export GITHUB_MOCK_URL=http://localhost:8081/mock-github
```

## Troubleshooting

If you encounter issues:

1. Check the logs: `docker-compose logs mcp-server`
2. Verify all services are running: `docker-compose ps`
3. Check the health endpoint: `curl http://localhost:8080/health`
4. Restart the services: `docker-compose restart`
5. Refer to the [Troubleshooting Guide](troubleshooting-guide.md) for more help