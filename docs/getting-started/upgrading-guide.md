# Upgrading DevOps MCP Server

This guide provides instructions for upgrading the DevOps MCP Server to newer versions. It covers the general upgrade process as well as version-specific considerations.

## Table of Contents

- [General Upgrade Process](#general-upgrade-process)
  - [Docker Compose Deployment](#docker-compose-deployment)
  - [Kubernetes Deployment](#kubernetes-deployment)
  - [Standalone Deployment](#standalone-deployment)
- [Database Migrations](#database-migrations)
- [Configuration Changes](#configuration-changes)
- [Version-Specific Instructions](#version-specific-instructions)
  - [Upgrading to 1.1.x](#upgrading-to-11x)
  - [Upgrading to 1.0.x](#upgrading-to-10x)
- [Rollback Procedures](#rollback-procedures)
- [Troubleshooting](#troubleshooting)

## General Upgrade Process

Before upgrading, always:

1. Read the release notes for the new version
2. Back up your database
3. Back up your configuration files
4. Test the upgrade in a non-production environment first

### Docker Compose Deployment

If you're using Docker Compose:

1. Pull the latest repository changes:
   ```bash
   git pull origin main
   ```

2. Update the Docker images:
   ```bash
   docker-compose pull
   ```

3. Restart the services:
   ```bash
   docker-compose down
   docker-compose up -d
   ```

4. Verify the upgrade:
   ```bash
   curl http://localhost:8080/health
   ```

### Kubernetes Deployment

If you're using Kubernetes:

1. Update your Kubernetes manifests:
   ```bash
   kubectl apply -f kubernetes/
   ```

2. Verify the deployment:
   ```bash
   kubectl get pods -n mcp-server
   kubectl get services -n mcp-server
   ```

3. Check the logs for any errors:
   ```bash
   kubectl logs -n mcp-server deployment/mcp-server
   ```

### Standalone Deployment

If you're running a standalone deployment:

1. Stop the MCP Server:
   ```bash
   systemctl stop mcp-server
   # or
   supervisorctl stop mcp-server
   ```

2. Download the new binary:
   ```bash
   # From binary releases
   curl -L -o mcp-server https://github.com/S-Corkum/mcp-server/releases/download/vX.Y.Z/mcp-server-linux-amd64
   chmod +x mcp-server
   
   # Or build from source
   git pull origin main
   go build -o mcp-server ./cmd/server
   ```

3. Update configuration files if needed
4. Start the service:
   ```bash
   systemctl start mcp-server
   # or
   supervisorctl start mcp-server
   ```

5. Verify the upgrade:
   ```bash
   curl http://your-server/health
   ```

## Database Migrations

DevOps MCP Server uses migration scripts to handle database schema changes:

1. Migrations run automatically on server startup by default.
2. For manual migrations, you can use the migration tool:
   ```bash
   ./mcp-server migrate
   # or
   go run cmd/migrate/main.go
   ```

3. To see the current migration status:
   ```bash
   ./mcp-server migrate status
   # or
   go run cmd/migrate/main.go status
   ```

See [Database Migrations](database-migrations.md) for more information.

## Configuration Changes

When upgrading:

1. Compare your current configuration file with the new template:
   ```bash
   diff configs/config.yaml configs/config.yaml.template
   ```

2. Add any new configuration options to your file
3. Update any changed configuration options
4. Remove any deprecated options

## Version-Specific Instructions

### Upgrading to 1.1.x

When upgrading from 1.0.x to 1.1.x:

1. New configuration options:
   - `api.rate_limit` - Added support for API rate limiting
   - `database.vector.max_dimensions` - Added support for configuring vector dimensions

2. Database changes:
   - Added support for multiple embedding models (migration script included)

3. API changes:
   - New endpoints for vector operations with multiple models
   - Backward compatibility maintained for existing endpoints

### Upgrading to 1.0.x

When upgrading from 0.9.x to 1.0.x:

1. Configuration changes:
   - Renamed `github` section to `adapters.github`
   - Added new AWS IAM authentication options

2. Database changes:
   - Added vector search support using pgvector (migration script included)
   - Added context reference tables

3. API changes:
   - Updated API paths from `/api/github/...` to `/api/v1/tools/github/...`
   - Added new context management endpoints

## Rollback Procedures

If you encounter issues after upgrading:

### Docker Compose Rollback

1. Stop the services:
   ```bash
   docker-compose down
   ```

2. Switch to the previous version tag:
   ```bash
   git checkout vX.Y.Z
   ```

3. Start the services:
   ```bash
   docker-compose up -d
   ```

### Kubernetes Rollback

1. Roll back to the previous deployment:
   ```bash
   kubectl rollout undo deployment/mcp-server -n mcp-server
   ```

2. Or apply the previous manifests:
   ```bash
   kubectl apply -f kubernetes/previous-version/
   ```

### Database Rollback

If database migrations need to be rolled back:

1. Use the migration tool with the `down` command:
   ```bash
   ./mcp-server migrate down
   # or
   go run cmd/migrate/main.go down
   ```

2. Or restore from your database backup

## Troubleshooting

If you encounter issues during the upgrade:

1. Check the server logs:
   ```bash
   docker-compose logs mcp-server
   # or
   kubectl logs -n mcp-server deployment/mcp-server
   # or
   journalctl -u mcp-server
   ```

2. Verify the health endpoint:
   ```bash
   curl http://your-server/health
   ```

3. Check the database connection:
   ```bash
   psql -h your-db-host -U your-db-user -d mcp -c "SELECT 1"
   ```

4. Verify Redis connectivity:
   ```bash
   redis-cli -h your-redis-host ping
   ```

For more troubleshooting information, see the [Troubleshooting Guide](troubleshooting-guide.md).
