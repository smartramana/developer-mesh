# Complete User Authentication & Onboarding Guide

This guide walks you through the complete process of registering as a user, managing API keys, and configuring personal access tokens for downstream tools in Developer Mesh.

## Table of Contents

1. [Organization Registration](#1-organization-registration)
2. [User API Key Management](#2-user-api-key-management)
3. [Personal Access Token Registration](#3-personal-access-token-registration)
4. [Using Your Credentials](#4-using-your-credentials)
5. [Security Best Practices](#5-security-best-practices)
6. [Troubleshooting](#6-troubleshooting)

## 1. Organization Registration

### Step 1: Register Your Organization

When you first set up Developer Mesh, you need to register an organization and create an admin user.

```bash
curl -X POST http://localhost:8081/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "My Company",
    "organization_slug": "my-company",
    "admin_email": "admin@mycompany.com",
    "admin_name": "Admin User",
    "admin_password": "SecurePass123!"
  }'
```

**Response:**
```json
{
  "organization": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "My Company",
    "slug": "my-company",
    "subscription_tier": "free",
    "max_users": 5,
    "max_agents": 10
  },
  "user": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "email": "admin@mycompany.com",
    "name": "Admin User",
    "role": "owner"
  },
  "api_key": "adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY",
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

**⚠️ CRITICAL:** Save the `api_key` value immediately. This is your initial admin API key and will **only be shown once**.

### Password Requirements

- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number

### Step 2: Verify Your Email

Check your email inbox for a verification link and click it to activate your account.

### Step 3: Test Your API Key

Verify your API key works:

```bash
curl -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY" \
  http://localhost:8081/api/v1/auth/profile
```

**Expected Response:**
```json
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "email": "admin@mycompany.com",
  "name": "Admin User",
  "role": "owner",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_name": "My Company"
}
```

## 2. User API Key Management

### Why Multiple API Keys?

You may want different API keys for:
- **Development**: Testing and development work
- **Production**: Live deployments
- **CI/CD**: Automated pipelines
- **Mobile Apps**: Mobile applications
- **AI Agents**: Specific AI agents or bots

### Create a New API Key

```bash
curl -X POST http://localhost:8081/api/v1/api-keys \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CI/CD Pipeline Key",
    "key_type": "user",
    "scopes": ["read", "write"]
  }'
```

**Response:**
```json
{
  "message": "API key created successfully. Save this key - it will not be shown again!",
  "api_key": "usr_FoOnztJiTtcoq1BUJSNpxL7rjZheDY5xLFn83VhnKaQ",
  "info": {
    "key_prefix": "usr_FoOn",
    "name": "CI/CD Pipeline Key",
    "key_type": "user",
    "scopes": ["read", "write"],
    "created_at": "2025-10-22T10:00:00Z"
  }
}
```

**⚠️ SAVE IT NOW:** The full API key (`usr_FoOnztJiTtcoq1BUJSNpxL7rjZheDY5xLFn83VhnKaQ`) is shown **only once**.

### Key Types

| Type | Prefix | Description | Use Case |
|------|--------|-------------|----------|
| **admin** | `adm_` | Full administrative access | Organization management, user management |
| **user** | `usr_` | Standard user access | Daily development work, API calls |
| **agent** | `agt_` | AI agent authentication | AI agents, automated bots |
| **gateway** | `gw_` | Service-to-service | Internal microservice communication |

### List Your API Keys

View all your API keys:

```bash
curl http://localhost:8081/api/v1/api-keys \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY"
```

**Response:**
```json
{
  "api_keys": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "key_prefix": "adm_R6C5",
      "name": "Initial Admin Key",
      "key_type": "admin",
      "scopes": ["read", "write", "admin"],
      "is_active": true,
      "created_at": "2025-10-22T09:00:00Z",
      "last_used_at": "2025-10-22T13:30:00Z",
      "usage_count": 247,
      "rate_limit": 1000,
      "rate_window": "60 seconds"
    },
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "key_prefix": "usr_FoOn",
      "name": "CI/CD Pipeline Key",
      "key_type": "user",
      "scopes": ["read", "write"],
      "is_active": true,
      "created_at": "2025-10-22T10:00:00Z",
      "usage_count": 12,
      "rate_limit": 100,
      "rate_window": "60 seconds"
    }
  ],
  "count": 2
}
```

**Note:** The actual key values are never returned. Only metadata is shown for security.

### Revoke an API Key

When a key is no longer needed or compromised:

```bash
curl -X DELETE http://localhost:8081/api/v1/api-keys/880e8400-e29b-41d4-a716-446655440003 \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY"
```

**Response:**
```json
{
  "message": "API key revoked successfully",
  "key_id": "880e8400-e29b-41d4-a716-446655440003"
}
```

**Important:** Revoked keys cannot be reactivated. You must create a new key.

## 3. Personal Access Token Registration

### Overview

Developer Mesh can automatically use your personal access tokens when calling external services like GitHub, Harness, Jira, etc. This means:

- ✅ Actions are performed under your identity
- ✅ Your specific permissions are used
- ✅ Tokens are encrypted (AES-256-GCM)
- ✅ No need to pass tokens with each request
- ✅ Complete audit trail

### Supported Services

| Service | `service_type` | Required Credentials |
|---------|----------------|----------------------|
| GitHub | `github` | `token` |
| Harness | `harness` | `token`, `account_id` |
| Jira | `jira` | `email`, `api_token`, `base_url` (metadata) |
| GitLab | `gitlab` | `token` |
| Bitbucket | `bitbucket` | `username`, `app_password` |
| Jenkins | `jenkins` | `username`, `api_token`, `base_url` (metadata) |
| SonarQube | `sonarqube` | `token`, `base_url` (metadata) |
| Artifactory | `artifactory` | `api_key` or `token` |
| Confluence | `confluence` | `email`, `api_token`, `base_url` (metadata) |
| AWS | `aws` | `access_key`, `secret_key`, `region` |
| Snyk | `snyk` | `token` |
| Slack | `slack` | `token` |

### Register a GitHub Personal Access Token

#### Step 1: Create a GitHub PAT

1. Go to GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Select scopes you need:
   - `repo` - Full repository access
   - `read:org` - Read organization data
   - `read:user` - Read user profile data
   - `workflow` - Update GitHub Actions workflows
   - `admin:org` - Full organization admin access
4. Generate and copy the token

#### Step 2: Register with Developer Mesh

```bash
curl -X POST http://localhost:8081/api/v1/credentials \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY" \
  -H "Content-Type: application/json" \
  -d '{
    "service_type": "github",
    "credentials": {
      "token": "ghp_YourGitHubPersonalAccessTokenHere"
    },
    "scopes": [
      "repo",
      "read:org",
      "read:user",
      "workflow",
      "admin:org"
    ],
    "metadata": {
      "description": "My GitHub PAT for DevMesh",
      "username": "your-github-username"
    }
  }'
```

**Response:**
```json
{
  "id": "990e8400-e29b-41d4-a716-446655440004",
  "service_type": "github",
  "is_active": true,
  "has_credentials": true,
  "metadata": {
    "description": "My GitHub PAT for DevMesh",
    "username": "your-github-username"
  },
  "created_at": "2025-10-22T11:00:00Z"
}
```

### Register a Harness Personal Access Token

#### Step 1: Create a Harness PAT

1. Log into Harness
2. Go to Profile → Personal Access Tokens
3. Create new token with desired permissions
4. Copy the token

#### Step 2: Register with Developer Mesh

```bash
curl -X POST http://localhost:8081/api/v1/credentials \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY" \
  -H "Content-Type: application/json" \
  -d '{
    "service_type": "harness",
    "credentials": {
      "token": "pat.Qn7GW44bQcm65PEEowLdaA.68d2989695a4245aec56dc1f.9yAOwvaNYub9IzO77I6d",
      "account_id": "your-harness-account-id"
    },
    "scopes": [
      "account:view",
      "pipeline:view",
      "pipeline:edit",
      "pipeline:execute",
      "service:view",
      "environment:view"
    ],
    "metadata": {
      "description": "Harness PAT for pipeline management"
    }
  }'
```

### Register a Jira API Token

```bash
curl -X POST http://localhost:8081/api/v1/credentials \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY" \
  -H "Content-Type: application/json" \
  -d '{
    "service_type": "jira",
    "credentials": {
      "email": "your-email@company.com",
      "api_token": "ATATT3xFfGF0..."
    },
    "metadata": {
      "base_url": "https://your-company.atlassian.net",
      "description": "Jira API token for issue management"
    }
  }'
```

### List Your Registered Credentials

```bash
curl http://localhost:8081/api/v1/credentials \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY"
```

**Response:**
```json
{
  "credentials": [
    {
      "id": "990e8400-e29b-41d4-a716-446655440004",
      "service_type": "github",
      "is_active": true,
      "has_credentials": true,
      "metadata": {
        "username": "your-github-username"
      },
      "created_at": "2025-10-22T11:00:00Z",
      "last_used_at": "2025-10-22T13:45:00Z"
    },
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440005",
      "service_type": "harness",
      "is_active": true,
      "has_credentials": true,
      "created_at": "2025-10-22T11:05:00Z"
    }
  ],
  "count": 2
}
```

### Validate Your Credentials

Test if your stored credentials work:

```bash
curl -X POST http://localhost:8081/api/v1/credentials/github/validate \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY"
```

**Response:**
```json
{
  "valid": true,
  "service_type": "github",
  "message": "Credentials validated successfully"
}
```

### Delete Credentials

```bash
curl -X DELETE http://localhost:8081/api/v1/credentials/github \
  -H "Authorization: Bearer adm_R6C5UUYphnIWjhr6hMdbn2J-hgwCdIvXq1cox12UdjY"
```

**Response:** `204 No Content`

## 4. Using Your Credentials

### How It Works

Once you've registered your credentials, Developer Mesh automatically uses them when you make tool calls:

```mermaid
sequenceDiagram
    participant User
    participant DevMesh
    participant GitHub

    User->>DevMesh: MCP tool call (with API key)
    DevMesh->>DevMesh: Validate API key
    DevMesh->>DevMesh: Load user's GitHub credentials
    DevMesh->>GitHub: API call (with user's GitHub PAT)
    GitHub->>DevMesh: Response
    DevMesh->>User: Result (attributed to user)
```

### Example: MCP Tool Call

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "github.create_issue",
    "arguments": {
      "owner": "my-org",
      "repo": "my-repo",
      "title": "New feature request",
      "body": "Description of the feature"
    }
  }
}
```

**What Happens:**
1. Developer Mesh validates your API key
2. Loads your stored GitHub credentials
3. Creates the issue using **your** GitHub PAT
4. Issue is created under **your** GitHub identity
5. Returns the result to you

### Credential Priority

Developer Mesh uses credentials in this order:

1. **User Database Credentials** (highest priority)
   - Your registered tokens in Developer Mesh
   - Retrieved from `mcp.user_credentials` table

2. **Passthrough Tokens** (medium priority)
   - Tokens passed directly in the request
   - Useful for one-time operations

3. **Service Account** (fallback)
   - Organization-wide service account
   - Used when you haven't registered personal tokens

## 5. Security Best Practices

### API Key Security

✅ **DO:**
- Store API keys in environment variables
- Use different keys for different environments
- Rotate keys regularly (every 90 days)
- Revoke unused keys immediately
- Use least-privilege key types

❌ **DON'T:**
- Commit API keys to source control
- Share API keys via email or chat
- Use admin keys for regular operations
- Hardcode keys in applications
- Reuse keys across multiple services

### Personal Access Token Security

✅ **DO:**
- Use minimum required scopes
- Set expiration dates when possible
- Store tokens only in Developer Mesh
- Monitor token usage in audit logs
- Revoke tokens when leaving projects

❌ **DON'T:**
- Grant full admin access unless needed
- Share tokens with other team members
- Use same token across multiple services
- Keep tokens for services you no longer use

### Monitoring

Check your credential usage regularly:

```bash
# View API key usage
curl http://localhost:8081/api/v1/api-keys \
  -H "Authorization: Bearer YOUR_API_KEY"

# Check credential usage
curl http://localhost:8081/api/v1/credentials \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## 6. Troubleshooting

### Problem: "Authentication required" error

**Solution:**
1. Verify your API key is correct (copy-paste carefully)
2. Check the Authorization header format: `Bearer YOUR_API_KEY`
3. Ensure the key hasn't been revoked
4. Verify key hasn't expired

```bash
# Test authentication
curl -v -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8081/api/v1/auth/profile
```

### Problem: "Invalid credentials" when validating

**Solution:**
1. Check the token hasn't expired
2. Verify token has required scopes/permissions
3. For self-hosted services, verify `base_url` is correct
4. Try regenerating the token from the source service

### Problem: Tool calls fail even with registered credentials

**Solution:**
1. Validate your credentials:
   ```bash
   curl -X POST http://localhost:8081/api/v1/credentials/github/validate \
     -H "Authorization: Bearer YOUR_API_KEY"
   ```

2. Check credential was loaded:
   ```bash
   curl http://localhost:8081/api/v1/credentials \
     -H "Authorization: Bearer YOUR_API_KEY"
   ```

3. Verify `last_used_at` timestamp updates after tool call

4. Check logs for credential loading:
   ```bash
   docker-compose -f docker-compose.local.yml logs rest-api | grep "credentials loaded"
   ```

### Problem: API key was lost

**Solution:**
- API keys cannot be retrieved after creation
- You must create a new API key
- Revoke the lost key immediately

```bash
# Create new key
curl -X POST http://localhost:8081/api/v1/api-keys \
  -H "Authorization: Bearer ANOTHER_VALID_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "Replacement Key", "key_type": "user"}'

# Revoke lost key (if you know its ID)
curl -X DELETE http://localhost:8081/api/v1/api-keys/LOST_KEY_ID \
  -H "Authorization: Bearer NEW_KEY"
```

### Problem: Rate limit exceeded

**Solution:**
- Check rate limits on your key:
  ```bash
  curl http://localhost:8081/api/v1/api-keys \
    -H "Authorization: Bearer YOUR_API_KEY"
  ```
- Wait for the rate window to reset (typically 60 seconds)
- Create an admin key for higher rate limits (requires admin role)

## Next Steps

Now that you're authenticated and have credentials configured:

1. **Connect AI Agents**: [AI Agent Integration Guide](../agents/agent-integration-examples.md)
2. **Use MCP Protocol**: [MCP Client Guide](../integrations/claude-code.md)
3. **Configure IDE**: [Cursor Integration](../integrations/cursor.md)
4. **Explore Tools**: [Tool Usage Examples](../../examples/tools/tool-usage-examples.md)
5. **Set up Workflows**: [Workflow Examples](../../examples/README.md)

## Support

If you encounter issues:

1. Check the [troubleshooting section](#6-troubleshooting)
2. Review [authentication operations guide](../../deployment/authentication-operations-guide.md)
3. Check [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)
4. Join [Discussions](https://github.com/developer-mesh/developer-mesh/discussions)

---

**Document Version:** 1.0
**Last Updated:** 2025-10-22
**Contributors:** Developer Mesh Team
