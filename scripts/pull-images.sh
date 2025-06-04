#!/bin/bash
# Script to pull DevOps MCP Docker images from GitHub Container Registry

set -e

# Configuration
GITHUB_USERNAME=${GITHUB_USERNAME:-"your-github-username"}
VERSION=${1:-"latest"}
REGISTRY="ghcr.io"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Services to pull
SERVICES=(
    "mcp-server"
    "rest-api"
    "worker"
    "mockserver"
)

echo -e "${GREEN}DevOps MCP Docker Image Pull Script${NC}"
echo "======================================"
echo "Registry: $REGISTRY"
echo "Username: $GITHUB_USERNAME"
echo "Version: $VERSION"
echo ""

# Check if GITHUB_USERNAME is set properly
if [ "$GITHUB_USERNAME" = "your-github-username" ]; then
    echo -e "${RED}Error: Please set GITHUB_USERNAME environment variable${NC}"
    echo "Example: GITHUB_USERNAME=s-corkum ./scripts/pull-images.sh"
    exit 1
fi

# Function to pull image
pull_image() {
    local service=$1
    local image="${REGISTRY}/${GITHUB_USERNAME}/devops-mcp-${service}:${VERSION}"
    
    echo -e "${YELLOW}Pulling ${service}...${NC}"
    if docker pull "$image"; then
        echo -e "${GREEN}✓ Successfully pulled ${service}${NC}"
        return 0
    else
        echo -e "${RED}✗ Failed to pull ${service}${NC}"
        return 1
    fi
}

# Pull all images
failed=0
for service in "${SERVICES[@]}"; do
    if ! pull_image "$service"; then
        ((failed++))
    fi
    echo ""
done

# Summary
echo "======================================"
if [ $failed -eq 0 ]; then
    echo -e "${GREEN}✓ All images pulled successfully!${NC}"
    echo ""
    echo "You can now run:"
    echo "  docker-compose -f docker-compose.prod.yml up -d"
else
    echo -e "${RED}✗ Failed to pull $failed images${NC}"
    echo ""
    echo "Please check:"
    echo "1. The GITHUB_USERNAME is correct"
    echo "2. The images are published to the registry"
    echo "3. You have access to the repository"
    exit 1
fi

# List pulled images
echo ""
echo "Pulled images:"
docker images | grep -E "(REPOSITORY|devops-mcp)" | grep -v "<none>"