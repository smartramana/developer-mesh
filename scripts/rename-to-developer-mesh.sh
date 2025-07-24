#!/bin/bash

# Script to rename project from developer-mesh to developer-mesh
# This script performs a comprehensive search and replace across the entire codebase

set -e

echo "=== Renaming project from developer-mesh to developer-mesh ==="
echo

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to show progress
show_progress() {
    echo -e "${GREEN}✓${NC} $1"
}

# Function to show warning
show_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Function to show error
show_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d ".git" ]; then
    show_error "This script must be run from the root of the developer-mesh project"
    exit 1
fi

# Backup important files
echo -e "${BLUE}Creating backup of important files...${NC}"
cp go.mod go.mod.bak
cp go.work go.work.bak
cp README.md README.md.bak
cp CLAUDE.md CLAUDE.md.bak
show_progress "Backup created"

# Step 1: Update Go module paths
echo
echo -e "${BLUE}Step 1: Updating Go module paths...${NC}"

# Update main go.mod
sed -i '' 's|github.com/developer-mesh/developer-mesh|github.com/developer-mesh/developer-mesh|g' go.mod
show_progress "Updated main go.mod"

# Update all go.mod files in the workspace
find . -name "go.mod" -type f | while read -r file; do
    sed -i '' 's|github.com/developer-mesh/developer-mesh|github.com/developer-mesh/developer-mesh|g' "$file"
    show_progress "Updated $file"
done

# Step 2: Update all Go import statements
echo
echo -e "${BLUE}Step 2: Updating Go import statements...${NC}"

# Find all Go files and update imports
find . -name "*.go" -type f | while read -r file; do
    if grep -q "github.com/developer-mesh/developer-mesh" "$file"; then
        sed -i '' 's|github.com/developer-mesh/developer-mesh|github.com/developer-mesh/developer-mesh|g' "$file"
        show_progress "Updated imports in $file"
    fi
done

# Step 3: Replace developer-mesh with developer-mesh (case-sensitive)
echo
echo -e "${BLUE}Step 3: Replacing 'developer-mesh' with 'developer-mesh'...${NC}"

# Create a list of file extensions to process
EXTENSIONS="md yaml yml go mod sh json xml html env example txt Makefile Dockerfile"

for ext in $EXTENSIONS; do
    if [ "$ext" = "Makefile" ] || [ "$ext" = "Dockerfile" ]; then
        find . -name "$ext" -type f | while read -r file; do
            if grep -q "developer-mesh" "$file"; then
                sed -i '' 's/developer-mesh/developer-mesh/g' "$file"
                show_progress "Updated $file"
            fi
        done
    else
        find . -name "*.$ext" -type f | while read -r file; do
            if grep -q "developer-mesh" "$file"; then
                sed -i '' 's/developer-mesh/developer-mesh/g' "$file"
                show_progress "Updated $file"
            fi
        done
    fi
done

# Step 4: Replace Developer Mesh with Developer Mesh (case-sensitive)
echo
echo -e "${BLUE}Step 4: Replacing 'Developer Mesh' with 'Developer Mesh'...${NC}"

for ext in $EXTENSIONS; do
    if [ "$ext" = "Makefile" ] || [ "$ext" = "Dockerfile" ]; then
        find . -name "$ext" -type f | while read -r file; do
            if grep -q "Developer Mesh" "$file"; then
                sed -i '' 's/Developer Mesh/Developer Mesh/g' "$file"
                show_progress "Updated $file"
            fi
        done
    else
        find . -name "*.$ext" -type f | while read -r file; do
            if grep -q "Developer Mesh" "$file"; then
                sed -i '' 's/Developer Mesh/Developer Mesh/g' "$file"
                show_progress "Updated $file"
            fi
        done
    fi
done

# Step 5: Update git remote origin
echo
echo -e "${BLUE}Step 5: Updating git remote origin...${NC}"

# Show current origin
CURRENT_ORIGIN=$(git remote get-url origin)
echo "Current origin: $CURRENT_ORIGIN"

# Update to new origin
git remote set-url origin https://github.com/developer-mesh/developer-mesh
NEW_ORIGIN=$(git remote get-url origin)
echo "New origin: $NEW_ORIGIN"
show_progress "Git remote origin updated"

# Step 6: Handle special cases
echo
echo -e "${BLUE}Step 6: Handling special cases...${NC}"

# Update docker image names if they exist
find . -name "docker-compose*.yml" -type f | while read -r file; do
    if grep -q "developer-mesh" "$file"; then
        sed -i '' 's|image: developer-mesh|image: developer-mesh|g' "$file"
        show_progress "Updated Docker image names in $file"
    fi
done

# Update any AWS resource names
find . -name "*.yaml" -o -name "*.yml" -o -name "*.env*" | while read -r file; do
    if grep -q "sean-mcp" "$file"; then
        show_warning "Found AWS resource name 'sean-mcp' in $file - manual review may be needed"
    fi
done

# Step 7: Clean up go.sum files
echo
echo -e "${BLUE}Step 7: Cleaning up go.sum files...${NC}"
find . -name "go.sum" -type f -exec rm {} \;
show_progress "Removed all go.sum files (will be regenerated)"

# Step 8: Summary
echo
echo -e "${BLUE}=== Summary ===${NC}"
echo

# Count changes
GO_FILES_CHANGED=$(find . -name "*.go" -type f -exec grep -l "developer-mesh" {} \; | wc -l)
MD_FILES_CHANGED=$(find . -name "*.md" -type f -exec grep -l "developer-mesh" {} \; | wc -l)
CONFIG_FILES_CHANGED=$(find . \( -name "*.yaml" -o -name "*.yml" -o -name "*.json" \) -type f -exec grep -l "developer-mesh" {} \; | wc -l)

echo "Files updated:"
echo "  - Go files: $GO_FILES_CHANGED"
echo "  - Markdown files: $MD_FILES_CHANGED"
echo "  - Config files: $CONFIG_FILES_CHANGED"
echo
echo "Git remote origin updated to: https://github.com/developer-mesh/developer-mesh"
echo

# Step 9: Next steps
echo -e "${BLUE}=== Next Steps ===${NC}"
echo
echo "1. Run 'go mod tidy' in the root directory to update dependencies"
echo "2. Run 'make clean && make b' to rebuild the project"
echo "3. Run 'make test' to ensure all tests pass"
echo "4. Review any AWS resource names that may need updating"
echo "5. Commit the changes: git add -A && git commit -m 'refactor: rename project from developer-mesh to developer-mesh'"
echo "6. Push to the new repository: git push -u origin main"
echo
echo "Backup files created: *.bak (you can remove these after verifying the changes)"
echo

show_progress "Renaming complete!"