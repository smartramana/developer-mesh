#!/bin/bash

# Update Application Dependencies
# This script updates application modules to use the root module for pkg imports

set -e

echo "=== Updating Application Dependencies ==="
echo ""

# Function to update go.mod to use root module
update_go_mod() {
    local gomod=$1
    local dir=$(dirname "$gomod")
    
    echo "Updating $gomod..."
    
    cd "$dir"
    
    # Remove all existing pkg requirements
    sed -i '' '/github.com\/S-Corkum\/devops-mcp\/pkg\//d' go.mod
    
    # Add requirement for the root module
    if ! grep -q "require github.com/S-Corkum/devops-mcp v0.0.0" go.mod; then
        # Find the require block and add the root module
        awk '/^require \(/ {
            print
            print "\tgithub.com/S-Corkum/devops-mcp v0.0.0"
            next
        }
        {print}' go.mod > go.mod.tmp && mv go.mod.tmp go.mod
    fi
    
    # Add replace directive for the root module
    if ! grep -q "replace github.com/S-Corkum/devops-mcp =>" go.mod; then
        echo "" >> go.mod
        echo "replace github.com/S-Corkum/devops-mcp => $(realpath --relative-to="$dir" /Users/seancorkum/projects/devops-mcp)" >> go.mod
    fi
    
    cd - > /dev/null
}

# Update each application module
for app_dir in apps/*/; do
    if [ -f "$app_dir/go.mod" ]; then
        update_go_mod "$app_dir/go.mod"
    fi
done

# Update test modules
if [ -f "test/functional/go.mod" ]; then
    update_go_mod "test/functional/go.mod"
fi

if [ -f "pkg/tests/integration/go.mod" ]; then
    update_go_mod "pkg/tests/integration/go.mod"
fi

echo ""
echo "=== Dependency update complete ==="
echo ""
echo "Next: Running go mod tidy on all modules..."

# Try to tidy the root module first
echo "Tidying root module..."
go mod tidy 2>&1 | head -20 || echo "Root module tidy failed (expected)"

echo ""
echo "Application modules will be tidied after fixing imports"