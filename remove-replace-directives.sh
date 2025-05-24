#!/bin/bash

# Remove replace directives from application modules when using workspace
# Workspace handles the module resolution automatically

set -e

echo "=== Removing Replace Directives from Application Modules ==="
echo ""

# Function to remove replace directives
remove_replace() {
    local gomod=$1
    echo "Cleaning $gomod..."
    
    # Remove replace directives for the root module
    sed -i '' '/replace github.com\/S-Corkum\/devops-mcp/d' "$gomod"
    
    # Remove empty replace blocks
    sed -i '' '/^replace ($/,/^)$/d' "$gomod"
    
    # Remove trailing empty lines
    sed -i '' -e :a -e '/^\s*$/d;N;ba' "$gomod"
}

# Clean each application module
for gomod in apps/*/go.mod test/functional/go.mod pkg/tests/integration/go.mod; do
    if [ -f "$gomod" ]; then
        remove_replace "$gomod"
    fi
done

echo ""
echo "=== Replace directive removal complete ==="
echo ""
echo "In a workspace, module resolution is handled automatically."
echo "Let's try building again..."