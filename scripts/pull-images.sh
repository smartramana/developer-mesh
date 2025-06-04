#!/usr/bin/env bash
# Script to pull Docker images from GitHub Container Registry

set -euo pipefail

# Configuration
REGISTRY="ghcr.io"
GITHUB_USERNAME="${GITHUB_USERNAME:-}"
IMAGE_PREFIX="devops-mcp"
VERSION="${VERSION:-latest}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Services to pull
SERVICES=("mcp-server" "rest-api" "worker" "mockserver")

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "info") echo -e "${GREEN}[INFO]${NC} $message" ;;
        "warn") echo -e "${YELLOW}[WARN]${NC} $message" ;;
        "error") echo -e "${RED}[ERROR]${NC} $message" ;;
    esac
}

# Check if GitHub username is provided
if [ -z "$GITHUB_USERNAME" ]; then
    print_status "error" "GitHub username not set. Please set GITHUB_USERNAME environment variable."
    echo "Usage: GITHUB_USERNAME=your-username $0 [version]"
    echo "Example: GITHUB_USERNAME=s-corkum $0 v1.2.3"
    exit 1
fi

# Use provided version or default to latest
if [ $# -ge 1 ]; then
    VERSION="$1"
fi

print_status "info" "Pulling Docker images from $REGISTRY"
print_status "info" "GitHub Username: $GITHUB_USERNAME"
print_status "info" "Version: $VERSION"
echo ""

# Check if authenticated for private registries
if ! docker pull "${REGISTRY}/hello-world" &>/dev/null; then
    print_status "warn" "Not authenticated to $REGISTRY. Attempting to pull public images..."
    print_status "info" "For private images, run: echo \$GITHUB_TOKEN | docker login $REGISTRY -u $GITHUB_USERNAME --password-stdin"
fi

# Pull each service image
failed_pulls=0
for service in "${SERVICES[@]}"; do
    image="${REGISTRY}/${GITHUB_USERNAME}/${IMAGE_PREFIX}-${service}:${VERSION}"
    print_status "info" "Pulling $image..."
    
    if docker pull "$image"; then
        print_status "info" "Successfully pulled $service"
    else
        print_status "error" "Failed to pull $service"
        ((failed_pulls++))
    fi
    echo ""
done

# Summary
echo "========================================"
if [ $failed_pulls -eq 0 ]; then
    print_status "info" "All images pulled successfully!"
    echo ""
    echo "To run the services, use:"
    echo "  docker-compose -f docker-compose.prod.yml up -d"
    echo ""
    echo "Make sure to update docker-compose.prod.yml with your GitHub username."
else
    print_status "error" "$failed_pulls image(s) failed to pull"
    exit 1
fi

# List pulled images
echo ""
print_status "info" "Pulled images:"
docker images | grep -E "(REPOSITORY|${GITHUB_USERNAME}/${IMAGE_PREFIX})" | grep -E "(REPOSITORY|${VERSION})"