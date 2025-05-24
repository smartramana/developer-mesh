#!/bin/bash
# Script to update references from pkg/mcp to pkg/models across the codebase
# This script is part of Phase 6.1 of the Go Workspace migration

set -e
echo "Updating references from pkg/mcp to pkg/models across the codebase..."

# Find .go files containing pkg/mcp references and update them
find /Users/seancorkum/projects/devops-mcp -type f -name "*.go" -exec grep -l "github.com/S-Corkum/devops-mcp/pkg/mcp" {} \; | while read file; do
    echo "Updating $file"
    # Replace import statements
    sed -i '' 's|"github.com/S-Corkum/devops-mcp/pkg/mcp"|"github.com/S-Corkum/devops-mcp/pkg/models"|g' "$file"
    
    # Replace any aliased imports (like legacymcp "github.com/S-Corkum/devops-mcp/pkg/mcp")
    sed -i '' 's|\([[:alnum:]]*\) "github.com/S-Corkum/devops-mcp/pkg/mcp"|\1 "github.com/S-Corkum/devops-mcp/pkg/models"|g' "$file"
    
    # Replace type references: mcp.Context to models.Context
    sed -i '' 's/mcp\.Context/models.Context/g' "$file"
    sed -i '' 's/mcp\.ContextItem/models.ContextItem/g' "$file"
    sed -i '' 's/mcp\.Event/models.Event/g' "$file"
    sed -i '' 's/\*mcp\./\*models./g' "$file"
    sed -i '' 's/\[\]mcp\./\[\]models./g' "$file"
    sed -i '' 's/\[\]\*mcp\./\[\]\*models./g' "$file"
done

# Update go.mod files that have pkg/mcp dependencies
find /Users/seancorkum/projects/devops-mcp -type f -name "go.mod" -exec grep -l "github.com/S-Corkum/devops-mcp/pkg/mcp" {} \; | while read file; do
    echo "Updating dependencies in $file"
    # Comment out the pkg/mcp dependency
    sed -i '' 's|github.com/S-Corkum/devops-mcp/pkg/mcp.*|// Removed reference to non-existent pkg/mcp package|g' "$file"
    
    # Also comment out replace directives for pkg/mcp
    sed -i '' 's|github.com/S-Corkum/devops-mcp/pkg/mcp =.*|// Removed replace directive for non-existent pkg/mcp package|g' "$file"
done

echo "Dependency update completed. Now running go mod tidy to clean up dependencies."

# Run go mod tidy on each module to clean up dependencies
find /Users/seancorkum/projects/devops-mcp -name "go.mod" -execdir sh -c "echo 'Running go mod tidy in \$(pwd)' && go mod tidy" \;

echo "Process completed. Please review the changes and run tests to verify functionality."
