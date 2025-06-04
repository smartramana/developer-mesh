#!/bin/bash
# Checks and optionally sets required environment variables for functional tests

# Define default values
declare -A DEFAULTS=(
  ["MCP_SERVER_URL"]="http://localhost:8080"
  ["MCP_API_KEY"]="dev-admin-key-1234567890"
  ["MOCKSERVER_URL"]="http://localhost:8082"
  ["GITHUB_TOKEN"]="test-github-token"
  ["GITHUB_REPO"]="test-repo"
  ["GITHUB_OWNER"]="test-org"
  ["ELASTICACHE_ENDPOINT"]="localhost"
  ["ELASTICACHE_PORT"]="6379"
  ["MCP_GITHUB_WEBHOOK_SECRET"]="docker-github-webhook-secret"
  ["REDIS_ADDR"]="localhost:6379"
)

# Load .env file if it exists
if [ -f "$(dirname "$0")/.env" ]; then
  export $(cat "$(dirname "$0")/.env" | grep -v '^#' | xargs)
fi

# Check and set variables
MISSING=()
for VAR in "${!DEFAULTS[@]}"; do
  if [ -z "${!VAR}" ]; then
    # Try to use default value
    DEFAULT="${DEFAULTS[$VAR]}"
    if [ -n "$DEFAULT" ]; then
      export "$VAR=$DEFAULT"
      echo "Set $VAR to default: $DEFAULT"
    else
      MISSING+=("$VAR")
    fi
  fi
done

if [ ${#MISSING[@]} -eq 0 ]; then
  echo "All required environment variables are set."
  exit 0
else
  echo "Missing environment variables: ${MISSING[*]}"
  echo "You can set them in your shell, .env file, or use setup_env.sh"
  exit 1
fi
