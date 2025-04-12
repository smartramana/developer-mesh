# MCP Server

MCP (Multi-Cloud Platform) Server is a centralized platform for integrating, monitoring, and managing DevOps tools across your organization. It provides a unified API and event system for various development tools and platforms.

## Features

- **Centralized Integration**: Connect to multiple DevOps tools through a single platform
- **Event-Driven Architecture**: React to events from various systems in real-time
- **Webhook Support**: Receive and process webhooks from multiple providers
- **Extensible Design**: Easily add new tool integrations through adapters
- **Resilient Processing**: Built-in retry mechanisms, circuit breakers, and error handling
- **Performance Optimized**: Connection pooling, caching, and concurrency management

## Supported Integrations

The MCP Server currently supports the following integrations:

1. **GitHub**: Repository events, pull requests, and commits
2. **Harness**: CI builds, CD deployments, STO experiments, and feature flags
3. **SonarQube**: Quality gates, code analysis, and security scanning
4. **JFrog Artifactory**: Artifact management and repository events
5. **JFrog Xray**: Security vulnerability scanning and license compliance

## Getting Started

### Prerequisites

- Go 1.20 or higher
- Docker and Docker Compose (for local development)
- Access to the integrated systems (GitHub, Harness, SonarQube, Artifactory, Xray)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server
```

2. Copy the configuration template:

```bash
cp configs/config.yaml.template configs/config.yaml
```

3. Edit the configuration file with your credentials and settings.

4. Create an `.env` file with your environment variables:

```bash
# GitHub configuration
GITHUB_API_TOKEN=your_github_token
GITHUB_WEBHOOK_SECRET=your_github_webhook_secret

# Harness configuration
HARNESS_API_TOKEN=your_harness_token
HARNESS_ACCOUNT_ID=your_harness_account
HARNESS_WEBHOOK_SECRET=your_harness_webhook_secret

# SonarQube configuration
SONARQUBE_URL=https://your-sonarqube-instance
SONARQUBE_TOKEN=your_sonarqube_token
SONARQUBE_WEBHOOK_SECRET=your_sonarqube_webhook_secret

# Artifactory configuration
ARTIFACTORY_URL=https://your-artifactory-instance
ARTIFACTORY_USERNAME=your_artifactory_username
ARTIFACTORY_PASSWORD=your_artifactory_password
ARTIFACTORY_API_KEY=your_artifactory_api_key
ARTIFACTORY_WEBHOOK_SECRET=your_artifactory_webhook_secret

# Xray configuration
XRAY_URL=https://your-xray-instance
XRAY_USERNAME=your_xray_username
XRAY_PASSWORD=your_xray_password
XRAY_API_KEY=your_xray_api_key
XRAY_WEBHOOK_SECRET=your_xray_webhook_secret
```

### Running with Docker Compose

The easiest way to run the MCP Server is using Docker Compose:

```bash
docker-compose up -d
```

This will start the MCP Server along with its dependencies (PostgreSQL, Redis, Prometheus, and Grafana).

### Building and Running Locally

1. Install Go dependencies:

```bash
go mod download
```

2. Build the server:

```bash
go build -o mcp-server ./cmd/server
```

3. Run the server:

```bash
./mcp-server
```

### Running Tests

```bash
go test ./...
```

## Configuration

The MCP Server can be configured using a YAML configuration file and/or environment variables. See the `configs/config.yaml.template` file for all available options.

### Environment Variables

All configuration options can be set using environment variables with the `MCP_` prefix. For example:

- `MCP_API_LISTEN_ADDRESS=:8080`
- `MCP_DATABASE_DSN=postgres://user:password@localhost:5432/mcp`
- `MCP_ENGINE_GITHUB_API_TOKEN=your_token`

## API Documentation

### Webhook Endpoints

- GitHub: `POST /api/v1/webhook/github`
- Harness: `POST /api/v1/webhook/harness`
- SonarQube: `POST /api/v1/webhook/sonarqube`
- Artifactory: `POST /api/v1/webhook/artifactory`
- Xray: `POST /api/v1/webhook/xray`

### Health and Metrics

- Health Check: `GET /health`
- Metrics: `GET /metrics`

## Monitoring

The MCP Server integrates with Prometheus and Grafana for monitoring and observability. The Docker Compose setup includes both services.

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

## Architecture

The MCP Server is built with a modular architecture:

- **Adapters**: Interface with external systems (GitHub, Harness, etc.)
- **Core Engine**: Manages events and orchestrates workflows
- **API Server**: Provides REST API and webhook endpoints
- **Database**: Persists configuration and state
- **Cache**: Improves performance for frequently accessed data

### Performance Optimizations

- **Concurrency Management**: Worker pools with configurable limits
- **Caching Strategy**: Multi-level caching with intelligent invalidation
- **Database Optimizations**: Connection pooling and prepared statements
- **Resilience Patterns**: Circuit breakers and retry mechanisms

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.