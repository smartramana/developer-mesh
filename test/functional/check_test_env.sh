#!/bin/bash
# Checks and optionally sets required environment variables for functional tests

# Define default values using simple variables
DEFAULT_MCP_SERVER_URL="http://localhost:8080"
DEFAULT_MCP_API_KEY="dev-admin-key-1234567890"
DEFAULT_MOCKSERVER_URL="http://localhost:8082"
DEFAULT_GITHUB_TOKEN="test-github-token"
DEFAULT_GITHUB_REPO="test-repo"
DEFAULT_GITHUB_OWNER="test-org"
DEFAULT_ELASTICACHE_ENDPOINT="localhost"
DEFAULT_ELASTICACHE_PORT="6379"
DEFAULT_MCP_GITHUB_WEBHOOK_SECRET="docker-github-webhook-secret"
DEFAULT_REDIS_ADDR="localhost:6379"

# Load .env file if it exists
if [ -f "$(dirname "$0")/.env" ]; then
  export $(cat "$(dirname "$0")/.env" | grep -v '^#' | xargs)
fi

# Check and set variables
MISSING=""

# Function to check and set variable
check_and_set() {
  local var_name=$1
  local default_var_name="DEFAULT_${var_name}"
  local default_value="${!default_var_name}"
  
  if [ -z "${!var_name}" ]; then
    if [ -n "$default_value" ]; then
      export "$var_name=$default_value"
      echo "Set $var_name to default: $default_value"
    else
      MISSING="$MISSING $var_name"
    fi
  fi
}

# Check each required variable
check_and_set MCP_SERVER_URL
check_and_set MCP_API_KEY
check_and_set MOCKSERVER_URL
check_and_set GITHUB_TOKEN
check_and_set GITHUB_REPO
check_and_set GITHUB_OWNER
check_and_set ELASTICACHE_ENDPOINT
check_and_set ELASTICACHE_PORT
check_and_set MCP_GITHUB_WEBHOOK_SECRET
check_and_set REDIS_ADDR

if [ -z "$MISSING" ]; then
  echo "All required environment variables are set."
  exit 0
else
  echo "Missing environment variables:$MISSING"
  echo "You can set them in your shell, .env file, or use setup_env.sh"
  exit 1
fi
