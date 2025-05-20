#!/bin/bash
# Script to update import paths from internal to pkg
set -e

REPO_ROOT="/Users/seancorkum/projects/devops-mcp"
cd "$REPO_ROOT"

log() {
  echo "[$(date +%H:%M:%S)] $1"
}

log "Starting import path updates..."

# Update all Go files to use pkg instead of internal in imports
find . -name "*.go" -not -path "*/vendor/*" -not -path "*/\.*" | while read -r file; do
  # Skip test files for now to speed up the process
  if [[ "$file" == *_test.go ]]; then
    continue
  fi
  
  # Replace internal imports with pkg imports, keeping the rest of the path the same
  sed -i '' 's|"github.com/S-Corkum/devops-mcp/internal/|"github.com/S-Corkum/devops-mcp/pkg/|g' "$file"
  
  # Specially handle the renamed packages like utils -> util
  sed -i '' 's|"github.com/S-Corkum/devops-mcp/pkg/utils"|"github.com/S-Corkum/devops-mcp/pkg/util"|g' "$file"
  
  # Handle the special cases where pkg paths have been updated (per migration tracker)
  sed -i '' 's|"github.com/S-Corkum/devops-mcp/pkg/aws"|"github.com/S-Corkum/devops-mcp/pkg/common/aws"|g' "$file"
  sed -i '' 's|"github.com/S-Corkum/devops-mcp/pkg/relationship"|"github.com/S-Corkum/devops-mcp/pkg/models/relationship"|g' "$file"
  sed -i '' 's|"github.com/S-Corkum/devops-mcp/pkg/common/config"|"github.com/S-Corkum/devops-mcp/pkg/config"|g' "$file"
  sed -i '' 's|"github.com/S-Corkum/devops-mcp/pkg/metrics"|"github.com/S-Corkum/devops-mcp/pkg/observability"|g' "$file"
  sed -i '' 's|"github.com/S-Corkum/devops-mcp/pkg/common/metrics"|"github.com/S-Corkum/devops-mcp/pkg/observability"|g' "$file"
done

log "✅ Import paths updated in all Go files"
log "Note: You will likely need to debug compilation errors next - run 'go build ./...' to see errors"
