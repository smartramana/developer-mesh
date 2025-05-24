#!/bin/bash

# Clean Application Module go.mod Files
# This script removes all the replace directives pointing to pkg subdirectories

set -e

echo "=== Cleaning Application Module go.mod Files ==="
echo ""

# Function to clean a go.mod file
clean_go_mod() {
    local gomod=$1
    echo "Cleaning $gomod..."
    
    # Create a temporary file
    temp_file=$(mktemp)
    
    # Process the file - remove the entire replace block
    awk '
        /^replace \(/ { in_replace=1; next }
        in_replace && /^\)/ { in_replace=0; next }
        !in_replace { print }
    ' "$gomod" > "$temp_file"
    
    # Also remove individual replace directives
    grep -v "^replace github.com/S-Corkum/devops-mcp/pkg/" "$temp_file" > "${temp_file}.2" || true
    mv "${temp_file}.2" "$temp_file"
    
    # Replace the original file
    mv "$temp_file" "$gomod"
}

# Clean each application module
for app_mod in apps/*/go.mod; do
    if [ -f "$app_mod" ]; then
        clean_go_mod "$app_mod"
    fi
done

# Also clean test modules
if [ -f "test/functional/go.mod" ]; then
    clean_go_mod "test/functional/go.mod"
fi

if [ -f "pkg/tests/integration/go.mod" ]; then
    clean_go_mod "pkg/tests/integration/go.mod"
fi

echo ""
echo "=== Application module cleaning complete ==="
echo ""
echo "Next: We need to update these modules to use the root module for pkg imports"