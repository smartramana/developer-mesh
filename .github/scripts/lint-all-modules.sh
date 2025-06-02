#!/bin/bash
# Lint all modules in the Go workspace
# This script dynamically discovers and lints all modules

set -euo pipefail

echo "=== Linting all modules in Go workspace ==="

# Ensure workspace is synchronized
echo "Synchronizing Go workspace..."
go work sync

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    echo "golangci-lint not found. Please install it first."
    exit 1
fi

# Get all module directories from go.work
modules=$(go work edit -json | jq -r '.Use[].DiskPath // empty' 2>/dev/null || echo "")

if [ -z "$modules" ]; then
    echo "No modules found in go.work, falling back to directory scan"
    modules="apps/mcp-server apps/rest-api apps/worker pkg"
fi

failed_modules=""

# Lint each module
for module in $modules; do
    if [ -d "$module" ]; then
        echo ""
        echo "=== Linting module: $module ==="
        if (cd "$module" && golangci-lint run ./...); then
            echo "✓ Linting passed for $module"
        else
            echo "✗ Linting failed for $module"
            failed_modules="$failed_modules $module"
        fi
    fi
done

# Report results
echo ""
echo "=== Lint Summary ==="
if [ -z "$failed_modules" ]; then
    echo "✓ All modules passed linting!"
    exit 0
else
    echo "✗ Linting failed in:$failed_modules"
    exit 1
fi