#!/bin/bash

# Create directory structure for MCP Server
mkdir -p \
  cmd/server \
  configs \
  internal/adapters/artifactory \
  internal/adapters/github \
  internal/adapters/harness \
  internal/adapters/sonarqube \
  internal/adapters/xray \
  internal/adapters/factory \
  internal/api \
  internal/cache \
  internal/config \
  internal/core \
  internal/database \
  internal/metrics \
  internal/storage \
  pkg/mcp \
  pkg/models \
  scripts/db

echo "Directory structure created successfully."

# Setup basic Go module
if [ ! -f go.mod ]; then
  go mod init github.com/developer-mesh/mcp-server
  echo "Go module initialized."
fi

# Fetch dependencies
go mod tidy
echo "Dependencies updated."

# Create basic configs directory
if [ ! -f configs/config.yaml ]; then
  cp configs/config.yaml.template configs/config.yaml
  echo "Config file copied from template."
fi

# Make script executable
chmod +x scripts/db/init.sql
echo "Setup completed."