#!/bin/bash
# Checks and optionally sets required environment variables for functional tests

REQUIRED_VARS=(
  MCP_SERVER_URL
  MCP_API_KEY
  MOCKSERVER_URL
  GITHUB_TOKEN
  GITHUB_REPO
  GITHUB_OWNER
  ELASTICACHE_ENDPOINT
  ELASTICACHE_PORT
  MCP_GITHUB_WEBHOOK_SECRET
  REDIS_ADDR
)

MISSING=()

for VAR in "${REQUIRED_VARS[@]}"; do
  if [ -z "${!VAR}" ]; then
    MISSING+=("$VAR")
  fi
done

if [ ${#MISSING[@]} -eq 0 ]; then
  echo "All required environment variables are set."
  exit 0
else
  echo "Missing environment variables: ${MISSING[*]}"
  echo "You can set them in your shell, .env file, or directly in this script."
  exit 1
fi
