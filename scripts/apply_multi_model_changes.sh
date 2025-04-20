#!/bin/bash

# Apply Multi-Model Vector Support Changes
# This script applies all the changes needed to support multiple LLM models with different vector dimensions

# Set script to exit on error
set -e

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Check if we're in the right location
if [ ! -d "$PROJECT_ROOT/internal" ] || [ ! -d "$PROJECT_ROOT/cmd" ]; then
    echo "Error: Script must be run from the project root directory"
    exit 1
fi

# Create backup directory
BACKUP_DIR="$PROJECT_ROOT/backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

echo "Creating backups in $BACKUP_DIR..."

# Function to backup and replace a file
backup_and_replace() {
    local source_file=$1
    local target_file=$2
    
    # Create backup directory structure
    local backup_path="$BACKUP_DIR/$(dirname "${target_file#$PROJECT_ROOT/}")"
    mkdir -p "$backup_path"
    
    # Backup original file if it exists
    if [ -f "$target_file" ]; then
        cp "$target_file" "$backup_path/$(basename "$target_file")"
    fi
    
    # Replace with new file
    cp "$source_file" "$target_file"
    echo "Updated: $target_file"
}

# Apply database migrations
echo "Copying database migrations..."
backup_and_replace "$PROJECT_ROOT/migrations/sql/003_enhance_vector_model_support.up.sql" "$PROJECT_ROOT/migrations/sql/003_enhance_vector_model_support.up.sql"
backup_and_replace "$PROJECT_ROOT/migrations/sql/003_enhance_vector_model_support.down.sql" "$PROJECT_ROOT/migrations/sql/003_enhance_vector_model_support.down.sql"

# Apply core implementation changes
echo "Applying core implementation changes..."
backup_and_replace "$PROJECT_ROOT/internal/repository/embedding_repository.go" "$PROJECT_ROOT/internal/repository/embedding_repository.go"
backup_and_replace "$PROJECT_ROOT/internal/database/vector.go" "$PROJECT_ROOT/internal/database/vector.go"
backup_and_replace "$PROJECT_ROOT/internal/common/vector_utils.go" "$PROJECT_ROOT/internal/common/vector_utils.go"
backup_and_replace "$PROJECT_ROOT/internal/common/errors.go" "$PROJECT_ROOT/internal/common/errors.go"
backup_and_replace "$PROJECT_ROOT/internal/api/vector_handlers.go" "$PROJECT_ROOT/internal/api/vector_handlers.go"
backup_and_replace "$PROJECT_ROOT/internal/api/server_vector.go" "$PROJECT_ROOT/internal/api/server_vector.go"
backup_and_replace "$PROJECT_ROOT/internal/config/database_config.go" "$PROJECT_ROOT/internal/config/database_config.go"
backup_and_replace "$PROJECT_ROOT/internal/config/monitoring_config.go" "$PROJECT_ROOT/internal/config/monitoring_config.go"

# Apply server changes
echo "Updating server implementation..."
backup_and_replace "$PROJECT_ROOT/internal/api/server.go.new" "$PROJECT_ROOT/internal/api/server.go"
backup_and_replace "$PROJECT_ROOT/cmd/server/main.go.new" "$PROJECT_ROOT/cmd/server/main.go"

# Apply configuration changes
echo "Updating configuration..."
backup_and_replace "$PROJECT_ROOT/configs/config.yaml.template.new" "$PROJECT_ROOT/configs/config.yaml.template"

# Add test files
echo "Adding tests..."
backup_and_replace "$PROJECT_ROOT/internal/database/vector_test.go" "$PROJECT_ROOT/internal/database/vector_test.go"
backup_and_replace "$PROJECT_ROOT/test/integration/vector_multi_model_test.go" "$PROJECT_ROOT/test/integration/vector_multi_model_test.go"
backup_and_replace "$PROJECT_ROOT/scripts/test_vector_multi_model.go" "$PROJECT_ROOT/scripts/test_vector_multi_model.go"

# Add documentation
echo "Adding documentation..."
backup_and_replace "$PROJECT_ROOT/docs/features/multi-model-vector-support.md" "$PROJECT_ROOT/docs/features/multi-model-vector-support.md"

# Make the test script executable
chmod +x "$PROJECT_ROOT/scripts/test_vector_multi_model.go"

echo "All changes applied successfully."
echo "To run tests, use: go test ./..."
echo "To test with multiple models, use: go run scripts/test_vector_multi_model.go"
echo 
echo "Review the changes and ensure they're working as expected."
echo "A backup of the original files is available in $BACKUP_DIR"
