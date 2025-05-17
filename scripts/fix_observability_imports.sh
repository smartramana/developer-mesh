#!/bin/bash
# Script to fix observability import issues across the codebase

echo "Fixing observability import issues in the codebase..."

# Find all Go files that contain "import" and "github.com/S-Corkum/devops-mcp/pkg/common/logging"
# but don't have the observability import
find /Users/seancorkum/projects/devops-mcp/apps -name "*.go" -type f -exec grep -l "github.com/S-Corkum/devops-mcp/pkg/common/logging" {} \; | xargs grep -L "github.com/S-Corkum/devops-mcp/pkg/observability" > files_to_fix.txt

echo "Found $(wc -l < files_to_fix.txt) files that need observability import added"

# For each file that needs fixing, add the observability import
while IFS= read -r file; do
  echo "Adding observability import to $file"
  # Add observability import after the common/logging import
  sed -i '' '/github.com\/S-Corkum\/devops-mcp\/pkg\/common\/logging/a\
\	"github.com/S-Corkum/devops-mcp/pkg/observability"
' "$file"
done < files_to_fix.txt

echo "Fixing pointer to interface issues..."

# Find all Go files that contain "*observability.Logger"
find /Users/seancorkum/projects/devops-mcp/apps -name "*.go" -type f -exec grep -l "\*observability\.Logger" {} \; > pointer_to_fix.txt

echo "Found $(wc -l < pointer_to_fix.txt) files with pointer to interface issues"

# For each file that needs fixing, replace "*observability.Logger" with "observability.Logger"
while IFS= read -r file; do
  echo "Fixing pointer to interface in $file"
  sed -i '' 's/\*observability\.Logger/observability.Logger/g' "$file"
done < pointer_to_fix.txt

# Clean up temporary files
rm files_to_fix.txt pointer_to_fix.txt

echo "Fixes complete. Please run 'go build' to check for remaining issues."
