#!/bin/bash
# Test all modules in the Go workspace
# This script dynamically discovers and tests all modules to avoid hardcoding paths

set -euo pipefail

echo "=== Testing all modules in Go workspace ==="

# Check if go.work exists, if not create it
if [ ! -f "go.work" ]; then
    echo "No go.work file found, creating one..."
    go work init
    # Add all modules
    for dir in apps/mcp-server apps/rest-api apps/worker pkg; do
        if [ -f "$dir/go.mod" ]; then
            go work use "$dir"
        fi
    done
fi

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
        # Skip functional tests in unit test runs
        if [[ "$module" == *"test/functional"* ]] || [[ "$module" == *"test/github-live"* ]]; then
            echo ""
            echo "=== Skipping functional test module: $module ==="
            continue
        fi
        
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

# Skip workspace-level tests as they're not supported with Go workspaces
# Individual module tests already cover all the code
echo ""
echo "=== Skipping workspace-level tests (not supported with Go workspaces) ==="

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