#!/bin/bash
# Update dependencies for all modules in the Go workspace
# This script handles the complexity of updating Go workspace dependencies

set -euo pipefail

echo "=== Updating dependencies for all modules ==="

# Ensure workspace is synchronized first
echo "Synchronizing Go workspace..."
go work sync

# Get all module directories from go.work
modules=$(go work edit -json | jq -r '.Use[].DiskPath // empty' 2>/dev/null || echo "")

if [ -z "$modules" ]; then
    echo "No modules found in go.work, falling back to directory scan"
    modules="apps/mcp-server apps/rest-api apps/worker pkg"
fi

# Update each module
for module in $modules; do
    if [ -d "$module" ]; then
        echo ""
        echo "=== Updating dependencies in: $module ==="
        (
            cd "$module"
            
            # Update direct dependencies
            echo "Updating direct dependencies..."
            go get -u ./...
            
            # Update test dependencies
            echo "Updating test dependencies..."
            go get -t -u ./...
            
            # Clean up
            echo "Running go mod tidy..."
            go mod tidy
        )
    fi
done

# Sync workspace after updates
echo ""
echo "=== Final workspace sync ==="
go work sync

echo ""
echo "✓ All dependencies updated successfully!"