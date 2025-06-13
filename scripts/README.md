# Scripts Directory

This directory contains utility scripts, automation tools, and test helpers for the DevOps MCP project.

## Directory Structure

- **aws/** - AWS service management scripts (ElastiCache tunnel, S3 IP updates)
- **db/** - Database initialization and migration scripts
- **local/** - Local development helper scripts
- **localstack/** - LocalStack initialization scripts

## Environment Management Scripts

- `start-functional-test-env.sh` - Start all services for functional testing with AWS
- `stop-functional-test-env.sh` - Stop functional test services
- `setup.sh` - Initial development environment setup

## Test Scripts

- `test-*.sh` - Various test scripts for different components
- `websocket-load-test.sh` - WebSocket performance testing
- `test-harness-webhook.sh` - Webhook testing harness

## Verification Scripts

- `verify_github_integration.sh` - Basic GitHub integration verification
- `verify_github_integration_complete.sh` - Comprehensive GitHub integration tests
- `validate-*.sh` - Various validation scripts for endpoints, response times, etc.

## Utility Scripts

- `restart_servers.sh` - Restart MCP server components
- `health-check.sh` - Check health of all services
- `pull-images.sh` - Pull required Docker images

## Usage

Most scripts can be run directly from the command line:

```bash
# To verify GitHub integration
./verify_github_integration.sh

# To restart the servers
./restart_servers.sh
```

If a script requires specific arguments or setup, check the script's header comments for details.
