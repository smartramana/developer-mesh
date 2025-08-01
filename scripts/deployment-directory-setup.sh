#!/bin/bash

# Script to ensure proper directory setup during deployment
# This should be run at the beginning of each deployment to prevent symlink issues

set -e

echo "🔧 Setting up deployment directory structure..."

# Function to safely create directory
safe_create_dir() {
    local dir=$1
    
    # If it's a symlink, remove it
    if [ -L "$dir" ]; then
        echo "Removing symlink: $dir"
        rm -f "$dir"
    fi
    
    # If it's a file, back it up and remove
    if [ -f "$dir" ]; then
        echo "Backing up file: $dir"
        mv "$dir" "${dir}.backup.$(date +%Y%m%d_%H%M%S)"
    fi
    
    # Create directory if it doesn't exist
    if [ ! -d "$dir" ]; then
        echo "Creating directory: $dir"
        mkdir -p "$dir"
    fi
}

# Main deployment directory
cd /home/ec2-user/developer-mesh || exit 1

# Ensure configs is a real directory, not a symlink
safe_create_dir "configs"
safe_create_dir "logs"
safe_create_dir "nginx"

# Remove symlinked docker-compose files
for file in docker-compose.yml docker-compose.production.yml; do
    if [ -L "$file" ]; then
        echo "Removing symlink: $file"
        rm -f "$file"
    fi
done

echo "✅ Directory structure prepared for deployment"
ls -la