#!/bin/bash
# Script to run GitHub integration tests against real API

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}GitHub Integration Test Runner${NC}"
echo "================================="

# Load environment variables
if [ -f "$PROJECT_ROOT/.env.test" ]; then
    echo -e "${YELLOW}Loading environment from .env.test${NC}"
    export $(grep -v '^#' "$PROJECT_ROOT/.env.test" | xargs)
else
    echo -e "${RED}Warning: .env.test not found${NC}"
fi

# Check required environment variables
# For GitHub App auth, we need either TOKEN or (APP_ID + PRIVATE_KEY)
if [ -z "$GITHUB_TOKEN" ] && [ -z "$GITHUB_APP_ID" ]; then
    echo -e "${RED}Error: Must provide either GITHUB_TOKEN or GITHUB_APP_ID${NC}"
    exit 1
fi

required_vars=(
    "GITHUB_TEST_ORG"
    "GITHUB_TEST_REPO"
)

# Add GitHub App requirements if using App auth
if [ -n "$GITHUB_APP_ID" ] && [ -z "$GITHUB_TOKEN" ]; then
    required_vars+=("GITHUB_APP_PRIVATE_KEY_PATH")
fi

missing_vars=()
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        missing_vars+=("$var")
    fi
done

if [ ${#missing_vars[@]} -ne 0 ]; then
    echo -e "${RED}Error: Missing required environment variables:${NC}"
    printf '%s\n' "${missing_vars[@]}"
    echo ""
    echo "Please set these in .env.test or export them before running this script."
    exit 1
fi

# Validate GitHub credentials
echo -e "${YELLOW}Validating GitHub credentials...${NC}"

if [ -n "$GITHUB_TOKEN" ]; then
    # Validate personal access token
    response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        https://api.github.com/user)
    
    if [ "$response" != "200" ]; then
        echo -e "${RED}Error: Invalid GitHub token (HTTP $response)${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ GitHub token valid${NC}"
elif [ -n "$GITHUB_APP_ID" ]; then
    # Validate GitHub App credentials
    if [ ! -f "$GITHUB_APP_PRIVATE_KEY_PATH" ]; then
        echo -e "${RED}Error: Private key file not found at $GITHUB_APP_PRIVATE_KEY_PATH${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ GitHub App credentials found (ID: $GITHUB_APP_ID)${NC}"
    echo -e "${GREEN}✓ Private key file exists${NC}"
fi

# Check if test repo exists (only if using token auth)
if [ -n "$GITHUB_TOKEN" ]; then
    echo -e "${YELLOW}Checking test repository...${NC}"
    response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/$GITHUB_TEST_ORG/$GITHUB_TEST_REPO")
    
    if [ "$response" != "200" ]; then
        echo -e "${RED}Error: Cannot access repository $GITHUB_TEST_ORG/$GITHUB_TEST_REPO (HTTP $response)${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Test repository accessible${NC}"
else
    # For GitHub App auth, we'll verify repo access during the actual tests
    echo -e "${YELLOW}Repository access will be verified during tests${NC}"
fi

# Run integration tests
echo ""
echo -e "${YELLOW}Running GitHub integration tests...${NC}"
cd "$PROJECT_ROOT"

# Use the GitHub integration config
export CONFIG_FILE="$PROJECT_ROOT/configs/config.github-integration.yaml"

# Run tests with github_real build tag
go test -tags="integration,github_real" \
    -v \
    -timeout 10m \
    ./test/integration \
    -run "TestGitHub" \
    2>&1 | tee github-integration-test.log

# Check test results
if [ ${PIPESTATUS[0]} -eq 0 ]; then
    echo -e "${GREEN}✓ All GitHub integration tests passed!${NC}"
else
    echo -e "${RED}✗ Some tests failed. Check github-integration-test.log for details${NC}"
    exit 1
fi

# Run functional tests if requested
if [ "$1" == "--with-functional" ]; then
    echo ""
    echo -e "${YELLOW}Running functional tests with real GitHub...${NC}"
    
    # Start services
    make docker-compose-up
    
    # Wait for services
    sleep 10
    
    # Run functional tests
    cd "$PROJECT_ROOT/test/functional"
    go test -v ./... 2>&1 | tee "$PROJECT_ROOT/github-functional-test.log"
    
    # Stop services
    cd "$PROJECT_ROOT"
    make docker-compose-down
fi

echo ""
echo -e "${GREEN}Testing complete!${NC}"