#!/bin/bash

# Fix Module Import Issues Script
# This script fixes the module import structure issues blocking the build

set -e

echo "=== Fixing Module Import Structure ==="

# 1. Remove incorrect replace directives for pkg/adapters/events
echo "Step 1: Removing pkg/adapters/events module references..."
for gomod in $(find . -name "go.mod" -type f); do
    if grep -q "pkg/adapters/events =>" "$gomod"; then
        echo "  Fixing $gomod"
        sed -i '' '/pkg\/adapters\/events =>/d' "$gomod"
    fi
done

# 2. Update imports to use pkg/adapters/events as package, not module
echo "Step 2: Updating imports to use correct path..."
find . -name "*.go" -type f | while read -r file; do
    if grep -q "github.com/your-org/devops-mcp/pkg/adapters/events" "$file"; then
        echo "  Updating $file"
        sed -i '' 's|github.com/your-org/devops-mcp/pkg/adapters/events|github.com/your-org/devops-mcp/pkg/adapters/events|g' "$file"
    fi
done

# 3. Clean up go.work file
echo "Step 3: Cleaning go.work file..."
if [ -f "go.work" ]; then
    # Remove non-existent modules
    sed -i '' '/pkg\/adapters\/events/d' go.work
    sed -i '' '/pkg\/adapters\/resilience/d' go.work
    sed -i '' '/pkg\/adapters\/providers/d' go.work
fi

# 4. Fix pkg/adapters go.mod to not have replace directives for its own subpackages
echo "Step 4: Fixing pkg/adapters/go.mod..."
if [ -f "pkg/adapters/go.mod" ]; then
    sed -i '' '/replace.*pkg\/adapters\//d' pkg/adapters/go.mod
fi

# 5. Run go mod tidy on all modules
echo "Step 5: Running go mod tidy on all modules..."
for gomod in $(find . -name "go.mod" -type f | grep -v "/vendor/" | grep -v "/backup/"); do
    dir=$(dirname "$gomod")
    echo "  Tidying $dir"
    (cd "$dir" && go mod tidy) || echo "    Warning: Failed to tidy $dir"
done

echo ""
echo "=== Module import fixes complete ==="
echo "Next steps:"
echo "1. Run 'make build' to verify the build works"
echo "2. Run './validate-refactor.sh' to check overall status"