#!/bin/bash
# Quick Migration Script for Moving Internal Packages to Pkg
set -e

REPO_ROOT="/Users/seancorkum/projects/devops-mcp"
INTERNAL_DIR="${REPO_ROOT}/internal"
PKG_DIR="${REPO_ROOT}/pkg"

# Log function
log() {
  echo "[$(date +%H:%M:%S)] $1"
}

# Create directory if it doesn't exist
ensure_dir() {
  if [ ! -d "$1" ]; then
    mkdir -p "$1"
    log "Created directory: $1"
  fi
}

# Migrate a single package
migrate_package() {
  local pkg=$1
  local src="${INTERNAL_DIR}/${pkg}"
  local dst="${PKG_DIR}/${pkg}"
  
  if [ -d "$src" ]; then
    log "Migrating ${pkg}..."
    ensure_dir "$dst"
    
    # Copy all files (this won't overwrite newer files in destination)
    cp -r -n "$src"/* "$dst"/ 2>/dev/null || true
    
    log "✅ Migrated: ${pkg}"
  else
    log "⚠️ Package not found: ${pkg}"
  fi
}

# Main migration function
main() {
  log "Starting quick migration from internal to pkg..."
  
  # Packages to migrate (add more as needed)
  packages=(
    "aws"
    "cache" 
    "chunking"
    "common"
    "config"
    "core"
    "interfaces"
    "metrics"
    "queue"
    "relationship"
    "resilience"
    "safety"
    "worker"
    # Continue migration for packages already in progress
    "adapters"
    "api"
    "events"
    "repository"
  )
  
  # Migrate each package
  for pkg in "${packages[@]}"; do
    migrate_package "$pkg"
  done
  
  log "Migration complete. Please run the import migrator next to update import paths."
  log "Command: go run scripts/import_migrator.go -dir ./pkg -mapping all -dry-run=false"
}

# Run the main function
main
