# MCP Server Quick Start Guide

This guide will help you get the MCP Server up and running quickly with Docker Compose for local development or testing.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) installed
- [Git](https://git-scm.com/downloads) installed
- GitHub account and personal access token (for GitHub integration)

## Step 1: Clone the Repository

```bash
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server
```

## Step 2: Configure the Environment

Create a `.env` file with your credentials:

```bash
# Copy example environment file
cp .env.example .env

# Edit the .env file with your credentials
nano .env
```

At minimum, set these variables in your `.env` file:

```
# GitHub Configuration
GITHUB_API_TOKEN=your_github_personal_access_token
GITHUB_WEBHOOK_SECRET=your_webhook_secret

# JWT Secret for API Authentication
MCP_AUTH_JWT_SECRET=your_jwt_secret

# API Keys (comma-separated list)
MCP_AUTH_API_KEYS=key1,key2,key3
```

## Step 3: Start the Server

Start all services using Docker Compose:

```bash
docker-compose up -d
```

This command will start:
- MCP Server on port 8080
- PostgreSQL database
- Redis cache
- Prometheus (metrics)
- Grafana (dashboards)

To check if all services are running:

```bash
docker-compose ps
```

You should see all services with the "Up" status.

## Step 4: Verify Installation

Check if the MCP Server is running properly:

```bash
curl http://localhost:8080/health
```

You should receive a JSON response with the health status of all components:

```json
{
  "status": "healthy",
  "components": {
    "engine": "healthy",
    "database": "healthy",
    "cache": "healthy",
    "github": "healthy"
  }
}
```

## Step 5: Access the API Documentation

MCP Server includes Swagger UI for API documentation and testing. Open your browser and navigate to:

```
http://localhost:8080/swagger/index.html
```

You can explore the available API endpoints and test them directly from the UI.

## Step 6: Create Your First Context

Let's create a simple conversation context using the API:

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer key1" \
  -d '{
    "agent_id": "test-agent",
    "model_id": "gpt-4",
    "max_tokens": 4000,
    "content": [
      {
        "role": "system",
        "content": "You are a DevOps assistant.",
        "tokens": 6
      }
    ]
  }' \
  http://localhost:8080/api/v1/contexts
```

If successful, you'll receive a response with the created context:

```json
{
  "id": "ctx_123456",
  "message": "context created"
}
```

Note the context ID (e.g., `ctx_123456`) for use in subsequent API calls.

## Step 7: Execute a GitHub Action

Now, let's execute a GitHub action using the MCP Server:

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer key1" \
  -d '{
    "owner": "your-username",
    "repo": "your-repo",
    "title": "Test Issue",
    "body": "This is a test issue created via MCP Server"
  }' \
  "http://localhost:8080/api/v1/tools/github/actions/create_issue?context_id=ctx_123456"
```

Replace `ctx_123456` with your actual context ID, and `your-username` and `your-repo` with your GitHub username and repository name.

If successful, you'll receive a response with the result of the action.

## Next Steps

Congratulations! You've set up and tested the MCP Server. Here's what to explore next:

### Integration with AI Agents

For integrating with AI agents, check out:
- [AI Agent Integration Guide](guides/ai-agent-integration-guide.md)
- [Complete AI Agent Example](examples/complete-ai-agent-example.md)

### Production Setup

For production deployment, see:
- [Deployment Guide](deployment-guide.md)
- [AWS Integration](aws/aws-irsa-setup.md)
- [Kubernetes Deployment](kubernetes-deployment.md)

### Development and Contribution

To contribute to the project:
- [Development Guide](development-guide.md)
- [System Architecture](system-architecture.md)
- [Contributing Guide](contributing-guide.md)

## Troubleshooting

If you encounter any issues:

1. **Check logs**:
   ```bash
   docker-compose logs mcp-server
   ```

2. **Verify services are running**:
   ```bash
   docker-compose ps
   ```

3. **Restart services**:
   ```bash
   docker-compose restart
   ```

4. **Clean up and restart**:
   ```bash
   docker-compose down
   docker-compose up -d
   ```

5. **Reference the comprehensive [Troubleshooting Guide](troubleshooting-guide.md)**

## Development Mode with Mock Responses

For development without real GitHub integration:

1. Edit your `.env` file to enable mock mode:
   ```
   GITHUB_MOCK_RESPONSES=true
   GITHUB_MOCK_URL=http://mockserver:8081/mock-github
   ```

2. Start both the MCP Server and the mock server:
   ```bash
   docker-compose up -d
   ```

This setup allows you to test the API without making real GitHub API calls.
