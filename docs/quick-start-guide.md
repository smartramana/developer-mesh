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

## Step 5: Test a Webhook

You can simulate a webhook event using curl:

```bash
# GitHub webhook example
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-Hub-Signature-256: sha256=mock-signature" \
  -d '{"repository":{"full_name":"test/repo"},"ref":"refs/heads/main"}' \
  http://localhost:8080/webhook/github
```

Check the MCP Server logs to see the event being processed:

```bash
docker-compose logs mcp-server
```

## Step 6: Use the API

You can interact with the MCP Server API:

```bash
# Get a list of GitHub repositories (using mock data)
curl -X GET \
  -H "Authorization: ApiKey mock-api-key" \
  http://localhost:8080/api/v1/github/repos
```

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

# Run both servers for local development
make local-dev
```

## Troubleshooting

If you encounter issues:

1. Check the logs: `docker-compose logs mcp-server`
2. Verify all services are running: `docker-compose ps`
3. Check the health endpoint: `curl http://localhost:8080/health`
4. Restart the services: `docker-compose restart`
5. Refer to the [Troubleshooting Guide](troubleshooting-guide.md) for more help