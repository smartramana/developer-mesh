#!/bin/bash
# Script to remove unused imports from Go files

echo "Removing unused common/logging imports..."

# Find all Go files that import common/logging but don't use it
find /Users/seancorkum/projects/devops-mcp/apps -name "*.go" -type f -exec grep -l "github.com/S-Corkum/devops-mcp/pkg/common/logging" {} \; > files_with_logging.txt

echo "Found $(wc -l < files_with_logging.txt) files that may have unused imports"

# For each file, remove the import if it's not used
while IFS= read -r file; do
  echo "Checking and cleaning imports in $file"
  # Remove the import line if it's not used
  sed -i '' '/github.com\/S-Corkum\/devops-mcp\/pkg\/common\/logging/d' "$file"
done < files_with_logging.txt

# Clean up temporary files
rm files_with_logging.txt

echo "Import cleaning complete."
