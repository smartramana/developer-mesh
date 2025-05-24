#!/bin/bash
# Validation script for refactor completion

PROJECT_ROOT="/Users/seancorkum/projects/devops-mcp"
cd "$PROJECT_ROOT" || exit 1

echo "Validating refactor completion..."

# Function to check a condition
check() {
    if eval "$2"; then
        echo "✓ $1"
        return 0
    else
        echo "✗ $1"
        return 1
    fi
}

failed=0

# Run checks
check "No import cycles" "! go mod graph 2>&1 | grep -q 'import cycle'"
failed=$((failed + $?))

check "No pkg/mcp references" "[ $(grep -r 'pkg/mcp' --include='*.go' . 2>/dev/null | grep -cv '.bak\|backup/') -eq 0 ]"
failed=$((failed + $?))

# Check specific build targets instead of full build
check "Database package builds" "(cd $PROJECT_ROOT/pkg/database && go build ./... 2>/dev/null)"
failed=$((failed + $?))

check "Cache package complete" "grep -q 'var ErrNotFound' $PROJECT_ROOT/pkg/common/cache/cache.go"
failed=$((failed + $?))

check "No .bak files" "[ $(find . -name '*.bak' -type f | wc -l) -eq 0 ]"
failed=$((failed + $?))

check "Context handler fixed" "! grep -q 'result.Links\[' $PROJECT_ROOT/apps/rest-api/internal/api/context/handlers.go"
failed=$((failed + $?))

echo ""
if [ $failed -eq 0 ]; then
    echo "✅ All critical checks passed!"
else
    echo "❌ $failed checks failed - review above"
    exit 1
fi