#!/bin/bash

# Comprehensive Module Structure Fix
# This script consolidates the module structure to fix import issues

set -e

echo "=== Comprehensive Module Structure Fix ==="
echo ""

# Step 1: Remove unnecessary go.mod files from pkg subdirectories
echo "Step 1: Removing unnecessary go.mod files from pkg subdirectories..."
# These should be packages, not modules
modules_to_remove=(
    "pkg/adapters/go.mod"
    "pkg/api/go.mod"
    "pkg/aws/go.mod"
    "pkg/cache/go.mod"
    "pkg/chunking/go.mod"
    "pkg/client/go.mod"
    "pkg/common/go.mod"
    "pkg/common/config/go.mod"
    "pkg/common/observability/go.mod"
    "pkg/config/go.mod"
    "pkg/core/go.mod"
    "pkg/database/go.mod"
    "pkg/database/adapters/go.mod"
    "pkg/embedding/go.mod"
    "pkg/events/go.mod"
    "pkg/events/system/go.mod"
    "pkg/interfaces/go.mod"
    "pkg/migrations/go.mod"
    "pkg/models/go.mod"
    "pkg/observability/go.mod"
    "pkg/queue/go.mod"
    "pkg/relationship/go.mod"
    "pkg/repository/go.mod"
    "pkg/repository/vector/go.mod"
    "pkg/storage/go.mod"
    "pkg/util/go.mod"
    "pkg/worker/go.mod"
)

for mod in "${modules_to_remove[@]}"; do
    if [ -f "$mod" ]; then
        echo "  Removing $mod"
        rm -f "$mod"
        # Also remove go.sum if it exists
        sum_file="${mod%.mod}.sum"
        if [ -f "$sum_file" ]; then
            rm -f "$sum_file"
        fi
    fi
done

# Step 2: Update go.work to only include actual application modules
echo ""
echo "Step 2: Updating go.work file..."
cat > go.work << 'EOF'
go 1.24.2

use (
    .
    ./apps/mcp-server
    ./apps/mcp-server/tests/integration
    ./apps/mockserver
    ./apps/rest-api
    ./apps/worker
    ./pkg/tests/integration
    ./test/functional
)
EOF

# Step 3: Update the root go.mod to not have replace directives for internal packages
echo ""
echo "Step 3: Cleaning root go.mod..."
# Remove all the internal replace directives
sed -i '' '/replace.*github.com\/S-Corkum\/devops-mcp\/pkg/d' go.mod
sed -i '' '/github.com\/S-Corkum\/devops-mcp\/pkg\//d' go.mod

# Step 4: Update application go.mod files to not have replace directives
echo ""
echo "Step 4: Cleaning application go.mod files..."
for app_mod in apps/*/go.mod; do
    if [ -f "$app_mod" ]; then
        echo "  Cleaning $app_mod"
        # Remove replace directives for internal packages
        sed -i '' '/replace.*github.com\/S-Corkum\/devops-mcp\/pkg/d' "$app_mod"
        sed -i '' '/replace.*github.com\/S-Corkum\/devops-mcp\/internal/d' "$app_mod"
    fi
done

# Step 5: Fix imports in Go files
echo ""
echo "Step 5: Fixing imports in Go files..."
# Update any imports that reference pkg packages as modules
find . -name "*.go" -type f | grep -v "/vendor/" | grep -v "/backup/" | while read -r file; do
    # Check if file has problematic imports
    if grep -q "github.com/S-Corkum/devops-mcp/internal/" "$file"; then
        # Get the directory of the file
        file_dir=$(dirname "$file")
        # Check if it's in a different module (not under apps/)
        if [[ ! "$file_dir" =~ ^./apps/ ]] && [[ "$file_dir" != "." ]]; then
            echo "  WARNING: $file imports from internal packages (violates Go internal rules)"
        fi
    fi
done

# Step 6: Run go mod tidy on remaining modules
echo ""
echo "Step 6: Running go mod tidy on application modules..."
# Root module
echo "  Tidying root module..."
go mod tidy || echo "    Warning: Failed to tidy root module"

# Application modules
for app_dir in apps/*/; do
    if [ -f "$app_dir/go.mod" ]; then
        echo "  Tidying $app_dir"
        (cd "$app_dir" && go mod tidy) || echo "    Warning: Failed to tidy $app_dir"
    fi
done

# Test modules
echo "  Tidying test/functional..."
(cd test/functional && go mod tidy) || echo "    Warning: Failed to tidy test/functional"

echo "  Tidying pkg/tests/integration..."
(cd pkg/tests/integration && go mod tidy) || echo "    Warning: Failed to tidy pkg/tests/integration"

echo ""
echo "=== Module structure fix complete ==="
echo ""
echo "Summary of changes:"
echo "1. Removed go.mod files from pkg subdirectories (they are now packages, not modules)"
echo "2. Updated go.work to only include application and test modules"
echo "3. Removed internal replace directives"
echo "4. The pkg/ directory is now part of the root module"
echo ""
echo "Next steps:"
echo "1. Run 'make build' to verify the build works"
echo "2. Fix any import issues that arise"
echo "3. Update imports in test files that reference internal packages"