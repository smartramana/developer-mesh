# Organization and User Registration System

## Overview

DevMesh now includes a complete organization and user registration system with multi-tenant isolation, JWT authentication, and invitation-based user onboarding.

## Key Features

- **Organization Self-Registration**: Organizations can register themselves with an admin user
- **Multi-Tenant Isolation**: Each organization gets its own tenant with full data isolation
- **JWT Authentication**: Secure token-based authentication with refresh tokens
- **Invitation System**: Existing users can invite new members to their organization
- **Role-Based Access Control**: Owner, Admin, Member, and ReadOnly roles
- **Password Security**: Bcrypt hashing with password strength requirements
- **Account Security**: Failed login tracking, account lockout, and audit logging
- **Email Verification**: Token-based email verification for new accounts

## Database Schema

### Core Tables

1. **organizations**: Stores organization details with subscription tiers and limits
2. **users**: User accounts with password hashes and organization associations
3. **user_invitations**: Pending invitations to join organizations
4. **user_sessions**: JWT refresh tokens and session management
5. **auth_audit_log**: Security audit trail for all authentication events
6. **email_verification_tokens**: Email verification tracking
7. **password_reset_tokens**: Password reset request tracking

## API Endpoints

### Public Endpoints (No Auth Required)

```
POST /api/v1/auth/register/organization  - Register new organization
POST /api/v1/auth/login                  - User login
POST /api/v1/auth/refresh                - Refresh access token
POST /api/v1/auth/invitation/accept      - Accept invitation
POST /api/v1/auth/password/reset         - Request password reset
POST /api/v1/auth/email/verify           - Verify email address
```

### Protected Endpoints (Auth Required)

```
POST /api/v1/users/invite         - Invite user to organization
GET  /api/v1/users                - List organization users
PUT  /api/v1/users/:id/role      - Update user role
DELETE /api/v1/users/:id          - Remove user
GET  /api/v1/organization         - Get organization details
PUT  /api/v1/organization         - Update organization
GET  /api/v1/profile              - Get user profile
PUT  /api/v1/profile              - Update profile
POST /api/v1/profile/password    - Change password
```

## Registration Flow

### 1. Organization Registration

```bash
curl -X POST http://localhost:8081/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "Acme Corp",
    "organization_slug": "acme-corp",
    "admin_email": "admin@acme.com",
    "admin_name": "John Doe",
    "admin_password": "SecurePass123!",
    "company_size": "10-50",
    "industry": "Technology",
    "use_case": "DevOps automation"
  }'
```

**Response:**
```json
{
  "organization": {
    "id": "uuid",
    "name": "Acme Corp",
    "slug": "acme-corp",
    "subscription_tier": "free",
    "max_users": 5,
    "max_agents": 10
  },
  "user": {
    "id": "uuid",
    "email": "admin@acme.com",
    "name": "John Doe",
    "role": "owner"
  },
  "api_key": "devmesh_xxxxxxxxxxxx",
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

### 2. User Login

```bash
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "SecurePass123!"
  }'
```

**Response:**
```json
{
  "user": {
    "id": "uuid",
    "email": "admin@acme.com",
    "name": "John Doe",
    "role": "owner"
  },
  "organization": {
    "id": "uuid",
    "name": "Acme Corp",
    "slug": "acme-corp"
  },
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "xxxxxxxxxxxx",
  "expires_in": 86400
}
```

### 3. Invite User

```bash
curl -X POST http://localhost:8081/api/v1/users/invite \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "developer@acme.com",
    "name": "Jane Smith",
    "role": "member"
  }'
```

### 4. Accept Invitation

```bash
curl -X POST http://localhost:8081/api/v1/auth/invitation/accept \
  -H "Content-Type: application/json" \
  -d '{
    "token": "invitation_token_from_email",
    "password": "MySecurePass456!"
  }'
```

## Security Features

### Password Requirements
- Minimum 8 characters
- Must contain uppercase letters
- Must contain lowercase letters
- Must contain numbers

### Account Protection
- Failed login attempts tracking
- Account lockout after 5 failed attempts (15 minutes)
- Session-based refresh tokens (30 days expiry)
- JWT access tokens (24 hours expiry)

### Audit Logging
All authentication events are logged:
- Registration attempts
- Login success/failure
- Password changes
- Invitation creation/acceptance
- Role changes

## User Roles

| Role | Permissions |
|------|------------|
| **Owner** | Full access, billing, user management, organization settings |
| **Admin** | User management, agent configuration, tool management |
| **Member** | Read/write access to tools and agents |
| **ReadOnly** | Read-only access to tools and agents |

## Tenant Isolation

Each organization operates in complete isolation:
- Separate tenant_id for all data
- No cross-tenant data access
- API keys scoped to tenant
- User sessions isolated per organization

## Subscription Tiers

| Tier | Max Users | Max Agents | Features |
|------|-----------|------------|----------|
| **Free** | 5 | 10 | Basic features |
| **Starter** | 20 | 50 | Advanced tools |
| **Pro** | 100 | 200 | Premium support |
| **Enterprise** | Unlimited | Unlimited | Custom features |

## Implementation Notes

### Services
- `OrganizationService`: Handles org registration and management
- `UserAuthService`: Manages authentication and user operations
- `RegistrationAPI`: REST API endpoints for auth flows

### Key Files
- `migrations/sql/000027_user_registration_auth.up.sql` - Database schema
- `internal/services/organization_service.go` - Organization logic
- `internal/services/user_auth_service.go` - Authentication logic
- `internal/api/registration_api.go` - API endpoints

## Migration

Run the database migration to set up the registration system:

```bash
make migrate-up
```

## Testing

```bash
# Test organization registration
./scripts/test-registration.sh

# Test user authentication
./scripts/test-auth.sh
```

## Next Steps

1. Implement email service for sending verification emails
2. Add OAuth2 providers (GitHub, Google)
3. Implement subscription management and billing
4. Add two-factor authentication
5. Create admin dashboard for organization management