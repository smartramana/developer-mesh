#!/bin/bash
# Test all modules in the Go workspace
# This script dynamically discovers and tests all modules to avoid hardcoding paths

set -euo pipefail

echo "=== Testing all modules in Go workspace ==="

# Ensure workspace is synchronized
echo "Synchronizing Go workspace..."
go work sync

# Get all module directories from go.work
modules=$(go work edit -json | jq -r '.Use[].DiskPath // empty' 2>/dev/null || echo "")

if [ -z "$modules" ]; then
    echo "No modules found in go.work, falling back to directory scan"
    modules="apps/mcp-server apps/rest-api apps/worker pkg"
fi

failed_modules=""
test_flags="${TEST_FLAGS:-}"

# Test each module
for module in $modules; do
    if [ -d "$module" ]; then
        echo ""
        echo "=== Testing module: $module ==="
        if (cd "$module" && go test $test_flags ./...); then
            echo "✓ Tests passed for $module"
        else
            echo "✗ Tests failed for $module"
            failed_modules="$failed_modules $module"
        fi
    fi
done

# Test from workspace root for cross-module integration
echo ""
echo "=== Running workspace-level tests ==="
if go test $test_flags ./...; then
    echo "✓ Workspace tests passed"
else
    echo "✗ Workspace tests failed"
    failed_modules="$failed_modules workspace"
fi

# Report results
echo ""
echo "=== Test Summary ==="
if [ -z "$failed_modules" ]; then
    echo "✓ All tests passed!"
    exit 0
else
    echo "✗ Tests failed in:$failed_modules"
    exit 1
fi