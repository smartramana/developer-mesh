#!/bin/bash
# Simple lint script that works reliably in CI
set -euo pipefail

echo "=== Running linter on all modules ==="

# Run from the root directory to maintain proper go.work context
failed=0

# Lint each module from the root directory
for module in apps/mcp-server apps/rest-api apps/worker apps/mockserver pkg; do
    if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
        echo "Linting $module..."
        if ! golangci-lint run "./${module}/..."; then
            echo "✗ Linting failed for $module"
            failed=1
        else
            echo "✓ Linting passed for $module"
        fi
    fi
done

if [ $failed -eq 0 ]; then
    echo "✓ All modules passed linting!"
    exit 0
else
    echo "✗ Some modules failed linting"
    exit 1
fi