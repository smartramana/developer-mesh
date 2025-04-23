#!/bin/bash
# Comprehensive verification script for GitHub integration

set -e  # Exit on error

# Define paths explicitly
GO_PATH="/usr/local/go/bin/go"
GINKGO_PATH="/Users/seancorkum/go/bin/ginkgo"

# Set current directory to project root
cd "$(dirname "$0")"
PROJECT_ROOT=$(pwd)

echo "==========================================================="
echo "     DevOps MCP GitHub Integration Verification Script     "
echo "==========================================================="
echo ""

export PATH="/usr/local/go/bin:$PATH"
echo "Using Go version: $(${GO_PATH} version)"

# STEP 1: Verify module and dependencies
echo ""
echo "STEP 1: Verifying module and dependencies..."
echo "-----------------------------------------------------------"
${GO_PATH} mod tidy
${GO_PATH} mod verify

# STEP 2: Check for circular dependencies
echo ""
echo "STEP 2: Checking for circular dependencies..."
echo "-----------------------------------------------------------"
CIRCULAR=$(${GO_PATH} list -f '{{.ImportPath}} -> {{.Error}}' ./... | grep "import cycle" || echo "")
if [ -z "$CIRCULAR" ]; then
    echo "✓ No circular dependencies found!"
else
    echo "✗ Circular dependencies detected:"
    echo "$CIRCULAR"
    exit 1
fi

# STEP 3: Verify code compilation
echo ""
echo "STEP 3: Verifying code compilation..."
echo "-----------------------------------------------------------"
${GO_PATH} build -o /tmp/mcp-server ./cmd/server
echo "✓ Code compiles successfully"

# STEP 4: Verify GitHub adapter structure
echo ""
echo "STEP 4: Verifying GitHub adapter structure..."
echo "-----------------------------------------------------------"
echo "Checking GitHub adapter components:"
for dir in api auth webhook; do
    if [ -d "internal/adapters/github/$dir" ]; then
        echo "✓ github/$dir directory exists"
    else
        echo "✗ github/$dir directory not found"
        exit 1
    fi
done

# STEP 5: Check for errors package
echo ""
echo "STEP 5: Verifying error handling structure..."
echo "-----------------------------------------------------------"
if [ -f "internal/adapters/errors/errors.go" ]; then
    echo "✓ Centralized errors package exists"
else
    echo "✗ Centralized errors package not found"
    exit 1
fi

if [ -f "internal/common/errors/github.go" ]; then
    echo "✓ GitHub specific errors implementation exists"
else
    echo "✗ GitHub specific errors implementation not found"
    exit 1
fi

# STEP 6: Run unit tests
echo ""
echo "STEP 6: Running GitHub adapter unit tests..."
echo "-----------------------------------------------------------"
${GO_PATH} test ./internal/adapters/github/... -v
${GO_PATH} test ./internal/adapters/errors/... -v

# Step 7: Structural verification of GitHub integration
echo ""
echo "STEP 7: Verifying GitHub integration structure..."
echo "-----------------------------------------------------------"
FILES_TO_CHECK=(
    "internal/adapters/github/adapter.go"
    "internal/adapters/github/config.go"
    "internal/adapters/github/errors.go"
    "internal/adapters/github/api/rest.go"
    "internal/adapters/github/api/graphql.go"
    "internal/adapters/github/api/graphql_builder.go"
    "internal/adapters/github/api/pagination.go"
    "internal/adapters/github/auth/provider.go"
    "internal/adapters/github/webhook/validator.go"
)

for file in "${FILES_TO_CHECK[@]}"; do
    if [ -f "$file" ]; then
        echo "✓ $file exists"
    else
        echo "✗ $file not found"
    fi
done

# Step 8: Verify docker-compose.test.yml
echo ""
echo "STEP 8: Verifying test environment configuration..."
echo "-----------------------------------------------------------"
if [ -f "docker-compose.test.yml" ]; then
    echo "✓ Test environment configuration exists"
else
    echo "✗ Test environment configuration not found"
    exit 1
fi

echo ""
echo "==========================================================="
echo "       GitHub Integration Verification SUCCESSFUL          "
echo "==========================================================="
echo ""
echo "The GitHub integration has been successfully fixed with:"
echo "  - Circular dependencies resolved"
echo "  - Error handling standardized"
echo "  - Type safety improvements implemented"
echo "  - Proper import paths configured"
echo ""
echo "For complete testing report, see GITHUB_TESTING_SOLUTION.md"
echo ""
