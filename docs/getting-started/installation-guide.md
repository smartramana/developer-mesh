# MCP Server Installation Guide

This guide provides detailed instructions for installing and setting up the MCP Server in various environments.

## Prerequisites

Before installing the MCP Server, ensure you have the following prerequisites:

- Go 1.20 or higher (for building from source)
- Docker and Docker Compose (for containerized deployment)
- Access to the integrated systems (GitHub, Harness, SonarQube, Artifactory, Xray)
- PostgreSQL 12+ database (can be run via Docker)
- Redis 6+ (can be run via Docker)

## Installation Methods

There are several ways to install and run the MCP Server:

1. [Docker Compose Deployment](#docker-compose-deployment) (Recommended for local development)
2. [Build and Run Locally](#build-and-run-locally)
3. [Kubernetes Deployment](#kubernetes-deployment)

## Docker Compose Deployment

The easiest way to run the MCP Server is using Docker Compose, which will set up the MCP Server and all required dependencies.

### Step 1: Clone the Repository

```bash
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server
```

### Step 2: Configure the Environment

Create a `.env` file with your environment variables:

```bash
# Copy example environment file
cp .env.example .env

# Edit the .env file with your credentials
nano .env
```

### Step 3: Start with Docker Compose

```bash
docker-compose up -d
```

This command will start:
- MCP Server on port 8080
- Mock server for simulating external services on port 8081
- PostgreSQL database on port 5432
- Redis cache on port 6379
- Prometheus for metrics on port 9090
- Grafana for dashboards on port 3000

### Step 4: Verify Installation

Check if all services are running:

```bash
docker-compose ps
```

Access the MCP Server health endpoint:

```bash
curl http://localhost:8080/health
```

## Build and Run Locally

You can build and run the MCP Server locally for development.

### Step 1: Clone the Repository

```bash
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server
```

### Step 2: Install Dependencies

```bash
make deps
```

### Step 3: Build the Server

```bash
make build
```

### Step 4: Configure the Server

```bash
# Copy the configuration template
cp configs/config.yaml.template configs/config.yaml

# Edit the configuration
nano configs/config.yaml
```

### Step 5: Run Local Dependencies

You can use Docker Compose to run only the required dependencies:

```bash
docker-compose up -d postgres redis
```

### Step 6: Run the Server

#### Option 1: With Mock Services (Recommended for Development)

```bash
# Build the mock server
make mockserver-build

# Run both servers
make local-dev
```

#### Option 2: With Real Services

```bash
./mcp-server
```

## Kubernetes Deployment

For production environments, you might want to deploy MCP Server on Kubernetes.

### Step 1: Prepare Kubernetes Configuration

Create Kubernetes deployment and service files (see examples in the `kubernetes` directory).

### Step 2: Create ConfigMap and Secrets

```bash
# Create ConfigMap for configuration
kubectl create configmap mcp-config --from-file=configs/config.yaml

# Create Secret for sensitive information
kubectl create secret generic mcp-secrets \
  --from-literal=GITHUB_API_TOKEN='your_github_token' \
  --from-literal=GITHUB_WEBHOOK_SECRET='your_github_webhook_secret' \
  # Add other secrets here
```

### Step 3: Deploy to Kubernetes

```bash
kubectl apply -f kubernetes/
```

### Step 4: Verify the Deployment

```bash
kubectl get pods
kubectl get services
```

## Configuration

After installation, you'll need to configure the MCP Server to connect to your services. See the [Configuration Guide](configuration-guide.md) for detailed information.

## Next Steps

1. See the [Quick Start Guide](quick-start-guide.md) for basic usage instructions
2. Configure the server using the [Configuration Guide](configuration-guide.md)
3. Set up integrations with your DevOps tools (see the Integration guides)

## Troubleshooting

If you encounter issues during installation, check the [Troubleshooting Guide](troubleshooting-guide.md) or refer to the following common solutions:

### Common Issues

1. **Database Connection Issues**

   Check if PostgreSQL is running and accessible:
   
   ```bash
   psql -h localhost -U postgres -d mcp
   ```

2. **Redis Connection Issues**

   Check if Redis is running:
   
   ```bash
   redis-cli ping
   ```

3. **Docker Compose Issues**

   Ensure Docker and Docker Compose are properly installed:
   
   ```bash
   docker --version
   docker-compose --version
   ```

4. **Port Conflicts**

   Ensure the required ports (8080, 8081, 5432, 6379, 9090, 3000) are not in use by other applications.