#!/bin/bash

# Script to insert API key into production database
# This script should be run from the production EC2 instance

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Developer Mesh - Production API Key Insertion${NC}"
echo "=========================================="

# Parse command line arguments
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -k, --key        Full API key (required)"
    echo "  -h, --hash       Key hash (required)"
    echo "  -p, --prefix     Key prefix (required)"
    echo "  -t, --type       Key type: admin|gateway|agent|user (required)"
    echo "  -n, --name       Key name (required)"
    echo "  -T, --tenant     Tenant ID (default: e2e-test-tenant)"
    echo "  -s, --scopes     Comma-separated scopes (default: read,write,admin)"
    echo "  -r, --rate       Rate limit requests per minute (default: 1000)"
    echo "  --help           Show this help message"
    echo ""
    echo "Example:"
    echo "  $0 -k 'adm_...' -h '93ed99...' -p 'adm_dHg8' -t admin -n 'E2E Test Key'"
    exit 1
}

# Default values
TENANT_ID="e2e-test-tenant"
SCOPES="read,write,admin"
RATE_LIMIT=1000
API_KEY=""
KEY_HASH=""
KEY_PREFIX=""
KEY_TYPE=""
KEY_NAME=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -k|--key)
            API_KEY="$2"
            shift 2
            ;;
        -h|--hash)
            KEY_HASH="$2"
            shift 2
            ;;
        -p|--prefix)
            KEY_PREFIX="$2"
            shift 2
            ;;
        -t|--type)
            KEY_TYPE="$2"
            shift 2
            ;;
        -n|--name)
            KEY_NAME="$2"
            shift 2
            ;;
        -T|--tenant)
            TENANT_ID="$2"
            shift 2
            ;;
        -s|--scopes)
            SCOPES="$2"
            shift 2
            ;;
        -r|--rate)
            RATE_LIMIT="$2"
            shift 2
            ;;
        --help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate required arguments
if [ -z "$API_KEY" ] || [ -z "$KEY_HASH" ] || [ -z "$KEY_PREFIX" ] || [ -z "$KEY_TYPE" ] || [ -z "$KEY_NAME" ]; then
    echo -e "${RED}ERROR: Missing required arguments${NC}"
    usage
fi

# Validate key type
if [[ ! "$KEY_TYPE" =~ ^(admin|gateway|agent|user)$ ]]; then
    echo -e "${RED}ERROR: Invalid key type: $KEY_TYPE${NC}"
    echo "Valid types: admin, gateway, agent, user"
    exit 1
fi

# Database connection parameters (should be set as environment variables)
DB_HOST="${DB_HOST:-}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-}"
DB_USER="${DB_USER:-}"
DB_PASS="${DB_PASS:-}"

# Check if required environment variables are set
if [ -z "$DB_HOST" ] || [ -z "$DB_NAME" ] || [ -z "$DB_USER" ]; then
    echo -e "${RED}ERROR: Database connection parameters not set${NC}"
    echo "Please set the following environment variables:"
    echo "  export DB_HOST=your-rds-endpoint"
    echo "  export DB_NAME=your-database-name"
    echo "  export DB_USER=your-database-user"
    echo "  export DB_PASS=your-database-password"
    exit 1
fi

# Convert comma-separated scopes to PostgreSQL array format
IFS=',' read -ra SCOPE_ARRAY <<< "$SCOPES"
PG_SCOPES="ARRAY["
for i in "${!SCOPE_ARRAY[@]}"; do
    if [ $i -gt 0 ]; then
        PG_SCOPES+=", "
    fi
    PG_SCOPES+="'${SCOPE_ARRAY[$i]}'"
done
PG_SCOPES+="]"

# SQL statement to insert the API key
SQL_STATEMENT="INSERT INTO mcp.api_keys (
    id, key_hash, key_prefix, tenant_id, user_id, name, key_type,
    scopes, is_active, rate_limit_requests, rate_limit_window_seconds,
    created_at, updated_at
) VALUES (
    uuid_generate_v4(), 
    '${KEY_HASH}', 
    '${KEY_PREFIX}', 
    '${TENANT_ID}', 
    NULL, 
    '${KEY_NAME}', 
    '${KEY_TYPE}',
    ${PG_SCOPES}, 
    true, 
    ${RATE_LIMIT}, 
    60,
    CURRENT_TIMESTAMP, 
    CURRENT_TIMESTAMP
) ON CONFLICT (key_hash) DO NOTHING
RETURNING id, key_prefix, name;"

echo -e "${YELLOW}API Key Details:${NC}"
echo "  Type:   $KEY_TYPE"
echo "  Prefix: $KEY_PREFIX"
echo "  Tenant: $TENANT_ID"
echo "  Name:   $KEY_NAME"
echo "  Scopes: $SCOPES"
echo "  Rate:   $RATE_LIMIT req/min"
echo ""

echo -e "${YELLOW}Database Connection:${NC}"
echo "  Host: $DB_HOST"
echo "  Port: $DB_PORT"
echo "  Database: $DB_NAME"
echo "  User: $DB_USER"
echo ""

# Confirm before proceeding
read -p "Do you want to insert this API key into the production database? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Operation cancelled."
    exit 0
fi

# Execute the SQL statement
echo -e "${YELLOW}Inserting API key into database...${NC}"

export PGPASSWORD="$DB_PASS"
RESULT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "$SQL_STATEMENT" 2>&1)
PSQL_EXIT_CODE=$?
unset PGPASSWORD

if [ $PSQL_EXIT_CODE -eq 0 ]; then
    if [ -n "$RESULT" ]; then
        echo -e "${GREEN}✓ API key inserted successfully!${NC}"
        echo "Result: $RESULT"
        echo ""
        echo -e "${GREEN}The API key has been created:${NC}"
        echo -e "${YELLOW}$API_KEY${NC}"
        echo ""
        echo "Next steps:"
        echo "1. Store this API key securely"
        echo "2. Update any GitHub secrets or environment variables"
        echo "3. Test authentication with the new key"
    else
        echo -e "${YELLOW}API key already exists (no rows returned)${NC}"
    fi
else
    echo -e "${RED}✗ Failed to insert API key${NC}"
    echo "Error: $RESULT"
    exit 1
fi

# Verify the key was inserted
echo -e "${YELLOW}Verifying API key in database...${NC}"
export PGPASSWORD="$DB_PASS"
VERIFY_RESULT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
SELECT key_prefix, name, key_type, is_active, tenant_id 
FROM mcp.api_keys 
WHERE key_prefix = '${KEY_PREFIX}';" 2>&1)
unset PGPASSWORD

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ API key verified in database:${NC}"
    echo "$VERIFY_RESULT"
else
    echo -e "${RED}✗ Failed to verify API key${NC}"
    echo "Error: $VERIFY_RESULT"
fi