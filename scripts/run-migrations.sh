#!/bin/bash
# Script to run database migrations for multi-agent collaboration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
DB_HOST=${DATABASE_HOST:-localhost}
DB_PORT=${DATABASE_PORT:-5432}
DB_NAME=${DATABASE_NAME:-dev}
DB_USER=${DATABASE_USER:-dev}
DB_PASSWORD=${DATABASE_PASSWORD:-dev}
MIGRATIONS_DIR="migrations"

# Usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo "Options:"
    echo "  -h, --host       Database host (default: localhost)"
    echo "  -p, --port       Database port (default: 5432)"
    echo "  -d, --database   Database name (default: dev)"
    echo "  -u, --user       Database user (default: dev)"
    echo "  -w, --password   Database password (default: dev)"
    echo "  --rollback       Run rollback instead of migrations"
    echo "  --dry-run        Show what would be executed without running"
    exit 1
}

# Parse arguments
ROLLBACK=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--host)
            DB_HOST="$2"
            shift 2
            ;;
        -p|--port)
            DB_PORT="$2"
            shift 2
            ;;
        -d|--database)
            DB_NAME="$2"
            shift 2
            ;;
        -u|--user)
            DB_USER="$2"
            shift 2
            ;;
        -w|--password)
            DB_PASSWORD="$2"
            shift 2
            ;;
        --rollback)
            ROLLBACK=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            usage
            ;;
    esac
done

# Export for psql
export PGPASSWORD=$DB_PASSWORD

# Connection string
PSQL_CMD="psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME"

echo -e "${GREEN}Database Migration Runner${NC}"
echo "========================="
echo "Host: $DB_HOST:$DB_PORT"
echo "Database: $DB_NAME"
echo "User: $DB_USER"
echo ""

# Test connection
echo -e "${YELLOW}Testing database connection...${NC}"
if ! $PSQL_CMD -c "SELECT 1" > /dev/null 2>&1; then
    echo -e "${RED}Failed to connect to database!${NC}"
    exit 1
fi
echo -e "${GREEN}Connection successful!${NC}"
echo ""

# Create migration tracking table if it doesn't exist
if [ "$DRY_RUN" = false ]; then
    $PSQL_CMD <<-EOSQL
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version VARCHAR(255) PRIMARY KEY,
            applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
        );
EOSQL
fi

# Run migrations or rollback
if [ "$ROLLBACK" = true ]; then
    echo -e "${YELLOW}Running rollback...${NC}"
    if [ "$DRY_RUN" = true ]; then
        echo "Would execute: migrations/rollback/rollback_all.sql"
    else
        $PSQL_CMD -f "$MIGRATIONS_DIR/rollback/rollback_all.sql"
        echo -e "${GREEN}Rollback completed!${NC}"
    fi
else
    # Get list of migration files
    MIGRATION_FILES=$(ls $MIGRATIONS_DIR/*.sql 2>/dev/null | grep -v rollback | sort)
    
    if [ -z "$MIGRATION_FILES" ]; then
        echo -e "${YELLOW}No migration files found in $MIGRATIONS_DIR${NC}"
        exit 0
    fi
    
    echo -e "${YELLOW}Running migrations...${NC}"
    echo ""
    
    for migration in $MIGRATION_FILES; do
        filename=$(basename "$migration")
        version="${filename%.*}"
        
        # Check if migration has already been applied
        if [ "$DRY_RUN" = false ]; then
            applied=$($PSQL_CMD -t -c "SELECT COUNT(*) FROM schema_migrations WHERE version = '$version'" | tr -d ' ')
            if [ "$applied" -gt 0 ]; then
                echo -e "${YELLOW}Skipping $filename (already applied)${NC}"
                continue
            fi
        fi
        
        echo -e "${GREEN}Applying $filename...${NC}"
        
        if [ "$DRY_RUN" = true ]; then
            echo "Would execute: $migration"
        else
            # Run migration
            if $PSQL_CMD -f "$migration"; then
                # Record successful migration
                $PSQL_CMD -c "INSERT INTO schema_migrations (version) VALUES ('$version')"
                echo -e "${GREEN}✓ $filename applied successfully${NC}"
            else
                echo -e "${RED}✗ Failed to apply $filename${NC}"
                exit 1
            fi
        fi
        echo ""
    done
    
    echo -e "${GREEN}All migrations completed successfully!${NC}"
fi

# Show current schema version
if [ "$DRY_RUN" = false ] && [ "$ROLLBACK" = false ]; then
    echo ""
    echo -e "${YELLOW}Current schema versions:${NC}"
    $PSQL_CMD -c "SELECT version, applied_at FROM schema_migrations ORDER BY applied_at DESC LIMIT 5"
fi