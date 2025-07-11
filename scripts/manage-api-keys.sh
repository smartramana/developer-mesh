#!/bin/bash

# manage-api-keys.sh - Admin CLI tool for managing API keys in DevOps MCP
# This script provides administrative functions for creating, listing, revoking,
# and managing API keys for the multi-tenant system.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
DEFAULT_API_URL="${API_BASE_URL:-http://localhost:8081}"
DEFAULT_DB_URL="${DATABASE_URL:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load environment variables if .env exists
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
fi

# Help function
show_help() {
    cat << EOF
DevOps MCP API Key Management Tool

Usage: $0 [command] [options]

Commands:
    create      Create a new API key
    list        List API keys
    show        Show details of a specific API key
    revoke      Revoke an API key
    rotate      Rotate an API key
    stats       Show API key usage statistics

Global Options:
    -h, --help          Show this help message
    -d, --database      Use direct database connection (requires DATABASE_URL)
    -u, --api-url URL   API endpoint URL (default: $DEFAULT_API_URL)
    -k, --api-key KEY   Admin API key for authentication

Create Options:
    -n, --name NAME           Name for the API key (required)
    -t, --tenant-id ID        Tenant ID (required)
    -T, --type TYPE           Key type: admin|gateway|agent|user (default: user)
    -s, --scopes SCOPES       Comma-separated list of scopes
    -e, --expires DURATION    Expiration duration (e.g., 30d, 1y)
    -S, --services SERVICES   Allowed services for gateway keys (comma-separated)
    -p, --parent-key ID       Parent key ID for hierarchical keys

List Options:
    -t, --tenant-id ID        Filter by tenant ID
    -T, --type TYPE           Filter by key type
    -a, --active              Show only active keys
    -i, --inactive            Show only inactive keys
    -l, --limit N             Limit number of results (default: 50)

Examples:
    # Create an admin key
    $0 create -n "Production Admin" -t tenant-123 -T admin

    # Create a gateway key with allowed services
    $0 create -n "GitHub Gateway" -t tenant-123 -T gateway -S github,gitlab

    # List all active keys for a tenant
    $0 list -t tenant-123 -a

    # Revoke a key
    $0 revoke key_abc123

    # Show key statistics
    $0 stats -t tenant-123

EOF
}

# Error handling
error() {
    echo -e "${RED}ERROR: $1${NC}" >&2
    exit 1
}

warning() {
    echo -e "${YELLOW}WARNING: $1${NC}" >&2
}

success() {
    echo -e "${GREEN}SUCCESS: $1${NC}"
}

info() {
    echo -e "${BLUE}INFO: $1${NC}"
}

# Check if required tools are installed
check_requirements() {
    local missing=()
    
    command -v psql >/dev/null 2>&1 || missing+=("psql")
    command -v jq >/dev/null 2>&1 || missing+=("jq")
    command -v curl >/dev/null 2>&1 || missing+=("curl")
    
    if [ ${#missing[@]} -ne 0 ]; then
        error "Missing required tools: ${missing[*]}\nPlease install them and try again."
    fi
}

# Generate a secure random API key
generate_api_key() {
    local type="${1:-user}"
    local prefix=""
    
    case "$type" in
        admin)   prefix="adm" ;;
        gateway) prefix="gw" ;;
        agent)   prefix="agt" ;;
        user)    prefix="usr" ;;
        *)       prefix="usr" ;;
    esac
    
    # Generate 32 bytes of random data and encode as base64url
    local random_part=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-43)
    echo "${prefix}_${random_part}"
}

# Create API key via database
create_key_db() {
    local name="$1"
    local tenant_id="$2"
    local key_type="${3:-user}"
    local scopes="${4:-}"
    local expires="${5:-}"
    local services="${6:-}"
    local parent_key="${7:-}"
    
    # Generate key
    local api_key=$(generate_api_key "$key_type")
    local key_hash=$(echo -n "$api_key" | sha256sum | cut -d' ' -f1)
    local key_prefix="${api_key:0:8}"
    
    # Convert scopes to PostgreSQL array format
    local scopes_array="NULL"
    if [ -n "$scopes" ]; then
        scopes_array="ARRAY[$(echo "$scopes" | sed "s/,/','/g" | sed "s/^/'/" | sed "s/$/'/")]"
    fi
    
    # Convert services to PostgreSQL array format
    local services_array="NULL"
    if [ -n "$services" ]; then
        services_array="ARRAY[$(echo "$services" | sed "s/,/','/g" | sed "s/^/'/" | sed "s/$/'/")]"
    fi
    
    # Calculate expiration
    local expires_at="NULL"
    if [ -n "$expires" ]; then
        # Parse duration (e.g., 30d, 1y)
        local number="${expires%[a-z]*}"
        local unit="${expires#*[0-9]}"
        
        case "$unit" in
            d) expires_at="NOW() + INTERVAL '$number days'" ;;
            m) expires_at="NOW() + INTERVAL '$number months'" ;;
            y) expires_at="NOW() + INTERVAL '$number years'" ;;
            *) error "Invalid expiration format. Use format like '30d', '6m', '1y'" ;;
        esac
    fi
    
    # Set rate limits based on key type
    local rate_limit=60
    case "$key_type" in
        admin)   rate_limit=1000 ;;
        gateway) rate_limit=500 ;;
        agent)   rate_limit=200 ;;
        user)    rate_limit=60 ;;
    esac
    
    # Create the key in database
    local query="
    INSERT INTO mcp.api_keys (
        id, key_hash, key_prefix, tenant_id, name, key_type,
        scopes, is_active, expires_at, rate_limit_requests,
        rate_limit_window_seconds, allowed_services, parent_key_id,
        created_at, updated_at
    ) VALUES (
        uuid_generate_v4(), '$key_hash', '$key_prefix', '$tenant_id', '$name', '$key_type',
        $scopes_array, true, $expires_at, $rate_limit, 60, $services_array,
        $([ -n "$parent_key" ] && echo "'$parent_key'" || echo "NULL"),
        NOW(), NOW()
    ) RETURNING id, key_prefix, created_at;"
    
    info "Creating API key..."
    
    local result=$(psql "$DATABASE_URL" -t -c "$query" 2>&1)
    if [ $? -eq 0 ]; then
        local key_id=$(echo "$result" | awk -F'|' '{print $1}' | xargs)
        success "API key created successfully!"
        echo
        echo "Key Details:"
        echo "  ID:        $key_id"
        echo "  Type:      $key_type"
        echo "  Prefix:    $key_prefix"
        echo "  API Key:   $api_key"
        echo
        echo -e "${YELLOW}IMPORTANT: Save this API key securely. It cannot be retrieved again.${NC}"
    else
        error "Failed to create API key: $result"
    fi
}

# Create API key via API
create_key_api() {
    local name="$1"
    local tenant_id="$2"
    local key_type="${3:-user}"
    local scopes="${4:-}"
    local expires="${5:-}"
    local services="${6:-}"
    local parent_key="${7:-}"
    local api_url="$8"
    local admin_key="$9"
    
    # Build JSON payload
    local payload=$(jq -n \
        --arg name "$name" \
        --arg tenant_id "$tenant_id" \
        --arg key_type "$key_type" \
        --argjson scopes "$([ -n "$scopes" ] && echo "[\"$(echo $scopes | sed 's/,/","/g')\"]" || echo "null")" \
        --argjson services "$([ -n "$services" ] && echo "[\"$(echo $services | sed 's/,/","/g')\"]" || echo "null")" \
        --arg parent_key "$parent_key" \
        --arg expires "$expires" \
        '{
            name: $name,
            tenant_id: $tenant_id,
            key_type: $key_type,
            scopes: $scopes,
            allowed_services: $services,
            parent_key_id: (if $parent_key != "" then $parent_key else null end),
            expires_at: (if $expires != "" then $expires else null end)
        }')
    
    info "Creating API key via API..."
    
    local response=$(curl -s -X POST \
        -H "Authorization: Bearer $admin_key" \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$api_url/v1/api-keys")
    
    local status=$(echo "$response" | jq -r '.status // "error"')
    if [ "$status" = "success" ]; then
        success "API key created successfully!"
        echo
        echo "$response" | jq -r '.data | "Key Details:\n  ID:        \(.id)\n  Type:      \(.key_type)\n  Prefix:    \(.key_prefix)\n  API Key:   \(.key)"'
        echo
        echo -e "${YELLOW}IMPORTANT: Save this API key securely. It cannot be retrieved again.${NC}"
    else
        local error_msg=$(echo "$response" | jq -r '.error // "Unknown error"')
        error "Failed to create API key: $error_msg"
    fi
}

# List API keys
list_keys() {
    local tenant_id="${1:-}"
    local key_type="${2:-}"
    local active_filter="${3:-}"
    local limit="${4:-50}"
    local use_db="${5:-false}"
    
    if [ "$use_db" = "true" ]; then
        list_keys_db "$tenant_id" "$key_type" "$active_filter" "$limit"
    else
        list_keys_api "$tenant_id" "$key_type" "$active_filter" "$limit" "$6" "$7"
    fi
}

# List API keys via database
list_keys_db() {
    local tenant_id="$1"
    local key_type="$2"
    local active_filter="$3"
    local limit="$4"
    
    local where_clauses=()
    [ -n "$tenant_id" ] && where_clauses+=("tenant_id = '$tenant_id'")
    [ -n "$key_type" ] && where_clauses+=("key_type = '$key_type'")
    [ "$active_filter" = "active" ] && where_clauses+=("is_active = true")
    [ "$active_filter" = "inactive" ] && where_clauses+=("is_active = false")
    
    local where_clause=""
    if [ ${#where_clauses[@]} -gt 0 ]; then
        where_clause="WHERE $(IFS=' AND '; echo "${where_clauses[*]}")"
    fi
    
    local query="
    SELECT 
        key_prefix,
        name,
        key_type,
        tenant_id,
        is_active,
        created_at,
        last_used_at,
        expires_at
    FROM mcp.api_keys
    $where_clause
    ORDER BY created_at DESC
    LIMIT $limit;"
    
    info "Listing API keys..."
    echo
    
    psql "$DATABASE_URL" -c "$query"
}

# List API keys via API
list_keys_api() {
    local tenant_id="$1"
    local key_type="$2"
    local active_filter="$3"
    local limit="$4"
    local api_url="$5"
    local admin_key="$6"
    
    local params=()
    [ -n "$tenant_id" ] && params+=("tenant_id=$tenant_id")
    [ -n "$key_type" ] && params+=("key_type=$key_type")
    [ -n "$active_filter" ] && params+=("status=$active_filter")
    params+=("limit=$limit")
    
    local query_string=$(IFS='&'; echo "${params[*]}")
    
    info "Listing API keys via API..."
    
    local response=$(curl -s -X GET \
        -H "Authorization: Bearer $admin_key" \
        "$api_url/v1/api-keys?$query_string")
    
    local status=$(echo "$response" | jq -r '.status // "error"')
    if [ "$status" = "success" ]; then
        echo
        echo "$response" | jq -r '.data[] | "\(.key_prefix)\t\(.name)\t\(.key_type)\t\(.tenant_id)\t\(.is_active)\t\(.created_at)"' | \
            awk 'BEGIN {print "PREFIX\tNAME\tTYPE\tTENANT\tACTIVE\tCREATED"} {print}'
    else
        local error_msg=$(echo "$response" | jq -r '.error // "Unknown error"')
        error "Failed to list API keys: $error_msg"
    fi
}

# Show API key details
show_key() {
    local key_identifier="$1"
    local use_db="${2:-false}"
    
    if [ "$use_db" = "true" ]; then
        show_key_db "$key_identifier"
    else
        show_key_api "$key_identifier" "$3" "$4"
    fi
}

# Show API key details via database
show_key_db() {
    local key_identifier="$1"
    
    local query="
    SELECT 
        id,
        key_prefix,
        name,
        key_type,
        tenant_id,
        user_id,
        is_active,
        scopes,
        allowed_services,
        parent_key_id,
        rate_limit_requests,
        rate_limit_window_seconds,
        created_at,
        updated_at,
        last_used_at,
        expires_at
    FROM mcp.api_keys
    WHERE key_prefix = '$key_identifier' OR id = '$key_identifier'
    LIMIT 1;"
    
    info "Showing API key details..."
    echo
    
    psql "$DATABASE_URL" -x -c "$query"
}

# Revoke API key
revoke_key() {
    local key_identifier="$1"
    local use_db="${2:-false}"
    
    if [ "$use_db" = "true" ]; then
        revoke_key_db "$key_identifier"
    else
        revoke_key_api "$key_identifier" "$3" "$4"
    fi
}

# Revoke API key via database
revoke_key_db() {
    local key_identifier="$1"
    
    local query="
    UPDATE mcp.api_keys
    SET is_active = false, updated_at = NOW()
    WHERE (key_prefix = '$key_identifier' OR id = '$key_identifier')
    AND is_active = true
    RETURNING key_prefix, name;"
    
    info "Revoking API key..."
    
    local result=$(psql "$DATABASE_URL" -t -c "$query" 2>&1)
    if [ $? -eq 0 ]; then
        if [ -z "$(echo "$result" | xargs)" ]; then
            warning "No active API key found with identifier: $key_identifier"
        else
            success "API key revoked successfully!"
            echo "$result"
        fi
    else
        error "Failed to revoke API key: $result"
    fi
}

# Show API key statistics
show_stats() {
    local tenant_id="${1:-}"
    local use_db="${2:-false}"
    
    if [ "$use_db" = "true" ]; then
        show_stats_db "$tenant_id"
    else
        show_stats_api "$tenant_id" "$3" "$4"
    fi
}

# Show API key statistics via database
show_stats_db() {
    local tenant_id="$1"
    
    local tenant_filter=""
    if [ -n "$tenant_id" ]; then
        tenant_filter="WHERE tenant_id = '$tenant_id'"
    fi
    
    local query="
    WITH key_stats AS (
        SELECT 
            key_type,
            COUNT(*) as total_keys,
            COUNT(*) FILTER (WHERE is_active = true) as active_keys,
            COUNT(*) FILTER (WHERE is_active = false) as inactive_keys,
            COUNT(*) FILTER (WHERE expires_at < NOW()) as expired_keys,
            COUNT(*) FILTER (WHERE last_used_at > NOW() - INTERVAL '7 days') as recently_used
        FROM mcp.api_keys
        $tenant_filter
        GROUP BY key_type
    )
    SELECT 
        key_type,
        total_keys,
        active_keys,
        inactive_keys,
        expired_keys,
        recently_used
    FROM key_stats
    ORDER BY key_type;"
    
    info "API Key Statistics"
    if [ -n "$tenant_id" ]; then
        echo "Tenant: $tenant_id"
    fi
    echo
    
    psql "$DATABASE_URL" -c "$query"
    
    # Additional summary
    local summary_query="
    SELECT 
        COUNT(*) as total_keys,
        COUNT(*) FILTER (WHERE is_active = true) as active_keys,
        COUNT(DISTINCT tenant_id) as total_tenants
    FROM mcp.api_keys
    $tenant_filter;"
    
    echo
    echo "Summary:"
    psql "$DATABASE_URL" -t -c "$summary_query" | \
        awk '{print "  Total Keys: " $1 "\n  Active Keys: " $3 "\n  Total Tenants: " $5}'
}

# Main command processing
main() {
    # Check requirements
    check_requirements
    
    # Parse global options
    local use_db=false
    local api_url="$DEFAULT_API_URL"
    local admin_key=""
    
    # No command provided
    if [ $# -eq 0 ]; then
        show_help
        exit 0
    fi
    
    # Get command
    local command="$1"
    shift
    
    # Parse options based on command
    case "$command" in
        create)
            local name=""
            local tenant_id=""
            local key_type="user"
            local scopes=""
            local expires=""
            local services=""
            local parent_key=""
            
            while [[ $# -gt 0 ]]; do
                case "$1" in
                    -n|--name) name="$2"; shift 2 ;;
                    -t|--tenant-id) tenant_id="$2"; shift 2 ;;
                    -T|--type) key_type="$2"; shift 2 ;;
                    -s|--scopes) scopes="$2"; shift 2 ;;
                    -e|--expires) expires="$2"; shift 2 ;;
                    -S|--services) services="$2"; shift 2 ;;
                    -p|--parent-key) parent_key="$2"; shift 2 ;;
                    -d|--database) use_db=true; shift ;;
                    -u|--api-url) api_url="$2"; shift 2 ;;
                    -k|--api-key) admin_key="$2"; shift 2 ;;
                    -h|--help) show_help; exit 0 ;;
                    *) error "Unknown option: $1" ;;
                esac
            done
            
            # Validate required fields
            [ -z "$name" ] && error "Name is required (-n/--name)"
            [ -z "$tenant_id" ] && error "Tenant ID is required (-t/--tenant-id)"
            
            # Validate key type
            case "$key_type" in
                admin|gateway|agent|user) ;;
                *) error "Invalid key type: $key_type (must be admin, gateway, agent, or user)" ;;
            esac
            
            # Check database or API mode
            if [ "$use_db" = "true" ]; then
                [ -z "$DATABASE_URL" ] && error "DATABASE_URL environment variable is required for database mode"
                create_key_db "$name" "$tenant_id" "$key_type" "$scopes" "$expires" "$services" "$parent_key"
            else
                [ -z "$admin_key" ] && error "Admin API key is required for API mode (-k/--api-key)"
                create_key_api "$name" "$tenant_id" "$key_type" "$scopes" "$expires" "$services" "$parent_key" "$api_url" "$admin_key"
            fi
            ;;
            
        list)
            local tenant_id=""
            local key_type=""
            local active_filter=""
            local limit="50"
            
            while [[ $# -gt 0 ]]; do
                case "$1" in
                    -t|--tenant-id) tenant_id="$2"; shift 2 ;;
                    -T|--type) key_type="$2"; shift 2 ;;
                    -a|--active) active_filter="active"; shift ;;
                    -i|--inactive) active_filter="inactive"; shift ;;
                    -l|--limit) limit="$2"; shift 2 ;;
                    -d|--database) use_db=true; shift ;;
                    -u|--api-url) api_url="$2"; shift 2 ;;
                    -k|--api-key) admin_key="$2"; shift 2 ;;
                    -h|--help) show_help; exit 0 ;;
                    *) error "Unknown option: $1" ;;
                esac
            done
            
            if [ "$use_db" = "true" ]; then
                [ -z "$DATABASE_URL" ] && error "DATABASE_URL environment variable is required for database mode"
                list_keys_db "$tenant_id" "$key_type" "$active_filter" "$limit"
            else
                [ -z "$admin_key" ] && error "Admin API key is required for API mode (-k/--api-key)"
                list_keys_api "$tenant_id" "$key_type" "$active_filter" "$limit" "$api_url" "$admin_key"
            fi
            ;;
            
        show)
            [ $# -eq 0 ] && error "Key identifier is required"
            local key_identifier="$1"
            shift
            
            while [[ $# -gt 0 ]]; do
                case "$1" in
                    -d|--database) use_db=true; shift ;;
                    -u|--api-url) api_url="$2"; shift 2 ;;
                    -k|--api-key) admin_key="$2"; shift 2 ;;
                    -h|--help) show_help; exit 0 ;;
                    *) error "Unknown option: $1" ;;
                esac
            done
            
            if [ "$use_db" = "true" ]; then
                [ -z "$DATABASE_URL" ] && error "DATABASE_URL environment variable is required for database mode"
                show_key_db "$key_identifier"
            else
                [ -z "$admin_key" ] && error "Admin API key is required for API mode (-k/--api-key)"
                show_key_api "$key_identifier" "$api_url" "$admin_key"
            fi
            ;;
            
        revoke)
            [ $# -eq 0 ] && error "Key identifier is required"
            local key_identifier="$1"
            shift
            
            while [[ $# -gt 0 ]]; do
                case "$1" in
                    -d|--database) use_db=true; shift ;;
                    -u|--api-url) api_url="$2"; shift 2 ;;
                    -k|--api-key) admin_key="$2"; shift 2 ;;
                    -h|--help) show_help; exit 0 ;;
                    *) error "Unknown option: $1" ;;
                esac
            done
            
            if [ "$use_db" = "true" ]; then
                [ -z "$DATABASE_URL" ] && error "DATABASE_URL environment variable is required for database mode"
                revoke_key_db "$key_identifier"
            else
                [ -z "$admin_key" ] && error "Admin API key is required for API mode (-k/--api-key)"
                revoke_key_api "$key_identifier" "$api_url" "$admin_key"
            fi
            ;;
            
        stats)
            local tenant_id=""
            
            while [[ $# -gt 0 ]]; do
                case "$1" in
                    -t|--tenant-id) tenant_id="$2"; shift 2 ;;
                    -d|--database) use_db=true; shift ;;
                    -u|--api-url) api_url="$2"; shift 2 ;;
                    -k|--api-key) admin_key="$2"; shift 2 ;;
                    -h|--help) show_help; exit 0 ;;
                    *) error "Unknown option: $1" ;;
                esac
            done
            
            if [ "$use_db" = "true" ]; then
                [ -z "$DATABASE_URL" ] && error "DATABASE_URL environment variable is required for database mode"
                show_stats_db "$tenant_id"
            else
                [ -z "$admin_key" ] && error "Admin API key is required for API mode (-k/--api-key)"
                show_stats_api "$tenant_id" "$api_url" "$admin_key"
            fi
            ;;
            
        -h|--help|help)
            show_help
            exit 0
            ;;
            
        *)
            error "Unknown command: $command\nRun '$0 --help' for usage information."
            ;;
    esac
}

# Run main function
main "$@"