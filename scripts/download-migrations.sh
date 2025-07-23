#!/bin/bash
# Download migrations from GitHub

set -e

MIGRATIONS_DIR="migrations"
GITHUB_REPO="S-Corkum/developer-mesh"
GITHUB_BRANCH="${GITHUB_BRANCH:-main}"

echo "Downloading migrations from GitHub..."

# Remove old migrations directory
rm -rf "$MIGRATIONS_DIR"
mkdir -p "$MIGRATIONS_DIR"

# Use git to clone just the migrations directory
cd "$MIGRATIONS_DIR"
git init
git remote add origin "https://github.com/$GITHUB_REPO.git"
git config core.sparseCheckout true
echo "apps/rest-api/migrations/*" > .git/info/sparse-checkout
git pull origin "$GITHUB_BRANCH"

# Move migrations to the expected location
if [ -d "apps/rest-api/migrations/sql" ]; then
    mv apps/rest-api/migrations/sql .
    rm -rf apps
    echo "Migrations downloaded successfully to $PWD/sql"
else
    echo "ERROR: Migrations directory not found"
    exit 1
fi

# List downloaded migrations
echo "Downloaded migrations:"
ls -la sql/