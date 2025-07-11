# Production API Key Setup Required

## Current Situation

The e2e tests are failing with 401 Unauthorized errors because:

1. Production is configured to use API keys from the database only (no static keys)
2. The GitHub secret `ADMIN_API_KEY` is set in the environment but not in the database
3. E2e tests need a valid API key in the production database

## Action Required

To fix the e2e tests, we need to insert an API key into the production database.

### Option 1: Use the Generated Key

A new admin API key has been generated for e2e testing:

```
API Key: adm_dHg84VsfXagE4sd9jY-F5ePfOrgZnLXaBfRpxb_TW-0=
```

This key needs to be:
1. Inserted into the production database using the SQL below
2. Updated in GitHub secrets as `E2E_API_KEY`

```sql
INSERT INTO mcp.api_keys (
    id, key_hash, key_prefix, tenant_id, user_id, name, key_type,
    scopes, is_active, rate_limit_requests, rate_limit_window_seconds,
    created_at, updated_at
) VALUES (
    uuid_generate_v4(), 
    '93ed99b44777208480257031b1983d69135d48a802b9a9d609538bfa269fab9a', 
    'adm_dHg8', 
    'e2e-test-tenant', 
    NULL, 
    'E2E Test Admin Key', 
    'admin',
    ARRAY['read', 'write', 'admin'], 
    true, 
    1000, 
    60,
    CURRENT_TIMESTAMP, 
    CURRENT_TIMESTAMP
);
```

### Option 2: Create Keys for Existing Secrets

If you have the current `ADMIN_API_KEY` from GitHub secrets:
1. Calculate its SHA-256 hash
2. Insert it into the database with appropriate metadata

### Option 3: Use SSH Access

1. SSH into the production EC2 instance
2. Use psql to connect to the RDS database
3. Insert the API key(s) as needed

## Important Notes

- The full API key can only be seen when generated - it cannot be retrieved from the database
- Store API keys securely in GitHub secrets or a secure vault
- The key hash in the database is a one-way SHA-256 hash
- Keys should be rotated regularly for security

## Debug Logging

Debug logging has been added to the WebSocket authentication to help diagnose issues:
- Logs incoming auth headers
- Logs when custom API key headers are used
- Logs validation failures with error details

These logs will be visible in the production CloudWatch logs.