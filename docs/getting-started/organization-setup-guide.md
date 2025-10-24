# Organization Setup and User Management Guide

This guide walks you through setting up your organization in Developer Mesh and managing users.

## Table of Contents

1. [Initial Setup](#initial-setup)
2. [Organization Registration](#organization-registration)
3. [Authentication](#authentication)
4. [User Management](#user-management)
5. [API Key Management](#api-key-management)
6. [Common Scenarios](#common-scenarios)
7. [Troubleshooting](#troubleshooting)

## Initial Setup

### Prerequisites

Before you begin, ensure you have:

1. Developer Mesh running locally (see [Quick Start Guide](./quick-start-guide.md))
2. Access to the REST API on port 8081
3. A valid email address for the admin account

### Verify Services

```bash
# Check REST API is running
curl http://localhost:8081/health

# Expected response:
# {"status":"healthy","components":{"database":"up","redis":"up"}}
```

## Organization Registration

Every user in Developer Mesh belongs to an organization. The first step is registering your organization.

### Register Your Organization

**Endpoint:** `POST /api/v1/auth/register/organization`

**Request:**

```bash
curl -X POST http://localhost:8081/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "Acme Corporation",
    "organization_slug": "acme-corp",
    "admin_email": "admin@acme.com",
    "admin_name": "John Doe",
    "admin_password": "SecurePass123!",
    "company_size": "50-200",
    "industry": "Technology",
    "use_case": "AI-powered DevOps automation"
  }'
```

**Field Requirements:**

| Field | Required | Format | Description |
|-------|----------|--------|-------------|
| `organization_name` | Yes | 3-100 chars | Your organization's display name |
| `organization_slug` | Yes | 3-50 chars, lowercase, alphanumeric + hyphens | URL-friendly identifier |
| `admin_email` | Yes | Valid email | Admin user's email (must be unique) |
| `admin_name` | Yes | 2-100 chars | Admin user's full name |
| `admin_password` | Yes | 8+ chars, uppercase, lowercase, numbers | Secure password |
| `company_size` | No | String | Optional metadata |
| `industry` | No | String | Optional metadata |
| `use_case` | No | String | Optional metadata |

**Slug Validation Rules:**
- Must match regex: `^[a-z0-9][a-z0-9-]{2,49}$`
- Must start with letter or number
- Can contain lowercase letters, numbers, and hyphens
- Cannot end with hyphen
- Examples: ✅ `acme-corp`, `startup2024` | ❌ `Acme-Corp`, `my_company`, `-company`

**Password Requirements:**
- Minimum 8 characters
- At least one uppercase letter (A-Z)
- At least one lowercase letter (a-z)
- At least one number (0-9)

### Understanding the Response

**Success Response (HTTP 201):**

```json
{
  "organization": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "subscription_tier": "free",
    "max_users": 5,
    "max_agents": 10,
    "billing_email": "admin@acme.com"
  },
  "user": {
    "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
    "email": "admin@acme.com",
    "name": "John Doe",
    "role": "owner",
    "email_verified": false
  },
  "api_key": "devmesh_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8",
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

**What Happens During Registration:**

1. ✅ **Validates** slug format and uniqueness
2. ✅ **Creates** organization record with UUID
3. ✅ **Creates** tenant for multi-tenant isolation
4. ✅ **Hashes** password using bcrypt
5. ✅ **Creates** admin user with "owner" role
6. ✅ **Links** user to organization
7. ✅ **Generates** email verification token
8. ✅ **Creates** initial admin API key (shown once)
9. ✅ **Logs** audit event
10. ✅ **Sends** welcome and verification emails (async)

**Important:** Save the `api_key` immediately. This is your master API key and won't be shown again.

### Error Responses

**Duplicate Slug (HTTP 409):**
```json
{
  "error": "Organization slug already taken",
  "details": "organization slug already exists"
}
```

**Duplicate Email (HTTP 409):**
```json
{
  "error": "Email already registered",
  "details": "email already registered"
}
```

**Invalid Slug Format (HTTP 400):**
```json
{
  "error": "Invalid organization slug format (use lowercase letters, numbers, and hyphens)",
  "details": "invalid organization slug format"
}
```

**Weak Password (HTTP 400):**
```json
{
  "error": "Invalid request",
  "details": "password must contain uppercase, lowercase, and numbers"
}
```

### Organization Limits

Each subscription tier has limits:

| Tier | Max Users | Max Agents | Max API Keys |
|------|-----------|------------|--------------|
| Free | 5 | 10 | 10 |
| Starter | 25 | 50 | 50 |
| Pro | 100 | 200 | 200 |
| Enterprise | Unlimited | Unlimited | Unlimited |

## Authentication

Developer Mesh supports two authentication methods for API access.

### Method 1: API Key Authentication (Recommended for Services)

Use the API key from registration for service-to-service authentication.

**Using Authorization Header (Recommended):**

```bash
curl -H "Authorization: Bearer devmesh_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  http://localhost:8081/api/v1/tools
```

**Using X-API-Key Header:**

```bash
curl -H "X-API-Key: devmesh_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  http://localhost:8081/api/v1/agents
```

**API Key Properties:**
- ✅ Never expires (until manually revoked)
- ✅ Admin-level permissions
- ✅ Scopes: `["read", "write", "admin"]`
- ✅ Tied to organization tenant
- ⚠️ Store securely (treat like a password)

### Method 2: JWT Token Authentication (Recommended for Users)

Login with email/password to get short-lived JWT tokens for user sessions.

**Endpoint:** `POST /api/v1/auth/login`

**Request:**

```bash
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "SecurePass123!"
  }'
```

**Success Response (HTTP 200):**

```json
{
  "user": {
    "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
    "email": "admin@acme.com",
    "name": "John Doe",
    "role": "owner",
    "status": "active",
    "email_verified": true
  },
  "organization": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "subscription_tier": "free",
    "max_users": 5,
    "max_agents": 10
  },
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiI2YmE3YjgxMCIsInRlbmFudF9pZCI6IjU1MGU4NDAwIiwiZW1haWwiOiJhZG1pbkBhY21lLmNvbSIsInNjb3BlcyI6WyJyZWFkIiwid3JpdGUiLCJhZG1pbiIsImJpbGxpbmciXSwiZXhwIjoxNzMwMDAwMDAwfQ.xyz...",
  "refresh_token": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6",
  "expires_in": 86400
}
```

**What Happens During Login:**

1. ✅ **Validates** email and password
2. ✅ **Checks** account status (active, locked, suspended)
3. ✅ **Verifies** password hash with bcrypt
4. ✅ **Resets** failed login attempt counter
5. ✅ **Updates** last_login_at timestamp
6. ✅ **Generates** JWT access token (24h expiry)
7. ✅ **Generates** refresh token (30 days expiry)
8. ✅ **Creates** session record
9. ✅ **Logs** successful login audit event

**Using the JWT Token:**

```bash
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  http://localhost:8081/api/v1/profile
```

**JWT Token Properties:**
- ✅ Expires after 24 hours
- ✅ Scopes based on user role (owner, admin, member, readonly)
- ✅ Includes user_id, tenant_id, email
- ✅ Can be refreshed using refresh token

### Login Error Responses

**Invalid Credentials (HTTP 401):**
```json
{
  "error": "Invalid email or password"
}
```

**Account Locked (HTTP 401):**
```json
{
  "error": "account locked until 2025-01-24T15:30:00Z"
}
```
*Note: Accounts lock after 5 failed login attempts for 15 minutes*

**Account Suspended (HTTP 401):**
```json
{
  "error": "account is suspended"
}
```

### Security Features

**Failed Login Protection:**
- After 5 failed attempts: Account locks for 15 minutes
- Each failed attempt is logged
- Successful login resets counter
- Locked until time shown in error message

**Session Management:**
- Access tokens: 24 hour expiry
- Refresh tokens: 30 day expiry
- Each login creates new session
- Sessions stored in database for revocation

**Audit Trail:**
- All login attempts logged to `mcp.auth_audit_log`
- Includes: user_id, email, timestamp, success/failure, reason

## User Management

### Inviting Users

Only organization **owners** and **admins** can invite new users.

**Endpoint:** `POST /api/v1/users/invite`

**Requirements:**
- Must be authenticated (API key or JWT token)
- User role must be `owner` or `admin`
- Organization must not exceed max_users limit

**Request Example:**

```bash
curl -X POST http://localhost:8081/api/v1/users/invite \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "developer@acme.com",
    "name": "Jane Smith",
    "role": "member"
  }'
```

**Field Requirements:**

| Field | Required | Format | Description |
|-------|----------|--------|-------------|
| `email` | Yes | Valid email | Must not be already registered |
| `name` | Yes | 2-100 chars | Invited user's full name |
| `role` | Yes | `admin`, `member`, or `readonly` | User's access level |

**Success Response (HTTP 200):**

```json
{
  "message": "Invitation sent successfully",
  "email": "developer@acme.com"
}
```

**What Happens During Invitation:**

1. ✅ **Validates** inviter has permission (owner or admin)
2. ✅ **Checks** organization user limit
3. ✅ **Verifies** email not already registered
4. ✅ **Checks** no pending invitation exists
5. ✅ **Generates** secure invitation token (64 chars)
6. ✅ **Stores** invitation (valid for 7 days)
7. ✅ **Sends** invitation email with token link
8. ✅ **Logs** invitation event

**Invitation Email Contains:**
- Inviter's name
- Organization name
- Unique invitation token
- Link to accept invitation
- Token expires in 7 days

### User Roles & Permissions

| Role | Scopes | Can Invite Users | Can Manage Settings | Can Delete Org | Typical Use |
|------|--------|-----------------|--------------------|--------------| ------------|
| `owner` | `read`, `write`, `admin`, `billing` | ✅ | ✅ | ✅ | Organization founder |
| `admin` | `read`, `write`, `admin` | ✅ | ✅ | ❌ | Team leads, managers |
| `member` | `read`, `write` | ❌ | ❌ | ❌ | Developers, users |
| `readonly` | `read` | ❌ | ❌ | ❌ | Auditors, viewers |

### Invitation Error Responses

**Insufficient Permissions (HTTP 403):**
```json
{
  "error": "You don't have permission to invite users"
}
```

**User Already Exists (HTTP 409):**
```json
{
  "error": "User with this email already exists"
}
```

**Invitation Already Sent (HTTP 409):**
```json
{
  "error": "Invitation already sent to this email"
}
```

**User Limit Reached (HTTP 400):**
```json
{
  "error": "organization has reached maximum user limit (5)"
}
```

### Accepting Invitations

When a user receives an invitation email, they use the token to create their account.

**Endpoint:** `POST /api/v1/auth/invitation/accept`

**Request:**

```bash
curl -X POST http://localhost:8081/api/v1/auth/invitation/accept \
  -H "Content-Type: application/json" \
  -d '{
    "token": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0e1f2",
    "password": "MySecurePass456!"
  }'
```

**Field Requirements:**

| Field | Required | Format | Description |
|-------|----------|--------|-------------|
| `token` | Yes | 64 hex chars | Token from invitation email |
| `password` | Yes | 8+ chars, uppercase, lowercase, numbers | New user's password |

**Success Response (HTTP 201 - Auto Login):**

```json
{
  "user": {
    "id": "770e8400-e29b-41d4-a716-446655440002",
    "email": "developer@acme.com",
    "name": "developer@acme.com",
    "role": "member",
    "status": "active",
    "email_verified": true
  },
  "organization": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "subscription_tier": "free",
    "max_users": 5,
    "max_agents": 10
  },
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "xyz789abc...",
  "expires_in": 86400
}
```

**What Happens During Acceptance:**

1. ✅ **Validates** password strength
2. ✅ **Finds** invitation by token
3. ✅ **Checks** invitation not already accepted
4. ✅ **Checks** invitation not expired (7 days)
5. ✅ **Gets** organization's tenant_id
6. ✅ **Hashes** password with bcrypt
7. ✅ **Creates** user account with assigned role
8. ✅ **Marks** email_verified = true (verified via invitation)
9. ✅ **Updates** invitation accepted_at timestamp
10. ✅ **Auto-logins** user with JWT tokens
11. ✅ **Logs** acceptance audit event

**Important Notes:**
- Email is automatically verified (no separate verification needed)
- User is immediately logged in after acceptance
- Default name is email (can be updated later)
- invitation token becomes invalid after use

### Acceptance Error Responses

**Invalid or Expired Token (HTTP 400):**
```json
{
  "error": "invalid or expired invitation"
}
```

**Invitation Already Accepted (HTTP 400):**
```json
{
  "error": "invitation already accepted"
}
```

**Weak Password (HTTP 400):**
```json
{
  "error": "password validation failed: password must contain uppercase, lowercase, and numbers"
}
```

### Complete Invitation Flow Example

```bash
# Step 1: Owner invites a developer
curl -X POST http://localhost:8081/api/v1/users/invite \
  -H "Authorization: Bearer devmesh_owner_api_key_here" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "dev@company.com",
    "name": "Alice Developer",
    "role": "member"
  }'

# Response:
# {"message": "Invitation sent successfully", "email": "dev@company.com"}

# Step 2: Alice receives email with token and accepts
curl -X POST http://localhost:8081/api/v1/auth/invitation/accept \
  -H "Content-Type: application/json" \
  -d '{
    "token": "abc123...def456",
    "password": "AliceSecure2024!"
  }'

# Response includes access_token - Alice is now logged in!

# Step 3: Alice uses the access token
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  http://localhost:8081/api/v1/tools
```

## API Key Management

### API Key Structure

Developer Mesh API keys follow this format:
- Prefix: `devmesh_` (8 characters)
- Random token: 64 hexadecimal characters
- Total length: 72 characters

Example: `devmesh_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0`

### API Key Scopes

API keys can have different scopes:

| Scope | Description |
|-------|-------------|
| `read` | Read access to resources |
| `write` | Create and modify resources |
| `admin` | Administrative operations |

### Security Best Practices

1. **Never share API keys** - Treat them like passwords
2. **Use environment variables** - Don't hardcode keys in code
3. **Rotate keys regularly** - Generate new keys periodically
4. **Use minimal scopes** - Only grant necessary permissions
5. **Monitor usage** - Check for unusual activity

## Common Scenarios

### Scenario 1: Setting Up a Development Team

```bash
# 1. Register organization
curl -X POST http://localhost:8081/api/v1/auth/register/organization \
  -d '{"organization_name":"Dev Team","organization_slug":"dev-team",...}'

# 2. Save the API key from response
export API_KEY="devmesh_xxxxx"

# 3. Invite team members
for email in alice@team.com bob@team.com charlie@team.com; do
  curl -X POST http://localhost:8081/api/v1/users/invite \
    -H "Authorization: Bearer $API_KEY" \
    -d "{\"email\":\"$email\",\"role\":\"member\"}"
done

# 4. Invite team lead as admin
curl -X POST http://localhost:8081/api/v1/users/invite \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"email":"lead@team.com","role":"admin"}'
```

### Scenario 2: CI/CD Integration

For CI/CD pipelines, use a dedicated API key:

```yaml
# GitHub Actions example
name: Deploy with Developer Mesh
env:
  DEVMESH_API_KEY: ${{ secrets.DEVMESH_API_KEY }}
  DEVMESH_API_URL: https://api.devmesh.io

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger Developer Mesh Workflow
        run: |
          curl -X POST $DEVMESH_API_URL/api/v1/workflows/deploy \
            -H "Authorization: Bearer $DEVMESH_API_KEY" \
            -d '{"environment":"production"}'
```

## Troubleshooting

### Common Issues

#### "Organization slug already exists"

The slug must be unique across all organizations. Try a different slug:

```bash
# Add a unique identifier
"organization_slug": "acme-corp-2024"

# Or use department/region
"organization_slug": "acme-engineering"
```

#### "Password validation failed"

Ensure your password meets all requirements:
- ✅ At least 8 characters
- ✅ Contains uppercase letter (A-Z)
- ✅ Contains lowercase letter (a-z)
- ✅ Contains number (0-9)

Good examples:
- `SecurePass123!`
- `MyP@ssw0rd2024`
- `DevMesh2024Admin`

#### "Email already registered"

Each email can only be associated with one user account. Options:
1. Use a different email address
2. Use email aliases (e.g., `admin+dev@company.com`)
3. Contact support to resolve duplicate accounts

#### "Insufficient permissions to invite users"

Only `owner` and `admin` roles can invite users. Check your role:

```bash
# Login and check your user info
curl -X POST http://localhost:8081/api/v1/auth/login \
  -d '{"email":"your@email.com","password":"YourPass123!"}'

# The response includes your role
```

### Getting Help

If you encounter issues not covered here:

1. Check the [API Reference](../reference/api/authentication-api-reference.md)
2. Review server logs: `docker-compose logs rest-api`
3. Search [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)
4. Ask in [Discussions](https://github.com/developer-mesh/developer-mesh/discussions)

## Next Steps

After setting up your organization:

1. [Connect AI Agents](./ai-agent-orchestration.md) - Set up your first AI agent
2. [Configure Tools](../reference/api/dynamic_tools_api.md) - Add DevOps tools
3. [Create Workflows](./multi-agent-collaboration.md) - Build automation workflows
4. [Monitor Usage](./cost-optimization-guide.md) - Track costs and usage