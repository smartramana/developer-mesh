#!/bin/bash
set -euo pipefail

# Script to set up E2E test API key in production database
# Usage: ./setup_e2e_api_key.sh

# Check if E2E_API_KEY is set
if [ -z "${E2E_API_KEY:-}" ]; then
    echo "Error: E2E_API_KEY environment variable is not set"
    echo "Please set it with: export E2E_API_KEY=your_api_key"
    exit 1
fi

# Check if DATABASE_URL is set
if [ -z "${DATABASE_URL:-}" ]; then
    echo "Error: DATABASE_URL environment variable is not set"
    exit 1
fi

# Extract key prefix (first 8 characters)
KEY_PREFIX="${E2E_API_KEY:0:8}"

# Generate SHA256 hash of the API key
KEY_HASH=$(echo -n "$E2E_API_KEY" | sha256sum | cut -d' ' -f1)

echo "Setting up E2E test API key..."
echo "Key prefix: $KEY_PREFIX"

# Create SQL command with proper escaping
SQL_COMMAND=$(cat <<EOF
BEGIN;

-- Ensure tenants table exists and has the E2E test tenant
INSERT INTO mcp.tenants (id, name, plan, metadata)
VALUES (
    'e2e-test-tenant'::uuid,
    'E2E Test Tenant',
    'premium',
    '{"purpose": "e2e-testing"}'::jsonb
) ON CONFLICT (id) DO NOTHING;

-- Insert or update E2E test API key
INSERT INTO mcp.api_keys (
    key_hash,
    key_prefix,
    tenant_id,
    user_id,
    name,
    key_type,
    scopes,
    is_active,
    rate_limit_requests,
    rate_limit_window_seconds,
    metadata
) VALUES (
    '$KEY_HASH',
    '$KEY_PREFIX',
    'e2e-test-tenant'::uuid,
    'e2e-test-user'::uuid,
    'E2E Test Admin Key',
    'admin',
    ARRAY['read', 'write', 'admin'],
    true,
    10000,  -- High rate limit for testing
    60,
    '{"purpose": "e2e-testing", "environment": "production"}'::jsonb
) ON CONFLICT (key_hash) DO UPDATE SET
    key_type = EXCLUDED.key_type,
    scopes = EXCLUDED.scopes,
    is_active = EXCLUDED.is_active,
    rate_limit_requests = EXCLUDED.rate_limit_requests,
    metadata = EXCLUDED.metadata,
    updated_at = CURRENT_TIMESTAMP;

COMMIT;
EOF
)

# Execute the SQL
psql "$DATABASE_URL" -c "$SQL_COMMAND"

# Verify the key was inserted
VERIFY_SQL="SELECT COUNT(*) FROM mcp.api_keys WHERE key_prefix = '$KEY_PREFIX' AND is_active = true;"
COUNT=$(psql "$DATABASE_URL" -t -c "$VERIFY_SQL" | tr -d ' ')

if [ "$COUNT" -eq "0" ]; then
    echo "Error: E2E API key was not inserted successfully"
    exit 1
fi

echo "E2E API key setup completed successfully"
echo "Active E2E keys with prefix $KEY_PREFIX: $COUNT"