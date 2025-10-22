# User-Specific Personal Access Token Authentication Flow

## Overview

Developer-mesh now supports user-specific personal access tokens for external tools (GitHub, Jira, SonarQube, Artifactory, Jenkins, GitLab, Bitbucket, Confluence, etc.). Users register their own credentials which are automatically used when making tool calls via the MCP.

## Architecture

### Components

1. **Database**: `mcp.user_credentials` table (already existed)
2. **Models**: Extended `ServiceType` enum with new tools
3. **Services**: `CredentialService` (already existed)
4. **Middleware**: New `UserCredentialMiddleware` loads credentials per request
5. **Auth Providers**: Updated to prioritize user credentials over service accounts

### Authentication Flow

```
┌─────────────┐
│ User/Agent  │
│ with API Key│
└──────┬──────┘
       │
       │ 1. API Request with API Key
       ▼
┌──────────────────┐
│ REST API         │
│ Auth Middleware  │──────┐
└────────┬─────────┘      │ 2. Validate API Key
         │                │    Get user_id & tenant_id
         │ ◄──────────────┘
         │
         │ 3. User authenticated
         ▼
┌──────────────────────┐
│ UserCredential       │
│ Middleware           │──────┐
└────────┬─────────────┘      │ 4. Load user credentials
         │                    │    from database
         │ ◄──────────────────┘    (if any exist)
         │
         │ 5. Credentials added to context
         ▼
┌──────────────────────┐
│ Tool Adapter         │
│ (GitHub, Jira, etc.) │──────┐
└────────┬─────────────┘      │ 6. Check for credentials
         │                    │    Priority:
         │ ◄──────────────────┘    1) User DB credentials
         │                         2) Passthrough tokens
         │                         3) Service account
         │
         │ 7. Execute with user's token
         ▼
┌──────────────────────┐
│ External Service     │
│ (GitHub API, etc.)   │
└──────────────────────┘
```

## Usage

### 1. User Registration

Users register with an organization and receive an API key:

```bash
POST /api/v1/auth/register/organization
{
  "organization_name": "Acme Corp",
  "organization_slug": "acme-corp",
  "admin_email": "admin@acme.com",
  "admin_name": "Jane Admin",
  "admin_password": "SecurePass123!"
}

Response:
{
  "organization": {
    "id": "uuid",
    "name": "Acme Corp",
    "slug": "acme-corp"
  },
  "user": {
    "id": "uuid",
    "email": "admin@acme.com",
    "name": "Jane Admin",
    "role": "owner"
  },
  "api_key": "adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY",
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

**Important:** Save the `api_key` value immediately. This is the only time it will be displayed.

### 1a. Create Additional User API Keys (New!)

After registration, users can create additional API keys for different purposes:

```bash
POST /api/v1/api-keys
Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY
Content-Type: application/json

{
  "name": "My Development Key",
  "key_type": "user",
  "scopes": ["read", "write"]
}

Response:
{
  "message": "API key created successfully. Save this key - it will not be shown again!",
  "api_key": "usr_FoOnztJiTtcoq1BUJSNpxL7rjZheDY5xLFn83VhnKaQ",
  "info": {
    "key_prefix": "usr_FoOn",
    "name": "My Development Key",
    "key_type": "user",
    "scopes": ["read", "write"],
    "created_at": "2025-10-22T10:00:00Z"
  }
}
```

**Key Types:**
- `user`: Standard user access
- `admin`: Administrative privileges (requires admin role)
- `agent`: AI agent authentication

### 1b. List Your API Keys

```bash
GET /api/v1/api-keys
Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY

Response:
{
  "api_keys": [
    {
      "id": "uuid-1",
      "key_prefix": "adm_R6C5",
      "name": "Initial Admin Key",
      "key_type": "admin",
      "scopes": ["read", "write", "admin"],
      "is_active": true,
      "created_at": "2025-10-22T09:00:00Z",
      "last_used_at": "2025-10-22T13:30:00Z",
      "usage_count": 247
    },
    {
      "id": "uuid-2",
      "key_prefix": "usr_FoOn",
      "name": "My Development Key",
      "key_type": "user",
      "scopes": ["read", "write"],
      "is_active": true,
      "created_at": "2025-10-22T10:00:00Z"
    }
  ],
  "count": 2
}
```

### 1c. Revoke an API Key

```bash
DELETE /api/v1/api-keys/{key-id}
Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY

Response:
{
  "message": "API key revoked successfully",
  "key_id": "uuid-2"
}
```

### 2. Store Personal Access Tokens

Users store their personal access tokens for various services:

```bash
POST /api/v1/credentials
Authorization: Bearer dm_1234567890abcdef
Content-Type: application/json

{
  "service_type": "github",
  "credentials": {
    "token": "ghp_xxxxxxxxxxxxx"
  },
  "metadata": {
    "scopes": ["repo", "read:user"]
  }
}
```

Supported service types:
- `github` - GitHub personal access token
- `jira` - Jira API token
- `sonarqube` - SonarQube token
- `artifactory` - Artifactory API key or token
- `jenkins` - Jenkins API token
- `gitlab` - GitLab personal access token
- `bitbucket` - Bitbucket app password or token
- `confluence` - Confluence API token
- `harness` - Harness.io API key
- `aws`, `azure`, `gcp` - Cloud provider credentials
- `snyk` - Snyk API token
- `slack` - Slack bot token
- `generic` - Custom service

### 3. MCP Tool Calls Automatically Use User Credentials

When a user makes a tool call via Claude Code, Cursor, or an agent:

```bash
# MCP Protocol via WebSocket
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "github.create_issue",
    "arguments": {
      "owner": "acme",
      "repo": "backend",
      "title": "Fix authentication bug"
    }
  }
}
```

The system automatically:
1. Validates the API key
2. Loads the user's stored GitHub credentials
3. Uses the user's personal access token for the GitHub API call
4. Returns the result

### 4. List Configured Credentials

```bash
GET /api/v1/credentials
Authorization: Bearer dm_1234567890abcdef

Response:
{
  "credentials": [
    {
      "id": "uuid",
      "service_type": "github",
      "is_active": true,
      "has_credentials": true,
      "created_at": "2025-10-21T10:00:00Z",
      "last_used_at": "2025-10-21T15:30:00Z"
    },
    {
      "id": "uuid",
      "service_type": "jira",
      "is_active": true,
      "has_credentials": true,
      "created_at": "2025-10-21T11:00:00Z"
    }
  ],
  "count": 2
}
```

### 5. Validate Credentials

Test if stored credentials are valid:

```bash
POST /api/v1/credentials/github/validate
Authorization: Bearer dm_1234567890abcdef

Response:
{
  "valid": true,
  "service_type": "github",
  "message": "Credentials validated successfully"
}
```

### 6. Delete Credentials

```bash
DELETE /api/v1/credentials/github
Authorization: Bearer dm_1234567890abcdef

Response: 204 No Content
```

## Security

### Encryption
- All credentials are encrypted at rest using AES-256-GCM
- Per-tenant encryption keys derived with PBKDF2
- Credentials are never logged or exposed in API responses

### Storage
- Encrypted credentials stored in `mcp.user_credentials` table
- Separate encryption key per tenant
- Encryption key version tracking for key rotation

### Audit Trail
- All credential operations logged in `mcp.user_credentials_audit`
- Tracks: create, update, delete, use, validate operations
- Includes IP address, user agent, success/failure

### Access Control
- Users can only manage their own credentials
- Row-level security enforced at database level
- Credentials scoped to user_id and tenant_id

## Credential Priority

When executing a tool, the system checks for credentials in this order:

1. **User Database Credentials** (highest priority)
   - Loaded from `mcp.user_credentials` via `UserCredentialMiddleware`
   - User's personal access tokens

2. **Passthrough Tokens** (medium priority)
   - Tokens passed in the request payload
   - Used for dynamic/temporary credentials

3. **Service Account** (fallback)
   - Organization-wide service account credentials
   - Used when user has no personal credentials configured

## Implementation Details

### Database Schema

```sql
mcp.user_credentials (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    service_type VARCHAR(50),  -- Extended with new types
    encrypted_credentials BYTEA NOT NULL,
    encryption_key_version INT DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    metadata JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    UNIQUE(tenant_id, user_id, service_type)
)
```

### New Service Types Added

Migration `000036_extend_service_types.up.sql` adds:
- `sonarqube`
- `artifactory`
- `jenkins`
- `gitlab`
- `bitbucket`
- `confluence`

### Middleware Stack (REST API)

```
Request
  ↓
Authentication Middleware (validates API key, sets user context)
  ↓
Tenant Context Extraction
  ↓
UserCredential Middleware (loads credentials from DB)
  ↓
Route Handler
  ↓
Tool Execution (uses credentials from context)
  ↓
Response
```

### Key Files Modified/Created

**New Files:**
- `migrations/sql/000036_extend_service_types.up.sql` - Database migration
- `migrations/sql/000036_extend_service_types.down.sql` - Rollback migration
- `apps/rest-api/internal/middleware/user_credential_middleware.go` - Credential loading middleware

**Modified Files:**
- `pkg/models/credential.go` - Added new service types and credential mapping
- `pkg/adapters/github/auth/passthrough_provider.go` - Updated priority order
- `apps/rest-api/internal/api/credential_handler.go` - Added validation endpoint
- `apps/rest-api/internal/api/server.go` - Registered middleware

**Existing (Reused):**
- `pkg/services/credential_service.go` - Already had all needed methods
- `pkg/repository/credential/` - Already implemented
- `mcp.user_credentials` table - Already existed
- `apps/rest-api/internal/api/credential_handler.go` - Already had CRUD endpoints

## Example Credential Formats

### GitHub
```json
{
  "service_type": "github",
  "credentials": {
    "token": "ghp_xxxxxxxxxxxxx"
  }
}
```

### Jira
```json
{
  "service_type": "jira",
  "credentials": {
    "email": "user@company.com",
    "api_token": "ATATT3xFfGF0..."
  },
  "metadata": {
    "base_url": "https://company.atlassian.net"
  }
}
```

### SonarQube
```json
{
  "service_type": "sonarqube",
  "credentials": {
    "token": "squ_xxxxxxxxxxxxx"
  },
  "metadata": {
    "base_url": "https://sonarcloud.io"
  }
}
```

### Jenkins
```json
{
  "service_type": "jenkins",
  "credentials": {
    "username": "admin",
    "api_token": "11xxxxxxxxxxxxxxxxxx"
  },
  "metadata": {
    "base_url": "https://jenkins.company.com"
  }
}
```

## Testing

### Unit Tests
```bash
cd pkg/auth
go test -v ./...
```

### Integration Test
```bash
# 1. Store credential
curl -X POST http://localhost:8081/api/v1/credentials \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "service_type": "github",
    "credentials": {
      "token": "ghp_test_token"
    }
  }'

# 2. List credentials
curl http://localhost:8081/api/v1/credentials \
  -H "Authorization: Bearer YOUR_API_KEY"

# 3. Validate credential
curl -X POST http://localhost:8081/api/v1/credentials/github/validate \
  -H "Authorization: Bearer YOUR_API_KEY"

# 4. Use tool (will automatically use stored credential)
# Via MCP protocol or REST API tool endpoint
```

## Migration Guide

### Running Migrations

```bash
# Apply new service types
cd apps/rest-api
make migrate-up

# Or using golang-migrate directly
migrate -path migrations/sql -database "postgres://..." up
```

### Rollback

```bash
migrate -path migrations/sql -database "postgres://..." down 1
```

## Troubleshooting

### Credentials not loading
- Check logs for "User credentials loaded" message
- Verify API key is valid and user exists
- Ensure credentials exist in database: `SELECT * FROM mcp.user_credentials WHERE user_id = 'xxx';`

### Wrong credentials being used
- Check credential priority order
- Verify user has stored credentials for the service
- Check `last_used_at` timestamp to confirm credentials were loaded

### Validation failing
- Ensure base_url is correct for self-hosted services
- Verify token has not expired
- Check token has required permissions/scopes

## Future Enhancements

- [ ] Implement actual validation API calls (currently only checks format)
- [ ] Add OAuth2 token refresh support
- [ ] Add credential expiry notifications
- [ ] Support multiple credentials per service type
- [ ] Add credential sharing within organizations
- [ ] Implement MCP edge service credential loading
