#!/bin/bash

# Script to fix remaining S-Corkum references

set -e

echo "Fixing remaining S-Corkum references..."

# Find all files with S-Corkum references and replace them
find . -type f \( -name "*.md" -o -name "*.yaml" -o -name "*.yml" -o -name "*.go" -o -name "*.sh" -o -name "*.json" \) \
  -not -path "./.git/*" \
  -not -path "./scripts/fix-remaining-references.sh" \
  -exec grep -l "S-Corkum" {} \; | while read -r file; do
    echo "Updating: $file"
    sed -i '' 's/S-Corkum/developer-mesh/g' "$file"
done

echo "Done! All S-Corkum references have been replaced with developer-mesh"