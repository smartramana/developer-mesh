#!/bin/bash

# Production E2E Test Runner for Developer Mesh
# This script runs E2E tests against the production environment at mcp.dev-mesh.io

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}Developer Mesh Production E2E Tests${NC}"
echo "================================"

# Check if API key is set
if [ -z "${E2E_API_KEY:-}" ]; then
    echo -e "${RED}ERROR: E2E_API_KEY is not set${NC}"
    echo "Please set your API key:"
    echo "  export E2E_API_KEY=your-production-api-key"
    exit 1
fi

# Set production environment
export MCP_BASE_URL="mcp.dev-mesh.io"
export API_BASE_URL="api.dev-mesh.io"
export E2E_TENANT_ID="${E2E_TENANT_ID:-e2e-test-tenant}"
export E2E_REPORT_DIR="test-results-$(date +%Y%m%d-%H%M%S)"

echo "Configuration:"
echo "  MCP URL: $MCP_BASE_URL"
echo "  API URL: $API_BASE_URL"
echo "  Tenant: $E2E_TENANT_ID"
echo "  Report: $E2E_REPORT_DIR"
echo ""

# Create report directory
mkdir -p "$E2E_REPORT_DIR"

# Change to E2E test directory
cd "$(dirname "$0")/../e2e"

# Install dependencies if needed
if ! command -v ginkgo &> /dev/null; then
    echo "Installing ginkgo..."
    go install github.com/onsi/ginkgo/v2/ginkgo@latest
fi

# Run tests based on argument
SUITE="${1:-all}"

case "$SUITE" in
    all)
        echo "Running all E2E tests..."
        make test
        ;;
    single)
        echo "Running single agent tests..."
        make test-single
        ;;
    multi)
        echo "Running multi-agent tests..."
        make test-multi
        ;;
    performance)
        echo "Running performance tests..."
        make test-performance
        ;;
    smoke)
        echo "Running smoke tests..."
        # Quick smoke test - just basic connectivity
        ginkgo -v \
            --timeout=5m \
            --focus="should establish a WebSocket connection|should complete full agent lifecycle" \
            --json-report="$E2E_REPORT_DIR/smoke-report.json" \
            .
        ;;
    *)
        echo -e "${RED}Unknown suite: $SUITE${NC}"
        echo "Usage: $0 [all|single|multi|performance|smoke]"
        exit 1
        ;;
esac

# Check results
if [ $? -eq 0 ]; then
    echo -e "\n${GREEN}✓ Tests passed!${NC}"
    
    # Show summary if report exists
    if [ -f "$E2E_REPORT_DIR/report.json" ]; then
        echo -e "\nTest Summary:"
        jq -r '.testsuite[] | "  \(.name): \(.tests) tests, \(.failures) failures"' "$E2E_REPORT_DIR/report.json" 2>/dev/null || true
    fi
else
    echo -e "\n${RED}✗ Tests failed!${NC}"
    
    # Show failed tests
    if [ -f "$E2E_REPORT_DIR/report.json" ]; then
        echo -e "\nFailed tests:"
        jq -r '.testsuite[] | select(.failures > 0) | .testcases[] | select(.failure) | "  - \(.name)"' "$E2E_REPORT_DIR/report.json" 2>/dev/null || true
    fi
    
    exit 1
fi

echo -e "\nReports available in: $E2E_REPORT_DIR/"