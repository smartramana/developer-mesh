#!/bin/bash

# DevOps MCP E2E Test Runner
# This script provides an easy way to run E2E tests against the production environment

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
DEFAULT_MCP_URL="mcp.dev-mesh.io"
DEFAULT_API_URL="api.dev-mesh.io"
DEFAULT_SUITE="all"
DEFAULT_TIMEOUT="30m"
DEFAULT_PARALLEL="5"

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    # Check Go
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go 1.24.3 or higher."
        exit 1
    fi
    
    # Check Ginkgo
    if ! command -v ginkgo &> /dev/null; then
        print_warn "Ginkgo is not installed. Installing..."
        go install github.com/onsi/ginkgo/v2/ginkgo@latest
    fi
    
    # Check environment variables
    if [ -z "${E2E_API_KEY:-}" ]; then
        print_error "E2E_API_KEY environment variable is not set."
        print_info "Please set your API key: export E2E_API_KEY=your-key"
        exit 1
    fi
    
    print_info "Prerequisites check passed ✓"
}

# Function to display help
show_help() {
    cat << EOF
DevOps MCP E2E Test Runner

Usage: $0 [OPTIONS]

Options:
    -s, --suite SUITE        Test suite to run (all, single, multi, performance)
                            Default: all
    -t, --timeout TIMEOUT    Test timeout (e.g., 30m, 1h)
                            Default: 30m
    -p, --parallel N         Number of parallel test specs
                            Default: 5
    -d, --debug             Enable debug logging
    -l, --local             Run against local environment
    -c, --coverage          Generate coverage report
    -w, --watch             Run in watch mode
    -h, --help              Show this help message

Environment Variables:
    E2E_API_KEY             API key for authentication (required)
    MCP_BASE_URL            MCP server URL (default: mcp.dev-mesh.io)
    API_BASE_URL            API server URL (default: api.dev-mesh.io)
    E2E_TENANT_ID           Tenant ID for test isolation
    E2E_DEBUG               Enable debug logging (true/false)

Examples:
    # Run all tests
    $0

    # Run single agent tests with debug
    $0 --suite single --debug

    # Run performance tests with extended timeout
    $0 --suite performance --timeout 60m

    # Run tests against local environment
    $0 --local

    # Run tests in watch mode
    $0 --watch
EOF
}

# Parse command line arguments
SUITE="$DEFAULT_SUITE"
TIMEOUT="$DEFAULT_TIMEOUT"
PARALLEL="$DEFAULT_PARALLEL"
DEBUG="false"
LOCAL="false"
COVERAGE="false"
WATCH="false"

while [[ $# -gt 0 ]]; do
    case $1 in
        -s|--suite)
            SUITE="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -p|--parallel)
            PARALLEL="$2"
            shift 2
            ;;
        -d|--debug)
            DEBUG="true"
            shift
            ;;
        -l|--local)
            LOCAL="true"
            shift
            ;;
        -c|--coverage)
            COVERAGE="true"
            shift
            ;;
        -w|--watch)
            WATCH="true"
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Main execution
main() {
    print_info "DevOps MCP E2E Test Runner"
    print_info "=========================="
    
    # Check prerequisites
    check_prerequisites
    
    # Set environment variables
    if [ "$LOCAL" = "true" ]; then
        export MCP_BASE_URL="localhost:8080"
        export API_BASE_URL="localhost:8081"
        print_info "Running against local environment"
    else
        export MCP_BASE_URL="${MCP_BASE_URL:-$DEFAULT_MCP_URL}"
        export API_BASE_URL="${API_BASE_URL:-$DEFAULT_API_URL}"
        print_info "Running against production environment"
    fi
    
    export E2E_DEBUG="$DEBUG"
    export E2E_PARALLEL_TESTS="$PARALLEL"
    
    print_info "Configuration:"
    print_info "  Suite: $SUITE"
    print_info "  MCP URL: $MCP_BASE_URL"
    print_info "  API URL: $API_BASE_URL"
    print_info "  Timeout: $TIMEOUT"
    print_info "  Parallel: $PARALLEL"
    print_info "  Debug: $DEBUG"
    
    # Create report directory
    mkdir -p test-results
    
    # Build command
    CMD="ginkgo -v"
    
    if [ "$WATCH" = "true" ]; then
        CMD="ginkgo watch -v"
    else
        CMD="$CMD --timeout=$TIMEOUT"
        CMD="$CMD --flake-attempts=2"
        CMD="$CMD --json-report=test-results/report.json"
        CMD="$CMD --junit-report=test-results/junit.xml"
        
        if [ "$COVERAGE" = "true" ]; then
            CMD="$CMD --cover"
            CMD="$CMD --coverprofile=test-results/coverage.out"
        fi
        
        if [ "$SUITE" != "all" ]; then
            case $SUITE in
                single)
                    CMD="$CMD --focus='Single Agent'"
                    ;;
                multi)
                    CMD="$CMD --focus='Multi-Agent'"
                    ;;
                performance)
                    CMD="$CMD --focus='Performance'"
                    ;;
                *)
                    print_error "Unknown suite: $SUITE"
                    exit 1
                    ;;
            esac
        fi
    fi
    
    CMD="$CMD . -- -suite=$SUITE -parallel=$PARALLEL -debug=$DEBUG"
    
    # Run tests
    print_info "Running tests..."
    print_info "Command: $CMD"
    echo ""
    
    # Execute tests
    if eval "$CMD"; then
        print_info "Tests completed successfully! ✓"
        
        # Generate coverage report if requested
        if [ "$COVERAGE" = "true" ] && [ "$WATCH" != "true" ]; then
            print_info "Generating coverage report..."
            go tool cover -html=test-results/coverage.out -o test-results/coverage.html
            print_info "Coverage report available at test-results/coverage.html"
        fi
        
        # Show summary
        if [ -f "test-results/report.json" ] && [ "$WATCH" != "true" ]; then
            print_info "Test Summary:"
            jq -r '.testsuite[] | "  \(.name): \(.tests) tests, \(.failures) failures, \(.time)s"' test-results/report.json
        fi
        
        exit 0
    else
        print_error "Tests failed! ✗"
        
        # Show failed tests
        if [ -f "test-results/report.json" ]; then
            print_error "Failed tests:"
            jq -r '.testsuite[] | select(.failures > 0) | .testcases[] | select(.failure) | "  - \(.name): \(.failure.message)"' test-results/report.json
        fi
        
        exit 1
    fi
}

# Run main function
main